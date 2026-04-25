package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/clang"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// ToolchainScreen lets the user pick Google/ZyC/Auto, query upstream, and
// fetch+install a clang toolchain.
type ToolchainScreen struct {
	app   *App
	prog  progress.Model
	busy  bool
	stage string
	last  string
}

func NewToolchainScreen(a *App) ToolchainScreen {
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 60
	return ToolchainScreen{app: a, prog: p}
}

func (s ToolchainScreen) Init() tea.Cmd { return nil }

// Messages used by the toolchain screen.
type tcQueryDoneMsg struct {
	rel clang.Release
	err error
}
type tcDownloadProgressMsg struct{ done, total int64 }
type tcInstallDoneMsg struct{ err error }

func (s ToolchainScreen) Update(msg tea.Msg) (ToolchainScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(m.String())
		if s.busy {
			return s, nil
		}
		switch key {
		case "a":
			s.app.Cfg.ClangSource = config.ClangAuto
			s.app.PersistConfig()
		case "g":
			s.app.Cfg.ClangSource = config.ClangGoogle
			s.app.PersistConfig()
		case "z":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.PersistConfig()
		case "l":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "latest"
			s.app.PersistConfig()
		case "1":
			// 1->15 (placeholder; "23" handled below as a sequence is awkward in bubbletea so we use 1/2/L)
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "15"
			s.app.PersistConfig()
		case "2":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "23"
			s.app.PersistConfig()
		case "c":
			s.busy = true
			s.stage = "querying upstream"
			return s, s.queryCmd()
		case "f":
			s.busy = true
			s.stage = "querying upstream"
			return s, s.queryThenFetchCmd()
		}
	case tcQueryDoneMsg:
		s.busy = false
		if m.err != nil {
			s.app.Toast = "Query failed: " + m.err.Error()
			s.app.ToastErr = true
			return s, nil
		}
		s.last = fmt.Sprintf("%s : %s (%s)", m.rel.Source, m.rel.Tag, sizeStr(m.rel.SizeBytes))
		s.app.Toast = "Latest " + s.last
		s.app.ToastErr = false
	case tcDownloadProgressMsg:
		if m.total > 0 {
			cmd := s.prog.SetPercent(float64(m.done) / float64(m.total))
			return s, cmd
		}
	case tcInstallDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = "Install failed: " + m.err.Error()
			s.app.ToastErr = true
		} else {
			s.app.Toast = "Clang installed at " + s.app.Paths.Clang
			s.app.ToastErr = false
		}
		return s, nil
	case progress.FrameMsg:
		var cmd tea.Cmd
		var pm tea.Model
		pm, cmd = s.prog.Update(m)
		s.prog = pm.(progress.Model)
		return s, cmd
	}
	return s, nil
}

func (s ToolchainScreen) View() string {
	w := s.app.Width
	if w < 60 {
		w = 60
	}
	if w > 100 {
		w = 100
	}

	var b strings.Builder
	b.WriteString(components.Banner("TOOLCHAIN  MANAGER", string(s.app.Cfg.ClangSource), w, ColorTitle, ColorAccent) + "\n")
	b.WriteString(components.Rule("", w, MutedText) + "\n\n")

	statusKey := lipgloss.NewStyle().Foreground(ColorDim)
	statusVal := lipgloss.NewStyle().Foreground(ColorAccent)
	b.WriteString(components.KV("Source", sourceLabel(s.app.Cfg), 12, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Origin", sourceOrigin(s.app.Cfg.ClangSource), 12, statusKey, statusVal) + "\n")
	local := clang.LocalVersion(s.app.Paths.Clang)
	if local == "" {
		b.WriteString(components.KV("Local", "(not installed)", 12, statusKey, ErrText) + "\n")
	} else {
		b.WriteString(components.KV("Local", local, 12, statusKey, OKText) + "\n")
	}
	b.WriteString(components.KV("Install", okOr(s.app.Paths.Clang, "(unset)"), 12, statusKey, MutedText) + "\n")
	if s.last != "" {
		b.WriteString(components.KV("Latest", s.last, 12, statusKey, AccentText) + "\n")
	}
	b.WriteString("\n")

	b.WriteString(components.Rule("Source", w, MutedText) + "\n")
	b.WriteString(srcRow("A", "Auto: Google primary, ZyC fallback", s.app.Cfg.ClangSource == config.ClangAuto) + "\n")
	b.WriteString(srcRow("G", "Google AOSP clang", s.app.Cfg.ClangSource == config.ClangGoogle) + "\n")
	b.WriteString(srcRow("Z", "ZyC Clang (community)", s.app.Cfg.ClangSource == config.ClangZyC) + "\n")
	if s.app.Cfg.ClangSource == config.ClangZyC {
		b.WriteString("\n")
		b.WriteString(components.Rule("ZyC target", w, MutedText) + "\n")
		b.WriteString(srcRow("2", "ZyC Clang 23.x", s.app.Cfg.ZyCTarget == "23") + "\n")
		b.WriteString(srcRow("1", "ZyC Clang 15.x", s.app.Cfg.ZyCTarget == "15") + "\n")
		b.WriteString(srcRow("L", "Latest (any version)", s.app.Cfg.ZyCTarget == "latest") + "\n")
	}
	b.WriteString("\n")
	b.WriteString(components.Rule("Action", w, MutedText) + "\n")
	b.WriteString(components.Hotkey("F", "Fetch selected", "check + download", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorOK), DimText) + "\n")
	b.WriteString(components.Hotkey("C", "Check latest", "query upstream only", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorAccent), DimText) + "\n")
	b.WriteString(components.Hotkey("ESC", "Return to main", "", HotKeyStyle, DimText, DimText) + "\n")

	if s.busy {
		b.WriteString("\n  " + AccentText.Render(s.stage) + "\n")
		b.WriteString("  " + s.prog.View() + "\n")
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

func srcRow(key, label string, selected bool) string {
	mark := "  "
	if selected {
		mark = SelText.Render(" ●")
	}
	return mark + " " + HotKeyStyle.Render("["+key+"]") + "  " + label
}

func sourceLabel(c config.Config) string {
	switch c.ClangSource {
	case config.ClangGoogle:
		return "Google AOSP clang  (" + c.GoogleTarget + ")"
	case config.ClangZyC:
		return "ZyC Clang  (" + c.ZyCTarget + ".x)"
	default:
		return "Auto: Google -> ZyC fallback"
	}
}

func sourceOrigin(s config.ClangSource) string {
	switch s {
	case config.ClangGoogle:
		return "android.googlesource.com (main-kernel)"
	case config.ClangZyC:
		return "github.com/ZyCromerZ/Clang"
	default:
		return "auto: AOSP gitiles + ZyC GitHub"
	}
}

func sizeStr(b int64) string {
	if b <= 0 {
		return "?"
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("%d MB", (b+mb-1)/mb)
}

// queryCmd resolves the latest release for the active source.
func (s ToolchainScreen) queryCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var src clang.Source
		var target string
		switch s.app.Cfg.ClangSource {
		case config.ClangGoogle:
			src = &clang.Google{}
			target = s.app.Cfg.GoogleTarget
		case config.ClangZyC:
			src = &clang.ZyC{}
			target = s.app.Cfg.ZyCTarget
		default:
			src = &clang.Auto{}
			target = s.app.Cfg.GoogleTarget
		}
		rel, err := src.Latest(ctx, target)
		return tcQueryDoneMsg{rel: rel, err: err}
	}
}

// queryThenFetchCmd does query + install in one shot.
func (s ToolchainScreen) queryThenFetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var src clang.Source
		var target string
		switch s.app.Cfg.ClangSource {
		case config.ClangGoogle:
			src = &clang.Google{}
			target = s.app.Cfg.GoogleTarget
		case config.ClangZyC:
			src = &clang.ZyC{}
			target = s.app.Cfg.ZyCTarget
		default:
			src = &clang.Auto{}
			target = s.app.Cfg.GoogleTarget
		}
		rel, err := src.Latest(ctx, target)
		if err != nil {
			return tcInstallDoneMsg{err: err}
		}
		dest := s.app.Paths.Clang
		if dest == "" && s.app.Paths.Kernel != "" {
			dest = s.app.Paths.Kernel + "/clang"
		}
		if dest == "" {
			return tcInstallDoneMsg{err: fmt.Errorf("install dir unknown (configure Setup > Paths first)")}
		}
		err = clang.Install(ctx, rel, dest, nil)
		return tcInstallDoneMsg{err: err}
	}
}
