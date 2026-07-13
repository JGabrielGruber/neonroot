package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repos",
	Long:  "Lists the repos registered in config, including the built-in scratch repo.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repos := app.Config.Repos
		if len(repos) == 0 {
			app.UI.Info("no repos configured")
			return nil
		}
		// Availability resolution lands in Phase 1; for now list name → path.
		out := cmd.OutOrStdout()
		for _, r := range repos {
			fmt.Fprintf(out, "%-12s %s\n", r.Name, r.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
