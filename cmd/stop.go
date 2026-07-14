package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var stopCmd = &cobra.Command{
	Use:   "stop <workspace>",
	Short: "Stop a workspace's session and drop its tmpfs copy",
	Long: `Kills the workspace's tmux session and removes its hydrated copy from
tmpfs. This DISCARDS any uncommitted changes — commit first if you want to keep
them.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !workspace.IsLoaded(app.Paths, name) {
			return fmt.Errorf("%w: %q is not loaded", domain.ErrWorkspaceNotFound, name)
		}

		lock, err := app.lock("ws-" + name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		if err := stopWorkspace(cmd.Context(), name); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("stopped and dropped %q (uncommitted changes discarded)", name))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
