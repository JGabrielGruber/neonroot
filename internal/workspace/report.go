package workspace

import (
	"context"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Report is a loaded workspace's live state: its git status (dirty/ahead/behind),
// tmpfs footprint, and any error querying it. It is the single source of the
// "does this workspace have unsynced work?" fact used by status, sync, doctor,
// guard, and the TUI.
type Report struct {
	Workspace domain.Workspace
	Status    git.Status
	HotBytes  int64
	// Err is a non-fatal error querying git (e.g. a corrupt clone); surfaced,
	// not swallowed, and treated as unsafe.
	Err error
}

// Unsafe reports whether the workspace holds work that would be lost on unplug or
// --clean: uncommitted changes, unpushed commits, or an unreadable git state.
func (r Report) Unsafe() bool {
	return r.Err != nil || r.Status.HasPendingWork()
}

// ReportFor computes the report for one loaded workspace by name.
func ReportFor(ctx context.Context, paths platform.Paths, g *git.Git, name string) (Report, error) {
	ws, err := ReadState(paths, name)
	if err != nil {
		return Report{}, err
	}
	return reportOf(ctx, g, *ws), nil
}

// Reports computes reports for every currently loaded workspace.
func Reports(ctx context.Context, paths platform.Paths, g *git.Git) ([]Report, error) {
	loaded, err := List(paths)
	if err != nil {
		return nil, err
	}
	out := make([]Report, 0, len(loaded))
	for _, ws := range loaded {
		out = append(out, reportOf(ctx, g, ws))
	}
	return out, nil
}

func reportOf(ctx context.Context, g *git.Git, ws domain.Workspace) Report {
	r := Report{Workspace: ws, HotBytes: HotSize(ws.Root)}
	st, err := g.Status(ctx, ws.Root)
	if err != nil {
		r.Err = err
		return r
	}
	r.Status = st
	return r
}
