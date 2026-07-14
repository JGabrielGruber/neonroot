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
	// Path is the absolute path to the vault root on the drive. Empty for a
	// remote vault, whose backing storage lives on an ssh server (see Remote).
	Path string `toml:"path,omitempty"`
	// Remote is the ssh URL (or scp-style target) of a vault hosted on a server
	// rather than a local drive. Empty means a local, drive-backed vault — the
	// omitempty tag keeps existing local configs byte-identical.
	Remote string `toml:"remote,omitempty"`
	// Rsync opts a remote vault into rsync (resume + skip-unchanged) for image
	// transfers, falling back to scp when rsync is absent. Ignored for local vaults.
	Rsync bool `toml:"rsync,omitempty"`
}

// IsRemote reports whether the vault is hosted over ssh rather than on a local
// drive. Remote vaults resolve availability optimistically (no mount table) and
// reach their catalog/workspaces/images over git and scp.
func (v Vault) IsRemote() bool { return v.Remote != "" }

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
	// VaultStateRemote means the vault is hosted over ssh; reachability is not
	// probed up front (offline-first), so this is a distinct, honest state rather
	// than a claim that the network is currently up.
	VaultStateRemote
)

func (s VaultState) String() string {
	switch s {
	case VaultStateAvailable:
		return "available"
	case VaultStateUnavailable:
		return "unavailable"
	case VaultStateRemote:
		return "remote"
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
	// Images are the container images the workspace runs inside. Empty means
	// host-only (no container) — a fully supported mode. The first is the
	// primary (where the shell attaches); the rest are sidecars (run in a pod).
	Images []string `toml:"images,omitempty"`
	// Mount is where the workspace is bind-mounted inside its container.
	// Defaults to /workspace when empty.
	Mount string `toml:"mount,omitempty"`
	// Shell is the command run when attaching into the container. Empty uses the
	// default (tmux if present, else bash).
	Shell []string `toml:"shell,omitempty"`
	// Ports are published from the container/pod to the host on load, so a dev
	// server is reachable at localhost. Each is "host:container" or just "port".
	Ports []string `toml:"ports,omitempty"`
	// Up is the command 'neonroot up' runs inside the container (e.g. the dev
	// server), when none is given on the command line.
	Up []string `toml:"up,omitempty"`
	// Secrets opts the workspace into identity passthrough on load: bananenv env
	// vars + the SSH agent socket + ~/.gitconfig, injected ephemerally into the
	// container (never on the card). Opt-in because it carries your identity in.
	Secrets bool `toml:"secrets,omitempty"`
}

// PrimaryImage returns the image the workspace's shell runs in, or "" for
// host-only.
func (w IndexWorkspace) PrimaryImage() string {
	if len(w.Images) == 0 {
		return ""
	}
	return w.Images[0]
}
