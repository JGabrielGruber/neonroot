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
	if len(c.Vaults) != 0 {
		t.Errorf("expected empty config, got %d vaults", len(c.Vaults))
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")

	orig := &Config{
		DefaultVault: "ext",
		Vaults: []domain.Vault{
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
	if got.DefaultVault != "ext" || len(got.Vaults) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	r, ok := got.Vault("backup")
	if !ok || r.Path != "/mnt/backup/nr" {
		t.Errorf("vault lookup failed: %+v ok=%v", r, ok)
	}
}

func TestAddVault_ReplacesByName(t *testing.T) {
	c := &Config{}
	c.AddVault(domain.Vault{Name: "ext", Path: "/old"})
	c.AddVault(domain.Vault{Name: "ext", Path: "/new"})
	if len(c.Vaults) != 1 {
		t.Fatalf("expected replace, got %d vaults", len(c.Vaults))
	}
	if r, _ := c.Vault("ext"); r.Path != "/new" {
		t.Errorf("expected /new, got %s", r.Path)
	}
}

func TestEnsureScratch(t *testing.T) {
	c := &Config{}
	c.EnsureScratch("/tmp/neonroot-1000/scratch")

	r, ok := c.Vault(ScratchVaultName)
	if !ok || r.Path != "/tmp/neonroot-1000/scratch" {
		t.Fatalf("scratch not added: %+v ok=%v", r, ok)
	}
	if c.DefaultVault != ScratchVaultName {
		t.Errorf("default should fall back to scratch, got %q", c.DefaultVault)
	}

	// Idempotent and non-clobbering.
	c.EnsureScratch("/different")
	if r, _ := c.Vault(ScratchVaultName); r.Path != "/tmp/neonroot-1000/scratch" {
		t.Errorf("EnsureScratch clobbered existing entry: %s", r.Path)
	}
}
