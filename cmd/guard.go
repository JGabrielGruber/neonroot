package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var guardCmd = &cobra.Command{
	Use:   "guard [vault]",
	Short: "Exit 0 only if it's safe to unplug (no unsynced work)",
	Long: `A scriptable unplug gate. Exits 0 when no loaded workspace has unsynced
work (uncommitted or unpushed), and non-zero otherwise — naming what blocks it.
Pass a vault to check only workspaces loaded from it. Wire it into an eject/udev
hook: 'neonroot guard ext && umount /mnt/ext'.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var only string
		if len(args) == 1 {
			only = args[0]
		}

		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required but was not found on PATH")
		}
		reports, err := workspace.Reports(cmd.Context(), app.Paths, g)
		if err != nil {
			return err
		}

		var blocking []string
		for _, r := range reports {
			if only != "" && r.Workspace.SourceVault != only {
				continue
			}
			if r.Unsafe() {
				blocking = append(blocking, r.Workspace.Name)
			}
		}

		if len(blocking) > 0 {
			return fmt.Errorf("unsafe to unplug — unsynced work in: %s (run 'neonroot sync')",
				strings.Join(blocking, ", "))
		}
		scope := "all loaded workspaces"
		if only != "" {
			scope = fmt.Sprintf("vault %q", only)
		}
		app.UI.Success(fmt.Sprintf("safe to unplug — %s clean", scope))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guardCmd)
}
