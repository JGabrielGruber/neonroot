// Package git adapts the git binary for NeonRoot. Workspaces are bare git repos
// inside a vault, accessed over the filesystem (no server): `load` clones into
// tmpfs, `commit` commits + pushes back. Git is distributed, so this is natively
// offline — clone while the drive is plugged, commit untethered, push on replug.
//
// The adapter goes through a platform.Runner seam so it is unit-testable without
// spawning git.
package git

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// defaultBranch is the branch NeonRoot standardizes on. `git init --bare` leaves
// HEAD at refs/heads/master; we pin it to this so clones default correctly.
const defaultBranch = "main"

// Git drives the git binary.
type Git struct {
	Runner platform.Runner
}

// Available reports whether git is on PATH.
func (g *Git) Available() bool {
	_, err := g.Runner.LookPath("git")
	return err == nil
}

// run invokes git, optionally inside a working tree (-C dir when dir != "").
func (g *Git) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	return g.Runner.Run(ctx, "git", full...)
}

// InitBare creates a bare repo and pins its default branch, so a later clone
// checks out `main` rather than an empty `master`.
func (g *Git) InitBare(ctx context.Context, barePath string) error {
	if _, err := g.run(ctx, "", "init", "--bare", "-q", barePath); err != nil {
		return err
	}
	_, err := g.run(ctx, "", "--git-dir", barePath, "symbolic-ref", "HEAD", "refs/heads/"+defaultBranch)
	return err
}

// SeedContent turns a directory of files into the initial commit of a bare repo:
// it inits a throwaway working tree in contentDir, commits everything on the
// default branch, and pushes to barePath. contentDir is expected to be temporary.
func (g *Git) SeedContent(ctx context.Context, barePath, contentDir string) error {
	if err := g.InitBare(ctx, barePath); err != nil {
		return err
	}
	steps := [][]string{
		{"init", "-q", "-b", defaultBranch},
		{"add", "-A"},
		{"-c", "user.name=neonroot", "-c", "user.email=neonroot@localhost", "commit", "-q", "-m", "initial"},
		{"push", "-q", barePath, defaultBranch},
	}
	for _, s := range steps {
		if _, err := g.run(ctx, contentDir, s...); err != nil {
			return err
		}
	}
	return nil
}

// Clone clones a bare repo into dst over the filesystem. --no-hardlinks makes the
// tmpfs clone independent of the drive (so an unplug can't corrupt it);
// --single-branch keeps only the default branch to bound RAM.
func (g *Git) Clone(ctx context.Context, origin, dst string) error {
	_, err := g.run(ctx, "", "clone", "-q", "--no-hardlinks", "--single-branch",
		"--branch", defaultBranch, origin, dst)
	return err
}

// Status is a workspace's live git state.
type Status struct {
	Dirty  bool // working-tree changes (uncommitted)
	Ahead  int  // commits not yet pushed to origin
	Behind int  // commits on origin not yet pulled
}

// HasPendingWork reports whether there is anything a `commit` would preserve:
// working-tree dirt OR unpushed commits. Both are precious — a clean-but-ahead
// clone (committed offline, not yet pushed) must be treated as dirty so a
// re-load never wipes it without an explicit --clean.
func (s Status) HasPendingWork() bool { return s.Dirty || s.Ahead > 0 }

// PendingWork reports whether a clone has anything a commit would preserve
// (working-tree dirt or unpushed commits). Used to gate non-destructive reuse.
func (g *Git) PendingWork(ctx context.Context, worktree string) (bool, error) {
	st, err := g.Status(ctx, worktree)
	if err != nil {
		return false, err
	}
	return st.HasPendingWork(), nil
}

// Status reports the working tree's dirt and its ahead/behind vs origin.
func (g *Git) Status(ctx context.Context, worktree string) (Status, error) {
	var st Status

	out, err := g.run(ctx, worktree, "status", "--porcelain")
	if err != nil {
		return st, err
	}
	st.Dirty = len(strings.TrimSpace(string(out))) > 0

	// rev-list --count --left-right @{u}...HEAD -> "<behind>\t<ahead>".
	out, err = g.run(ctx, worktree, "rev-list", "--count", "--left-right", "@{u}...HEAD")
	if err != nil {
		// No upstream / unborn branch: treat as no ahead/behind info.
		return st, nil
	}
	fields := strings.Fields(string(out))
	if len(fields) == 2 {
		st.Behind, _ = strconv.Atoi(fields[0])
		st.Ahead, _ = strconv.Atoi(fields[1])
	}
	return st, nil
}

// CommitAll stages everything and commits. It returns committed=false (no error)
// when the working tree is clean so `commit` can still proceed to push any
// unpushed history.
func (g *Git) CommitAll(ctx context.Context, worktree, msg string) (committed bool, err error) {
	if _, err := g.run(ctx, worktree, "add", "-A"); err != nil {
		return false, err
	}
	// Nothing staged -> skip commit (diff --cached --quiet exits 0 when clean).
	if _, err := g.run(ctx, worktree, "diff", "--cached", "--quiet"); err == nil {
		return false, nil
	}
	if _, err := g.run(ctx, worktree,
		"-c", "user.name=neonroot", "-c", "user.email=neonroot@localhost",
		"commit", "-q", "-m", msg); err != nil {
		return false, err
	}
	return true, nil
}

// Push pushes the default branch to origin. rejected=true (no error) signals a
// non-fast-forward — the vault moved ahead, i.e. NeonRoot's conflict — so the
// caller can offer --rebase/--as/--force-with-lease instead of failing hard.
func (g *Git) Push(ctx context.Context, worktree string) (rejected bool, err error) {
	_, err = g.run(ctx, worktree, "push", "-q", "origin", defaultBranch)
	if err == nil {
		return false, nil
	}
	if isNonFastForward(err) {
		return true, nil
	}
	return false, err
}

// isNonFastForward inspects a push error's stderr for git's rejection markers.
func isNonFastForward(err error) bool {
	var runErr *platform.RunError
	if !errors.As(err, &runErr) {
		return false
	}
	s := runErr.Stderr
	return strings.Contains(s, "non-fast-forward") ||
		strings.Contains(s, "fetch first") ||
		strings.Contains(s, "[rejected]")
}
