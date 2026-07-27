package agent

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

const sampleSource = `---
name: code-reviewer
description: Reviews changed code for safety, readability, and performance.
model: high
effort: high
tools: [Read, Grep, Glob, Bash]
color: blue
useonly: all

claude:
  permissionMode: default
  maxTurns: 30
  mcpServers: [github]
  memory: project

opencode:
  mode: subagent
  temperature: 0.2
  steps: 40
  permission:
    bash: ask
    edit: deny

codex:
  sandbox_mode: read-only

kilo:
  mode: subagent
  temperature: 0.2
  steps: 40
  permission:
    bash: ask
    edit: deny
  hidden: false
---

You are a senior code reviewer.
`

func TestParseSampleSource(t *testing.T) {
	src, err := Parse(sampleSource)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if src.Name != "code-reviewer" {
		t.Errorf("name: %q", src.Name)
	}
	if src.Model != "high" {
		t.Errorf("model: %q", src.Model)
	}
	if src.Effort != "high" {
		t.Errorf("effort: %q", src.Effort)
	}
	wantTools := []string{"Read", "Grep", "Glob", "Bash"}
	if !reflect.DeepEqual(src.Tools, wantTools) {
		t.Errorf("tools: %v", src.Tools)
	}
	if src.UseOnly != "all" {
		t.Errorf("useonly: %q", src.UseOnly)
	}
	if !strings.HasPrefix(src.Body, "You are a senior code reviewer.") {
		t.Errorf("body: %q", src.Body)
	}
	if got, want := src.Codex["sandbox_mode"], "read-only"; got != want {
		t.Errorf("codex.sandbox_mode: %v want %v", got, want)
	}
}

func TestValidateModelTier(t *testing.T) {
	cases := []struct {
		model string
		ok    bool
	}{
		{"high", true},
		{"medium", true},
		{"low", true},
		{"", true},
		{"sonnet", false},
		{"opus", false},
		{"haiku", false},
		{"midium", false},
		{"anthropic/claude-sonnet-4-6", false},
	}
	for _, c := range cases {
		src := &Source{Name: "a", Description: "d", Model: c.model}
		err := Validate(src)
		if (err == nil) != c.ok {
			t.Errorf("model %q: got err=%v, want ok=%v", c.model, err, c.ok)
		}
	}
}

func TestValidateEffort(t *testing.T) {
	cases := map[string]bool{
		"":       true,
		"low":    true,
		"medium": true,
		"high":   true,
		"xhigh":  true,
		"max":    true,
		"x":      false,
	}
	for v, ok := range cases {
		src := &Source{Name: "a", Description: "d", Effort: v}
		err := Validate(src)
		if (err == nil) != ok {
			t.Errorf("effort %q: got err=%v, want ok=%v", v, err, ok)
		}
	}
}

func TestParseTargetSpec(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		err  bool
	}{
		{"", []string{"claude", "codex", "kilo", "opencode"}, false},
		{"all", []string{"claude", "codex", "kilo", "opencode"}, false},
		{"cl,op", []string{"claude", "opencode"}, false},
		{"co", []string{"codex"}, false},
		{"codex,kilo,claude,opencode", []string{"claude", "codex", "kilo", "opencode"}, false},
		{"ki , co", []string{"codex", "kilo"}, false},
		{"unknown", nil, true},
	}
	for _, c := range cases {
		got, err := ParseTargetSpec(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseTargetSpec(%q) want err", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTargetSpec(%q): %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseTargetSpec(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseLockTargetLegacy(t *testing.T) {
	if got := ParseLockTarget(""); !reflect.DeepEqual(got, []string{"claude", "opencode"}) {
		t.Errorf("empty: %v", got)
	}
	if got := ParseLockTarget("all"); !reflect.DeepEqual(got, []string{"claude", "opencode"}) {
		t.Errorf("all: %v", got)
	}
	if got := ParseLockTarget("claude"); !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("claude: %v", got)
	}
	got := ParseLockTarget("co,ki")
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"codex", "kilo"}) {
		t.Errorf("co,ki: %v", got)
	}
}

func TestEffectiveTargetsUseOnly(t *testing.T) {
	src := &Source{UseOnly: "claude"}
	got := EffectiveTargets(src, []string{"claude", "opencode"})
	if !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("useonly=claude: %v", got)
	}

	src = &Source{UseOnly: "kilo"}
	if got := EffectiveTargets(src, []string{"claude", "opencode"}); len(got) != 0 {
		t.Errorf("useonly=kilo with no kilo selected: %v", got)
	}

	src = &Source{UseOnly: "all"}
	got = EffectiveTargets(src, nil)
	if !reflect.DeepEqual(got, AllPlatformNames()) {
		t.Errorf("useonly=all default: %v", got)
	}
}

func TestRenderClaudeModelMapping(t *testing.T) {
	src := mustParse(t, sampleSource)
	out, _, err := renderClaude(src)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "model: opus") {
		t.Errorf("expected model: opus in claude output, got:\n%s", body)
	}
	if !strings.Contains(body, "effort: high") {
		t.Errorf("expected effort: high")
	}
	if !strings.Contains(body, "tools: Read, Grep, Glob, Bash") {
		t.Errorf("expected tools csv")
	}
}

func TestRenderOpencodeModelMapping(t *testing.T) {
	src := mustParse(t, sampleSource)
	out, _, err := renderOpencode(src)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "model: openai/gpt-5.5") {
		t.Errorf("expected opencode high model, got:\n%s", body)
	}
	if !strings.Contains(body, "color: \"#3b82f6\"") {
		t.Errorf("expected hex color for blue")
	}
}

func TestRenderCodexEffortField(t *testing.T) {
	src := mustParse(t, sampleSource)
	out, _, err := renderCodex(src)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	// model + model_reasoning_effort + sandbox_mode + developer_instructions block
	for _, want := range []string{
		`name = "code-reviewer"`,
		`model = "gpt-5.5"`,
		`model_reasoning_effort = "high"`,
		`sandbox_mode = "read-only"`,
		`developer_instructions = """`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("codex output missing %q:\n%s", want, body)
		}
	}
	// Codex has no per-agent tool-allowlist or color field, and no `prompt`
	// field — sampleSource sets tools: and color:, so a regression here would
	// silently write a key Codex can't deserialize (see
	// WebSearchToolConfigInput) or a key Codex just ignores.
	for _, unwanted := range []string{"tools =", "color =", "reasoningEffort =", "prompt ="} {
		if strings.Contains(body, unwanted) {
			t.Errorf("codex output should not contain %q:\n%s", unwanted, body)
		}
	}
}

func TestRenderKiloReasoningEffort(t *testing.T) {
	src := mustParse(t, sampleSource)
	out, _, err := renderKilo(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "reasoningEffort: high") {
		t.Errorf("kilo expected reasoningEffort, got:\n%s", out)
	}
}

func mustParse(t *testing.T, s string) *Source {
	t.Helper()
	src, err := Parse(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return src
}

func TestPlatformDirsProject(t *testing.T) {
	cases := map[string]struct {
		agents string
		rules  string
		docs   string
	}{
		"claude":   {".claude/agents", ".claude/rules", ".claude/docs"},
		"codex":    {".codex/agents", ".codex/rules", ".codex/docs"},
		"kilo":     {".kilo/agents", ".kilo/rules", ".kilo/docs"},
		"opencode": {".opencode/agents", ".opencode/rules", ".opencode/docs"},
	}
	for name, want := range cases {
		p, ok := ResolvePlatform(name)
		if !ok {
			t.Fatalf("platform %q not found", name)
		}
		if got := p.AgentsDir("project"); got != want.agents {
			t.Errorf("%s agents: %q want %q", name, got, want.agents)
		}
		if got := p.RulesDir("project"); got != want.rules {
			t.Errorf("%s rules: %q want %q", name, got, want.rules)
		}
		if got := p.DocsDir("project"); got != want.docs {
			t.Errorf("%s docs: %q want %q", name, got, want.docs)
		}
	}
}

func TestLinkedAgentPathAlwaysMD(t *testing.T) {
	for _, name := range []string{"claude", "codex", "kilo", "opencode"} {
		p, ok := ResolvePlatform(name)
		if !ok {
			t.Fatalf("platform %q not found", name)
		}
		got := p.LinkedAgentPath("project", "code-reviewer")
		if !strings.HasSuffix(got, "code-reviewer.md") {
			t.Errorf("%s: LinkedAgentPath %q must end in .md", name, got)
		}
	}
}

func TestEnsureClaudeAlwaysIncluded(t *testing.T) {
	got := EnsureClaude([]string{"codex"})
	if !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Errorf("EnsureClaude([codex]) = %v", got)
	}
	got = EnsureClaude([]string{"claude", "codex"})
	if !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Errorf("EnsureClaude([claude,codex]) = %v", got)
	}
	got = EnsureClaude(nil)
	if !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("EnsureClaude(nil) = %v", got)
	}
}

func TestHasNonClaude(t *testing.T) {
	if HasNonClaude([]string{"claude"}) {
		t.Error("HasNonClaude([claude]) should be false")
	}
	if !HasNonClaude([]string{"claude", "codex"}) {
		t.Error("HasNonClaude([claude,codex]) should be true")
	}
	if !HasNonClaude([]string{"opencode"}) {
		t.Error("HasNonClaude([opencode]) should be true")
	}
}
