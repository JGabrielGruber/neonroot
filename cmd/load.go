package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var loadRepoFlag string

var loadCmd = &cobra.Command{
	Use:   "load <workspace>",
	Short: "Hydrate a workspace from a repo into tmpfs",
	Long: `Copies a workspace from its repo on cold storage into tmpfs (RAM) so you
can unplug the drive and work untethered. Records a manifest of what was copied
so a later commit can compute exactly what changed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		r, err := app.resolveRepo(loadRepoFlag)
		if err != nil {
			return err
		}

		// Guard the tmpfs payload for this workspace against concurrent loads.
		lock, err := app.lock("ws-" + name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		loader := &workspace.Loader{Paths: app.Paths, UI: app.UI}
		ws, err := loader.Load(r, name)
		if err != nil {
			return err
		}

		app.UI.Info(fmt.Sprintf("ready at %s — safe to unplug %q", ws.Root, r.Name))
		return nil
	},
}

func init() {
	loadCmd.Flags().StringVarP(&loadRepoFlag, "repo", "r", "", "source repo (default: configured default repo)")
	rootCmd.AddCommand(loadCmd)
}
