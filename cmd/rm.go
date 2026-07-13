package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/vault"
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

		idx, err := vault.ReadIndex(v.Path)
		if err != nil {
			return err
		}
		entry, ok := vault.Workspace(idx, name)
		if !ok {
			return fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, name, v.Name)
		}

		if err := os.RemoveAll(filepath.Join(v.Path, entry.Root)); err != nil {
			return err
		}
		kept := idx.Workspaces[:0:0]
		for _, w := range idx.Workspaces {
			if w.Name != name {
				kept = append(kept, w)
			}
		}
		idx.Workspaces = kept
		vault.Bump(idx)
		if err := vault.WriteIndex(v.Path, idx); err != nil {
			return err
		}

		app.UI.Success(fmt.Sprintf("deleted workspace %q from vault %q (revision %d)", name, v.Name, idx.Revision))
		return nil
	},
}

func init() {
	rmCmd.Flags().StringVar(&rmVaultFlag, "vault", "", "vault holding the workspace (default: configured default vault)")
	rootCmd.AddCommand(rmCmd)
}
