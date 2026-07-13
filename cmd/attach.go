package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/session"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var attachCmd = &cobra.Command{
	Use:   "attach <workspace>",
	Short: "Attach to a loaded workspace's session",
	Long: `Attaches to the workspace's session. For a containerized workspace it
execs into the container (opening its tmux, so session saving works); for a
host-only workspace it attaches host tmux, recreating it if you had exited it.
The workspace and its container persist until 'neonroot stop', so you can always
get back in.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// ReadState fails with ErrWorkspaceNotFound if the workspace isn't loaded.
		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}

		// Containerized: exec straight into the still-running container. Its tmux
		// (via the default shell) is the durable session — no host tmux nesting.
		if ws.ContainerID != "" {
			podmanPath, err := app.Runner.LookPath("podman")
			if err != nil {
				return fmt.Errorf("podman not found on PATH: %w", err)
			}
			pod, err := app.podman()
			if err != nil {
				return err
			}
			argv := pod.ExecArgs(ws.ContainerID, ws.Shell)
			return syscall.Exec(podmanPath, argv, os.Environ())
		}

		// Host-only: recreate the host tmux session if it exited, then attach.
		tmuxPath, err := app.Runner.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found on PATH: %w", err)
		}
		tmux := &session.Tmux{Runner: app.Runner}
		if err := tmux.Ensure(name, ws.Root, nil); err != nil {
			return err
		}
		argv := append([]string{"tmux"}, session.AttachArgs(name)...)
		return syscall.Exec(tmuxPath, argv, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
