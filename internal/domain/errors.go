// Package domain holds NeonRoot's core data types and their invariants.
// It performs no I/O and imports no adapter packages, so every other
// package can depend on it without risking an import cycle.
package domain

import "errors"

// Sentinel errors. Callers match these with errors.Is; the CLI layer maps
// each to a human-readable message and an exit code (see cmd).
var (
	// ErrVaultUnavailable is returned when a vault's backing drive is not
	// currently mounted/reachable. This is an expected state, not a crash:
	// the external drive is usually absent.
	ErrVaultUnavailable = errors.New("vault unavailable: backing drive not mounted")

	// ErrVaultNotFound is returned when a vault name is not present in the
	// user's config registry.
	ErrVaultNotFound = errors.New("vault not found in config")

	// ErrWorkspaceExists is returned when loading would clobber a workspace
	// that is already hydrated in tmpfs.
	ErrWorkspaceExists = errors.New("workspace already loaded")

	// ErrWorkspaceNotFound is returned when an operation targets a workspace
	// that is not currently loaded.
	ErrWorkspaceNotFound = errors.New("workspace not loaded")

	// ErrIndexVersionUnsupported is returned when a vault index declares a
	// SchemaVersion this build cannot safely read.
	ErrIndexVersionUnsupported = errors.New("vault index schema version unsupported")

	// ErrLocked is returned when another NeonRoot process holds the lock for
	// the operation being attempted.
	ErrLocked = errors.New("another neonroot process is running")

	// ErrCommitConflict is returned when the target vault changed since the
	// workspace was loaded, so committing would overwrite newer data.
	ErrCommitConflict = errors.New("target vault changed since load")
)
