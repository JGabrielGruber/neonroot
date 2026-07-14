package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var listVaultFlag string

// listCmd is workspace-first: the bare `neonroot list` shows your workspaces.
// Vaults are background config, listed via `neonroot vault list`.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your workspaces",
	Long:  "Lists workspaces across available vaults, with their vault, image, and loaded state.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-14s %-10s %-9s %s\n", "WORKSPACE", "VAULT", "STATE", "IMAGE")

		cat := app.catalog()
		var rows int
		for _, r := range app.Config.Vaults {
			if listVaultFlag != "" && r.Name != listVaultFlag {
				continue
			}
			// Local vaults must be mounted to be listed; remote vaults are read
			// over ssh (an unreachable one warns rather than being skipped).
			if !r.IsRemote() && vault.State(r.Path, mounts) != domain.VaultStateAvailable {
				continue
			}
			idx, err := cat.Read(cmd.Context(), r)
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
				image := "-"
				if len(w.Images) > 0 {
					image = strings.Join(w.Images, ",")
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
	listCmd.Flags().StringVarP(&listVaultFlag, "vault", "", "", "limit to one vault")
	rootCmd.AddCommand(listCmd)
}
