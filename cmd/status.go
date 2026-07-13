package cmd

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/commit"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var statusCmd = &cobra.Command{
	Use:   "status [workspace]",
	Short: "Show vault availability, or a loaded workspace's pending changes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return workspaceStatus(cmd, args[0])
		}
		return overviewStatus(cmd)
	},
}

// workspaceStatus shows the pending diff for one loaded workspace.
func workspaceStatus(cmd *cobra.Command, name string) error {
	ws, err := workspace.ReadState(app.Paths, name)
	if err != nil {
		return err
	}
	man, err := hydration.ReadManifest(app.Paths.ManifestPath(name))
	if err != nil {
		return err
	}
	changes, err := commit.Diff(ws.Root, man)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (from %s) at %s\n", ws.Name, ws.SourceVault, ws.Root)
	if len(changes) == 0 {
		fmt.Fprintln(out, "  clean — no changes since load")
		return nil
	}
	for _, c := range changes {
		fmt.Fprintf(out, "  %-8s %s\n", c.Kind, c.Path)
	}
	return nil
}

// overviewStatus shows every vault's availability and contents plus loaded
// workspaces.
func overviewStatus(cmd *cobra.Command) error {
	mounts, err := platform.Mounts()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	for _, r := range app.Config.Vaults {
		state := vault.State(r.Path, mounts)
		fmt.Fprintf(out, "%-12s %-11s %s\n", r.Name, state, r.Path)

		if state != domain.VaultStateAvailable {
			continue
		}
		idx, err := vault.ReadIndex(r.Path)
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

	loaded, err := workspace.List(app.Paths)
	if err != nil {
		return err
	}
	if len(loaded) > 0 {
		fmt.Fprintf(out, "\nloaded workspaces (in tmpfs):\n")
		for _, w := range loaded {
			fmt.Fprintf(out, "  %-12s from %-10s %s\n", w.Name, w.SourceVault, w.Root)
		}
		fmt.Fprintf(out, "\nrun 'neonroot status <workspace>' to see pending changes\n")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
