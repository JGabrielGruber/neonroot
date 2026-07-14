package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/remote"
)

// catalogRepo is the bare git repo at a remote vault's root that holds its
// index.toml. Using a repo (rather than a raw scp'd file) maps a concurrent
// metadata write onto git's non-fast-forward conflict — the same mechanism the
// workspace repos already use — so two devices can't silently clobber the
// catalog.
const catalogRepo = "_catalog.git"

// CatalogGit is the git capability the remote Catalog needs: clone the catalog
// repo (tolerating an empty one), commit the index, and push it (surfacing a
// non-fast-forward as rejected). *git.Git satisfies it.
type CatalogGit interface {
	CloneCatalog(ctx context.Context, origin, dst string) error
	CommitAll(ctx context.Context, worktree, msg string) (committed bool, err error)
	Push(ctx context.Context, worktree string) (rejected bool, err error)
}

// Catalog reads and writes a vault's index kind-agnostically. A local vault's
// index is the index.toml on its drive; a remote vault's is the index.toml
// tracked in its _catalog.git repo, cloned into a tmpfs cache on demand. Callers
// (list, load, create, image) use this instead of ReadIndex/WriteIndex(v.Path)
// so they never special-case kind.
type Catalog struct {
	// Git clones/commits/pushes the remote catalog repo. Unused for local vaults.
	Git CatalogGit
	// Runner backs the ssh transport that lazily initializes the remote catalog
	// repo on first write. Unused for local vaults.
	Runner platform.Runner
	// CacheDir is the tmpfs base under which remote catalogs are cloned.
	CacheDir string
}

// Read returns a vault's index. For a remote vault a fresh clone of _catalog.git
// is made under CacheDir (the catalog is tiny, so clone-per-read keeps one code
// path); an unreachable remote surfaces as ErrVaultUnavailable.
func (c Catalog) Read(ctx context.Context, v domain.Vault) (*domain.Index, error) {
	if !v.IsRemote() {
		return ReadIndex(v.Path)
	}
	dir, err := c.cloneCatalog(ctx, v)
	if err != nil {
		return nil, err
	}
	return ReadIndex(dir)
}

// Write persists idx to the vault. A local vault writes index.toml atomically. A
// remote vault ensures _catalog.git exists (lazily, idempotently), commits the
// index into it, and pushes — so a concurrent writer's push is rejected
// (ErrCommitConflict) rather than silently clobbering the catalog.
func (c Catalog) Write(ctx context.Context, v domain.Vault, idx *domain.Index) error {
	if !v.IsRemote() {
		return WriteIndex(v.Path, idx)
	}
	addr, err := remote.Parse(v.Remote)
	if err != nil {
		return err
	}
	t := remote.Transport{Runner: c.Runner, Addr: addr}
	if err := t.InitBare(ctx, catalogRepo); err != nil {
		return fmt.Errorf("%w: preparing remote catalog for %q: %v",
			domain.ErrVaultUnavailable, v.Name, err)
	}
	dir, err := c.cloneCatalog(ctx, v)
	if err != nil {
		return err
	}
	if err := WriteIndex(dir, idx); err != nil {
		return err
	}
	committed, err := c.Git.CommitAll(ctx, dir, "neonroot: update catalog")
	if err != nil {
		return err
	}
	if !committed {
		return nil
	}
	rejected, err := c.Git.Push(ctx, dir)
	if err != nil {
		return err
	}
	if rejected {
		return fmt.Errorf("%w: remote vault %q catalog moved ahead since you read it — retry",
			domain.ErrCommitConflict, v.Name)
	}
	return nil
}

// cloneCatalog clones the remote vault's _catalog.git into a per-vault tmpfs dir,
// replacing any previous clone, and returns the working-tree path. A plain clone
// tolerates an empty (freshly-init'd) repo, whose empty working tree ReadIndex
// then reports as fs.ErrNotExist (an unpopulated catalog).
func (c Catalog) cloneCatalog(ctx context.Context, v domain.Vault) (string, error) {
	addr, err := remote.Parse(v.Remote)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(c.CacheDir, "catalog", v.Name)
	_ = os.RemoveAll(dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	if err := c.Git.CloneCatalog(ctx, addr.SSHURL(catalogRepo), dst); err != nil {
		return "", fmt.Errorf("%w: remote vault %q catalog unreachable: %v",
			domain.ErrVaultUnavailable, v.Name, err)
	}
	return dst, nil
}
