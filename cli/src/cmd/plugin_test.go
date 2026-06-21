package cmd

import (
	"reflect"
	"testing"

	"github.com/nexturecorp/aico/src/config"
)

func testManifest() *config.Manifest {
	return &config.Manifest{
		Plugins: map[string]config.PluginEntry{
			"unity":           {Chain: []string{"csharp"}, Version: "1.0.0"},
			"csharp":          {Version: "1.0.0"},
			"claude-workflow": {Target: []string{"claude"}, Version: "1.0.0"},
		},
	}
}

func TestExpandChains(t *testing.T) {
	m := testManifest()
	got := expandChains(m, []string{"unity"})
	want := []string{"unity", "csharp"} // chained plugin follows its parent
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expandChains(unity) = %v, want %v", got, want)
	}

	// already-present chain target is not duplicated
	got = expandChains(m, []string{"csharp", "unity"})
	want = []string{"csharp", "unity"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expandChains(csharp,unity) = %v, want %v", got, want)
	}
}

func TestEffectiveTargets(t *testing.T) {
	all := []string{"claude", "codex", "kilo", "opencode"}

	// empty plugin target → pass through CLI selection
	if got := effectiveTargets(all, nil); !reflect.DeepEqual(got, all) {
		t.Errorf("effectiveTargets(all, nil) = %v, want %v", got, all)
	}

	// plugin target restricts even when CLI is "all"
	got := effectiveTargets(all, []string{"claude"})
	if !reflect.DeepEqual(got, []string{"claude"}) {
		t.Errorf("effectiveTargets(all, [claude]) = %v, want [claude]", got)
	}

	// intersection of a narrowed CLI and a plugin target
	got = effectiveTargets([]string{"codex"}, []string{"claude", "codex"})
	if !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("effectiveTargets([codex], [claude,codex]) = %v, want [codex]", got)
	}

	// disjoint → empty
	got = effectiveTargets([]string{"opencode"}, []string{"claude"})
	if len(got) != 0 {
		t.Errorf("effectiveTargets([opencode], [claude]) = %v, want empty", got)
	}
}
