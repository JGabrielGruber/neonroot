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
	"github.com/JGabrielGruber/neonroot/internal/hydration"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
	"github.com/JGabrielGruber/neonroot/internal/ui"
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
	// Available reports whether the container runtime is usable.
	Available() bool
	// Start launches a container for the workspace bind-mounted at
	// workspaceDir and returns its ID.
	Start(ctx context.Context, image, name, workspaceDir string) (string, error)
	// ExecArgs returns the session command to open a shell inside the container.
	ExecArgs(containerID string) []string
}

// Loader hydrates workspaces from repos into tmpfs.
type Loader struct {
	Paths platform.Paths
	UI    ui.Reporter
	// Sessions, if set, starts a host session for the workspace after
	// hydration. A session failure degrades gracefully — it never fails a load.
	Sessions Sessions
	// Runtime, if set, starts a container for workspaces that declare an image.
	Runtime Runtime
	// NoContainer forces host-only even when a workspace declares an image.
	NoContainer bool
}

// Load hydrates the named workspace from repo r into tmpfs and records the
// manifest and state needed to commit it back later. It refuses to run if the
// repo's drive is not mounted or if the workspace is already loaded.
func (l *Loader) Load(r domain.Repo, name string) (*domain.Workspace, error) {
	// The drive must be reachable to read from.
	state, err := repo.StateLive(r.Path)
	if err != nil {
		return nil, err
	}
	if state != domain.RepoStateAvailable {
		return nil, fmt.Errorf("%w: %q at %s — plug in the drive and retry",
			domain.ErrRepoUnavailable, r.Name, r.Path)
	}

	idx, err := repo.ReadIndex(r.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: repo %q has no workspaces", domain.ErrWorkspaceNotFound, r.Name)
	}
	if err != nil {
		return nil, err
	}
	entry, ok := repo.Workspace(idx, name)
	if !ok {
		return nil, fmt.Errorf("%w: %q in repo %q", domain.ErrWorkspaceNotFound, name, r.Name)
	}

	if IsLoaded(l.Paths, name) {
		return nil, fmt.Errorf("%w: %q (commit or drop it first)", domain.ErrWorkspaceExists, name)
	}

	src := filepath.Join(r.Path, entry.Root)
	dst := l.Paths.WorkspaceRoot(name)

	// A leftover payload without a state file: clear it so hydration is clean.
	_ = os.RemoveAll(dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return nil, err
	}

	man, err := hydration.Hydrate(name, src, dst, l.UI)
	if err != nil {
		// Roll back a partial payload so a failed load leaves no half-state.
		_ = os.RemoveAll(dst)
		return nil, err
	}

	if err := hydration.WriteManifest(l.Paths.ManifestPath(name), man); err != nil {
		return nil, err
	}

	ws := &domain.Workspace{
		Name:              name,
		SourceRepo:        r.Name,
		Root:              dst,
		HydratedAt:        time.Now().UTC().Format(time.RFC3339),
		SourceFingerprint: repo.Fingerprint(idx),
		Image:             entry.Image,
	}
	if err := WriteState(l.Paths, ws); err != nil {
		return nil, err
	}

	// Optionally start a container for a workspace that declares an image. The
	// session then execs a shell inside it; otherwise the session is host-only.
	// Any container failure degrades to host-only — it never fails the load.
	var command []string
	if entry.Image != "" && !l.NoContainer && l.Runtime != nil && l.Runtime.Available() {
		l.UI.Step(fmt.Sprintf("starting container (%s)", entry.Image))
		cid, err := l.Runtime.Start(context.Background(), entry.Image, containerName(name), dst)
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

	// Start a host session so the user can attach immediately. Graceful
	// degradation: if tmux is missing or errors, the workspace is still loaded.
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
