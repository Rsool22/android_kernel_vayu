// Package ui — Settings screen.
//
// Lets the user pick the visual variant (bash / modern / mono), set or
// clear the accent-colour override, and persist the choice to
// ~/.config/vayu_builder/config so it survives a restart. Mirrors the
// look and feel of the toolchain source-selector.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// SettingsScreen toggles the live theme variant and accent colour.
type SettingsScreen struct {
	app     *App
	editing bool
	input   textinput.Model
}

// NewSettingsScreen constructs the screen with a textinput for the
// accent colour override (256-colour ANSI index or hex string).
func NewSettingsScreen(a *App) SettingsScreen {
	ti := textinput.New()
	ti.Placeholder = "214  or  #5fafff   (empty to clear override)"
	ti.CharLimit = 16
	ti.Width = 28
	ti.Prompt = "▶ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorValue)
	return SettingsScreen{app: a, input: ti}
}

func (s SettingsScreen) Init() tea.Cmd { return nil }

func (s SettingsScreen) Update(msg tea.Msg) (SettingsScreen, tea.Cmd) {
	if s.editing {
		switch m := msg.(type) {
		case tea.KeyMsg:
			switch m.String() {
			case "enter":
				v := strings.TrimSpace(s.input.Value())
				s.app.Cfg.AccentColor = v
				ApplyAccentOverride(v)
				s.app.PersistConfig()
				s.app.Toast = "Accent colour updated."
				s.app.ToastErr = false
				s.editing = false
				s.input.Blur()
				return s, nil
			case "esc":
				s.editing = false
				s.input.Blur()
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(m.String()) {
		case "1", "b":
			ApplyStyle(StyleBash)
			s.app.Cfg.Theme = "bash"
			s.app.PersistConfig()
			s.app.Toast = "Theme: bash (double-line)."
			s.app.ToastErr = false
		case "2", "m":
			ApplyStyle(StyleModern)
			s.app.Cfg.Theme = "modern"
			s.app.PersistConfig()
			s.app.Toast = "Theme: modern (rounded, soft palette)."
			s.app.ToastErr = false
		case "3", "n":
			ApplyStyle(StyleMono)
			s.app.Cfg.Theme = "mono"
			s.app.PersistConfig()
			s.app.Toast = "Theme: mono (low-colour)."
			s.app.ToastErr = false
		case "a":
			s.editing = true
			s.input.SetValue(s.app.Cfg.AccentColor)
			s.input.Focus()
			return s, textinput.Blink
		case "r":
			s.app.Cfg.AccentColor = ""
			ApplyAccentOverride("")
			s.app.PersistConfig()
			s.app.Toast = "Accent colour reset to theme default."
			s.app.ToastErr = false
		case "p":
			// Cycle accent presets (orange → cyan → green → magenta → reset).
			next := nextAccentPreset(s.app.Cfg.AccentColor)
			s.app.Cfg.AccentColor = next
			ApplyAccentOverride(next)
			s.app.PersistConfig()
			if next == "" {
				s.app.Toast = "Accent colour reset to theme default."
			} else {
				s.app.Toast = "Accent colour: " + next
			}
			s.app.ToastErr = false
		}
	}
	return s, nil
}

// accentPresets is the cycle order for the [P] hotkey. Empty resets
// back to the theme default.
var accentPresets = []string{"", "214", "87", "46", "201", "117", "141"}

func nextAccentPreset(cur string) string {
	for i, p := range accentPresets {
		if p == cur {
			return accentPresets[(i+1)%len(accentPresets)]
		}
	}
	return accentPresets[0]
}

func (s SettingsScreen) View() string {
	w := panelWidth(s.app.Width)
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"SETTINGS  ·  THEME",
		"choose visual style and accent colour",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// State panel
	var st strings.Builder
	st.WriteString(components.KV("Theme", string(themeLabel(CurrentStyle)), 11, LabelStyle, ValueStyle) + "\n")
	accent := s.app.Cfg.AccentColor
	if accent == "" {
		accent = "(theme default)"
	}
	st.WriteString(components.KV("Accent", accent, 11, LabelStyle, AccentText))
	statePanel := components.Panel("Active", st.String(), w, PanelBorder, TitleStyle)

	// Theme variants
	var themes strings.Builder
	themes.WriteString(themeRow("1", "Bash", "double-line magenta / cyan / yellow", CurrentStyle == StyleBash, innerW) + "\n")
	themes.WriteString(themeRow("2", "Modern", "rounded corners, soft purple / sky / lavender", CurrentStyle == StyleModern, innerW) + "\n")
	themes.WriteString(themeRow("3", "Mono", "single colour, low-colour terminal friendly", CurrentStyle == StyleMono, innerW))
	themesPanel := components.Panel("Theme", themes.String(), w, PanelBorder, TitleStyle)

	// Accent picker
	var acc strings.Builder
	acc.WriteString(components.GlobalBracketTag("A", HotKeyStyle) + "  " +
		ValueStyle.Render("Set accent colour") + "  " +
		MutedText.Render("(256-colour index or #hex)") + "\n")
	acc.WriteString(components.GlobalBracketTag("P", HotKeyStyle) + "  " +
		ValueStyle.Render("Cycle preset") + "  " +
		MutedText.Render("orange · cyan · green · magenta · sky · lavender") + "\n")
	acc.WriteString(components.GlobalBracketTag("R", HotKeyStyle) + "  " +
		ValueStyle.Render("Reset to theme default"))
	accPanel := components.Panel("Accent", acc.String(), w, PanelBorder, TitleStyle)

	// Inline editor
	var editor string
	if s.editing {
		var b strings.Builder
		b.WriteString(LabelStyle.Render("New accent: ") + "\n")
		b.WriteString(s.input.View())
		editor = "\n" + components.Panel("Edit accent", b.String(), w,
			PanelBorder.BorderForeground(ColorBanner),
			TitleStyle.Foreground(ColorBanner)) + "\n"
	}

	notice := components.Notice("info",
		"Theme + accent colour are persisted to ~/.config/vayu_builder/config and reapplied on next launch.",
		w, PanelDim, TitleStyle, DimText)

	actions := components.HotkeyStrip([]components.Hotkey{
		{Key: "1", Desc: "Bash"},
		{Key: "2", Desc: "Modern"},
		{Key: "3", Desc: "Mono"},
		{Key: "A", Desc: "Accent"},
		{Key: "P", Desc: "Cycle"},
		{Key: "R", Desc: "Reset"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := "  " + components.Separator(innerW, MutedText) + "\n"
	out := banner + "\n" + statePanel + "\n" + themesPanel + "\n" + accPanel + editor + "\n" + notice + "\n" +
		divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

func themeLabel(v StyleVariant) string {
	switch v {
	case StyleModern:
		return "Modern"
	case StyleMono:
		return "Mono"
	default:
		return "Bash"
	}
}

// themeRow renders one theme option with a navigation chevron and a
// dotted leader pulling the ACTIVE badge to the right edge.
func themeRow(key, name, desc string, selected bool, width int) string {
	arrow := components.NavArrow(selected, SelText)
	tag := components.GlobalBracketTag(key, HotKeyStyle)
	prefix := arrow + tag + "  " +
		ValueStyle.Render(name) + "  " +
		DimText.Render(desc)
	if selected {
		return components.LeaderRow(prefix, components.Badge("ACTIVE", BadgeAccent), width, MutedText)
	}
	return prefix
}
