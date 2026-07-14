package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
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
		"run -d --pull=never --replace --name nr-webapp -v /tmp/nr/workspaces/webapp:/workspace -w /workspace " +
		"localhost/arch-minimal sleep infinity"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("run args:\n got %q\nwant %q", got, want)
	}
}

func TestPodman_StartKeepsAlive(t *testing.T) {
	rec := runnertest.New()
	rec.Stdout["podman"] = "cid\n"
	if _, err := newPodman(rec).Start(context.Background(), "img", "nr-app", "/tmp/nr/workspaces/app", "/code", []string{"3000", "5432:5432"}, domain.SessionOpts{}); err != nil {
		t.Fatal(err)
	}
	want := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers " +
		"run -d --pull=never --replace --name nr-app -v /tmp/nr/workspaces/app:/code -w /code " +
		"-p 3000:3000 -p 5432:5432 img sleep infinity"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("start args:\n got %q\nwant %q", got, want)
	}
}

// Secrets extras: read-only bind-mounts and an --env-file (values kept out of argv).
func TestPodman_StartWithSecrets(t *testing.T) {
	rec := runnertest.New()
	rec.Stdout["podman"] = "cid\n"
	opts := domain.SessionOpts{
		EnvFile: "/run/user/1000/nr/secrets/app.env",
		Mounts: []domain.Mount{
			{Source: "/run/user/1000/ssh-agent.sock", Target: "/ssh-agent"},
			{Source: "/home/me/.gitconfig", Target: "/root/.gitconfig", ReadOnly: true},
		},
	}
	if _, err := newPodman(rec).Start(context.Background(), "img", "nr-app", "/tmp/ws", "/workspace", nil, opts); err != nil {
		t.Fatal(err)
	}
	want := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers " +
		"run -d --pull=never --replace --name nr-app -v /tmp/ws:/workspace -w /workspace " +
		"-v /run/user/1000/ssh-agent.sock:/ssh-agent -v /home/me/.gitconfig:/root/.gitconfig:ro " +
		"--env-file /run/user/1000/nr/secrets/app.env img sleep infinity"
	if got := rec.Lines()[0]; got != want {
		t.Errorf("start+secrets args:\n got %q\nwant %q", got, want)
	}
}

func TestPodman_ExecArgs(t *testing.T) {
	p := newPodman(runnertest.New())
	base := []string{"podman", "--root", "/tmp/nr/containers", "--runroot", "/run/user/1000/nr/containers", "exec", "-it", "cid"}

	// Default: the tmux-preferring shell.
	got := p.ExecArgs("cid", nil)
	if len(got) != len(base)+len(DefaultShell) {
		t.Fatalf("default ExecArgs = %v", got)
	}
	if got[len(got)-1] != DefaultShell[len(DefaultShell)-1] {
		t.Errorf("default should end with the shell command, got %v", got)
	}

	// Explicit command overrides.
	got = p.ExecArgs("cid", []string{"bash"})
	want := append(append([]string{}, base...), "bash")
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ExecArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPodman_ImageOps(t *testing.T) {
	rec := runnertest.New()
	p := newPodman(rec)
	ctx := context.Background()
	if err := p.LoadImage(ctx, "/vault/images/x/image.tar"); err != nil {
		t.Fatal(err)
	}
	if err := p.Build(ctx, "localhost/neonroot-x:latest", "/vault/images/x"); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(ctx, "localhost/neonroot-x:latest", "/vault/images/x/image.tar"); err != nil {
		t.Fatal(err)
	}
	base := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers "
	want := []string{
		base + "load -i /vault/images/x/image.tar",
		base + "build -t localhost/neonroot-x:latest /vault/images/x",
		base + "save -o /vault/images/x/image.tar localhost/neonroot-x:latest",
	}
	for i, w := range want {
		if rec.Lines()[i] != w {
			t.Errorf("line %d:\n got %q\nwant %q", i, rec.Lines()[i], w)
		}
	}
}

func TestPodman_EnsureImage_SkipsWhenPresent(t *testing.T) {
	rec := runnertest.New() // image exists -> no load
	if err := newPodman(rec).EnsureImage(context.Background(), "ref", "/tar", false); err != nil {
		t.Fatal(err)
	}
	for _, l := range rec.Lines() {
		if strings.Contains(l, "load") {
			t.Errorf("should not load when image present: %v", rec.Lines())
		}
	}
}

func TestPodman_StartPod(t *testing.T) {
	rec := runnertest.New()
	if _, err := newPodman(rec).StartPod(context.Background(), "nr-app",
		[]string{"img-primary", "img-side"}, "nr-app", "/tmp/ws", "/workspace", []string{"3000"}, domain.SessionOpts{}); err != nil {
		t.Fatal(err)
	}
	base := "podman --root /tmp/nr/containers --runroot /run/user/1000/nr/containers "
	lines := rec.Lines()
	// Ports are published on the pod (which owns the network), not the containers.
	if lines[0] != base+"pod create --replace --name nr-app -p 3000:3000" {
		t.Errorf("pod create: %q", lines[0])
	}
	if lines[1] != base+"run -d --pull=never --replace --pod nr-app --name nr-app -v /tmp/ws:/workspace -w /workspace img-primary sleep infinity" {
		t.Errorf("primary: %q", lines[1])
	}
	if lines[2] != base+"run -d --pull=never --replace --pod nr-app --name nr-app-side1 img-side" {
		t.Errorf("sidecar: %q", lines[2])
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
