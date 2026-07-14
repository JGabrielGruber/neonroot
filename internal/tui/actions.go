package tui

import (
	"errors"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// actionDoneMsg reports the result of a re-invoked CLI verb.
type actionDoneMsg struct {
	out string
	err error
}

var errNoSelf = errors.New("cannot locate the neonroot binary to run actions")

// runVerb runs `neonroot <args...>` in the background with its output captured
// (the dashboard stays visible — no flash), returning an actionDoneMsg. Used for
// every non-interactive action (load/commit/stop/sync/create).
func (m model) runVerb(args ...string) tea.Cmd {
	self := m.deps.Self
	return func() tea.Msg {
		if self == "" {
			return actionDoneMsg{err: errNoSelf}
		}
		out, err := exec.Command(self, args...).CombinedOutput()
		return actionDoneMsg{out: string(out), err: err}
	}
}

// runInteractive hands the terminal to `neonroot <args...>` (attach): the cockpit
// suspends, the child takes the tty (so its own tmux/podman-exec handoff works),
// then the cockpit resumes. Only truly interactive verbs use this.
func (m model) runInteractive(args ...string) tea.Cmd {
	if m.deps.Self == "" {
		return func() tea.Msg { return actionDoneMsg{err: errNoSelf} }
	}
	c := exec.Command(m.deps.Self, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return actionDoneMsg{err: err} })
}

// lastLine returns the last non-empty, trimmed line of s (the verb's summary).
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}
