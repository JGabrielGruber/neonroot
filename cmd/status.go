package cmd

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:   "status [workspace]",
	Short: "Show loaded workspaces and their pending changes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app.UI.Warn("status is not implemented yet (Phase 1/4)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
