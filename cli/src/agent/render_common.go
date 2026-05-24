package agent

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Model + colour mapping tables
// ---------------------------------------------------------------------------

// claudeModelMap maps the common tier to Claude's named model alias.
var claudeModelMap = map[string]string{
	"high":   "opus",
	"medium": "sonnet",
	"low":    "haiku",
}

// codexModelMap maps the common tier to Codex's named model id. Keep in sync
// with whatever the local codex CLI accepts.
var codexModelMap = map[string]string{
	"high":   "gpt-5.5",
	"medium": "gpt-5.3-codex",
	"low":    "gpt-5.4-mini",
}

// opencodeModelMap maps the common tier to the opencode provider/model id.
var opencodeModelMap = map[string]string{
	"high":   "anthropic/claude-opus-4-7",
	"medium": "anthropic/claude-sonnet-4-6",
	"low":    "anthropic/claude-haiku-4-5-20251001",
}

// kiloModelMap reuses the opencode provider ids — Kilo accepts
// provider/model-id strings, same as opencode.
var kiloModelMap = map[string]string{
	"high":   "anthropic/claude-opus-4-7",
	"medium": "anthropic/claude-sonnet-4-6",
	"low":    "anthropic/claude-haiku-4-5-20251001",
}

var colorNameToHex = map[string]string{
	"red":    "#ef4444",
	"orange": "#f97316",
	"yellow": "#eab308",
	"green":  "#22c55e",
	"blue":   "#3b82f6",
	"purple": "#a855f7",
	"pink":   "#ec4899",
	"gray":   "#6b7280",
}

func colorHex(name string) (string, bool) {
	hx, ok := colorNameToHex[strings.ToLower(name)]
	return hx, ok
}

// modelForPlatform translates the common tier into a platform-specific model
// id. Returns "" when the tier is unknown (Validate should have caught that).
func modelForPlatform(tier, platform string) string {
	tier = strings.ToLower(strings.TrimSpace(tier))
	if tier == "" {
		return ""
	}
	switch platform {
	case "claude":
		return claudeModelMap[tier]
	case "codex":
		return codexModelMap[tier]
	case "opencode":
		return opencodeModelMap[tier]
	case "kilo":
		return kiloModelMap[tier]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Tiny ordered-mapping helpers used by the markdown renderers.
// We do not use yaml.v3 marshalling directly because we want deterministic
// key order matching the platform's documented convention.
// ---------------------------------------------------------------------------

type kv struct {
	Key string
	Val any
}

// orderedFrontmatter holds the rendered key/value pairs and writes them out
// as a markdown frontmatter block.
type orderedFrontmatter struct {
	pairs []kv
	keys  map[string]int // key -> index into pairs (for upserts)
}

func newFrontmatter() *orderedFrontmatter {
	return &orderedFrontmatter{keys: map[string]int{}}
}

func (f *orderedFrontmatter) set(key string, val any) {
	if i, ok := f.keys[key]; ok {
		f.pairs[i].Val = val
		return
	}
	f.keys[key] = len(f.pairs)
	f.pairs = append(f.pairs, kv{Key: key, Val: val})
}

func (f *orderedFrontmatter) has(key string) bool {
	_, ok := f.keys[key]
	return ok
}

// String renders the frontmatter block (no surrounding --- delimiters).
func (f *orderedFrontmatter) String() string {
	var b strings.Builder
	for _, p := range f.pairs {
		emitYAML(&b, p.Key, p.Val, 0)
	}
	return b.String()
}

// renderMarkdownDoc combines a frontmatter block with the agent body.
func renderMarkdownDoc(front *orderedFrontmatter, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(front.String())
	b.WriteString("---\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// emitYAML writes a single key/value pair in our restricted YAML subset.
func emitYAML(b *strings.Builder, key string, val any, indent int) {
	pad := strings.Repeat(" ", indent)
	switch v := val.(type) {
	case nil:
		fmt.Fprintf(b, "%s%s: null\n", pad, key)
	case bool:
		fmt.Fprintf(b, "%s%s: %t\n", pad, key, v)
	case int, int64, float64:
		fmt.Fprintf(b, "%s%s: %v\n", pad, key, v)
	case string:
		fmt.Fprintf(b, "%s%s: %s\n", pad, key, yamlScalar(v))
	case []string:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: []\n", pad, key)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", pad, key)
		for _, item := range v {
			fmt.Fprintf(b, "%s  - %s\n", pad, yamlScalar(item))
		}
	case []any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: []\n", pad, key)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", pad, key)
		for _, item := range v {
			switch it := item.(type) {
			case map[string]any:
				fmt.Fprintf(b, "%s  -\n", pad)
				for _, sk := range sortedKeys(it) {
					emitYAML(b, sk, it[sk], indent+4)
				}
			default:
				fmt.Fprintf(b, "%s  - %s\n", pad, yamlScalar(toString(item)))
			}
		}
	case map[string]bool:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: {}\n", pad, key)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", pad, key)
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(b, "%s  %s: %t\n", pad, k, v[k])
		}
	case map[string]any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: {}\n", pad, key)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", pad, key)
		for _, k := range sortedKeys(v) {
			emitYAML(b, k, v[k], indent+2)
		}
	default:
		fmt.Fprintf(b, "%s%s: %s\n", pad, key, yamlScalar(toString(v)))
	}
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// yamlScalar quotes a string when it would otherwise be ambiguous (contains
// `:`, leading whitespace, starts with YAML-significant punctuation, looks
// like a bool/number/null literal, or is empty).
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	if needsYAMLQuoting(s) {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

func needsYAMLQuoting(s string) bool {
	if s == "" {
		return true
	}
	low := strings.ToLower(s)
	switch low {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	first := s[0]
	switch first {
	case ' ', '\t', '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' || c == '\r' {
			return true
		}
		// `: ` inside a string is YAML-significant (looks like a mapping).
		if c == ':' && i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\t') {
			return true
		}
		if c == '#' && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
			return true
		}
	}
	// numeric-looking strings need quoting so YAML doesn't reinterpret them
	if isNumericLike(s) {
		return true
	}
	return false
}

func isNumericLike(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' || s[0] == '+' {
		i = 1
	}
	if i == len(s) {
		return false
	}
	sawDigit := false
	sawDot := false
	for ; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			sawDigit = true
		case c == '.' && !sawDot:
			sawDot = true
		default:
			return false
		}
	}
	return sawDigit
}
