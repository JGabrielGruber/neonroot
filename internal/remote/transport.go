package remote

import (
	"context"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Transport moves files and runs setup over ssh for a remote vault. Git already
// handles the workspace and catalog repos; Transport covers the non-git pieces —
// fetching/uploading image tarballs (scp) and initializing bare repos on the
// server (ssh). It goes through platform.Runner so the exact scp/ssh argv is
// unit-testable without spawning.
type Transport struct {
	Runner platform.Runner
	Addr   Addr
}

// scpRemote renders the scp file spec "[user@]host:path" for a vault-relative
// path, bracketing an IPv6 host as scp requires.
func (t Transport) scpRemote(rel string) string {
	host := t.Addr.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	ui := ""
	if t.Addr.User != "" {
		ui = t.Addr.User + "@"
	}
	return ui + host + ":" + t.Addr.RemotePath(rel)
}

// scpArgs prepends the port flag (scp uses -P, not -p) when a port is set.
func (t Transport) scpArgs(src, dst string) []string {
	var args []string
	if t.Addr.Port != "" {
		args = append(args, "-P", t.Addr.Port)
	}
	return append(args, src, dst)
}

// Fetch copies a vault-relative remote file (e.g. images/dev/image.tar) to a
// local path, typically in tmpfs before a podman load.
func (t Transport) Fetch(ctx context.Context, remoteRel, localDst string) error {
	_, err := t.Runner.Run(ctx, "scp", t.scpArgs(t.scpRemote(remoteRel), localDst)...)
	return err
}

// Upload copies a local file to a vault-relative remote path (e.g. a freshly
// saved image.tar). Used when building/snapshotting images against a remote.
func (t Transport) Upload(ctx context.Context, localSrc, remoteRel string) error {
	_, err := t.Runner.Run(ctx, "scp", t.scpArgs(localSrc, t.scpRemote(remoteRel))...)
	return err
}
