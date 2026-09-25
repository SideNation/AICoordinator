package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <plugin>...",
	Short: "Remove installed plugins (their agents, skills, docs, rules, hooks), or doc sets with --docs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRm(args)
	},
}

var flagRmGlobal, flagRmDocs bool

func init() {
	rmCmd.Flags().BoolVarP(&flagRmGlobal, "global", "g", false, "remove from user-scope (home) install instead of current project")
	rmCmd.Flags().BoolVar(&flagRmDocs, "docs", false, "remove doc sets (installed by aico docs) instead of plugins")
	rmCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
	rootCmd.AddCommand(rmCmd)
}

func runRm(patterns []string) error {
	scope := scopeFromGlobal(flagRmGlobal)
	installPath := scopeRoot(scope)

	lock, err := config.LoadLock()
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range lock.Installs {
		if r.Path == installPath && r.Scope == scope {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("no install tracked at %s (scope=%s)", installPath, scope)
	}
	rec := &lock.Installs[idx]

	if scope == "project" {
		orig, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(rec.Path); err != nil {
			return fmt.Errorf("chdir %s: %w", rec.Path, err)
		}
		defer os.Chdir(orig)
	}

	src, err := resolvePackagesSrc()
	if err != nil {
		return err
	}
	cloneDir := filepath.Dir(src)
	manifest, _ := config.LoadManifest(cloneDir) // best-effort; rm works from lock too

	if flagRmDocs {
		var resolve func(string) (string, bool)
		if manifest != nil {
			resolve = manifest.ResolveDoc
		}
		matches := matchInstalled(rec.Docs, resolve, patterns)
		if len(matches) == 0 {
			fmt.Fprintf(os.Stderr, "warning: no installed docs match %v\n", patterns)
			return nil
		}
		for _, name := range matches {
			removeDocAssets(name, scope)
			delete(rec.Docs, name)
			fmt.Printf("removed doc %s\n", name)
		}
		rec.Docs = nilIfEmpty(rec.Docs)
		return config.SaveLock(lock)
	}

	var resolve func(string) (string, bool)
	if manifest != nil {
		resolve = func(p string) (string, bool) {
			name, _, ok := manifest.ResolvePlugin(p)
			return name, ok
		}
	}
	matches := matchInstalled(rec.Plugins, resolve, patterns)
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no installed plugins match %v\n", patterns)
		return nil
	}

	for _, name := range matches {
		st := rec.Plugins[name]
		removePluginAssets(pluginDirFor(cloneDir, manifest, name), scope, st.Assets)
		delete(rec.Plugins, name)
		rec.InitDone = removeStr(rec.InitDone, name)
		fmt.Printf("removed plugin %s\n", name)
	}
	rec.Plugins = nilIfEmptyStates(rec.Plugins)

	return config.SaveLock(lock)
}

// matchInstalled resolves patterns against the installed names (keys of
// installed). A pattern may be a name, a manifest-resolved name/alias (via
// resolve, which may be nil), or a glob over installed names. Results are
// sorted and deduplicated.
func matchInstalled[V any](installed map[string]V, resolve func(string) (string, bool), patterns []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if _, ok := installed[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, p := range patterns {
		if isGlob(p) {
			for name := range installed {
				if ok, _ := filepath.Match(p, name); ok {
					add(name)
				}
			}
			continue
		}
		if resolve != nil {
			if name, ok := resolve(p); ok {
				add(name)
				continue
			}
		}
		add(p)
	}
	sort.Strings(out)
	return out
}

func isGlob(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// removePluginAssets deletes the on-disk artifacts a plugin installed. When the
// recorded asset set is available (assets != nil) it removes exactly those, so
// removal works even if the plugin source folder has since changed or vanished.
// For legacy records with no recorded assets it falls back to re-reading the
// current plugin source folder. Missing files are tolerated either way.
func removePluginAssets(pluginDir, scope string, assets *config.PluginAssets) {
	if assets != nil {
		removeRecordedAssets(assets, scope)
		return
	}
	removeSourceAssets(pluginDir, scope)
}

// removeRecordedAssets deletes exactly the assets listed in a lock record.
func removeRecordedAssets(a *config.PluginAssets, scope string) {
	for _, name := range a.Skills {
		os.RemoveAll(filepath.Join(skillsDir(scope), name))
	}
	for _, name := range a.Agents {
		for _, p := range agent.Platforms() {
			os.Remove(p.Path(scope, name))
		}
	}
	for _, dir := range []struct {
		root string
		rels []string
	}{
		{docsDir(scope), a.Docs},
		{rulesDir(scope), a.Rules},
		{hooksDir(scope), a.Hooks},
	} {
		for _, rel := range dir.rels {
			full := filepath.Join(dir.root, rel)
			os.Remove(full)
			pruneEmptyParents(dir.root, filepath.Dir(full))
		}
	}
}

// removeSourceAssets deletes a plugin's artifacts by re-reading its source
// folder — the legacy path used when no asset set was recorded.
func removeSourceAssets(pluginDir, scope string) {
	// agents — remove the rendered file for every platform extension.
	if entries, err := os.ReadDir(filepath.Join(pluginDir, "agents")); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			for _, p := range agent.Platforms() {
				os.Remove(p.Path(scope, name))
			}
		}
	}
	// skills — remove each named skill directory from the canonical store.
	if entries, err := os.ReadDir(filepath.Join(pluginDir, "skills")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				os.RemoveAll(filepath.Join(skillsDir(scope), e.Name()))
			}
		}
	}
	// docs / rules / hooks — remove the files this plugin contributed.
	removeMirroredTree(filepath.Join(pluginDir, "docs"), docsDir(scope))
	removeMirroredTree(filepath.Join(pluginDir, "rules"), rulesDir(scope))
	removeMirroredTree(filepath.Join(pluginDir, "hooks"), hooksDir(scope))
}

// removeMirroredTree removes, from dst, every file that exists at the same
// relative path under src. Empty and missing trees are ignored.
func removeMirroredTree(src, dst string) {
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return nil
		}
		os.Remove(filepath.Join(dst, rel))
		return nil
	})
}
