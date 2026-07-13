package domain

// Workspace is a vault's contents cloned into ephemeral tmpfs storage, where the
// user works after unplugging the drive. The clone is a git working tree, so its
// dirty/ahead state is computed live from git — never persisted here (a stored
// dirty flag goes stale the moment the user runs a shell command).
type Workspace struct {
	// Name identifies the workspace (matches its IndexWorkspace.Name).
	Name string `toml:"name"`
	// SourceVault is the name of the vault it was loaded from.
	SourceVault string `toml:"source_vault"`
	// Root is the absolute path to the git working tree in tmpfs. Git stores the
	// origin (the vault's bare repo) in the clone's own .git/config.
	Root string `toml:"root"`
	// HydratedAt is an RFC3339 timestamp of when Load completed.
	HydratedAt string `toml:"hydrated_at"`
	// Images are the container images the workspace runs inside, if any.
	Images []string `toml:"images,omitempty"`
	// ContainerID is the running (primary) container backing this workspace.
	ContainerID string `toml:"container_id,omitempty"`
	// Pod is the podman pod name when the workspace runs multiple images
	// (primary + sidecars); empty for a single container or host-only.
	Pod string `toml:"pod,omitempty"`
	// Shell is the command to run when attaching into the container. Empty uses
	// the default (tmux if present, else bash).
	Shell []string `toml:"shell,omitempty"`
}

// Fingerprint is a cheap identity of a vault index's state at a point in time,
// used to guard structural (catalog) edits — not workspace content, which git
// versions.
type Fingerprint struct {
	Revision  int64  `toml:"revision"`
	UpdatedAt string `toml:"updated_at"`
}
