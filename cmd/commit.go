package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/commit"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var (
	commitRepoFlag  string
	commitAsFlag    string
	commitForceFlag bool
)

var commitCmd = &cobra.Command{
	Use:   "commit <workspace>",
	Short: "Write workspace changes back to a repo",
	Long: `Writes a loaded workspace's changes back to cold storage. By default it
commits in place (only the changed files) to the repo it was loaded from, after
checking the repo has not changed underneath you. Use --as to save a copy under
a new name, --repo to target a different repo, and --force to override.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ws, err := workspace.ReadState(app.Paths, name)
		if err != nil {
			return err
		}

		targetName := commitRepoFlag
		if targetName == "" {
			targetName = ws.SourceRepo
		}
		target, err := app.resolveRepo(targetName)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(target); err != nil {
			return err
		}

		lock, err := app.lock("repo-" + target.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		committer := &commit.Committer{Paths: app.Paths, UI: app.UI}
		res, err := committer.Commit(ws, target, commit.Options{AsName: commitAsFlag, Force: commitForceFlag})
		if err != nil {
			return err
		}

		reportCommit(res)
		return nil
	},
}

func reportCommit(res *commit.Result) {
	if res.SavedAs {
		app.UI.Success(fmt.Sprintf("saved to repo %q as %q: %d file(s), revision %d",
			res.TargetRepo, res.TargetName, res.FileCount, res.Revision))
		return
	}
	if len(res.Changes) == 0 {
		app.UI.Info("nothing to commit — workspace matches the repo")
		return
	}
	var added, modified, deleted int
	for _, c := range res.Changes {
		app.UI.Info(fmt.Sprintf("  %-8s %s", c.Kind, c.Path))
		switch c.Kind {
		case domain.ChangeAdded:
			added++
		case domain.ChangeModified:
			modified++
		case domain.ChangeDeleted:
			deleted++
		}
	}
	app.UI.Success(fmt.Sprintf("committed to %q: +%d ~%d -%d, revision %d",
		res.TargetRepo, added, modified, deleted, res.Revision))
}

func init() {
	commitCmd.Flags().StringVarP(&commitRepoFlag, "repo", "r", "", "target repo (default: the repo it was loaded from)")
	commitCmd.Flags().StringVar(&commitAsFlag, "as", "", "save under a new workspace name instead of committing in place")
	commitCmd.Flags().BoolVar(&commitForceFlag, "force", false, "override a conflict or overwrite an existing target")
	rootCmd.AddCommand(commitCmd)
}
