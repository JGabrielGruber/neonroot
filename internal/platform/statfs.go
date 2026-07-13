package platform

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// FreeBytes returns the space available to an unprivileged user on the
// filesystem backing path.
func FreeBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Bavail is blocks available to unprivileged users; Bsize is block size.
	return st.Bavail * uint64(st.Bsize), nil
}

// CheckSpace verifies that at least need bytes are free on the filesystem
// backing path, returning a clear, actionable error otherwise. Hydration calls
// this up front so a copy into RAM never dies half-way with a raw ENOSPC.
func CheckSpace(path string, need uint64) error {
	free, err := FreeBytes(path)
	if err != nil {
		return fmt.Errorf("checking free space at %s: %w", path, err)
	}
	if free < need {
		return fmt.Errorf("not enough space at %s: need %s, have %s",
			path, humanBytes(need), humanBytes(free))
	}
	return nil
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
