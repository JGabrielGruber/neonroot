// Package vault resolves vaults from cold storage: reading and writing the
// index.toml at a vault's root, reporting whether the backing drive is currently
// available, and capturing the fingerprint used later to detect whether the
// drive changed underneath a loaded workspace.
package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

// indexFile is the name of the index at a vault's root.
const indexFile = "index.toml"

// IndexPath returns the location of a vault's index.
func IndexPath(vaultPath string) string {
	return filepath.Join(vaultPath, indexFile)
}

// NewIndex returns an empty index stamped with the current schema version.
func NewIndex() *domain.Index {
	return &domain.Index{SchemaVersion: domain.SchemaVersion}
}

// ReadIndex reads and validates a vault's index. A missing index surfaces as
// fs.ErrNotExist (callers decide whether to initialize). An index declaring a
// newer schema than this build understands is rejected with
// ErrIndexVersionUnsupported rather than being mis-parsed.
func ReadIndex(vaultPath string) (*domain.Index, error) {
	var idx domain.Index
	if _, err := toml.DecodeFile(IndexPath(vaultPath), &idx); err != nil {
		return nil, err
	}
	if idx.SchemaVersion > domain.SchemaVersion {
		return nil, fmt.Errorf("%w: index is v%d, this build supports up to v%d",
			domain.ErrIndexVersionUnsupported, idx.SchemaVersion, domain.SchemaVersion)
	}
	return &idx, nil
}

// WriteIndex writes idx to the vault atomically (temp file + rename) so a crash
// or an abruptly unplugged drive never leaves a truncated index.
func WriteIndex(vaultPath string, idx *domain.Index) error {
	path := IndexPath(vaultPath)
	f, err := os.CreateTemp(vaultPath, ".index-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := toml.NewEncoder(f).Encode(idx); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
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

// Bump advances the index to a new revision and timestamps it. Called on every
// mutation (create, commit) so Fingerprint comparisons detect out-of-band
// changes to the vault.
func Bump(idx *domain.Index) {
	idx.Revision++
	idx.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// Fingerprint captures a vault's identity at a point in time for cheap
// conflict detection.
func Fingerprint(idx *domain.Index) domain.Fingerprint {
	return domain.Fingerprint{Revision: idx.Revision, UpdatedAt: idx.UpdatedAt}
}

// Workspace returns the index entry for name, if present.
func Workspace(idx *domain.Index, name string) (domain.IndexWorkspace, bool) {
	for _, w := range idx.Workspaces {
		if w.Name == name {
			return w, true
		}
	}
	return domain.IndexWorkspace{}, false
}
