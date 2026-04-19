package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RenderClaude serialises a ClaudeOut + body to markdown bytes.
func RenderClaude(out *ClaudeOut, body string) ([]byte, error) {
	return renderMarkdown(out, body)
}

// RenderOpencode serialises an OpencodeOut + body to markdown bytes.
func RenderOpencode(out *OpencodeOut, body string) ([]byte, error) {
	return renderMarkdown(out, body)
}

func renderMarkdown(front interface{}, body string) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("---\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(front); err != nil {
		return nil, fmt.Errorf("yaml marshal error: %w", err)
	}
	enc.Close()

	buf.WriteString("---\n")
	if body != "" {
		buf.WriteByte('\n')
		buf.WriteString(body)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

// WriteFile writes data to path, creating directories as needed.
// If force=false, it skips writing when the destination is newer than mtime.
func WriteFile(path string, data []byte, srcMtime int64, force bool) (wrote bool, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	if !force {
		if info, err := os.Stat(path); err == nil {
			if info.ModTime().Unix() >= srcMtime {
				return false, nil // up-to-date
			}
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
