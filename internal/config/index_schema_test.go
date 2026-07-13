package config

import (
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

// Guards that domain.Index's TOML tags match the on-disk vault index format.
// The vault package (Phase 1) reads this format with version checks; this keeps
// the type and the format from drifting apart.
func TestIndexSchemaDecodes(t *testing.T) {
	const idxTOML = `
schema_version = 1
revision = 7
updated_at = "2026-07-13T00:00:00Z"

[[workspace]]
name = "python"
root = "workspaces/python"
`
	var idx domain.Index
	if _, err := toml.Decode(idxTOML, &idx); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if idx.SchemaVersion != 1 || idx.Revision != 7 {
		t.Errorf("scalar mismatch: %+v", idx)
	}
	if len(idx.Workspaces) != 1 || idx.Workspaces[0].Name != "python" ||
		idx.Workspaces[0].Root != "workspaces/python" {
		t.Errorf("workspace mismatch: %+v", idx.Workspaces)
	}
}
