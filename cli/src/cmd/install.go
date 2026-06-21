package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [plugin...]",
	Short: "Install plugins (agents, skills, docs, rules, hooks, platform sidecars)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args)
	},
}

var (
	flagGlobal bool   // true = user scope (home), false = project scope (cwd)
	flagTarget string // comma list of platform names/aliases, or "all"
	flagAll    bool   // install every plugin in the manifest
	flagSrc    string // override packages source root
)

func init() {
	installCmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "install to user home (~/.claude, ~/.codex, ~/.config/...) instead of current project")
	installCmd.Flags().StringVar(&flagTarget, "target", "all", "target platforms: comma list of claude|cl, codex|co, kilo|ki, opencode|op, or all")
	installCmd.Flags().BoolVar(&flagAll, "all", false, "install every plugin declared in the manifest")
	installCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
}

func runInstall(args []string) error {
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

	names, err := resolveRequestedPlugins(manifest, args, flagAll)
	if err != nil {
		return err
	}
	if names == nil {
		return nil // picker message already printed
	}
	names = expandChains(manifest, names)

	scope := scopeFromGlobal(flagGlobal)
	lock, err := config.LoadLock()
	if err != nil {
		return err
	}
	rec := loadOrNewRecord(lock, scope, cliTargets)

	for _, name := range names {
		entry := manifest.Plugins[name]
		targets := effectiveTargets(cliTargets, entry.Target)
		if len(targets) == 0 {
			fmt.Printf("skipped %s (no enabled targets match plugin target=%v)\n", name, entry.Target)
			continue
		}
		ver, err := installPlugin(manifest, cloneDir, name, scope, targets, &rec)
		if err != nil {
			return err
		}
		if rec.Plugins == nil {
			rec.Plugins = map[string]config.PluginState{}
		}
		rec.Plugins[name] = config.PluginState{Version: ver, Updated: nowStamp()}
	}

	rec.Version = gitCommit(repoRoot(src))
	lock.Upsert(rec)
	return config.SaveLock(lock)
}

// ---------------------------------------------------------------------------
// plugin selection
// ---------------------------------------------------------------------------

// resolveRequestedPlugins turns CLI args / --all into a list of canonical
// plugin names. Returns (nil, nil) after printing the picker when neither a
// name nor --all was given.
func resolveRequestedPlugins(m *config.Manifest, args []string, all bool) ([]string, error) {
	if all {
		return m.PluginNames(), nil
	}
	if len(args) == 0 {
		printPluginPicker(m)
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range args {
		name, _, ok := m.ResolvePlugin(a)
		if !ok {
			return nil, fmt.Errorf("unknown plugin %q — run `aico list` to see available plugins", a)
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

func printPluginPicker(m *config.Manifest) {
	fmt.Println("플러그인 이름을 지정하세요.  예) aico install <name>   또는   aico install --all")
	fmt.Println("설치 가능한 플러그인:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, n := range m.PluginNames() {
		e := m.Plugins[n]
		fmt.Fprintf(w, "  %s\t%s\t%s\n", n, e.Version, strings.Join(e.Alias, ", "))
	}
	w.Flush()
}

// expandChains returns names plus every plugin reachable through chain[],
// deduplicated, with chained plugins ordered after the plugin that pulls them.
func expandChains(m *config.Manifest, names []string) []string {
	seen := map[string]bool{}
	var out []string
	var visit func(string)
	visit = func(n string) {
		if seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
		if e, ok := m.Plugins[n]; ok {
			for _, c := range e.Chain {
				if cn, _, ok := m.ResolvePlugin(c); ok {
					visit(cn)
				}
			}
		}
	}
	for _, n := range names {
		visit(n)
	}
	return out
}

// effectiveTargets intersects the user's --target selection with the plugin's
// declared target list. An empty plugin target means "any platform", so the
// CLI selection passes through unchanged.
func effectiveTargets(cli, pluginTarget []string) []string {
	if len(pluginTarget) == 0 {
		return cli
	}
	allow := map[string]bool{}
	for _, t := range pluginTarget {
		if p, ok := agent.ResolvePlatform(t); ok {
			allow[p.Name] = true
		}
	}
	var out []string
	for _, t := range cli {
		if allow[t] {
			out = append(out, t)
		}
	}
	return out
}

func loadOrNewRecord(lock *config.Lock, scope string, cliTargets []string) config.InstallRecord {
	installPath := scopeRoot(scope)
	for _, r := range lock.Installs {
		if r.Path == installPath && r.Scope == scope {
			return r
		}
	}
	return config.InstallRecord{
		Path:   installPath,
		Scope:  scope,
		Target: agent.FormatTargetList(cliTargets),
	}
}

// ---------------------------------------------------------------------------
// per-plugin install
// ---------------------------------------------------------------------------

// installPlugin materialises one plugin's assets for the given targets and
// returns the version recorded in the lock (the plugin's _meta.meta SemVer,
// falling back to the manifest version).
func installPlugin(m *config.Manifest, cloneDir, name, scope string, targets []string, rec *config.InstallRecord) (string, error) {
	pluginDir := m.PluginDir(cloneDir, name)
	if _, err := os.Stat(pluginDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("plugin %q not found at %s", name, pluginDir)
		}
		return "", err
	}
	fmt.Printf("→ installing plugin %s (targets=%s)\n", name, strings.Join(targets, ","))

	if err := installPluginAgents(pluginDir, scope, targets); err != nil {
		return "", err
	}
	if err := installPluginSkills(pluginDir, scope, targets); err != nil {
		return "", err
	}
	if err := installPluginDocs(pluginDir, scope, targets); err != nil {
		return "", err
	}
	if agent.HasClaude(targets) {
		// rules + hooks are Claude-only per PRD.
		if err := copySubtreeIfExists(filepath.Join(pluginDir, "rules"), rulesDir(scope)); err != nil {
			return "", err
		}
		if err := copySubtreeIfExists(filepath.Join(pluginDir, "hooks"), hooksDir(scope)); err != nil {
			return "", err
		}
	}
	for _, t := range targets {
		p, ok := agent.ResolvePlatform(t)
		if !ok {
			continue
		}
		if err := installSidecar(pluginDir, p, scope); err != nil {
			return "", err
		}
	}

	// _init scaffold runs once per plugin, project scope only. Gentle copy
	// (overwrite=false) so it never clobbers existing project files.
	if scope == "project" && !containsStr(rec.InitDone, name) {
		applied, err := applyInitTree(pluginDir, scopeRoot(scope), false)
		if err != nil {
			return "", err
		}
		if applied {
			rec.InitDone = append(rec.InitDone, name)
		}
	}

	ver := config.PluginMetaVersion(pluginDir)
	if ver == "" {
		ver, _ = m.PluginVersion(name)
	}
	return ver, nil
}

// installPluginAgents renders every agents/*.md to each target's native agent
// path (claude/opencode/kilo = .md, codex = .toml). Real files, no symlinks.
func installPluginAgents(pluginDir, scope string, targets []string) error {
	agentsSrc := filepath.Join(pluginDir, "agents")
	entries, err := os.ReadDir(agentsSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		srcPath := filepath.Join(agentsSrc, e.Name())
		src, err := agent.LoadFile(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s (load failed: %v)\n", srcPath, err)
			continue
		}
		if err := agent.Validate(src); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping agent %q (%v)\n", src.Name, err)
			continue
		}
		picked := agent.EffectiveTargets(src, targets)
		for _, tname := range picked {
			p, ok := agent.ResolvePlatform(tname)
			if !ok {
				continue
			}
			data, warns, err := p.Render(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s for %s (render failed: %v)\n", src.Name, tname, err)
				continue
			}
			for _, w := range warns {
				fmt.Fprintf(os.Stderr, "warning [%s -> %s]: %s\n", src.Name, tname, w)
			}
			dst := p.Path(scope, src.Name)
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}
			fmt.Printf("  agent %s → %s\n", src.Name, dst)
		}
	}
	return nil
}

// installPluginSkills copies each skill into Claude's canonical skills dir and,
// when any non-Claude target is selected, links the shared .agents/skills
// bridge so Codex/Kilo/opencode can reach them.
func installPluginSkills(pluginDir, scope string, targets []string) error {
	skillsSrc := filepath.Join(pluginDir, "skills")
	entries, err := os.ReadDir(skillsSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	any := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := installTree(filepath.Join(skillsSrc, e.Name()), filepath.Join(skillsDir(scope), e.Name())); err != nil {
			return err
		}
		any = true
	}
	if any && agent.HasNonClaude(targets) {
		if err := linkSkillsBridge(scope); err != nil {
			return err
		}
	}
	return nil
}

// installPluginDocs copies the plugin's docs tree into Claude's canonical docs
// dir and links a per-platform docs directory for every non-Claude target.
func installPluginDocs(pluginDir, scope string, targets []string) error {
	docsSrc := filepath.Join(pluginDir, "docs")
	if _, err := os.Stat(docsSrc); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := installTree(docsSrc, docsDir(scope)); err != nil {
		return err
	}
	if agent.HasNonClaude(targets) {
		if err := linkDocsBridge(scope, targets); err != nil {
			return err
		}
	}
	return nil
}

// installSidecar copies a plugin's `.<platform>/` folder into the platform's
// config root for the given scope. settings.json (Claude) and
// config.project.toml (Codex) are deep-merged; everything else is copied.
func installSidecar(pluginDir string, p agent.Platform, scope string) error {
	base := filepath.Base(p.ConfigRoot("project")) // ".claude", ".codex", ...
	srcRoot := filepath.Join(pluginDir, base)
	if _, err := os.Stat(srcRoot); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dstRoot := p.ConfigRoot(scope)
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(srcRoot, path)
		if p.Name == "claude" && rel == "settings.json" {
			fmt.Printf("  merge %s → %s\n", path, filepath.Join(dstRoot, rel))
			return mergeJSONFile(path, filepath.Join(dstRoot, "settings.json"))
		}
		if p.Name == "codex" && rel == "config.project.toml" {
			dst := filepath.Join(dstRoot, "config.toml")
			fmt.Printf("  merge %s → %s\n", path, dst)
			return mergeTOMLFile(path, dst)
		}
		dst := filepath.Join(dstRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		if err := copyFile(path, dst); err != nil {
			return err
		}
		fmt.Printf("  sidecar %s → %s\n", path, dst)
		return nil
	})
}

func copySubtreeIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return installTree(src, dst)
}

// ---------------------------------------------------------------------------
// link bridges
// ---------------------------------------------------------------------------

// linkSkillsBridge links the shared .agents/skills directory to Claude's
// installed skills so non-Claude platforms can reach them.
func linkSkillsBridge(scope string) error {
	target := skillsDir(scope)
	link := filepath.Join(agent.SharedAgentsRoot(scope), "skills")
	if err := replaceTreeBridge(target, link); err != nil {
		return fmt.Errorf("skills bridge: %w", err)
	}
	fmt.Printf("  linked %s → %s\n", link, target)
	return nil
}

// linkDocsBridge links each non-Claude target's docs directory to Claude's
// canonical docs tree.
func linkDocsBridge(scope string, targets []string) error {
	target := docsDir(scope)
	for _, t := range targets {
		if t == "claude" {
			continue
		}
		p, ok := agent.ResolvePlatform(t)
		if !ok {
			continue
		}
		link := p.DocsDir(scope)
		if err := replaceTreeBridge(target, link); err != nil {
			return fmt.Errorf("docs bridge for %s: %w", t, err)
		}
		fmt.Printf("  linked %s → %s\n", link, target)
	}
	return nil
}

// ---------------------------------------------------------------------------
// shared helpers (reused by update/list/rm)
// ---------------------------------------------------------------------------

// resolvePackagesSrc returns the root holding plugins/ and docs/ to install
// from. Priority: --src flag > .aicorc clone_dir/packages > ./packages.
func resolvePackagesSrc() (string, error) {
	if flagSrc != "" {
		return flagSrc, nil
	}
	rc, err := config.LoadRc()
	if err != nil {
		return "", err
	}
	if rc != nil && rc.CloneDir != "" {
		return config.PackagesDir(rc.CloneDir), nil
	}
	if _, err := os.Stat("packages"); err == nil {
		fmt.Fprintln(os.Stderr, "warning: .aicorc not found — falling back to ./packages. Run `aico init` first.")
		return "packages", nil
	}
	return "", fmt.Errorf("no package source found — run `aico init` first or pass --src")
}

func scopeFromGlobal(global bool) string {
	if global {
		return "user"
	}
	return "project"
}

func scopeRoot(scope string) string {
	if scope == "user" {
		h, _ := os.UserHomeDir()
		return h
	}
	cwd, _ := os.Getwd()
	return cwd
}

func repoRoot(src string) string {
	dir, err := filepath.Abs(src)
	if err != nil {
		return src
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return src
		}
		dir = parent
	}
}

func gitCommit(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func containsStr(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// nowStamp returns the current local date/time for recording install/update
// time in the lock.
func nowStamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ---------- copy primitives ----------

func installTree(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}
	return nil
}

// ---------- path helpers for Claude-canonical content ----------

func skillsDir(scope string) string { return filepath.Join(claudeRoot(scope), "skills") }
func docsDir(scope string) string   { return filepath.Join(claudeRoot(scope), "docs") }
func rulesDir(scope string) string  { return filepath.Join(claudeRoot(scope), "rules") }
func hooksDir(scope string) string  { return filepath.Join(claudeRoot(scope), "hooks") }

func claudeRoot(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude")
	}
	return ".claude"
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "~"
}
