// Package config manages the user-global aico configuration under ~/.aico/.
//
// Layout:
//
//	~/.aico/
//	  .aicorc   — YAML: init_dir, clone_dir, git_url
//	  .lock     — YAML: list of install records
//	  packages/ — git-cloned package repo (contains packages/ subdir)
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Rc struct {
	InitDir  string                       `yaml:"init_dir"`          // directory where `aico init` was run
	CloneDir string                       `yaml:"clone_dir"`         // absolute path to cloned package repo
	GitURL   string                       `yaml:"git_url,omitempty"` // git remote url used for clone
	Models   map[string]map[string]string `yaml:"models,omitempty"`  // platform -> tier -> model id overrides
}

// PluginState records what was installed for one plugin: the manifest version
// and the date/time of the last install or update (local time, "2006-01-02
// 15:04:05").
type PluginState struct {
	Version string `yaml:"version"`
	Updated string `yaml:"updated,omitempty"`
}

// InstallRecord describes one install destination tracked in .lock.
// Plugins maps each installed plugin's name to its state (version + timestamp);
// Docs maps remote docs to their version. InitDone lists the plugins whose
// _init scaffold has already been applied (so update and repeat installs do not
// re-copy it).
type InstallRecord struct {
	Path     string                 `yaml:"path"`               // absolute install directory (project root or user home)
	Scope    string                 `yaml:"scope"`              // "project" | "user"
	Target   string                 `yaml:"target"`             // "claude" | "opencode" | "all"
	Plugins  map[string]PluginState `yaml:"plugins,omitempty"`  // plugin name -> {version, updated}
	Docs     map[string]string      `yaml:"docs,omitempty"`     // doc name -> manifest version
	InitDone []string               `yaml:"initdone,omitempty"` // plugins whose _init scaffold was applied
	Version  string                 `yaml:"version,omitempty"`  // package repo commit hash at install time
}

type Lock struct {
	Installs []InstallRecord `yaml:"installs"`
}

// AicoDir returns ~/.aico, creating it if missing.
func AicoDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".aico")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func rcPath() (string, error) {
	d, err := AicoDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, ".aicorc"), nil
}

func lockPath() (string, error) {
	d, err := AicoDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, ".lock"), nil
}

// LoadRc reads ~/.aico/.aicorc. Returns (nil, nil) when the file does not exist.
func LoadRc() (*Rc, error) {
	p, err := rcPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rc Rc
	if err := yaml.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &rc, nil
}

func SaveRc(rc *Rc) error {
	p, err := rcPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(rc)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// LoadLock reads ~/.aico/.lock. Returns an empty Lock when missing.
func LoadLock() (*Lock, error) {
	p, err := lockPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lock{}, nil
		}
		return nil, err
	}
	var lock Lock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &lock, nil
}

func SaveLock(lock *Lock) error {
	p, err := lockPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// Upsert replaces an existing record matching (path, scope) or appends a new one.
func (l *Lock) Upsert(rec InstallRecord) {
	for i, r := range l.Installs {
		if r.Path == rec.Path && r.Scope == rec.Scope {
			l.Installs[i] = rec
			return
		}
	}
	l.Installs = append(l.Installs, rec)
}

// Remove deletes records whose Path no longer exists on disk. Returns the
// removed entries for reporting.
func (l *Lock) RemoveMissing() []InstallRecord {
	var kept, removed []InstallRecord
	for _, r := range l.Installs {
		if r.Scope == "user" {
			kept = append(kept, r)
			continue
		}
		if _, err := os.Stat(r.Path); err != nil && os.IsNotExist(err) {
			removed = append(removed, r)
			continue
		}
		kept = append(kept, r)
	}
	l.Installs = kept
	return removed
}

// LoadDotenv parses KEY=VALUE lines from .env in dir. Missing file is not an error.
// Lines starting with # are treated as comments. Values may be optionally
// wrapped in single or double quotes.
func LoadDotenv(dir string) (map[string]string, error) {
	path := filepath.Join(dir, ".env")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		out[key] = val
	}
	return out, scanner.Err()
}

// PackagesDir returns the directory under the cloned repo that holds
// agents/, skills/, docs/. Per the project layout, packages live in
// `<clone_dir>/packages/`.
func PackagesDir(cloneDir string) string {
	return filepath.Join(cloneDir, "packages")
}

// ManifestEntry holds the declared version of a remote doc. Source is the
// upstream URL or a packages-relative path.
type ManifestEntry struct {
	Source  string `yaml:"source,omitempty"`
	Version string `yaml:"version"`
}

// PluginEntry is one plugin declared in the root manifest. Source is the
// folder name under packages/plugins/. Alias lists alternative names accepted
// on the command line. Chain lists other plugins to install alongside this
// one. Target restricts which platforms the plugin installs to — even when the
// user passes --target all. Version mirrors the plugin's _meta.meta SemVer.
type PluginEntry struct {
	Source  string   `yaml:"source,omitempty"`
	Alias   []string `yaml:"alias,omitempty"`
	Chain   []string `yaml:"chain,omitempty"`
	Target  []string `yaml:"target,omitempty"`
	Version string   `yaml:"version"`
}

// Manifest declares the authoritative plugin set and remote docs. The root
// manifest.yaml is the single source of truth for plugin definitions
// (alias/chain/target) and versions; it is edited by humans and the CLI.
type Manifest struct {
	Plugins map[string]PluginEntry   `yaml:"plugins,omitempty"`
	Docs    map[string]ManifestEntry `yaml:"docs,omitempty"`
}

// ManifestPath returns the expected path of the manifest file for a given
// clone directory (the directory recorded as clone_dir in .aicorc).
func ManifestPath(cloneDir string) string {
	return filepath.Join(cloneDir, "manifest.yaml")
}

// LoadManifest reads <cloneDir>/manifest.yaml. Returns an error if the file
// does not exist — manifest is required for install/update under the
// version-tracking flow.
func LoadManifest(cloneDir string) (*Manifest, error) {
	p := ManifestPath(cloneDir)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", p, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", p, err)
	}
	return &m, nil
}

// ResolvePlugin maps a name or alias (case-insensitive) to the canonical
// plugin name and its entry. Returns ok=false when the token matches nothing.
func (m *Manifest) ResolvePlugin(token string) (string, PluginEntry, bool) {
	key := strings.ToLower(strings.TrimSpace(token))
	if key == "" {
		return "", PluginEntry{}, false
	}
	if e, ok := m.Plugins[key]; ok {
		return key, e, true
	}
	for name, e := range m.Plugins {
		if strings.ToLower(name) == key {
			return name, e, true
		}
		for _, a := range e.Alias {
			if strings.ToLower(strings.TrimSpace(a)) == key {
				return name, e, true
			}
		}
	}
	return "", PluginEntry{}, false
}

// PluginNames returns every declared plugin name, sorted.
func (m *Manifest) PluginNames() []string {
	out := make([]string, 0, len(m.Plugins))
	for n := range m.Plugins {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// PluginVersion returns the declared version for a plugin, or "" if missing.
func (m *Manifest) PluginVersion(name string) (string, bool) {
	e, ok := m.Plugins[name]
	return e.Version, ok
}

// PluginDir returns the on-disk folder for a plugin under packages/plugins/.
// It uses the entry's source folder name when set, else the plugin name.
func (m *Manifest) PluginDir(cloneDir, name string) string {
	folder := name
	if e, ok := m.Plugins[name]; ok && e.Source != "" {
		folder = e.Source
	}
	return filepath.Join(PackagesDir(cloneDir), "plugins", folder)
}

// DocVersion returns the declared version for a doc, or "" if missing.
func (m *Manifest) DocVersion(name string) (string, bool) {
	e, ok := m.Docs[name]
	return e.Version, ok
}

// PluginMetaVersion reads the SemVer from a plugin folder's _meta.meta.
// Returns "" when the file is missing or has no version field.
func PluginMetaVersion(pluginDir string) string {
	data, err := os.ReadFile(filepath.Join(pluginDir, "_meta.meta"))
	if err != nil {
		return ""
	}
	var m struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.Version
}
