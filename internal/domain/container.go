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

// SessionOpts carries the optional, per-load container extras: an env-file
// (secrets, via podman --env-file so values stay out of argv), bind-mounts, and
// an isolation profile. Its zero value is "no extras" — the common case.
type SessionOpts struct {
	EnvFile string
	Mounts  []Mount
	// Sandbox, when set, locks the container down (an agent sandbox); nil is the
	// trusting default.
	Sandbox *Sandbox
}

// Sandbox is the isolation intent for an agent workspace — the inverse of a
// trusting dev container. It carries what to restrict; the runtime translates it
// to podman flags. The zero value restricts nothing.
type Sandbox struct {
	NoNetwork bool   // cut the network (untrusted code)
	DropCaps  bool   // drop all Linux capabilities
	NoNewPriv bool   // forbid privilege escalation (setuid, etc.)
	ReadOnly  bool   // read-only rootfs (with tmpfs on /tmp,/run,…); best for run-not-build
	Memory    string // memory cap, e.g. "2g"; "" leaves it unset
	PidsLimit int    // max processes; 0 leaves it unset
}

// Isolation profile names, stored per workspace.
const (
	// IsolationSandbox: no host identity, dropped caps, resource limits — but the
	// network stays up (an agent that builds/tests needs to fetch dependencies).
	IsolationSandbox = "sandbox"
	// IsolationIsolated: sandbox plus no network — for running untrusted code.
	IsolationIsolated = "isolated"
)

// SandboxFor returns the preset for a profile name. ok is false for "" (no
// sandboxing — today's trusting default).
func SandboxFor(profile string) (Sandbox, bool) {
	switch profile {
	case IsolationSandbox:
		return Sandbox{DropCaps: true, NoNewPriv: true, Memory: "2g", PidsLimit: 512}, true
	case IsolationIsolated:
		return Sandbox{NoNetwork: true, DropCaps: true, NoNewPriv: true, Memory: "2g", PidsLimit: 512}, true
	default:
		return Sandbox{}, false
	}
}
