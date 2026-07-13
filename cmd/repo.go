package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage the repo registry (one-time setup)",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Register a repo path in config",
	Long: `Registers a named cold-storage location (typically a directory on an
external drive). If no real default repo is set yet, the new repo becomes the
default so workspace commands need no --repo. This writes config, the only file
NeonRoot stores on the SD card.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, path := args[0], args[1]
		if !filepath.IsAbs(path) {
			return fmt.Errorf("path must be absolute: %s", path)
		}
		app.Config.AddRepo(domain.Repo{Name: name, Path: filepath.Clean(path)})

		// Configure-once: the first real repo becomes the default, replacing the
		// volatile scratch placeholder.
		madeDefault := false
		if app.Config.DefaultRepo == "" || app.Config.DefaultRepo == config.ScratchRepoName {
			app.Config.DefaultRepo = name
			madeDefault = true
		}
		if err := saveConfig(); err != nil {
			return err
		}

		msg := fmt.Sprintf("registered repo %q → %s", name, path)
		if madeDefault {
			msg += " (now the default)"
		}
		app.UI.Success(msg)
		return nil
	},
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured repos and their availability",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(app.Config.Repos) == 0 {
			app.UI.Info("no repos configured")
			return nil
		}
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for _, r := range app.Config.Repos {
			marker := " "
			if r.Name == app.Config.DefaultRepo {
				marker = "*"
			}
			fmt.Fprintf(out, "%s %-12s %-11s %s\n", marker, r.Name, repo.State(r.Path, mounts), r.Path)
		}
		return nil
	},
}

var repoSetDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Set the default repo for workspace commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, ok := app.Config.Repo(name); !ok {
			return fmt.Errorf("%w: %q", domain.ErrRepoNotFound, name)
		}
		app.Config.DefaultRepo = name
		if err := saveConfig(); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("default repo is now %q", name))
		return nil
	},
}

// saveConfig persists the config to the card.
func saveConfig() error {
	return config.Save(app.Config, filepath.Join(app.Paths.Config, "config.toml"))
}

func init() {
	repoCmd.AddCommand(repoAddCmd, repoListCmd, repoSetDefaultCmd)
	rootCmd.AddCommand(repoCmd)
}
