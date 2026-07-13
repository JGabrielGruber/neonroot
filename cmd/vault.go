package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage the vault registry (one-time setup)",
}

var vaultAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Register a vault path in config",
	Long: `Registers a named cold-storage location (typically a directory on an
external drive). If no real default vault is set yet, the new vault becomes the
default so workspace commands need no --vault. This writes config, the only file
NeonRoot stores on the SD card.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, path := args[0], args[1]
		if !filepath.IsAbs(path) {
			return fmt.Errorf("path must be absolute: %s", path)
		}
		app.Config.AddVault(domain.Vault{Name: name, Path: filepath.Clean(path)})

		// Configure-once: the first real vault becomes the default, replacing the
		// volatile scratch placeholder.
		madeDefault := false
		if app.Config.DefaultVault == "" || app.Config.DefaultVault == config.ScratchVaultName {
			app.Config.DefaultVault = name
			madeDefault = true
		}
		if err := saveConfig(); err != nil {
			return err
		}

		msg := fmt.Sprintf("registered vault %q → %s", name, path)
		if madeDefault {
			msg += " (now the default)"
		}
		app.UI.Success(msg)
		return nil
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured vaults and their availability",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(app.Config.Vaults) == 0 {
			app.UI.Info("no vaults configured")
			return nil
		}
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for _, r := range app.Config.Vaults {
			marker := " "
			if r.Name == app.Config.DefaultVault {
				marker = "*"
			}
			fmt.Fprintf(out, "%s %-12s %-11s %s\n", marker, r.Name, vault.State(r.Path, mounts), r.Path)
		}
		return nil
	},
}

var vaultSetDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Set the default vault for workspace commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, ok := app.Config.Vault(name); !ok {
			return fmt.Errorf("%w: %q", domain.ErrVaultNotFound, name)
		}
		app.Config.DefaultVault = name
		if err := saveConfig(); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("default vault is now %q", name))
		return nil
	},
}

// saveConfig persists the config to the card.
func saveConfig() error {
	return config.Save(app.Config, filepath.Join(app.Paths.Config, "config.toml"))
}

func init() {
	vaultCmd.AddCommand(vaultAddCmd, vaultListCmd, vaultSetDefaultCmd)
	rootCmd.AddCommand(vaultCmd)
}
