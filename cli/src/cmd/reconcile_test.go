package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nexturecorp/aico/src/agent"
	"github.com/nexturecorp/aico/src/config"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCollectPluginAssets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skills", "a", "SKILL.md"), "a")
	writeFile(t, filepath.Join(dir, "skills", "b", "SKILL.md"), "b")
	writeFile(t, filepath.Join(dir, "skills", "loose.txt"), "ignored") // not a dir
	writeFile(t, filepath.Join(dir, "agents", "alpha.md"), "---\nname: alpha-agent\ndescription: d\n---\nb\n")
	// invalid model (rejected alias) → fails Validate → skipped.
	writeFile(t, filepath.Join(dir, "agents", "bad.md"), "---\nname: bad-agent\ndescription: d\nmodel: opus\n---\nb\n")
	writeFile(t, filepath.Join(dir, "docs", "guide.md"), "g")
	writeFile(t, filepath.Join(dir, "docs", "sub", "deep.md"), "d")
	writeFile(t, filepath.Join(dir, "rules", "r1.md"), "r")
	writeFile(t, filepath.Join(dir, "hooks", "h1.sh"), "h")

	got := collectPluginAssets(dir, "user", []string{"claude"})
	want := config.PluginAssets{
		Skills: []string{"a", "b"},
		Agents: []string{"alpha-agent"},
		Docs:   []string{"guide.md", filepath.Join("sub", "deep.md")},
		Rules:  []string{"r1.md"},
		Hooks:  []string{"h1.sh"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectPluginAssets(claude) =\n  %+v\nwant\n  %+v", got, want)
	}

	// Rules and hooks are Claude-only; a codex-only target omits them.
	got = collectPluginAssets(dir, "user", []string{"codex"})
	if got.Rules != nil || got.Hooks != nil {
		t.Errorf("codex-only target should omit rules/hooks, got rules=%v hooks=%v", got.Rules, got.Hooks)
	}
	if !reflect.DeepEqual(got.Skills, []string{"a", "b"}) {
		t.Errorf("codex-only skills = %v, want [a b]", got.Skills)
	}
}

func TestReconcilePluginAssets(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	restore := confirmRemove
	confirmRemove = func(kind, name, scope, reason string) bool { return true }
	defer func() { confirmRemove = restore }()

	// skills a, b, c on disk; c will be dropped.
	for _, s := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(skillsDir("user"), s, "SKILL.md"), s)
	}
	// a doc file under a subdir that should be pruned when emptied.
	writeFile(t, filepath.Join(docsDir("user"), "sub", "x.md"), "x")
	// an agent rendered for claude.
	claude, _ := agent.ResolvePlatform("claude")
	writeFile(t, claude.Path("user", "gone-agent"), "agent")

	old := config.PluginAssets{
		Skills: []string{"a", "b", "c"},
		Agents: []string{"gone-agent"},
		Docs:   []string{filepath.Join("sub", "x.md")},
	}
	cur := config.PluginAssets{Skills: []string{"a", "b"}}

	reconcilePluginAssets(&old, cur, "user")

	// c removed, a and b kept.
	for _, s := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(skillsDir("user"), s)); err != nil {
			t.Errorf("skill %s should be kept, got %v", s, err)
		}
	}
	if _, err := os.Stat(filepath.Join(skillsDir("user"), "c")); !os.IsNotExist(err) {
		t.Errorf("skill c should be removed, got err=%v", err)
	}
	// agent removed.
	if _, err := os.Stat(claude.Path("user", "gone-agent")); !os.IsNotExist(err) {
		t.Errorf("gone-agent should be removed, got err=%v", err)
	}
	// doc file removed and its now-empty parent pruned.
	if _, err := os.Stat(filepath.Join(docsDir("user"), "sub", "x.md")); !os.IsNotExist(err) {
		t.Errorf("doc should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(docsDir("user"), "sub")); !os.IsNotExist(err) {
		t.Errorf("emptied sub dir should be pruned, got err=%v", err)
	}
}

func TestReconcilePluginAssetsNilOldNoop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeFile(t, filepath.Join(skillsDir("user"), "keep", "SKILL.md"), "k")

	// nil old (legacy record) must not remove anything, regardless of confirm.
	restore := confirmRemove
	confirmRemove = func(kind, name, scope, reason string) bool { return true }
	defer func() { confirmRemove = restore }()

	reconcilePluginAssets(nil, config.PluginAssets{}, "user")

	if _, err := os.Stat(filepath.Join(skillsDir("user"), "keep")); err != nil {
		t.Errorf("nil old should be a no-op, but skill was removed: %v", err)
	}
}

func TestReconcileKeepsOnDecline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	writeFile(t, filepath.Join(skillsDir("user"), "c", "SKILL.md"), "c")

	restore := confirmRemove
	confirmRemove = func(kind, name, scope, reason string) bool { return false } // decline
	defer func() { confirmRemove = restore }()

	old := config.PluginAssets{Skills: []string{"c"}}
	reconcilePluginAssets(&old, config.PluginAssets{}, "user")

	if _, err := os.Stat(filepath.Join(skillsDir("user"), "c")); err != nil {
		t.Errorf("declined removal should keep skill c, got %v", err)
	}
}
