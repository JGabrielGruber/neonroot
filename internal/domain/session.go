package domain

// Session ties a hydrated workspace to its live host-side tmux session and, if
// started, its container. tmux runs on the host so it survives the drive being
// unplugged; the container storage lives in tmpfs for the same reason.
type Session struct {
	// Name is the tmux session / socket name (derived from the workspace).
	Name string `toml:"name"`
	// Workspace is the name of the workspace this session serves.
	Workspace string `toml:"workspace"`
	// ContainerID is the Podman container backing the session, empty if none.
	ContainerID string `toml:"container_id"`
}
