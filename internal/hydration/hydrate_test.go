package hydration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/ui"
)

func quietReporter() ui.Reporter {
	return ui.New(&bytes.Buffer{}, ui.Options{})
}

func TestHydrate_CopiesTreeAndBuildsManifest(t *testing.T) {
	src := t.TempDir()
	// Files, a nested dir, and a symlink.
	mustWrite(t, filepath.Join(src, "main.go"), "package main\n")
	mustMkdir(t, filepath.Join(src, "pkg"))
	mustWrite(t, filepath.Join(src, "pkg", "util.go"), "package pkg\n")
	if err := os.Symlink("main.go", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "ws")
	man, err := Hydrate("test", src, dst, quietReporter())
	if err != nil {
		t.Fatal(err)
	}

	// Files landed in tmpfs.
	if got := readFile(t, filepath.Join(dst, "pkg", "util.go")); got != "package pkg\n" {
		t.Errorf("nested file not copied correctly: %q", got)
	}
	// Symlink recreated.
	if link, err := os.Readlink(filepath.Join(dst, "link")); err != nil || link != "main.go" {
		t.Errorf("symlink not recreated: %q err=%v", link, err)
	}

	// Manifest has an entry per file (2 regular + 1 symlink) with hashes.
	if len(man.Files) != 3 {
		t.Fatalf("expected 3 manifest entries, got %d: %+v", len(man.Files), man.Files)
	}
	for _, e := range man.Files {
		if e.Hash == "" {
			t.Errorf("entry %s missing hash", e.Path)
		}
	}
}

func TestHydrate_PreservesMtime(t *testing.T) {
	src := t.TempDir()
	p := filepath.Join(src, "f.txt")
	mustWrite(t, p, "hi")
	info, _ := os.Stat(p)

	dst := filepath.Join(t.TempDir(), "ws")
	if _, err := Hydrate("t", src, dst, quietReporter()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.Stat(filepath.Join(dst, "f.txt"))
	if !got.ModTime().Equal(info.ModTime()) {
		t.Errorf("mtime not preserved: src=%v dst=%v", info.ModTime(), got.ModTime())
	}
}

func TestManifestRoundTrip(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a"), "aaa")
	dst := filepath.Join(t.TempDir(), "ws")
	man, err := Hydrate("rt", src, dst, quietReporter())
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "manifest.toml")
	if err := WriteManifest(path, man); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace != "rt" || len(got.Files) != 1 || got.Files[0].Path != "a" {
		t.Errorf("manifest round-trip mismatch: %+v", got)
	}
	if got.Files[0].Hash != man.Files[0].Hash {
		t.Errorf("hash not preserved through round-trip")
	}
}

func TestTreeSize(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "a"), "12345") // 5 bytes
	mustMkdir(t, filepath.Join(src, "d"))
	mustWrite(t, filepath.Join(src, "d", "b"), "678") // 3 bytes
	n, err := TreeSize(src)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Errorf("TreeSize = %d, want 8", n)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
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
