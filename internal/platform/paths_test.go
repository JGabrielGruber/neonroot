package platform

import (
	"strings"
	"testing"
)

// TestResolvePaths_SDWriteGuard is the core safety test: given a home directory
// that stands in for the SD card, every path except Config must resolve onto
// tmpfs and never underneath home. This encodes the tool's central invariant.
func TestResolvePaths_SDWriteGuard(t *testing.T) {
	const card = "/home/pi" // stand-in for the SD card's $HOME

	cases := map[string]pathsEnv{
		"xdg runtime present": {
			uid:           1000,
			home:          card,
			xdgRuntimeDir: "/run/user/1000",
			tmpDir:        "/tmp",
		},
		"no xdg runtime, /run/user exists": {
			uid:           1000,
			home:          card,
			xdgRuntimeDir: "",
			tmpDir:        "/tmp",
			dirExists:     func(string) bool { return true },
		},
		"no runtime dir at all, tmpfs fallback": {
			uid:           1000,
			home:          card,
			xdgRuntimeDir: "",
			tmpDir:        "/tmp",
			dirExists:     func(string) bool { return false },
		},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			p := resolvePaths(env)
			for _, d := range []struct {
				label, path string
			}{
				{"Runtime", p.Runtime},
				{"Workspaces", p.Workspaces},
				{"Cache", p.Cache},
			} {
				if strings.HasPrefix(d.path, card) {
					t.Errorf("%s resolved onto the SD card: %s", d.label, d.path)
				}
				if !strings.HasPrefix(d.path, "/tmp") && !strings.HasPrefix(d.path, "/run/user") {
					t.Errorf("%s not on tmpfs: %s", d.label, d.path)
				}
			}
			// Config is the one path allowed on the card.
			if !strings.HasPrefix(p.Config, card) {
				t.Errorf("Config should live under home (card): %s", p.Config)
			}
		})
	}
}

func TestResolvePaths_XDGConfigHonored(t *testing.T) {
	p := resolvePaths(pathsEnv{
		uid:           1000,
		home:          "/home/pi",
		xdgConfigHome: "/custom/config",
		xdgRuntimeDir: "/run/user/1000",
		tmpDir:        "/tmp",
	})
	if p.Config != "/custom/config/neonroot" {
		t.Errorf("XDG_CONFIG_HOME not honored: %s", p.Config)
	}
}
