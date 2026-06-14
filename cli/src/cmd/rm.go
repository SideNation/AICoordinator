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
	Short: "Remove installed plugins (their agents, skills, docs, rules, hooks)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRm(args)
	},
}

var flagRmGlobal bool

func init() {
	rmCmd.Flags().BoolVarP(&flagRmGlobal, "global", "g", false, "remove from user-scope (home) install instead of current project")
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

	matches := matchPlugins(rec, manifest, patterns)
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no installed plugins match %v\n", patterns)
		return nil
	}

	for _, name := range matches {
		removePluginAssets(pluginDirFor(cloneDir, manifest, name), scope)
		delete(rec.Plugins, name)
		rec.InitDone = removeStr(rec.InitDone, name)
		fmt.Printf("removed plugin %s\n", name)
	}
	rec.Plugins = nilIfEmpty(rec.Plugins)

	return config.SaveLock(lock)
}

// matchPlugins resolves patterns against the installed plugin set. A pattern
// may be a plugin name, an alias (resolved via the manifest), or a glob over
// installed plugin names. Results are sorted and deduplicated.
func matchPlugins(rec *config.InstallRecord, manifest *config.Manifest, patterns []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if _, ok := rec.Plugins[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, p := range patterns {
		if isGlob(p) {
			for name := range rec.Plugins {
				if ok, _ := filepath.Match(p, name); ok {
					add(name)
				}
			}
			continue
		}
		if manifest != nil {
			if name, _, ok := manifest.ResolvePlugin(p); ok {
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

// removePluginAssets deletes the on-disk artifacts a plugin installed by
// re-reading the plugin source folder. Missing files are tolerated.
func removePluginAssets(pluginDir, scope string) {
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
