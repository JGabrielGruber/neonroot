package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/remote"
)

// catalogRepo is the bare git repo at a remote vault's root that holds its
// index.toml. Using a repo (rather than a raw scp'd file) maps a concurrent
// metadata write onto git's non-fast-forward conflict — the same mechanism the
// workspace repos already use — so two devices can't silently clobber the
// catalog.
const catalogRepo = "_catalog.git"

// Cloner is the git capability the remote Catalog needs: clone a bare repo's
// default branch into a fresh directory. *git.Git satisfies it.
type Cloner interface {
	Clone(ctx context.Context, origin, dst string) error
}

// Catalog reads a vault's index kind-agnostically. A local vault's index is the
// index.toml on its drive; a remote vault's is the index.toml tracked in its
// _catalog.git repo, cloned into a tmpfs cache on demand. Callers (list, load,
// create) use this instead of ReadIndex(v.Path) so they never special-case kind.
type Catalog struct {
	// Git clones the remote catalog repo. Unused for local vaults.
	Git Cloner
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

// cloneCatalog clones the remote vault's _catalog.git into a per-vault tmpfs dir,
// replacing any previous clone, and returns the working-tree path.
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
	if err := c.Git.Clone(ctx, addr.SSHURL(catalogRepo), dst); err != nil {
		return "", fmt.Errorf("%w: remote vault %q catalog unreachable: %v",
			domain.ErrVaultUnavailable, v.Name, err)
	}
	return dst, nil
}
