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
	Long: `Attaches to the workspace's tmux session, recreating it if it's gone (e.g.
after you exited the shell with Ctrl-D). The workspace and its container persist
until 'neonroot stop', so you can always get back in.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// ReadState fails with ErrWorkspaceNotFound if the workspace isn't loaded.
		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}
		tmuxPath, err := app.Runner.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found on PATH: %w", err)
		}

		// Recreate the session if it exited. The container (started with
		// `sleep infinity`) is still alive, so we just re-exec a shell into it;
		// a host-only workspace gets a fresh shell in its directory.
		var command []string
		if ws.ContainerID != "" {
			pod, err := app.podman()
			if err != nil {
				return err
			}
			command = pod.ExecArgs(ws.ContainerID)
		}
		tmux := &session.Tmux{Runner: app.Runner}
		if err := tmux.Ensure(name, ws.Root, command); err != nil {
			return err
		}

		// Hand the terminal fully over to tmux by replacing this process.
		argv := append([]string{"tmux"}, session.AttachArgs(name)...)
		return syscall.Exec(tmuxPath, argv, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
