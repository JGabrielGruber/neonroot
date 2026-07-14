package remote

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name             string
		in               string
		user, host, port string
		path             string
		scp              bool
		wantErr          bool
	}{
		{name: "ssh url full", in: "ssh://git@host.example:2222/srv/vault",
			user: "git", host: "host.example", port: "2222", path: "/srv/vault"},
		{name: "ssh url no user no port", in: "ssh://host/srv/vault",
			host: "host", path: "/srv/vault"},
		{name: "ssh url ipv6 with port", in: "ssh://git@[::1]:22/p",
			user: "git", host: "::1", port: "22", path: "/p"},
		{name: "ssh url ipv6 no port", in: "ssh://[fe80::1]/p",
			host: "fe80::1", path: "/p"},
		{name: "ssh url trailing slash", in: "ssh://host/srv/vault/",
			host: "host", path: "/srv/vault/"},
		{name: "scp with user", in: "git@host:srv/vault",
			user: "git", host: "host", path: "srv/vault", scp: true},
		{name: "scp no user", in: "host:vault",
			host: "host", path: "vault", scp: true},
		// scp form carries no port: the digits after the colon are a path.
		{name: "scp colon is path not port", in: "git@host:22/vault",
			user: "git", host: "host", path: "22/vault", scp: true},
		{name: "empty", in: "", wantErr: true},
		{name: "scp no colon", in: "just-a-host", wantErr: true},
		{name: "non-ssh scheme", in: "https://host/p", wantErr: true},
		{name: "no host", in: "@host-missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %+v, want error", tt.in, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if a.User != tt.user || a.Host != tt.host || a.Port != tt.port || a.Path != tt.path || a.scp != tt.scp {
				t.Errorf("Parse(%q) = %+v (scp=%v), want user=%q host=%q port=%q path=%q scp=%v",
					tt.in, a, a.scp, tt.user, tt.host, tt.port, tt.path, tt.scp)
			}
		})
	}
}

func TestSSHURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		sub  []string
		want string
	}{
		{name: "url joins absolute", in: "ssh://git@host/srv/vault",
			sub: []string{"workspaces", "web.git"}, want: "ssh://git@host/srv/vault/workspaces/web.git"},
		{name: "url with port", in: "ssh://git@host:2222/srv/vault",
			sub: []string{"_catalog.git"}, want: "ssh://git@host:2222/srv/vault/_catalog.git"},
		{name: "url ipv6 with port", in: "ssh://git@[::1]:22/p",
			sub: []string{"x.git"}, want: "ssh://git@[::1]:22/p/x.git"},
		{name: "url ipv6 no port", in: "ssh://[fe80::1]/p",
			sub: []string{"x.git"}, want: "ssh://[fe80::1]/p/x.git"},
		{name: "url empty path", in: "ssh://host",
			sub: []string{"workspaces", "web.git"}, want: "ssh://host/workspaces/web.git"},
		{name: "scp stays scp relative", in: "git@host:vaults/mine",
			sub: []string{"workspaces", "web.git"}, want: "git@host:vaults/mine/workspaces/web.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			if got := a.SSHURL(tt.sub...); got != tt.want {
				t.Errorf("SSHURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargetAndRemotePath(t *testing.T) {
	a, err := Parse("ssh://git@host:22/srv/vault")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.Target(); got != "git@host" {
		t.Errorf("Target = %q, want git@host", got)
	}
	if got := a.RemotePath("images", "dev", "image.tar"); got != "/srv/vault/images/dev/image.tar" {
		t.Errorf("RemotePath = %q", got)
	}
	b, _ := Parse("host:vault")
	if got := b.Target(); got != "host" {
		t.Errorf("Target (no user) = %q, want host", got)
	}
}

func TestLooks(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"ssh://host/p", true},
		{"git@host:user/repo", true},
		{"host:vault", true},
		{"/mnt/ext/neonroot", false},
		{"./relative/path", false},
		{"/mnt/ext:foo", false}, // colon after a slash: a local path, not a host
		{"plainword", false},
	}
	for _, tt := range tests {
		if got := Looks(tt.in); got != tt.want {
			t.Errorf("Looks(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
