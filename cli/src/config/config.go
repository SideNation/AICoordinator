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
type InstallRecord struct {
	Path    string `yaml:"path"`              // absolute install directory (project root or user home)
	Scope   string `yaml:"scope"`             // "project" | "user"
	Target  string `yaml:"target"`            // "claude" | "opencode" | "all"
	Docs    bool   `yaml:"docs"`              // whether docs were installed
	Version string `yaml:"version,omitempty"` // package repo commit hash at install time
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
