package vault

import (
	"os"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// State reports whether a vault's backing storage is reachable right now, given
// a snapshot of the mount table.
//
// The tricky case is a stale mountpoint: a directory like /mnt/ext still exists
// after the drive is unplugged, so os.Stat alone would wrongly report it
// present. We resolve it two ways:
//
//   - A readable index.toml is definitive proof the vault (and its drive) is
//     mounted and initialized.
//   - Otherwise the path must sit on a *distinct* mount (a drive or tmpfs, not
//     the SD card's root filesystem). An unmounted mountpoint dir resolves onto
//     "/" and is correctly reported unavailable; a genuinely mounted-but-empty
//     drive resolves onto its own mount and is available for initialization.
func State(vaultPath string, mounts []platform.Mount) domain.VaultState {
	info, err := os.Stat(vaultPath)
	if err != nil || !info.IsDir() {
		return domain.VaultStateUnavailable
	}
	if fileExists(IndexPath(vaultPath)) {
		return domain.VaultStateAvailable
	}
	if m, ok := platform.MountpointFor(mounts, vaultPath); ok && m.MountPoint != "/" {
		return domain.VaultStateAvailable
	}
	return domain.VaultStateUnavailable
}

// StateLive resolves availability against the current mount table.
func StateLive(vaultPath string) (domain.VaultState, error) {
	mounts, err := platform.Mounts()
	if err != nil {
		return domain.VaultStateUnknown, err
	}
	return State(vaultPath, mounts), nil
}

// StateForVault resolves a vault's state regardless of kind. A remote vault is
// VaultStateRemote (reachability is deferred to the first ssh op — no network
// probe here, to keep status/list snappy and offline-first); a local vault is
// resolved against the live mount table.
func StateForVault(v domain.Vault) (domain.VaultState, error) {
	if v.IsRemote() {
		return domain.VaultStateRemote, nil
	}
	return StateLive(v.Path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
