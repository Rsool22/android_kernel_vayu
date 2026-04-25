package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// SetupScreen is the strict 1:1 port of the bash `do_setup` /
// `_draw_setup_menu`: a 2-row wrapper menu that dispatches to either the
// path-editor (PathsScreen) or the dependency probe (DepsScreen).
//
// Hotkeys: P / D / R (return). ESC also returns to main.
type SetupScreen struct {
	app    *App
	cursor int // 0..2 — P, D, R rows
}

func NewSetupScreen(a *App) SetupScreen { return SetupScreen{app: a} }

func (s SetupScreen) Init() tea.Cmd { return nil }

func (s SetupScreen) Update(msg tea.Msg) (SetupScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(m.String())
		switch key {
		case "up":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil
		case "down":
			if s.cursor < 2 {
				s.cursor++
			}
			return s, nil
		case "enter":
			key = []string{"p", "d", "r"}[s.cursor]
		}
		switch key {
		case "p":
			s.app.Screen = ScreenPaths
			return s, s.app.paths.Init()
		case "d":
			s.app.Screen = ScreenDeps
			return s, s.app.deps.Init()
		case "r", "esc":
			s.app.Screen = ScreenMain
			return s, nil
		}
	}
	return s, nil
}

func (s SetupScreen) View() string {
	w := panelWidth(s.app.Width)
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"SETUP",
		"paths · dependencies",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	type menuItem struct {
		key, label, rhs string
	}
	items := []menuItem{
		{"P", "Configure Paths", "kernel/clang/output dirs"},
		{"D", "Check Dependencies", "verify + install packages"},
		{"R", "Return to Main Menu", ""},
	}

	var menu strings.Builder
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	maxKey := 1
	for i, it := range items {
		labelStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
		rhsStyle := lipgloss.NewStyle().Foreground(ColorAccent)
		if it.rhs == "" {
			rhsStyle = lipgloss.NewStyle().Foreground(ColorMuted)
		}
		mark := "  "
		if i == s.cursor {
			mark = AccentText.Render(" ›")
		}
		row := components.MenuRow(it.key, it.label, it.rhs, innerW-2, maxKey,
			HotKeyStyle, labelStyle, rhsStyle, leader,
		)
		menu.WriteString(mark + row)
		if i < len(items)-1 {
			menu.WriteString("\n")
		}
	}
	menuPanel := components.Panel("Setup", menu.String(), w, PanelBorder, TitleStyle)

	// Path warnings — same surface as bash _draw_setup_menu.
	var extras strings.Builder
	if s.app.Paths.Clang == "" || s.app.Paths.AnyKernel == "" {
		var msgs []string
		if s.app.Paths.Clang == "" {
			msgs = append(msgs, "[!] Clang not found: "+defaultOr(s.app.Cfg.ClangDir, "(unset)"))
		}
		if s.app.Paths.AnyKernel == "" {
			msgs = append(msgs, "[!] AnyKernel3 not found: "+defaultOr(s.app.Cfg.AnyKernelDir, "(unset)"))
		}
		msgs = append(msgs, "Use [P] to fix paths or [T] in main menu to download Clang.")
		body := ErrText.Render(strings.Join(msgs, "\n"))
		extras.WriteString(components.Panel("Path Issues", body, w, PanelErr, ErrText))
		extras.WriteString("\n")
	}

	help := HelpStyle.Render("  Select [P/D/R] · esc to return")
	var toast string
	if s.app.Toast != "" {
		toast = "  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return banner + "\n" + menuPanel + "\n" + extras.String() + toast + help
}

// installHints returns the distro-specific install command(s) for missing
// host build packages. Used by DepsScreen and PathsScreen.
func installHints(p discover.Paths) []string {
	pkgs := []string{"git", "ccache", "make", "bc", "bison", "flex", "zip", "unzip", "rsync", "python3", "build-essential"}
	if p.GccArm64 == "" {
		pkgs = append(pkgs, "gcc-aarch64-linux-gnu")
	}
	if p.GccArm == "" {
		pkgs = append(pkgs, "gcc-arm-linux-gnueabi")
	}
	switch p.Distro {
	case discover.PMApt:
		return []string{"sudo apt-get install -y " + strings.Join(pkgs, " ")}
	case discover.PMDnf:
		return []string{"sudo dnf install -y " + strings.Join(pkgs, " ")}
	case discover.PMPacman:
		return []string{"sudo pacman -S --needed " + strings.Join(pkgs, " ")}
	case discover.PMZypper:
		return []string{"sudo zypper install " + strings.Join(pkgs, " ")}
	case discover.PMApk:
		return []string{"sudo apk add " + strings.Join(pkgs, " ")}
	}
	return []string{"(distro unknown -- install: " + strings.Join(pkgs, ", ") + ")"}
}

// wrapValue is a thin wrapper over wrapPlainPaths that keeps long path
// values on a single line until they exceed `width`, then breaks on path
// separators preferentially.
func wrapValue(s string, width int) []string {
	if width <= 0 || len(s) <= width {
		return []string{s}
	}
	return wrapPlainPaths(s, width)
}

// wrapPlainPaths breaks s at /, space, or hyphen boundaries near the width
// limit; falls back to a hard cut.
func wrapPlainPaths(s string, width int) []string {
	var lines []string
	for len(s) > width {
		cut := width
		for i := width; i > width/2 && i < len(s); i-- {
			c := s[i]
			if c == '/' || c == ' ' || c == '-' || c == '_' {
				cut = i + 1
				break
			}
		}
		if cut > len(s) {
			cut = len(s)
		}
		lines = append(lines, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

// padRight right-pads s to the given visual width with spaces. Safe
// when s is already wider than width (returns s unchanged) and never
// panics on a negative repeat count.
func padRight(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s
	}
	return s + components.SafeRepeat(" ", width-lipgloss.Width(s))
}

// defaultOr returns d when d is non-empty, otherwise alt. Used to surface
// the *configured default* when discovery fails so the user sees what the
// builder is going to try, instead of "(not found)" alone.
func defaultOr(d, alt string) string {
	if d == "" {
		return alt
	}
	return d
}
