package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var (
	commitMessageFlag string
	commitAsFlag      string
	commitRebaseFlag  bool
	commitMergeFlag   bool
	commitForceFlag   bool
)

var commitCmd = &cobra.Command{
	Use:   "commit <workspace>",
	Short: "Commit and push a workspace's changes back to its vault",
	Long: `Commits the workspace's current changes and pushes them to its vault.
If the vault moved ahead since you loaded (someone else committed), the push is
refused rather than overwriting. Resolve with:
  --rebase / --merge   integrate the vault's changes, then push
  --as <name>          save your work to a new branch instead (no conflict)
  --force              overwrite, but only if nobody pushed since (force-with-lease)`,
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

		v, err := app.resolveVault(ws.SourceVault)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		ctx := cmd.Context()

		// --as: save to a new branch, sidestepping any conflict entirely.
		if commitAsFlag != "" {
			if err := g.PushBranch(ctx, ws.Root, commitAsFlag); err != nil {
				return err
			}
			app.UI.Success(fmt.Sprintf("saved %q to branch %q in vault %q", name, commitAsFlag, ws.SourceVault))
			return nil
		}

		rejected, err := g.Push(ctx, ws.Root)
		if err != nil {
			return err
		}
		if rejected {
			if err := resolveConflict(ctx, g, ws, name); err != nil {
				return err
			}
		}

		if committed {
			app.UI.Success(fmt.Sprintf("committed and pushed %q to %q", name, ws.SourceVault))
		} else {
			app.UI.Success(fmt.Sprintf("pushed pending commits for %q to %q", name, ws.SourceVault))
		}
		return nil
	},
}

// resolveConflict handles a rejected push per the chosen flag. Default (no flag)
// refuses, keeping the non-destructive invariant.
func resolveConflict(ctx context.Context, g *git.Git, ws *domain.Workspace, name string) error {
	switch {
	case commitForceFlag:
		rejected, err := g.PushForceWithLease(ctx, ws.Root)
		if err != nil {
			return err
		}
		if rejected {
			return fmt.Errorf("%w: someone pushed to %q since you last synced — "+
				"reload and merge instead of forcing", domain.ErrCommitConflict, ws.SourceVault)
		}
		return nil

	case commitRebaseFlag || commitMergeFlag:
		app.UI.Step("integrating the vault's changes")
		conflicted, err := g.Pull(ctx, ws.Root, commitRebaseFlag)
		if err != nil {
			return err
		}
		if conflicted {
			return fmt.Errorf("%w: merge conflicts in %q — resolve them in the workspace "+
				"(attach and fix the files), then run commit again", domain.ErrCommitConflict, name)
		}
		if rejected, err := g.Push(ctx, ws.Root); err != nil {
			return err
		} else if rejected {
			return fmt.Errorf("%w: vault %q moved again during resolution — retry",
				domain.ErrCommitConflict, ws.SourceVault)
		}
		return nil

	default:
		return fmt.Errorf("%w: vault %q moved ahead since you loaded %q — "+
			"re-run with --rebase (replay your work on top), --merge, "+
			"--as <name> (save a copy), or --force",
			domain.ErrCommitConflict, ws.SourceVault, name)
	}
}

func init() {
	commitCmd.Flags().StringVarP(&commitMessageFlag, "message", "m", "", "commit message")
	commitCmd.Flags().StringVar(&commitAsFlag, "as", "", "push to a new branch instead of committing in place")
	commitCmd.Flags().BoolVar(&commitRebaseFlag, "rebase", false, "on conflict, rebase your work onto the vault's, then push")
	commitCmd.Flags().BoolVar(&commitMergeFlag, "merge", false, "on conflict, merge the vault's changes, then push")
	commitCmd.Flags().BoolVar(&commitForceFlag, "force", false, "on conflict, overwrite (force-with-lease) if nobody pushed since")
	rootCmd.AddCommand(commitCmd)
}
