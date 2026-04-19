package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type modelMapping struct {
	Aliases map[string]string `yaml:"aliases"`
}

type colorMapping struct {
	Colors map[string]string `yaml:"colors"`
}

var (
	modelMap modelMapping
	colorMap colorMapping
)

func init() {
	// Locate mapping directory relative to this source file (dev) or binary (prod).
	dir := mappingDir()
	if err := loadYAML(filepath.Join(dir, "models.yaml"), &modelMap); err != nil {
		panic(fmt.Sprintf("failed to load models.yaml from %s: %v", dir, err))
	}
	if err := loadYAML(filepath.Join(dir, "colors.yaml"), &colorMap); err != nil {
		panic(fmt.Sprintf("failed to load colors.yaml from %s: %v", dir, err))
	}
}

func mappingDir() string {
	// Try next to the binary first (production layout: bin/../src/mapping)
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "src", "mapping")
		if _, err := os.Stat(filepath.Join(candidate, "models.yaml")); err == nil {
			return candidate
		}
	}
	// Fall back to path relative to this source file (go run / go test)
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "mapping")
}

func loadYAML(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// ModelToOpencode converts a Claude alias to opencode provider/model-id.
func ModelToOpencode(alias string) (string, bool) {
	v, ok := modelMap.Aliases[strings.ToLower(alias)]
	return v, ok
}

// ModelToClaude converts a provider/model-id to a Claude alias.
func ModelToClaude(providerID string) (string, bool) {
	for alias, pid := range modelMap.Aliases {
		if pid == providerID {
			return alias, true
		}
	}
	return "", false
}

// ColorToHex converts a Claude color name to hex.
func ColorToHex(name string) (string, bool) {
	v, ok := colorMap.Colors[strings.ToLower(name)]
	return v, ok
}

// ColorToName tries to find the closest Claude color name for a hex value.
func ColorToName(hex string) (string, bool) {
	lower := strings.ToLower(hex)
	for name, h := range colorMap.Colors {
		if strings.ToLower(h) == lower {
			return name, true
		}
	}
	return "", false
}
