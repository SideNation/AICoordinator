package cmd

import (
	"fmt"
	"os"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Pull latest package repo and reinstall to tracked locations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate()
	},
}

var (
	flagUpdateAll  bool
	flagUpdateUser bool
	flagUpdateDocs bool
)

func init() {
	updateCmd.Flags().BoolVar(&flagUpdateAll, "all", false, "update every tracked install in .lock (prunes missing project dirs)")
	updateCmd.Flags().BoolVar(&flagUpdateUser, "user", false, "update only the user-scope install")
	updateCmd.Flags().BoolVar(&flagUpdateDocs, "docs", false, "also install docs where missing")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate() error {
	rc, err := config.LoadRc()
	if err != nil {
		return err
	}
	if rc == nil || rc.CloneDir == "" {
		return fmt.Errorf(".aicorc not found — run `aico init` first")
	}

	// Step 1: pull latest. A pull failure (offline, detached HEAD, unconfigured
	// upstream, etc.) should NOT block reinstall — the local clone is still a
	// valid source and --docs may need to fill in missing files.
	fmt.Printf("→ pulling %s\n", rc.CloneDir)
	if err := runGit(rc.CloneDir, "pull", "--ff-only"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git pull failed (%v) — continuing with current clone\n", err)
	}

	lock, err := config.LoadLock()
	if err != nil {
		return err
	}

	// Step 2: select records to update
	records := pickUpdateRecords(lock, rc)
	if len(records) == 0 {
		fmt.Println("no matching installs to update")
		return nil
	}

	src := config.PackagesDir(rc.CloneDir)
	version := gitCommit(rc.CloneDir)

	for i, rec := range records {
		docs := rec.Docs || flagUpdateDocs
		fmt.Printf("→ updating %s (scope=%s target=%s docs=%v)\n", rec.Path, rec.Scope, rec.Target, docs)
		if err := installAtPath(src, rec.Path, rec.Scope, rec.Target, docs); err != nil {
			return err
		}
		records[i].Docs = docs
		records[i].Version = version
		lock.Upsert(records[i])
	}

	return config.SaveLock(lock)
}

// pickUpdateRecords returns the subset of lock entries to refresh, based on
// flags. Also removes stale (missing project path) entries when --all is used.
func pickUpdateRecords(lock *config.Lock, rc *config.Rc) []config.InstallRecord {
	if flagUpdateAll {
		removed := lock.RemoveMissing()
		for _, r := range removed {
			fmt.Printf("pruned missing install %s (scope=%s)\n", r.Path, r.Scope)
		}
		return lock.Installs
	}
	if flagUpdateUser {
		var out []config.InstallRecord
		for _, r := range lock.Installs {
			if r.Scope == "user" {
				out = append(out, r)
			}
		}
		return out
	}
	// no flag: update current directory if tracked
	cwd, _ := os.Getwd()
	var out []config.InstallRecord
	for _, r := range lock.Installs {
		if r.Scope == "project" && r.Path == cwd {
			out = append(out, r)
		}
	}
	return out
}

// installAtPath runs doInstall with cwd temporarily set so relative project
// paths (.claude/agents, .opencode/agents) resolve to the record's Path.
func installAtPath(src, path, scope, target string, docs bool) error {
	if scope == "project" {
		orig, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(path); err != nil {
			return fmt.Errorf("chdir %s: %w", path, err)
		}
		defer os.Chdir(orig)
	}
	return doInstall(src, scope, target, docs)
}
