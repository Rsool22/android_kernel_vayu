// ActivityLog -- a small, append-only log buffer rendered as a panel
// at the bottom of every screen (prompt item #8). Keeps the last N
// entries (ring-buffer style), prefixing each with a timestamp and a
// severity glyph so long-running background work always has a visible
// status trail.
//
// Central (App-level) instance so all screens share the same history:
// pressing [T] from the main menu, then [F] to fetch, then [esc] back
// to main still shows the fetch progress line in the log. Screens
// paint ActivityPanel(log, width) wherever they want the log to show.
package components

import (
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ActivityLevel classifies log lines. Driver for the glyph + colour
// picked by ActivityPanel (matches Notice() semantics).
type ActivityLevel int

const (
	ActInfo ActivityLevel = iota
	ActOK
	ActWarn
	ActErr
)

// ActivityEntry is a single line in the log.
type ActivityEntry struct {
	When  time.Time
	Level ActivityLevel
	Msg   string
}

// ActivityLog is the append-only buffer. Safe for concurrent writes
// from background goroutines (tea.Cmd) + reads from the render thread.
type ActivityLog struct {
	mu      sync.Mutex
	entries []ActivityEntry
	max     int
}

// NewActivityLog creates a ring-buffered log that keeps the last `max`
// entries (older ones are dropped as new ones are appended).
func NewActivityLog(max int) *ActivityLog {
	if max <= 0 {
		max = 64
	}
	return &ActivityLog{max: max}
}

// Append drops a new line into the log. Safe to call from any goroutine.
func (l *ActivityLog) Append(level ActivityLevel, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, ActivityEntry{When: time.Now(), Level: level, Msg: msg})
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
}

// Info / Ok / Warn / Err are shortcut writers for the 4 severity levels.
func (l *ActivityLog) Info(msg string) { l.Append(ActInfo, msg) }
func (l *ActivityLog) Ok(msg string)   { l.Append(ActOK, msg) }
func (l *ActivityLog) Warn(msg string) { l.Append(ActWarn, msg) }
func (l *ActivityLog) Err(msg string)  { l.Append(ActErr, msg) }

// Snapshot returns a copy of the current entries, newest last. The
// returned slice is safe to mutate / iterate without holding the lock.
func (l *ActivityLog) Snapshot() []ActivityEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]ActivityEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Clear wipes the log -- useful when starting a new build or switching
// screens that shouldn't share context.
func (l *ActivityLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

// ActivityPanel renders the last `maxRows` entries of the log inside a
// Panel box of outer width `width`. The panel is titled "Activity" and
// uses the dim border variant so it doesn't visually compete with the
// primary content boxes above it.
func ActivityPanel(log *ActivityLog, width, maxRows int, border lipgloss.Style, titleStyle lipgloss.Style) string {
	if log == nil {
		return ""
	}
	inner := InnerWidth(width)
	all := log.Snapshot()
	if maxRows > 0 && len(all) > maxRows {
		all = all[len(all)-maxRows:]
	}
	if len(all) == 0 {
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true).
			Render("(no activity yet)")
		return Panel("Activity", empty, width, border, titleStyle)
	}
	var b strings.Builder
	for i, e := range all {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderActivityLine(e, inner))
	}
	return Panel("Activity", b.String(), width, border, titleStyle)
}

// renderActivityLine formats one entry: `[12:04:55] [i]  message`.
// Truncates long messages with an ellipsis so the right border never
// wraps. inner = the panel's content-area width.
func renderActivityLine(e ActivityEntry, inner int) string {
	var glyph, colour string
	switch e.Level {
	case ActOK:
		glyph, colour = "[\u2713]", "46"
	case ActWarn:
		glyph, colour = "[!]", "214"
	case ActErr:
		glyph, colour = "[\u00d7]", "196"
	default:
		glyph, colour = "[i]", "51"
	}
	ts := e.When.Format("15:04:05")
	tsStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(ts)
	glyphStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Bold(true).Render(glyph)
	prefix := tsStyled + "  " + glyphStyled + "  "
	pw := lipgloss.Width(prefix)
	budget := inner - pw
	if budget < 8 {
		budget = 8
	}
	msg := e.Msg
	if lipgloss.Width(msg) > budget {
		// Truncate preserving the tail (most informative part of a path /
		// error). Leading ellipsis marks the truncation.
		msg = "\u2026" + msg[len(msg)-budget+1:]
	}
	return prefix + msg
}
