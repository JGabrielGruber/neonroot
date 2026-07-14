package vault

import (
	"context"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

// A remote vault's catalog is read by cloning its _catalog.git over ssh.
func TestCatalog_ReadRemoteClonesCatalogRepo(t *testing.T) {
	rec := runnertest.New()
	cat := Catalog{Git: &git.Git{Runner: rec}, CacheDir: t.TempDir()}
	v := domain.Vault{Name: "cloud", Remote: "ssh://git@host/srv/vault"}

	// ReadIndex fails (the recorder doesn't actually populate the clone); we only
	// assert the transport — the exact ssh origin the clone targeted.
	_, _ = cat.Read(context.Background(), v)

	if len(rec.Calls) == 0 {
		t.Fatal("expected a git clone call")
	}
	line := rec.Lines()[0]
	if !strings.Contains(line, "clone") {
		t.Errorf("expected a clone, got %q", line)
	}
	if !strings.Contains(line, "ssh://git@host/srv/vault/_catalog.git") {
		t.Errorf("clone did not target the remote catalog repo: %q", line)
	}
}

// A local vault's catalog is the on-drive index.toml — no git, no network.
func TestCatalog_ReadLocalUsesIndexFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteIndex(dir, NewIndex()); err != nil {
		t.Fatal(err)
	}
	rec := runnertest.New()
	cat := Catalog{Git: &git.Git{Runner: rec}, CacheDir: t.TempDir()}

	idx, err := cat.Read(context.Background(), domain.Vault{Name: "ext", Path: dir})
	if err != nil {
		t.Fatalf("Read local: %v", err)
	}
	if idx.SchemaVersion != domain.SchemaVersion {
		t.Errorf("schema = %d, want %d", idx.SchemaVersion, domain.SchemaVersion)
	}
	if len(rec.Calls) != 0 {
		t.Errorf("local read should not invoke git: %v", rec.Lines())
	}
}
