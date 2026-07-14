package cmd

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
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

// workspaceStatus shows one loaded workspace's live git state: uncommitted
// changes and how far ahead/behind its vault it is.
func workspaceStatus(cmd *cobra.Command, name string) error {
	g := &git.Git{Runner: app.Runner}
	if !g.Available() {
		return fmt.Errorf("git is required but was not found on PATH")
	}
	r, err := workspace.ReportFor(cmd.Context(), app.Paths, g, name)
	if err != nil {
		return err
	}
	if r.Err != nil {
		return r.Err
	}
	ws, st := r.Workspace, r.Status
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (from %s) at %s\n", ws.Name, ws.SourceVault, ws.Root)
	switch {
	case st.Dirty && st.Ahead > 0:
		fmt.Fprintf(out, "  uncommitted changes, and %d commit(s) not yet pushed\n", st.Ahead)
	case st.Dirty:
		fmt.Fprintln(out, "  uncommitted changes")
	case st.Ahead > 0:
		fmt.Fprintf(out, "  %d commit(s) not yet pushed — run 'neonroot commit %s'\n", st.Ahead, name)
	default:
		fmt.Fprintln(out, "  clean — nothing to commit")
	}
	if st.Behind > 0 {
		fmt.Fprintf(out, "  vault is %d commit(s) ahead — reload to catch up\n", st.Behind)
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

	reports, err := workspace.Reports(cmd.Context(), app.Paths, &git.Git{Runner: app.Runner})
	if err != nil {
		return err
	}
	if len(reports) > 0 {
		fmt.Fprintf(out, "\nhot storage — loaded workspaces (in tmpfs):\n")
		var total int64
		for _, r := range reports {
			total += r.HotBytes
			fmt.Fprintf(out, "  %-12s from %-10s %8s  %s  %s\n",
				r.Workspace.Name, r.Workspace.SourceVault, humanSize(r.HotBytes),
				pendingMark(r), r.Workspace.Root)
		}
		fmt.Fprintf(out, "  %-12s %27s\n", "TOTAL", humanSize(total))
		fmt.Fprintf(out, "\nrun 'neonroot status <workspace>' for details\n")
	}
	return nil
}

// pendingMark is a compact indicator of a loaded workspace's sync state.
func pendingMark(r workspace.Report) string {
	switch {
	case r.Err != nil:
		return "?"
	case r.Status.Dirty && r.Status.Ahead > 0:
		return "±"
	case r.Status.Dirty:
		return "*"
	case r.Status.Ahead > 0:
		return "↑"
	default:
		return "✓"
	}
}

// humanSize renders a byte count compactly.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
