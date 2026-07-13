package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/session"
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

		// Kill the session (best-effort — tmux may be absent or already gone).
		tmux := &session.Tmux{Runner: app.Runner}
		if tmux.Available() {
			if err := tmux.Kill(name); err != nil {
				app.UI.Warn(fmt.Sprintf("could not kill session: %v", err))
			}
		}

		// Drop the tmpfs payload and bookkeeping.
		if err := os.RemoveAll(app.Paths.WorkspaceRoot(name)); err != nil {
			return err
		}
		if err := os.RemoveAll(app.Paths.WorkspaceStateDir(name)); err != nil {
			return err
		}

		app.UI.Success(fmt.Sprintf("stopped and dropped %q (uncommitted changes discarded)", name))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
