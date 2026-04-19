package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install agents, skills, and (optionally) docs into project or user environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall()
	},
}

var (
	flagScope  string // "project" | "user"
	flagTarget string // "claude" | "opencode" | "all"
	flagDocs   bool   // include docs (default false)
	flagSrc    string // override packages source root
)

func init() {
	installCmd.Flags().StringVar(&flagScope, "scope", "project", "install scope: project or user")
	installCmd.Flags().StringVar(&flagTarget, "target", "claude", "agent target: claude, opencode, or all")
	installCmd.Flags().BoolVar(&flagDocs, "docs", false, "also install docs (default: skipped)")
	installCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
}

func runInstall() error {
	src, err := resolvePackagesSrc()
	if err != nil {
		return err
	}
	if err := doInstall(src, flagScope, flagTarget, flagDocs); err != nil {
		return err
	}
	return recordInstall(src, flagScope, flagTarget, flagDocs)
}

// doInstall copies agents (and optionally docs/skills) from src packages dir
// into the scope's destinations.
func doInstall(src, scope, target string, withDocs bool) error {
	doClaude := target == "claude" || target == "all"
	doOpencode := target == "opencode" || target == "all"

	if doClaude {
		if err := installGlob(filepath.Join(src, "agents", "claude"), agentDirClaude(scope), "*.md"); err != nil {
			return err
		}
	}
	if doOpencode {
		if err := installGlob(filepath.Join(src, "agents", "opencode"), agentDirOpencode(scope), "*.md"); err != nil {
			return err
		}
	}

	if err := installTree(filepath.Join(src, "skills"), skillsDir(scope)); err != nil {
		return err
	}
	if withDocs {
		if err := installTree(filepath.Join(src, "docs"), docsDir(scope)); err != nil {
			return err
		}
	}
	return nil
}

// resolvePackagesSrc returns the root of `agents/ skills/ docs/` to install from.
// Priority: --src flag > .aicorc clone_dir/packages > ./packages (legacy fallback).
func resolvePackagesSrc() (string, error) {
	if flagSrc != "" {
		return flagSrc, nil
	}
	rc, err := config.LoadRc()
	if err != nil {
		return "", err
	}
	if rc != nil && rc.CloneDir != "" {
		return config.PackagesDir(rc.CloneDir), nil
	}
	// legacy fallback for users who have not run `aico init` yet
	if _, err := os.Stat("packages"); err == nil {
		fmt.Fprintln(os.Stderr, "warning: .aicorc not found — falling back to ./packages. Run `aico init` to set up the package repo.")
		return "packages", nil
	}
	return "", fmt.Errorf("no package source found — run `aico init` first or pass --src")
}

// recordInstall updates ~/.aico/.lock with this install.
func recordInstall(src, scope, target string, withDocs bool) error {
	lock, err := config.LoadLock()
	if err != nil {
		return err
	}

	installPath := scopeRoot(scope)
	version := gitCommit(repoRoot(src))

	lock.Upsert(config.InstallRecord{
		Path:    installPath,
		Scope:   scope,
		Target:  target,
		Docs:    withDocs,
		Version: version,
	})
	if err := config.SaveLock(lock); err != nil {
		return fmt.Errorf("save .lock: %w", err)
	}
	return nil
}

// scopeRoot returns the anchor directory for a scope record — project uses CWD
// so different projects each get their own .lock entry.
func scopeRoot(scope string) string {
	if scope == "user" {
		h, _ := os.UserHomeDir()
		return h
	}
	cwd, _ := os.Getwd()
	return cwd
}

// repoRoot walks up from src to find a `.git` directory. Returns src when none
// is found (so gitCommit yields "" gracefully).
func repoRoot(src string) string {
	dir, err := filepath.Abs(src)
	if err != nil {
		return src
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return src
		}
		dir = parent
	}
}

func gitCommit(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---------- copy primitives ----------

func installGlob(src, dst, pattern string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	entries, err := filepath.Glob(filepath.Join(src, pattern))
	if err != nil {
		return fmt.Errorf("glob %s: %w", src, err)
	}
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}
	for _, f := range entries {
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		dest := filepath.Join(dst, filepath.Base(f))
		if err := copyFile(f, dest); err != nil {
			return err
		}
		fmt.Printf("installed %s → %s\n", f, dest)
	}
	return nil
}

func installTree(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		fmt.Printf("installed %s → %s\n", path, target)
		return nil
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	return nil
}

// ---------- path helpers ----------

func agentDirClaude(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "agents")
	}
	return filepath.Join(".claude", "agents")
}

func agentDirOpencode(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".config", "opencode", "agents")
	}
	return filepath.Join(".opencode", "agents")
}

func skillsDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "skills")
	}
	return filepath.Join(".claude", "skills")
}

func docsDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "docs")
	}
	return filepath.Join(".claude", "docs")
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "~"
}
