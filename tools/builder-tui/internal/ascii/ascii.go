// Package ascii holds the verbatim block-art banners (SUCCESS, FAILED,
// CANCELLED) lifted from the original bash build.sh, plus helpers that
// render them with a per-line color gradient and an animated reveal.
//
// The art and color choices mirror bash: green ramp for success, red ramp
// for failure, amber ramp for cancellation. AnimateCmds returns a slice of
// tea.Cmd so the BuildScreen can schedule a tea.Tick per row for a one-shot
// fade-in effect.
package ascii

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Kind discriminates which ASCII banner to render.
type Kind int

const (
	Success Kind = iota
	Failed
	Cancelled
)

// success/failed/cancelled — verbatim from bash print_*_art().
var (
	successLines = []string{
		"░█▀▀░█░█░█▀▀░█▀▀░█▀▀░█▀▀░█▀▀░█░░",
		"░▀▀█░█░█░█░░░█░░░█▀▀░▀▀█░▀▀█░▀░░",
		"░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀░░",
	}
	failedLines = []string{
		"░█▀▀░█▀█░▀█▀░█░░░█▀▀░█▀▄░▀▀█",
		"░█▀▀░█▀█░░█░░█░░░█▀▀░█░█░░▀░",
		"░▀░░░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░░▀░",
	}
	cancelledLines = []string{
		"░█▀▀░▀█▀░█▀█░█▀█░█▀█░█▀▀░█░░░█░░░█▀▀░█▀▄",
		"░█░░░░█░░█▀█░█░█░█░░░█▀▀░█░░░█░░░█▀▀░█░█",
		"░▀▀▀░▀▀▀░▀░▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░",
	}

	// Per-kind 256-color ramps (3 shades). Picked to feel close to the
	// bash colours (LGR/LRD/YEL) while giving each line a small gradient
	// so the rows don't look flat.
	successRamp   = []lipgloss.Color{"34", "40", "46"}
	failedRamp    = []lipgloss.Color{"124", "160", "196"}
	cancelledRamp = []lipgloss.Color{"172", "208", "214"}
)

// String returns the kind name (used as a panel title).
func (k Kind) String() string {
	switch k {
	case Success:
		return "Success"
	case Failed:
		return "Failed"
	case Cancelled:
		return "Cancelled"
	}
	return "?"
}

// Lines returns the verbatim raw rows for the given kind.
func Lines(k Kind) []string {
	switch k {
	case Success:
		return append([]string(nil), successLines...)
	case Failed:
		return append([]string(nil), failedLines...)
	case Cancelled:
		return append([]string(nil), cancelledLines...)
	}
	return nil
}

// Ramp returns the color ramp for the kind.
func Ramp(k Kind) []lipgloss.Color {
	switch k {
	case Success:
		return successRamp
	case Failed:
		return failedRamp
	case Cancelled:
		return cancelledRamp
	}
	return successRamp
}

// Render returns the fully-coloured ASCII block for kind k. revealed >=
// len(lines) renders the full block; lower values render only the first
// `revealed` rows (the rest are blank-padded so layout doesn't jump). A
// soft `glow` prefix/suffix is added per line to mimic lipgloss-side glow.
func Render(k Kind, revealed int) string {
	lines := Lines(k)
	ramp := Ramp(k)
	if revealed < 0 {
		revealed = 0
	}
	if revealed > len(lines) {
		revealed = len(lines)
	}
	var b strings.Builder
	for i, l := range lines {
		if i >= revealed {
			b.WriteString("  " + strings.Repeat(" ", lipglossWidth(l)) + "\n")
			continue
		}
		c := ramp[i%len(ramp)]
		st := lipgloss.NewStyle().Foreground(c).Bold(true)
		b.WriteString("  " + st.Render(l) + "\n")
	}
	return b.String()
}

// AnimateCmds returns one tea.Cmd per line, each producing a RevealMsg
// (carrying the row index) after a small delay. Use tea.Sequence to chain
// them for a top-to-bottom fade-in. The delay between rows is fixed at
// 80ms for a snappy but visible reveal.
func AnimateCmds(k Kind) []tea.Cmd {
	n := len(Lines(k))
	cmds := make([]tea.Cmd, 0, n)
	for i := 0; i < n; i++ {
		idx := i
		cmds = append(cmds, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
			return RevealMsg{Kind: k, Row: idx + 1}
		}))
	}
	return cmds
}

// RevealMsg is sent by the AnimateCmds tickers; the build screen advances
// its `revealed` counter when it receives one and re-renders.
type RevealMsg struct {
	Kind Kind
	Row  int
}

// lipglossWidth is a thin wrapper to avoid an import cycle. Returns 0 for
// the empty string and the rune count otherwise (we only render plain
// box-drawing chars so a rune count is exact for our purposes).
func lipglossWidth(s string) int { return lipgloss.Width(s) }
