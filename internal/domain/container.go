package domain

// Mount is a bind-mount injected into a workspace's container: a host path
// exposed at a container path, optionally read-only. Used to pass ephemeral
// identity (the SSH agent socket, ~/.gitconfig) into a loaded workspace without
// copying anything durable.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// SessionOpts carries the optional, per-load container extras that secrets
// passthrough introduces: an env-file (podman --env-file, so secret values stay
// out of argv) and bind-mounts. Its zero value is "no extras" — the common case.
type SessionOpts struct {
	EnvFile string
	Mounts  []Mount
}
