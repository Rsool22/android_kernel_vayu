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

// GlobalKeyWidth is the maximum bracketed-key label width across every
// screen in the TUI. All [X] / [ESC] / [main] tags are padded to this
// width so closing brackets line up in the same column regardless of
// which page the user is on. Computed once at package init from a
// curated list of the keys actually rendered by the screens.
var GlobalKeyWidth = computeGlobalKeyWidth()

func computeGlobalKeyWidth() int {
	// Keys used across every screen + the longest branch labels. Keep this
	// list in sync when adding new hotkeys; it is intentionally a static
	// list rather than computed at runtime so screens render identically
	// before any data is loaded.
	known := []string{
		"B", "P", "T", "K", "F", "S", "D", "Q",
		"N", "I", "C", "X", "R", "E", "V", "M",
		"A", "G", "Z", "L",
		"1", "2", "3", "4", "5", "6",
		"main", "dev",
		"ESC", "Enter", "Ctrl+C", "↑/↓", "1-6",
	}
	w := 1
	for _, k := range known {
		if lipgloss.Width(k) > w {
			w = lipgloss.Width(k)
		}
	}
	return w
}

// Leader returns a dotted leader of `n` dots wrapped in single-space
// gutters, e.g. ` ··········· `. Returns just the gutters (no dots) when
// the available pad is too small. Always safe -- never panics on small
// or negative input. style is applied to the entire leader.
func Leader(pad int, style lipgloss.Style) string {
	n := pad - 2
	if n < 0 {
		n = 0
	}
	return style.Render(" " + strings.Repeat("·", n) + " ")
}

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
	inner := InnerWidth(width)
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
	inner := InnerWidth(width)
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

// BracketTag renders "[label]" right-padded so the closing bracket sits at
//
//	column (1 + maxLabelWidth + 1). Used to keep [main]/[dev] columns aligned
//
// regardless of inner text length.
func BracketTag(label string, maxLabelWidth int, style lipgloss.Style) string {
	if maxLabelWidth < lipgloss.Width(label) {
		maxLabelWidth = lipgloss.Width(label)
	}
	pad := maxLabelWidth - lipgloss.Width(label)
	if pad < 0 {
		pad = 0
	}
	inner := label + strings.Repeat(" ", pad)
	return style.Render("[" + inner + "]")
}

// GlobalBracketTag renders BracketTag using GlobalKeyWidth so the closing
// `]` aligns to the same column on every screen of the TUI.
func GlobalBracketTag(label string, style lipgloss.Style) string {
	return BracketTag(label, GlobalKeyWidth, style)
}

// NavArrow returns a single-glyph navigation chevron. Selected items get a
// solid right-pointing arrow; unselected items get a thin spacer that takes
// the same width so columns line up. Used by source/branch/feature menus
// so the active option is unambiguous (the previous bullet `●` was easy
// to mistake for a list marker).
func NavArrow(selected bool, style lipgloss.Style) string {
	if selected {
		return style.Render("▸ ")
	}
	return "  "
}

// MenuRow renders a hotkey-style menu line that fills available width:
//
//	  [B]  Build kernel ......................... compile + package
//
// The dotted leader stretches between label and rhs to align values to the right.
// keyWidth pads the bracketed key so closing brackets line up across rows.
func MenuRow(key, label, rhs string, width, keyWidth int, keyStyle, labelStyle, rhsStyle, leaderStyle lipgloss.Style) string {
	tag := BracketTag(key, keyWidth, keyStyle)
	prefix := tag + "  " + labelStyle.Render(label)
	rendered := lipgloss.Width(prefix) + lipgloss.Width(rhs) + 2
	pad := width - rendered
	if pad < 1 {
		pad = 1
	}
	return prefix + Leader(pad, leaderStyle) + rhsStyle.Render(rhs)
}

// LeaderRow renders an arbitrary `prefix .......... rhs` row, computing
// the dotted leader so prefix's left edge and rhs's right edge meet the
// available width. Both prefix and rhs must already be styled by the
// caller; the leader is styled with leaderStyle. Safe for any width and
// any prefix/rhs visible widths -- never panics.
func LeaderRow(prefix, rhs string, width int, leaderStyle lipgloss.Style) string {
	rendered := lipgloss.Width(prefix) + lipgloss.Width(rhs) + 2
	pad := width - rendered
	if pad < 1 {
		pad = 1
	}
	return prefix + Leader(pad, leaderStyle) + rhs
}

// Notice renders a single-line, panel-titled informational note. Used
// for [i] info / [!] warning / [×] error banners that mirror the bash
// build script's coloured tip lines:
//
//	╔══════════════════════════ NOTE ════════════════════════════════════╗
//	║   [i]  Stash your local edits before switching branches            ║
//	╚══════════════════════════════════════════════════════════════════════╝
func Notice(level, msg string, width int, border lipgloss.Style, titleStyle, bodyStyle lipgloss.Style) string {
	inner := InnerWidth(width)
	if inner < 12 {
		inner = 12
	}
	prefix := "[i]"
	title := "NOTE"
	switch strings.ToLower(level) {
	case "warn", "warning":
		prefix = "[!]"
		title = "WARNING"
	case "err", "error":
		prefix = "[\u00d7]"
		title = "ERROR"
	case "ok", "success":
		prefix = "[\u2713]"
		title = "OK"
	}
	body := bodyStyle.Render(prefix+" ") + bodyStyle.Render(msg)
	return Panel(title, body, width, border, titleStyle)
}

// Hotkey describes one entry in a footer action strip.
type Hotkey struct {
	Key, Desc, Sub string
}

// HotkeyStrip renders a list of hotkeys with brackets right-padded so every
// closing `]` aligns to the same column inside the strip. Items are joined
// with a coloured separator.
func HotkeyStrip(items []Hotkey, keyStyle, descStyle, subStyle, sepStyle lipgloss.Style) string {
	maxKey := 0
	for _, h := range items {
		if len(h.Key) > maxKey {
			maxKey = len(h.Key)
		}
	}
	parts := make([]string, 0, len(items))
	for _, h := range items {
		row := BracketTag(h.Key, maxKey, keyStyle) + " " + descStyle.Render(h.Desc)
		if h.Sub != "" {
			row += " " + subStyle.Render("("+h.Sub+")")
		}
		parts = append(parts, row)
	}
	sep := sepStyle.Render("  \u2502  ")
	return strings.Join(parts, sep)
}

// Separator returns a thin horizontal divider used between sections at
// `width` cols, drawn with `style`. Pure function -- no padding.
func Separator(width int, style lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	return style.Render(strings.Repeat("\u2500", width))
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
