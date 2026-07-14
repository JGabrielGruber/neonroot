package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.inputting {
			return m.updateInput(msg)
		}
		return m.updateKey(msg)

	case snapshotMsg:
		m.snap = snapshot(msg)
		m.loaded = true
		m.refreshing = false
		if n := len(m.selectable()); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		return m, nil

	case actionDoneMsg:
		m.lastErr = ""
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		}
		cmd := m.refresh() // re-read state after the action
		return m, cmd

	case tickMsg:
		cmd := m.refresh()
		return m, tea.Batch(cmd, tick())
	}
	return m, nil
}

// updateKey handles keys in normal (navigation/action) mode.
func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "r":
		cmd := m.refresh()
		return m, cmd
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.selectable())-1 {
			m.cursor++
		}
		return m, nil

	case "n": // new workspace
		m.inputting = true
		m.input = ""
		m.lastErr = ""
		return m, nil
	case "y": // sync all
		return m, m.runVerb("sync")

	case "l": // load the selected cold workspace
		if w, ok := m.selected(); ok && !w.loaded {
			return m, m.runVerb("load", w.name, "--vault", w.vaultName)
		}
	case "a": // attach the selected loaded workspace
		if w, ok := m.selected(); ok && w.loaded {
			return m, m.runVerb("attach", w.name)
		}
	case "c": // commit the selected loaded workspace
		if w, ok := m.selected(); ok && w.loaded {
			return m, m.runVerb("commit", w.name)
		}
	case "x": // stop the selected loaded workspace
		if w, ok := m.selected(); ok && w.loaded {
			return m, m.runVerb("stop", w.name)
		}
	}
	return m, nil
}

// updateInput handles keys while typing a new workspace name.
func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.input)
		m.inputting = false
		m.input = ""
		if name != "" {
			return m, m.runVerb("create", name)
		}
		return m, nil
	case "esc":
		m.inputting = false
		m.input = ""
		return m, nil
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	default:
		if s := msg.String(); len(s) == 1 {
			m.input += s
		}
		return m, nil
	}
}

// refresh issues a snapshot unless one is already in flight (overlap guard). It
// mutates m, so callers must capture its result before returning m.
func (m *model) refresh() tea.Cmd {
	if m.refreshing {
		return nil
	}
	m.refreshing = true
	return m.deps.snapshotCmd()
}
