package platform

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

// FileLock is a held advisory lock backed by an open file descriptor. The lock
// is released on Unlock or when the process exits.
type FileLock struct {
	f *os.File
}

// TryLock takes a non-blocking exclusive advisory lock (flock LOCK_EX|LOCK_NB)
// on the given path, creating the lock file if needed. If another process holds
// the lock it returns domain.ErrLocked immediately rather than waiting, so the
// CLI can print a friendly "another neonroot is running" message.
//
// Lock files belong under the runtime tmpfs, never the SD card.
func TryLock(path string) (*FileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, domain.ErrLocked
		}
		return nil, err
	}
	return &FileLock{f: f}, nil
}

// Unlock releases the lock and closes the underlying file.
func (l *FileLock) Unlock() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	closeErr := l.f.Close()
	l.f = nil
	if err != nil {
		return err
	}
	return closeErr
}
