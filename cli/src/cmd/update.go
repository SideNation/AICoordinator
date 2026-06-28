package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Pull latest package repo and reinstall plugins whose manifest version changed",
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

	// A pull failure should not block the update — the local clone is still
	// a valid source.
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
	cloneDir := rc.CloneDir
	version := gitCommit(cloneDir)

	for _, rec := range records {
		fmt.Printf("→ updating %s (scope=%s target=%s)\n", rec.Path, rec.Scope, rec.Target)
		updated, err := updateRecord(cloneDir, src, rec, manifest)
		if err != nil {
			return err
		}
		updated.Version = version
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
	cwd, _ := os.Getwd()
	var out []config.InstallRecord
	for _, r := range lock.Installs {
		if r.Scope == "project" && r.Path == cwd {
			out = append(out, r)
		}
	}
	return out
}

// updateRecord reconciles one install record against the manifest. Plugins
// whose declared version differs are reinstalled (assets only — the _init
// scaffold is guarded by InitDone and is not re-applied). Plugins no longer in
// the manifest are offered for removal.
func updateRecord(cloneDir, src string, rec config.InstallRecord, manifest *config.Manifest) (config.InstallRecord, error) {
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

	cliTargets := agent.ParseLockTarget(rec.Target)
	newPlugins := map[string]config.PluginState{}
	for name, locked := range rec.Plugins {
		declared, ok := manifest.PluginVersion(name)
		if !ok {
			if confirmRemoval("plugin", name, rec.Scope) {
				removePluginAssets(pluginDirFor(cloneDir, manifest, name), rec.Scope)
				rec.InitDone = removeStr(rec.InitDone, name)
				continue
			}
			newPlugins[name] = locked
			continue
		}
		if declared == locked.Version {
			newPlugins[name] = locked // unchanged — keep original install/update time
			continue
		}
		fmt.Printf("  %s: %s → %s\n", name, displayVersion(locked.Version), declared)
		targets := effectiveTargets(cliTargets, manifest.Plugins[name].Target)
		ver, err := installPlugin(manifest, cloneDir, name, rec.Scope, targets, &rec)
		if err != nil {
			return rec, err
		}
		newPlugins[name] = config.PluginState{Version: ver, Updated: nowStamp()}
	}

	rec.Plugins = nilIfEmptyStates(newPlugins)

	newDocs := map[string]string{}
	for name, lockedVer := range rec.Docs {
		declared, ok := manifest.DocVersion(name)
		if !ok {
			if confirmRemoval("doc", name, rec.Scope) {
				removeDocAssets(name, rec.Scope)
				continue
			}
			newDocs[name] = lockedVer
			continue
		}
		if declared == lockedVer {
			newDocs[name] = lockedVer // unchanged
			continue
		}
		fmt.Printf("  %s: %s → %s\n", name, displayVersion(lockedVer), declared)
		ver, err := installDoc(manifest, cloneDir, name, rec.Scope, cliTargets)
		if err != nil {
			return rec, err
		}
		newDocs[name] = ver
	}
	rec.Docs = nilIfEmpty(newDocs)

	return rec, nil
}

// removeDocAssets deletes a doc set installed under the canonical docs dir.
// Missing files are tolerated.
func removeDocAssets(name, scope string) {
	os.RemoveAll(filepath.Join(docsDir(scope), name))
}

func nilIfEmptyStates(m map[string]config.PluginState) map[string]config.PluginState {
	if len(m) == 0 {
		return nil
	}
	return m
}

// pluginDirFor resolves a plugin folder even when the manifest no longer
// declares it (removed plugin): falls back to packages/plugins/<name>.
func pluginDirFor(cloneDir string, manifest *config.Manifest, name string) string {
	if _, ok := manifest.Plugins[name]; ok {
		return manifest.PluginDir(cloneDir, name)
	}
	return filepath.Join(config.PackagesDir(cloneDir), "plugins", name)
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

func removeStr(s []string, target string) []string {
	var out []string
	for _, v := range s {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}

// confirmRemoval prompts before deleting an item that no longer appears in the
// manifest. Defaults to "no" on empty input or non-TTY.
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
