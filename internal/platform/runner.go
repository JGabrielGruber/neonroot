package platform

import (
	"bytes"
	"context"
	"os/exec"
)

// Runner is the seam through which adapters (podman, tmux, git) invoke
// external binaries. Injecting a Runner lets those adapters be unit-tested by
// asserting the command and args without spawning a real process.
type Runner interface {
	// Run executes name with args and returns its captured stdout. stderr is
	// folded into the returned error on failure.
	Run(ctx context.Context, name string, args ...string) (stdout []byte, err error)
	// LookPath reports whether a binary is resolvable on PATH.
	LookPath(name string) (string, error)
}

// ExecRunner is the real Runner backed by os/exec.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &RunError{Name: name, Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}

// LookPath implements Runner.
func (ExecRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// RunError carries the failing command's context so callers can surface an
// actionable message.
type RunError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

func (e *RunError) Error() string {
	msg := e.Name + ": " + e.Err.Error()
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *RunError) Unwrap() error { return e.Err }
