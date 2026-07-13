package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/session"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var attachCmd = &cobra.Command{
	Use:   "attach <workspace>",
	Short: "Attach to a loaded workspace's session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !workspace.IsLoaded(app.Paths, name) {
			return fmt.Errorf("%w: %q — load it first", domain.ErrWorkspaceNotFound, name)
		}
		tmuxPath, err := app.Runner.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found on PATH: %w", err)
		}
		// Hand the terminal fully over to tmux by replacing this process.
		argv := append([]string{"tmux"}, session.AttachArgs(name)...)
		return syscall.Exec(tmuxPath, argv, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
