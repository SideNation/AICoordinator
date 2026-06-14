package cmd

import (
	"reflect"
	"testing"
)

func TestDeepMergeMissing(t *testing.T) {
	dst := map[string]any{
		"a": "keep",
		"nested": map[string]any{
			"x": 1,
		},
		"list": []any{"one", "two"},
	}
	src := map[string]any{
		"a": "OVERWRITE?", // must NOT overwrite
		"b": "added",      // new key added
		"nested": map[string]any{
			"x": 99,    // must NOT overwrite
			"y": "new", // added
		},
		"list": []any{"two", "three"}, // "three" appended, "two" not duplicated
	}

	changed := deepMergeMissing(dst, src)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if dst["a"] != "keep" {
		t.Errorf("a was overwritten: %v", dst["a"])
	}
	if dst["b"] != "added" {
		t.Errorf("b not added: %v", dst["b"])
	}
	nested := dst["nested"].(map[string]any)
	if nested["x"] != 1 {
		t.Errorf("nested.x overwritten: %v", nested["x"])
	}
	if nested["y"] != "new" {
		t.Errorf("nested.y not added: %v", nested["y"])
	}
	gotList := dst["list"].([]any)
	wantList := []any{"one", "two", "three"}
	if !reflect.DeepEqual(gotList, wantList) {
		t.Errorf("list = %v, want %v", gotList, wantList)
	}
}

func TestDeepMergeMissingNoChange(t *testing.T) {
	dst := map[string]any{"a": "1", "b": map[string]any{"x": 1}}
	src := map[string]any{"a": "2", "b": map[string]any{"x": 2}}
	if deepMergeMissing(dst, src) {
		t.Error("expected changed=false when src only conflicts with existing keys")
	}
}
