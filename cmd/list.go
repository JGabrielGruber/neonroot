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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repos and their availability",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repos := app.Config.Repos
		if len(repos) == 0 {
			app.UI.Info("no repos configured")
			return nil
		}
		// Read the mount table once and resolve every repo against it.
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for _, r := range repos {
			state := repo.State(r.Path, mounts)
			marker := " "
			if r.Name == app.Config.DefaultRepo {
				marker = "*"
			}
			fmt.Fprintf(out, "%s %-12s %-11s %s\n", marker, r.Name, state, r.Path)
		}
		return nil
	},
}

var listRepoFlag string

var listWorkspacesCmd = &cobra.Command{
	Use:   "workspaces",
	Short: "List workspaces across available repos",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-10s %-14s %-9s %s\n", "REPO", "WORKSPACE", "STATE", "IMAGE")
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
				fmt.Fprintf(out, "%-10s %-14s %-9s %s\n", r.Name, w.Name, state, image)
			}
		}
		return nil
	},
}

func init() {
	listWorkspacesCmd.Flags().StringVarP(&listRepoFlag, "repo", "r", "", "limit to one repo")
	listCmd.AddCommand(listWorkspacesCmd)
	rootCmd.AddCommand(listCmd)
}
