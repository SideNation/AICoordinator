package agent

import (
	"fmt"
	"log"
	"strings"
)

// TransformOpencode converts a Source into an OpencodeOut + body.
func TransformOpencode(src *Source, strict bool) (*OpencodeOut, []string, error) {
	var warnings []string

	out := &OpencodeOut{
		Description: src.Description,
	}

	// Model — alias → provider/model-id
	if src.Model != "" {
		if strings.Contains(src.Model, "/") {
			out.Model = src.Model
		} else {
			if pid, ok := ModelToOpencode(src.Model); ok {
				out.Model = pid
			} else {
				msg := fmt.Sprintf("model alias %q has no opencode provider/model-id mapping, using as-is", src.Model)
				if strict {
					return nil, nil, fmt.Errorf("%s", msg)
				}
				warnings = append(warnings, msg)
				out.Model = src.Model
			}
		}
	}

	// Tools — []string → map[string]bool (lowercase keys)
	if len(src.Tools) > 0 {
		out.Tools = make(map[string]bool, len(src.Tools))
		for _, t := range src.Tools {
			out.Tools[strings.ToLower(t)] = true
		}
	}

	// Color — name → hex
	if src.Color != "" {
		if strings.HasPrefix(src.Color, "#") {
			out.Color = src.Color
		} else {
			if hex, ok := ColorToHex(src.Color); ok {
				out.Color = hex
			} else {
				msg := fmt.Sprintf("color name %q has no hex mapping, dropping", src.Color)
				if strict {
					return nil, nil, fmt.Errorf("%s", msg)
				}
				warnings = append(warnings, msg)
			}
		}
	}

	// Merge opencode-specific overrides
	if src.Opencode != nil {
		applyOpencodeOverrides(out, src.Opencode, &warnings)
	}

	return out, warnings, nil
}

func applyOpencodeOverrides(out *OpencodeOut, overrides map[string]interface{}, warnings *[]string) {
	for k, v := range overrides {
		switch k {
		case "mode":
			out.Mode = fmt.Sprintf("%v", v)
		case "temperature":
			out.Temperature = v
		case "top_p":
			out.TopP = v
		case "steps":
			out.Steps = v
		case "permission":
			if m, ok := v.(map[string]interface{}); ok {
				out.Permission = m
			} else {
				*warnings = append(*warnings, fmt.Sprintf("opencode.permission has unexpected type %T", v))
			}
		case "disable":
			out.Disable = v
		case "hidden":
			out.Hidden = v
		// shared fields that can be overridden in opencode block
		case "model":
			out.Model = fmt.Sprintf("%v", v)
		case "tools":
			out.Tools = toToolMap(v, warnings)
		case "color":
			out.Color = fmt.Sprintf("%v", v)
		default:
			log.Printf("warning: unknown opencode override field %q, ignoring", k)
		}
	}
}

func toToolMap(v interface{}, warnings *[]string) map[string]bool {
	switch val := v.(type) {
	case []interface{}:
		m := make(map[string]bool, len(val))
		for _, item := range val {
			m[strings.ToLower(fmt.Sprintf("%v", item))] = true
		}
		return m
	case map[string]interface{}:
		m := make(map[string]bool, len(val))
		for k, b := range val {
			if bv, ok := b.(bool); ok {
				m[strings.ToLower(k)] = bv
			}
		}
		return m
	default:
		*warnings = append(*warnings, fmt.Sprintf("opencode tools has unexpected type %T, dropping", v))
		return nil
	}
}
