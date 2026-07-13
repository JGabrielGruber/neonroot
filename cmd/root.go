package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
	"github.com/JGabrielGruber/neonroot/internal/ui"
)

// version is stamped into the binary and surfaced via --version.
const version = "0.0.2"

// App is the composition root: the resolved configuration, paths, and adapters
// that every command operates through. Building it in one place keeps
// dependency wiring explicit and makes the whole tree injectable in tests.
type App struct {
	Paths  platform.Paths
	Config *config.Config
	UI     ui.Reporter
	Runner platform.Runner
}

// flags holds the persistent CLI flags parsed before any command runs.
var flags struct {
	quiet bool
	plain bool
}

// app is the process-wide composition root, populated by PersistentPreRunE.
var app *App

var rootCmd = &cobra.Command{
	Use:   "neonroot",
	Short: "NeonRoot — portable, ephemeral workspace manager",
	Long: `NeonRoot hydrates development workspaces from cold storage (an external
drive) into tmpfs so you can unplug and work untethered, then commit changes
back to the drive when you choose. It never writes to the SD card.`,
	Version:       version,
	SilenceUsage:  true, // usage on error is noise; we render errors ourselves
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		a, err := buildApp()
		if err != nil {
			return err
		}
		app = a
		return nil
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// buildApp resolves paths, prepares the tmpfs runtime dirs, loads config, and
// constructs the reporter. It is the single place adapters get wired together.
func buildApp() (*App, error) {
	paths := platform.ResolvePaths()
	if err := paths.EnsureRuntimeDirs(); err != nil {
		return nil, fmt.Errorf("preparing runtime dirs: %w", err)
	}

	cfg, err := config.Load(filepath.Join(paths.Config, "config.toml"))
	if err != nil {
		return nil, err
	}
	// The built-in scratch repo lives on tmpfs so there is always a target.
	scratchPath := filepath.Join(paths.Cache, "scratch")
	cfg.EnsureScratch(scratchPath)
	if err := ensureScratchRepo(scratchPath); err != nil {
		return nil, err
	}

	reporter := ui.NewStderr(ui.Options{Quiet: flags.quiet, ForcePlain: flags.plain})

	return &App{Paths: paths, Config: cfg, UI: reporter, Runner: platform.ExecRunner{}}, nil
}

// ensureScratchRepo materializes the tmpfs scratch repo: its directory and an
// initial index, so it is immediately available and usable as a target.
func ensureScratchRepo(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(repo.IndexPath(path)); errors.Is(err, os.ErrNotExist) {
		return repo.WriteIndex(path, repo.NewIndex())
	}
	return nil
}

// resolveRepo returns the repo registered under name, or the default repo when
// name is empty.
func (a *App) resolveRepo(name string) (domain.Repo, error) {
	if name == "" {
		name = a.Config.DefaultRepo
	}
	r, ok := a.Config.Repo(name)
	if !ok {
		return domain.Repo{}, fmt.Errorf("%w: %q", domain.ErrRepoNotFound, name)
	}
	return r, nil
}

// requireAvailable returns ErrRepoUnavailable (with a plug-in hint) unless the
// repo's backing drive is currently mounted.
func (a *App) requireAvailable(r domain.Repo) error {
	state, err := repo.StateLive(r.Path)
	if err != nil {
		return err
	}
	if state != domain.RepoStateAvailable {
		return fmt.Errorf("%w: %q at %s — plug in the drive and retry",
			domain.ErrRepoUnavailable, r.Name, r.Path)
	}
	return nil
}

// lock takes a non-blocking advisory lock under the runtime tmpfs (never the
// card), scoped by key. Callers namespace keys (e.g. "repo-ext", "ws-webapp")
// so repo and workspace locks never collide.
func (a *App) lock(key string) (*platform.FileLock, error) {
	dir := filepath.Join(a.Paths.Runtime, "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return platform.TryLock(filepath.Join(dir, key+".lock"))
}

// Execute runs the CLI, rendering sentinel errors with clear messages and
// mapping them to stable exit codes.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "neonroot: "+err.Error())
		os.Exit(exitCode(err))
	}
}

// exitCode maps known failure classes to distinct exit codes so scripts can
// branch on them; everything else is a generic failure.
func exitCode(err error) int {
	switch {
	case errors.Is(err, domain.ErrRepoUnavailable):
		return 3
	case errors.Is(err, domain.ErrLocked):
		return 4
	case errors.Is(err, domain.ErrCommitConflict):
		return 5
	default:
		return 1
	}
}

func init() {
	rootCmd.SetVersionTemplate("NeonRoot {{.Version}}\n")
	pf := rootCmd.PersistentFlags()
	pf.BoolVarP(&flags.quiet, "quiet", "q", false, "suppress progress output, show only warnings")
	pf.BoolVar(&flags.plain, "plain", false, "disable colored/interactive output")
}
