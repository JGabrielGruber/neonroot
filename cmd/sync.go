package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Commit and push every loaded workspace with pending work",
	Long: `The "before I unplug" button: commits and pushes back every loaded
workspace that has uncommitted or unpushed changes. Clean workspaces are skipped;
a workspace whose vault is unmounted is skipped with a note; a workspace whose
vault moved ahead is reported (never force-pushed) so you can resolve it with
'neonroot commit <ws> --rebase'.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required but was not found on PATH")
		}
		reports, err := workspace.Reports(cmd.Context(), app.Paths, g)
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		var synced, conflicts, skipped, clean int
		for _, r := range reports {
			ws := r.Workspace
			switch {
			case r.Err != nil:
				app.UI.Warn(fmt.Sprintf("%s: cannot read git state: %v", ws.Name, r.Err))
				skipped++
				continue
			case !r.Status.HasPendingWork():
				clean++
				continue
			}

			v, err := app.resolveVault(ws.SourceVault)
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: %v", ws.Name, err))
				skipped++
				continue
			}
			if err := app.requireAvailable(v); err != nil {
				app.UI.Info(fmt.Sprintf("%s: vault %q not available — skipping", ws.Name, v.Name))
				skipped++
				continue
			}

			if _, err := g.CommitAll(ctx, ws.Root, "neonroot sync"); err != nil {
				app.UI.Warn(fmt.Sprintf("%s: commit failed: %v", ws.Name, err))
				skipped++
				continue
			}
			rejected, err := g.Push(ctx, ws.Root)
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: push failed: %v", ws.Name, err))
				skipped++
				continue
			}
			if rejected {
				app.UI.Warn(fmt.Sprintf("%s: vault %q moved ahead — resolve with 'neonroot commit %s --rebase'",
					ws.Name, v.Name, ws.Name))
				conflicts++
				continue
			}
			app.UI.Success(fmt.Sprintf("synced %q → %q", ws.Name, v.Name))
			synced++
		}

		app.UI.Info(fmt.Sprintf("done: %d synced, %d conflict(s), %d skipped, %d already clean",
			synced, conflicts, skipped, clean))
		if conflicts > 0 {
			return fmt.Errorf("%w: %d workspace(s) need resolution", domain.ErrCommitConflict, conflicts)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
