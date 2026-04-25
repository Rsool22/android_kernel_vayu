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
	var sel strings.Builder
	sel.WriteString(components.KV("Active", s.app.Cfg.KSUBranch, 9, LabelStyle, ValueStyle) + "\n")
	sel.WriteString(components.KV("Target", s.app.Paths.Kernel+"/drivers/kernelsu", 9, LabelStyle, AccentText))
	selPanel := components.Panel("Selection", sel.String(), w, PanelBorder, TitleStyle)

	// ── Upstream branch states ──────────────────────────────────────────────
	var ups strings.Builder
	ups.WriteString(probeRow("main", s.main, s.probed) + "\n")
	ups.WriteString(probeRow("dev ", s.dev, s.probed))
	upsPanel := components.Panel("Upstream branches", ups.String(), w, PanelBorder, TitleStyle)

	// ── Action strip ────────────────────────────────────────────────────────
	actions := components.HotkeyStrip([]string{
		components.Hotkey("I", "Install / update", "from active branch", HotKeyStyle, OKText, DimText),
		components.Hotkey("S", "Switch", "main ↔ dev", HotKeyStyle, AccentText, DimText),
		components.Hotkey("P", "Re-probe", "git ls-remote", HotKeyStyle, WarnText, DimText),
		components.Hotkey("ESC", "Back", "", HotKeyStyle, DimText, DimText),
	}, MutedText)

	out := banner + "\n" + selPanel + "\n" + upsPanel + "\n  " + actions + "\n"

	if s.busy {
		out += "\n  " + AccentText.Render(s.stage+" …") + "\n"
	}
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

func probeRow(branch string, p resukisu.Probe, probed bool) string {
	tag := HotKeyStyle.Render("[" + strings.TrimSpace(branch) + "]")
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
			components.Badge("present", BadgeOK) + "  " +
			DimText.Render("HEAD@") + AccentText.Render(short)
	case resukisu.StateAbsent:
		return "  " + tag + "  " +
			components.Badge("absent", BadgeWarn) + "  " +
			DimText.Render("branch removed or merged upstream")
	case resukisu.StateNetworkFail:
		return "  " + tag + "  " +
			components.Badge("network", BadgeErr) + "  " +
			DimText.Render("could not reach upstream — check connection")
	}
	return ""
}
