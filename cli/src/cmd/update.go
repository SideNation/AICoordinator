package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Pull latest package repo and reinstall items whose manifest version changed",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate()
	},
}

var (
	flagUpdateAll    bool
	flagUpdateGlobal bool
)

func init() {
	updateCmd.Flags().BoolVar(&flagUpdateAll, "all", false, "update every tracked install in .lock (prunes missing project dirs)")
	updateCmd.Flags().BoolVarP(&flagUpdateGlobal, "global", "g", false, "update only the user-scope (home) install")
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

	// Step 1: pull latest. A pull failure should not block the update — the
	// local clone is still a valid source.
	fmt.Printf("→ pulling %s\n", rc.CloneDir)
	if err := runGit(rc.CloneDir, "pull", "--ff-only"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git pull failed (%v) — continuing with current clone\n", err)
	}

	manifest, err := config.LoadManifest(rc.CloneDir)
	if err != nil {
		return err
	}

	lock, err := config.LoadLock()
	if err != nil {
		return err
	}

	records := pickUpdateRecords(lock, rc)
	if len(records) == 0 {
		fmt.Println("no matching installs to update")
		return nil
	}

	src := config.PackagesDir(rc.CloneDir)
	version := gitCommit(rc.CloneDir)

	for i, rec := range records {
		fmt.Printf("→ updating %s (scope=%s target=%s)\n", rec.Path, rec.Scope, rec.Target)
		updated, err := updateRecord(src, rec, manifest)
		if err != nil {
			return err
		}
		updated.Version = version
		records[i] = updated
		lock.Upsert(updated)
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
	if flagUpdateGlobal {
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

// updateRecord reconciles one install record against the manifest:
//   - items whose manifest version differs from the locked version are reinstalled
//   - items no longer in the manifest are offered for deletion (y/n)
//   - items whose version matches are left alone
//
// Returns the updated record with refreshed version maps.
func updateRecord(src string, rec config.InstallRecord, manifest *config.Manifest) (config.InstallRecord, error) {
	if rec.Scope == "project" {
		orig, err := os.Getwd()
		if err != nil {
			return rec, err
		}
		if err := os.Chdir(rec.Path); err != nil {
			return rec, fmt.Errorf("chdir %s: %w", rec.Path, err)
		}
		defer os.Chdir(orig)
	}

	doClaude := rec.Target == "claude" || rec.Target == "all"
	doOpencode := rec.Target == "opencode" || rec.Target == "all"

	// ----- agents -----
	newAgents := map[string]string{}
	for name, locked := range rec.Agents {
		declared, ok := manifest.AgentVersion(name)
		if !ok {
			if confirmRemoval("agent", name, rec.Scope) {
				removeAgent(rec.Scope, name, doClaude, doOpencode)
				continue
			}
			newAgents[name] = locked
			continue
		}
		if declared == locked {
			newAgents[name] = locked
			continue
		}
		fmt.Printf("  agents/%s: %s → %s\n", name, displayVersion(locked), declared)
		if err := reinstallAgent(src, rec.Scope, name, doClaude, doOpencode); err != nil {
			return rec, err
		}
		newAgents[name] = declared
	}

	// ----- skills -----
	newSkills := map[string]string{}
	for name, locked := range rec.Skills {
		declared, ok := manifest.SkillVersion(name)
		if !ok {
			if confirmRemoval("skill", name, rec.Scope) {
				target := filepath.Join(skillsDir(rec.Scope), name)
				if err := os.RemoveAll(target); err != nil {
					return rec, fmt.Errorf("remove %s: %w", target, err)
				}
				continue
			}
			newSkills[name] = locked
			continue
		}
		if declared == locked {
			newSkills[name] = locked
			continue
		}
		fmt.Printf("  skills/%s: %s → %s\n", name, displayVersion(locked), declared)
		target := filepath.Join(skillsDir(rec.Scope), name)
		if err := os.RemoveAll(target); err != nil {
			return rec, fmt.Errorf("remove %s: %w", target, err)
		}
		if err := installTree(filepath.Join(src, "skills", name), target); err != nil {
			return rec, err
		}
		newSkills[name] = declared
	}

	// ----- docs -----
	newDocs := map[string]string{}
	for name, locked := range rec.Docs {
		declared, ok := manifest.DocVersion(name)
		if !ok {
			if confirmRemoval("doc", name, rec.Scope) {
				target := filepath.Join(docsDir(rec.Scope), name)
				if err := os.RemoveAll(target); err != nil {
					return rec, fmt.Errorf("remove %s: %w", target, err)
				}
				continue
			}
			newDocs[name] = locked
			continue
		}
		if declared == locked {
			newDocs[name] = locked
			continue
		}
		fmt.Printf("  docs/%s: %s → %s\n", name, displayVersion(locked), declared)
		target := filepath.Join(docsDir(rec.Scope), name)
		if err := os.RemoveAll(target); err != nil {
			return rec, fmt.Errorf("remove %s: %w", target, err)
		}
		if err := installTree(filepath.Join(src, "docs", name), target); err != nil {
			return rec, err
		}
		newDocs[name] = declared
	}

	rec.Agents = nilIfEmpty(newAgents)
	rec.Skills = nilIfEmpty(newSkills)
	rec.Docs = nilIfEmpty(newDocs)
	return rec, nil
}

func displayVersion(v string) string {
	if v == "" {
		return "(untracked)"
	}
	return v
}

func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

// reinstallAgent copies the agent .md file for the enabled targets, creating
// the destination directory when needed.
func reinstallAgent(src, scope, name string, doClaude, doOpencode bool) error {
	if doClaude {
		if _, err := installAgentFile(filepath.Join(src, "agents", "claude"), agentDirClaude(scope), name); err != nil {
			return err
		}
	}
	if doOpencode {
		if _, err := installAgentFile(filepath.Join(src, "agents", "opencode"), agentDirOpencode(scope), name); err != nil {
			return err
		}
	}
	return nil
}

// removeAgent deletes the installed `<name>.md` agent file from whichever
// target directories are enabled for this record. Missing files are ignored.
func removeAgent(scope, name string, doClaude, doOpencode bool) {
	if doClaude {
		os.Remove(filepath.Join(agentDirClaude(scope), name+".md"))
	}
	if doOpencode {
		os.Remove(filepath.Join(agentDirOpencode(scope), name+".md"))
	}
}

// confirmRemoval prompts the user before deleting an item that no longer
// appears in the manifest. Defaults to "no" on empty input or non-TTY.
func confirmRemoval(kind, name, scope string) bool {
	fmt.Printf("  %s %q (scope=%s) is not in the manifest. Remove it? [y/N]: ", kind, name, scope)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
