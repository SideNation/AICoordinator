package agent_test

import (
	"strings"
	"testing"

	"github.com/nexturecorp/aico/src/agent"
)

func TestParseNoFrontmatter(t *testing.T) {
	_, err := agent.Parse("just body text with no frontmatter")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseUnclosedFrontmatter(t *testing.T) {
	_, err := agent.Parse("---\nname: x\ndescription: y\n")
	if err == nil {
		t.Error("expected error for unclosed frontmatter")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := agent.Parse("---\n: invalid: yaml: [\n---\n")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseBodyPreserved(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !strings.Contains(src.Body, "Code Reviewer Agent") {
		t.Errorf("body not preserved, got: %q", src.Body)
	}
}

func TestParseBodyEmpty(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-no-body.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if src.Body != "" {
		t.Errorf("expected empty body, got %q", src.Body)
	}
}

func TestParseToolsArray(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	want := []string{"Read", "Grep", "Bash"}
	if len(src.Tools) != len(want) {
		t.Fatalf("tools len = %d, want %d", len(src.Tools), len(want))
	}
	for i, w := range want {
		if src.Tools[i] != w {
			t.Errorf("tools[%d] = %q, want %q", i, src.Tools[i], w)
		}
	}
}

func TestParseClaudeBlock(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if src.Claude == nil {
		t.Fatal("claude block should not be nil")
	}
	if _, ok := src.Claude["permissionMode"]; !ok {
		t.Error("claude block missing permissionMode")
	}
}

func TestParseOpencodeBlock(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if src.Opencode == nil {
		t.Fatal("opencode block should not be nil")
	}
	if _, ok := src.Opencode["mode"]; !ok {
		t.Error("opencode block missing mode")
	}
}

func TestParseUseOnlyOpencode(t *testing.T) {
	src, err := agent.Parse(fixture(t, "useonly-opencode.md"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if src.UseOnly != agent.UseOnlyOpencode {
		t.Errorf("useonly = %q, want opencode", src.UseOnly)
	}
}
