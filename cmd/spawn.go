package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/runtime"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var (
	spawnVaultFlag    string
	spawnImageFlag    string
	spawnSeedFlag     string
	spawnSandboxFlag  bool
	spawnIsolatedFlag bool
	spawnKeepFlag     bool
)

var spawnCmd = &cobra.Command{
	Use:   "spawn [name] --image <img> -- <command...>",
	Short: "Create a throwaway workspace, run a command in it, then reap it",
	Long: `The agent primitive: create a fresh workspace, start its container, run a
command inside, propagate the command's exit code, and delete the workspace
(unless --keep). Defaults to the ephemeral scratch vault; pair with --sandbox
(no host identity, dropped caps, limits) or --isolated (also no network) for a
safe throwaway box, and --seed <dir> to run against an existing project.

  neonroot spawn --image ci --sandbox --seed . -- go test ./...

Everything after '--' is the command; an optional name may precede it (else one
is generated). Requires --image (a container to run in).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Split "[name] -- cmd..." on the '--' terminator.
		dash := cmd.ArgsLenAtDash()
		if dash < 0 {
			return fmt.Errorf("provide the command after '--', e.g. spawn --image ci -- go test ./...")
		}
		pre, command := args[:dash], args[dash:]
		if len(pre) > 1 {
			return fmt.Errorf("at most one name may precede '--'")
		}
		if len(command) == 0 {
			return fmt.Errorf("no command given after '--'")
		}
		if spawnImageFlag == "" {
			return fmt.Errorf("spawn requires --image (a container to run the command in)")
		}
		name := spawnName()
		if len(pre) == 1 {
			name = pre[0]
		}

		vaultName := spawnVaultFlag
		if vaultName == "" {
			vaultName = config.ScratchVaultName // ephemeral by default
		}
		target, err := app.resolveVault(vaultName)
		if err != nil {
			return err
		}
		if target.IsRemote() {
			return fmt.Errorf("spawn needs a local vault (it reaps the workspace afterward)")
		}
		if err := app.requireAvailable(target); err != nil {
			return err
		}
		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required but was not found on PATH")
		}
		ctx := cmd.Context()

		// 1. Create the throwaway workspace (holding the vault lock only for this).
		lock, err := app.lock("vault-" + target.Name)
		if err != nil {
			return err
		}
		_, _, err = createWorkspace(ctx, g, target, name, wsSpec{
			Image:     spawnImageFlag,
			Seed:      spawnSeedFlag,
			Isolation: isolationProfile(spawnSandboxFlag, spawnIsolatedFlag),
		})
		lock.Unlock()
		if err != nil {
			return err
		}

		// 2. Load it (starts the container; no host session — we exec via `run`).
		pod, err := app.podman()
		if err != nil {
			return err
		}
		loader := &workspace.Loader{
			Paths: app.Paths, UI: app.UI,
			Git:     &git.Git{Runner: app.Runner},
			Catalog: app.catalog(),
			Runner:  app.Runner,
			Runtime: pod,
		}
		ws, err := loader.Load(target, name)
		if err != nil {
			reap(ctx, target, name, false)
			return err
		}
		if ws.ContainerID == "" {
			reap(ctx, target, name, false)
			return fmt.Errorf("spawn could not start a container for %q (check the image)", name)
		}

		// 3. Run the command headless, streaming, and capture its exit code.
		code := runInContainer(pod, ws.ContainerID, command)

		// 4. Reap (unless --keep) regardless of the command's outcome.
		if spawnKeepFlag {
			app.UI.Info(fmt.Sprintf("kept %q — inspect with 'neonroot attach %s', then 'neonroot rm %s'", name, name, name))
		} else {
			reap(ctx, target, name, true)
		}
		if code != 0 {
			return &exitError{code: code}
		}
		return nil
	},
}

// runInContainer execs command in the container (headless, inherited stdio) and
// returns its exit code; a failure to launch returns 1.
func runInContainer(pod *runtime.Podman, id string, command []string) int {
	podmanPath, err := app.Runner.LookPath("podman")
	if err != nil {
		app.UI.Warn(fmt.Sprintf("podman not found: %v", err))
		return 1
	}
	argv := append(podBaseArgs(pod), "exec", id)
	argv = append(argv, command...)
	c := exec.Command(podmanPath, argv...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		app.UI.Warn(fmt.Sprintf("run failed to start: %v", err))
		return 1
	}
	return 0
}

// reap stops and removes a spawned workspace. Best-effort; warnings only.
func reap(ctx context.Context, target domain.Vault, name string, announce bool) {
	if err := stopWorkspace(ctx, name); err != nil {
		app.UI.Warn(fmt.Sprintf("reap: stop %q: %v", name, err))
	}
	lock, err := app.lock("vault-" + target.Name)
	if err == nil {
		if err := removeWorkspace(target, name); err != nil {
			app.UI.Warn(fmt.Sprintf("reap: remove %q: %v", name, err))
		}
		lock.Unlock()
	}
	if announce {
		app.UI.Info(fmt.Sprintf("reaped %q", name))
	}
}

func spawnName() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "spawn-" + hex.EncodeToString(b)
}

func init() {
	spawnCmd.Flags().StringVar(&spawnVaultFlag, "vault", "", "target vault (default: the ephemeral scratch vault)")
	spawnCmd.Flags().StringVar(&spawnImageFlag, "image", "", "image the workspace runs in (required)")
	spawnCmd.Flags().StringVar(&spawnSeedFlag, "seed", "", "seed the workspace from a host directory")
	spawnCmd.Flags().BoolVar(&spawnSandboxFlag, "sandbox", false, "sandbox the container (no identity, dropped caps, limits; network on)")
	spawnCmd.Flags().BoolVar(&spawnIsolatedFlag, "isolated", false, "sandbox + no network (untrusted code)")
	spawnCmd.Flags().BoolVar(&spawnKeepFlag, "keep", false, "do not reap the workspace after the command (inspect/commit it)")
	rootCmd.AddCommand(spawnCmd)
}
