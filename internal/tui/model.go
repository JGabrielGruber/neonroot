package tui

import tea "github.com/charmbracelet/bubbletea"

type model struct {
	deps          Deps
	snap          snapshot
	cursor        int    // index into the flat selectable-workspace list
	refreshing    bool   // a snapshot is in flight (prevents overlap)
	loaded        bool   // first snapshot has arrived
	inputting     bool   // capturing text (a name) for inputKind
	inputKind     string // what the text prompt commits: "create" | "rename"
	confirming    bool   // a destructive action (delete) awaits y/N
	target        wsRow  // subject of a rename/delete prompt
	input         string
	busy          string // a background action is running (its label), else ""
	status        string // last action's result line
	statusErr     bool   // status is an error
	width, height int
}

func newModel(d Deps) model { return model{deps: d} }

func (m model) Init() tea.Cmd {
	return tea.Batch(m.deps.snapshotCmd(), tick())
}

// selectable flattens all workspaces (loaded and cold) into a navigable list, in
// the same order the view renders them, so the cursor lines up with the display.
func (m model) selectable() []wsRow {
	var out []wsRow
	for _, v := range m.snap.vaults {
		out = append(out, v.workspaces...)
	}
	return out
}

// selected returns the workspace under the cursor, if any.
func (m model) selected() (wsRow, bool) {
	flat := m.selectable()
	if m.cursor >= 0 && m.cursor < len(flat) {
		return flat[m.cursor], true
	}
	return wsRow{}, false
}
