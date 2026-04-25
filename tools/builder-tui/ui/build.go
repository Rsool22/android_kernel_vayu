package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"

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
	return BuildScreen{app: a, vp: vp}
}

func (s BuildScreen) Init() tea.Cmd { return nil }

type buildLineMsg struct{ line string }
type buildDoneMsg struct{ res kbuild.Result }

func (s BuildScreen) Update(msg tea.Msg) (BuildScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.vp.Width = m.Width - 4
		if s.vp.Width < 40 {
			s.vp.Width = 40
		}
		s.vp.Height = m.Height - 12
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
	w := s.app.Width
	if w < 60 {
		w = 60
	}
	if w > 120 {
		w = 120
	}

	var b strings.Builder
	b.WriteString(components.Banner("BUILD  KERNEL", s.app.Paths.Kernel, w, ColorTitle, ColorAccent) + "\n")
	b.WriteString(components.Rule("", w, MutedText) + "\n\n")
	b.WriteString("  " + AccentText.Render("Press [B] to start build, [ESC] to return.") + "\n\n")

	b.WriteString(components.Rule("Compiler output", w, MutedText) + "\n")
	b.WriteString(s.vp.View() + "\n")

	if s.app.Toast != "" {
		st := OKText
		if s.app.ToastErr {
			st = ErrText
		}
		b.WriteString("\n" + st.Render("  "+s.app.Toast))
	}
	return b.String()
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
