package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var logsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs <workspace>",
	Short: "Show a loaded workspace's container (or pod) logs",
	Long: `Streams the workspace's container logs, or its pod logs when it runs
sidecars — for debugging a container that misbehaves or a sidecar service (e.g.
postgres/redis). Requires a loaded, containerized workspace.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}
		if ws.ContainerID == "" && ws.Pod == "" {
			return fmt.Errorf("workspace %q has no running container to show logs for", name)
		}
		podmanPath, err := app.Runner.LookPath("podman")
		if err != nil {
			return fmt.Errorf("podman not found on PATH: %w", err)
		}
		pod, err := app.podman()
		if err != nil {
			return err
		}

		argv := append([]string{"podman"}, podBaseArgs(pod)...)
		target := ws.ContainerID
		if ws.Pod != "" {
			argv = append(argv, "pod", "logs") // covers the primary + every sidecar
			target = ws.Pod
		} else {
			argv = append(argv, "logs")
		}
		if logsFollow {
			argv = append(argv, "-f")
		}
		argv = append(argv, target)
		return syscall.Exec(podmanPath, argv, os.Environ())
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream new log output")
	rootCmd.AddCommand(logsCmd)
}
