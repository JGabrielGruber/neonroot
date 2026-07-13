package domain

// SchemaVersion is the current major version of the repo index format.
// A reader rejects any index whose SchemaVersion is greater than this with
// ErrIndexVersionUnsupported rather than mis-parsing it.
const SchemaVersion = 1

// Repo is a named cold-storage location, typically a directory on an external
// drive. It is the unit that workspaces are loaded from and committed to.
// A Repo is just a name→path mapping in user config; whether its backing drive
// is currently reachable is a separate, runtime-resolved concern (RepoState).
type Repo struct {
	// Name is the user-facing identifier used on the CLI (e.g. "ext").
	Name string `toml:"name"`
	// Path is the absolute path to the repo root on the drive.
	Path string `toml:"path"`
}

// RepoState describes whether a repo's backing storage is reachable right now.
// It is resolved at command time from the live mount table, never assumed.
type RepoState int

const (
	// RepoStateUnknown means availability has not been resolved yet.
	RepoStateUnknown RepoState = iota
	// RepoStateAvailable means the backing drive is mounted and the repo path
	// resolves onto it.
	RepoStateAvailable
	// RepoStateUnavailable means the drive is not mounted (the common case for
	// the default, untethered workflow).
	RepoStateUnavailable
)

func (s RepoState) String() string {
	switch s {
	case RepoStateAvailable:
		return "available"
	case RepoStateUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// Index is the on-disk manifest at the root of a repo (index.toml). It records
// the format version for forward-compatible migration and the workspaces the
// repo contains. Revision is a monotonic counter bumped on every commit; it is
// the cheap first-line check for the "drive changed underneath you" conflict.
type Index struct {
	SchemaVersion int              `toml:"schema_version"`
	Revision      int64            `toml:"revision"`
	UpdatedAt     string           `toml:"updated_at"`
	Workspaces    []IndexWorkspace `toml:"workspace"`
}

// IndexWorkspace is a repo's record of one stored workspace.
type IndexWorkspace struct {
	Name string `toml:"name"`
	// Root is the workspace directory relative to the repo Path.
	Root string `toml:"root"`
	// Image is an optional container image the workspace runs inside. Empty
	// means host-only (no container) — a fully supported mode.
	Image string `toml:"image,omitempty"`
}
