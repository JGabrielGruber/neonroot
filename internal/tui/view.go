package tui

import (
	"fmt"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/ui"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

func (m model) View() string {
	th := m.deps.Theme
	if !m.loaded {
		return "\n  loading…\n"
	}

	var b strings.Builder
	b.WriteString("\n " + th.Accent.Render("◤ NEONROOT") + th.Muted.Render("  cockpit") + "\n\n")
	if m.snap.err != nil {
		b.WriteString(" " + th.Error.Render(m.snap.err.Error()) + "\n\n")
	}

	idx := 0 // running index into the flat selectable list, matched to m.cursor
	for _, v := range m.snap.vaults {
		b.WriteString(vaultHeader(th, v))
		if v.state != domain.VaultStateAvailable {
			continue
		}
		if len(v.workspaces) == 0 {
			b.WriteString(th.Muted.Render("      (no workspaces)") + "\n")
		}
		for _, w := range v.workspaces {
			b.WriteString(wsLine(th, w, idx == m.cursor))
			idx++
		}
		for _, im := range v.images {
			b.WriteString(imageLine(th, im))
		}
		b.WriteString("\n")
	}

	if m.inputting {
		b.WriteString(" " + th.Accent.Render("new workspace: ") + th.Step.Render(m.input+"▌") + "\n")
	}
	if m.busy != "" {
		b.WriteString(" " + th.Step.Render("⟳ "+m.busy+"…") + "\n")
	} else if m.status != "" {
		style := th.Success
		if m.statusErr {
			style = th.Error
		}
		b.WriteString(" " + style.Render(m.status) + "\n")
	}
	b.WriteString(footer(th, m.inputting))
	return b.String()
}

func vaultHeader(th ui.Theme, v vaultRow) string {
	star := " "
	if v.isDefault {
		star = th.Accent.Render("*")
	}
	state := th.Muted.Render("unavailable")
	rev := ""
	if v.state == domain.VaultStateAvailable {
		state = th.Success.Render("available")
		rev = th.Muted.Render(fmt.Sprintf("  rev %d", v.revision))
	}
	return fmt.Sprintf(" %s %s  %s%s\n", star, th.Step.Render(v.name), state, rev)
}

func wsLine(th ui.Theme, w wsRow, selected bool) string {
	mark := th.Muted.Render("·")
	tail := th.Muted.Render("cold")
	if w.loaded {
		mark = syncMark(th, w.report)
		tail = th.Muted.Render(fmt.Sprintf("hot  %s", humanSize(w.report.HotBytes)))
	}
	img := ""
	if len(w.images) > 0 {
		img = th.Muted.Render("  [" + strings.Join(w.images, ",") + "]")
	}
	name := fmt.Sprintf("%-16s", w.name)
	body := fmt.Sprintf("%s %s %s%s", mark, name, tail, img)
	if selected {
		return "   " + th.Accent.Render("▸ "+body) + "\n"
	}
	return "     " + body + "\n"
}

func imageLine(th ui.Theme, im imgRow) string {
	if im.built {
		return "     " + th.Muted.Render(fmt.Sprintf("image %s  %s", im.name, humanSize(im.size))) + "\n"
	}
	return "     " + th.Muted.Render(fmt.Sprintf("image %s  (not built)", im.name)) + "\n"
}

func syncMark(th ui.Theme, r workspace.Report) string {
	switch {
	case r.Err != nil:
		return th.Warn.Render("?")
	case r.Status.Dirty && r.Status.Ahead > 0:
		return th.Warn.Render("±")
	case r.Status.Dirty:
		return th.Warn.Render("*")
	case r.Status.Ahead > 0:
		return th.Warn.Render("↑")
	default:
		return th.Success.Render("✓")
	}
}

func footer(th ui.Theme, inputting bool) string {
	if inputting {
		return "\n " + th.Muted.Render("type a name · enter create · esc cancel") + "\n"
	}
	keys := "↑/↓ move · l load · a attach · c commit · x stop · n new · y sync · r refresh · q quit"
	return "\n " + th.Muted.Render(keys) + "\n"
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
