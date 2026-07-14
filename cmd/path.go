package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var pathCmd = &cobra.Command{
	Use:   "path <workspace>",
	Short: "Print a loaded workspace's directory (for host tools/editors)",
	Long: `Prints the tmpfs directory of a loaded workspace and nothing else, so it
composes: 'cd "$(neonroot path webapp)"' or 'code "$(neonroot path webapp)"'. A
loaded workspace is a normal directory — any host editor or tool works on it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspace.ReadState(app.Paths, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), ws.Root)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pathCmd)
}
