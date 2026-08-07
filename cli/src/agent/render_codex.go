package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Codex agent files are TOML. The schema is intentionally minimal — common
// fields at the top level, then any platform overrides as additional keys.
// The agent body is rendered as a triple-quoted `developer_instructions`
// literal (Codex's required field for the agent's core instructions) so
// authors don't have to escape newlines.
var codexFieldOrder = []string{
	"name", "description", "model", "model_reasoning_effort",
	"sandbox_mode", "approval_policy",
}

func renderCodex(src *Source) ([]byte, []string, error) {
	var warnings []string

	doc := newTomlDoc()
	doc.set("name", src.Name)
	doc.set("description", src.Description)
	if src.Model != "" {
		doc.set("model", modelForPlatform(src.Model, "codex"))
	}
	if src.Effort != "" {
		doc.set("model_reasoning_effort", effortForPlatform(src.Effort, "codex"))
	}
	// Codex's agent-role TOML has no tool-allowlist field: its `tools` key
	// is reserved for web_search/experimental settings (ToolsToml), not a
	// permission list. Writing src.Tools there produces a table Codex can't
	// deserialize (fields land positionally and collide with the untagged
	// web_search variant), so it's intentionally never emitted here.
	// Codex also has no `color` field (checked against config_toml.rs and
	// config.schema.json — no match), so src.Color is not emitted either.

	for _, k := range sortedKeys(src.Codex) {
		v := src.Codex[k]
		switch v.(type) {
		case map[string]any, map[any]any:
			warnings = append(warnings, fmt.Sprintf("codex override %q is a table; nested tables are not yet supported, ignored", k))
			continue
		}
		doc.set(k, v)
	}

	var b strings.Builder
	for _, k := range codexFieldOrder {
		if doc.has(k) {
			b.WriteString(tomlPair(k, doc.get(k)))
		}
	}
	// trailing keys (registry extras / pass-through fields)
	for _, p := range doc.pairs {
		if !containsString(codexFieldOrder, p.Key) {
			b.WriteString(tomlPair(p.Key, p.Val))
		}
	}
	if src.Body != "" {
		b.WriteString("\n")
		b.WriteString("developer_instructions = ")
		b.WriteString(tomlMultiline(src.Body))
		b.WriteString("\n")
	}
	return []byte(b.String()), warnings, nil
}

func containsString(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Minimal ordered TOML emitter
// ---------------------------------------------------------------------------

type tomlDoc struct {
	pairs []kv
	keys  map[string]int
}

func newTomlDoc() *tomlDoc { return &tomlDoc{keys: map[string]int{}} }

func (d *tomlDoc) set(key string, val any) {
	if i, ok := d.keys[key]; ok {
		d.pairs[i].Val = val
		return
	}
	d.keys[key] = len(d.pairs)
	d.pairs = append(d.pairs, kv{Key: key, Val: val})
}

func (d *tomlDoc) has(key string) bool { _, ok := d.keys[key]; return ok }
func (d *tomlDoc) get(key string) any  { return d.pairs[d.keys[key]].Val }

func tomlPair(key string, val any) string {
	return fmt.Sprintf("%s = %s\n", key, tomlValue(val))
}

func tomlValue(v any) string {
	switch x := v.(type) {
	case nil:
		return `""`
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
	case string:
		return tomlString(x)
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = tomlString(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = tomlValue(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s = %s", k, tomlValue(x[k])))
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	default:
		return tomlString(toString(v))
	}
}

func tomlString(s string) string {
	if strings.ContainsAny(s, "\n\r") {
		return tomlMultiline(s)
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// tomlMultiline emits a triple-quoted basic string. We escape backslashes and
// closing triple-quotes to keep the string syntactically valid.
func tomlMultiline(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"""`, `\"\"\"`)
	// Leading newline after the opening delimiter is consumed by TOML, so
	// add one explicitly to keep the original text intact.
	return "\"\"\"\n" + escaped + "\n\"\"\""
}
