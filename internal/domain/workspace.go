package domain

// Workspace is a vault's contents hydrated into ephemeral tmpfs storage, where
// the user actually works after unplugging the drive. It carries enough
// provenance to commit changes back to cold storage later.
type Workspace struct {
	// Name identifies the workspace (matches its IndexWorkspace.Name).
	Name string `toml:"name"`
	// SourceVault is the name of the vault it was loaded from.
	SourceVault string `toml:"source_vault"`
	// Root is the absolute path to the hydrated tree in tmpfs.
	Root string `toml:"root"`
	// HydratedAt is an RFC3339 timestamp of when Load completed.
	HydratedAt string `toml:"hydrated_at"`
	// SourceFingerprint captures the origin vault's state at load time so a
	// later commit can detect whether cold storage changed underneath us.
	SourceFingerprint Fingerprint `toml:"source_fingerprint"`
	// Image is the container image the workspace runs inside, if any.
	Image string `toml:"image,omitempty"`
	// ContainerID is the running container backing this workspace, if started.
	ContainerID string `toml:"container_id,omitempty"`
}

// Fingerprint is a cheap identity of a vault's state at a point in time. The
// Revision/UpdatedAt pair is compared first; a full manifest compare is only
// needed when they differ.
type Fingerprint struct {
	Revision  int64  `toml:"revision"`
	UpdatedAt string `toml:"updated_at"`
}
