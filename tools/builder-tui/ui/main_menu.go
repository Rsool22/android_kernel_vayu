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
	w := m.app.Width
	if w < 60 {
		w = 60
	}
	if w > 100 {
		w = 100
	}

	title := components.Banner("VAYU  KERNEL  BUILDER", "Android 16 / NonGKI / SM8150", w, ColorTitle, ColorAccent)

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(components.Rule("", w, MutedText) + "\n\n")

	// Status block
	statusKey := lipgloss.NewStyle().Foreground(ColorDim).Bold(false)
	statusVal := lipgloss.NewStyle().Foreground(ColorAccent)
	b.WriteString(components.KV("Kernel", okOr(m.app.Paths.Kernel, "(not found)"), 12, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Clang ", okOr(m.app.Paths.Clang, "(not found)"), 12, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("AnyKernel3", okOr(m.app.Paths.AnyKernel, "(not found)"), 12, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Distro", string(m.app.Paths.Distro), 12, statusKey, statusVal) + "\n")
	b.WriteString("\n")

	b.WriteString(components.Rule("Menu", w, MutedText) + "\n")
	b.WriteString(components.Hotkey("B", "Build kernel", "compile + package", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorOK), DimText) + "\n")
	b.WriteString(components.Hotkey("T", "Toolchain manager", "Google / ZyC clang", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorAccent), DimText) + "\n")
	b.WriteString(components.Hotkey("K", "ReSukiSU driver", "install / update", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorWarn), DimText) + "\n")
	b.WriteString(components.Hotkey("S", "Setup / paths", "deps + config", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorAccent), DimText) + "\n")
	b.WriteString(components.Hotkey("Q", "Quit", "", HotKeyStyle, DimText, DimText) + "\n")
	b.WriteString("\n")

	if m.app.Toast != "" {
		st := OKText
		if m.app.ToastErr {
			st = ErrText
		}
		b.WriteString(st.Render("  " + m.app.Toast) + "\n")
	}

	help := DimText.Render(fmt.Sprintf("  ESC/Q to quit • size %dx%d", m.app.Width, m.app.Height))
	b.WriteString("\n" + help)
	return b.String()
}

func okOr(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}
