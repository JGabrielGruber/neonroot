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
	"path/filepath"
	"testing"

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
