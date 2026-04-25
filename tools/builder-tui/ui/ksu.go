package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	w := s.app.Width
	if w < 60 {
		w = 60
	}
	if w > 100 {
		w = 100
	}

	var b strings.Builder
	b.WriteString(components.Banner("ReSukiSU  DRIVER  MANAGER", "github.com/ReSukiSU/ReSukiSU", w, ColorTitle, ColorAccent) + "\n")
	b.WriteString(components.Rule("", w, MutedText) + "\n\n")

	keyStyle := lipgloss.NewStyle().Foreground(ColorDim)
	b.WriteString(components.KV("Active", s.app.Cfg.KSUBranch, 10, keyStyle, AccentText) + "\n")
	b.WriteString(components.KV("Target", s.app.Paths.Kernel+"/drivers/kernelsu", 10, keyStyle, MutedText) + "\n\n")

	b.WriteString(components.Rule("Upstream", w, MutedText) + "\n")
	b.WriteString(probeRow("main", s.main, s.probed) + "\n")
	b.WriteString(probeRow("dev", s.dev, s.probed) + "\n\n")

	b.WriteString(components.Rule("Action", w, MutedText) + "\n")
	b.WriteString(components.Hotkey("I", "Install / update from "+s.app.Cfg.KSUBranch, "", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorOK), DimText) + "\n")
	b.WriteString(components.Hotkey("S", "Switch branch", "main <-> dev", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorAccent), DimText) + "\n")
	b.WriteString(components.Hotkey("P", "Re-probe upstream", "", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorWarn), DimText) + "\n")
	b.WriteString(components.Hotkey("ESC", "Return to main", "", HotKeyStyle, DimText, DimText) + "\n")

	if s.busy {
		b.WriteString("\n  " + AccentText.Render(s.stage) + "...\n")
	}
	if s.app.Toast != "" {
		st := OKText
		if s.app.ToastErr {
			st = ErrText
		}
		b.WriteString("\n" + st.Render("  "+s.app.Toast))
	}
	return b.String()
}

func probeRow(branch string, p resukisu.Probe, probed bool) string {
	if !probed {
		return "  " + DimText.Render(branch+" : (not yet probed)")
	}
	switch p.State {
	case resukisu.StatePresent:
		short := p.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		return "  " + AccentText.Render(branch) + "  " + OKText.Render("present  ") + DimText.Render(short)
	case resukisu.StateAbsent:
		return "  " + AccentText.Render(branch) + "  " + WarnText.Render("absent (branch removed or merged)")
	case resukisu.StateNetworkFail:
		return "  " + AccentText.Render(branch) + "  " + ErrText.Render("network unreachable")
	}
	return ""
}
