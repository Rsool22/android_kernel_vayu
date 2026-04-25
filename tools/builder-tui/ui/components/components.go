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
	// Lipgloss adds 2 border + 2 horizontal padding columns when the
	// caller's style sets Padding(0, 1) (BannerBorder/PanelBorder do).
	// We size the inner block to width - 4 so the rendered box is
	// exactly `width` columns wide.
	inner := width - 4
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
// width is the *outer* width. The panel borders are drawn manually (rather
// than via lipgloss.Style.Border) so the title divider can use the proper
// double-border intersection runes ╠ and ╣ — matching the bash original's
// box_div output where the divider visually fuses with the side borders.
func Panel(title, body string, width int, border lipgloss.Style, titleStyle lipgloss.Style) string {
	if width < 12 {
		width = 12
	}
	// 2 columns for the side borders, 2 for the 1-column horizontal
	// padding kept on each side so body text doesn't hug the borders.
	inner := width - 2
	if inner < 6 {
		inner = 6
	}
	pad := 1
	contentW := inner - 2*pad

	color := border.GetBorderTopForeground()
	sideStyle := lipgloss.NewStyle().Foreground(color)
	side := sideStyle.Render("║")

	bar := strings.Repeat("═", inner)
	top := sideStyle.Render("╔" + bar + "╗")
	bot := sideStyle.Render("╚" + bar + "╝")
	div := sideStyle.Render("╠" + bar + "╣")
	leftPad := strings.Repeat(" ", pad)
	rightPad := strings.Repeat(" ", pad)

	wrapRow := func(row string) string {
		rw := lipgloss.Width(row)
		fill := contentW - rw
		if fill < 0 {
			row = truncate(row, contentW)
			fill = 0
		}
		return side + leftPad + row + strings.Repeat(" ", fill) + rightPad + side
	}

	var lines []string
	lines = append(lines, top)
	if title != "" {
		t := lipgloss.PlaceHorizontal(contentW, lipgloss.Center,
			titleStyle.Render(strings.ToUpper(truncate(title, contentW))))
		lines = append(lines, wrapRow(t), div)
	}
	for _, row := range strings.Split(body, "\n") {
		lines = append(lines, wrapRow(row))
	}
	lines = append(lines, bot)
	return strings.Join(lines, "\n")
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

// BracketTag renders "[label]" with optional right-padding so the closing
// bracket sits at column (1 + maxLabelWidth + 1). Used to keep [main]/[dev]
// columns aligned in tabular layouts (e.g. the ReSukiSU branch table).
// Most callers pass maxLabelWidth = len(label) (or 0/1) so brackets hug
// their label, matching the bash original (`[B]` not `[B  ]`).
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
	return prefix + DotLeader(pad, leaderStyle) + rhsStyle.Render(rhs)
}

// DotLeader returns ` ··· ` (with one space on each side) of exactly
// `width` columns, styled with `style`. Used by MenuRow and any
// caller that wants the same dot-fill between label and right value.
//
// Safe for narrow terminals: falls back to plain spaces (or empty)
// when width is too small to fit ` · ` so it never panics on a
// negative `strings.Repeat` count.
func DotLeader(width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if width < 3 {
		return strings.Repeat(" ", width)
	}
	return style.Render(" " + strings.Repeat("·", width-2) + " ")
}

// SafeRepeat is strings.Repeat clamped to non-negative counts. Returns
// "" for n <= 0 instead of panicking like the stdlib does.
func SafeRepeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
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

// HotkeyStripWrap is HotkeyStrip with a maxWidth (0 = no wrap). Brackets
// are NOT cross-aligned to the longest key in the strip — each tag hugs
// its own label so a strip with `[L]` and `[23]` doesn't render as
// `[L ] | [23]`.
func HotkeyStripWrap(items []Hotkey, maxWidth int, keyStyle, descStyle, subStyle, sepStyle lipgloss.Style) string {
	rendered := make([]string, 0, len(items))
	for _, h := range items {
		row := BracketTag(h.Key, len(h.Key), keyStyle) + " " + descStyle.Render(h.Desc)
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
