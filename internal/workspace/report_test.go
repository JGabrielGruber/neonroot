package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

// statusRunner returns a recorder whose `git status`/`rev-list` answers encode a
// given dirty flag and ahead/behind counts (matching git.Git's Status parsing).
func statusRunner(dirty bool, behind, ahead int) *runnertest.Recorder {
	rec := runnertest.New()
	rec.Handler = func(_ string, args []string) ([]byte, error) {
		for _, a := range args {
			switch a {
			case "status":
				if dirty {
					return []byte(" M file\n"), nil
				}
				return nil, nil
			case "rev-list":
				return []byte("" + itoa(behind) + "\t" + itoa(ahead) + "\n"), nil
			}
		}
		return nil, nil
	}
	return rec
}

func itoa(n int) string { return string(rune('0' + n)) } // single-digit test helper

// loadedEnv sets up a Paths with one loaded workspace whose Root exists.
func loadedEnv(t *testing.T, name string) platform.Paths {
	t.Helper()
	base := t.TempDir()
	paths := platform.Paths{Runtime: filepath.Join(base, "run"), Workspaces: filepath.Join(base, "ws")}
	root := paths.WorkspaceRoot(name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(paths, &domain.Workspace{Name: name, SourceVault: "ext", Root: root}); err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestReportFor_States(t *testing.T) {
	cases := []struct {
		name         string
		dirty        bool
		ahead        int
		wantUnsafe   bool
		wantHotBytes int64
	}{
		{"clean", false, 0, false, 5},
		{"dirty", true, 0, true, 5},
		{"clean but ahead (unpushed)", false, 1, true, 5},
		{"dirty and ahead", true, 2, true, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paths := loadedEnv(t, "app")
			g := &git.Git{Runner: statusRunner(c.dirty, 0, c.ahead)}
			r, err := ReportFor(context.Background(), paths, g, "app")
			if err != nil {
				t.Fatal(err)
			}
			if r.Unsafe() != c.wantUnsafe {
				t.Errorf("Unsafe = %v, want %v (status %+v)", r.Unsafe(), c.wantUnsafe, r.Status)
			}
			if r.HotBytes != c.wantHotBytes {
				t.Errorf("HotBytes = %d, want %d", r.HotBytes, c.wantHotBytes)
			}
		})
	}
}

func TestReportFor_GitErrorIsUnsafe(t *testing.T) {
	paths := loadedEnv(t, "app")
	rec := runnertest.New()
	rec.Errs["git"] = os.ErrPermission // any git failure
	g := &git.Git{Runner: rec}
	r, err := ReportFor(context.Background(), paths, g, "app")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Unsafe() {
		t.Error("a git-status error must make the workspace unsafe")
	}
}

func TestReports_AllLoaded(t *testing.T) {
	paths := loadedEnv(t, "app")
	// Add a second loaded workspace.
	root := paths.WorkspaceRoot("api")
	_ = os.MkdirAll(root, 0o755)
	_ = WriteState(paths, &domain.Workspace{Name: "api", SourceVault: "ext", Root: root})

	g := &git.Git{Runner: statusRunner(false, 0, 0)}
	reports, err := Reports(context.Background(), paths, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
}
