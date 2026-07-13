package domain

// Manifest is the record, captured at hydration time, of every file copied
// into tmpfs. At commit time the workspace is re-walked and diffed against this
// manifest to compute what changed. It lives in tmpfs alongside the workspace.
type Manifest struct {
	// Workspace is the name of the workspace this manifest describes.
	Workspace string `toml:"workspace"`
	// Files is keyed by path relative to the workspace root.
	Files []FileEntry `toml:"file"`
}

// FileEntry records the identity of a single hydrated file. Size and ModTime
// are the fast-path comparison; Hash confirms a real change when ModTime
// differs, guarding against tmpfs↔drive mtime-granularity false positives.
type FileEntry struct {
	Path    string `toml:"path"`
	Size    int64  `toml:"size"`
	ModTime int64  `toml:"modtime"` // Unix nanoseconds
	Hash    string `toml:"hash"`    // fast non-crypto content hash
}

// ChangeKind classifies how a file differs between the load-time manifest and
// the current tmpfs state.
type ChangeKind int

const (
	ChangeAdded ChangeKind = iota
	ChangeModified
	ChangeDeleted
)

func (c ChangeKind) String() string {
	switch c {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// Change is one entry in a commit diff.
type Change struct {
	Path string
	Kind ChangeKind
}
