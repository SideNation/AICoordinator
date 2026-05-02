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
	Short:   "List installed items, manifest contents, or items available to install",
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
	listCmd.Flags().BoolVarP(&flagListAvailable, "available", "v", false, "show items declared in manifest but not yet installed at the target scope")
	listCmd.Flags().BoolVarP(&flagListManifest, "manifest", "m", false, "show the full manifest with install status for the target scope")
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

// listInstalled prints the install record matching cwd (or home for -g).
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
	printSection("Agents", rec.Agents, "")
	printSection("Skills", rec.Skills, "")
	printSection("Docs", rec.Docs, "")
	printSection("Rules", rec.Rules, "")

	if len(rec.Agents)+len(rec.Skills)+len(rec.Docs)+len(rec.Rules) == 0 {
		fmt.Println("  (no items)")
	}
	return nil
}

// listAllRecords prints every install record in .lock, in a compact form.
func listAllRecords(lock *config.Lock) error {
	if len(lock.Installs) == 0 {
		fmt.Println("no installs tracked")
		return nil
	}
	for i, rec := range lock.Installs {
		fmt.Printf("Install %d: %s (scope=%s, target=%s)\n", i+1, rec.Path, rec.Scope, rec.Target)
		printSection("Agents", rec.Agents, "  ")
		printSection("Skills", rec.Skills, "  ")
		printSection("Docs", rec.Docs, "  ")
		printSection("Rules", rec.Rules, "  ")
		fmt.Println()
	}
	return nil
}

// listVsManifest compares manifest contents against the target-scope install
// record and prints either available-to-install items or the full manifest
// with status, depending on which flag is set.
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
	installed := installedView(rec)

	if flagListAvailable {
		printAvailable(manifest, installed)
		return nil
	}
	printManifestWithStatus(manifest, installed)
	return nil
}

// installedView extracts the per-kind installed maps from a record (which may
// be nil) so callers can do membership/version lookups without nil checks.
func installedView(rec *config.InstallRecord) struct {
	agents, skills, docs, rules map[string]string
} {
	if rec == nil {
		return struct {
			agents, skills, docs, rules map[string]string
		}{}
	}
	return struct {
		agents, skills, docs, rules map[string]string
	}{rec.Agents, rec.Skills, rec.Docs, rec.Rules}
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

// printSection writes a "<indent>Title:\n<indent>  name version\n" block via
// tabwriter so columns line up. Empty maps print nothing. The indent prefix
// is preserved on every line so nested blocks (--all view) align correctly.
func printSection(title string, m map[string]string, indent string) {
	if len(m) == 0 {
		return
	}
	fmt.Printf("%s%s:\n", indent, title)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, name := range sortedKeys(m) {
		v := m[name]
		if v == "" {
			v = "(untracked)"
		}
		fmt.Fprintf(w, "%s  %s\t%s\n", indent, name, v)
	}
	w.Flush()
}

// printAvailable lists manifest entries not present in the installed maps.
func printAvailable(manifest *config.Manifest, installed struct {
	agents, skills, docs, rules map[string]string
}) {
	avail := func(title string, declared map[string]config.ManifestEntry, have map[string]string) {
		var names []string
		for name := range declared {
			if _, ok := have[name]; !ok {
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			return
		}
		sort.Strings(names)
		fmt.Printf("%s:\n", title)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, n := range names {
			fmt.Fprintf(w, "  %s\t%s\n", n, declared[n].Version)
		}
		w.Flush()
	}

	fmt.Println("Available to install (declared in manifest, not yet installed):")
	avail("Agents", manifest.Agents, installed.agents)
	avail("Skills", manifest.Skills, installed.skills)
	avail("Docs", manifest.Docs, installed.docs)
	avail("Rules", manifest.Rules, installed.rules)
}

// printManifestWithStatus prints every manifest entry annotated with install
// status: ✓ installed (with version delta), ○ not installed.
func printManifestWithStatus(manifest *config.Manifest, installed struct {
	agents, skills, docs, rules map[string]string
}) {
	section := func(title string, declared map[string]config.ManifestEntry, have map[string]string) {
		if len(declared) == 0 {
			return
		}
		fmt.Printf("%s:\n", title)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		var names []string
		for n := range declared {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			declVer := declared[n].Version
			haveVer, ok := have[n]
			marker := "○"
			status := "not installed"
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
	}

	section("Agents", manifest.Agents, installed.agents)
	section("Skills", manifest.Skills, installed.skills)
	section("Docs", manifest.Docs, installed.docs)
	section("Rules", manifest.Rules, installed.rules)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
