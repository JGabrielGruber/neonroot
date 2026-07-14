package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

func runes(s string) tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func withOne(loaded bool) model {
	return model{
		loaded: true,
		snap: snapshot{vaults: []vaultRow{{
			state:      domain.VaultStateAvailable,
			workspaces: []wsRow{{name: "app", vaultName: "ext", loaded: loaded}},
		}}},
	}
}

func TestActionKeys_DispatchByState(t *testing.T) {
	// Loaded workspace: attach/commit/stop fire; load does not.
	m := withOne(true)
	for _, k := range []string{"a", "c", "x"} {
		if _, cmd := m.updateKey(runes(k)); cmd == nil {
			t.Errorf("key %q on a loaded workspace should dispatch", k)
		}
	}
	if _, cmd := m.updateKey(runes("l")); cmd != nil {
		t.Error("load must not fire on an already-loaded workspace")
	}

	// Cold workspace: load fires; attach does not.
	c := withOne(false)
	if _, cmd := c.updateKey(runes("l")); cmd == nil {
		t.Error("load should fire on a cold workspace")
	}
	if _, cmd := c.updateKey(runes("a")); cmd != nil {
		t.Error("attach must not fire on a cold workspace")
	}

	// sync is always available.
	if _, cmd := m.updateKey(runes("y")); cmd == nil {
		t.Error("sync should always dispatch")
	}
}

func TestNewWorkspaceInput(t *testing.T) {
	m := withOne(false)
	nm, _ := m.updateKey(runes("n"))
	m = nm.(model)
	if !m.inputting {
		t.Fatal("'n' should enter input mode")
	}
	// Type "api".
	for _, r := range []string{"a", "p", "i"} {
		nm, _ := m.updateInput(runes(r))
		m = nm.(model)
	}
	if m.input != "api" {
		t.Fatalf("input = %q, want api", m.input)
	}
	// Backspace once.
	nm, _ = m.updateInput(key(tea.KeyBackspace))
	m = nm.(model)
	if m.input != "ap" {
		t.Fatalf("after backspace input = %q, want ap", m.input)
	}
	// Enter dispatches and leaves input mode.
	nm, cmd := m.updateInput(key(tea.KeyEnter))
	m = nm.(model)
	if m.inputting {
		t.Error("enter should leave input mode")
	}
	if cmd == nil {
		t.Error("enter with a name should dispatch create")
	}
}

func TestInputEscCancels(t *testing.T) {
	m := withOne(false)
	m.inputting = true
	m.input = "half"
	nm, _ := m.updateInput(key(tea.KeyEsc))
	m = nm.(model)
	if m.inputting || m.input != "" {
		t.Error("esc should cancel input and clear the buffer")
	}
}

func TestActionErrorSurfaced(t *testing.T) {
	m := withOne(true)
	nm, _ := m.Update(actionDoneMsg{err: errNoSelf})
	got := nm.(model)
	if got.status == "" || !got.statusErr {
		t.Errorf("an action error should surface in status (err), got status=%q err=%v", got.status, got.statusErr)
	}
	if got.busy != "" {
		t.Error("busy should clear when the action completes")
	}
}

func TestBackgroundActionSetsBusyAndGuards(t *testing.T) {
	m := withOne(true)
	// 'c' (commit) on a loaded workspace sets busy.
	nm, cmd := m.updateKey(runes("c"))
	m = nm.(model)
	if m.busy == "" || cmd == nil {
		t.Fatal("commit should set a busy label and dispatch")
	}
	// A second action while busy is ignored (no overlapping subprocess).
	if _, cmd := m.updateKey(runes("x")); cmd != nil {
		t.Error("actions should be ignored while busy")
	}
}
