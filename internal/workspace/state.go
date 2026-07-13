// Package workspace orchestrates the load use case — resolving a vault, checking
// availability, hydrating into tmpfs, and recording the manifest plus the state
// needed to commit changes back later. It also tracks which workspaces are
// currently loaded.
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// WriteState persists a loaded workspace's record to its tmpfs state file.
func WriteState(paths platform.Paths, ws *domain.Workspace) error {
	path := paths.StatePath(ws.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := toml.NewEncoder(f).Encode(ws); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// ReadState loads a workspace's state record. A missing record surfaces as
// domain.ErrWorkspaceNotFound.
func ReadState(paths platform.Paths, name string) (*domain.Workspace, error) {
	var ws domain.Workspace
	if _, err := toml.DecodeFile(paths.StatePath(name), &ws); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.ErrWorkspaceNotFound
		}
		return nil, err
	}
	return &ws, nil
}

// IsLoaded reports whether a workspace is currently hydrated.
func IsLoaded(paths platform.Paths, name string) bool {
	_, err := os.Stat(paths.StatePath(name))
	return err == nil
}

// List returns every currently loaded workspace.
func List(paths platform.Paths) ([]domain.Workspace, error) {
	dir := filepath.Join(paths.Runtime, "workspaces")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []domain.Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ws, err := ReadState(paths, e.Name())
		if errors.Is(err, domain.ErrWorkspaceNotFound) {
			continue // a stale dir without a state file
		}
		if err != nil {
			return nil, err
		}
		out = append(out, *ws)
	}
	return out, nil
}
