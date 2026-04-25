package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// MainMenu is the entry screen. Hotkeys: B/P/T/K/F/S/Q.
type MainMenu struct {
	app *App
}

func NewMainMenu(a *App) MainMenu { return MainMenu{app: a} }

func (m MainMenu) Init() tea.Cmd { return nil }

// pathsLocked reports whether building should be blocked because either
// Clang or AnyKernel3 is missing.
func (m MainMenu) pathsLocked() bool {
	return m.app.Paths.Clang == "" || m.app.Paths.AnyKernel == ""
}

func (m MainMenu) Update(msg tea.Msg) (MainMenu, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "b":
			if m.pathsLocked() {
				m.app.Toast = "Cannot build — fix Clang / AnyKernel3 paths in Setup first"
				m.app.ToastErr = true
				return m, nil
			}
			m.app.Screen = ScreenBuildOptions
			return m, m.app.buildOpts.Init()
		case "p":
			// Package-only — only valid when Image already exists in out/.
			if !m.app.HasImage {
				m.app.Toast = "No previous Image in out/ — run a full build first"
				m.app.ToastErr = true
				return m, nil
			}
			if m.pathsLocked() {
				m.app.Toast = "Cannot package — AnyKernel3 path missing"
				m.app.ToastErr = true
				return m, nil
			}
			m.app.PackageOnly = true
			m.app.Screen = ScreenBuild
			b, cmd := m.app.build.startBuild()
			m.app.build = b
			return m, cmd
		case "t":
			m.app.Screen = ScreenToolchain
			return m, m.app.toolchain.Init()
		case "k":
			m.app.Screen = ScreenKSU
			return m, m.app.ksu.Init()
		case "f":
			m.app.Screen = ScreenFeatures
			return m, m.app.features.Init()
		case "s":
			m.app.Screen = ScreenSetup
			return m, m.app.setup.Init()
		}
	}
	return m, nil
}

func (m MainMenu) View() string {
	// Refresh persisted state every time we paint the main menu so the
	// "Previous Build" panel and HasImage gating stay live after returning
	// from a sub-screen (a build, a counter reset, a menuconfig session).
	m.app.refreshPersistence()

	w := panelWidth(m.app.Width)
	innerW := innerContentWidth(w)

	// ── Banner ───────────────────────────────────────────────────────────────
	banner := components.Banner(
		"VAYU  KERNEL  BUILDER",
		"Linux 4.14 NonGKI · Poco X3 Pro (vayu) · Android 16 · ReSukiSU",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Previous Build panel ─────────────────────────────────────────────────
	// Mirrors the bash mode menu's grey "Previous Build" block. Empty body
	// shows a single-line "No previous build recorded" centered.
	var prevBody strings.Builder
	pb := m.app.PrevBuild
	if pb.Empty() {
		prevBody.WriteString(lipgloss.PlaceHorizontal(innerW, lipgloss.Center,
			DimText.Render("No previous build recorded")))
	} else {
		const lblW = 14
		buildLine := fmt.Sprintf("#%d", pb.Num)
		if pb.Date != "" {
			buildLine = buildLine + "   " + pb.Date
		}
		prevBody.WriteString(components.KV("Build", buildLine, lblW, LabelStyle, DimText))
		if pb.KernelName != "" {
			prevBody.WriteString("\n")
			prevBody.WriteString(components.KV("Kernel-Name", pb.KernelName, lblW, LabelStyle, DimText))
		}
		if pb.Mode != "" {
			prevBody.WriteString("\n")
			prevBody.WriteString(components.KV("Mode", pb.Mode, lblW, LabelStyle, DimText))
		}
		prevBody.WriteString("\n")
		prevBody.WriteString(components.Separator(innerW, MutedText))
		prevBody.WriteString("\n")
		cap := pb.Cap
		if cap == "" {
			cap = "Unknown"
		}
		prevBody.WriteString(components.KV("Capabilities", cap, lblW, LabelStyle, DimText))
		ext := pb.ExtFeat
		if ext == "" {
			ext = "[None]"
		}
		prevBody.WriteString("\n")
		prevBody.WriteString(components.KV("Features", ext, lblW, LabelStyle, DimText))
	}
	prevPanel := components.Panel("Previous Build", prevBody.String(), w, PanelDim, DimText.Bold(true))

	// ── Environment status panel ────────────────────────────────────────────
	var status strings.Builder
	rows := []struct {
		key, val string
		bad      bool
	}{
		{"Kernel", okOr(m.app.Paths.Kernel, "(not found)"), m.app.Paths.Kernel == ""},
		{"Clang", okOr(m.app.Paths.Clang, "(not installed)"), m.app.Paths.Clang == ""},
		{"AnyKernel3", okOr(m.app.Paths.AnyKernel, "(not found)"), m.app.Paths.AnyKernel == ""},
		{"Distro", string(m.app.Paths.Distro), false},
	}
	for i, r := range rows {
		v := ValueStyle
		if r.bad {
			v = ErrText
		}
		status.WriteString(components.KV(r.key, r.val, 11, LabelStyle, v))
		if i < len(rows)-1 {
			status.WriteString("\n")
		}
	}
	statusPanel := components.Panel("Environment", status.String(), w, PanelBorder, TitleStyle)

	// ── Menu panel ───────────────────────────────────────────────────────────
	type menuItem struct {
		key, label, rhs string
		fg              lipgloss.Color
		blocked         bool
	}
	branchTag := m.app.Builder.KSUBranch
	if branchTag == "" {
		branchTag = "main"
	}
	items := []menuItem{
		{"B", "Build kernel", "options → compile → package", ColorOK, m.pathsLocked()},
	}
	if m.app.HasImage {
		items = append(items, menuItem{"P", "Package existing image", "skip compile", ColorOK, m.pathsLocked()})
	}
	items = append(items,
		menuItem{"T", "Toolchain manager", "Google AOSP / ZyC", ColorAccent, false},
		menuItem{"K", "ReSukiSU driver", "branch: " + branchTag, ColorWarn, false},
		menuItem{"F", "Feature toggles", "KSU / SuSFS / KPM / menuconfig", ColorAccent, false},
		menuItem{"S", "Setup / paths", "deps + config", ColorAccent, false},
		menuItem{"Q", "Quit", "exit builder", ColorMuted, false},
	)
	var menu strings.Builder
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	maxKey := 0
	for _, it := range items {
		if len(it.key) > maxKey {
			maxKey = len(it.key)
		}
	}
	for i, it := range items {
		labelStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
		rhs := it.rhs
		rhsStyle := lipgloss.NewStyle().Foreground(it.fg)
		if it.blocked {
			labelStyle = lipgloss.NewStyle().Foreground(ColorMuted).Strikethrough(true)
			rhs = "BLOCKED — fix paths first"
			rhsStyle = ErrText
		}
		row := components.MenuRow(it.key, it.label, rhs, innerW, maxKey,
			HotKeyStyle, labelStyle, rhsStyle, leader,
		)
		menu.WriteString(row)
		if i < len(items)-1 {
			menu.WriteString("\n")
		}
	}
	menuPanel := components.Panel("Menu", menu.String(), w, PanelBorder, TitleStyle)

	// ── Conditional banners ──────────────────────────────────────────────────
	var extras strings.Builder
	if m.pathsLocked() {
		var msgs []string
		if m.app.Paths.Clang == "" {
			msgs = append(msgs, "[!] Clang not found — use [T] to fetch or [S] to fix path")
		}
		if m.app.Paths.AnyKernel == "" {
			msgs = append(msgs, "[!] AnyKernel3 not found — vendor it or fix path in [S]")
		}
		body := ErrText.Render(strings.Join(msgs, "\n"))
		extras.WriteString(components.Panel("Path Issues", body, w, PanelErr, ErrText))
		extras.WriteString("\n")
	}
	if m.app.MenuconfigPreserved {
		body := lipgloss.PlaceHorizontal(innerW, lipgloss.Center,
			lipgloss.NewStyle().Foreground(ColorBanner).Bold(true).
				Render("Preserved menuconfig .config — will be restored on next build"))
		extras.WriteString(components.Panel("", body, w,
			PanelBorder.BorderForeground(ColorBanner), TitleStyle))
		extras.WriteString("\n")
	}

	// ── Toast + help ─────────────────────────────────────────────────────────
	var toast string
	if m.app.Toast != "" {
		toast = "  " + components.Toast(m.app.Toast, m.app.ToastErr) + "\n"
	}

	// Build the option string dynamically: [B/P/T/K/F/S/Q]
	opts := []string{"B"}
	if m.app.HasImage {
		opts = append(opts, "P")
	}
	opts = append(opts, "T", "K", "F", "S", "Q")
	help := HelpStyle.Render(fmt.Sprintf(
		"  press [%s] · esc/q to quit · terminal %dx%d",
		strings.Join(opts, "/"), m.app.Width, m.app.Height,
	))

	divider := "  " + components.Separator(innerW, MutedText) + "\n"
	return banner + "\n" +
		prevPanel + "\n" +
		statusPanel + "\n" +
		menuPanel + "\n" +
		extras.String() +
		divider +
		toast +
		help
}

func okOr(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

// clampWidth returns the outer panel/banner width to use given the terminal
// columns. We clamp to a minimum so very narrow terminals still produce
// readable output, and to a generous maximum so ultra-wide terminals
// (>200 cols) don't render absurdly stretched dotted leaders. Pass max <= 0
// to disable the upper bound.
func clampWidth(w, min, max int) int {
	if w < min {
		return min
	}
	if max > 0 && w > max {
		return max
	}
	return w
}

// panelWidth returns the screen width used for banners/panels: the full
// terminal width minus a 2-column right gutter so trailing borders never
// touch the right edge of the terminal (looks much cleaner on most emulators).
func panelWidth(termWidth int) int {
	if termWidth <= 0 {
		termWidth = 80
	}
	w := termWidth - 2
	if w < 60 {
		w = 60
	}
	if w > 160 {
		w = 160
	}
	return w
}

// innerContentWidth is the visible content area inside a panel after
// border (2) + horizontal padding (2). Used to size MenuRow leaders, KV
// padding, etc.
func innerContentWidth(outerWidth int) int {
	w := outerWidth - 4
	if w < 20 {
		w = 20
	}
	return w
}
