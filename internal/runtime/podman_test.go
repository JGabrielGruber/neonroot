package runtime

import (
	"context"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

func newPodman(rec *runnertest.Recorder) *Podman {
	return &Podman{Runner: rec, GraphRoot: "/tmp/nr/containers", RunRoot: "/run/user/1000/nr/containers"}
}

// Every invocation must pin storage onto the tmpfs roots so the default
// SD-card-backed store is never touched.
func TestPodman_PinsStorageRoots(t *testing.T) {
	rec := runnertest.New()
	rec.Stdout["podman"] = "5.2.0\n"

	if _, err := newPodman(rec).Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers version --format {{.Client.Version}}"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("version args:\n got %q\nwant %q", got, want)
	}
}

func TestPodman_Version(t *testing.T) {
	rec := runnertest.New()
	rec.Stdout["podman"] = "  5.2.0  \n"
	v, err := newPodman(rec).Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "5.2.0" {
		t.Errorf("Version = %q, want 5.2.0", v)
	}
}

func TestPodman_RunBuildsArgs(t *testing.T) {
	rec := runnertest.New()
	rec.Stdout["podman"] = "abc123\n"

	id, err := newPodman(rec).Run(context.Background(), RunSpec{
		Image:        "localhost/arch-minimal",
		Name:         "nr-webapp",
		WorkspaceDir: "/tmp/nr/workspaces/webapp",
		Command:      []string{"sleep", "infinity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc123" {
		t.Errorf("id = %q, want abc123", id)
	}
	want := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers " +
		"run -d --name nr-webapp -v /tmp/nr/workspaces/webapp:/workspace -w /workspace " +
		"localhost/arch-minimal sleep infinity"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("run args:\n got %q\nwant %q", got, want)
	}
}

func TestPodman_Available(t *testing.T) {
	rec := runnertest.New()
	if !newPodman(rec).Available() {
		t.Error("podman should be available")
	}
	rec.Missing["podman"] = true
	if newPodman(rec).Available() {
		t.Error("podman should be reported missing")
	}
}
