package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed plugins, the manifest, or plugins available to install",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

var (
	flagListGlobal    bool
	flagListAll       bool
	flagListAvailable bool
	flagListManifest  bool
)

func init() {
	listCmd.Flags().BoolVarP(&flagListGlobal, "global", "g", false, "operate on the user-scope (home) install instead of current project")
	listCmd.Flags().BoolVarP(&flagListAll, "all", "a", false, "show every install record in .lock (across projects and user)")
	listCmd.Flags().BoolVarP(&flagListAvailable, "available", "v", false, "show plugins declared in the manifest but not yet installed")
	listCmd.Flags().BoolVarP(&flagListManifest, "manifest", "m", false, "show the full manifest with install status")
	rootCmd.AddCommand(listCmd)
}

func runList() error {
	if flagListAvailable && flagListManifest {
		return fmt.Errorf("--available and --manifest are mutually exclusive")
	}
	lock, err := config.LoadLock()
	if err != nil {
		return err
	}
	if flagListAll {
		return listAllRecords(lock)
	}
	if flagListAvailable || flagListManifest {
		return listVsManifest(lock)
	}
	return listInstalled(lock)
}

func listInstalled(lock *config.Lock) error {
	scope := scopeFromGlobal(flagListGlobal)
	rec := findInstall(lock, scope)
	if rec == nil {
		fmt.Printf("no install tracked at %s (scope=%s)\n", scopeRoot(scope), scope)
		return nil
	}
	fmt.Printf("Install: %s (scope=%s, target=%s)\n", rec.Path, rec.Scope, rec.Target)
	if rec.Version != "" {
		fmt.Printf("  package version: %s\n", rec.Version)
	}
	printPluginSection("Plugins", rec.Plugins, "")
	printPluginSection("Docs", rec.Docs, "")
	if len(rec.Plugins)+len(rec.Docs) == 0 {
		fmt.Println("  (no items)")
	}
	return nil
}

func listAllRecords(lock *config.Lock) error {
	if len(lock.Installs) == 0 {
		fmt.Println("no installs tracked")
		return nil
	}
	for i, rec := range lock.Installs {
		fmt.Printf("Install %d: %s (scope=%s, target=%s)\n", i+1, rec.Path, rec.Scope, rec.Target)
		printPluginSection("Plugins", rec.Plugins, "  ")
		printPluginSection("Docs", rec.Docs, "  ")
		fmt.Println()
	}
	return nil
}

func listVsManifest(lock *config.Lock) error {
	rc, err := config.LoadRc()
	if err != nil {
		return err
	}
	if rc == nil || rc.CloneDir == "" {
		return fmt.Errorf(".aicorc not found — run `aico init` first")
	}
	manifest, err := config.LoadManifest(rc.CloneDir)
	if err != nil {
		return err
	}
	scope := scopeFromGlobal(flagListGlobal)
	rec := findInstall(lock, scope)
	installed := map[string]string{}
	if rec != nil {
		installed = rec.Plugins
	}

	if flagListAvailable {
		var names []string
		for n := range manifest.Plugins {
			if _, ok := installed[n]; !ok {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		if len(names) == 0 {
			fmt.Println("all manifest plugins are installed")
			return nil
		}
		fmt.Println("Available to install (declared in manifest, not yet installed):")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, n := range names {
			fmt.Fprintf(w, "  %s\t%s\n", n, manifest.Plugins[n].Version)
		}
		w.Flush()
		return nil
	}

	fmt.Println("Plugins:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, n := range manifest.PluginNames() {
		declVer := manifest.Plugins[n].Version
		haveVer, ok := installed[n]
		marker, status := "○", "not installed"
		if ok {
			marker = "✓"
			switch {
			case haveVer == "":
				status = "installed (untracked) → update available"
			case haveVer == declVer:
				status = "up to date"
			default:
				status = fmt.Sprintf("installed %s → update available", haveVer)
			}
		}
		fmt.Fprintf(w, "  %s %s\t%s\t%s\n", marker, n, declVer, status)
	}
	w.Flush()
	return nil
}

func findInstall(lock *config.Lock, scope string) *config.InstallRecord {
	path := scopeRoot(scope)
	for i, r := range lock.Installs {
		if r.Path == path && r.Scope == scope {
			return &lock.Installs[i]
		}
	}
	return nil
}

func printPluginSection(title string, m map[string]string, indent string) {
	if len(m) == 0 {
		return
	}
	fmt.Printf("%s%s:\n", indent, title)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		v := m[name]
		if v == "" {
			v = "(untracked)"
		}
		fmt.Fprintf(w, "%s  %s\t%s\n", indent, name, v)
	}
	w.Flush()
}
