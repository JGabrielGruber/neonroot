package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/session"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var (
	loadVaultFlag   string
	loadNoSession   bool
	loadNoContainer bool
	loadClean       bool
)

var loadCmd = &cobra.Command{
	Use:   "load <workspace>",
	Short: "Clone a workspace from a vault into tmpfs",
	Long: `Clones a workspace's git repo from its vault into tmpfs (RAM) so you can
unplug the drive and work untethered; commit pushes your changes back. Starts a
tmux session (inside its container if the workspace declares an image). An
already-loaded workspace is reused; --clean re-clones fresh.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		r, err := app.resolveVault(loadVaultFlag)
		if err != nil {
			return err
		}

		// Guard the tmpfs payload for this workspace against concurrent loads.
		lock, err := app.lock("ws-" + name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		loader := &workspace.Loader{
			Paths:       app.Paths,
			UI:          app.UI,
			Git:         &git.Git{Runner: app.Runner},
			NoContainer: loadNoContainer,
			Clean:       loadClean,
		}
		if !loadNoSession {
			tmux := &session.Tmux{Runner: app.Runner}
			if tmux.Available() {
				loader.Sessions = tmux
			} else {
				app.UI.Warn("tmux not found on PATH — loading without a session")
			}
		}
		if !loadNoContainer {
			pod, err := app.podman()
			if err != nil {
				return err
			}
			loader.Runtime = pod
		}

		ws, err := loader.Load(r, name)
		if err != nil {
			return err
		}

		app.UI.Info(fmt.Sprintf("ready at %s — safe to unplug %q", ws.Root, r.Name))
		if loader.Sessions != nil {
			app.UI.Info(fmt.Sprintf("attach with: neonroot attach %s", name))
		}
		return nil
	},
}

func init() {
	loadCmd.Flags().StringVar(&loadVaultFlag, "vault", "", "source vault (default: configured default vault)")
	loadCmd.Flags().BoolVar(&loadNoSession, "no-session", false, "do not start a tmux session")
	loadCmd.Flags().BoolVar(&loadNoContainer, "no-container", false, "run host-only even if the workspace declares an image")
	loadCmd.Flags().BoolVar(&loadClean, "clean", false, "discard an already-loaded copy and re-clone fresh")
	rootCmd.AddCommand(loadCmd)
}
