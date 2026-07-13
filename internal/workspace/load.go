package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
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
	Start(ctx context.Context, image, name, workspaceDir string) (string, error)
	ExecArgs(containerID string) []string
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
	// Sessions/Runtime start a host session / container after the clone; both
	// degrade gracefully — a failure there never fails a load.
	Sessions Sessions
	Runtime  Runtime
	// NoContainer forces host-only even when a workspace declares an image.
	NoContainer bool
	// Clean discards an already-loaded clone (uncommitted work included) and
	// re-clones fresh. Without it, an already-loaded workspace is reused.
	Clean bool
}

// Load clones the named workspace from vault v into tmpfs and records its state.
// It refuses if the vault is unreachable or the workspace is unknown. If the
// workspace is already loaded it is reused (non-destructive) unless Clean is set.
func (l *Loader) Load(v domain.Vault, name string) (*domain.Workspace, error) {
	state, err := vault.StateLive(v.Path)
	if err != nil {
		return nil, err
	}
	if state != domain.VaultStateAvailable {
		return nil, fmt.Errorf("%w: %q at %s — plug in the drive and retry",
			domain.ErrVaultUnavailable, v.Name, v.Path)
	}

	idx, err := vault.ReadIndex(v.Path)
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
	ctx := context.Background()

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

	origin := filepath.Join(v.Path, entry.Root)
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
		Image:       entry.Image,
	}
	if err := WriteState(l.Paths, ws); err != nil {
		return nil, err
	}

	// Optionally start a container for a workspace that declares an image; the
	// session then execs a shell inside it. Any failure degrades to host-only.
	var command []string
	if entry.Image != "" && !l.NoContainer && l.Runtime != nil && l.Runtime.Available() {
		l.UI.Step(fmt.Sprintf("starting container (%s)", entry.Image))
		cid, err := l.Runtime.Start(ctx, entry.Image, containerName(name), dst)
		if err != nil {
			l.UI.Warn(fmt.Sprintf("container not started (host-only): %v", err))
		} else {
			ws.ContainerID = cid
			command = l.Runtime.ExecArgs(cid)
			if err := WriteState(l.Paths, ws); err != nil {
				return nil, err
			}
		}
	}

	if l.Sessions != nil {
		l.UI.Step("starting session")
		if err := l.Sessions.Ensure(name, dst, command); err != nil {
			l.UI.Warn(fmt.Sprintf("session not started (workspace is still loaded): %v", err))
		}
	}
	return ws, nil
}

// containerName derives a stable container name from a workspace name, matching
// the session naming so the two are easy to correlate.
func containerName(workspace string) string { return "nr-" + workspace }
