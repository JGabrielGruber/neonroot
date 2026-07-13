// Package config loads and saves NeonRoot's user configuration: the registry
// of repos (name→path) and user preferences. Config is TOML, hand-editable, and
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

// ScratchRepoName is the built-in volatile repo that lives on tmpfs. It is a
// staging area that disappears on reboot, so users always have a target even
// with no external drive plugged in.
const ScratchRepoName = "scratch"

// Config is the on-disk user configuration.
type Config struct {
	// DefaultRepo is the repo used when a command omits an explicit target.
	DefaultRepo string `toml:"default_repo"`
	// Repos is the registry of named cold-storage locations.
	Repos []domain.Repo `toml:"repo"`
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

// Repo returns the repo registered under name.
func (c *Config) Repo(name string) (domain.Repo, bool) {
	for _, r := range c.Repos {
		if r.Name == name {
			return r, true
		}
	}
	return domain.Repo{}, false
}

// AddRepo registers a repo, replacing any existing entry with the same name.
func (c *Config) AddRepo(r domain.Repo) {
	for i := range c.Repos {
		if c.Repos[i].Name == r.Name {
			c.Repos[i] = r
			return
		}
	}
	c.Repos = append(c.Repos, r)
}

// EnsureScratch guarantees the built-in scratch repo exists, pointing at the
// given tmpfs path. It does not overwrite a user-provided "scratch" entry.
func (c *Config) EnsureScratch(tmpfsPath string) {
	if _, ok := c.Repo(ScratchRepoName); !ok {
		c.Repos = append(c.Repos, domain.Repo{Name: ScratchRepoName, Path: tmpfsPath})
	}
	if c.DefaultRepo == "" {
		c.DefaultRepo = ScratchRepoName
	}
}
