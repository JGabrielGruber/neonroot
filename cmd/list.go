package cmd

import (
	"encoding/json"
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

var (
	listVaultFlag  string
	listJSONFlag   bool
	listLoadedFlag bool
)

// listRow is one workspace as reported by `list` — the machine-readable shape
// behind `--json`, so an agent/SDK can drive NeonRoot programmatically.
type listRow struct {
	Name      string   `json:"name"`
	Vault     string   `json:"vault"`
	State     string   `json:"state"` // "loaded" | "available"
	Loaded    bool     `json:"loaded"`
	Images    []string `json:"images,omitempty"`
	Secrets   bool     `json:"secrets,omitempty"`
	Isolation string   `json:"isolation,omitempty"` // "", "sandbox", "isolated"
}

// listCmd is workspace-first: the bare `neonroot list` shows your workspaces.
// Vaults are background config, listed via `neonroot vault list`.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your workspaces",
	Long: `Lists workspaces across available vaults, with their vault, image, and loaded
state. --loaded limits to the running fleet; --json emits machine-readable output
(the surface an agent/SDK drives).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		cat := app.catalog()

		rows := []listRow{}
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
				loaded := workspace.IsLoaded(app.Paths, w.Name)
				if listLoadedFlag && !loaded {
					continue
				}
				state := "available"
				if loaded {
					state = "loaded"
				}
				rows = append(rows, listRow{
					Name: w.Name, Vault: r.Name, State: state, Loaded: loaded,
					Images: w.Images, Secrets: w.Secrets, Isolation: w.Isolation,
				})
			}
		}

		out := cmd.OutOrStdout()
		if listJSONFlag {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(rows)
		}

		fmt.Fprintf(out, "%-14s %-10s %-9s %s\n", "WORKSPACE", "VAULT", "STATE", "IMAGE")
		for _, w := range rows {
			image := "-"
			if len(w.Images) > 0 {
				image = strings.Join(w.Images, ",")
			}
			if w.Secrets {
				image += " (secrets)"
			}
			if w.Isolation != "" {
				image += " (" + w.Isolation + ")"
			}
			fmt.Fprintf(out, "%-14s %-10s %-9s %s\n", w.Name, w.Vault, w.State, image)
		}
		if len(rows) == 0 {
			app.UI.Info("no workspaces yet — create one with 'neonroot create <name>'")
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listVaultFlag, "vault", "", "limit to one vault")
	listCmd.Flags().BoolVar(&listLoadedFlag, "loaded", false, "only currently loaded workspaces (the running fleet)")
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "emit machine-readable JSON")
	rootCmd.AddCommand(listCmd)
}
