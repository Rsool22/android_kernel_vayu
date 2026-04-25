package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/kbuild"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// BuildScreen drives a kernel build, streaming compile output into a viewport.
// It is value-copied through the App's screen routing, so any mutable state
// must live in slices/maps -- no embedded sync primitives.
type BuildScreen struct {
	app    *App
	vp     viewport.Model
	lines  []string
	busy   bool
	result *kbuild.Result
}

func NewBuildScreen(a *App) BuildScreen {
	vp := viewport.New(80, 18)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(ColorBuild).
		Padding(0, 1)
	return BuildScreen{app: a, vp: vp}
}

func (s BuildScreen) Init() tea.Cmd { return nil }

type buildLineMsg struct{ line string }
type buildDoneMsg struct{ res kbuild.Result }

func (s BuildScreen) Update(msg tea.Msg) (BuildScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		// Match the viewport's outer width to the surrounding panels' outer
		// width. viewport.Width is the *content* width; rounded border (2) +
		// padding (2) are added on top, so set it to panelOuter - 4.
		w := panelWidth(m.Width)
		s.vp.Width = w - 4
		if s.vp.Width < 40 {
			s.vp.Width = 40
		}
		s.vp.Height = m.Height - 18
		if s.vp.Height < 8 {
			s.vp.Height = 8
		}
	case tea.KeyMsg:
		switch strings.ToLower(m.String()) {
		case "b":
			if !s.busy {
				s.busy = true
				s.result = nil
				s.lines = nil
				return s, s.runCmd()
			}
		}
		var cmd tea.Cmd
		s.vp, cmd = s.vp.Update(msg)
		return s, cmd
	case buildLineMsg:
		s.lines = append(s.lines, m.line)
		if len(s.lines) > 5000 {
			s.lines = s.lines[len(s.lines)-5000:]
		}
		s.vp.SetContent(strings.Join(s.lines, "\n"))
		s.vp.GotoBottom()
	case buildDoneMsg:
		s.busy = false
		s.result = &m.res
		if m.res.ExitCode == 0 {
			s.app.Toast = "Build OK in " + kbuild.FormatElapsed(m.res.Elapsed) + " — " + m.res.Image
			s.app.ToastErr = false
		} else {
			s.app.Toast = "Build failed (exit " + itoa(m.res.ExitCode) + ")"
			s.app.ToastErr = true
		}
	}
	return s, nil
}

func (s BuildScreen) View() string {
	w := panelWidth(s.app.Width)
	inner := innerContentWidth(w)

	banner := components.Banner(
		"BUILD  KERNEL",
		s.app.Paths.Kernel,
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Status panel ────────────────────────────────────────────────────────
	var st strings.Builder
	clangVal := okOr(s.app.Paths.Clang, "(not installed)")
	clangSty := ValueStyle
	if s.app.Paths.Clang == "" {
		clangSty = ErrText
	}
	st.WriteString(components.KV("Source", sourceLabel(s.app.Cfg), 11, LabelStyle, AccentText) + "\n")
	st.WriteString(components.KV("Clang", clangVal, 11, LabelStyle, clangSty) + "\n")
	st.WriteString(components.KV("KSU", s.app.Cfg.KSUBranch, 11, LabelStyle, ValueStyle) + "\n")
	statusSty := WarnText
	statusVal := "idle — press [B] to start"
	if s.busy {
		statusSty = AccentText
		statusVal = "compiling …"
	}
	if s.result != nil {
		if s.result.ExitCode == 0 {
			statusSty = OKText
			statusVal = "build OK · " + kbuild.FormatElapsed(s.result.Elapsed)
		} else {
			statusSty = ErrText
			statusVal = "build failed · exit " + itoa(s.result.ExitCode)
		}
	}
	st.WriteString(components.KV("Status", statusVal, 11, LabelStyle, statusSty))
	statusPanel := components.Panel("Build status", st.String(), w, PanelBorder, TitleStyle)

	// ── Compiler output viewport ─────────────────────────────────────────────
	// Build the rule so its visible end aligns with the panel borders above.
	headLabel := lipgloss.NewStyle().Foreground(ColorBuild).Bold(true).Render(" Compiler output ")
	headFillW := w - lipgloss.Width(headLabel)
	if headFillW < 0 {
		headFillW = 0
	}
	vpHeader := headLabel + MutedText.Render(strings.Repeat("─", headFillW)) + "\n"
	_ = inner

	actions := components.HotkeyStrip([]string{
		components.Hotkey("B", "Build", "compile + package", HotKeyStyle, OKText, DimText),
		components.Hotkey("↑/↓", "Scroll", "compiler output", HotKeyStyle, AccentText, DimText),
		components.Hotkey("ESC", "Back", "", HotKeyStyle, DimText, DimText),
	}, MutedText)

	out := banner + "\n" + statusPanel + "\n" + vpHeader + s.vp.View() + "\n\n  " + actions + "\n"

	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

func (s BuildScreen) runCmd() tea.Cmd {
	return func() tea.Msg {
		o := kbuild.Defaults(s.app.Paths.Kernel)
		o.OutputDir = s.app.Paths.Output
		o.ClangDir = s.app.Paths.Clang
		o.GccArm64 = s.app.Paths.GccArm64
		o.GccArm = s.app.Paths.GccArm
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		// Stream compile lines into the TUI via Program.Send so they show up
		// live in the viewport while make(1) is still running.
		stream := func(line string, _ bool) {
			if Program != nil {
				Program.Send(buildLineMsg{line: line})
			}
		}
		res := kbuild.Run(ctx, o, stream)
		return buildDoneMsg{res: res}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
