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
	"github.com/JGabrielGruber/neonroot/internal/ui"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

// fakeGit stands in for the git adapter: Clone materializes a dir instead of
// running git, and PendingWork returns a canned value.
type fakeGit struct {
	cloned  []string
	pending bool
}

func (f *fakeGit) Clone(_ context.Context, origin, dst string) error {
	f.cloned = append(f.cloned, origin)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, "main.go"), []byte("package main\n"), 0o644)
}

func (f *fakeGit) PendingWork(context.Context, string) (bool, error) { return f.pending, nil }

func testEnv(t *testing.T) (*Loader, domain.Vault, *fakeGit) {
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

	// A vault with one workspace catalogued (a bare repo path).
	drive := t.TempDir()
	idx := vault.NewIndex()
	idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: "app", Root: "workspaces/app.git"})
	vault.Bump(idx)
	if err := vault.WriteIndex(drive, idx); err != nil {
		t.Fatal(err)
	}

	g := &fakeGit{}
	loader := &Loader{Paths: paths, UI: ui.New(&bytes.Buffer{}, ui.Options{}), Git: g}
	return loader, domain.Vault{Name: "ext", Path: drive}, g
}

func TestLoad_ClonesAndRecordsState(t *testing.T) {
	loader, v, g := testEnv(t)
	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatal(err)
	}
	if ws.SourceVault != "ext" {
		t.Errorf("source vault: %+v", ws)
	}
	if len(g.cloned) != 1 || filepath.Base(g.cloned[0]) != "app.git" {
		t.Errorf("expected one clone of app.git, got %v", g.cloned)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "main.go")); err != nil {
		t.Errorf("clone content missing: %v", err)
	}
	if !IsLoaded(loader.Paths, "app") {
		t.Error("IsLoaded should be true after Load")
	}
	loaded, err := List(loader.Paths)
	if err != nil || len(loaded) != 1 || loaded[0].Name != "app" {
		t.Errorf("List = %+v err=%v", loaded, err)
	}
}

func TestLoad_ReusesWhenAlreadyLoaded(t *testing.T) {
	loader, v, g := testEnv(t)
	if _, err := loader.Load(v, "app"); err != nil {
		t.Fatal(err)
	}
	// Second load without --clean must reuse, not re-clone.
	if _, err := loader.Load(v, "app"); err != nil {
		t.Fatal(err)
	}
	if len(g.cloned) != 1 {
		t.Errorf("reuse should not re-clone, got %d clones", len(g.cloned))
	}
}

func TestLoad_CleanReclones(t *testing.T) {
	loader, v, g := testEnv(t)
	if _, err := loader.Load(v, "app"); err != nil {
		t.Fatal(err)
	}
	loader.Clean = true
	g.pending = true // even with pending work, --clean is the explicit opt-in
	if _, err := loader.Load(v, "app"); err != nil {
		t.Fatal(err)
	}
	if len(g.cloned) != 2 {
		t.Errorf("--clean should re-clone, got %d clones", len(g.cloned))
	}
}

func TestLoad_UnknownWorkspace(t *testing.T) {
	loader, v, _ := testEnv(t)
	_, err := loader.Load(v, "ghost")
	if !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestLoad_UnavailableVault(t *testing.T) {
	loader, _, _ := testEnv(t)
	_, err := loader.Load(domain.Vault{Name: "gone", Path: "/no/such/drive"}, "app")
	if !errors.Is(err, domain.ErrVaultUnavailable) {
		t.Fatalf("expected ErrVaultUnavailable, got %v", err)
	}
}

// --- session / container seams ---

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

type fakeRuntime struct {
	available bool
	ensured   []string // image refs loaded into the store
	started   []string // image refs run as the primary container
	mount     string
	ports     []string
	opts      domain.SessionOpts
	id        string
}

func (f *fakeRuntime) Available() bool { return f.available }
func (f *fakeRuntime) EnsureImage(_ context.Context, ref, _ string, _ bool) error {
	f.ensured = append(f.ensured, ref)
	return nil
}
func (f *fakeRuntime) Start(_ context.Context, image, _, _, mountTarget string, ports []string, opts domain.SessionOpts) (string, error) {
	f.started = append(f.started, image)
	f.mount = mountTarget
	f.ports = ports
	f.opts = opts
	return f.id, nil
}
func (f *fakeRuntime) StartPod(_ context.Context, _ string, refs []string, _, _, mountTarget string, ports []string, opts domain.SessionOpts) (string, error) {
	f.started = append(f.started, refs...)
	f.mount = mountTarget
	f.ports = ports
	f.opts = opts
	return f.id, nil
}

func setImage(t *testing.T, vaultPath, ws, image string) {
	t.Helper()
	idx, err := vault.ReadIndex(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := range idx.Workspaces {
		if idx.Workspaces[i].Name == ws {
			idx.Workspaces[i].Images = []string{image}
		}
	}
	if err := vault.WriteIndex(vaultPath, idx); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_StartsSessionAtWorkspaceRoot(t *testing.T) {
	loader, v, _ := testEnv(t)
	sess := &fakeSessions{}
	loader.Sessions = sess
	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatal(err)
	}
	if sess.dir != ws.Root {
		t.Errorf("session at %q, want %q", sess.dir, ws.Root)
	}
}

func TestLoad_StartsContainerWhenImageDeclared(t *testing.T) {
	loader, v, _ := testEnv(t)
	setImage(t, v.Path, "app", "arch-go")
	rt := &fakeRuntime{available: true, id: "cid123"}
	loader.Runtime = rt
	sess := &fakeSessions{}
	loader.Sessions = sess

	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatal(err)
	}
	ref := vault.ImageRef("arch-go")
	if len(rt.ensured) != 1 || rt.ensured[0] != ref {
		t.Errorf("image not loaded into store: %v", rt.ensured)
	}
	if len(rt.started) != 1 || rt.started[0] != ref {
		t.Errorf("container not started with image ref: %v", rt.started)
	}
	if ws.ContainerID != "cid123" {
		t.Errorf("container id not recorded: %q", ws.ContainerID)
	}
	// A containerized workspace must NOT start a host tmux session — attach
	// execs into the container instead.
	if sess.dir != "" {
		t.Errorf("containerized load should not start a host session, got dir %q", sess.dir)
	}
}

func TestLoad_MultiImageStartsPod(t *testing.T) {
	loader, v, _ := testEnv(t)
	// Give the workspace two images (primary + sidecar).
	idx, _ := vault.ReadIndex(v.Path)
	idx.Workspaces[0].Images = []string{"app", "db"}
	_ = vault.WriteIndex(v.Path, idx)

	rt := &fakeRuntime{available: true, id: "primary-cid"}
	loader.Runtime = rt
	loader.Sessions = &fakeSessions{}

	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Pod == "" {
		t.Error("multi-image workspace should record a pod name")
	}
	if len(rt.started) != 2 {
		t.Errorf("pod should start primary + sidecar, got %v", rt.started)
	}
}

func TestLoad_NoContainerFlagStaysHostOnly(t *testing.T) {
	loader, v, _ := testEnv(t)
	setImage(t, v.Path, "app", "localhost/arch-minimal")
	loader.Runtime = &fakeRuntime{available: true, id: "cid"}
	loader.NoContainer = true
	sess := &fakeSessions{}
	loader.Sessions = sess
	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatal(err)
	}
	if ws.ContainerID != "" || len(sess.command) != 0 {
		t.Errorf("expected host-only, got cid=%q cmd=%v", ws.ContainerID, sess.command)
	}
}

func TestLoad_SessionFailureDoesNotFailLoad(t *testing.T) {
	loader, v, _ := testEnv(t)
	loader.Sessions = &fakeSessions{err: errors.New("tmux exploded")}
	ws, err := loader.Load(v, "app")
	if err != nil {
		t.Fatalf("session failure must not fail load: %v", err)
	}
	if !IsLoaded(loader.Paths, "app") || ws == nil {
		t.Error("workspace should be loaded despite session failure")
	}
}
