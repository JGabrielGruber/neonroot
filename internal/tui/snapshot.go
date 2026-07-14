package tui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/platform"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

// A snapshot is one consistent read of everything the cockpit shows. It is
// gathered off the Update loop (in a tea.Cmd goroutine, under a timeout) so the
// UI never blocks on mount-table / git I/O.

type wsRow struct {
	name, vaultName string
	loaded          bool
	report          workspace.Report // valid when loaded
	images          []string
	isolation       string // "", "sandbox", or "isolated"
}

type imgRow struct {
	name  string
	built bool
	size  int64
}

type vaultRow struct {
	name, path string
	state      domain.VaultState
	isDefault  bool
	revision   int64
	workspaces []wsRow
	images     []imgRow
}

type snapshot struct {
	vaults []vaultRow
	err    error
}

type snapshotMsg snapshot
type tickMsg time.Time

const refreshEvery = 4 * time.Second

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (d Deps) snapshotCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return snapshotMsg(gather(ctx, d))
	}
}

func gather(ctx context.Context, d Deps) snapshot {
	mounts, err := platform.Mounts()
	if err != nil {
		return snapshot{err: err}
	}
	g := &git.Git{Runner: d.Runner}
	reports, _ := workspace.Reports(ctx, d.Paths, g)
	repByName := make(map[string]workspace.Report, len(reports))
	for _, r := range reports {
		repByName[r.Workspace.Name] = r
	}

	var vaults []vaultRow
	for _, v := range d.Config.Vaults {
		row := vaultRow{
			name:      v.Name,
			path:      v.Path,
			state:     vault.State(v.Path, mounts),
			isDefault: v.Name == d.Config.DefaultVault,
		}
		if row.state == domain.VaultStateAvailable {
			if idx, ierr := vault.ReadIndex(v.Path); ierr == nil {
				row.revision = idx.Revision
				for _, w := range idx.Workspaces {
					wr := wsRow{name: w.Name, vaultName: v.Name, images: w.Images, isolation: w.Isolation}
					if r, ok := repByName[w.Name]; ok && r.Workspace.SourceVault == v.Name {
						wr.loaded = true
						wr.report = r
					}
					row.workspaces = append(row.workspaces, wr)
				}
				row.images = gatherImages(v.Path)
			}
		}
		vaults = append(vaults, row)
	}
	return snapshot{vaults: vaults}
}

func gatherImages(vaultPath string) []imgRow {
	entries, err := os.ReadDir(filepath.Join(vaultPath, "images"))
	if err != nil {
		return nil
	}
	var out []imgRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		row := imgRow{name: e.Name()}
		if info, err := os.Stat(vault.ImageTarPath(vaultPath, e.Name())); err == nil {
			row.built = true
			row.size = info.Size()
		}
		out = append(out, row)
	}
	return out
}
