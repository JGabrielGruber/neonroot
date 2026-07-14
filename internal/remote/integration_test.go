//go:build integration

// Integration tests exercise the real ssh/scp/rsync transport and the git-over-ssh
// catalog against a localhost sshd. They are excluded from the default build and
// run only where passwordless ssh to localhost is configured:
//
//	go test -tags integration ./internal/remote/
//
// This is where the flagged E3 risks get validated for real: scp/ssh/rsync argv
// and port handling, git clone/init-bare over ssh, and cross-device catalog
// non-fast-forward. Because the "remote" is localhost, the remote paths are also
// local files, so results are verified by reading them directly.
package remote_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/remote"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

// skipUnlessSSH skips unless passwordless, non-interactive ssh to localhost works
// (BatchMode fails fast instead of prompting), and the needed binaries exist.
func skipUnlessSSH(t *testing.T, bins ...string) {
	t.Helper()
	for _, b := range append([]string{"ssh", "scp", "git"}, bins...) {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s not installed", b)
		}
	}
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "localhost", "true")
	if err := cmd.Run(); err != nil {
		t.Skip("passwordless ssh to localhost not configured")
	}
}

// addrForms returns the two remote-address spellings for a localhost path, so the
// transport is exercised in scp-style (no port) and ssh:// (with the :22 port,
// hitting the -P/-p flag paths).
func addrForms(root string) map[string]string {
	return map[string]string{
		"scp": fmt.Sprintf("localhost:%s", root),
		"url": fmt.Sprintf("ssh://localhost:22%s", root),
	}
}

func TestIntegration_TransportRoundTrip(t *testing.T) {
	skipUnlessSSH(t)
	ctx := context.Background()
	runner := platform.ExecRunner{}

	for form, spec := range addrForms(t.TempDir()) {
		t.Run(form, func(t *testing.T) {
			addr, err := remote.Parse(spec)
			if err != nil {
				t.Fatal(err)
			}
			tr := remote.Transport{Runner: runner, Addr: addr}

			// Mkdir + Upload + Fetch a file round-trips byte-for-byte.
			if err := tr.Mkdir(ctx, "images/dev"); err != nil {
				t.Fatal(err)
			}
			payload := []byte("neon\x00payload")
			src := filepath.Join(t.TempDir(), "image.tar")
			if err := os.WriteFile(src, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tr.Upload(ctx, src, "images/dev/image.tar"); err != nil {
				t.Fatalf("upload: %v", err)
			}
			back := filepath.Join(t.TempDir(), "fetched.tar")
			if err := tr.Fetch(ctx, "images/dev/image.tar", back); err != nil {
				t.Fatalf("fetch: %v", err)
			}
			got, _ := os.ReadFile(back)
			if !bytes.Equal(got, payload) {
				t.Errorf("round-trip mismatch: %q", got)
			}

			// RemoveAll drops the remote dir (verifiable directly — it's localhost).
			if err := tr.RemoveAll(ctx, "images/dev"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(addr.Path, "images", "dev")); !os.IsNotExist(err) {
				t.Errorf("RemoveAll left the dir: %v", err)
			}
		})
	}
}

func TestIntegration_DirTransfer(t *testing.T) {
	skipUnlessSSH(t)
	ctx := context.Background()
	addr, _ := remote.Parse(fmt.Sprintf("localhost:%s", t.TempDir()))
	tr := remote.Transport{Runner: platform.ExecRunner{}, Addr: addr}

	// A local build-context dir uploaded, then fetched back into a parent.
	stage := filepath.Join(t.TempDir(), "dev")
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "Containerfile"), []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tr.Mkdir(ctx, "images"); err != nil {
		t.Fatal(err)
	}
	if err := tr.UploadDir(ctx, stage, "images"); err != nil {
		t.Fatalf("upload dir: %v", err)
	}
	parent := t.TempDir()
	if err := tr.FetchDir(ctx, "images/dev", parent); err != nil {
		t.Fatalf("fetch dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "dev", "Containerfile")); err != nil {
		t.Errorf("dir round-trip missing Containerfile: %v", err)
	}
}

func TestIntegration_Rsync(t *testing.T) {
	skipUnlessSSH(t, "rsync")
	ctx := context.Background()
	addr, _ := remote.Parse(fmt.Sprintf("localhost:%s", t.TempDir()))
	tr := remote.Transport{Runner: platform.ExecRunner{}, Addr: addr, Rsync: true}

	if err := tr.Mkdir(ctx, "images/dev"); err != nil {
		t.Fatal(err)
	}
	payload := []byte("rsynced")
	src := filepath.Join(t.TempDir(), "image.tar")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tr.Upload(ctx, src, "images/dev/image.tar"); err != nil {
		t.Fatalf("rsync upload: %v", err)
	}
	back := filepath.Join(t.TempDir(), "out.tar")
	if err := tr.Fetch(ctx, "images/dev/image.tar", back); err != nil {
		t.Fatalf("rsync fetch: %v", err)
	}
	got, _ := os.ReadFile(back)
	if !bytes.Equal(got, payload) {
		t.Errorf("rsync round-trip mismatch: %q", got)
	}
}

// The whole remote-workspace repo path: init a bare repo over ssh, seed it with an
// initial commit pushed over ssh, and clone it back checked out on main.
func TestIntegration_InitSeedCloneOverSSH(t *testing.T) {
	skipUnlessSSH(t)
	ctx := context.Background()
	runner := platform.ExecRunner{}
	addr, _ := remote.Parse(fmt.Sprintf("localhost:%s", t.TempDir()))
	tr := remote.Transport{Runner: runner, Addr: addr}
	g := &git.Git{Runner: runner}

	if err := tr.InitBare(ctx, "workspaces/web.git"); err != nil {
		t.Fatalf("init bare over ssh: %v", err)
	}
	content := t.TempDir()
	if err := os.WriteFile(filepath.Join(content, "README"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	origin := addr.SSHURL("workspaces/web.git")
	if err := g.SeedPush(ctx, origin, content); err != nil {
		t.Fatalf("seed push over ssh: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(ctx, origin, dst); err != nil {
		t.Fatalf("clone over ssh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "README")); err != nil {
		t.Errorf("clone missing seeded file: %v", err)
	}
	branch, err := runner.Run(ctx, "git", "-C", dst, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || string(bytes.TrimSpace(branch)) != "main" {
		t.Errorf("clone not on main: %q (%v)", branch, err)
	}
}

func TestIntegration_CatalogWriteRead(t *testing.T) {
	skipUnlessSSH(t)
	ctx := context.Background()
	runner := platform.ExecRunner{}
	spec := fmt.Sprintf("localhost:%s", t.TempDir())
	v := domain.Vault{Name: "cloud", Remote: spec}
	cat := vault.Catalog{Git: &git.Git{Runner: runner}, Runner: runner, CacheDir: t.TempDir()}

	idx := vault.NewIndex()
	idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: "web", Root: "workspaces/web.git"})
	vault.Bump(idx)
	if err := cat.Write(ctx, v, idx); err != nil {
		t.Fatalf("catalog write over ssh: %v", err)
	}

	got, err := cat.Read(ctx, v)
	if err != nil {
		t.Fatalf("catalog read over ssh: %v", err)
	}
	if _, ok := vault.Workspace(got, "web"); !ok {
		t.Errorf("read-back catalog missing 'web': %+v", got.Workspaces)
	}
}

// Cross-device concurrency: two independent clones of _catalog.git both commit;
// the second push is rejected as a non-fast-forward — the mechanism the remote
// catalog relies on so two machines can't silently clobber each other.
func TestIntegration_CatalogNonFastForward(t *testing.T) {
	skipUnlessSSH(t)
	ctx := context.Background()
	runner := platform.ExecRunner{}
	spec := fmt.Sprintf("localhost:%s", t.TempDir())
	v := domain.Vault{Name: "cloud", Remote: spec}
	g := &git.Git{Runner: runner}
	cat := vault.Catalog{Git: g, Runner: runner, CacheDir: t.TempDir()}

	// Seed the catalog repo with an initial commit.
	if err := cat.Write(ctx, v, vault.NewIndex()); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	addr, _ := remote.Parse(spec)
	origin := addr.SSHURL("_catalog.git")

	clone := func(name string) string {
		dst := filepath.Join(t.TempDir(), name)
		if err := g.CloneCatalog(ctx, origin, dst); err != nil {
			t.Fatalf("clone %s: %v", name, err)
		}
		return dst
	}
	commit := func(dir, file string) {
		if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := g.CommitAll(ctx, dir, "edit "+file); err != nil {
			t.Fatal(err)
		}
	}

	// Both devices clone the same base, then each commits its own change.
	a, b := clone("a"), clone("b")
	commit(a, "a.txt")
	commit(b, "b.txt")

	// First push wins; the second (from a stale base) must be rejected.
	if rejected, err := g.Push(ctx, a); err != nil || rejected {
		t.Fatalf("first push should succeed: rejected=%v err=%v", rejected, err)
	}
	rejected, err := g.Push(ctx, b)
	if err != nil {
		t.Fatalf("second push errored unexpectedly: %v", err)
	}
	if !rejected {
		t.Error("second push should be rejected as a non-fast-forward")
	}
}
