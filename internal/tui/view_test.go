package tui

import (
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/ui"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

func TestView_RendersSnapshot(t *testing.T) {
	m := model{
		deps:   Deps{Theme: ui.NeonTheme()},
		loaded: true,
		snap: snapshot{vaults: []vaultRow{{
			name:      "ext",
			state:     domain.VaultStateAvailable,
			isDefault: true,
			revision:  3,
			workspaces: []wsRow{
				{name: "webapp", vaultName: "ext", loaded: true,
					report: workspace.Report{Status: git.Status{Ahead: 1}, HotBytes: 2048}},
				{name: "api", vaultName: "ext"}, // cold
			},
			images: []imgRow{{name: "dev", built: true, size: 1024}},
		}}},
	}

	// Strip ANSI so assertions test content, not styling.
	out := stripANSI(m.View())
	for _, want := range []string{"NEONROOT", "ext", "webapp", "hot", "api", "cold", "image dev", "move", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q; got:\n%s", want, out)
		}
	}
}

func TestView_Loading(t *testing.T) {
	m := model{deps: Deps{Theme: ui.NeonTheme()}}
	if !strings.Contains(m.View(), "loading") {
		t.Error("pre-snapshot view should show a loading state")
	}
}

func TestSelectedCursor(t *testing.T) {
	m := model{snap: snapshot{vaults: []vaultRow{{
		state:      domain.VaultStateAvailable,
		workspaces: []wsRow{{name: "a"}, {name: "b"}},
	}}}}
	m.cursor = 1
	sel, ok := m.selected()
	if !ok || sel.name != "b" {
		t.Errorf("cursor 1 should select 'b', got %+v ok=%v", sel, ok)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
