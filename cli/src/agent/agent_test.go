package agent_test

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture reads a test fixture file relative to the test/fixtures directory.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "test", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return string(data)
}
