package session

import (
	"errors"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

func TestName(t *testing.T) {
	if got := Name("webapp"); got != "nr-webapp" {
		t.Errorf("Name = %q, want nr-webapp", got)
	}
}

func TestEnsure_CreatesWhenAbsent(t *testing.T) {
	rec := runnertest.New()
	// has-session fails (absent); new-session succeeds.
	rec.Handler = func(_ string, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "has-session" {
			// Mirror ExecRunner: a non-zero exit is wrapped as *platform.RunError.
			return nil, &platform.RunError{Name: "tmux", Err: errors.New("no server running")}
		}
		return nil, nil
	}
	tm := &Tmux{Runner: rec}

	if err := tm.Ensure("webapp", "/tmp/ws/webapp"); err != nil {
		t.Fatal(err)
	}
	lines := rec.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected has-session then new-session, got %v", lines)
	}
	want := "tmux new-session -d -s nr-webapp -c /tmp/ws/webapp"
	if lines[1] != want {
		t.Errorf("new-session args:\n got %q\nwant %q", lines[1], want)
	}
}

func TestEnsure_NoOpWhenPresent(t *testing.T) {
	rec := runnertest.New() // all calls succeed → has-session reports present
	tm := &Tmux{Runner: rec}

	if err := tm.Ensure("webapp", "/tmp/ws/webapp"); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.Calls); got != 1 {
		t.Errorf("expected only has-session (1 call), got %d: %v", got, rec.Lines())
	}
}

func TestAvailable(t *testing.T) {
	rec := runnertest.New()
	if !(&Tmux{Runner: rec}).Available() {
		t.Error("tmux should be available")
	}
	rec.Missing["tmux"] = true
	if (&Tmux{Runner: rec}).Available() {
		t.Error("tmux should be reported missing")
	}
}
