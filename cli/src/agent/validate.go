package agent

import (
	"fmt"
	"strings"
)

// Allowed common model tiers. Platform-native model ids are NOT allowed at
// the common level — authors must use the tier name and let renderers map to
// the platform's id.
var allowedModels = map[string]struct{}{
	"high":   {},
	"medium": {},
	"low":    {},
}

// Legacy aliases that used to be accepted; we reject them now with a clear
// message so old sources fail fast instead of silently picking a default.
var rejectedModelAliases = map[string]string{
	"opus":   "use model: high",
	"sonnet": "use model: medium",
	"haiku":  "use model: low",
	"midium": "typo for medium — use model: medium",
}

// Allowed common effort values. Renderers may still warn if a specific
// platform does not support a given value.
// xhigh: all platforms. max: claude only — non-claude renderers substitute xhigh.
var allowedEfforts = map[string]struct{}{
	"":       {}, // unset
	"low":    {},
	"medium": {},
	"high":   {},
	"xhigh":  {},
	"max":    {},
}

// Validate runs source-level checks shared across all platforms. Returns an
// aggregated error listing every problem found.
func Validate(src *Source) error {
	var errs []string

	if strings.TrimSpace(src.Name) == "" {
		errs = append(errs, "missing required field: name")
	}
	if strings.TrimSpace(src.Description) == "" {
		errs = append(errs, "missing required field: description")
	}

	if src.Model != "" {
		key := strings.ToLower(strings.TrimSpace(src.Model))
		if reason, bad := rejectedModelAliases[key]; bad {
			errs = append(errs, fmt.Sprintf("invalid model %q: %s", src.Model, reason))
		} else if _, ok := allowedModels[key]; !ok {
			errs = append(errs, fmt.Sprintf(
				"invalid model %q: allowed values are high, medium, low", src.Model))
		}
	}

	if src.Effort != "" {
		key := strings.ToLower(strings.TrimSpace(src.Effort))
		if _, ok := allowedEfforts[key]; !ok {
			errs = append(errs, fmt.Sprintf(
				"invalid effort %q: allowed values are low, medium, high, xhigh, max", src.Effort))
		}
	}

	if src.UseOnly != "" {
		if _, ok := ResolvePlatform(src.UseOnly); !ok && !isUseOnlyAll(src.UseOnly) {
			errs = append(errs, fmt.Sprintf(
				"invalid useonly %q: must be all, claude, codex, kilo, opencode (or alias)", src.UseOnly))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid agent %q:\n  - %s",
			src.Name, strings.Join(errs, "\n  - "))
	}
	return nil
}

func isUseOnlyAll(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "all", "both", "*":
		return true
	}
	return false
}

// EffectiveTargets intersects the source's useonly with the user's --target
// selection. An empty selection means "every platform". Returns canonical
// platform names in a deterministic order.
func EffectiveTargets(src *Source, selected []string) []string {
	// Start from --target selection or all platforms.
	var pool []string
	if len(selected) == 0 {
		pool = AllPlatformNames()
	} else {
		seen := map[string]struct{}{}
		for _, s := range selected {
			p, ok := ResolvePlatform(s)
			if !ok {
				continue
			}
			if _, dup := seen[p.Name]; dup {
				continue
			}
			seen[p.Name] = struct{}{}
			pool = append(pool, p.Name)
		}
	}

	if isUseOnlyAll(src.UseOnly) {
		return pool
	}
	p, ok := ResolvePlatform(src.UseOnly)
	if !ok {
		return nil
	}
	for _, name := range pool {
		if name == p.Name {
			return []string{p.Name}
		}
	}
	return nil
}
