// Package components provides reusable widgets for the vayu-builder TUI:
// banners, panels, key/value rows, hotkey rows, section rules, status badges.
//
// Pure renderers -- they take styles and content, return a string. State is
// owned by the screen models; this package only paints.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Banner renders the magenta double-bordered title banner that opens every
// screen. The visual is modeled on the bash TUI's:
//
//	╔════════════════════════════════════════════════════════════════════╗
//	║          VAYU  KERNEL  BUILDER  --  AnyMore Project                ║
//	║   Linux 4.14 NonGKI | Poco X3 Pro (vayu) | Android 16              ║
//	╚════════════════════════════════════════════════════════════════════╝
//
// title is rendered in titleStyle (bold, value color), subtitle in subtleStyle
// (italic accent), both centered. width is the *outer* width including borders.
func Banner(title, subtitle string, width int, border lipgloss.Style, titleStyle, subtleStyle lipgloss.Style) string {
	if width < 30 {
		width = 30
	}
	inner := width - 4 // border (2) + padding (2)
	titleLine := lipgloss.PlaceHorizontal(inner, lipgloss.Center, titleStyle.Render(title))
	body := titleLine
	if subtitle != "" {
		body += "\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, subtleStyle.Render(subtitle))
	}
	return border.Width(inner).Render(body)
}

// Panel renders a cyan double-bordered panel with an optional centered title
// row at the top. body is rendered as-is (already styled by the caller).
//
//	╔══════════════════════════ TITLE ═══════════════════════════════════╗
//	║   ...body...                                                        ║
//	╚══════════════════════════════════════════════════════════════════════╝
//
// width is the *outer* width.
func Panel(title, body string, width int, border lipgloss.Style, titleStyle lipgloss.Style) string {
	if width < 20 {
		width = 20
	}
	inner := width - 4
	if title != "" {
		head := lipgloss.PlaceHorizontal(inner, lipgloss.Center, titleStyle.Render(strings.ToUpper(title)))
		div := lipgloss.NewStyle().Foreground(border.GetBorderTopForeground()).Render(strings.Repeat("─", inner))
		body = head + "\n" + div + "\n" + body
	}
	return border.Width(inner).Render(body)
}

// KV renders a left-aligned key/value row used inside panels:
//
//	  Source       : Auto: Google -> ZyC fallback
//
// keyWidth pads the key column; the colon and spacing are added here.
func KV(key, value string, keyWidth int, keyStyle, valStyle lipgloss.Style) string {
	k := keyStyle.Render(fmt.Sprintf("%-*s", keyWidth, key))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" : ")
	return k + sep + valStyle.Render(value)
}

// MenuRow renders a hotkey-style menu line that fills available width:
//
//	  [B]  Build kernel ......................... compile + package
//
// The dotted leader stretches between label and rhs to align values to the right.
func MenuRow(key, label, rhs string, width int, keyStyle, labelStyle, rhsStyle, leaderStyle lipgloss.Style) string {
	prefix := keyStyle.Render("["+key+"]") + "  " + labelStyle.Render(label)
	rendered := lipgloss.Width(prefix) + lipgloss.Width(rhs) + 2
	pad := width - rendered
	if pad < 1 {
		pad = 1
	}
	leader := leaderStyle.Render(" " + strings.Repeat("·", pad-2) + " ")
	return prefix + leader + rhsStyle.Render(rhs)
}

// Hotkey renders a compact "[K] Description (subtext)" row used in action
// strips at the bottom of screens.
func Hotkey(key, desc, sub string, keyStyle, descStyle, subStyle lipgloss.Style) string {
	out := keyStyle.Render("["+key+"]") + " " + descStyle.Render(desc)
	if sub != "" {
		out += " " + subStyle.Render("("+sub+")")
	}
	return out
}

// HotkeyStrip joins multiple hotkey strings with a thin separator, used as
// a footer action bar.
func HotkeyStrip(items []string, sepStyle lipgloss.Style) string {
	sep := sepStyle.Render("  │  ")
	return strings.Join(items, sep)
}

// Rule draws a horizontal `── label ───────...` divider, used between sections
// inside a panel.
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
	return style.Render(strings.Repeat("─", left)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true).Render(mid) +
		style.Render(strings.Repeat("─", right))
}

// Badge wraps a short status word in a coloured pill. Use the BadgeOK/Warn/Err
// styles from theme.go.
func Badge(text string, style lipgloss.Style) string {
	return style.Render(strings.ToUpper(text))
}

// Toast renders a final-line status message (success or error) with a small
// gem prefix so it's distinct from regular content.
func Toast(msg string, isErr bool) string {
	if msg == "" {
		return ""
	}
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true).Render("✓ ")
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(msg)
	if isErr {
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("✗ ")
		body = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(msg)
	}
	return prefix + body
}

// Spinner returns a unicode spinner frame for a step counter, used while
// build/probe/fetch is running. Pure function -- caller advances counter.
func Spinner(step int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[step%len(frames)]
}
