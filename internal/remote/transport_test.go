package remote

import (
	"context"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

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
	for i, w := range want {
		if got := rec.Lines()[i]; got != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, w)
		}
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
