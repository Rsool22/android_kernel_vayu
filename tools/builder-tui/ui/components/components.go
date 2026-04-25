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
// screen.
//
// width is the *outer* width including borders. The banner auto-wraps long
// titles/subtitles to the inner width so a narrow terminal doesn't overflow.
func Banner(title, subtitle string, width int, border lipgloss.Style, titleStyle, subtleStyle lipgloss.Style) string {
	if width < 20 {
		width = 20
	}
	inner := width - 4 // border (2) + padding (2)
	if inner < 8 {
		inner = 8
	}
	titleLine := lipgloss.PlaceHorizontal(inner, lipgloss.Center,
		titleStyle.Render(truncate(title, inner)))
	body := titleLine
	if subtitle != "" {
		body += "\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center,
			subtleStyle.Render(truncate(subtitle, inner)))
	}
	return border.Width(inner).Render(body)
}

// Panel renders a cyan double-bordered panel with an optional centered title
// row at the top. body is rendered as-is (already styled by the caller).
//
// width is the *outer* width.
func Panel(title, body string, width int, border lipgloss.Style, titleStyle lipgloss.Style) string {
	if width < 12 {
		width = 12
	}
	inner := width - 4
	if inner < 4 {
		inner = 4
	}
	if title != "" {
		head := lipgloss.PlaceHorizontal(inner, lipgloss.Center,
			titleStyle.Render(strings.ToUpper(truncate(title, inner))))
		div := lipgloss.NewStyle().Foreground(border.GetBorderTopForeground()).Render(strings.Repeat("─", inner))
		body = head + "\n" + div + "\n" + body
	}
	return border.Width(inner).Render(body)
}

// KV renders a left-aligned key/value row used inside panels.
//
// keyWidth pads the key column; the colon and spacing are added here.
// When the rendered value is wider than maxValueWidth (when > 0), it wraps to
// a continuation line indented under the value column so long paths don't
// overflow off-screen.
func KV(key, value string, keyWidth int, keyStyle, valStyle lipgloss.Style) string {
	return KVWrap(key, value, keyWidth, 0, keyStyle, valStyle)
}

// KVWrap is KV with a value wrap width. valueWidth=0 disables wrapping.
func KVWrap(key, value string, keyWidth, valueWidth int, keyStyle, valStyle lipgloss.Style) string {
	k := keyStyle.Render(fmt.Sprintf("%-*s", keyWidth, key))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" : ")
	prefix := k + sep
	prefixW := lipgloss.Width(prefix)
	indent := strings.Repeat(" ", prefixW)
	if valueWidth <= 0 || lipgloss.Width(value) <= valueWidth {
		return prefix + valStyle.Render(value)
	}
	lines := wrapPlain(value, valueWidth)
	out := prefix + valStyle.Render(lines[0])
	for _, l := range lines[1:] {
		out += "\n" + indent + valStyle.Render(l)
	}
	return out
}

// BracketTag renders "[label]" right-padded so the closing bracket sits at
// column (1 + maxLabelWidth + 1). Used to keep [main]/[dev] columns aligned
// regardless of inner text length.
func BracketTag(label string, maxLabelWidth int, style lipgloss.Style) string {
	if maxLabelWidth < len(label) {
		maxLabelWidth = len(label)
	}
	inner := label + strings.Repeat(" ", maxLabelWidth-len(label))
	return style.Render("[" + inner + "]")
}

// MenuRow renders a hotkey-style menu line that fills available width:
//
//	  [B]  Build kernel ......................... compile + package
//
// The dotted leader stretches between label and rhs to align values to the right.
// keyWidth pads the bracketed key so closing brackets line up across rows.
// Falls back to label-only when width is too narrow for the leader+rhs.
func MenuRow(key, label, rhs string, width, keyWidth int, keyStyle, labelStyle, rhsStyle, leaderStyle lipgloss.Style) string {
	tag := BracketTag(key, keyWidth, keyStyle)
	prefix := tag + "  " + labelStyle.Render(label)
	if rhs == "" || width <= lipgloss.Width(prefix)+4 {
		return prefix
	}
	rendered := lipgloss.Width(prefix) + lipgloss.Width(rhs) + 2
	pad := width - rendered
	if pad < 1 {
		// rhs would overflow; drop it cleanly rather than wrap mid-row.
		return prefix
	}
	leader := leaderStyle.Render(" " + strings.Repeat("·", pad-2) + " ")
	return prefix + leader + rhsStyle.Render(rhs)
}

// Hotkey describes one entry in a footer action strip.
type Hotkey struct {
	Key, Desc, Sub string
}

// HotkeyStrip renders a list of hotkeys with brackets right-padded so every
// closing `]` aligns to the same column inside the strip. Items are joined
// with a coloured separator. Wraps to multiple lines when the joined width
// exceeds maxWidth (when > 0) so narrow terminals don't get a mid-bracket
// wrap from the terminal emulator.
func HotkeyStrip(items []Hotkey, keyStyle, descStyle, subStyle, sepStyle lipgloss.Style) string {
	return HotkeyStripWrap(items, 0, keyStyle, descStyle, subStyle, sepStyle)
}

// HotkeyStripWrap is HotkeyStrip with a maxWidth (0 = no wrap).
func HotkeyStripWrap(items []Hotkey, maxWidth int, keyStyle, descStyle, subStyle, sepStyle lipgloss.Style) string {
	maxKey := 0
	for _, h := range items {
		if len(h.Key) > maxKey {
			maxKey = len(h.Key)
		}
	}
	rendered := make([]string, 0, len(items))
	for _, h := range items {
		row := BracketTag(h.Key, maxKey, keyStyle) + " " + descStyle.Render(h.Desc)
		if h.Sub != "" {
			row += " " + subStyle.Render("("+h.Sub+")")
		}
		rendered = append(rendered, row)
	}
	sep := sepStyle.Render("  \u2502  ")
	sepW := lipgloss.Width(sep)
	if maxWidth <= 0 {
		return strings.Join(rendered, sep)
	}
	// Pack into rows of width <= maxWidth.
	var lines []string
	var cur string
	curW := 0
	for _, r := range rendered {
		rw := lipgloss.Width(r)
		if cur == "" {
			cur = r
			curW = rw
			continue
		}
		if curW+sepW+rw > maxWidth {
			lines = append(lines, cur)
			cur = r
			curW = rw
		} else {
			cur += sep + r
			curW += sepW + rw
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
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

// truncate shortens s to fit width using a trailing ellipsis. Returns s
// unchanged when width is large enough.
func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	// Naive byte-trim is fine here because the strings we feed in are ASCII.
	cut := width - 1
	if cut > len(s) {
		cut = len(s)
	}
	return s[:cut] + "…"
}

// wrapPlain wraps s into lines no wider than width, breaking on path
// separators, hyphens, and spaces preferentially, otherwise hard-cutting.
// Used by KVWrap to wrap long path values to a continuation line.
func wrapPlain(s string, width int) []string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return []string{s}
	}
	var lines []string
	for lipgloss.Width(s) > width {
		// Find the last separator at or before width.
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
