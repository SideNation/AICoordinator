package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm [agent|skill|doc|rule] <pattern>...",
	Short: "Remove installed agents, skills, docs, or rules (supports globs and auto-discovery)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRm(args)
	},
}

var flagRmGlobal bool

func init() {
	rmCmd.Flags().BoolVarP(&flagRmGlobal, "global", "g", false, "remove from user-scope (home) install instead of current project")
	rootCmd.AddCommand(rmCmd)
}

// runRm parses args, locates the matching install record, deletes matched
// items from disk, and saves the updated .lock.
func runRm(args []string) error {
	kind, patterns := parseRmArgs(args)
	if len(patterns) == 0 {
		return fmt.Errorf("at least one name or pattern is required")
	}

	scope := scopeFromGlobal(flagRmGlobal)
	installPath := scopeRoot(scope)

	lock, err := config.LoadLock()
	if err != nil {
		return err
	}

	idx := findRecord(lock, installPath, scope)
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

	matches := collectMatches(rec, kind, patterns)
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no items match %v\n", patterns)
		return nil
	}

	for _, m := range matches {
		if err := removeItem(rec, m.kind, m.name); err != nil {
			return err
		}
		delete(itemMap(rec, m.kind), m.name)
		fmt.Printf("removed %s %s\n", m.kind, m.name)
	}

	rec.Agents = nilIfEmpty(rec.Agents)
	rec.Skills = nilIfEmpty(rec.Skills)
	rec.Docs = nilIfEmpty(rec.Docs)
	rec.Rules = nilIfEmpty(rec.Rules)

	return config.SaveLock(lock)
}

// parseRmArgs splits args into (kind, patterns). When the first arg is
// exactly "agent"/"skill"/"doc"/"rule", it becomes the kind filter; otherwise
// kind is "" (auto-discovery across all kinds).
func parseRmArgs(args []string) (string, []string) {
	switch args[0] {
	case "agent", "skill", "doc", "rule":
		return args[0], args[1:]
	}
	return "", args
}

func findRecord(lock *config.Lock, path, scope string) int {
	for i, r := range lock.Installs {
		if r.Path == path && r.Scope == scope {
			return i
		}
	}
	return -1
}

type matchedItem struct {
	kind string // "agent" | "skill" | "doc"
	name string
}

// collectMatches walks the maps relevant to `kind` (or all maps when kind is
// "") and returns every entry whose name matches any of the patterns. Results
// are deduplicated and sorted for deterministic output.
func collectMatches(rec *config.InstallRecord, kind string, patterns []string) []matchedItem {
	seen := map[matchedItem]bool{}
	var out []matchedItem

	add := func(k string, m map[string]string) {
		if m == nil {
			return
		}
		for name := range m {
			if !matchesAny(name, patterns) {
				continue
			}
			item := matchedItem{kind: k, name: name}
			if seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}

	if kind == "" || kind == "agent" {
		add("agent", rec.Agents)
	}
	if kind == "" || kind == "skill" {
		add("skill", rec.Skills)
	}
	if kind == "" || kind == "doc" {
		add("doc", rec.Docs)
	}
	if kind == "" || kind == "rule" {
		add("rule", rec.Rules)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].kind != out[j].kind {
			return out[i].kind < out[j].kind
		}
		return out[i].name < out[j].name
	})
	return out
}

// matchesAny reports whether name matches any of the patterns. A pattern with
// no glob metacharacters (* ? [) must match exactly; otherwise filepath.Match
// is used (returns false on syntax errors).
func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		if isGlob(p) {
			ok, err := filepath.Match(p, name)
			if err == nil && ok {
				return true
			}
		} else if p == name {
			return true
		}
	}
	return false
}

func isGlob(p string) bool {
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*', '?', '[':
			return true
		}
	}
	return false
}

// itemMap returns the InstallRecord map field corresponding to kind, so the
// caller can delete keys from it directly.
func itemMap(rec *config.InstallRecord, kind string) map[string]string {
	switch kind {
	case "agent":
		return rec.Agents
	case "skill":
		return rec.Skills
	case "doc":
		return rec.Docs
	case "rule":
		return rec.Rules
	}
	return nil
}

// removeItem deletes the on-disk artifacts for one tracked item. Missing
// files are tolerated since the user may have removed them already.
func removeItem(rec *config.InstallRecord, kind, name string) error {
	platforms := agent.EnsureClaude(agent.ParseLockTarget(rec.Target))
	switch kind {
	case "agent":
		for _, pn := range platforms {
			p, ok := agent.ResolvePlatform(pn)
			if !ok {
				continue
			}
			if err := removeIfExists(p.Path(rec.Scope, name)); err != nil {
				return err
			}
			if err := removeIfExists(p.LinkedAgentPath(rec.Scope, name)); err != nil {
				return err
			}
		}
	case "skill":
		// Bridge symlink stays — it covers the whole skills directory and
		// other skills may still need it.
		return removeIfExists(filepath.Join(skillsDir(rec.Scope), name))
	case "doc":
		for _, pn := range platforms {
			if pn == "claude" {
				continue
			}
			p, ok := agent.ResolvePlatform(pn)
			if !ok {
				continue
			}
			if err := removeIfExists(filepath.Join(p.DocsDir(rec.Scope), name)); err != nil {
				return err
			}
		}
		return removeIfExists(filepath.Join(docsDir(rec.Scope), name))
	case "rule":
		// Rules are shared via a single directory symlink per non-Claude
		// platform — deleting the Claude file removes it from every
		// platform's view at once.
		return removeIfExists(filepath.Join(rulesDir(rec.Scope), name+".md"))
	}
	return nil
}

func removeIfExists(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
