package components

import (
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Activity is a thread-safe ring buffer of log lines for live operation
// feedback (download progress, guard fixer, install / extract). Screens own
// one Activity instance and append to it from background tea.Cmd goroutines
// or from Update() handlers; the View() side calls Render() to draw.
//
// Each line has a level (info/ok/warn/err) which controls its color. Lines
// are rendered with a subtle timestamp prefix. The buffer holds up to MaxLines
// lines; older lines are dropped silently.
type Activity struct {
	mu       sync.Mutex
	lines    []activityLine
	MaxLines int
}

// activityLine is one entry in the buffer.
type activityLine struct {
	t     time.Time
	level activityLevel
	text  string
}

// activityLevel selects the color a line is rendered in.
type activityLevel int

const (
	ActInfo activityLevel = iota
	ActOK
	ActWarn
	ActErr
	ActProgress // grey, used for spammy progress lines (overwrites previous progress)
)

// NewActivity returns an empty buffer with sensible defaults.
func NewActivity() *Activity {
	return &Activity{MaxLines: 200}
}

// Append adds a line at the given level.
func (a *Activity) Append(level activityLevel, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	max := a.MaxLines
	if max <= 0 {
		max = 200
	}
	// If the new line is a progress update and the last line is also progress,
	// replace it in place so the panel doesn't fill up with spam.
	if level == ActProgress && len(a.lines) > 0 && a.lines[len(a.lines)-1].level == ActProgress {
		a.lines[len(a.lines)-1] = activityLine{t: time.Now(), level: level, text: text}
		return
	}
	a.lines = append(a.lines, activityLine{t: time.Now(), level: level, text: text})
	if len(a.lines) > max {
		a.lines = a.lines[len(a.lines)-max:]
	}
}

// Info / OK / Warn / Err / Progress are convenience wrappers over Append.
func (a *Activity) Info(text string)     { a.Append(ActInfo, text) }
func (a *Activity) OK(text string)       { a.Append(ActOK, text) }
func (a *Activity) Warn(text string)     { a.Append(ActWarn, text) }
func (a *Activity) Err(text string)      { a.Append(ActErr, text) }
func (a *Activity) Progress(text string) { a.Append(ActProgress, text) }

// Clear empties the buffer.
func (a *Activity) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lines = nil
}

// Lines returns a snapshot of the current buffer (most recent N lines, or
// all when n <= 0).
func (a *Activity) Lines(n int) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.lines
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	rendered := make([]string, 0, len(out))
	for _, ln := range out {
		rendered = append(rendered, renderActivityLine(ln))
	}
	return rendered
}

// IsEmpty reports whether the buffer holds no lines.
func (a *Activity) IsEmpty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.lines) == 0
}

// Render returns the buffer as a single panel-ready string. visible is the
// number of recent lines to show; the rest are dropped from the rendered output
// (but stay in the buffer).
func (a *Activity) Render(visible int) string {
	lines := a.Lines(visible)
	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true).
			Render("(no activity yet)")
	}
	return strings.Join(lines, "\n")
}

func renderActivityLine(ln activityLine) string {
	ts := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(ln.t.Format("15:04:05") + "  ")
	var prefix string
	var color lipgloss.Color
	switch ln.level {
	case ActOK:
		prefix = "✓"
		color = lipgloss.Color("46")
	case ActWarn:
		prefix = "!"
		color = lipgloss.Color("214")
	case ActErr:
		prefix = "✗"
		color = lipgloss.Color("196")
	case ActProgress:
		prefix = "·"
		color = lipgloss.Color("245")
	default:
		prefix = "›"
		color = lipgloss.Color("87")
	}
	pre := lipgloss.NewStyle().Foreground(color).Bold(ln.level == ActOK || ln.level == ActErr).Render(prefix + " ")
	body := lipgloss.NewStyle().Foreground(color).Render(ln.text)
	return ts + pre + body
}
