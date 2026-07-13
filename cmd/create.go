package cmd

import "github.com/spf13/cobra"

var createCmd = &cobra.Command{
	Use:   "create <workspace>",
	Short: "Create a new empty workspace in a repo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app.UI.Warn("create is not implemented yet (Phase 1: repo resolution)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
