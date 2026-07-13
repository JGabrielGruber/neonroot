package vault

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

func TestReadIndex_MissingIsNotExist(t *testing.T) {
	_, err := ReadIndex(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist for missing index, got %v", err)
	}
}

func TestReadIndex_RejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	future := &domain.Index{SchemaVersion: domain.SchemaVersion + 1}
	if err := WriteIndex(dir, future); err != nil {
		t.Fatal(err)
	}
	_, err := ReadIndex(dir)
	if !errors.Is(err, domain.ErrIndexVersionUnsupported) {
		t.Fatalf("expected ErrIndexVersionUnsupported, got %v", err)
	}
}

func TestWriteReadIndex_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	idx := NewIndex()
	idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: "py", Root: "workspaces/py"})
	Bump(idx)

	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != domain.SchemaVersion || got.Revision != 1 {
		t.Errorf("unexpected header: %+v", got)
	}
	if got.UpdatedAt == "" {
		t.Error("Bump should have stamped UpdatedAt")
	}
	if w, ok := Workspace(got, "py"); !ok || w.Root != "workspaces/py" {
		t.Errorf("workspace round-trip failed: %+v ok=%v", w, ok)
	}
	// No temp files left behind.
	entries, _ := filepath.Glob(filepath.Join(dir, ".index-*.tmp"))
	if len(entries) != 0 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

func TestFingerprint(t *testing.T) {
	var idx domain.Index
	_, _ = toml.Decode("schema_version=1\nrevision=9\nupdated_at=\"t\"", &idx)
	fp := Fingerprint(&idx)
	if fp.Revision != 9 || fp.UpdatedAt != "t" {
		t.Errorf("bad fingerprint: %+v", fp)
	}
}
