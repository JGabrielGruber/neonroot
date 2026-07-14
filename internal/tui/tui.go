// Package tui is NeonRoot's interactive cockpit: a Bubble Tea dashboard over the
// same state the CLI reads and the same verbs it runs. It depends only on a
// Deps DTO (never on package cmd), so there is no import cycle.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/JGabrielGruber/neonroot/internal/config"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/ui"
)

// Deps is everything the cockpit needs, handed in by cmd/root.go.
type Deps struct {
	Paths  platform.Paths
	Config *config.Config
	Runner platform.Runner
	Self   string   // path to the neonroot binary, for re-invoking verbs (E1.7)
	Theme  ui.Theme // the shared neon theme
}

// Run launches the cockpit and blocks until the user quits.
func Run(d Deps) error {
	_, err := tea.NewProgram(newModel(d), tea.WithAltScreen()).Run()
	return err
}
