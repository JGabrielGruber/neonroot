package tui

import (
	"errors"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// actionDoneMsg reports the result of a re-invoked CLI verb.
type actionDoneMsg struct{ err error }

var errNoSelf = errors.New("cannot locate the neonroot binary to run actions")

// runVerb suspends the cockpit and runs `neonroot <args...>` as a child with the
// terminal handed to it, then resumes. This is the whole dispatch strategy: every
// mutating/interactive action re-invokes the CLI, so attach's terminal takeover
// and load's live progress work exactly as they do from the shell — with zero
// orchestration logic duplicated in the TUI.
func (m model) runVerb(args ...string) tea.Cmd {
	if m.deps.Self == "" {
		return func() tea.Msg { return actionDoneMsg{errNoSelf} }
	}
	c := exec.Command(m.deps.Self, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg { return actionDoneMsg{err} })
}
