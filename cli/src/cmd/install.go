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
}

var (
	flagScope  string // "project" | "user"
	flagTarget string // "claude" | "opencode" | "all"
)

func init() {
	installCmd.AddCommand(installAgentCmd, installSkillsCmd, installDocsCmd)

	for _, cmd := range []*cobra.Command{installAgentCmd, installSkillsCmd, installDocsCmd} {
		cmd.Flags().StringVar(&flagScope, "scope", "project", "install scope: project or user")
	}
	installAgentCmd.Flags().StringVar(&flagTarget, "target", "claude", "agent target: claude, opencode, or all")
}

// ---------- install agent ----------

var installAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Install built agent files into claude/opencode agent directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstallAgent()
	},
}

func runInstallAgent() error {
	pkgDir := "packages/agents"

	installTarget := func(src, dst string) error {
		entries, err := filepath.Glob(filepath.Join(src, "*.md"))
		if err != nil {
			return fmt.Errorf("glob %s: %w", src, err)
		}
		if len(entries) == 0 {
			fmt.Printf("no files in %s\n", src)
			return nil
		}
		if err := os.MkdirAll(dst, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dst, err)
		}
		for _, src := range entries {
			name := filepath.Base(src)
			dest := filepath.Join(dst, name)
			if err := copyFile(src, dest); err != nil {
				return err
			}
			fmt.Printf("installed %s → %s\n", src, dest)
		}
		return nil
	}

	doClaude := flagTarget == "claude" || flagTarget == "all"
	doOpencode := flagTarget == "opencode" || flagTarget == "all"

	if doClaude {
		dst := agentDirClaude(flagScope)
		if err := installTarget(filepath.Join(pkgDir, "claude"), dst); err != nil {
			return err
		}
	}
	if doOpencode {
		dst := agentDirOpencode(flagScope)
		if err := installTarget(filepath.Join(pkgDir, "opencode"), dst); err != nil {
			return err
		}
	}
	return nil
}

// ---------- install skills ----------

var installSkillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Install skill files into .claude/skills (project or user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstallDir("packages/skills", skillsDir(flagScope), "skill")
	},
}

// ---------- install docs ----------

var installDocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Install doc files into .claude/docs (project or user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstallDir("packages/docs", docsDir(flagScope), "doc")
	},
}

func runInstallDir(src, dst, kind string) error {
	entries, err := filepath.Glob(filepath.Join(src, "*"))
	if err != nil {
		return fmt.Errorf("glob %s: %w", src, err)
	}
	if len(entries) == 0 {
		fmt.Printf("no files in %s\n", src)
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
		fmt.Printf("installed %s %s → %s\n", kind, f, dest)
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
