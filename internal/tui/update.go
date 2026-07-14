package tui

import tea "github.com/charmbracelet/bubbletea"

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
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
		}
		return m, nil

	case snapshotMsg:
		m.snap = snapshot(msg)
		m.loaded = true
		m.refreshing = false
		if n := len(m.selectable()); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		return m, nil

	case tickMsg:
		cmd := m.refresh()
		return m, tea.Batch(cmd, tick())
	}
	return m, nil
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
