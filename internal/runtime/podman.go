// Package runtime adapts a container runtime (Podman) for NeonRoot. Container
// storage is relocated onto tmpfs so image layers live in RAM alongside the
// hydrated workspace — unplugging the external drive never strands container
// state.
//
// NOTE: rootless Podman with its graphroot on tmpfs exercises user-namespace
// overlay / fuse-overlayfs in image-dependent ways. The adapter and its
// argument construction are unit-tested here; running real containers is
// validated by the //go:build integration suite on target hardware.
package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Runtime is the container-runtime capability the rest of NeonRoot depends on.
// Keeping it small lets sessions and (later) commit orchestration swap in a fake.
type Runtime interface {
	// Available reports whether the runtime binary is usable.
	Available() bool
	// Version returns the runtime's client version string.
	Version(ctx context.Context) (string, error)
	// Run starts a detached container from image with the given options and
	// returns its container ID.
	Run(ctx context.Context, spec RunSpec) (string, error)
	// Stop stops (and removes) a container by ID or name.
	Stop(ctx context.Context, id string) error
}

// RunSpec describes a container to start.
type RunSpec struct {
	Image string
	Name  string
	// WorkspaceDir is bind-mounted into the container as the working directory.
	WorkspaceDir string
	// MountTarget is where WorkspaceDir appears inside the container.
	MountTarget string
	// Command overrides the image's default command, if set.
	Command []string
	// Pod, if set, joins the container to that pod (shared network).
	Pod string
}

// Podman is the exec-backed Runtime. GraphRoot/RunRoot relocate storage onto
// tmpfs; both are passed to every invocation so the process never touches the
// default (SD-card-backed) container store.
type Podman struct {
	Runner    platform.Runner
	GraphRoot string
	RunRoot   string
}

// baseArgs are prepended to every podman call to pin storage onto tmpfs.
func (p *Podman) baseArgs() []string {
	return []string{"--root", p.GraphRoot, "--runroot", p.RunRoot}
}

// Available reports whether podman is on PATH.
func (p *Podman) Available() bool {
	_, err := p.Runner.LookPath("podman")
	return err == nil
}

// Version returns the podman client version.
func (p *Podman) Version(ctx context.Context) (string, error) {
	args := append(p.baseArgs(), "version", "--format", "{{.Client.Version}}")
	out, err := p.Runner.Run(ctx, "podman", args...)
	return strings.TrimSpace(string(out)), err
}

// Run starts a detached container and returns its ID. Images are always local
// base images, so --pull=never makes a missing image fail fast instead of
// hitting a registry.
func (p *Podman) Run(ctx context.Context, spec RunSpec) (string, error) {
	args := append(p.baseArgs(), "run", "-d", "--pull=never")
	if spec.Pod != "" {
		args = append(args, "--pod", spec.Pod)
	}
	if spec.Name != "" {
		args = append(args, "--name", spec.Name)
	}
	if spec.WorkspaceDir != "" {
		target := spec.MountTarget
		if target == "" {
			target = "/workspace"
		}
		args = append(args, "-v", spec.WorkspaceDir+":"+target, "-w", target)
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)

	out, err := p.Runner.Run(ctx, "podman", args...)
	return strings.TrimSpace(string(out)), err
}

// Stop stops and removes a container.
func (p *Podman) Stop(ctx context.Context, id string) error {
	args := append(p.baseArgs(), "rm", "-f", id)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// Start launches a long-lived detached container for a workspace (kept alive
// with `sleep infinity`) with the tmpfs workspace bind-mounted at mountTarget
// (defaults to /workspace), and returns its container ID. A session then execs a
// shell into it.
func (p *Podman) Start(ctx context.Context, image, name, workspaceDir, mountTarget string) (string, error) {
	return p.Run(ctx, RunSpec{
		Image:        image,
		Name:         name,
		WorkspaceDir: workspaceDir,
		MountTarget:  mountTarget,
		Command:      []string{"sleep", "infinity"},
	})
}

// StartPod starts a workspace whose image list is a pod: the primary image
// (imageRefs[0]) runs with the workspace bind-mounted and is where the shell
// attaches; the remaining images run as sidecars sharing the pod's network
// (reachable over localhost). Returns the primary container's ID.
func (p *Podman) StartPod(ctx context.Context, podName string, imageRefs []string, primaryName, workspaceDir, mountTarget string) (string, error) {
	args := append(p.baseArgs(), "pod", "create", "--name", podName)
	if _, err := p.Runner.Run(ctx, "podman", args...); err != nil {
		return "", err
	}
	primaryID, err := p.Run(ctx, RunSpec{
		Pod: podName, Image: imageRefs[0], Name: primaryName,
		WorkspaceDir: workspaceDir, MountTarget: mountTarget,
		Command: []string{"sleep", "infinity"},
	})
	if err != nil {
		return "", err
	}
	for i, ref := range imageRefs[1:] {
		if _, err := p.Run(ctx, RunSpec{
			Pod: podName, Image: ref,
			Name: fmt.Sprintf("%s-side%d", primaryName, i+1),
		}); err != nil {
			return "", err
		}
	}
	return primaryID, nil
}

// StopPod stops and removes a pod and all its containers.
func (p *Podman) StopPod(ctx context.Context, name string) error {
	args := append(p.baseArgs(), "pod", "rm", "-f", name)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// ImageExists reports whether an image reference is present in the tmpfs store.
func (p *Podman) ImageExists(ctx context.Context, ref string) bool {
	args := append(p.baseArgs(), "image", "exists", ref)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err == nil
}

// LoadImage loads a `podman save` tarball into the store. The tar is read
// straight from the (mounted) vault path — it is never staged in tmpfs, so only
// the unpacked layers occupy RAM.
func (p *Podman) LoadImage(ctx context.Context, tarPath string) error {
	args := append(p.baseArgs(), "load", "-i", tarPath)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// EnsureImage makes sure ref is in the store, loading it from tarPath if absent
// (or always when reload is set).
func (p *Podman) EnsureImage(ctx context.Context, ref, tarPath string, reload bool) error {
	if !reload && p.ImageExists(ctx, ref) {
		return nil
	}
	return p.LoadImage(ctx, tarPath)
}

// Build builds an image from a Containerfile directory and tags it ref.
func (p *Podman) Build(ctx context.Context, ref, containerfileDir string) error {
	args := append(p.baseArgs(), "build", "-t", ref, containerfileDir)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// Commit captures a running container's current state as an image (podman
// commit) under ref — how inside-container changes become durable image data.
func (p *Podman) Commit(ctx context.Context, containerID, ref string) error {
	args := append(p.baseArgs(), "commit", containerID, ref)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// Tag adds a new reference to an existing image.
func (p *Podman) Tag(ctx context.Context, from, to string) error {
	args := append(p.baseArgs(), "tag", from, to)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// Save writes an image's data to a tarball (podman save), for storage in a vault.
// A prior tar is removed first — podman's docker-archive format cannot write
// over an existing archive, and a save always produces a complete new one.
func (p *Podman) Save(ctx context.Context, ref, tarPath string) error {
	_ = os.Remove(tarPath)
	args := append(p.baseArgs(), "save", "-o", tarPath, ref)
	_, err := p.Runner.Run(ctx, "podman", args...)
	return err
}

// DefaultShell opens a plain login shell in the container (bash, else sh).
// NeonRoot deliberately does NOT impose tmux here: you likely run your own tmux
// on the host, and forcing a container-side tmux would nest inside it. To work
// in the image's tmux (e.g. arch-dev's, with session saving), run `tmux` once
// inside, or set the workspace shell: `set <ws> --shell "tmux new-session -A"`.
var DefaultShell = []string{"sh", "-c",
	"if command -v bash >/dev/null 2>&1; then exec bash -l; else exec sh; fi"}

// ExecArgs returns the full command (argv) to open an interactive session inside
// a container. command overrides the shell (empty uses DefaultShell). It carries
// the tmpfs storage roots so the container in the tmpfs graphroot is found.
func (p *Podman) ExecArgs(id string, command []string) []string {
	if len(command) == 0 {
		command = DefaultShell
	}
	argv := []string{"podman"}
	argv = append(argv, p.baseArgs()...)
	argv = append(argv, "exec", "-it", id)
	return append(argv, command...)
}
