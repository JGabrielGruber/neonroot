package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var rmVaultFlag string

var rmCmd = &cobra.Command{
	Use:   "rm <workspace>",
	Short: "Delete a workspace from its vault",
	Long: `Permanently removes the workspace's git repo and its catalog entry from the
vault. This is destructive — pushed history is gone. Stop the workspace first if
it is currently loaded.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if workspace.IsLoaded(app.Paths, name) {
			return fmt.Errorf("workspace %q is loaded — run 'neonroot stop %s' first", name, name)
		}

		v, err := app.resolveVault(rmVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}

		lock, err := app.lock("vault-" + v.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		if err := removeWorkspace(v, name); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("deleted workspace %q from vault %q", name, v.Name))
		return nil
	},
}

func init() {
	rmCmd.Flags().StringVar(&rmVaultFlag, "vault", "", "vault holding the workspace (default: configured default vault)")
	rootCmd.AddCommand(rmCmd)
}
