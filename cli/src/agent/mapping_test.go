package agent_test

import (
	"testing"

	"github.com/nexturecorp/aico/src/agent"
)

// ---------- model mapping ----------

func TestModelToOpencodeKnownAliases(t *testing.T) {
	cases := map[string]string{
		"sonnet": "anthropic/claude-sonnet-4-6",
		"opus":   "anthropic/claude-opus-4-7",
		"haiku":  "anthropic/claude-haiku-4-5-20251001",
	}
	for alias, want := range cases {
		got, ok := agent.ModelToOpencode(alias)
		if !ok {
			t.Errorf("ModelToOpencode(%q) not found", alias)
		}
		if got != want {
			t.Errorf("ModelToOpencode(%q) = %q, want %q", alias, got, want)
		}
	}
}

func TestModelToOpencodeUnknown(t *testing.T) {
	_, ok := agent.ModelToOpencode("nonexistent-model")
	if ok {
		t.Error("expected not ok for unknown alias")
	}
}

func TestModelToClaudeReverseMap(t *testing.T) {
	cases := map[string]string{
		"anthropic/claude-sonnet-4-6":         "sonnet",
		"anthropic/claude-opus-4-7":           "opus",
		"anthropic/claude-haiku-4-5-20251001": "haiku",
	}
	for providerID, want := range cases {
		got, ok := agent.ModelToClaude(providerID)
		if !ok {
			t.Errorf("ModelToClaude(%q) not found", providerID)
		}
		if got != want {
			t.Errorf("ModelToClaude(%q) = %q, want %q", providerID, got, want)
		}
	}
}

func TestModelToClaudeUnknown(t *testing.T) {
	_, ok := agent.ModelToClaude("openai/gpt-4o")
	if ok {
		t.Error("expected not ok for unknown provider id")
	}
}

// ---------- color mapping ----------

func TestColorToHexKnownNames(t *testing.T) {
	cases := map[string]string{
		"blue":   "#3b82f6",
		"red":    "#ef4444",
		"green":  "#22c55e",
		"purple": "#a855f7",
	}
	for name, want := range cases {
		got, ok := agent.ColorToHex(name)
		if !ok {
			t.Errorf("ColorToHex(%q) not found", name)
		}
		if got != want {
			t.Errorf("ColorToHex(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestColorToHexCaseInsensitive(t *testing.T) {
	got, ok := agent.ColorToHex("BLUE")
	if !ok {
		t.Fatal("ColorToHex(BLUE) not found")
	}
	if got != "#3b82f6" {
		t.Errorf("ColorToHex(BLUE) = %q, want #3b82f6", got)
	}
}

func TestColorToHexUnknown(t *testing.T) {
	_, ok := agent.ColorToHex("chartreuse")
	if ok {
		t.Error("expected not ok for unknown color name")
	}
}

func TestColorToNameRoundTrip(t *testing.T) {
	names := []string{"blue", "red", "green", "purple", "orange", "yellow", "pink", "gray"}
	for _, name := range names {
		hex, ok := agent.ColorToHex(name)
		if !ok {
			t.Errorf("ColorToHex(%q) not found", name)
			continue
		}
		got, ok := agent.ColorToName(hex)
		if !ok {
			t.Errorf("ColorToName(%q) not found for name %q", hex, name)
			continue
		}
		if got != name {
			t.Errorf("round-trip color %q → %q → %q", name, hex, got)
		}
	}
}

func TestColorToNameUnknown(t *testing.T) {
	_, ok := agent.ColorToName("#000000")
	if ok {
		t.Error("expected not ok for unmapped hex")
	}
}
