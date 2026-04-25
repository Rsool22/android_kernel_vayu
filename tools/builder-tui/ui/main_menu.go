package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// MainMenu is the entry screen. Hotkeys: B/T/K/S/Q.
type MainMenu struct {
	app *App
}

func NewMainMenu(a *App) MainMenu { return MainMenu{app: a} }

func (m MainMenu) Init() tea.Cmd { return nil }

func (m MainMenu) Update(msg tea.Msg) (MainMenu, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "b":
			m.app.Screen = ScreenBuild
			return m, m.app.build.Init()
		case "t":
			m.app.Screen = ScreenToolchain
			return m, m.app.toolchain.Init()
		case "k":
			m.app.Screen = ScreenKSU
			return m, m.app.ksu.Init()
		case "s":
			m.app.Screen = ScreenSetup
			return m, m.app.setup.Init()
		}
	}
	return m, nil
}

func (m MainMenu) View() string {
	w := clampWidth(m.app.Width, 64, 110)

	// ── Banner ───────────────────────────────────────────────────────────────
	banner := components.Banner(
		"VAYU  KERNEL  BUILDER",
		"Linux 4.14 NonGKI · Poco X3 Pro (vayu) · Android 16 · ReSukiSU",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Status panel ─────────────────────────────────────────────────────────
	innerW := w - 4
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
	menuItems := []struct {
		key, label, rhs string
		fg              lipgloss.Color
	}{
		{"B", "Build kernel", "compile + package", ColorOK},
		{"T", "Toolchain manager", "Google AOSP / ZyC", ColorAccent},
		{"K", "ReSukiSU driver", "install / update", ColorWarn},
		{"S", "Setup / paths", "deps + config", ColorAccent},
		{"Q", "Quit", "exit builder", ColorMuted},
	}
	var menu strings.Builder
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	innerMenuW := innerW - 2 // panel padding (1) on each side
	for i, it := range menuItems {
		row := components.MenuRow(it.key, it.label, it.rhs, innerMenuW,
			HotKeyStyle,
			lipgloss.NewStyle().Foreground(ColorValue).Bold(true),
			lipgloss.NewStyle().Foreground(it.fg),
			leader,
		)
		menu.WriteString(row)
		if i < len(menuItems)-1 {
			menu.WriteString("\n")
		}
	}
	menuPanel := components.Panel("Menu", menu.String(), w, PanelBorder, TitleStyle)

	// ── Toast + help ─────────────────────────────────────────────────────────
	var toast string
	if m.app.Toast != "" {
		toast = "  " + components.Toast(m.app.Toast, m.app.ToastErr) + "\n"
	}

	help := HelpStyle.Render(fmt.Sprintf("  press a hotkey · esc/q to quit · terminal %dx%d", m.app.Width, m.app.Height))

	return banner + "\n" +
		statusPanel + "\n" +
		menuPanel + "\n" +
		toast +
		help
}

func okOr(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func clampWidth(w, min, max int) int {
	if w < min {
		return min
	}
	if w > max {
		return max
	}
	return w
}
