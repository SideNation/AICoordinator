package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Renderer converts a parsed Source into the byte payload written to a
// platform's install path. Renderers may emit warnings for non-fatal mapping
// issues (unknown fields, dropped colors, etc.).
type Renderer func(src *Source) ([]byte, []string, error)

// Platform describes one supported target (claude, codex, kilo, opencode).
// New platforms can be added by appending an entry to platformRegistry — the
// install/update flows iterate over the registry instead of hard-coding names.
type Platform struct {
	Name      string   // canonical name, used in .lock
	Aliases   []string // short forms accepted on --target
	Extension string   // file extension including dot (".md" / ".toml")
	Render    Renderer
	// InstallDir returns the directory where compiled agent files live for
	// the given scope. scope is "project" or "user".
	InstallDir func(scope string) string
}

// FileName returns the agent file name (e.g. "code-reviewer.md") for this
// platform.
func (p Platform) FileName(agentName string) string {
	return agentName + p.Extension
}

// Path returns the absolute (scope-resolved) destination path for an agent
// file on this platform.
func (p Platform) Path(scope, agentName string) string {
	return filepath.Join(p.InstallDir(scope), p.FileName(agentName))
}

var platformRegistry = []Platform{
	{
		Name:       "claude",
		Aliases:    []string{"cl"},
		Extension:  ".md",
		Render:     renderClaude,
		InstallDir: claudeInstallDir,
	},
	{
		Name:       "codex",
		Aliases:    []string{"co"},
		Extension:  ".toml",
		Render:     renderCodex,
		InstallDir: codexInstallDir,
	},
	{
		Name:       "kilo",
		Aliases:    []string{"ki"},
		Extension:  ".md",
		Render:     renderKilo,
		InstallDir: kiloInstallDir,
	},
	{
		Name:       "opencode",
		Aliases:    []string{"op"},
		Extension:  ".md",
		Render:     renderOpencode,
		InstallDir: opencodeInstallDir,
	},
}

// ResolvePlatform accepts a canonical name, alias, or "all" and returns the
// matching Platform. "all" returns (Platform{}, false) — callers should detect
// that case before calling.
func ResolvePlatform(s string) (Platform, bool) {
	key := strings.ToLower(strings.TrimSpace(s))
	for _, p := range platformRegistry {
		if key == p.Name {
			return p, true
		}
		for _, a := range p.Aliases {
			if key == a {
				return p, true
			}
		}
	}
	return Platform{}, false
}

// Platforms returns the registry slice. Order is stable (registration order).
func Platforms() []Platform {
	out := make([]Platform, len(platformRegistry))
	copy(out, platformRegistry)
	return out
}

// AllPlatformNames returns the canonical name list in registry order.
func AllPlatformNames() []string {
	out := make([]string, len(platformRegistry))
	for i, p := range platformRegistry {
		out[i] = p.Name
	}
	return out
}

// ParseTargetSpec turns a --target value (comma-separated list, possibly
// containing aliases or "all") into a canonical, deduplicated, alphabetically
// sorted platform name list. Unknown tokens cause an error.
func ParseTargetSpec(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || strings.EqualFold(spec, "all") {
		return AllPlatformNames(), nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range strings.Split(spec, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		if strings.EqualFold(tok, "all") {
			return AllPlatformNames(), nil
		}
		p, ok := ResolvePlatform(tok)
		if !ok {
			return nil, &UnknownTargetError{Token: tok}
		}
		if _, dup := seen[p.Name]; dup {
			continue
		}
		seen[p.Name] = struct{}{}
		out = append(out, p.Name)
	}
	if len(out) == 0 {
		return nil, &UnknownTargetError{Token: spec}
	}
	// keep registry order for stability rather than alpha — registry already
	// has a meaningful order, but we sort to match canonicalisation expected
	// in the plan ("anstable order" — alphabetical is the simplest stable
	// choice and survives registry reordering).
	sort.Strings(out)
	return out, nil
}

// UnknownTargetError is returned by ParseTargetSpec when an unrecognised
// token is provided.
type UnknownTargetError struct{ Token string }

func (e *UnknownTargetError) Error() string {
	return "unknown --target value: " + e.Token + " (allowed: all, claude/cl, codex/co, kilo/ki, opencode/op)"
}

// FormatTargetList joins canonical names with commas for storage in .lock.
func FormatTargetList(names []string) string {
	return strings.Join(names, ",")
}

// ParseLockTarget reads the .lock `target` field and returns the canonical
// platform list. Legacy values are mapped:
//   - "" or "all" → claude,opencode (the original pair)
//   - "claude"/"opencode" → that single platform
//   - csv of canonical names/aliases → parsed normally
func ParseLockTarget(s string) []string {
	s = strings.TrimSpace(s)
	switch s {
	case "", "all":
		return []string{"claude", "opencode"}
	}
	parts := strings.Split(s, ",")
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range parts {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		p, ok := ResolvePlatform(tok)
		if !ok {
			continue
		}
		if _, dup := seen[p.Name]; dup {
			continue
		}
		seen[p.Name] = struct{}{}
		out = append(out, p.Name)
	}
	if len(out) == 0 {
		return []string{"claude", "opencode"}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Install directories
// ---------------------------------------------------------------------------

func claudeInstallDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".claude", "agents")
	}
	return filepath.Join(".claude", "agents")
}

func codexInstallDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".codex", "agents")
	}
	return filepath.Join(".codex", "agents")
}

func kiloInstallDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".config", "kilo", "agents")
	}
	return filepath.Join(".kilo", "agents")
}

func opencodeInstallDir(scope string) string {
	if scope == "user" {
		return filepath.Join(homeDir(), ".config", "opencode", "agents")
	}
	return filepath.Join(".opencode", "agents")
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "~"
}
