package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize aico in the current directory (loads .env, clones package repo)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	env, err := config.LoadDotenv(cwd)
	if err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	gitURL := env["PACKAGE_GIT_URL"]
	if gitURL == "" {
		gitURL = os.Getenv("PACKAGE_GIT_URL")
	}
	if gitURL == "" {
		gitURL, err = promptGitURL()
		if err != nil {
			return err
		}
	}
	gitURL = strings.TrimSpace(gitURL)
	if gitURL == "" {
		return fmt.Errorf("PACKAGE_GIT_URL is required")
	}

	cloneDir := filepath.Join(cwd, repoName(gitURL))

	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err == nil {
		fmt.Printf("package repo already cloned at %s\n", cloneDir)
		fmt.Println("→ pulling latest changes")
		if err := runGit(cloneDir, "pull", "--ff-only"); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	} else {
		fmt.Printf("cloning %s → %s\n", gitURL, cloneDir)
		if err := runGit("", "clone", gitURL, cloneDir); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
	}

	rc := &config.Rc{
		InitDir:  cwd,
		CloneDir: cloneDir,
		GitURL:   gitURL,
	}
	if err := config.SaveRc(rc); err != nil {
		return fmt.Errorf("save .aicorc: %w", err)
	}
	aicoDir, _ := config.AicoDir()
	fmt.Printf("wrote %s/.aicorc\n", aicoDir)
	return nil
}

// repoName derives a directory name from a git URL.
//   git@github.com:acme/aico-packages.git → aico-packages
//   https://github.com/acme/aico-packages.git → aico-packages
//   https://example.com/foo/bar → bar
func repoName(gitURL string) string {
	s := strings.TrimSpace(gitURL)
	// strip trailing slash
	s = strings.TrimRight(s, "/")
	// last path segment (handles both : and / separators)
	idx := strings.LastIndexAny(s, "/:")
	if idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	if s == "" {
		return "packages"
	}
	return s
}

func promptGitURL() (string, error) {
	fmt.Print("PACKAGE_GIT_URL (git clone URL): ")
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// runGit executes git in the given directory (empty = cwd inherits).
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
