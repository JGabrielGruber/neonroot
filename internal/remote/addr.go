// Package remote addresses and transports NeonRoot vaults that live on an ssh
// server rather than a local drive. A remote vault uses the same on-disk layout
// as a local one (index.toml catalog, workspaces/<name>.git bare repos,
// images/<name>/image.tar); only the transport differs — git and scp over ssh
// instead of the filesystem. This file is the pure addressing layer: parsing an
// ssh URL / scp-style target and building the concrete git origins and remote
// paths the rest of the system needs.
package remote

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
)

// Addr is a parsed remote vault location.
type Addr struct {
	User string // optional ssh user
	Host string // hostname or IP (IPv6 without brackets)
	Port string // optional ssh port; only carried by the ssh:// form
	Path string // vault root path on the remote
	scp  bool   // parsed from scp-style user@host:path (no scheme)
}

// Looks reports whether s should be treated as a remote address rather than a
// local filesystem path. A string with "://" is a URL; a bare "host:path" is
// scp-style as long as the part before the first colon has no slash (so a local
// path like /mnt/ext or ./a:b is not misread as remote).
func Looks(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	if i := strings.Index(s, ":"); i > 0 {
		return !strings.Contains(s[:i], "/")
	}
	return false
}

// Parse parses an ssh:// URL or an scp-style user@host:path target. The presence
// of "://" disambiguates the two: in ssh://host:22/p the :22 is a port, while in
// host:22/p (no scheme) the 22/p is a path (git's scp syntax carries no port).
func Parse(s string) (Addr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Addr{}, fmt.Errorf("empty remote address")
	}
	if strings.Contains(s, "://") {
		return parseURL(s)
	}
	return parseSCP(s)
}

func parseURL(s string) (Addr, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Addr{}, fmt.Errorf("invalid remote url %q: %w", s, err)
	}
	if u.Scheme != "ssh" {
		return Addr{}, fmt.Errorf("unsupported remote scheme %q (only ssh)", u.Scheme)
	}
	if u.Hostname() == "" {
		return Addr{}, fmt.Errorf("remote url %q has no host", s)
	}
	a := Addr{Host: u.Hostname(), Port: u.Port(), Path: u.Path}
	if u.User != nil {
		a.User = u.User.Username()
	}
	return a, nil
}

func parseSCP(s string) (Addr, error) {
	rest := s
	var user string
	if at := strings.Index(s, "@"); at >= 0 {
		user, rest = s[:at], s[at+1:]
	}
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return Addr{}, fmt.Errorf("invalid remote address %q: expected user@host:path or ssh://...", s)
	}
	host, p := rest[:colon], rest[colon+1:]
	if host == "" {
		return Addr{}, fmt.Errorf("remote address %q has no host", s)
	}
	return Addr{User: user, Host: host, Path: p, scp: true}, nil
}

// Target is the ssh/scp destination "[user@]host". The port, when set, is passed
// separately as a command flag (ssh -p / scp -P), never folded in here.
func (a Addr) Target() string {
	if a.User != "" {
		return a.User + "@" + a.Host
	}
	return a.Host
}

// RemotePath is the vault-relative path joined onto the remote's filesystem root,
// e.g. RemotePath("workspaces", "web.git") for an ssh command's argument.
func (a Addr) RemotePath(sub ...string) string {
	return path.Join(append([]string{a.Path}, sub...)...)
}

// SSHURL builds a git-usable origin for a sub-path of the vault, preserving the
// address form it was parsed from (scp-style stays scp-style so relative,
// home-anchored paths survive; url-style stays ssh://). Example:
// SSHURL("workspaces", "web.git").
func (a Addr) SSHURL(sub ...string) string {
	p := path.Join(append([]string{a.Path}, sub...)...)
	if a.scp {
		return a.Target() + ":" + p
	}
	p = "/" + strings.TrimPrefix(p, "/")
	hostport := a.Host
	if strings.Contains(a.Host, ":") { // bare IPv6 literal needs brackets
		hostport = "[" + a.Host + "]"
	}
	if a.Port != "" {
		hostport = net.JoinHostPort(a.Host, a.Port)
	}
	ui := ""
	if a.User != "" {
		ui = a.User + "@"
	}
	return "ssh://" + ui + hostport + p
}
