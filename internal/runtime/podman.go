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

// Run starts a detached container and returns its ID.
func (p *Podman) Run(ctx context.Context, spec RunSpec) (string, error) {
	args := append(p.baseArgs(), "run", "-d")
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
