package commit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/repo"
	"github.com/JGabrielGruber/neonroot/internal/ui"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

// Committer writes a loaded workspace's changes back to a repo. It assumes the
// caller has already verified the target repo is available and taken the repo
// lock.
type Committer struct {
	Paths platform.Paths
	UI    ui.Reporter
}

// Options tune a commit.
type Options struct {
	// AsName commits under a different workspace name (save-as). Empty means
	// commit under the workspace's own name.
	AsName string
	// Force overrides a conflict (in-place) or an existing target (save-as).
	Force bool
}

// Result summarizes what a commit did.
type Result struct {
	TargetRepo string
	TargetName string
	// Changes is the applied diff for an in-place commit (nil for save-as).
	Changes []domain.Change
	// SavedAs is true when the commit wrote a full copy to a new destination.
	SavedAs bool
	// FileCount is the number of files written for a save-as.
	FileCount int
	// Revision is the target repo's index revision after the commit.
	Revision int64
}

// Commit writes ws's changes to the target repo. In-place (same repo, same
// name) applies only the diff after a conflict check; otherwise it snapshots the
// whole workspace to a new name/repo (save-as).
func (c *Committer) Commit(ws *domain.Workspace, target domain.Repo, opt Options) (*Result, error) {
	manPath := c.Paths.ManifestPath(ws.Name)
	man, err := hydration.ReadManifest(manPath)
	if err != nil {
		return nil, err
	}

	idx, err := repo.ReadIndex(target.Path)
	if errors.Is(err, fs.ErrNotExist) {
		idx = repo.NewIndex()
	} else if err != nil {
		return nil, err
	}

	targetName := ws.Name
	if opt.AsName != "" {
		targetName = opt.AsName
	}
	inPlace := opt.AsName == "" && target.Name == ws.SourceRepo
	res := &Result{TargetRepo: target.Name, TargetName: targetName}

	if inPlace {
		if HasConflict(repo.Fingerprint(idx), ws.SourceFingerprint) && !opt.Force {
			return nil, fmt.Errorf("%w: repo %q advanced to revision %d (loaded at %d) — use --force to overwrite or --as <name> to save a copy",
				domain.ErrCommitConflict, target.Name, idx.Revision, ws.SourceFingerprint.Revision)
		}
		changes, err := Diff(ws.Root, man)
		if err != nil {
			return nil, err
		}
		res.Changes = changes
		if len(changes) == 0 {
			return res, nil // nothing to commit; leave the repo untouched
		}
		entry, ok := repo.Workspace(idx, ws.Name)
		if !ok {
			return nil, fmt.Errorf("%w: %q missing from repo index", domain.ErrWorkspaceNotFound, ws.Name)
		}
		if err := ApplyDiff(ws.Root, filepath.Join(target.Path, entry.Root), changes); err != nil {
			return nil, err
		}
		newMan, err := UpdateManifest(man, ws.Root, changes)
		if err != nil {
			return nil, err
		}
		if err := hydration.WriteManifest(manPath, newMan); err != nil {
			return nil, err
		}
	} else {
		existing, exists := repo.Workspace(idx, targetName)
		if exists && !opt.Force {
			return nil, fmt.Errorf("%w: %q in repo %q — use --force to overwrite",
				domain.ErrWorkspaceExists, targetName, target.Name)
		}
		root := filepath.Join("workspaces", targetName)
		if exists {
			root = existing.Root
		}
		dstRoot := filepath.Join(target.Path, root)
		if err := os.RemoveAll(dstRoot); err != nil {
			return nil, err
		}
		copied, err := hydration.Hydrate(targetName, ws.Root, dstRoot, c.UI)
		if err != nil {
			return nil, err
		}
		if !exists {
			idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: targetName, Root: root})
		}
		res.SavedAs = true
		res.FileCount = len(copied.Files)
	}

	repo.Bump(idx)
	if err := repo.WriteIndex(target.Path, idx); err != nil {
		return nil, err
	}
	res.Revision = idx.Revision

	// If we advanced the workspace's own source repo, refresh its fingerprint so
	// a later in-place commit does not see this commit as a conflict.
	if target.Name == ws.SourceRepo {
		ws.SourceFingerprint = repo.Fingerprint(idx)
		if err := workspace.WriteState(c.Paths, ws); err != nil {
			return nil, err
		}
	}
	return res, nil
}
