// Package components has small reusable widgets: hotkey rows, key/value
// rows, header banners. Pure renderers (no state).
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Banner returns a centered title bar.
func Banner(title, subtitle string, width int, fg, accent lipgloss.Color) string {
	t := lipgloss.NewStyle().Foreground(fg).Bold(true).Render(title)
	s := ""
	if subtitle != "" {
		s = "  " + lipgloss.NewStyle().Foreground(accent).Render(subtitle)
	}
	bar := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(t + s)
	return bar
}

// KV renders a key/value row used in info boxes.
//
//	  Source     : Auto: Google -> ZyC fallback
//
// keyWidth pads the key column for alignment.
func KV(key, value string, keyWidth int, keyStyle, valStyle lipgloss.Style) string {
	k := keyStyle.Render(fmt.Sprintf("%-*s", keyWidth, key))
	v := valStyle.Render(value)
	return "  " + k + "  " + v
}

// Hotkey renders "[K] Description (subtext)".
func Hotkey(key, desc, sub string, keyStyle, descStyle, subStyle lipgloss.Style) string {
	out := keyStyle.Render("[" + key + "]") + "  " + descStyle.Render(desc)
	if sub != "" {
		out += "  " + subStyle.Render("("+sub+")")
	}
	return "  " + out
}

// Rule draws a horizontal line of length width with optional inline label.
func Rule(label string, width int, style lipgloss.Style) string {
	if label == "" {
		return style.Render(strings.Repeat("─", width))
	}
	mid := " " + label + " "
	left := 2
	right := width - len(mid) - left
	if right < 0 {
		right = 0
	}
	return style.Render(strings.Repeat("─", left) + mid + strings.Repeat("─", right))
}
