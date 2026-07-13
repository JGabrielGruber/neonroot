package hydration

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

// WriteManifest persists a manifest to path (atomically), creating parents.
func WriteManifest(path string, m *domain.Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := toml.NewEncoder(f).Encode(m); err != nil {
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

// ReadManifest loads a previously written manifest.
func ReadManifest(path string) (*domain.Manifest, error) {
	var m domain.Manifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
