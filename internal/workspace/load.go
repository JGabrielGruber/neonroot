package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/remote"
	"github.com/JGabrielGruber/neonroot/internal/ui"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

// Sessions is the host-session capability Load uses. Defining it here (the
// consumer) keeps workspace decoupled from the concrete tmux adapter, which
// satisfies this interface structurally.
type Sessions interface {
	// Ensure starts a session for the workspace rooted at dir if absent. A
	// non-empty command becomes the session's initial command (e.g. exec into a
	// container); empty means the default shell.
	Ensure(workspace, dir string, command []string) error
}

// Runtime is the optional container capability Load uses. A workspace that
// declares an image is started inside a container; one that does not (or when
// the runtime is unavailable) runs host-only.
type Runtime interface {
	Available() bool
	// EnsureImage makes sure ref is in the store, loading it from tarPath (on the
	// vault) if absent, or always when reload is set.
	EnsureImage(ctx context.Context, ref, tarPath string, reload bool) error
	// Start launches the primary container with the workspace bind-mounted at
	// mountTarget (empty = default), publishing ports, and returns its ID. opts
	// carries optional secrets extras (env-file, bind-mounts); its zero value is none.
	Start(ctx context.Context, image, name, workspaceDir, mountTarget string, ports []string, opts domain.SessionOpts) (string, error)
	// StartPod launches a pod: the primary image (refs[0]) with the workspace
	// mounted, plus sidecars sharing the network; ports are published on the pod.
	StartPod(ctx context.Context, podName string, imageRefs []string, primaryName, workspaceDir, mountTarget string, ports []string, opts domain.SessionOpts) (string, error)
}

// Git is the version-control capability Load uses to clone a workspace's bare
// repo from the vault into tmpfs and to detect pending work for safe reuse.
type Git interface {
	Clone(ctx context.Context, origin, dst string) error
	// PendingWork reports whether a loaded clone has uncommitted or unpushed work.
	PendingWork(ctx context.Context, worktree string) (bool, error)
}

// Loader clones workspaces from a vault into tmpfs.
type Loader struct {
	Paths platform.Paths
	UI    ui.Reporter
	Git   Git
	// Catalog reads the vault's index kind-agnostically (local index.toml or a
	// remote _catalog.git). Its zero value works for local vaults.
	Catalog vault.Catalog
	// Runner backs the ssh/scp transport used to fetch a remote vault's image
	// tarballs into tmpfs. Unused for local vaults.
	Runner platform.Runner
	// Sessions/Runtime start a host session / container after the clone; both
	// degrade gracefully — a failure there never fails a load.
	Sessions Sessions
	Runtime  Runtime
	// NoContainer forces host-only even when a workspace declares an image.
	NoContainer bool
	// Clean discards an already-loaded clone (uncommitted work included) and
	// re-clones fresh. Without it, an already-loaded workspace is reused.
	Clean bool
	// ReloadImage re-loads image data from the vault even if already in the store.
	ReloadImage bool
}

// Load clones the named workspace from vault v into tmpfs and records its state.
// It refuses if the vault is unreachable or the workspace is unknown. If the
// workspace is already loaded it is reused (non-destructive) unless Clean is set.
func (l *Loader) Load(v domain.Vault, name string) (*domain.Workspace, error) {
	ctx := context.Background()

	// A local vault must be mounted; a remote vault is reached lazily over ssh
	// (its unreachability surfaces on the clone below, not here).
	if !v.IsRemote() {
		state, err := vault.StateLive(v.Path)
		if err != nil {
			return nil, err
		}
		if state != domain.VaultStateAvailable {
			return nil, fmt.Errorf("%w: %q at %s — plug in the drive and retry",
				domain.ErrVaultUnavailable, v.Name, v.Path)
		}
	}

	idx, err := l.Catalog.Read(ctx, v)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: vault %q has no workspaces", domain.ErrWorkspaceNotFound, v.Name)
	}
	if err != nil {
		return nil, err
	}
	entry, ok := vault.Workspace(idx, name)
	if !ok {
		return nil, fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, name, v.Name)
	}

	dst := l.Paths.WorkspaceRoot(name)

	// Non-destructive reuse: an already-loaded workspace is kept as-is unless the
	// user explicitly asks to --clean. --clean must not silently discard pending
	// work (uncommitted OR unpushed) — warn loudly first.
	if IsLoaded(l.Paths, name) {
		if !l.Clean {
			ws, err := ReadState(l.Paths, name)
			if err != nil {
				return nil, err
			}
			l.UI.Info(fmt.Sprintf("%q is already loaded — reusing (use --clean to re-clone)", name))
			return ws, nil
		}
		if l.Git != nil {
			if pending, _ := l.Git.PendingWork(ctx, dst); pending {
				l.UI.Warn(fmt.Sprintf("--clean is discarding uncommitted or unpushed work in %q", name))
			}
		}
		_ = os.RemoveAll(dst)
		_ = os.RemoveAll(l.Paths.WorkspaceStateDir(name))
	}

	// The bare repo lives at <vault>/workspaces/<name>.git — on the local
	// filesystem, or over ssh for a remote vault. git.Clone accepts either.
	origin := filepath.Join(v.Path, entry.Root)
	if v.IsRemote() {
		addr, err := remote.Parse(v.Remote)
		if err != nil {
			return nil, err
		}
		origin = addr.SSHURL(entry.Root)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return nil, err
	}
	l.UI.Step(fmt.Sprintf("cloning %q from %q", name, v.Name))
	if err := l.Git.Clone(ctx, origin, dst); err != nil {
		_ = os.RemoveAll(dst)
		return nil, fmt.Errorf("cloning %q: %w", name, err)
	}

	ws := &domain.Workspace{
		Name:        name,
		SourceVault: v.Name,
		Root:        dst,
		HydratedAt:  time.Now().UTC().Format(time.RFC3339),
		Images:      entry.Images,
		Shell:       entry.Shell,
	}
	if err := WriteState(l.Paths, ws); err != nil {
		return nil, err
	}

	// Start a container for a workspace that declares an image. When one runs,
	// the session lives *inside* it (attach execs into the container, defaulting
	// to its tmux) — no host tmux, so container-side session saving just works.
	// If there's no container (host-only, or the container failed), fall back to
	// a host tmux session.
	containerized := false
	if len(entry.Images) > 0 && !l.NoContainer && l.Runtime != nil && l.Runtime.Available() {
		if cid, err := l.startContainer(ctx, v, entry, name, dst, domain.SessionOpts{}); err != nil {
			l.UI.Warn(fmt.Sprintf("container not started (host-only): %v", err))
		} else {
			ws.ContainerID = cid
			if len(entry.Images) > 1 {
				ws.Pod = containerName(name)
			}
			if err := WriteState(l.Paths, ws); err != nil {
				return nil, err
			}
			containerized = true
		}
	}

	if !containerized && l.Sessions != nil {
		l.UI.Step("starting session")
		if err := l.Sessions.Ensure(name, dst, nil); err != nil {
			l.UI.Warn(fmt.Sprintf("session not started (workspace is still loaded): %v", err))
		}
	}
	return ws, nil
}

// startContainer ensures the workspace's image data is loaded into the tmpfs
// store (from the vault, offline) and starts its primary container with the
// workspace bind-mounted at the configured target. Sidecar images (the rest of
// the list) are loaded too but only run as a pod in a later phase.
func (l *Loader) startContainer(ctx context.Context, v domain.Vault, entry domain.IndexWorkspace, name, dst string, opts domain.SessionOpts) (string, error) {
	refs := make([]string, len(entry.Images))
	for i, img := range entry.Images {
		l.UI.Step(fmt.Sprintf("loading image %q", img))
		refs[i] = vault.ImageRef(img)
		// Local vaults load the tar straight off the drive; remote vaults fetch it
		// into tmpfs over scp first (podman load needs a local file).
		tar := vault.ImageTarPath(v.Path, img)
		if v.IsRemote() {
			fetched, err := l.fetchRemoteImage(ctx, v, img)
			if err != nil {
				return "", err
			}
			tar = fetched
		}
		if err := l.Runtime.EnsureImage(ctx, refs[i], tar, l.ReloadImage); err != nil {
			return "", err
		}
	}
	// One image → a single container; multiple → a pod (primary + sidecars).
	if len(refs) == 1 {
		l.UI.Step(fmt.Sprintf("starting container (%s)", entry.Images[0]))
		return l.Runtime.Start(ctx, refs[0], containerName(name), dst, entry.Mount, entry.Ports, opts)
	}
	l.UI.Step(fmt.Sprintf("starting pod (%s + %d sidecar(s))", entry.Images[0], len(refs)-1))
	return l.Runtime.StartPod(ctx, containerName(name), refs, containerName(name), dst, entry.Mount, entry.Ports, opts)
}

// fetchRemoteImage downloads a remote vault's images/<img>/image.tar into a
// tmpfs cache path over scp and returns that local path for podman load.
func (l *Loader) fetchRemoteImage(ctx context.Context, v domain.Vault, img string) (string, error) {
	addr, err := remote.Parse(v.Remote)
	if err != nil {
		return "", err
	}
	dstDir := filepath.Join(l.Paths.Cache, "images", v.Name, img)
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", err
	}
	localTar := filepath.Join(dstDir, "image.tar")
	l.UI.Step(fmt.Sprintf("fetching image %q from %q", img, v.Name))
	t := remote.Transport{Runner: l.Runner, Addr: addr, Rsync: v.Rsync, Warn: l.UI.Warn}
	if err := t.Fetch(ctx, path.Join("images", img, "image.tar"), localTar); err != nil {
		return "", err
	}
	return localTar, nil
}

// containerName derives a stable container name from a workspace name, matching
// the session naming so the two are easy to correlate.
func containerName(workspace string) string { return "nr-" + workspace }
