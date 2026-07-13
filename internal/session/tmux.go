// Package session manages host-side tmux sessions for workspaces. tmux runs on
// the host (not in the container or on the drive), so a session survives the
// external drive being unplugged — you keep working in the same session after
// pulling the USB.
package session

import (
	"context"
	"errors"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// namePrefix keeps NeonRoot's sessions from colliding with a user's own tmux
// sessions.
const namePrefix = "nr-"

// Name returns the tmux session name for a workspace.
func Name(workspace string) string { return namePrefix + workspace }

// AttachArgs returns the tmux arguments to attach to a workspace's session.
// Attaching hands the terminal over, so callers run tmux directly with
// inherited stdio rather than through platform.Runner.
func AttachArgs(workspace string) []string {
	return []string{"attach-session", "-t", Name(workspace)}
}

// Tmux drives the tmux binary through a platform.Runner, which makes it
// unit-testable without a real tmux server.
type Tmux struct {
	Runner platform.Runner
}

// Available reports whether tmux is on PATH. When it isn't, callers degrade
// gracefully rather than failing the operation.
func (t *Tmux) Available() bool {
	_, err := t.Runner.LookPath("tmux")
	return err == nil
}

// Exists reports whether a workspace's session is currently running. A non-zero
// exit from `tmux has-session` means "no such session", which is not an error.
func (t *Tmux) Exists(workspace string) (bool, error) {
	if _, err := t.Runner.Run(context.Background(), "tmux", "has-session", "-t", Name(workspace)); err != nil {
		var runErr *platform.RunError
		if errors.As(err, &runErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Ensure starts a detached session for the workspace rooted at dir if one is
// not already running. Idempotent.
func (t *Tmux) Ensure(workspace, dir string) error {
	ok, err := t.Exists(workspace)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = t.Runner.Run(context.Background(),
		"tmux", "new-session", "-d", "-s", Name(workspace), "-c", dir)
	return err
}

// Kill terminates a workspace's session if present.
func (t *Tmux) Kill(workspace string) error {
	ok, err := t.Exists(workspace)
	if err != nil || !ok {
		return err
	}
	_, err = t.Runner.Run(context.Background(), "tmux", "kill-session", "-t", Name(workspace))
	return err
}
