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
	flagListDocs      bool
	flagListPlugins   bool
)

func init() {
	listCmd.Flags().BoolVarP(&flagListGlobal, "global", "g", false, "operate on the user-scope (home) install instead of current project")
	listCmd.Flags().BoolVarP(&flagListAll, "all", "a", false, "show every install record in .lock (across projects and user)")
	listCmd.Flags().BoolVarP(&flagListAvailable, "available", "v", false, "show items declared in the manifest but not yet installed")
	listCmd.Flags().BoolVarP(&flagListManifest, "manifest", "m", false, "show the full manifest with install status")
	listCmd.Flags().BoolVarP(&flagListDocs, "docs", "d", false, "show only docs (default: docs and plugins)")
	listCmd.Flags().BoolVarP(&flagListPlugins, "plugins", "p", false, "show only plugins (default: docs and plugins)")
	rootCmd.AddCommand(listCmd)
}

// showPlugins / showDocs decode the --docs/--plugins filter: neither flag means
// show both; either flag narrows output to that kind.
func showPlugins() bool { return flagListPlugins || !flagListDocs }
func showDocs() bool    { return flagListDocs || !flagListPlugins }

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
	shown := 0
	if showPlugins() {
		printPluginStates("Plugins", rec.Plugins, "")
		shown += len(rec.Plugins)
	}
	if showDocs() {
		printPluginSection("Docs", rec.Docs, "")
		shown += len(rec.Docs)
	}
	if shown == 0 {
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
		if showPlugins() {
			printPluginStates("Plugins", rec.Plugins, "  ")
		}
		if showDocs() {
			printPluginSection("Docs", rec.Docs, "  ")
		}
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
	instPlugins := map[string]config.PluginState{}
	instDocs := map[string]string{}
	if rec != nil {
		instPlugins = rec.Plugins
		instDocs = rec.Docs
	}

	if flagListAvailable {
		printed := false
		if showPlugins() {
			printed = printAvailable("plugins", manifest.PluginNames(), func(n string) (string, bool) {
				_, ok := instPlugins[n]
				return manifest.Plugins[n].Version, ok
			}) || printed
		}
		if showDocs() {
			printed = printAvailable("docs", manifest.DocNames(), func(n string) (string, bool) {
				_, ok := instDocs[n]
				return manifest.Docs[n].Version, ok
			}) || printed
		}
		if !printed {
			fmt.Println("all manifest items are installed")
		}
		return nil
	}

	if showPlugins() {
		fmt.Println("Plugins:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, n := range manifest.PluginNames() {
			declVer := manifest.Plugins[n].Version
			st, ok := instPlugins[n]
			marker, status := installStatus(declVer, st.Version, ok)
			if ok && st.Updated != "" {
				status += " (" + st.Updated + ")"
			}
			fmt.Fprintf(w, "  %s %s\t%s\t%s\n", marker, n, declVer, status)
		}
		w.Flush()
	}
	if showDocs() {
		fmt.Println("Docs:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, n := range manifest.DocNames() {
			declVer := manifest.Docs[n].Version
			iv, ok := instDocs[n]
			marker, status := installStatus(declVer, iv, ok)
			fmt.Fprintf(w, "  %s %s\t%s\t%s\n", marker, n, declVer, status)
		}
		w.Flush()
	}
	return nil
}

// installStatus returns a ✓/○ marker and a human status comparing a declared
// manifest version to the installed version (instVer; "" = untracked install).
func installStatus(declVer, instVer string, installed bool) (string, string) {
	if !installed {
		return "○", "not installed"
	}
	switch {
	case instVer == "":
		return "✓", "installed (untracked) → update available"
	case instVer == declVer:
		return "✓", "up to date"
	default:
		return "✓", fmt.Sprintf("installed %s → update available", instVer)
	}
}

// printAvailable lists manifest items of one kind (names, pre-sorted) that are
// not yet installed. lookup returns the declared version and whether the item
// is installed. Returns true when it printed a section.
func printAvailable(kind string, names []string, lookup func(string) (string, bool)) bool {
	var avail []string
	for _, n := range names {
		if _, ok := lookup(n); !ok {
			avail = append(avail, n)
		}
	}
	if len(avail) == 0 {
		return false
	}
	fmt.Printf("Available %s (declared in manifest, not yet installed):\n", kind)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, n := range avail {
		ver, _ := lookup(n)
		fmt.Fprintf(w, "  %s\t%s\n", n, ver)
	}
	w.Flush()
	return true
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

// printPluginStates prints "<name> <version> <installed/updated time>" for each
// installed plugin so `list` shows what is installed and when.
func printPluginStates(title string, m map[string]config.PluginState, indent string) {
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
		st := m[name]
		v := st.Version
		if v == "" {
			v = "(untracked)"
		}
		upd := st.Updated
		if upd == "" {
			upd = "-"
		}
		fmt.Fprintf(w, "%s  %s\t%s\t%s\n", indent, name, v, upd)
	}
	w.Flush()
}
