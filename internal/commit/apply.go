package commit

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
)

// ApplyDiff writes the changes from the live workspace (srcRoot, in tmpfs) into
// the target workspace directory (dstRoot, on the drive): added/modified files
// are copied (preserving mode/mtime), deleted files are removed. Only the delta
// touches the drive, minimizing writes to cold storage.
func ApplyDiff(srcRoot, dstRoot string, changes []domain.Change) error {
	for _, ch := range changes {
		src := filepath.Join(srcRoot, ch.Path)
		dst := filepath.Join(dstRoot, ch.Path)

		switch ch.Kind {
		case domain.ChangeAdded, domain.ChangeModified:
			info, err := os.Lstat(src)
			if err != nil {
				return err
			}
			if info.Mode()&fs.ModeSymlink != 0 {
				if _, err := hydration.CopySymlink(src, dst); err != nil {
					return err
				}
			} else if _, err := hydration.CopyFile(src, dst); err != nil {
				return err
			}

		case domain.ChangeDeleted:
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// UpdateManifest returns a new manifest reflecting the workspace state after a
// commit: entries for added/modified files are recomputed from srcRoot and
// deleted entries dropped. Only changed files are re-hashed, so re-baselining is
// cheap. This keeps the next diff correct after an in-place commit.
func UpdateManifest(man *domain.Manifest, srcRoot string, changes []domain.Change) (*domain.Manifest, error) {
	byPath := make(map[string]domain.FileEntry, len(man.Files))
	for _, e := range man.Files {
		byPath[e.Path] = e
	}
	for _, ch := range changes {
		if ch.Kind == domain.ChangeDeleted {
			delete(byPath, ch.Path)
			continue
		}
		entry, err := hydration.EntryOf(filepath.Join(srcRoot, ch.Path), ch.Path)
		if err != nil {
			return nil, err
		}
		byPath[ch.Path] = entry
	}

	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := &domain.Manifest{Workspace: man.Workspace}
	for _, p := range paths {
		out.Files = append(out.Files, byPath[p])
	}
	return out, nil
}
