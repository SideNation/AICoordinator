package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	toml "github.com/pelletier/go-toml/v2"
)

// deepMergeMissing overlays src onto dst, adding only what dst lacks:
//   - keys absent from dst are copied from src
//   - keys present in both, where both values are maps, are merged recursively
//   - keys present in both, where both values are slices, get src elements that
//     are not already present (deep-equal) appended
//   - any other conflict keeps the dst value untouched (never overwrite)
//
// Returns true when dst was modified.
func deepMergeMissing(dst, src map[string]any) bool {
	changed := false
	for k, sv := range src {
		dv, ok := dst[k]
		if !ok {
			dst[k] = sv
			changed = true
			continue
		}
		dm, dIsMap := dv.(map[string]any)
		sm, sIsMap := sv.(map[string]any)
		if dIsMap && sIsMap {
			if deepMergeMissing(dm, sm) {
				changed = true
			}
			continue
		}
		ds, dIsSlice := dv.([]any)
		ss, sIsSlice := sv.([]any)
		if dIsSlice && sIsSlice {
			merged, added := appendMissing(ds, ss)
			if added {
				dst[k] = merged
				changed = true
			}
			continue
		}
		// scalar or type mismatch: keep dst.
	}
	return changed
}

// appendMissing returns dst with every src element not already present (by
// deep equality) appended, and whether anything was added.
func appendMissing(dst, src []any) ([]any, bool) {
	added := false
	for _, s := range src {
		found := false
		for _, d := range dst {
			if reflect.DeepEqual(d, s) {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, s)
			added = true
		}
	}
	return dst, added
}

// mergeJSONFile merges srcPath into dstPath using deepMergeMissing. When
// dstPath does not exist it is created as a copy of src (re-encoded with
// 2-space indent). Returns nil and does nothing when srcPath is absent.
func mergeJSONFile(srcPath, dstPath string) error {
	src, err := readJSONObject(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst, err := readJSONObject(dstPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		dst = map[string]any{}
	}
	deepMergeMissing(dst, src)
	out, err := json.MarshalIndent(dst, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json %s: %w", dstPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(dstPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("write %s: %w", dstPath, err)
	}
	return nil
}

// mergeTOMLFile merges srcPath into dstPath using deepMergeMissing, decoding
// and re-encoding via TOML. When dstPath is absent it is created from src.
func mergeTOMLFile(srcPath, dstPath string) error {
	src, err := readTOMLObject(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst, err := readTOMLObject(dstPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		dst = map[string]any{}
	}
	deepMergeMissing(dst, src)
	out, err := toml.Marshal(dst)
	if err != nil {
		return fmt.Errorf("encode toml %s: %w", dstPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(dstPath, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", dstPath, err)
	}
	return nil
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func readTOMLObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse toml %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}
