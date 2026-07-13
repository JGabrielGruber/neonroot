package commit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
	"github.com/JGabrielGruber/neonroot/internal/ui"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hydrateFixture builds a source tree, hydrates it into a tmpfs workspace, and
// returns (repoDir=source, wsDir=hydrated copy, manifest).
func hydrateFixture(t *testing.T) (string, string, *domain.Manifest) {
	t.Helper()
	repoDir := t.TempDir()
	write(t, filepath.Join(repoDir, "main.go"), "package main\n")
	write(t, filepath.Join(repoDir, "pkg", "a.go"), "package pkg\n")
	write(t, filepath.Join(repoDir, "doomed.txt"), "delete me\n")

	wsDir := filepath.Join(t.TempDir(), "ws")
	man, err := hydration.Hydrate("t", repoDir, wsDir, ui.New(&bytes.Buffer{}, ui.Options{}))
	if err != nil {
		t.Fatal(err)
	}
	return repoDir, wsDir, man
}

func TestDiff_DetectsAddModifyDelete(t *testing.T) {
	_, ws, man := hydrateFixture(t)

	// Mutate the hydrated tree.
	write(t, filepath.Join(ws, "pkg", "a.go"), "package pkg // edited\n") // modify
	write(t, filepath.Join(ws, "new.go"), "package new\n")                // add
	if err := os.Remove(filepath.Join(ws, "doomed.txt")); err != nil {    // delete
		t.Fatal(err)
	}

	changes, err := Diff(ws, man)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.ChangeKind{}
	for _, c := range changes {
		got[c.Path] = c.Kind
	}
	want := map[string]domain.ChangeKind{
		"new.go":     domain.ChangeAdded,
		"pkg/a.go":   domain.ChangeModified,
		"doomed.txt": domain.ChangeDeleted,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d changes %v, want %v", len(got), got, want)
	}
	for p, k := range want {
		if got[p] != k {
			t.Errorf("%s: got %s, want %s", p, got[p], k)
		}
	}
}

func TestDiff_UnchangedIsEmpty(t *testing.T) {
	_, ws, man := hydrateFixture(t)
	changes, err := Diff(ws, man)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("freshly hydrated tree should have no changes, got %v", changes)
	}
}

func TestApplyDiff_AndRebaseline(t *testing.T) {
	vault, ws, man := hydrateFixture(t)

	write(t, filepath.Join(ws, "pkg", "a.go"), "package pkg // edited\n")
	write(t, filepath.Join(ws, "new.go"), "package new\n")
	os.Remove(filepath.Join(ws, "doomed.txt"))

	changes, err := Diff(ws, man)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyDiff(ws, vault, changes); err != nil {
		t.Fatal(err)
	}

	// The vault now mirrors the workspace.
	if got := readFile(t, filepath.Join(vault, "pkg", "a.go")); got != "package pkg // edited\n" {
		t.Errorf("modified file not written back: %q", got)
	}
	if _, err := os.Stat(filepath.Join(vault, "new.go")); err != nil {
		t.Errorf("added file not written back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "doomed.txt")); !os.IsNotExist(err) {
		t.Errorf("deleted file should be gone from vault, err=%v", err)
	}

	// After re-baselining the manifest, a fresh diff is empty.
	newMan, err := UpdateManifest(man, ws, changes)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Diff(ws, newMan)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("post-commit diff should be empty, got %v", again)
	}
}

func TestHasConflict(t *testing.T) {
	base := domain.Fingerprint{Revision: 3, UpdatedAt: "t1"}
	if HasConflict(base, base) {
		t.Error("identical fingerprints must not conflict")
	}
	if !HasConflict(domain.Fingerprint{Revision: 4, UpdatedAt: "t2"}, base) {
		t.Error("changed revision must conflict")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
