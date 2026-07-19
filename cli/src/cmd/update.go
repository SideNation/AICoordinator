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
				removePluginAssets(pluginDirFor(cloneDir, manifest, name), rec.Scope, locked.Assets)
				rec.InitDone = removeStr(rec.InitDone, name)
				continue
			}
			newPlugins[name] = locked
			continue
		}
		if declared == locked.Version {
			// A skill (or other asset) can be deleted from a plugin without a
			// version bump, so reconcile against the current source and refresh
			// the owned set even on an unchanged version — but only when the
			// source is present, so a missing clone never triggers wholesale
			// removal of everything the record owns.
			pluginDir := pluginDirFor(cloneDir, manifest, name)
			if _, err := os.Stat(pluginDir); err == nil {
				targets := effectiveTargets(cliTargets, manifest.Plugins[name].Target)
				collected := collectPluginAssets(pluginDir, rec.Scope, targets)
				reconcilePluginAssets(locked.Assets, collected, otherPluginAssets(&rec, name), rec.Scope)
				locked.Assets = &collected
				rec.Plugins[name] = locked
			}
			newPlugins[name] = locked // unchanged — keep original install/update time
			continue
		}
		fmt.Printf("  %s: %s → %s\n", name, displayVersion(locked.Version), declared)
		targets := effectiveTargets(cliTargets, manifest.Plugins[name].Target)
		ver, assets, err := installPlugin(manifest, cloneDir, name, rec.Scope, targets, &rec)
		if err != nil {
			return rec, err
		}
		state := config.PluginState{Version: ver, Updated: nowStamp(), Assets: &assets}
		newPlugins[name] = state
		rec.Plugins[name] = state
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

// pruneVanishedPlugins removes, from rec, every installed plugin that no longer
// appears in the manifest (after per-item confirmation). Called at the start of
// `aico plugin` so a manifest edit that drops a plugin also cleans up its
// installed assets. Only touches the current scope's paths, so the caller must
// already be in the record's directory (install runs in cwd).
func pruneVanishedPlugins(cloneDir string, manifest *config.Manifest, rec *config.InstallRecord) {
	if len(rec.Plugins) == 0 {
		return
	}
	kept := map[string]config.PluginState{}
	for name, st := range rec.Plugins {
		if _, ok := manifest.PluginVersion(name); ok {
			kept[name] = st
			continue
		}
		if confirmRemoval("plugin", name, rec.Scope) {
			removePluginAssets(pluginDirFor(cloneDir, manifest, name), rec.Scope, st.Assets)
			rec.InitDone = removeStr(rec.InitDone, name)
			continue
		}
		kept[name] = st
	}
	rec.Plugins = nilIfEmptyStates(kept)
}

// pruneVanishedDocs removes, from rec, every installed doc that no longer
// appears in the manifest (after per-item confirmation). Called at the start of
// `aico docs`.
func pruneVanishedDocs(manifest *config.Manifest, rec *config.InstallRecord) {
	if len(rec.Docs) == 0 {
		return
	}
	kept := map[string]string{}
	for name, ver := range rec.Docs {
		if _, ok := manifest.DocVersion(name); ok {
			kept[name] = ver
			continue
		}
		if confirmRemoval("doc", name, rec.Scope) {
			removeDocAssets(name, rec.Scope)
			continue
		}
		kept[name] = ver
	}
	rec.Docs = nilIfEmpty(kept)
}

// pruneOrphanSkills performs a one-time cleanup of skill directories left over
// from before asset tracking existed: any skill in the canonical store that no
// installed plugin's current source provides. It runs only when at least one
// installed plugin has no recorded assets (a legacy record) — once every record
// carries its asset set, precise per-plugin reconciliation replaces this sweep.
// If any installed plugin's source folder can't be read it skips the sweep
// entirely, so a missing clone never mistakes a plugin's real skills for orphans.
// Scoped to skills only: that is where stranding actually occurs and where a
// union diff over the shared store is safe enough (user skills default to keep).
func pruneOrphanSkills(cloneDir string, manifest *config.Manifest, rec *config.InstallRecord, scope string) {
	if len(rec.Plugins) == 0 || !hasLegacyRecord(rec) {
		return
	}
	provided := map[string]struct{}{}
	for name := range rec.Plugins {
		pluginDir := pluginDirFor(cloneDir, manifest, name)
		if _, err := os.Stat(pluginDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: plugin %q source not found (%v) — skipping orphan-skill cleanup\n", name, err)
			return
		}
		entries, err := os.ReadDir(filepath.Join(pluginDir, "skills"))
		if err != nil {
			if os.IsNotExist(err) {
				continue // plugin ships no skills
			}
			fmt.Fprintf(os.Stderr, "warning: cannot read %q skills (%v) — skipping orphan-skill cleanup\n", name, err)
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				provided[e.Name()] = struct{}{}
			}
		}
	}

	installed, err := os.ReadDir(skillsDir(scope))
	if err != nil {
		return
	}
	for _, e := range installed {
		if !e.IsDir() {
			continue
		}
		if _, ok := provided[e.Name()]; ok {
			continue
		}
		if confirmRemove("skill", e.Name(), scope, "is not provided by any installed plugin") {
			os.RemoveAll(filepath.Join(skillsDir(scope), e.Name()))
		}
	}
}

// hasLegacyRecord reports whether any installed plugin predates asset tracking.
func hasLegacyRecord(rec *config.InstallRecord) bool {
	for _, st := range rec.Plugins {
		if st.Assets == nil {
			return true
		}
	}
	return false
}

// reconcilePluginAssets removes the assets a plugin used to install but no
// longer provides — everything recorded in old that is absent from the current
// source set cur and not owned by another installed plugin — after per-item
// confirmation. old is nil for legacy records, in which case there is nothing
// to reconcile (the one-time sweep handles those).
func reconcilePluginAssets(old *config.PluginAssets, cur, other config.PluginAssets, scope string) {
	if old == nil {
		return
	}
	const reason = "was removed from its plugin"
	for _, name := range subtractStrs(subtractStrs(old.Skills, cur.Skills), other.Skills) {
		if confirmRemove("skill", name, scope, reason) {
			os.RemoveAll(filepath.Join(skillsDir(scope), name))
		}
	}
	for _, name := range subtractStrs(subtractStrs(old.Agents, cur.Agents), other.Agents) {
		if confirmRemove("agent", name, scope, reason) {
			for _, p := range agent.Platforms() {
				os.Remove(p.Path(scope, name))
			}
		}
	}
	reconcileFiles(subtractStrs(subtractStrs(old.Docs, cur.Docs), other.Docs), docsDir(scope), "doc", scope, reason)
	reconcileFiles(subtractStrs(subtractStrs(old.Rules, cur.Rules), other.Rules), rulesDir(scope), "rule", scope, reason)
	reconcileFiles(subtractStrs(subtractStrs(old.Hooks, cur.Hooks), other.Hooks), hooksDir(scope), "hook", scope, reason)
}

// otherPluginAssets returns the union of assets recorded for every plugin in
// rec except name. These are still installed in the same scope, so a shared
// artifact must survive this plugin's reconciliation.
func otherPluginAssets(rec *config.InstallRecord, name string) config.PluginAssets {
	var out config.PluginAssets
	for otherName, state := range rec.Plugins {
		if otherName == name || state.Assets == nil {
			continue
		}
		out.Skills = append(out.Skills, state.Assets.Skills...)
		out.Agents = append(out.Agents, state.Assets.Agents...)
		out.Docs = append(out.Docs, state.Assets.Docs...)
		out.Rules = append(out.Rules, state.Assets.Rules...)
		out.Hooks = append(out.Hooks, state.Assets.Hooks...)
	}
	return out
}

// reconcileFiles removes each relative path under dir after confirmation, then
// prunes directories left empty by the removals.
func reconcileFiles(rels []string, dir, kind, scope, reason string) {
	for _, rel := range rels {
		if confirmRemove(kind, rel, scope, reason) {
			full := filepath.Join(dir, rel)
			os.Remove(full)
			pruneEmptyParents(dir, filepath.Dir(full))
		}
	}
}

// pruneEmptyParents removes empty directories from leaf upward, stopping before
// root. A non-empty or missing directory ends the walk.
func pruneEmptyParents(root, leaf string) {
	for leaf != root && strings.HasPrefix(leaf, root+string(filepath.Separator)) {
		if err := os.Remove(leaf); err != nil {
			return
		}
		leaf = filepath.Dir(leaf)
	}
}

// subtractStrs returns the elements of a that are not in b, preserving a's order.
func subtractStrs(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	var out []string
	for _, x := range a {
		if _, ok := set[x]; !ok {
			out = append(out, x)
		}
	}
	return out
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
	return confirmRemovalReason(kind, name, scope, "is not in the manifest")
}

// confirmRemove is the confirmation hook used by asset reconciliation and the
// orphan sweep. It is a variable so tests can stub the interactive prompt.
var confirmRemove = confirmRemovalReason

// confirmRemovalReason prompts before deleting an item, stating why. Defaults
// to "no" on empty input or non-TTY.
func confirmRemovalReason(kind, name, scope, reason string) bool {
	fmt.Printf("  %s %q (scope=%s) %s. Remove it? [y/N]: ", kind, name, scope, reason)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
