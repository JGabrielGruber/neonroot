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
	cfg.EnsureScratch(filepath.Join(paths.Cache, "scratch"))

	reporter := ui.NewStderr(ui.Options{Quiet: flags.quiet, ForcePlain: flags.plain})

	return &App{Paths: paths, Config: cfg, UI: reporter}, nil
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
