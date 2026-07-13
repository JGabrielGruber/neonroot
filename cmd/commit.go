package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var commitMessageFlag string

var commitCmd = &cobra.Command{
	Use:   "commit <workspace>",
	Short: "Commit and push a workspace's changes back to its vault",
	Long: `Commits the workspace's current changes and pushes them to its vault. If
the vault moved ahead since you loaded (someone else committed), the push is
refused rather than overwriting — resolve it with the conflict flags.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}

		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required but was not found on PATH")
		}

		msg := commitMessageFlag
		if msg == "" {
			msg = "neonroot commit"
		}
		committed, err := g.CommitAll(cmd.Context(), ws.Root, msg)
		if err != nil {
			return err
		}

		// Push requires the vault to be reachable; check for a clear message
		// rather than an opaque git error when the drive is unplugged.
		v, err := app.resolveVault(ws.SourceVault)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}

		rejected, err := g.Push(cmd.Context(), ws.Root)
		if err != nil {
			return err
		}
		if rejected {
			return fmt.Errorf("%w: vault %q moved ahead since you loaded %q — "+
				"reload and merge, or re-run with a conflict flag (Phase E)",
				domain.ErrCommitConflict, ws.SourceVault, name)
		}

		if committed {
			app.UI.Success(fmt.Sprintf("committed and pushed %q to %q", name, ws.SourceVault))
		} else {
			app.UI.Success(fmt.Sprintf("no new changes; pushed any pending commits for %q", name))
		}
		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMessageFlag, "message", "m", "", "commit message")
	rootCmd.AddCommand(commitCmd)
}
