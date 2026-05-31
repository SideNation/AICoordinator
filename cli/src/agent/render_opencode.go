package agent

import (
	"fmt"
	"strings"
)

var opencodeFieldOrder = []string{
	"description", "mode", "model", "tools", "color",
	"effort",
	"temperature", "top_p", "steps", "permission", "disable", "hidden",
}

func renderOpencode(src *Source) ([]byte, []string, error) {
	var warnings []string
	f := newFrontmatter()
	f.set("description", src.Description)

	if src.Model != "" {
		f.set("model", modelForPlatform(src.Model, "opencode"))
	}
	if src.Effort != "" {
		f.set("effort", effortForPlatform(src.Effort, "opencode"))
	}
	if len(src.Tools) > 0 {
		f.set("tools", toolsMap(src.Tools))
	}
	if src.Color != "" {
		if strings.HasPrefix(src.Color, "#") {
			f.set("color", src.Color)
		} else if hx, ok := colorHex(src.Color); ok {
			f.set("color", hx)
		} else {
			warnings = append(warnings,
				fmt.Sprintf("color name %q has no hex mapping, dropping", src.Color))
		}
	}

	for _, k := range sortedKeys(src.Opencode) {
		v := src.Opencode[k]
		switch k {
		case "mode":
			f.set("mode", fmt.Sprintf("%v", v))
		case "temperature", "top_p", "steps", "disable", "hidden", "permission":
			f.set(k, v)
		case "model":
			f.set("model", fmt.Sprintf("%v", v))
		case "color":
			f.set("color", fmt.Sprintf("%v", v))
		case "effort":
			f.set("effort", fmt.Sprintf("%v", v))
		case "tools":
			f.set("tools", toolsMapFromAny(v))
		default:
			warnings = append(warnings, fmt.Sprintf("unknown opencode override %q, ignored", k))
		}
	}

	return renderMarkdownDoc(orderedView(f, opencodeFieldOrder), src.Body), warnings, nil
}

func toolsMap(tools []string) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, t := range tools {
		out[strings.ToLower(t)] = true
	}
	return out
}

func toolsMapFromAny(v any) map[string]bool {
	switch x := v.(type) {
	case []string:
		return toolsMap(x)
	case []any:
		out := map[string]bool{}
		for _, e := range x {
			out[strings.ToLower(toString(e))] = true
		}
		return out
	case map[string]any:
		out := map[string]bool{}
		for k, val := range x {
			if b, ok := val.(bool); ok {
				out[strings.ToLower(k)] = b
			}
		}
		return out
	case map[string]bool:
		return x
	}
	return map[string]bool{}
}
