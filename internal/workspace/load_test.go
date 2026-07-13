package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
	"github.com/JGabrielGruber/neonroot/internal/ui"
)

// testEnv builds a Loader over temp dirs standing in for the drive and tmpfs,
// plus a repo containing one workspace.
func testEnv(t *testing.T) (*Loader, domain.Repo) {
	t.Helper()
	base := t.TempDir()
	paths := platform.Paths{
		Runtime:    filepath.Join(base, "run"),
		Workspaces: filepath.Join(base, "ws"),
		Cache:      filepath.Join(base, "cache"),
	}
	if err := os.MkdirAll(paths.Workspaces, 0o700); err != nil {
		t.Fatal(err)
	}

	// A repo on a "drive" with a workspace holding one file.
	drive := t.TempDir()
	wsRoot := filepath.Join(drive, "workspaces", "app")
	if err := os.MkdirAll(wsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := repo.NewIndex()
	idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: "app", Root: "workspaces/app"})
	repo.Bump(idx)
	if err := repo.WriteIndex(drive, idx); err != nil {
		t.Fatal(err)
	}

	loader := &Loader{Paths: paths, UI: ui.New(&bytes.Buffer{}, ui.Options{})}
	return loader, domain.Repo{Name: "ext", Path: drive}
}

func TestLoad_HydratesAndRecordsState(t *testing.T) {
	loader, r := testEnv(t)

	ws, err := loader.Load(r, "app")
	if err != nil {
		t.Fatal(err)
	}
	if ws.SourceRepo != "ext" || ws.SourceFingerprint.Revision != 1 {
		t.Errorf("unexpected workspace record: %+v", ws)
	}
	// Payload hydrated.
	if _, err := os.Stat(filepath.Join(ws.Root, "main.go")); err != nil {
		t.Errorf("payload not hydrated: %v", err)
	}
	// Manifest + state persisted; workspace reported as loaded.
	if _, err := os.Stat(loader.Paths.ManifestPath("app")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
	if !IsLoaded(loader.Paths, "app") {
		t.Error("IsLoaded should be true after Load")
	}

	loaded, err := List(loader.Paths)
	if err != nil || len(loaded) != 1 || loaded[0].Name != "app" {
		t.Errorf("List = %+v err=%v", loaded, err)
	}
}

func TestLoad_RefusesDoubleLoad(t *testing.T) {
	loader, r := testEnv(t)
	if _, err := loader.Load(r, "app"); err != nil {
		t.Fatal(err)
	}
	_, err := loader.Load(r, "app")
	if !errors.Is(err, domain.ErrWorkspaceExists) {
		t.Fatalf("expected ErrWorkspaceExists on double load, got %v", err)
	}
}

func TestLoad_UnknownWorkspace(t *testing.T) {
	loader, r := testEnv(t)
	_, err := loader.Load(r, "ghost")
	if !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestLoad_UnavailableRepo(t *testing.T) {
	loader, _ := testEnv(t)
	_, err := loader.Load(domain.Repo{Name: "gone", Path: "/no/such/drive"}, "app")
	if !errors.Is(err, domain.ErrRepoUnavailable) {
		t.Fatalf("expected ErrRepoUnavailable, got %v", err)
	}
}

// fakeSessions records Ensure calls and can be made to fail.
type fakeSessions struct {
	dir     string
	command []string
	err     error
}

func (f *fakeSessions) Ensure(_, dir string, command []string) error {
	f.dir = dir
	f.command = command
	return f.err
}

// fakeRuntime records container starts and returns a canned ID.
type fakeRuntime struct {
	available bool
	started   []string // images started
	id        string
}

func (f *fakeRuntime) Available() bool { return f.available }

func (f *fakeRuntime) Start(_ context.Context, image, _, _ string) (string, error) {
	f.started = append(f.started, image)
	return f.id, nil
}

func (f *fakeRuntime) ExecArgs(id string) []string {
	return []string{"podman", "exec", "-it", id, "/bin/bash"}
}

func TestLoad_StartsSessionAtWorkspaceRoot(t *testing.T) {
	loader, r := testEnv(t)
	sess := &fakeSessions{}
	loader.Sessions = sess

	ws, err := loader.Load(r, "app")
	if err != nil {
		t.Fatal(err)
	}
	if sess.dir != ws.Root {
		t.Errorf("session started at %q, want workspace root %q", sess.dir, ws.Root)
	}
}

// setImage rewrites a repo's index to give a workspace an image binding.
func setImage(t *testing.T, repoPath, ws, image string) {
	t.Helper()
	idx, err := repo.ReadIndex(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := range idx.Workspaces {
		if idx.Workspaces[i].Name == ws {
			idx.Workspaces[i].Image = image
		}
	}
	if err := repo.WriteIndex(repoPath, idx); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_StartsContainerWhenImageDeclared(t *testing.T) {
	loader, r := testEnv(t)
	setImage(t, r.Path, "app", "localhost/arch-minimal")
	rt := &fakeRuntime{available: true, id: "cid123"}
	loader.Runtime = rt
	sess := &fakeSessions{}
	loader.Sessions = sess

	ws, err := loader.Load(r, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.started) != 1 || rt.started[0] != "localhost/arch-minimal" {
		t.Errorf("container not started for declared image: %v", rt.started)
	}
	if ws.ContainerID != "cid123" {
		t.Errorf("container id not recorded: %q", ws.ContainerID)
	}
	// The session must exec into the container.
	if len(sess.command) == 0 || sess.command[0] != "podman" {
		t.Errorf("session should run the container exec command, got %v", sess.command)
	}
}

func TestLoad_NoContainerFlagStaysHostOnly(t *testing.T) {
	loader, r := testEnv(t)
	setImage(t, r.Path, "app", "localhost/arch-minimal")
	rt := &fakeRuntime{available: true, id: "cid"}
	loader.Runtime = rt
	loader.NoContainer = true
	sess := &fakeSessions{}
	loader.Sessions = sess

	ws, err := loader.Load(r, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.started) != 0 {
		t.Errorf("--no-container must not start a container, got %v", rt.started)
	}
	if ws.ContainerID != "" || len(sess.command) != 0 {
		t.Errorf("expected host-only session, got cid=%q cmd=%v", ws.ContainerID, sess.command)
	}
}

func TestLoad_HostOnlyWhenNoImage(t *testing.T) {
	loader, r := testEnv(t) // "app" has no image
	rt := &fakeRuntime{available: true, id: "cid"}
	loader.Runtime = rt
	loader.Sessions = &fakeSessions{}

	if _, err := loader.Load(r, "app"); err != nil {
		t.Fatal(err)
	}
	if len(rt.started) != 0 {
		t.Errorf("workspace without an image must not start a container")
	}
}

func TestLoad_SessionFailureDoesNotFailLoad(t *testing.T) {
	loader, r := testEnv(t)
	loader.Sessions = &fakeSessions{err: errors.New("tmux exploded")}

	ws, err := loader.Load(r, "app")
	if err != nil {
		t.Fatalf("session failure must not fail the load, got %v", err)
	}
	if !IsLoaded(loader.Paths, "app") || ws == nil {
		t.Error("workspace should be loaded despite session failure")
	}
}
