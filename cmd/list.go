package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repos and their availability",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repos := app.Config.Repos
		if len(repos) == 0 {
			app.UI.Info("no repos configured")
			return nil
		}
		// Read the mount table once and resolve every repo against it.
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for _, r := range repos {
			state := repo.State(r.Path, mounts)
			marker := " "
			if r.Name == app.Config.DefaultRepo {
				marker = "*"
			}
			fmt.Fprintf(out, "%s %-12s %-11s %s\n", marker, r.Name, state, r.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
