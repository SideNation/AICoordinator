// Package agent parses single-source agent markdown files and renders them
// into platform-specific outputs (Claude/opencode markdown, Codex TOML, Kilo
// markdown). The package owns the platform registry so install/update flows
// stay platform-agnostic.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source is the parsed representation of a single-source agent markdown file.
// Platform-specific override maps are kept untyped so renderers can interpret
// them according to each platform's schema.
type Source struct {
	Name        string
	Description string
	Model       string
	Effort      string
	Tools       []string
	Color       string
	UseOnly     string
	Body        string

	Claude   map[string]any
	Opencode map[string]any
	Codex    map[string]any
	Kilo     map[string]any

	// SourcePath is the absolute path the source was loaded from. Populated
	// by LoadFile; empty when Parse is used directly.
	SourcePath string
}

// Parse reads frontmatter + body from a markdown source. The frontmatter is
// expected to be a YAML mapping delimited by `---` lines, matching the
// `python` agent_lib.py reference implementation.
func Parse(content string) (*Source, error) {
	// strip UTF-8 BOM (U+FEFF)
	content = strings.TrimPrefix(content, "\uFEFF")
	stripped := strings.TrimLeft(content, " \t\r\n")
	if !strings.HasPrefix(stripped, "---") {
		return nil, fmt.Errorf("file must begin with --- frontmatter delimiter")
	}

	rest := stripped[3:]
	// find closing delimiter on its own line
	idx := indexClosing(rest)
	if idx < 0 {
		return nil, fmt.Errorf("missing closing --- frontmatter delimiter")
	}

	raw := strings.TrimSpace(rest[:idx])
	body := strings.TrimLeft(rest[idx+len("\n---"):], "\n")
	body = strings.TrimSpace(body)

	var data map[string]any
	if err := yaml.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if data == nil {
		data = map[string]any{}
	}

	src := &Source{
		Name:        toString(data["name"]),
		Description: toString(data["description"]),
		Model:       toString(data["model"]),
		Effort:      toString(data["effort"]),
		Tools:       toStringSlice(data["tools"]),
		Color:       toString(data["color"]),
		UseOnly:     strings.TrimSpace(toString(data["useonly"])),
		Body:        body,
	}
	src.Claude = toMap(data["claude"])
	src.Opencode = toMap(data["opencode"])
	src.Codex = toMap(data["codex"])
	src.Kilo = toMap(data["kilo"])
	return src, nil
}

// LoadFile reads a source file from disk. The agent name defaults to the file
// stem when the frontmatter omits `name`.
func LoadFile(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src, err := Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if src.Name == "" {
		base := filepath.Base(path)
		src.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	abs, absErr := filepath.Abs(path)
	if absErr == nil {
		src.SourcePath = abs
	} else {
		src.SourcePath = path
	}
	return src, nil
}

// indexClosing finds the position of "\n---" that terminates the frontmatter.
// The closing delimiter must be the start of a line and either be followed by
// EOF, a newline, or whitespace.
func indexClosing(s string) int {
	from := 0
	for from < len(s) {
		idx := strings.Index(s[from:], "\n---")
		if idx < 0 {
			return -1
		}
		pos := from + idx
		end := pos + len("\n---")
		if end == len(s) {
			return pos
		}
		next := s[end]
		if next == '\n' || next == '\r' || next == ' ' || next == '\t' {
			return pos
		}
		from = end
	}
	return -1
}

func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", x), "0"), ".")
	default:
		return fmt.Sprintf("%v", x)
	}
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			s := strings.TrimSpace(toString(e))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := v.(string); ok {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	return nil
}

func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	// gopkg.in/yaml.v3 emits map[interface{}]interface{} when keys are not
	// strings, but for our schema keys are always strings.
	if m, ok := v.(map[any]any); ok {
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[toString(k)] = val
		}
		return out
	}
	return map[string]any{}
}
