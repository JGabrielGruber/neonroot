package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/repo"
)

var createRepoFlag string

var createCmd = &cobra.Command{
	Use:   "create <workspace>",
	Short: "Create a new empty workspace in a repo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		r, err := app.resolveRepo(createRepoFlag)
		if err != nil {
			return err
		}
		if state, err := repo.StateLive(r.Path); err != nil {
			return err
		} else if state != domain.RepoStateAvailable {
			return fmt.Errorf("%w: %q at %s — plug in the drive and retry",
				domain.ErrRepoUnavailable, r.Name, r.Path)
		}

		// Serialize index mutation against other neonroot processes.
		lock, err := app.lock("repo-" + r.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		idx, err := repo.ReadIndex(r.Path)
		if errors.Is(err, fs.ErrNotExist) {
			idx = repo.NewIndex() // first workspace initializes the repo
		} else if err != nil {
			return err
		}

		if _, exists := repo.Workspace(idx, name); exists {
			return fmt.Errorf("%w: %q in repo %q", domain.ErrWorkspaceExists, name, r.Name)
		}

		root := filepath.Join("workspaces", name)
		if err := os.MkdirAll(filepath.Join(r.Path, root), 0o755); err != nil {
			return err
		}
		idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: name, Root: root})
		repo.Bump(idx)
		if err := repo.WriteIndex(r.Path, idx); err != nil {
			return err
		}

		app.UI.Success(fmt.Sprintf("created workspace %q in repo %q (revision %d)", name, r.Name, idx.Revision))
		return nil
	},
}

func init() {
	createCmd.Flags().StringVarP(&createRepoFlag, "repo", "r", "", "target repo (default: configured default repo)")
	rootCmd.AddCommand(createCmd)
}
