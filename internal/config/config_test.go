package config

import (
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

func TestLoad_MissingFileIsEmpty(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if len(c.Repos) != 0 {
		t.Errorf("expected empty config, got %d repos", len(c.Repos))
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")

	orig := &Config{
		DefaultRepo: "ext",
		Repos: []domain.Repo{
			{Name: "ext", Path: "/mnt/ext/neonroot"},
			{Name: "backup", Path: "/mnt/backup/nr"},
		},
	}
	if err := Save(orig, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.DefaultRepo != "ext" || len(got.Repos) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	r, ok := got.Repo("backup")
	if !ok || r.Path != "/mnt/backup/nr" {
		t.Errorf("repo lookup failed: %+v ok=%v", r, ok)
	}
}

func TestAddRepo_ReplacesByName(t *testing.T) {
	c := &Config{}
	c.AddRepo(domain.Repo{Name: "ext", Path: "/old"})
	c.AddRepo(domain.Repo{Name: "ext", Path: "/new"})
	if len(c.Repos) != 1 {
		t.Fatalf("expected replace, got %d repos", len(c.Repos))
	}
	if r, _ := c.Repo("ext"); r.Path != "/new" {
		t.Errorf("expected /new, got %s", r.Path)
	}
}

func TestEnsureScratch(t *testing.T) {
	c := &Config{}
	c.EnsureScratch("/tmp/neonroot-1000/scratch")

	r, ok := c.Repo(ScratchRepoName)
	if !ok || r.Path != "/tmp/neonroot-1000/scratch" {
		t.Fatalf("scratch not added: %+v ok=%v", r, ok)
	}
	if c.DefaultRepo != ScratchRepoName {
		t.Errorf("default should fall back to scratch, got %q", c.DefaultRepo)
	}

	// Idempotent and non-clobbering.
	c.EnsureScratch("/different")
	if r, _ := c.Repo(ScratchRepoName); r.Path != "/tmp/neonroot-1000/scratch" {
		t.Errorf("EnsureScratch clobbered existing entry: %s", r.Path)
	}
}
