// Package hydration copies a workspace from cold storage into tmpfs and records
// a load-time manifest of everything copied. The manifest is what a later
// commit diffs against to compute exactly which files changed. Hydration is the
// slow path, so it reports steady progress through a ui.Reporter.
package hydration

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/ui"
)

// Hydrate copies src into dst, building and returning the load-time manifest.
// It pre-flights free space against the filesystem backing dst so a copy into
// RAM never dies half-way with a raw ENOSPC. File modes and mtimes are
// preserved so the freshly hydrated tree compares as unchanged.
func Hydrate(workspace, src, dst string, rep ui.Reporter) (*domain.Manifest, error) {
	total, err := TreeSize(src)
	if err != nil {
		return nil, err
	}
	// Check space on the parent (which exists) since dst is about to be created.
	if err := platform.CheckSpace(filepath.Dir(dst), uint64(total)); err != nil {
		return nil, err
	}

	rep.Step(fmt.Sprintf("hydrating %q (%s)", workspace, humanBytes(total)))

	man := &domain.Manifest{Workspace: workspace}
	var done int64

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())

		case d.Type()&fs.ModeSymlink != 0:
			entry, err := copySymlink(path, target, rel)
			if err != nil {
				return err
			}
			man.Files = append(man.Files, entry)
			return nil

		case d.Type().IsRegular():
			entry, n, err := copyFile(path, target, rel)
			if err != nil {
				return err
			}
			man.Files = append(man.Files, entry)
			done += n
			rep.Progress("copying", done, total)
			return nil

		default:
			// Sockets, devices, fifos: not meaningful in a dev workspace.
			rep.Warn(fmt.Sprintf("skipping unsupported file: %s", rel))
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	rep.Success(fmt.Sprintf("hydrated %q: %d file(s), %s", workspace, len(man.Files), humanBytes(total)))
	return man, nil
}

// TreeSize sums the bytes of all regular files under root, for the free-space
// pre-flight and progress denominator.
func TreeSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// copyFile copies a regular file, preserving mode and mtime, and returns its
// manifest entry. The content hash is computed in the same read as the copy via
// an io.MultiWriter, so hydration never reads a file twice.
func copyFile(src, dst, rel string) (domain.FileEntry, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return domain.FileEntry{}, 0, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return domain.FileEntry{}, 0, err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return domain.FileEntry{}, 0, err
	}

	h := fnv.New64a()
	n, err := io.Copy(io.MultiWriter(out, h), in)
	if err != nil {
		out.Close()
		return domain.FileEntry{}, 0, err
	}
	if err := out.Close(); err != nil {
		return domain.FileEntry{}, 0, err
	}

	mtime := info.ModTime()
	if err := os.Chtimes(dst, mtime, mtime); err != nil {
		return domain.FileEntry{}, 0, err
	}

	return domain.FileEntry{
		Path:    rel,
		Size:    n,
		ModTime: mtime.UnixNano(),
		Hash:    hex.EncodeToString(h.Sum(nil)),
	}, n, nil
}

// copySymlink recreates a symlink at dst and records its target's hash, so a
// retargeted link is detected as a change at commit time.
func copySymlink(src, dst, rel string) (domain.FileEntry, error) {
	link, err := os.Readlink(src)
	if err != nil {
		return domain.FileEntry{}, err
	}
	// Replace any existing entry so re-hydration is idempotent.
	_ = os.Remove(dst)
	if err := os.Symlink(link, dst); err != nil {
		return domain.FileEntry{}, err
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(link))
	return domain.FileEntry{
		Path: rel,
		Size: int64(len(link)),
		Hash: "link:" + hex.EncodeToString(h.Sum(nil)),
	}, nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
