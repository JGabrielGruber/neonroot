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

// A remote write first ensures the catalog repo exists over ssh (idempotent init,
// port carried as ssh's -p), then clones it to stage the commit.
func TestCatalog_WriteRemoteInitsThenClones(t *testing.T) {
	rec := runnertest.New()
	cat := Catalog{Git: &git.Git{Runner: rec}, Runner: rec, CacheDir: t.TempDir()}
	v := domain.Vault{Name: "cloud", Remote: "ssh://git@host:2222/srv/vault"}

	// WriteIndex into the (unpopulated) clone dir fails under the recorder; we only
	// assert the transport leading up to it.
	_ = cat.Write(context.Background(), v, NewIndex())

	lines := rec.Lines()
	if len(lines) < 2 {
		t.Fatalf("expected init + clone, got %v", lines)
	}
	if !strings.Contains(lines[0], "ssh") || !strings.Contains(lines[0], "-p 2222") ||
		!strings.Contains(lines[0], "git init --bare") || !strings.Contains(lines[0], "_catalog.git") {
		t.Errorf("init line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "clone") ||
		!strings.Contains(lines[1], "ssh://git@host:2222/srv/vault/_catalog.git") {
		t.Errorf("clone line: %q", lines[1])
	}
}

// A local write is an atomic index.toml write — no git, no ssh.
func TestCatalog_WriteLocal(t *testing.T) {
	dir := t.TempDir()
	rec := runnertest.New()
	cat := Catalog{Git: &git.Git{Runner: rec}, Runner: rec, CacheDir: t.TempDir()}
	if err := cat.Write(context.Background(), domain.Vault{Name: "ext", Path: dir}, NewIndex()); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIndex(dir); err != nil {
		t.Fatalf("index not written: %v", err)
	}
	if len(rec.Calls) != 0 {
		t.Errorf("local write should not invoke git/ssh: %v", rec.Lines())
	}
}
