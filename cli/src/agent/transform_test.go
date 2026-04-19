package agent_test

import (
	"strings"
	"testing"

	"github.com/nexturecorp/aico/src/agent"
)

// ---------- transform_claude ----------

func TestTransformClaudeProviderModelReversed(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-provider-model.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, warnings, err := agent.TransformClaude(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	// provider/model-id should be reverse-mapped to alias
	if out.Model != "sonnet" {
		t.Errorf("model = %q, want sonnet", out.Model)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTransformClaudeHexColorDropped(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-hex-color.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// #3b82f6 maps to "blue" — should resolve
	out, warnings, err := agent.TransformClaude(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if out.Color != "blue" {
		t.Errorf("color = %q, want blue", out.Color)
	}
	_ = warnings
}

func TestTransformClaudeUnknownHexStrictFails(t *testing.T) {
	src := &agent.Source{
		Name:        "x",
		Description: "y",
		Color:       "#000000", // not in palette
	}
	_, _, err := agent.TransformClaude(src, true)
	if err == nil {
		t.Error("expected strict error for unmapped hex color")
	}
}

func TestTransformClaudeUnknownHexNonStrictWarns(t *testing.T) {
	src := &agent.Source{
		Name:        "x",
		Description: "y",
		Color:       "#000000",
	}
	out, warnings, err := agent.TransformClaude(src, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Color != "" {
		t.Errorf("color should be dropped, got %q", out.Color)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for unmapped hex color")
	}
}

func TestTransformClaudeNamespaceOverridesShared(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-namespace-override.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, err := agent.TransformClaude(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	// claude block overrides model to opus
	if out.Model != "opus" {
		t.Errorf("model = %q, want opus (from claude override)", out.Model)
	}
	// claude block overrides tools
	if !strings.Contains(out.Tools, "Write") {
		t.Errorf("tools should contain Write (from claude override), got %q", out.Tools)
	}
	// claude block overrides color to red
	if out.Color != "red" {
		t.Errorf("color = %q, want red (from claude override)", out.Color)
	}
}

func TestTransformClaudeDisallowedToolsCSV(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformClaude(src, false)
	if out.DisallowedTools != "Write" {
		t.Errorf("disallowedTools = %q, want Write", out.DisallowedTools)
	}
}

func TestTransformClaudeNoBody(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-no-body.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, err := agent.TransformClaude(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if out.Name != "edge-no-body" {
		t.Errorf("name = %q", out.Name)
	}
}

func TestTransformClaudeUnknownModelStrictFails(t *testing.T) {
	src := &agent.Source{
		Name:        "x",
		Description: "y",
		Model:       "openai/gpt-4o",
	}
	_, _, err := agent.TransformClaude(src, true)
	if err == nil {
		t.Error("expected strict error for unknown provider model")
	}
}

// ---------- transform_opencode ----------

func TestTransformOpencodeProviderModelPassthrough(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-provider-model.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, warnings, err := agent.TransformOpencode(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if out.Model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("model = %q, want anthropic/claude-sonnet-4-6", out.Model)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTransformOpencodeHexColorPassthrough(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-hex-color.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, err := agent.TransformOpencode(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if out.Color != "#3b82f6" {
		t.Errorf("color = %q, want #3b82f6", out.Color)
	}
}

func TestTransformOpencodeNameColorToHex(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformOpencode(src, false)
	if !strings.HasPrefix(out.Color, "#") {
		t.Errorf("opencode color should be hex, got %q", out.Color)
	}
}

func TestTransformOpencodeToolsLowercase(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformOpencode(src, false)
	for k := range out.Tools {
		if k != strings.ToLower(k) {
			t.Errorf("tool key %q should be lowercase", k)
		}
	}
}

func TestTransformOpencodeToolsAllTrue(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformOpencode(src, false)
	for k, v := range out.Tools {
		if !v {
			t.Errorf("tool %q should be true", k)
		}
	}
}

func TestTransformOpencodeNamespaceOverridesShared(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-namespace-override.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, err := agent.TransformOpencode(src, false)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	// opencode block overrides model to haiku provider id
	if out.Model != "anthropic/claude-haiku-4-5-20251001" {
		t.Errorf("model = %q, want anthropic/claude-haiku-4-5-20251001", out.Model)
	}
	// opencode block overrides tools to only Grep
	if _, ok := out.Tools["grep"]; !ok {
		t.Errorf("tools should only contain grep, got %v", out.Tools)
	}
	if len(out.Tools) != 1 {
		t.Errorf("tools should have 1 entry, got %d: %v", len(out.Tools), out.Tools)
	}
}

func TestTransformOpencodePermission(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformOpencode(src, false)
	if out.Permission == nil {
		t.Fatal("permission should not be nil")
	}
	if out.Permission["edit"] != "ask" {
		t.Errorf("permission.edit = %v, want ask", out.Permission["edit"])
	}
	if out.Permission["bash"] != "allow" {
		t.Errorf("permission.bash = %v, want allow", out.Permission["bash"])
	}
}

func TestTransformOpencodeUnknownModelStrictFails(t *testing.T) {
	src := &agent.Source{
		Name:        "x",
		Description: "y",
		Model:       "unknownalias",
	}
	_, _, err := agent.TransformOpencode(src, true)
	if err == nil {
		t.Error("expected strict error for unknown model alias")
	}
}

func TestTransformOpencodeUnknownModelNonStrictWarns(t *testing.T) {
	src := &agent.Source{
		Name:        "x",
		Description: "y",
		Model:       "unknownalias",
	}
	out, warnings, err := agent.TransformOpencode(src, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Model != "unknownalias" {
		t.Errorf("model = %q, want unknownalias (pass-through)", out.Model)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for unknown model alias")
	}
}

func TestTransformOpencodeNoTools(t *testing.T) {
	src := &agent.Source{Name: "x", Description: "y"}
	out, _, err := agent.TransformOpencode(src, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Tools != nil {
		t.Errorf("tools should be nil for empty source, got %v", out.Tools)
	}
}
