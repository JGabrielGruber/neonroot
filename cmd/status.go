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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show repos, their availability, and contents",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for _, r := range app.Config.Repos {
			state := repo.State(r.Path, mounts)
			fmt.Fprintf(out, "%-12s %-11s %s\n", r.Name, state, r.Path)

			if state != domain.RepoStateAvailable {
				continue
			}
			idx, err := repo.ReadIndex(r.Path)
			if errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(out, "    (uninitialized)\n")
				continue
			}
			if err != nil {
				app.UI.Warn(fmt.Sprintf("%s: %v", r.Name, err))
				continue
			}
			fmt.Fprintf(out, "    revision %d, %d workspace(s)\n", idx.Revision, len(idx.Workspaces))
			for _, w := range idx.Workspaces {
				fmt.Fprintf(out, "      - %s\n", w.Name)
			}
		}
		// Currently loaded (hydrated) workspaces.
		loaded, err := workspace.List(app.Paths)
		if err != nil {
			return err
		}
		if len(loaded) > 0 {
			fmt.Fprintf(out, "\nloaded workspaces (in tmpfs):\n")
			for _, w := range loaded {
				fmt.Fprintf(out, "  %-12s from %-10s %s\n", w.Name, w.SourceRepo, w.Root)
			}
		}
		// Per-workspace change diff lands in Phase 4.
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
