package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var reapAllFlag bool

var reapCmd = &cobra.Command{
	Use:   "reap [workspace...]",
	Short: "Stop and delete loaded workspaces (fleet cleanup)",
	Long: `Tears down and removes loaded workspaces — the bulk cleanup for agent /
throwaway boxes (stop + rm in one go). Name them explicitly, or --all to reap
every currently-loaded workspace.

This is destructive: uncommitted work and each workspace's repo are removed.
Commit anything worth keeping first ('neonroot commit <ws> --as <branch>').`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names := args
		if reapAllFlag {
			loaded, err := workspace.List(app.Paths)
			if err != nil {
				return err
			}
			names = names[:0]
			for _, ws := range loaded {
				names = append(names, ws.Name)
			}
		}
		if len(names) == 0 {
			return fmt.Errorf("name a workspace to reap, or pass --all")
		}

		var reaped int
		for _, name := range names {
			ws, err := workspace.ReadState(app.Paths, name)
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: not loaded — skipping", name))
				continue
			}
			target, err := app.resolveVault(ws.SourceVault)
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: %v", name, err))
				continue
			}
			reap(cmd.Context(), target, name, false)
			reaped++
		}
		app.UI.Success(fmt.Sprintf("reaped %d workspace(s)", reaped))
		return nil
	},
}

func init() {
	reapCmd.Flags().BoolVar(&reapAllFlag, "all", false, "reap every currently-loaded workspace")
	rootCmd.AddCommand(reapCmd)
}
