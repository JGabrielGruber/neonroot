package cmd

import "github.com/spf13/cobra"

var commitCmd = &cobra.Command{
	Use:   "commit <workspace>",
	Short: "Write workspace changes back to a repo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app.UI.Warn("commit is not implemented yet (Phase 4: commit & dirty-tracking)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
