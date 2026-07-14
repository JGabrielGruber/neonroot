package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var upCmd = &cobra.Command{
	Use:   "up <workspace> [-- command...]",
	Short: "Run the dev command inside a loaded workspace's container",
	Long: `Runs a command inside the workspace's running container — typically the dev
server. Pass it after '--', or set a default with 'create --up "<cmd>"'. With
published ports (create --port), the server is reachable at localhost on the host.
Requires a containerized, loaded workspace.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		override := args[1:]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}
		if ws.ContainerID == "" {
			return fmt.Errorf("workspace %q has no running container (load it with an image first)", name)
		}

		command := override
		if len(command) == 0 {
			// Fall back to the workspace's declared up command.
			v, err := app.resolveVault(ws.SourceVault)
			if err != nil {
				return err
			}
			if idx, err := vault.ReadIndex(v.Path); err == nil {
				if entry, ok := vault.Workspace(idx, name); ok {
					command = entry.Up
				}
			}
		}
		if len(command) == 0 {
			return fmt.Errorf("no command: pass one after '--', or set 'create --up \"<cmd>\"'")
		}

		podmanPath, err := app.Runner.LookPath("podman")
		if err != nil {
			return fmt.Errorf("podman not found on PATH: %w", err)
		}
		pod, err := app.podman()
		if err != nil {
			return err
		}
		// Hand the terminal over: the dev server runs in the foreground.
		argv := pod.ExecArgs(ws.ContainerID, command)
		return syscall.Exec(podmanPath, argv, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
