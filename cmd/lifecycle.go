package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/session"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

// stopWorkspace tears down a loaded workspace's runtime: its tmux session, its
// container/pod, and its tmpfs payload + state. Best-effort on the session and
// container (a missing piece warns, not fails); shared by `stop` and `spawn`.
func stopWorkspace(ctx context.Context, name string) error {
	ws, err := workspace.ReadState(app.Paths, name)
	if err != nil {
		return err
	}
	tmux := &session.Tmux{Runner: app.Runner}
	if tmux.Available() {
		if err := tmux.Kill(name); err != nil {
			app.UI.Warn(fmt.Sprintf("could not kill session: %v", err))
		}
	}
	if ws.Pod != "" || ws.ContainerID != "" {
		pod, err := app.podman()
		if err != nil {
			return err
		}
		if ws.Pod != "" {
			if err := pod.StopPod(ctx, ws.Pod); err != nil {
				app.UI.Warn(fmt.Sprintf("could not stop pod: %v", err))
			}
		} else if err := pod.Stop(ctx, ws.ContainerID); err != nil {
			app.UI.Warn(fmt.Sprintf("could not stop container: %v", err))
		}
	}
	if err := os.RemoveAll(app.Paths.WorkspaceRoot(name)); err != nil {
		return err
	}
	return os.RemoveAll(app.Paths.WorkspaceStateDir(name))
}

// removeWorkspace deletes a workspace from a local vault: its bare repo and its
// catalog entry. Shared by `rm` and `spawn` (reap). Remote vaults are not
// supported here.
func removeWorkspace(v domain.Vault, name string) error {
	if v.IsRemote() {
		return fmt.Errorf("removing a workspace from a remote vault is not supported")
	}
	idx, err := vault.ReadIndex(v.Path)
	if err != nil {
		return err
	}
	entry, ok := vault.Workspace(idx, name)
	if !ok {
		return fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, name, v.Name)
	}
	if err := os.RemoveAll(filepath.Join(v.Path, entry.Root)); err != nil {
		return err
	}
	kept := idx.Workspaces[:0:0]
	for _, w := range idx.Workspaces {
		if w.Name != name {
			kept = append(kept, w)
		}
	}
	idx.Workspaces = kept
	vault.Bump(idx)
	return vault.WriteIndex(v.Path, idx)
}
