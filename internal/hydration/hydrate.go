// Package hydration copies a workspace from cold storage into tmpfs and records
// a load-time manifest of everything copied. The manifest is what a later
// commit diffs against to compute exactly which files changed. Hydration is the
// slow path, so it reports steady progress through a ui.Reporter.
//
// The file-identity helpers (HashFile, SymlinkHash, EntryOf) and copy helpers
// (CopyFile, CopySymlink) are shared with the commit package so hydration and
// commit compute identity the same way — a divergence would corrupt diffs.
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

// symlinkPrefix distinguishes a symlink's target hash from a regular file hash
// in a manifest entry, so a file and a symlink with coincidentally equal hashes
// never compare equal.
const symlinkPrefix = "link:"

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
			entry, err := CopySymlink(path, target)
			if err != nil {
				return err
			}
			entry.Path = rel
			man.Files = append(man.Files, entry)
			return nil

		case d.Type().IsRegular():
			entry, err := CopyFile(path, target)
			if err != nil {
				return err
			}
			entry.Path = rel
			man.Files = append(man.Files, entry)
			done += entry.Size
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

// HashFile returns the fast non-crypto content hash of a regular file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := fnv.New64a()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SymlinkHash returns the manifest hash for a symlink with the given target.
func SymlinkHash(target string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(target))
	return symlinkPrefix + hex.EncodeToString(h.Sum(nil))
}

// EntryOf computes a manifest entry for the file at abs (regular or symlink)
// without copying it. rel is the returned entry's Path.
func EntryOf(abs, rel string) (domain.FileEntry, error) {
	info, err := os.Lstat(abs)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(abs)
		if err != nil {
			return domain.FileEntry{}, err
		}
		return domain.FileEntry{Path: rel, Size: int64(len(target)), Hash: SymlinkHash(target)}, nil
	}
	hash, err := HashFile(abs)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return domain.FileEntry{
		Path:    rel,
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
		Hash:    hash,
	}, nil
}

// CopyFile copies a regular file, preserving mode and mtime, and returns its
// manifest entry (Path unset — caller sets it). The content hash is computed in
// the same read as the copy via an io.MultiWriter, so hydration never reads a
// file twice.
func CopyFile(src, dst string) (domain.FileEntry, error) {
	in, err := os.Open(src)
	if err != nil {
		return domain.FileEntry{}, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return domain.FileEntry{}, err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return domain.FileEntry{}, err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return domain.FileEntry{}, err
	}

	h := fnv.New64a()
	n, err := io.Copy(io.MultiWriter(out, h), in)
	if err != nil {
		out.Close()
		return domain.FileEntry{}, err
	}
	if err := out.Close(); err != nil {
		return domain.FileEntry{}, err
	}

	mtime := info.ModTime()
	if err := os.Chtimes(dst, mtime, mtime); err != nil {
		return domain.FileEntry{}, err
	}

	return domain.FileEntry{
		Size:    n,
		ModTime: mtime.UnixNano(),
		Hash:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// CopySymlink recreates a symlink at dst and returns its manifest entry (Path
// unset — caller sets it).
func CopySymlink(src, dst string) (domain.FileEntry, error) {
	target, err := os.Readlink(src)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return domain.FileEntry{}, err
	}
	_ = os.Remove(dst) // idempotent re-create
	if err := os.Symlink(target, dst); err != nil {
		return domain.FileEntry{}, err
	}
	return domain.FileEntry{Size: int64(len(target)), Hash: SymlinkHash(target)}, nil
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
