//go:build integration

// Integration tests exercise a real Podman against a tmpfs graphroot. They are
// excluded from the default build and run only on target hardware:
//
//	go test -tags integration ./internal/runtime/
//
// This is where the flagged risk — rootless Podman with its graphroot on tmpfs
// (user-namespace overlay / fuse-overlayfs) — gets validated for real.
package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

func TestIntegration_PodmanVersionOnTmpfs(t *testing.T) {
	base := t.TempDir() // note: run with TMPDIR on tmpfs to be faithful
	p := &Podman{
		Runner:    platform.ExecRunner{},
		GraphRoot: filepath.Join(base, "containers"),
		RunRoot:   filepath.Join(base, "run"),
	}
	if !p.Available() {
		t.Skip("podman not installed")
	}
	for _, d := range []string{p.GraphRoot, p.RunRoot} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	v, err := p.Version(context.Background())
	if err != nil {
		t.Fatalf("podman version against tmpfs roots failed: %v", err)
	}
	t.Logf("podman client version: %s", v)
}

// TestIntegration_SecretsReachContainer validates the E3.6 secrets plumbing for
// real: a --env-file's vars and a bind-mount actually appear inside a running
// container (and a :ro mount is read-only). Uses busybox as a tiny stand-in image.
func TestIntegration_SecretsReachContainer(t *testing.T) {
	ctx := context.Background()
	// Not t.TempDir(): rootless overlay leaves subuid-owned files the default
	// cleanup can't unlink. Remove them inside the user namespace instead.
	base, err := os.MkdirTemp("", "nr-secrets-it-")
	if err != nil {
		t.Fatal(err)
	}
	p := &Podman{
		Runner:    platform.ExecRunner{},
		GraphRoot: filepath.Join(base, "containers"),
		RunRoot:   filepath.Join(base, "run"),
	}
	if !p.Available() {
		t.Skip("podman not installed")
	}
	t.Cleanup(func() {
		_, _ = p.Runner.Run(context.Background(), "podman", append(p.baseArgs(), "system", "reset", "--force")...)
		_ = exec.Command("podman", "unshare", "rm", "-rf", base).Run()
	})
	for _, d := range []string{p.GraphRoot, p.RunRoot} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	const ref = "docker.io/library/busybox:latest"
	if _, err := p.Runner.Run(ctx, "podman", append(p.baseArgs(), "pull", ref)...); err != nil {
		t.Skipf("could not pull %s (offline?): %v", ref, err)
	}

	// An env-file with a secret, and a mount dir standing in for ~/.gitconfig.
	envFile := filepath.Join(base, "secrets.env")
	if err := os.WriteFile(envFile, []byte("FOO=bar\nTOKEN=s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mountDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mountDir, "id"), []byte("identity"), 0o600); err != nil {
		t.Fatal(err)
	}

	id, err := p.Run(ctx, RunSpec{
		Image:   ref,
		Name:    "nr-secrets-it",
		Command: []string{"sleep", "300"},
		EnvFile: envFile,
		Mounts:  []domain.Mount{{Source: mountDir, Target: "/secret", ReadOnly: true}},
	})
	if err != nil {
		t.Fatalf("run with secrets: %v", err)
	}
	defer p.Stop(ctx, id)

	exec := func(args ...string) (string, error) {
		full := append(p.baseArgs(), append([]string{"exec", id}, args...)...)
		out, err := p.Runner.Run(ctx, "podman", full...)
		return strings.TrimSpace(string(out)), err
	}

	// The env-file var is present in the container's environment.
	if out, err := exec("printenv", "FOO"); err != nil || out != "bar" {
		t.Errorf("FOO in container = %q (%v), want bar", out, err)
	}
	// The bind-mount is present and readable.
	if out, err := exec("cat", "/secret/id"); err != nil || out != "identity" {
		t.Errorf("/secret/id = %q (%v), want identity", out, err)
	}
	// The :ro mount rejects writes.
	if _, err := exec("sh", "-c", "echo x > /secret/w"); err == nil {
		t.Error("read-only mount accepted a write")
	}
}
