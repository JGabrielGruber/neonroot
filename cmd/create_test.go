package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyHostDir(t *testing.T) {
	src := t.TempDir()
	// A regular file, a nested file, an executable, and a .git dir to skip.
	mustWrite(t, filepath.Join(src, "go.mod"), "module x\n", 0o644)
	mustWrite(t, filepath.Join(src, "internal", "a.go"), "package a\n", 0o644)
	mustWrite(t, filepath.Join(src, "run.sh"), "#!/bin/sh\n", 0o755)
	mustWrite(t, filepath.Join(src, ".git", "config"), "[core]\n", 0o644)

	dst := t.TempDir()
	if err := copyHostDir(src, dst); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(dst, "go.mod")); got != "module x\n" {
		t.Errorf("go.mod = %q", got)
	}
	if got := readFile(t, filepath.Join(dst, "internal", "a.go")); got != "package a\n" {
		t.Errorf("nested file = %q", got)
	}
	// .git must be skipped (fresh history).
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git should be skipped, stat err = %v", err)
	}
	// The executable bit is preserved.
	fi, err := os.Stat(filepath.Join(dst, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("run.sh lost its exec bit: %v", fi.Mode())
	}
}

func TestCopyHostDir_NotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	mustWrite(t, f, "x", 0o644)
	if err := copyHostDir(f, t.TempDir()); err == nil {
		t.Error("copying a non-directory should error")
	}
}

func mustWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
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
