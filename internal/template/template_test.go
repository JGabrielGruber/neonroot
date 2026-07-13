package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDefault(t *testing.T) {
	dst := t.TempDir()
	if err := WriteDefault(dst, "myproj"); err != nil {
		t.Fatal(err)
	}

	// The dotfile must be embedded and written.
	if _, err := os.Stat(filepath.Join(dst, ".gitignore")); err != nil {
		t.Errorf(".gitignore not written: %v", err)
	}
	// The placeholder must be substituted.
	readme, err := os.ReadFile(filepath.Join(dst, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "myproj") {
		t.Errorf("workspace name not substituted:\n%s", readme)
	}
	if strings.Contains(string(readme), "{{workspace}}") {
		t.Errorf("placeholder left unsubstituted:\n%s", readme)
	}
}
