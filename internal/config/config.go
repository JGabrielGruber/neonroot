// Package config loads and saves NeonRoot's user configuration: the registry
// of vaults (name→path) and user preferences. Config is TOML, hand-editable, and
// is the only NeonRoot data allowed to live on the SD card.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

// ScratchVaultName is the built-in volatile vault that lives on tmpfs. It is a
// staging area that disappears on reboot, so users always have a target even
// with no external drive plugged in.
const ScratchVaultName = "scratch"

// Config is the on-disk user configuration.
type Config struct {
	// DefaultVault is the vault used when a command omits an explicit target.
	DefaultVault string `toml:"default_vault"`
	// Vaults is the registry of named cold-storage locations.
	Vaults []domain.Vault `toml:"vault"`
}

// Load reads config from path. A missing file is not an error: it yields an
// empty config, so a fresh install works with zero setup.
func Load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &c, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return &c, nil
}

// Save writes config to path as TOML, creating the parent directory if needed.
// This is the one sanctioned write to the SD card; it happens only on explicit
// config changes, never during load/commit.
func Save(c *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Atomic replace so a crash never leaves a truncated config.
	return os.Rename(tmp, path)
}

// Vault returns the vault registered under name.
func (c *Config) Vault(name string) (domain.Vault, bool) {
	for _, r := range c.Vaults {
		if r.Name == name {
			return r, true
		}
	}
	return domain.Vault{}, false
}

// AddVault registers a vault, replacing any existing entry with the same name.
func (c *Config) AddVault(r domain.Vault) {
	for i := range c.Vaults {
		if c.Vaults[i].Name == r.Name {
			c.Vaults[i] = r
			return
		}
	}
	c.Vaults = append(c.Vaults, r)
}

// EnsureScratch guarantees the built-in scratch vault exists, pointing at the
// given tmpfs path. It does not overwrite a user-provided "scratch" entry.
func (c *Config) EnsureScratch(tmpfsPath string) {
	if _, ok := c.Vault(ScratchVaultName); !ok {
		c.Vaults = append(c.Vaults, domain.Vault{Name: ScratchVaultName, Path: tmpfsPath})
	}
	if c.DefaultVault == "" {
		c.DefaultVault = ScratchVaultName
	}
}
