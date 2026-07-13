// Package template seeds new workspaces with starter content. It ships a
// default skeleton embedded in the binary; callers may instead seed from an
// existing workspace (handled by the caller copying that workspace's files).
package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The `all:` prefix is required so dotfiles like .gitignore are embedded.
//
//go:embed all:files/default
var defaultFS embed.FS

const defaultRoot = "files/default"

// WriteDefault writes the shipped default template into dstDir, substituting
// the {{workspace}} placeholder with name in file contents.
func WriteDefault(dstDir, name string) error {
	return fs.WalkDir(defaultFS, defaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(defaultRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := defaultFS.ReadFile(path)
		if err != nil {
			return err
		}
		data = []byte(strings.ReplaceAll(string(data), "{{workspace}}", name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
