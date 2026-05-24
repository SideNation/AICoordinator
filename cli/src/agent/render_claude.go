package agent

import (
	"fmt"
	"strings"
)

// Order of frontmatter keys in Claude agent files.
var claudeFieldOrder = []string{
	"name", "description", "model", "tools", "color",
	"effort",
	"permissionMode", "disallowedTools", "maxTurns", "skills",
	"mcpServers", "hooks", "memory", "background",
	"isolation", "initialPrompt",
}

func renderClaude(src *Source) ([]byte, []string, error) {
	var warnings []string
	f := newFrontmatter()
	f.set("name", src.Name)
	f.set("description", src.Description)

	if src.Model != "" {
		f.set("model", modelForPlatform(src.Model, "claude"))
	}
	if src.Effort != "" {
		f.set("effort", strings.ToLower(src.Effort))
	}
	if len(src.Tools) > 0 {
		f.set("tools", strings.Join(src.Tools, ", "))
	}
	if src.Color != "" {
		f.set("color", src.Color)
	}

	for _, k := range sortedKeys(src.Claude) {
		v := src.Claude[k]
		switch k {
		case "permissionMode", "memory", "isolation", "initialPrompt":
			f.set(k, fmt.Sprintf("%v", v))
		case "disallowedTools":
			f.set(k, csvFromAny(v))
		case "tools":
			f.set("tools", csvFromAny(v))
		case "maxTurns", "background", "skills", "mcpServers", "hooks", "color":
			f.set(k, v)
		case "model":
			f.set("model", fmt.Sprintf("%v", v))
		case "effort":
			f.set("effort", fmt.Sprintf("%v", v))
		default:
			warnings = append(warnings, fmt.Sprintf("unknown claude override %q, ignored", k))
		}
	}

	return renderMarkdownDoc(orderedView(f, claudeFieldOrder), src.Body), warnings, nil
}

// orderedView returns a new frontmatter where known keys come first in the
// requested order; any extra keys are appended in their original order.
func orderedView(in *orderedFrontmatter, order []string) *orderedFrontmatter {
	out := newFrontmatter()
	for _, key := range order {
		if in.has(key) {
			out.set(key, in.pairs[in.keys[key]].Val)
		}
	}
	for _, p := range in.pairs {
		if !out.has(p.Key) {
			out.set(p.Key, p.Val)
		}
	}
	return out
}

func csvFromAny(v any) string {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ", ")
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, toString(e))
		}
		return strings.Join(parts, ", ")
	case string:
		return x
	default:
		return toString(v)
	}
}
