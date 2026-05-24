// Package config manages the user-global aico configuration under ~/.aico/.
//
// Layout:
//   ~/.aico/
//     .aicorc   — YAML: init_dir, clone_dir, git_url
//     .lock     — YAML: list of install records
//     packages/ — git-cloned package repo (contains packages/ subdir)
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Rc struct {
	InitDir  string `yaml:"init_dir"`            // directory where `aico init` was run
	CloneDir string `yaml:"clone_dir"`           // absolute path to cloned package repo
	GitURL   string `yaml:"git_url,omitempty"`   // git remote url used for clone
}

// InstallRecord describes one install destination tracked in .lock.
// Agents, Skills, Docs, and Rules map each installed item's name to the
// manifest version it was installed from. Empty version means "installed
// before manifest-based tracking existed."
type InstallRecord struct {
	Path    string            `yaml:"path"`              // absolute install directory (project root or user home)
	Scope   string            `yaml:"scope"`             // "project" | "user"
	Target  string            `yaml:"target"`            // "claude" | "opencode" | "all"
	Agents  map[string]string `yaml:"agents,omitempty"`  // agent name -> manifest version
	Skills  map[string]string `yaml:"skills,omitempty"`  // skill name -> manifest version
	Docs    map[string]string `yaml:"docs,omitempty"`    // doc name -> manifest version
	Rules   map[string]string `yaml:"rules,omitempty"`   // rule name -> manifest version
	Version string            `yaml:"version,omitempty"` // package repo commit hash at install time
}

// UnmarshalYAML accepts legacy forms of `docs`:
//   - `docs: true/false` (ancient bool form)
//   - `docs: [name, ...]` (list form)
// and the current map form `docs: {name: version}`. Legacy entries are
// migrated to the map with an empty version string.
func (r *InstallRecord) UnmarshalYAML(node *yaml.Node) error {
	type rawMap struct {
		Path    string            `yaml:"path"`
		Scope   string            `yaml:"scope"`
		Target  string            `yaml:"target"`
		Agents  map[string]string `yaml:"agents,omitempty"`
		Skills  map[string]string `yaml:"skills,omitempty"`
		Docs    map[string]string `yaml:"docs,omitempty"`
		Rules   map[string]string `yaml:"rules,omitempty"`
		Version string            `yaml:"version,omitempty"`
	}
	type rawList struct {
		Path    string   `yaml:"path"`
		Scope   string   `yaml:"scope"`
		Target  string   `yaml:"target"`
		Docs    []string `yaml:"docs,omitempty"`
		Version string   `yaml:"version,omitempty"`
	}
	type rawBool struct {
		Path    string `yaml:"path"`
		Scope   string `yaml:"scope"`
		Target  string `yaml:"target"`
		Docs    bool   `yaml:"docs"`
		Version string `yaml:"version,omitempty"`
	}

	var m rawMap
	if err := node.Decode(&m); err == nil {
		r.Path, r.Scope, r.Target = m.Path, m.Scope, m.Target
		r.Agents, r.Skills, r.Docs, r.Rules = m.Agents, m.Skills, m.Docs, m.Rules
		r.Version = m.Version
		return nil
	}
	var list rawList
	if err := node.Decode(&list); err == nil {
		r.Path, r.Scope, r.Target, r.Version = list.Path, list.Scope, list.Target, list.Version
		if list.Docs != nil {
			r.Docs = map[string]string{}
			for _, name := range list.Docs {
				r.Docs[name] = ""
			}
		}
		return nil
	}
	var b rawBool
	if err := node.Decode(&b); err != nil {
		return err
	}
	r.Path, r.Scope, r.Target, r.Version = b.Path, b.Scope, b.Target, b.Version
	if b.Docs {
		r.Docs = map[string]string{}
	}
	return nil
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

// ManifestEntry holds the declared version of a single agent/skill/doc.
// Source is optional — when set, it overrides the default packages/<kind>/<name>
// lookup path. Relative source paths are resolved against the clone root.
type ManifestEntry struct {
	Source  string `yaml:"source,omitempty"`
	Version string `yaml:"version"`
}

// Manifest declares the authoritative version for each packaged item.
// Lives at <clone_dir>/manifest.yaml and is edited by humans.
type Manifest struct {
	Agents map[string]ManifestEntry `yaml:"agents,omitempty"`
	Skills map[string]ManifestEntry `yaml:"skills,omitempty"`
	Docs   map[string]ManifestEntry `yaml:"docs,omitempty"`
	Rules  map[string]ManifestEntry `yaml:"rules,omitempty"`
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

// AgentVersion returns the declared version for an agent, or "" if missing.
func (m *Manifest) AgentVersion(name string) (string, bool) {
	e, ok := m.Agents[name]
	return e.Version, ok
}

// AgentSource returns the resolved on-disk path of the single-source agent
// markdown for `name`, given the clone root. When manifest.source is set it
// wins; otherwise we default to <cloneDir>/packages/agents/<name>.md.
// Relative manifest.source values are resolved against cloneDir.
func (m *Manifest) AgentSource(cloneDir, name string) string {
	if e, ok := m.Agents[name]; ok && e.Source != "" {
		if filepath.IsAbs(e.Source) {
			return e.Source
		}
		return filepath.Join(cloneDir, e.Source)
	}
	return filepath.Join(PackagesDir(cloneDir), "agents", name+".md")
}

// SkillVersion returns the declared version for a skill, or "" if missing.
func (m *Manifest) SkillVersion(name string) (string, bool) {
	e, ok := m.Skills[name]
	return e.Version, ok
}

// DocVersion returns the declared version for a doc, or "" if missing.
func (m *Manifest) DocVersion(name string) (string, bool) {
	e, ok := m.Docs[name]
	return e.Version, ok
}

// RuleVersion returns the declared version for a rule, or "" if missing.
func (m *Manifest) RuleVersion(name string) (string, bool) {
	e, ok := m.Rules[name]
	return e.Version, ok
}
