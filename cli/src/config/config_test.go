package config

import "testing"

func sampleManifest() *Manifest {
	return &Manifest{
		Plugins: map[string]PluginEntry{
			"unity":                   {Source: "unity", Alias: []string{"unity3d"}, Chain: []string{"csharp"}, Version: "1.0.0"},
			"csharp":                  {Source: "csharp", Alias: []string{"c#"}, Version: "1.0.0"},
			"claude-code-game-studio": {Source: "claude-code-game-studio", Alias: []string{"ccgs", "game-studio"}, Target: []string{"claude", "codex"}, Version: "1.1.0"},
		},
	}
}

func TestResolvePlugin(t *testing.T) {
	m := sampleManifest()
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"unity", "unity", true},
		{"unity3d", "unity", true}, // alias
		{"UNITY3D", "unity", true}, // case-insensitive alias
		{"c#", "csharp", true},     // alias with symbol
		{"ccgs", "claude-code-game-studio", true},
		{"game-studio", "claude-code-game-studio", true},
		{"nope", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		name, _, ok := m.ResolvePlugin(c.in)
		if ok != c.ok || name != c.want {
			t.Errorf("ResolvePlugin(%q) = (%q,%v), want (%q,%v)", c.in, name, ok, c.want, c.ok)
		}
	}
}

func TestPluginNames(t *testing.T) {
	got := sampleManifest().PluginNames()
	want := []string{"claude-code-game-studio", "csharp", "unity"} // sorted
	if len(got) != len(want) {
		t.Fatalf("PluginNames len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("PluginNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
