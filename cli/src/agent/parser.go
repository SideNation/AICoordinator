package agent

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const delimiter = "---"

// Parse reads a single-source agent markdown and returns a Source.
func Parse(content string) (*Source, error) {
	content = strings.TrimPrefix(content, "\xef\xbb\xbf") // strip BOM

	if !strings.HasPrefix(strings.TrimSpace(content), delimiter) {
		return nil, fmt.Errorf("file must begin with --- frontmatter delimiter")
	}

	// Strip leading ---
	rest := strings.TrimSpace(content)
	rest = strings.TrimPrefix(rest, delimiter)

	// Find closing ---
	idx := strings.Index(rest, "\n"+delimiter)
	if idx == -1 {
		return nil, fmt.Errorf("missing closing --- frontmatter delimiter")
	}

	rawFront := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+1+len(delimiter):])

	// Strip trailing --- if it has nothing after
	body = strings.TrimPrefix(body, "\n")

	var src Source
	if err := yaml.Unmarshal([]byte(rawFront), &src); err != nil {
		return nil, fmt.Errorf("frontmatter parse error: %w", err)
	}
	src.Body = body

	return &src, nil
}

// Validate checks required fields and useonly value.
func Validate(src *Source) []string {
	var errs []string

	if strings.TrimSpace(src.Name) == "" {
		errs = append(errs, "missing required field: name")
	}
	if strings.TrimSpace(src.Description) == "" {
		errs = append(errs, "missing required field: description")
	}

	switch src.UseOnly {
	case UseOnlyBoth, UseOnlyClaude, UseOnlyOpencode:
		// valid
	case "both", "all":
		// treat as both — normalised during transform
	default:
		if src.UseOnly != "" {
			errs = append(errs, fmt.Sprintf("invalid useonly value %q: must be claude, opencode, both, all, or empty", src.UseOnly))
		}
	}

	// Warn about tool-specific fields leaking into top-level (not fatal, but flagged)
	claudeOnlyTopLevel := []string{
		"permissionMode", "disallowedTools", "maxTurns", "skills",
		"mcpServers", "hooks", "memory", "background", "effort",
		"isolation", "initialPrompt",
	}
	_ = claudeOnlyTopLevel // checked via raw YAML in future strict pass

	return errs
}

// NormalizeUseOnly converts "both"/"all"/"" to UseOnlyBoth.
func NormalizeUseOnly(u UseOnly) UseOnly {
	switch u {
	case "both", "all", "":
		return UseOnlyBoth
	default:
		return u
	}
}

// ShouldBuildClaude returns true if the source targets Claude.
func ShouldBuildClaude(src *Source, cliOnly string) bool {
	target := resolveTarget(src, cliOnly)
	return target == UseOnlyBoth || target == UseOnlyClaude
}

// ShouldBuildOpencode returns true if the source targets opencode.
func ShouldBuildOpencode(src *Source, cliOnly string) bool {
	target := resolveTarget(src, cliOnly)
	return target == UseOnlyBoth || target == UseOnlyOpencode
}

func resolveTarget(src *Source, cliOnly string) UseOnly {
	if cliOnly != "" {
		return UseOnly(cliOnly)
	}
	return NormalizeUseOnly(src.UseOnly)
}
