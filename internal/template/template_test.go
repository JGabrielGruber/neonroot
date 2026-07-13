package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_ShippedDefault(t *testing.T) {
	dst := t.TempDir()
	if err := Write("default", t.TempDir(), dst, "myproj"); err != nil {
		t.Fatal(err)
	}
	// The dotfile must be embedded and written.
	if _, err := os.Stat(filepath.Join(dst, ".gitignore")); err != nil {
		t.Errorf(".gitignore not written: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(dst, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "myproj") || strings.Contains(string(readme), "{{workspace}}") {
		t.Errorf("placeholder not substituted:\n%s", readme)
	}
}

func TestWrite_UserOverridesShipped(t *testing.T) {
	userDir := t.TempDir()
	// A user template named "default" shadows the shipped one.
	if err := os.MkdirAll(filepath.Join(userDir, "default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "default", "marker"), []byte("user {{workspace}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := Write("default", userDir, dst, "proj"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "marker"))
	if err != nil {
		t.Fatalf("user template not used: %v", err)
	}
	if string(data) != "user proj" {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestWrite_UnknownTemplate(t *testing.T) {
	if err := Write("nope", t.TempDir(), t.TempDir(), "x"); !os.IsNotExist(err) {
		t.Errorf("expected not-exist for unknown template, got %v", err)
	}
}

func TestList_IncludesShippedAndUser(t *testing.T) {
	userDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(userDir, "mine"), 0o755)

	names := map[string]Source{}
	for _, tpl := range List(userDir) {
		names[tpl.Name] = tpl.Source
	}
	if names["default"] != Shipped {
		t.Errorf("expected shipped default, got %v", names["default"])
	}
	if names["python"] != Shipped {
		t.Errorf("expected shipped python, got %v", names["python"])
	}
	if names["mine"] != User {
		t.Errorf("expected user template 'mine', got %v", names["mine"])
	}
}

func TestScaffold(t *testing.T) {
	userDir := t.TempDir()
	path, err := Scaffold(userDir, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "README.md")); err != nil {
		t.Errorf("scaffold should seed from default: %v", err)
	}
	if _, err := Scaffold(userDir, "custom"); !os.IsExist(err) {
		t.Errorf("expected ErrExist on duplicate scaffold, got %v", err)
	}
}
