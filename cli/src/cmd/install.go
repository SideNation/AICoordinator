package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install agents, skills, and (optionally) docs into project or user environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd)
	},
}

var (
	flagGlobal bool   // true = user scope (home), false = project scope (cwd)
	flagTarget string // comma list of platform names/aliases, or "all"
	flagDocs   string // "" = none, "*" = all, "a,b,c" = selected
	flagSrc    string // override packages source root
)

func init() {
	installCmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "install to user home (~/.claude, ~/.codex, ~/.config/...) instead of current project")
	installCmd.Flags().StringVar(&flagTarget, "target", "all", "agent target list: comma-separated values from claude|cl, codex|co, kilo|ki, opencode|op, or all. claude is always installed")
	installCmd.Flags().StringVar(&flagDocs, "docs", "", "install docs: comma-separated names, or empty (= all when flag present, skip when absent)")
	// allow bare `--docs` (no value) to mean "all docs"
	installCmd.Flags().Lookup("docs").NoOptDefVal = "*"
	installCmd.Flags().StringVar(&flagSrc, "src", "", "packages source directory (default: <clone_dir>/packages from .aicorc)")
}

func runInstall(cmd *cobra.Command) error {
	src, err := resolvePackagesSrc()
	if err != nil {
		return err
	}

	manifest, err := loadManifestForSrc(src)
	if err != nil {
		return err
	}

	targets, err := agent.ParseTargetSpec(flagTarget)
	if err != nil {
		return err
	}
	// Claude is always part of the install — it's the canonical destination
	// every other platform symlinks back to.
	targets = agent.EnsureClaude(targets)

	scope := scopeFromGlobal(flagGlobal)
	docsReq := parseSelectFlag(cmd, "docs", flagDocs, manifestDocsNames(manifest))
	summary, err := doInstall(src, scope, targets, docsReq, manifest)
	if err != nil {
		return err
	}
	return recordInstall(src, scope, targets, summary)
}

// installSummary captures which items were installed and their manifest versions,
// so the .lock can be updated with per-item version tracking.
type installSummary struct {
	agents map[string]string // name -> version
	skills map[string]string
	docs   map[string]string
	rules  map[string]string
}

// loadManifestForSrc resolves the clone dir that contains `src` (its parent,
// since src = <clone_dir>/packages) and loads manifest.yaml from there.
func loadManifestForSrc(src string) (*config.Manifest, error) {
	cloneDir := filepath.Dir(src)
	return config.LoadManifest(cloneDir)
}

// scopeFromGlobal maps the -g/--global bool flag to the internal scope string.
func scopeFromGlobal(global bool) string {
	if global {
		return "user"
	}
	return "project"
}

// parseSelectFlag translates a comma-list flag (--docs, --rules) into a list
// of names to install. Returns nil when the flag was not set, meaning "skip".
// When the flag is set with no value (or "*"), returns every name in `all`.
func parseSelectFlag(cmd *cobra.Command, flagName, value string, all []string) []string {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	if value == "*" || strings.TrimSpace(value) == "" {
		out := make([]string, 0, len(all))
		out = append(out, all...)
		return out
	}
	var out []string
	for _, p := range strings.Split(value, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func manifestDocsNames(m *config.Manifest) []string {
	out := make([]string, 0, len(m.Docs))
	for n := range m.Docs {
		out = append(out, n)
	}
	return out
}

// doInstall writes the manifest contents to disk under Claude's canonical
// directories and creates per-platform symlinks for every non-Claude target
// in `targets`. Skills install only to .claude/skills; for non-Claude
// targets we add a single .agents/skills bridge symlink so Codex/Kilo/
// opencode can reach them.
func doInstall(src, scope string, targets []string, docsReq []string, manifest *config.Manifest) (*installSummary, error) {
	sum := &installSummary{
		agents: map[string]string{},
		skills: map[string]string{},
		docs:   map[string]string{},
		rules:  map[string]string{},
	}

	cloneDir := filepath.Dir(src)
	for name, entry := range manifest.Agents {
		srcPath := manifest.AgentSource(cloneDir, name)
		installed, err := installAgentFromSource(srcPath, scope, targets)
		if err != nil {
			return nil, err
		}
		if installed {
			sum.agents[name] = entry.Version
		}
	}
	if len(sum.agents) > 0 && agent.HasNonClaude(targets) {
		if err := linkAgentsDirForPlatforms(scope, targets); err != nil {
			return nil, err
		}
	}

	for name, entry := range manifest.Skills {
		skillSrc := filepath.Join(src, "skills", name)
		if _, err := os.Stat(skillSrc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skill %q not found in package source, skipping\n", name)
			continue
		}
		if err := installTree(skillSrc, filepath.Join(skillsDir(scope), name)); err != nil {
			return nil, err
		}
		sum.skills[name] = entry.Version
	}

	if len(sum.skills) > 0 && agent.HasNonClaude(targets) {
		if err := linkSkillsBridge(scope); err != nil {
			return nil, err
		}
	}

	for _, name := range docsReq {
		entry, ok := manifest.Docs[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: doc %q not declared in manifest, skipping\n", name)
			continue
		}
		docSrc := filepath.Join(src, "docs", name)
		if _, err := os.Stat(docSrc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: doc %q not found in package source, skipping\n", name)
			continue
		}
		docDst := filepath.Join(docsDir(scope), name)
		if _, err := os.Stat(docDst); err != nil {
			if err := installTree(docSrc, docDst); err != nil {
				return nil, err
			}
		} else {
			fmt.Printf("docs: %s already installed, skipping copy\n", name)
		}
		if err := linkDocForPlatforms(scope, name, targets); err != nil {
			return nil, err
		}
		sum.docs[name] = entry.Version
	}

	for name, entry := range manifest.Rules {
		ruleSrc := filepath.Join(src, "rules", name+".md")
		if _, err := os.Stat(ruleSrc); err != nil {
			fmt.Fprintf(os.Stderr, "warning: rule %q not found in package source, skipping\n", name)
			continue
		}
		ruleDst := filepath.Join(rulesDir(scope), name+".md")
		if err := os.MkdirAll(filepath.Dir(ruleDst), 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(ruleDst), err)
		}
		if err := copyFile(ruleSrc, ruleDst); err != nil {
			return nil, err
		}
		fmt.Printf("installed %s → %s\n", ruleSrc, ruleDst)
		sum.rules[name] = entry.Version
	}
	if len(sum.rules) > 0 && agent.HasNonClaude(targets) {
		if err := linkRulesBridge(scope, targets); err != nil {
			return nil, err
		}
	}
	return sum, nil
}

// installAgentFromSource renders the Claude version of the agent to
// .claude/agents/<name>.md and creates symlinks from every other selected
// platform's agents directory pointing back to the Claude file.
//
// useonly still applies — if the source restricts itself to a single
// non-Claude platform, we render that platform's file directly and skip the
// Claude symlink target. Returns true when at least one file landed on disk.
func installAgentFromSource(srcPath, scope string, targets []string) (bool, error) {
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: agent source %q not found, skipping\n", srcPath)
			return false, nil
		}
		return false, err
	}

	src, err := agent.LoadFile(srcPath)
	if err != nil {
		return false, fmt.Errorf("load %s: %w", srcPath, err)
	}
	if err := agent.Validate(src); err != nil {
		return false, err
	}

	picked := agent.EffectiveTargets(src, targets)
	if len(picked) == 0 {
		fmt.Printf("skipped %s (no enabled targets match useonly=%q)\n", srcPath, src.UseOnly)
		return false, nil
	}

	claudeIncluded := false
	for _, n := range picked {
		if n == "claude" {
			claudeIncluded = true
			break
		}
	}

	wrote := false
	claude, _ := agent.ResolvePlatform("claude")
	if claudeIncluded {
		data, warns, err := claude.Render(src)
		if err != nil {
			return wrote, fmt.Errorf("render %s for claude: %w", srcPath, err)
		}
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "warning [%s -> claude]: %s\n", src.Name, w)
		}
		dst := claude.Path(scope, src.Name)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return wrote, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return wrote, fmt.Errorf("write %s: %w", dst, err)
		}
		fmt.Printf("installed %s → %s\n", srcPath, dst)
		wrote = true
	}

	if !claudeIncluded {
		// Claude was excluded (useonly=<non-claude platform>) — render the
		// platform's native file directly into Claude's agents dir so the
		// directory-level link still exposes it to that platform.
		for _, name := range picked {
			if name == "claude" {
				continue
			}
			p, ok := agent.ResolvePlatform(name)
			if !ok {
				continue
			}
			data, warns, err := p.Render(src)
			if err != nil {
				return wrote, fmt.Errorf("render %s for %s: %w", srcPath, name, err)
			}
			for _, w := range warns {
				fmt.Fprintf(os.Stderr, "warning [%s -> %s]: %s\n", src.Name, name, w)
			}
			dst := claude.Path(scope, src.Name)
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return wrote, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return wrote, fmt.Errorf("write %s: %w", dst, err)
			}
			fmt.Printf("installed %s → %s\n", srcPath, dst)
			wrote = true
		}
	}
	return wrote, nil
}

// linkAgentsDirForPlatforms creates one directory symlink per non-Claude
// target from <platform>/agents/ → .claude/agents/ so all platforms share
// the same agents tree without per-file bookkeeping.
func linkAgentsDirForPlatforms(scope string, targets []string) error {
	target := agentsDir(scope)
	for _, t := range targets {
		if t == "claude" {
			continue
		}
		p, ok := agent.ResolvePlatform(t)
		if !ok {
			continue
		}
		link := p.AgentsDir(scope)
		if err := replaceTreeBridge(target, link); err != nil {
			return fmt.Errorf("agents dir bridge for %s: %w", t, err)
		}
		fmt.Printf("linked %s → %s\n", link, target)
	}
	return nil
}

// linkRulesBridge creates one directory symlink per non-Claude target so the
// whole rules tree is shared from Claude. Rules are platform-agnostic, so a
// directory-level link is enough and avoids per-file bookkeeping.
func linkRulesBridge(scope string, targets []string) error {
	target := rulesDir(scope)
	for _, t := range targets {
		if t == "claude" {
			continue
		}
		p, ok := agent.ResolvePlatform(t)
		if !ok {
			continue
		}
		link := p.RulesDir(scope)
		if err := replaceTreeBridge(target, link); err != nil {
			return fmt.Errorf("rules bridge for %s: %w", t, err)
		}
		fmt.Printf("linked %s → %s\n", link, target)
	}
	return nil
}

// linkDocForPlatforms creates one directory symlink per non-Claude target
// from <platform-root>/docs/<name> back to Claude's doc tree.
func linkDocForPlatforms(scope, name string, targets []string) error {
	target := filepath.Join(docsDir(scope), name)
	for _, t := range targets {
		if t == "claude" {
			continue
		}
		p, ok := agent.ResolvePlatform(t)
		if !ok {
			continue
		}
		link := filepath.Join(p.DocsDir(scope), name)
		if err := replaceLink(target, link, true); err != nil {
			return fmt.Errorf("link doc %s for %s: %w", name, t, err)
		}
		fmt.Printf("linked %s → %s\n", link, target)
	}
	return nil
}

// linkSkillsBridge creates the single shared .agents/skills directory link
// that Codex/Kilo/opencode use to reach Claude's installed skills.
func linkSkillsBridge(scope string) error {
	target := skillsDir(scope)
	link := filepath.Join(agent.SharedAgentsRoot(scope), "skills")
	if err := replaceTreeBridge(target, link); err != nil {
		return fmt.Errorf("skills bridge: %w", err)
	}
	fmt.Printf("linked %s → %s\n", link, target)
	return nil
}

// resolvePackagesSrc returns the root of `agents/ skills/ docs/` to install from.
// Priority: --src flag > .aicorc clone_dir/packages > ./packages (legacy fallback).
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
	// legacy fallback for users who have not run `aico init` yet
	if _, err := os.Stat("packages"); err == nil {
		fmt.Fprintln(os.Stderr, "warning: .aicorc not found — falling back to ./packages. Run `aico init` to set up the package repo.")
		return "packages", nil
	}
	return "", fmt.Errorf("no package source found — run `aico init` first or pass --src")
}

// recordInstall updates ~/.aico/.lock with this install, merging per-item
// version maps with the existing record for the same (path, scope).
func recordInstall(src, scope string, targets []string, sum *installSummary) error {
	lock, err := config.LoadLock()
	if err != nil {
		return err
	}

	installPath := scopeRoot(scope)
	version := gitCommit(repoRoot(src))

	var existing config.InstallRecord
	for _, r := range lock.Installs {
		if r.Path == installPath && r.Scope == scope {
			existing = r
			break
		}
	}

	lock.Upsert(config.InstallRecord{
		Path:    installPath,
		Scope:   scope,
		Target:  agent.FormatTargetList(targets),
		Agents:  mergeVersions(existing.Agents, sum.agents),
		Skills:  mergeVersions(existing.Skills, sum.skills),
		Docs:    mergeVersions(existing.Docs, sum.docs),
		Rules:   mergeVersions(existing.Rules, sum.rules),
		Version: version,
	})
	if err := config.SaveLock(lock); err != nil {
		return fmt.Errorf("save .lock: %w", err)
	}
	return nil
}

// mergeVersions returns a new map with b overlaid on a (b wins on conflict).
// Nil inputs are treated as empty. Returns nil if both are empty to keep the
// YAML output clean.
func mergeVersions(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// scopeRoot returns the anchor directory for a scope record — project uses CWD
// so different projects each get their own .lock entry.
func scopeRoot(scope string) string {
	if scope == "user" {
		h, _ := os.UserHomeDir()
		return h
	}
	cwd, _ := os.Getwd()
	return cwd
}

// repoRoot walks up from src to find a `.git` directory. Returns src when none
// is found (so gitCommit yields "" gracefully).
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
		if err := copyFile(path, target); err != nil {
			return err
		}
		fmt.Printf("installed %s → %s\n", path, target)
		return nil
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

func agentsDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "agents")
	}
	return filepath.Join(".claude", "agents")
}

func skillsDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "skills")
	}
	return filepath.Join(".claude", "skills")
}

func docsDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "docs")
	}
	return filepath.Join(".claude", "docs")
}

func rulesDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "rules")
	}
	return filepath.Join(".claude", "rules")
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "~"
}
