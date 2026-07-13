package platform

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
)

func TestTryLock_Contention(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "neonroot.lock")

	first, err := TryLock(lockPath)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}

	// A second attempt on the same path must be refused with ErrLocked.
	if _, err := TryLock(lockPath); !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("expected ErrLocked on contention, got %v", err)
	}

	// After release the lock is reacquirable.
	if err := first.Unlock(); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	second, err := TryLock(lockPath)
	if err != nil {
		t.Fatalf("re-lock after unlock failed: %v", err)
	}
	second.Unlock()
}
