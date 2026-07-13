package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestClone_Args(t *testing.T) {
	rec := runnertest.New()
	if err := (&Git{Runner: rec}).Clone(context.Background(), "/vault/ws.git", "/tmp/ws"); err != nil {
		t.Fatal(err)
	}
	want := "git clone -q --no-hardlinks --single-branch --branch main /vault/ws.git /tmp/ws"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("clone args:\n got %q\nwant %q", got, want)
	}
}

func TestInitBare_PinsDefaultBranch(t *testing.T) {
	rec := runnertest.New()
	if err := (&Git{Runner: rec}).InitBare(context.Background(), "/vault/ws.git"); err != nil {
		t.Fatal(err)
	}
	lines := rec.Lines()
	if lines[0] != "git init --bare -q /vault/ws.git" {
		t.Errorf("init: %q", lines[0])
	}
	if lines[1] != "git --git-dir /vault/ws.git symbolic-ref HEAD refs/heads/main" {
		t.Errorf("symbolic-ref: %q", lines[1])
	}
}

func TestPush_NonFastForwardDetected(t *testing.T) {
	rec := runnertest.New()
	rec.Handler = func(_ string, _ []string) ([]byte, error) {
		return nil, &platform.RunError{Name: "git", Err: errors.New("exit 1"),
			Stderr: " ! [rejected]        main -> main (non-fast-forward)"}
	}
	rejected, err := (&Git{Runner: rec}).Push(context.Background(), "/tmp/ws")
	if err != nil {
		t.Fatalf("non-ff must not be a hard error: %v", err)
	}
	if !rejected {
		t.Error("expected rejected=true for non-fast-forward push")
	}
}

func TestPush_RealErrorPropagates(t *testing.T) {
	rec := runnertest.New()
	rec.Handler = func(_ string, _ []string) ([]byte, error) {
		return nil, &platform.RunError{Name: "git", Err: errors.New("boom"),
			Stderr: "fatal: could not read from remote"}
	}
	rejected, err := (&Git{Runner: rec}).Push(context.Background(), "/tmp/ws")
	if err == nil || rejected {
		t.Errorf("a genuine push failure must propagate as error, got rejected=%v err=%v", rejected, err)
	}
}

func TestStatus_ParsesDirtyAndAheadBehind(t *testing.T) {
	rec := runnertest.New()
	rec.Handler = func(_ string, args []string) ([]byte, error) {
		switch {
		case hasArg(args, "status"):
			return []byte(" M main.go\n?? new.txt\n"), nil
		case hasArg(args, "rev-list"):
			return []byte("2\t3\n"), nil // behind 2, ahead 3
		}
		return nil, nil
	}
	st, err := (&Git{Runner: rec}).Status(context.Background(), "/tmp/ws")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Dirty || st.Behind != 2 || st.Ahead != 3 {
		t.Errorf("unexpected status: %+v", st)
	}
	if !st.HasPendingWork() {
		t.Error("dirty+ahead must be pending work")
	}
}

func TestStatus_CleanButAheadIsPending(t *testing.T) {
	rec := runnertest.New()
	rec.Handler = func(_ string, args []string) ([]byte, error) {
		if hasArg(args, "status") {
			return []byte(""), nil // clean working tree
		}
		return []byte("0\t1\n"), nil // ahead 1 (committed, unpushed)
	}
	st, _ := (&Git{Runner: rec}).Status(context.Background(), "/tmp/ws")
	if st.Dirty {
		t.Error("working tree is clean")
	}
	if !st.HasPendingWork() {
		t.Error("a clean-but-ahead clone MUST be pending work (unpushed commits are precious)")
	}
}

func TestCommitAll_NothingToCommit(t *testing.T) {
	rec := runnertest.New()
	// add succeeds; diff --cached --quiet returns success (exit 0) => nothing staged.
	committed, err := (&Git{Runner: rec}).CommitAll(context.Background(), "/tmp/ws", "msg")
	if err != nil {
		t.Fatal(err)
	}
	if committed {
		t.Error("expected committed=false when nothing is staged")
	}
	if strings.Contains(strings.Join(rec.Lines(), "\n"), "commit -q") {
		t.Error("should not run commit when nothing is staged")
	}
}
