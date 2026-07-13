package domain

// SchemaVersion is the current major version of the vault index format.
// A reader rejects any index whose SchemaVersion is greater than this with
// ErrIndexVersionUnsupported rather than mis-parsing it.
const SchemaVersion = 1

// Vault is a named cold-storage location, typically a directory on an external
// drive. It is the unit that workspaces are loaded from and committed to.
// A Vault is just a name→path mapping in user config; whether its backing drive
// is currently reachable is a separate, runtime-resolved concern (VaultState).
type Vault struct {
	// Name is the user-facing identifier used on the CLI (e.g. "ext").
	Name string `toml:"name"`
	// Path is the absolute path to the vault root on the drive.
	Path string `toml:"path"`
}

// VaultState describes whether a vault's backing storage is reachable right now.
// It is resolved at command time from the live mount table, never assumed.
type VaultState int

const (
	// VaultStateUnknown means availability has not been resolved yet.
	VaultStateUnknown VaultState = iota
	// VaultStateAvailable means the backing drive is mounted and the vault path
	// resolves onto it.
	VaultStateAvailable
	// VaultStateUnavailable means the drive is not mounted (the common case for
	// the default, untethered workflow).
	VaultStateUnavailable
)

func (s VaultState) String() string {
	switch s {
	case VaultStateAvailable:
		return "available"
	case VaultStateUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// Index is the on-disk manifest at the root of a vault (index.toml). It records
// the format version for forward-compatible migration and the workspaces the
// vault contains. Revision is a monotonic counter bumped on every commit; it is
// the cheap first-line check for the "drive changed underneath you" conflict.
type Index struct {
	SchemaVersion int              `toml:"schema_version"`
	Revision      int64            `toml:"revision"`
	UpdatedAt     string           `toml:"updated_at"`
	Workspaces    []IndexWorkspace `toml:"workspace"`
}

// IndexWorkspace is a vault's record of one stored workspace.
type IndexWorkspace struct {
	Name string `toml:"name"`
	// Root is the workspace directory relative to the vault Path.
	Root string `toml:"root"`
	// Image is an optional container image the workspace runs inside. Empty
	// means host-only (no container) — a fully supported mode.
	Image string `toml:"image,omitempty"`
}
