package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install agents, skills, and docs into project or user environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall()
	},
}

var (
	flagScope  string // "project" | "user"
	flagTarget string // "claude" | "opencode" | "all"
)

func init() {
	installCmd.Flags().StringVar(&flagScope, "scope", "project", "install scope: project or user")
	installCmd.Flags().StringVar(&flagTarget, "target", "claude", "agent target: claude, opencode, or all")
}

func runInstall() error {
	// agents
	doClaude := flagTarget == "claude" || flagTarget == "all"
	doOpencode := flagTarget == "opencode" || flagTarget == "all"

	if doClaude {
		if err := installGlob("packages/agents/claude", agentDirClaude(flagScope), "*.md"); err != nil {
			return err
		}
	}
	if doOpencode {
		if err := installGlob("packages/agents/opencode", agentDirOpencode(flagScope), "*.md"); err != nil {
			return err
		}
	}

	// skills & docs always go to claude paths
	if err := installTree("packages/skills", skillsDir(flagScope)); err != nil {
		return err
	}
	if err := installTree("packages/docs", docsDir(flagScope)); err != nil {
		return err
	}
	return nil
}

func installGlob(src, dst, pattern string) error {
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

func installTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
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

// ---------- file copy ----------

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
