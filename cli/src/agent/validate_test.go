package agent_test

import (
	"testing"

	"github.com/nexturecorp/aico/src/agent"
)

func TestValidateOK(t *testing.T) {
	src := &agent.Source{Name: "my-agent", Description: "does things"}
	errs := agent.Validate(src)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateMissingDescription(t *testing.T) {
	src := &agent.Source{Name: "my-agent"}
	errs := agent.Validate(src)
	if len(errs) == 0 {
		t.Error("expected validation error for missing description")
	}
}

func TestValidateBothMissing(t *testing.T) {
	src := &agent.Source{}
	errs := agent.Validate(src)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateUseOnlyBoth(t *testing.T) {
	for _, v := range []string{"", "both", "all"} {
		src := &agent.Source{Name: "x", Description: "y", UseOnly: agent.UseOnly(v)}
		if errs := agent.Validate(src); len(errs) != 0 {
			t.Errorf("useonly=%q should be valid, got errors: %v", v, errs)
		}
	}
}

func TestValidateUseOnlyClaude(t *testing.T) {
	src := &agent.Source{Name: "x", Description: "y", UseOnly: agent.UseOnlyClaude}
	if errs := agent.Validate(src); len(errs) != 0 {
		t.Errorf("useonly=claude should be valid, got: %v", errs)
	}
}

func TestValidateUseOnlyOpencode(t *testing.T) {
	src := &agent.Source{Name: "x", Description: "y", UseOnly: agent.UseOnlyOpencode}
	if errs := agent.Validate(src); len(errs) != 0 {
		t.Errorf("useonly=opencode should be valid, got: %v", errs)
	}
}

func TestValidateUseOnlyInvalid(t *testing.T) {
	for _, v := range []string{"vscode", "CLAUDE", "Open Code", "1"} {
		src := &agent.Source{Name: "x", Description: "y", UseOnly: agent.UseOnly(v)}
		if errs := agent.Validate(src); len(errs) == 0 {
			t.Errorf("useonly=%q should be invalid", v)
		}
	}
}

func TestNormalizeUseOnly(t *testing.T) {
	cases := map[agent.UseOnly]agent.UseOnly{
		"":              agent.UseOnlyBoth,
		"both":          agent.UseOnlyBoth,
		"all":           agent.UseOnlyBoth,
		"claude":        agent.UseOnlyClaude,
		"opencode":      agent.UseOnlyOpencode,
	}
	for in, want := range cases {
		got := agent.NormalizeUseOnly(in)
		if got != want {
			t.Errorf("NormalizeUseOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
