package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var runCmd = &cobra.Command{
	Use:   "run <workspace> -- <command...>",
	Short: "Run a command in a loaded workspace's container (headless)",
	Long: `Runs a command inside the workspace's container and propagates its exit
code — the CI / scripting primitive. Unlike 'up' (the long-running dev server) it
takes no default command; unlike 'attach' it allocates no TTY, so output streams
cleanly into logs and pipelines. Requires a containerized, loaded workspace.

  neonroot run web -- go test ./...`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		command := args[1:]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}
		if ws.ContainerID == "" {
			return fmt.Errorf("workspace %q has no running container (load it with an image first)", name)
		}
		podmanPath, err := app.Runner.LookPath("podman")
		if err != nil {
			return fmt.Errorf("podman not found on PATH: %w", err)
		}
		pod, err := app.podman()
		if err != nil {
			return err
		}
		// Headless exec (no -it): stream stdio and let the child's exit status
		// become ours, so a failed test run fails the pipeline.
		argv := append([]string{"podman"}, podBaseArgs(pod)...)
		argv = append(argv, "exec", ws.ContainerID)
		argv = append(argv, command...)
		return syscall.Exec(podmanPath, argv, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
