package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <workspace> <label>",
	Short: "Save a labeled point-in-time copy of a workspace",
	Long: `Tags the workspace's current commit and pushes the tag to its vault — a
durable, immutable snapshot you can return to. (For image state, see
'neonroot image snapshot'.)`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, label := args[0], args[1]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}
		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required but was not found on PATH")
		}
		v, err := app.resolveVault(ws.SourceVault)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		if err := g.Snapshot(cmd.Context(), ws.Root, label); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("snapshot %q of workspace %q saved to vault %q", label, name, ws.SourceVault))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
