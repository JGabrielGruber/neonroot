// Package platform holds NeonRoot's native-Linux seams: SD-safe path
// resolution, mount-table inspection, advisory locking, free-space checks, and
// the exec runner used by adapters. Centralizing path resolution here is what
// enforces the core invariant that nothing but tiny config ever touches the
// write-sensitive SD card.
package platform

import (
	"os"
	"path/filepath"
	"strconv"
)

// Paths are NeonRoot's resolved directories. Only Config may live on the SD
// card; everything else is redirected to tmpfs so the card is never written to
// during normal operation.
type Paths struct {
	// Config holds user config (config.toml). Small and rarely written; the
	// only path allowed on the card.
	Config string
	// Runtime holds state, lock files, and per-workspace manifests. Backed by
	// the per-user runtime tmpfs (/run/user/$UID), which is small but safe.
	Runtime string
	// Workspaces holds the large hydrated payloads. Backed by /tmp, which is
	// tmpfs on the target image and typically roomier than /run/user.
	Workspaces string
	// Cache holds regenerable scratch data. tmpfs.
	Cache string
}

// pathsEnv is the pure input to path resolution, extracted so the logic can be
// unit-tested without touching the real environment.
type pathsEnv struct {
	uid           int
	home          string
	xdgConfigHome string
	xdgRuntimeDir string
	tmpDir        string
	// dirExists reports whether a directory is present (injected for tests).
	dirExists func(string) bool
}

// resolvePaths is the deterministic core of path resolution.
//
// Config: $XDG_CONFIG_HOME/neonroot, else $HOME/.config/neonroot (on card).
// Runtime: $XDG_RUNTIME_DIR/neonroot, else /run/user/$UID/neonroot if present,
// else $TMPDIR/neonroot-$UID/run (tmpfs fallbacks, never the card).
// Workspaces/Cache: always under $TMPDIR (tmpfs), never the card.
func resolvePaths(e pathsEnv) Paths {
	configBase := e.xdgConfigHome
	if configBase == "" {
		configBase = filepath.Join(e.home, ".config")
	}

	tmp := e.tmpDir
	if tmp == "" {
		tmp = "/tmp"
	}
	tmpBase := filepath.Join(tmp, "neonroot-"+strconv.Itoa(e.uid))

	var runtimeBase string
	switch {
	case e.xdgRuntimeDir != "":
		runtimeBase = e.xdgRuntimeDir
	case e.dirExists != nil && e.dirExists("/run/user/"+strconv.Itoa(e.uid)):
		runtimeBase = "/run/user/" + strconv.Itoa(e.uid)
	default:
		runtimeBase = filepath.Join(tmpBase, "run")
	}

	return Paths{
		Config:     filepath.Join(configBase, "neonroot"),
		Runtime:    filepath.Join(runtimeBase, "neonroot"),
		Workspaces: filepath.Join(tmpBase, "workspaces"),
		Cache:      filepath.Join(tmpBase, "cache"),
	}
}

// ResolvePaths resolves NeonRoot's directories from the real environment.
func ResolvePaths() Paths {
	home, _ := os.UserHomeDir()
	return resolvePaths(pathsEnv{
		uid:           os.Getuid(),
		home:          home,
		xdgConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		xdgRuntimeDir: os.Getenv("XDG_RUNTIME_DIR"),
		tmpDir:        os.TempDir(),
		dirExists:     isDir,
	})
}

// EnsureRuntimeDirs creates the tmpfs directories (0700). Config is created
// lazily by the config layer so a read-only run never writes to the card.
func (p Paths) EnsureRuntimeDirs() error {
	for _, d := range []string{p.Runtime, p.Workspaces, p.Cache} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// TemplatesDir is where user-defined workspace templates live, on the card
// alongside config (small, rarely written).
func (p Paths) TemplatesDir() string {
	return filepath.Join(p.Config, "templates")
}

// WorkspaceStateDir returns the tmpfs directory holding a loaded workspace's
// bookkeeping (state record), separate from the clone itself.
func (p Paths) WorkspaceStateDir(workspace string) string {
	return filepath.Join(p.Runtime, "workspaces", workspace)
}

// StatePath returns the tmpfs location of a loaded workspace's state record.
func (p Paths) StatePath(workspace string) string {
	return filepath.Join(p.WorkspaceStateDir(workspace), "workspace.toml")
}

// WorkspaceRoot returns the tmpfs directory a workspace hydrates into.
func (p Paths) WorkspaceRoot(workspace string) string {
	return filepath.Join(p.Workspaces, workspace)
}

// ContainersGraphRoot is Podman's image/layer store, placed on the roomier /tmp
// tmpfs so container storage lives in RAM and unplugging the drive never
// strands it.
func (p Paths) ContainersGraphRoot() string {
	return filepath.Join(filepath.Dir(p.Workspaces), "containers")
}

// ContainersRunRoot is Podman's transient runtime state, on the per-user
// runtime tmpfs.
func (p Paths) ContainersRunRoot() string {
	return filepath.Join(p.Runtime, "containers")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
