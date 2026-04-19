package agent

import (
	"fmt"
	"log"
	"strings"
)

// TransformClaude converts a Source into a ClaudeOut + body.
// warnings are non-fatal issues found during conversion.
func TransformClaude(src *Source, strict bool) (*ClaudeOut, []string, error) {
	var warnings []string

	out := &ClaudeOut{
		Name:        src.Name,
		Description: src.Description,
	}

	// Model
	if src.Model != "" {
		if strings.Contains(src.Model, "/") {
			// provider/id form — reverse-map to alias
			if alias, ok := ModelToClaude(src.Model); ok {
				out.Model = alias
			} else {
				msg := fmt.Sprintf("model %q has no Claude alias mapping, using as-is", src.Model)
				if strict {
					return nil, nil, fmt.Errorf("%s", msg)
				}
				warnings = append(warnings, msg)
				out.Model = src.Model
			}
		} else {
			out.Model = src.Model
		}
	}

	// Tools — array → CSV
	if len(src.Tools) > 0 {
		out.Tools = strings.Join(src.Tools, ", ")
	}

	// Color — name stays as name; hex → try to find name
	if src.Color != "" {
		if strings.HasPrefix(src.Color, "#") {
			if name, ok := ColorToName(src.Color); ok {
				out.Color = name
			} else {
				msg := fmt.Sprintf("color hex %q has no Claude name mapping, dropping", src.Color)
				if strict {
					return nil, nil, fmt.Errorf("%s", msg)
				}
				warnings = append(warnings, msg)
			}
		} else {
			out.Color = src.Color
		}
	}

	// Merge claude-specific overrides
	if src.Claude != nil {
		applyClaudeOverrides(out, src.Claude, &warnings)
	}

	return out, warnings, nil
}

func applyClaudeOverrides(out *ClaudeOut, overrides map[string]interface{}, warnings *[]string) {
	for k, v := range overrides {
		switch k {
		case "permissionMode":
			out.PermissionMode = fmt.Sprintf("%v", v)
		case "disallowedTools":
			out.DisallowedTools = toCSV(v)
		case "maxTurns":
			out.MaxTurns = v
		case "skills":
			out.Skills = v
		case "mcpServers":
			out.McpServers = v
		case "hooks":
			out.Hooks = v
		case "memory":
			out.Memory = fmt.Sprintf("%v", v)
		case "background":
			out.Background = v
		case "effort":
			out.Effort = fmt.Sprintf("%v", v)
		case "isolation":
			out.Isolation = fmt.Sprintf("%v", v)
		case "initialPrompt":
			out.InitialPrompt = fmt.Sprintf("%v", v)
		// shared fields that can be overridden in claude block
		case "model":
			out.Model = fmt.Sprintf("%v", v)
		case "tools":
			out.Tools = toCSV(v)
		case "color":
			out.Color = fmt.Sprintf("%v", v)
		default:
			log.Printf("warning: unknown claude override field %q, ignoring", k)
		}
	}
}

func toCSV(v interface{}) string {
	switch val := v.(type) {
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	case string:
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}
