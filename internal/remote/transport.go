package remote

import (
	"context"
	"fmt"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Transport moves files and runs setup over ssh for a remote vault. Git already
// handles the workspace and catalog repos; Transport covers the non-git pieces —
// fetching/uploading image tarballs and initializing bare repos on the server.
// It goes through platform.Runner so the exact scp/ssh/rsync argv is
// unit-testable without spawning.
type Transport struct {
	Runner platform.Runner
	Addr   Addr
	// Rsync prefers rsync (resume + skip-unchanged) over scp for file/dir
	// transfers, when rsync is also present locally; on any rsync failure the
	// transfer falls back to scp once, so a remote lacking rsync still works.
	Rsync bool
	// Warn, if set, surfaces a non-fatal note (e.g. the rsync→scp fallback).
	Warn func(string)
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

// scpArgs prepends the port flag (scp uses -P, not -p) and any leading flags
// (e.g. -r) before the src/dst pair.
func (t Transport) scpArgs(flags []string, src, dst string) []string {
	args := append([]string{}, flags...)
	if t.Addr.Port != "" {
		args = append(args, "-P", t.Addr.Port)
	}
	return append(args, src, dst)
}

// Fetch copies a vault-relative remote file (e.g. images/dev/image.tar) to a
// local path, typically in tmpfs before a podman load.
func (t Transport) Fetch(ctx context.Context, remoteRel, localDst string) error {
	return t.transfer(ctx, false, t.scpRemote(remoteRel), localDst)
}

// Upload copies a local file to a vault-relative remote path (e.g. a freshly
// saved image.tar). Used when building/snapshotting images against a remote.
func (t Transport) Upload(ctx context.Context, localSrc, remoteRel string) error {
	return t.transfer(ctx, false, localSrc, t.scpRemote(remoteRel))
}

// FetchDir recursively copies a vault-relative remote directory (e.g. an image's
// build context) to a local parent, so it lands at localParent/<basename>.
func (t Transport) FetchDir(ctx context.Context, remoteRel, localParent string) error {
	return t.transfer(ctx, true, t.scpRemote(remoteRel), localParent)
}

// UploadDir recursively copies a local directory to a vault-relative remote path.
func (t Transport) UploadDir(ctx context.Context, localSrc, remoteRel string) error {
	return t.transfer(ctx, true, localSrc, t.scpRemote(remoteRel))
}

// transfer runs one copy, preferring rsync when enabled and locally available,
// else scp. A remote spec (src or dst) is "[user@]host:path"; the dest of a
// recursive copy is the parent, so both scp -r and rsync land the source at
// dest/<basename> — no trailing-slash divergence. On rsync failure it falls back
// to scp once, so a remote without rsync still succeeds.
func (t Transport) transfer(ctx context.Context, recursive bool, src, dst string) error {
	if t.useRsync() {
		if _, err := t.Runner.Run(ctx, "rsync", t.rsyncArgs(recursive, src, dst)...); err == nil {
			return nil
		} else if t.Warn != nil {
			t.Warn(fmt.Sprintf("rsync failed (%v) — falling back to scp", err))
		}
	}
	var flags []string
	if recursive {
		flags = []string{"-r"}
	}
	_, err := t.Runner.Run(ctx, "scp", t.scpArgs(flags, src, dst)...)
	return err
}

// useRsync reports whether rsync is both requested and present locally. It can't
// prove the remote has rsync — the scp fallback in transfer covers that.
func (t Transport) useRsync() bool {
	if !t.Rsync {
		return false
	}
	_, err := t.Runner.LookPath("rsync")
	return err == nil
}

// rsyncArgs builds the rsync argv: the ssh transport (carrying the port via
// -e "ssh -p N"), --partial for resume, and -a for a recursive/archive dir copy.
func (t Transport) rsyncArgs(recursive bool, src, dst string) []string {
	shell := "ssh"
	if t.Addr.Port != "" {
		shell = "ssh -p " + t.Addr.Port
	}
	var args []string
	if recursive {
		args = append(args, "-a")
	}
	args = append(args, "-e", shell, "--partial", src, dst)
	return args
}

// Mkdir creates a vault-relative remote directory (and parents) over ssh, so an
// upload target exists before scp runs.
func (t Transport) Mkdir(ctx context.Context, remoteRel string) error {
	_, err := t.Runner.Run(ctx, "ssh",
		t.sshArgs("mkdir -p "+shellArg(t.Addr.RemotePath(remoteRel)))...)
	return err
}

// sshArgs prepends the port flag (ssh uses -p, unlike scp's -P) when set.
func (t Transport) sshArgs(remoteCmd string) []string {
	var args []string
	if t.Addr.Port != "" {
		args = append(args, "-p", t.Addr.Port)
	}
	return append(args, t.Addr.Target(), remoteCmd)
}

// RemoveAll deletes a vault-relative remote path (recursively) over ssh — the
// remote counterpart of os.RemoveAll, e.g. dropping an image's images/<name> dir.
func (t Transport) RemoveAll(ctx context.Context, remoteRel string) error {
	_, err := t.Runner.Run(ctx, "ssh",
		t.sshArgs("rm -rf "+shellArg(t.Addr.RemotePath(remoteRel)))...)
	return err
}

// InitBare creates a bare git repo at a vault-relative remote path and pins its
// default branch, so a later clone checks out main. It is idempotent — re-running
// on an existing repo is harmless — which lets the catalog/workspace repos be
// created lazily on first write.
func (t Transport) InitBare(ctx context.Context, remoteRel string) error {
	p := shellArg(t.Addr.RemotePath(remoteRel))
	// "main" mirrors git.defaultBranch (unexported there); keep them in sync.
	remoteCmd := fmt.Sprintf("git init --bare -q %s && git --git-dir=%s symbolic-ref HEAD refs/heads/main", p, p)
	_, err := t.Runner.Run(ctx, "ssh", t.sshArgs(remoteCmd)...)
	return err
}

// shellArg single-quotes a string for safe interpolation into a remote shell
// command (vault paths are user-controlled config).
func shellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
