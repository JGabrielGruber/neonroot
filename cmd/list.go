package cmd

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var listRepoFlag string

// listCmd is workspace-first: the bare `neonroot list` shows your workspaces.
// Repos are background config, listed via `neonroot repo list`.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your workspaces",
	Long:  "Lists workspaces across available repos, with their repo, image, and loaded state.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-14s %-10s %-9s %s\n", "WORKSPACE", "REPO", "STATE", "IMAGE")

		var rows int
		for _, r := range app.Config.Repos {
			if listRepoFlag != "" && r.Name != listRepoFlag {
				continue
			}
			if repo.State(r.Path, mounts) != domain.RepoStateAvailable {
				continue
			}
			idx, err := repo.ReadIndex(r.Path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: %v", r.Name, err))
				continue
			}
			for _, w := range idx.Workspaces {
				state := "available"
				if workspace.IsLoaded(app.Paths, w.Name) {
					state = "loaded"
				}
				image := w.Image
				if image == "" {
					image = "-"
				}
				fmt.Fprintf(out, "%-14s %-10s %-9s %s\n", w.Name, r.Name, state, image)
				rows++
			}
		}
		if rows == 0 {
			app.UI.Info("no workspaces yet — create one with 'neonroot create <name>'")
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVarP(&listRepoFlag, "repo", "r", "", "limit to one repo")
	rootCmd.AddCommand(listCmd)
}
