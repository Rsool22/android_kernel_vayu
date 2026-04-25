// Package ui implements the Bubble Tea TUI for the vayu kernel builder.
//
// Visual conventions:
//   • Lipgloss styles named after roles (title, ok, warn, err) are the
//     single source of truth -- never inline ANSI in models.
//   • Boxes are 78 columns wide by default, auto-resized to terminal width.
//   • Each "screen" is its own Model implementing tea.Model. The root app
//     (App) holds the current screen and routes window/keyboard events.
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Width is the target draw width; resized live by App on tea.WindowSizeMsg.
var Width = 80

// Palette adapted from the bash TUI: orange title, cyan accents,
// green/yellow/red status, dim greys for secondary text.
var (
	ColorTitle = lipgloss.Color("214") // orange
	ColorAccent = lipgloss.Color("87") // cyan
	ColorOK     = lipgloss.Color("82") // green
	ColorWarn   = lipgloss.Color("221")
	ColorErr    = lipgloss.Color("203")
	ColorDim    = lipgloss.Color("245")
	ColorMuted  = lipgloss.Color("240")
	ColorSel    = lipgloss.Color("212") // pink-ish for selected list rows
)

var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorTitle).
			Bold(true).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(false)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorAccent).
			Padding(0, 1)

	BoxOK = BoxStyle.BorderForeground(ColorOK)
	BoxWarn = BoxStyle.BorderForeground(ColorWarn)
	BoxErr = BoxStyle.BorderForeground(ColorErr)
	BoxDim = BoxStyle.BorderForeground(ColorMuted)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorDim)

	HotKeyStyle = lipgloss.NewStyle().
			Foreground(ColorTitle).
			Bold(true)

	OKText = lipgloss.NewStyle().Foreground(ColorOK)
	WarnText = lipgloss.NewStyle().Foreground(ColorWarn)
	ErrText = lipgloss.NewStyle().Foreground(ColorErr)
	DimText = lipgloss.NewStyle().Foreground(ColorDim)
	MutedText = lipgloss.NewStyle().Foreground(ColorMuted)
	AccentText = lipgloss.NewStyle().Foreground(ColorAccent)
	SelText = lipgloss.NewStyle().Foreground(ColorSel).Bold(true)
)
