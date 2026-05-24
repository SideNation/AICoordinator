package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var codexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Create a .agents symlink pointing to .claude (so Codex can reuse Claude assets)",
	Long: `Create a link named ".agents" that points at ".claude" in the current
directory (or in the user's home with -g). On macOS/Linux this is a regular
symlink; on Windows it is a directory junction (created via "mklink /J"),
which works without administrator privileges.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCodex()
	},
}

var (
	flagCodexGlobal bool
	flagCodexForce  bool
)

func init() {
	codexCmd.Flags().BoolVarP(&flagCodexGlobal, "global", "g", false, "link in user home (~/.agents → ~/.claude) instead of current project")
	codexCmd.Flags().BoolVarP(&flagCodexForce, "force", "f", false, "replace an existing .agents symlink/junction (regular directories are never replaced)")
	rootCmd.AddCommand(codexCmd)
}

func runCodex() error {
	base := "."
	if flagCodexGlobal {
		h, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home dir: %w", err)
		}
		base = h
	}

	target := filepath.Join(base, ".claude")
	link := filepath.Join(base, ".agents")

	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("create %s: %w", target, err)
		}
		fmt.Printf("created %s\n", target)
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", target, err)
	}

	if info, err := os.Lstat(link); err == nil {
		isLink := info.Mode()&os.ModeSymlink != 0
		if !isLink {
			return fmt.Errorf("%s already exists and is not a symlink; refusing to replace", link)
		}
		if !flagCodexForce {
			return fmt.Errorf("%s already exists; pass --force to replace", link)
		}
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("remove existing %s: %w", link, err)
		}
		fmt.Printf("removed existing link %s\n", link)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", link, err)
	}

	if err := createDirLink(target, link); err != nil {
		return fmt.Errorf("link %s → %s: %w", link, target, err)
	}
	fmt.Printf("linked %s → %s\n", link, target)
	return nil
}

