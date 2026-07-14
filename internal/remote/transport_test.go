package remote

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

var errStub = errors.New("stub failure")

func TestTransport_FetchAndUpload(t *testing.T) {
	rec := runnertest.New()
	addr, err := Parse("ssh://git@host:2222/srv/vault")
	if err != nil {
		t.Fatal(err)
	}
	tr := Transport{Runner: rec, Addr: addr}
	ctx := context.Background()

	if err := tr.Fetch(ctx, "images/dev/image.tar", "/tmp/nr/image.tar"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Upload(ctx, "/tmp/nr/image.tar", "images/dev/image.tar"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		// scp carries the port as -P (not ssh's -p).
		"scp -P 2222 git@host:/srv/vault/images/dev/image.tar /tmp/nr/image.tar",
		"scp -P 2222 /tmp/nr/image.tar git@host:/srv/vault/images/dev/image.tar",
	}
	for i, w := range want {
		if got := rec.Lines()[i]; got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, w)
		}
	}
}

func TestTransport_DirsInitAndMkdir(t *testing.T) {
	rec := runnertest.New()
	addr, _ := Parse("ssh://git@host:2222/srv/vault")
	tr := Transport{Runner: rec, Addr: addr}
	ctx := context.Background()

	if err := tr.FetchDir(ctx, "images/dev", "/tmp/build"); err != nil {
		t.Fatal(err)
	}
	if err := tr.UploadDir(ctx, "/tmp/stage/dev", "images"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Mkdir(ctx, "images"); err != nil {
		t.Fatal(err)
	}
	if err := tr.InitBare(ctx, "_catalog.git"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"scp -r -P 2222 git@host:/srv/vault/images/dev /tmp/build",
		"scp -r -P 2222 /tmp/stage/dev git@host:/srv/vault/images",
		"ssh -p 2222 git@host mkdir -p '/srv/vault/images'",
		"ssh -p 2222 git@host git init --bare -q '/srv/vault/_catalog.git' && git --git-dir='/srv/vault/_catalog.git' symbolic-ref HEAD refs/heads/main",
	}
	if err := tr.RemoveAll(ctx, "images/dev"); err != nil {
		t.Fatal(err)
	}
	want = append(want, "ssh -p 2222 git@host rm -rf '/srv/vault/images/dev'")
	for i, w := range want {
		if got := rec.Lines()[i]; got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, w)
		}
	}
}

func TestTransport_RsyncPreferred(t *testing.T) {
	rec := runnertest.New()
	addr, _ := Parse("ssh://git@host:2222/srv/vault")
	tr := Transport{Runner: rec, Addr: addr, Rsync: true}
	ctx := context.Background()

	if err := tr.Fetch(ctx, "images/dev/image.tar", "/tmp/nr/image.tar"); err != nil {
		t.Fatal(err)
	}
	if err := tr.FetchDir(ctx, "images/dev", "/tmp/build"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		// File: no -a; the port rides inside -e "ssh -p 2222"; --partial for resume.
		"rsync -e ssh -p 2222 --partial git@host:/srv/vault/images/dev/image.tar /tmp/nr/image.tar",
		// Dir: -a (archive/recursive), dest is the parent → lands at /tmp/build/dev.
		"rsync -a -e ssh -p 2222 --partial git@host:/srv/vault/images/dev /tmp/build",
	}
	for i, w := range want {
		if got := rec.Lines()[i]; got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, w)
		}
	}
}

func TestTransport_RsyncFallsBackToScp(t *testing.T) {
	rec := runnertest.New()
	rec.Errs["rsync"] = errStub // rsync present locally but fails (e.g. remote lacks it)
	addr, _ := Parse("ssh://git@host/srv/vault")
	tr := Transport{Runner: rec, Addr: addr, Rsync: true}

	if err := tr.Fetch(context.Background(), "images/dev/image.tar", "/tmp/x.tar"); err != nil {
		t.Fatal(err)
	}
	lines := rec.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected rsync attempt then scp fallback, got %v", lines)
	}
	if !strings.HasPrefix(lines[0], "rsync ") {
		t.Errorf("first attempt should be rsync: %q", lines[0])
	}
	if lines[1] != "scp git@host:/srv/vault/images/dev/image.tar /tmp/x.tar" {
		t.Errorf("fallback should be scp: %q", lines[1])
	}
}

func TestTransport_RsyncAbsentUsesScp(t *testing.T) {
	rec := runnertest.New()
	rec.Missing["rsync"] = true // rsync requested but not installed locally
	addr, _ := Parse("ssh://git@host/srv/vault")
	tr := Transport{Runner: rec, Addr: addr, Rsync: true}

	if err := tr.Fetch(context.Background(), "images/dev/image.tar", "/tmp/x.tar"); err != nil {
		t.Fatal(err)
	}
	if got := rec.Lines()[0]; !strings.HasPrefix(got, "scp ") {
		t.Errorf("should go straight to scp when rsync absent: %q", got)
	}
}

func TestTransport_NoPortAndIPv6(t *testing.T) {
	rec := runnertest.New()
	addr, _ := Parse("ssh://[::1]/srv/vault")
	tr := Transport{Runner: rec, Addr: addr}
	if err := tr.Fetch(context.Background(), "images/x/image.tar", "/tmp/x.tar"); err != nil {
		t.Fatal(err)
	}
	// No port flag; IPv6 host bracketed for scp.
	want := "scp [::1]:/srv/vault/images/x/image.tar /tmp/x.tar"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
