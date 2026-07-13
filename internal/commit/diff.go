// Package commit computes what changed in a hydrated workspace since it was
// loaded and writes those changes back to cold storage. It diffs the live tmpfs
// tree against the load-time manifest, detects whether the target repo changed
// underneath the workspace, and applies only the delta — it never merges.
package commit

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
)

// Diff compares the live workspace tree at root against its load-time manifest,
// returning the added, modified, and deleted files (sorted by path). The
// comparison is mtime-first: a file whose size and mtime match the manifest is
// assumed unchanged; only on a mismatch is the content hashed to confirm a real
// change, which avoids false positives from tmpfs↔drive mtime granularity.
func Diff(root string, man *domain.Manifest) ([]domain.Change, error) {
	want := make(map[string]domain.FileEntry, len(man.Files))
	for _, e := range man.Files {
		want[e.Path] = e
	}
	seen := make(map[string]bool, len(man.Files))
	var changes []domain.Change

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		isSymlink := d.Type()&fs.ModeSymlink != 0
		if !d.Type().IsRegular() && !isSymlink {
			return nil // skip sockets/devices/fifos, as hydration did
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		seen[rel] = true

		prev, known := want[rel]
		if !known {
			changes = append(changes, domain.Change{Path: rel, Kind: domain.ChangeAdded})
			return nil
		}

		if isSymlink {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if hydration.SymlinkHash(target) != prev.Hash {
				changes = append(changes, domain.Change{Path: rel, Kind: domain.ChangeModified})
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() == prev.Size && info.ModTime().UnixNano() == prev.ModTime {
			return nil // fast path: unchanged
		}
		hash, err := hydration.HashFile(path)
		if err != nil {
			return err
		}
		if hash != prev.Hash {
			changes = append(changes, domain.Change{Path: rel, Kind: domain.ChangeModified})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Anything in the manifest not seen on disk was deleted.
	for path := range want {
		if !seen[path] {
			changes = append(changes, domain.Change{Path: path, Kind: domain.ChangeDeleted})
		}
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

// HasConflict reports whether the target repo changed since the workspace was
// loaded, by comparing the repo's current fingerprint against the one captured
// at load time. A conflict means committing in place would overwrite newer
// data, so the caller must refuse unless forced or redirected to a new name.
func HasConflict(current, atLoad domain.Fingerprint) bool {
	return current.Revision != atLoad.Revision || current.UpdatedAt != atLoad.UpdatedAt
}
