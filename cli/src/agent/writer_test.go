package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexturecorp/aico/src/agent"
)

// ---------- render ----------

func TestRenderClaudeBodyIncluded(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformClaude(src, false)
	data, err := agent.RenderClaude(out, src.Body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(data), "Code Reviewer Agent") {
		t.Error("rendered output should include body content")
	}
}

func TestRenderClaudeEmptyBody(t *testing.T) {
	src, err := agent.Parse(fixture(t, "edge-no-body.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformClaude(src, false)
	data, err := agent.RenderClaude(out, src.Body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		t.Error("should start with ---")
	}
	if strings.Count(s, "---") < 2 {
		t.Error("should have opening and closing ---")
	}
}

func TestRenderOpencodeNoNameField(t *testing.T) {
	src, err := agent.Parse(fixture(t, "full.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, _ := agent.TransformOpencode(src, false)
	data, err := agent.RenderOpencode(out, src.Body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(data), "\nname:") {
		t.Error("opencode output must not contain name field")
	}
}

func TestRenderOpencodeDelimiters(t *testing.T) {
	src, _ := agent.Parse(fixture(t, "minimal.md"))
	out, _, _ := agent.TransformOpencode(src, false)
	data, _ := agent.RenderOpencode(out, src.Body)
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		t.Error("should start with ---")
	}
	if !strings.Contains(s, "\n---\n") {
		t.Error("should have closing ---")
	}
}

func TestRenderClaudeCSVTools(t *testing.T) {
	src, _ := agent.Parse(fixture(t, "full.md"))
	out, _, _ := agent.TransformClaude(src, false)
	data, _ := agent.RenderClaude(out, src.Body)
	s := string(data)
	if !strings.Contains(s, "tools: Read, Grep, Bash") {
		t.Errorf("claude tools should be CSV in YAML, content:\n%s", s)
	}
}

func TestRenderOpencodeToolsMap(t *testing.T) {
	src, _ := agent.Parse(fixture(t, "full.md"))
	out, _, _ := agent.TransformOpencode(src, false)
	data, _ := agent.RenderOpencode(out, src.Body)
	s := string(data)
	// map keys should appear as yaml map entries
	if !strings.Contains(s, "read: true") {
		t.Errorf("opencode tools should be map with 'read: true', content:\n%s", s)
	}
}

// ---------- WriteFile ----------

func TestWriteFileCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "out.md")
	_, err := agent.WriteFile(path, []byte("hello"), 0, true)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWriteFileForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if _, err := agent.WriteFile(path, []byte("v1"), 100, true); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// future mtime = 200, file mtime < 200, so without force it would skip
	// but with force=true it must write
	wrote, err := agent.WriteFile(path, []byte("v2"), 200, true)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !wrote {
		t.Error("expected wrote=true with force")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "v2" {
		t.Errorf("content = %q, want v2", data)
	}
}

func TestWriteFileSkipsWhenUpToDate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	// write first
	if _, err := agent.WriteFile(path, []byte("v1"), 0, true); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// srcMtime=0 means file is always newer → skip
	wrote, err := agent.WriteFile(path, []byte("v2"), 0, false)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if wrote {
		t.Error("expected wrote=false when destination is up-to-date")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "v1" {
		t.Errorf("content should remain v1, got %q", data)
	}
}
