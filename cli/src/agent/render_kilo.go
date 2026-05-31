package agent

import (
	"fmt"
	"strings"
)

// Kilo's markdown agent schema mirrors opencode closely but accepts extra
// pass-through keys via the kilo: override map.
var kiloFieldOrder = []string{
	"description", "mode", "model", "tools", "color",
	"reasoningEffort", "effort",
	"temperature", "top_p", "steps", "permission", "disable", "hidden",
}

func renderKilo(src *Source) ([]byte, []string, error) {
	var warnings []string
	f := newFrontmatter()
	f.set("description", src.Description)

	if src.Model != "" {
		f.set("model", modelForPlatform(src.Model, "kilo"))
	}
	if src.Effort != "" {
		// Kilo documents `reasoningEffort` as a pass-through provider option
		// (matches OpenAI's naming). Use that so opencode-style `effort` does
		// not clash with provider conventions.
		f.set("reasoningEffort", effortForPlatform(src.Effort, "kilo"))
	}
	if len(src.Tools) > 0 {
		f.set("tools", strings.Join(src.Tools, ", "))
	}
	if src.Color != "" {
		f.set("color", src.Color)
	}

	for _, k := range sortedKeys(src.Kilo) {
		v := src.Kilo[k]
		switch k {
		case "mode", "model", "color", "reasoningEffort":
			f.set(k, fmt.Sprintf("%v", v))
		case "temperature", "top_p", "steps", "disable", "hidden", "permission":
			f.set(k, v)
		case "tools":
			f.set("tools", csvFromAny(v))
		default:
			// Per the plan, unknown fields are pass-through — but record a
			// warning so authors notice typos.
			warnings = append(warnings, fmt.Sprintf("unknown kilo override %q, passing through", k))
			f.set(k, v)
		}
	}

	return renderMarkdownDoc(orderedView(f, kiloFieldOrder), src.Body), warnings, nil
}
