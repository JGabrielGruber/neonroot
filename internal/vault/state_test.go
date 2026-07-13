package vault

import (
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

func TestState(t *testing.T) {
	// A vault dir with an index is unambiguously available.
	withIndex := t.TempDir()
	if err := WriteIndex(withIndex, NewIndex()); err != nil {
		t.Fatal(err)
	}

	// An empty dir standing in for a mounted-but-uninitialized drive.
	mountedEmpty := t.TempDir()
	// An empty dir standing in for a stale, unmounted mountpoint (on root fs).
	staleEmpty := t.TempDir()

	mounts := []platform.Mount{
		{MountPoint: "/", Device: "8:2", FSType: "ext4"},
		// mountedEmpty sits on its own distinct mount → drive present.
		{MountPoint: mountedEmpty, Device: "8:16", FSType: "vfat"},
	}

	cases := []struct {
		name string
		path string
		want domain.VaultState
	}{
		{"initialized vault", withIndex, domain.VaultStateAvailable},
		{"mounted uninitialized drive", mountedEmpty, domain.VaultStateAvailable},
		{"stale unmounted mountpoint", staleEmpty, domain.VaultStateUnavailable},
		{"nonexistent path", "/nope/does/not/exist", domain.VaultStateUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := State(c.path, mounts); got != c.want {
				t.Errorf("State(%s) = %s, want %s", c.path, got, c.want)
			}
		})
	}
}
