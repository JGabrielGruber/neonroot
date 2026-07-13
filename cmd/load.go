package cmd

import "github.com/spf13/cobra"

var loadCmd = &cobra.Command{
	Use:   "load <workspace>",
	Short: "Hydrate a workspace from a repo into tmpfs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app.UI.Warn("load is not implemented yet (Phase 2: hydration)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}
