package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

// lowSpace is the tmpfs free-space threshold below which doctor warns.
const lowSpace = 512 << 20 // 512 MiB

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the environment and flag anything risky",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()
		hardFail := false

		// Tools. git is required; tmux/podman are optional (host-only degrade).
		fmt.Fprintln(out, "tools:")
		if _, err := app.Runner.LookPath("git"); err != nil {
			fmt.Fprintln(out, "  ✗ git      MISSING (required)")
			hardFail = true
		} else {
			fmt.Fprintln(out, "  ✓ git")
		}
		for _, t := range []string{"podman", "tmux"} {
			if _, err := app.Runner.LookPath(t); err != nil {
				fmt.Fprintf(out, "  ! %-8s not found (optional — host-only without it)\n", t)
			} else {
				fmt.Fprintf(out, "  ✓ %s\n", t)
			}
		}

		// Vaults.
		fmt.Fprintln(out, "vaults:")
		mounts, err := platform.Mounts()
		if err != nil {
			return err
		}
		for _, v := range app.Config.Vaults {
			state := vault.State(v.Path, mounts)
			mark := "✓"
			if state != domain.VaultStateAvailable {
				mark = "·"
			}
			fmt.Fprintf(out, "  %s %-12s %-11s %s\n", mark, v.Name, state, v.Path)
		}

		// tmpfs headroom for loading workspaces.
		fmt.Fprintln(out, "hot storage:")
		if free, err := platform.FreeBytes(app.Paths.Workspaces); err != nil {
			fmt.Fprintf(out, "  ! could not read free space: %v\n", err)
		} else if free < lowSpace {
			fmt.Fprintf(out, "  ! only %s free in tmpfs (%s) — loads may not fit\n",
				humanSize(int64(free)), app.Paths.Workspaces)
		} else {
			fmt.Fprintf(out, "  ✓ %s free in tmpfs\n", humanSize(int64(free)))
		}

		// Pending work in loaded workspaces (the unplug-safety check).
		fmt.Fprintln(out, "loaded workspaces:")
		g := &git.Git{Runner: app.Runner}
		reports, err := workspace.Reports(cmd.Context(), app.Paths, g)
		if err != nil {
			return err
		}
		if len(reports) == 0 {
			fmt.Fprintln(out, "  (none loaded)")
		}
		unsafe := 0
		for _, r := range reports {
			if r.Unsafe() {
				unsafe++
				fmt.Fprintf(out, "  ! %-12s unsynced work — 'neonroot sync' before unplugging %q\n",
					r.Workspace.Name, r.Workspace.SourceVault)
			} else {
				fmt.Fprintf(out, "  ✓ %-12s clean\n", r.Workspace.Name)
			}
		}

		fmt.Fprintln(out)
		switch {
		case hardFail:
			return fmt.Errorf("doctor found a blocking problem (see above)")
		case unsafe > 0:
			app.UI.Warn(fmt.Sprintf("%d loaded workspace(s) have unsynced work — run 'neonroot sync'", unsafe))
		default:
			app.UI.Success("all good")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
