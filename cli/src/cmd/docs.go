package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs [name...]",
	Short: "Install documentation sets declared in the manifest (docs: section)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDocs(args)
	},
}

func init() {
	docsCmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "install to user home (~/.claude/docs ...) instead of current project")
	docsCmd.Flags().StringVar(&flagTarget, "target", "claude,codex", "target platforms: comma list of claude|cl, codex|co, kilo|ki, opencode|op, or all. opencode/kilo are opt-in")
	docsCmd.Flags().BoolVar(&flagAll, "all", false, "install every doc declared in the manifest")
	docsCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
	rootCmd.AddCommand(docsCmd)
}

func runDocs(args []string) error {
	src, err := resolvePackagesSrc()
	if err != nil {
		return err
	}
	cloneDir := filepath.Dir(src)
	manifest, err := config.LoadManifest(cloneDir)
	if err != nil {
		return err
	}

	cliTargets, err := agent.ParseTargetSpec(flagTarget)
	if err != nil {
		return err
	}

	names, err := resolveRequestedDocs(manifest, args, flagAll)
	if err != nil {
		return err
	}
	if names == nil {
		return nil // picker message already printed
	}

	scope := scopeFromGlobal(flagGlobal)
	lock, err := config.LoadLock()
	if err != nil {
		return err
	}
	rec := loadOrNewRecord(lock, scope, cliTargets)

	// Reconcile: drop installed docs that vanished from the manifest.
	pruneVanishedDocs(manifest, &rec)

	for _, name := range names {
		ver, err := installDoc(manifest, cloneDir, name, scope, cliTargets)
		if err != nil {
			return err
		}
		if rec.Docs == nil {
			rec.Docs = map[string]string{}
		}
		rec.Docs[name] = ver
	}

	rec.Version = gitCommit(repoRoot(src))
	lock.Upsert(rec)
	return config.SaveLock(lock)
}

// resolveRequestedDocs turns CLI args / --all into a list of canonical doc
// names. Returns (nil, nil) after printing the picker when neither a name nor
// --all was given.
func resolveRequestedDocs(m *config.Manifest, args []string, all bool) ([]string, error) {
	if all {
		return m.DocNames(), nil
	}
	if len(args) == 0 {
		printDocPicker(m)
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range args {
		name, ok := m.ResolveDoc(a)
		if !ok {
			return nil, fmt.Errorf("unknown doc %q — run `aico list --docs --manifest` to see available docs", a)
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

func printDocPicker(m *config.Manifest) {
	fmt.Println("문서 이름을 지정하세요.  예) aico docs <name>   또는   aico docs --all")
	fmt.Println("설치 가능한 문서:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, n := range m.DocNames() {
		e := m.Docs[n]
		fmt.Fprintf(w, "  %s\t%s\t%s\n", n, e.Version, e.Source)
	}
	w.Flush()
}

// installDoc copies one manifest doc set from packages/docs/<name>/ into
// Claude's canonical docs dir (.claude/docs/<name>/) and links a per-platform
// docs directory for every non-Claude target. Returns the version recorded in
// the lock (the manifest's docs.<name>.version).
func installDoc(m *config.Manifest, cloneDir, name, scope string, targets []string) (string, error) {
	docSrc := filepath.Join(config.PackagesDir(cloneDir), "docs", name)
	if _, err := os.Stat(docSrc); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("doc %q not found at %s — crawl it first (see the docs-to-markdown skill)", name, docSrc)
		}
		return "", err
	}
	fmt.Printf("→ installing doc %s (targets=%s)\n", name, strings.Join(targets, ","))

	dst := filepath.Join(docsDir(scope), name)
	if err := installTree(docSrc, dst); err != nil {
		return "", err
	}
	fmt.Printf("  docs %s → %s\n", name, dst)

	if agent.HasNonClaude(targets) {
		if err := linkDocsBridge(scope, targets); err != nil {
			return "", err
		}
	}

	ver, _ := m.DocVersion(name)
	return ver, nil
}
