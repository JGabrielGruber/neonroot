package repo

import (
	"os"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// State reports whether a repo's backing storage is reachable right now, given
// a snapshot of the mount table.
//
// The tricky case is a stale mountpoint: a directory like /mnt/ext still exists
// after the drive is unplugged, so os.Stat alone would wrongly report it
// present. We resolve it two ways:
//
//   - A readable index.toml is definitive proof the repo (and its drive) is
//     mounted and initialized.
//   - Otherwise the path must sit on a *distinct* mount (a drive or tmpfs, not
//     the SD card's root filesystem). An unmounted mountpoint dir resolves onto
//     "/" and is correctly reported unavailable; a genuinely mounted-but-empty
//     drive resolves onto its own mount and is available for initialization.
func State(repoPath string, mounts []platform.Mount) domain.RepoState {
	info, err := os.Stat(repoPath)
	if err != nil || !info.IsDir() {
		return domain.RepoStateUnavailable
	}
	if fileExists(IndexPath(repoPath)) {
		return domain.RepoStateAvailable
	}
	if m, ok := platform.MountpointFor(mounts, repoPath); ok && m.MountPoint != "/" {
		return domain.RepoStateAvailable
	}
	return domain.RepoStateUnavailable
}

// StateLive resolves availability against the current mount table.
func StateLive(repoPath string) (domain.RepoState, error) {
	mounts, err := platform.Mounts()
	if err != nil {
		return domain.RepoStateUnknown, err
	}
	return State(repoPath, mounts), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
