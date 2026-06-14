package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initTreeCmd = &cobra.Command{
	Use:   "init-tree [plugin...]",
	Short: "Copy a plugin's _init scaffold into the current project (re-runnable)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInitTree(args)
	},
}

var flagInitTreeAll bool

func init() {
	initTreeCmd.Flags().BoolVar(&flagInitTreeAll, "all", false, "scaffold every plugin declared in the manifest")
	initTreeCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
	rootCmd.AddCommand(initTreeCmd)
}

func runInitTree(args []string) error {
	src, err := resolvePackagesSrc()
	if err != nil {
		return err
	}
	cloneDir := filepath.Dir(src)
	manifest, err := config.LoadManifest(cloneDir)
	if err != nil {
		return err
	}

	names, err := resolveRequestedPlugins(manifest, args, flagInitTreeAll)
	if err != nil {
		return err
	}
	if names == nil {
		return nil
	}
	names = expandChains(manifest, names)

	projectRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	lock, err := config.LoadLock()
	if err != nil {
		return err
	}
	rec := loadOrNewRecord(lock, "project", []string{})

	for _, name := range names {
		pluginDir := manifest.PluginDir(cloneDir, name)
		applied, err := applyInitTree(pluginDir, projectRoot, true)
		if err != nil {
			return err
		}
		if applied && !containsStr(rec.InitDone, name) {
			rec.InitDone = append(rec.InitDone, name)
		}
		if !applied {
			fmt.Printf("plugin %s has no init-tree.yaml, skipping\n", name)
		}
	}

	lock.Upsert(rec)
	return config.SaveLock(lock)
}

// applyInitTree copies a plugin's _init scaffold into projectRoot following the
// plugin's init-tree.yaml mapping. The YAML maps a destination group to a list
// of entries: the "workspace" group copies to projectRoot/<entry>; any other
// group G copies to projectRoot/<G>/<entry>. Each entry's source is
// _init/<entry>, falling back to <pluginDir>/<group>/<entry> (e.g. a sidecar
// file like .claude/statusline.sh). When overwrite is false, existing files are
// left untouched. Returns false when the plugin has no init-tree.yaml.
func applyInitTree(pluginDir, projectRoot string, overwrite bool) (bool, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "init-tree.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var tree map[string][]string
	if err := yaml.Unmarshal(data, &tree); err != nil {
		return false, fmt.Errorf("parse init-tree.yaml for %s: %w", filepath.Base(pluginDir), err)
	}

	initRoot := filepath.Join(pluginDir, "_init")
	for group, entries := range tree {
		for _, entry := range entries {
			srcPath := filepath.Join(initRoot, entry)
			if _, err := os.Stat(srcPath); err != nil {
				alt := filepath.Join(pluginDir, group, entry)
				if _, err2 := os.Stat(alt); err2 == nil {
					srcPath = alt
				} else {
					fmt.Fprintf(os.Stderr, "warning: scaffold entry %q not found (group %s), skipping\n", entry, group)
					continue
				}
			}
			var dst string
			if group == "workspace" {
				dst = filepath.Join(projectRoot, entry)
			} else {
				dst = filepath.Join(projectRoot, group, entry)
			}
			if err := copyPath(srcPath, dst, overwrite); err != nil {
				return false, err
			}
			fmt.Printf("  scaffold %s → %s\n", entry, dst)
		}
	}
	return true, nil
}

// copyPath copies a file or directory tree from src to dst. When overwrite is
// false, files already present at the destination are skipped.
func copyPath(src, dst string, overwrite bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if !overwrite {
			if _, err := os.Stat(dst); err == nil {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return copyFile(src, dst)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !overwrite {
			if _, err := os.Stat(target); err == nil {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}
