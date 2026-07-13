package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/domain"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage the repo registry",
}

var repoAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Register a repo path in config",
	Long: `Registers a named cold-storage location (typically a directory on an
external drive) so it can be loaded from and committed to. This writes config,
the only file NeonRoot stores on the SD card.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, path := args[0], args[1]
		if !filepath.IsAbs(path) {
			return fmt.Errorf("path must be absolute: %s", path)
		}
		app.Config.AddRepo(domain.Repo{Name: name, Path: filepath.Clean(path)})

		cfgPath := filepath.Join(app.Paths.Config, "config.toml")
		if err := config.Save(app.Config, cfgPath); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("registered repo %q → %s", name, path))
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoAddCmd)
	rootCmd.AddCommand(repoCmd)
}
