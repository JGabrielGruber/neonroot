// Package runnertest provides a recording platform.Runner for unit-testing
// adapters (podman, tmux, git) without spawning real processes. It records
// every invocation and returns scripted results, so tests can assert exactly
// which command and arguments an adapter built.
package runnertest

import (
	"context"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Call is one recorded invocation.
type Call struct {
	Name string
	Args []string
}

// Line renders the call as a space-joined command line for easy assertions.
func (c Call) Line() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

// Recorder implements platform.Runner, recording calls and returning canned
// responses keyed by the invoked binary name.
type Recorder struct {
	Calls []Call

	// Stdout maps a binary name to the stdout it should return.
	Stdout map[string]string
	// Errs maps a binary name to an error it should return.
	Errs map[string]error
	// Missing lists binary names that LookPath should report as absent.
	Missing map[string]bool
	// Handler, if set, fully determines the result for a call (overriding
	// Stdout/Errs). Use it when the outcome depends on the subcommand/args.
	Handler func(name string, args []string) ([]byte, error)
}

// New returns an empty Recorder.
func New() *Recorder {
	return &Recorder{
		Stdout:  map[string]string{},
		Errs:    map[string]error{},
		Missing: map[string]bool{},
	}
}

// Run records the call and returns the scripted result for name.
func (r *Recorder) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.Calls = append(r.Calls, Call{Name: name, Args: args})
	if r.Handler != nil {
		return r.Handler(name, args)
	}
	if err := r.Errs[name]; err != nil {
		return nil, &platform.RunError{Name: name, Args: args, Err: err}
	}
	return []byte(r.Stdout[name]), nil
}

// LookPath reports name as found unless listed in Missing.
func (r *Recorder) LookPath(name string) (string, error) {
	if r.Missing[name] {
		return "", &platform.RunError{Name: name, Err: errNotFound}
	}
	return "/usr/bin/" + name, nil
}

// Lines returns every recorded call as a command line.
func (r *Recorder) Lines() []string {
	out := make([]string, len(r.Calls))
	for i, c := range r.Calls {
		out[i] = c.Line()
	}
	return out
}

type sentinel string

func (s sentinel) Error() string { return string(s) }

const errNotFound = sentinel("executable file not found in $PATH")
