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
		if m.confirming {
			return m.updateConfirm(msg)
		}
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
		m.busy = ""
		m.statusErr = msg.err != nil
		switch {
		case msg.err != nil && lastLine(msg.out) != "":
			m.status = lastLine(msg.out)
		case msg.err != nil:
			m.status = msg.err.Error()
		default:
			m.status = lastLine(msg.out)
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

	case "n": // new workspace: "name [image]"
		if m.busy != "" {
			return m, nil
		}
		m.inputting = true
		m.inputKind = "create"
		m.input = ""
		m.status = ""
		return m, nil

	case "e": // rename the selected workspace (CLI refuses if it's loaded)
		if w, ok := m.selected(); ok && m.busy == "" {
			m.inputting = true
			m.inputKind = "rename"
			m.target = w
			m.input = ""
			m.status = ""
		}
		return m, nil

	case "d": // delete the selected workspace (confirm first — destructive)
		if w, ok := m.selected(); ok && m.busy == "" {
			m.confirming = true
			m.target = w
			m.status = ""
		}
		return m, nil

	// attach hands over the terminal; every other action runs in the background
	// with its output captured, so the dashboard never flashes to the log.
	case "a":
		if w, ok := m.selected(); ok && w.loaded && m.busy == "" {
			return m, m.runInteractive("attach", w.name)
		}
	case "y":
		return m.bgAction("syncing all", "sync")
	case "l":
		if w, ok := m.selected(); ok && !w.loaded {
			return m.bgAction("loading "+w.name, "load", w.name, "--vault", w.vaultName)
		}
	case "c":
		if w, ok := m.selected(); ok && w.loaded {
			return m.bgAction("committing "+w.name, "commit", w.name)
		}
	case "x":
		if w, ok := m.selected(); ok && w.loaded {
			return m.bgAction("stopping "+w.name, "stop", w.name)
		}
	}
	return m, nil
}

// bgAction starts a background verb unless one is already running (prevents
// overlapping subprocesses), setting the busy label shown in the status line.
func (m model) bgAction(label string, args ...string) (tea.Model, tea.Cmd) {
	if m.busy != "" {
		return m, nil
	}
	m.busy = label
	m.status = ""
	return m, m.runVerb(args...)
}

// updateInput handles keys while typing a new workspace name.
func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.input)
		kind, target := m.inputKind, m.target
		m.inputting = false
		m.input = ""
		if text == "" {
			return m, nil
		}
		if kind == "rename" {
			return m.bgAction("renaming "+target.name, "set", target.name, "--vault", target.vaultName, "--rename", text)
		}
		// create: first token is the name, an optional second is the image.
		fields := strings.Fields(text)
		args := []string{"create", fields[0]}
		if len(fields) > 1 {
			args = append(args, "--image", fields[1])
		}
		return m.bgAction("creating "+fields[0], args...)
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

// updateConfirm handles the y/N gate before a destructive delete. Only an
// explicit 'y' proceeds; any other key cancels.
func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	target := m.target
	m.confirming = false
	switch msg.String() {
	case "y", "Y":
		return m.bgAction("deleting "+target.name, "rm", target.name, "--vault", target.vaultName)
	default:
		m.status = "delete cancelled"
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
