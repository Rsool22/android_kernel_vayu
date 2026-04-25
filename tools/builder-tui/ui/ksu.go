package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/resukisu"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// KSUScreen surfaces ReSukiSU branch availability and install/update actions.
// Branch probes run on entry and on demand (key 'p').
type KSUScreen struct {
	app    *App
	main   resukisu.Probe
	dev    resukisu.Probe
	probed bool
	busy   bool
	stage  string
}

func NewKSUScreen(a *App) KSUScreen { return KSUScreen{app: a} }

func (s KSUScreen) Init() tea.Cmd { return s.probeCmd() }

type ksuProbedMsg struct{ main, dev resukisu.Probe }
type ksuActionDoneMsg struct {
	stage string
	err   error
}

func (s KSUScreen) probeCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		m, d := resukisu.ProbeAll(ctx)
		return ksuProbedMsg{main: m, dev: d}
	}
}

func (s KSUScreen) Update(msg tea.Msg) (KSUScreen, tea.Cmd) {
	switch m := msg.(type) {
	case ksuProbedMsg:
		s.main = m.main
		s.dev = m.dev
		s.probed = true
		s.busy = false
		s.app.Toast = "Upstream probed."
		s.app.ToastErr = false
		return s, nil
	case ksuActionDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = m.stage + " failed: " + m.err.Error()
			s.app.ToastErr = true
		} else {
			s.app.Toast = m.stage + " complete."
			s.app.ToastErr = false
		}
		return s, nil
	case tea.KeyMsg:
		if s.busy {
			return s, nil
		}
		switch strings.ToLower(m.String()) {
		case "p":
			s.busy = true
			s.stage = "probing upstream"
			return s, s.probeCmd()
		case "s":
			if s.app.Cfg.KSUBranch == "main" {
				s.app.Cfg.KSUBranch = "dev"
			} else {
				s.app.Cfg.KSUBranch = "main"
			}
			s.app.PersistConfig()
		case "i", "u":
			selected := s.app.Cfg.KSUBranch
			state := s.branchState(selected)
			if state != resukisu.StatePresent {
				s.app.Toast = "Cannot install: " + selected + " branch is " + string(state)
				s.app.ToastErr = true
				return s, nil
			}
			s.busy = true
			s.stage = "installing " + selected
			return s, s.installCmd(selected)
		}
	}
	return s, nil
}

func (s KSUScreen) branchState(b string) resukisu.State {
	switch b {
	case "main":
		return s.main.State
	case "dev":
		return s.dev.State
	}
	return resukisu.StateAbsent
}

func (s KSUScreen) installCmd(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := resukisu.Install(ctx, s.app.Paths.Kernel, branch, nil)
		return ksuActionDoneMsg{stage: "Install (" + branch + ")", err: err}
	}
}

func (s KSUScreen) View() string {
	w := panelWidth(s.app.Width)

	banner := components.Banner(
		"ReSukiSU  DRIVER  MANAGER",
		"github.com/ReSukiSU/ReSukiSU  ·  branch probing",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Active selection ────────────────────────────────────────────────────
	activeBadge := components.Badge(strings.ToUpper(s.app.Cfg.KSUBranch), BadgeAccent)
	var sel strings.Builder
	sel.WriteString(components.KV("Active", activeBadge, 9, LabelStyle, ValueStyle) + "\n")
	sel.WriteString(components.KV("Target", s.app.Paths.Kernel+"/drivers/kernelsu", 9, LabelStyle, AccentText))
	selPanel := components.Panel("Selection", sel.String(), w, PanelBorder, TitleStyle)

	// ── Upstream branch states ──────────────────────────────────────────────
	// Right-pad branch label to the widest value so badges align in a column.
	branchW := 4 // max(len("main"), len("dev"))
	var ups strings.Builder
	ups.WriteString(probeRow("main", branchW, s.main, s.probed) + "\n")
	ups.WriteString(probeRow("dev", branchW, s.dev, s.probed))
	upsPanel := components.Panel("Upstream branches", ups.String(), w, PanelBorder, TitleStyle)

	// ── Action strip ────────────────────────────────────────────────────────
	actions := components.HotkeyStrip([]components.Hotkey{
		{Key: "I", Desc: "Install / update", Sub: "from active branch"},
		{Key: "S", Desc: "Switch", Sub: "main ↔ dev"},
		{Key: "P", Desc: "Re-probe", Sub: "git ls-remote"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := "  " + components.Separator(innerContentWidth(w), MutedText) + "\n"
	out := banner + "\n" + selPanel + "\n" + upsPanel + "\n" + divider + "  " + actions + "\n"

	if s.busy {
		out += "\n  " + AccentText.Render(s.stage+" …") + "\n"
	}
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// padTo right-pads a string with spaces to a fixed visible width.
func padTo(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// probeRow renders a single branch probe row with all columns aligned
// regardless of branch label or status length:
//
//	  [main]  PRESENT  HEAD@7adffacb
//	  [dev ]  ABSENT   branch removed upstream
//	  [foo ]  NETWORK  could not reach upstream
//
// branchW is the widest branch label across all rows (used for [..] padding);
// badge text is padded to 7 chars (max of "present"/"absent"/"network").
func probeRow(branch string, branchW int, p resukisu.Probe, probed bool) string {
	const badgeW = 7
	tag := components.BracketTag(strings.TrimSpace(branch), branchW, HotKeyStyle)
	if !probed {
		return "  " + tag + "  " + DimText.Render("(probing …)")
	}
	switch p.State {
	case resukisu.StatePresent:
		short := p.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		return "  " + tag + "  " +
			components.Badge(padTo("PRESENT", badgeW), BadgeOK) + "  " +
			DimText.Render("HEAD@") + AccentText.Bold(true).Render(short)
	case resukisu.StateAbsent:
		return "  " + tag + "  " +
			components.Badge(padTo("ABSENT", badgeW), BadgeWarn) + "  " +
			DimText.Render("branch removed or merged upstream")
	case resukisu.StateNetworkFail:
		return "  " + tag + "  " +
			components.Badge(padTo("NETWORK", badgeW), BadgeErr) + "  " +
			DimText.Render("could not reach upstream — check connection")
	}
	return ""
}
