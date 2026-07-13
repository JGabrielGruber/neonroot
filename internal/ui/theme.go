// Package ui renders user feedback. All command output goes through the
// Reporter interface so hydration and commit can emit rich, styled progress on
// a terminal while falling back to plain lines when piped or in --quiet mode.
package ui

import "github.com/charmbracelet/lipgloss"

// Theme is NeonRoot's synthwave/neon palette, applied consistently across every
// command's output. Colors are adaptive so they stay legible on light and dark
// terminals.
type Theme struct {
	Accent  lipgloss.Style // tool name / headings — neon magenta
	Step    lipgloss.Style // an operation in progress — cyan
	Success lipgloss.Style // completion — neon green
	Warn    lipgloss.Style // recoverable issue — amber
	Error   lipgloss.Style // failure — hot pink/red
	Muted   lipgloss.Style // secondary detail — dim
}

// NeonTheme is the default palette.
func NeonTheme() Theme {
	return Theme{
		Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5fff")).Bold(true),
		Step:    lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("#39ff14")).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb000")),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3860")).Bold(true),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#8b8b9e")),
	}
}
