// Layout constants and helpers shared by every screen so boxes render at
// the same uniform width and same inner padding regardless of which screen
// the user is on.
//
// Usage from a screen's View():
//
//	w := components.PageWidth(app.Width)           // one value, every box on this page
//	inner := components.InnerWidth(w)              // safe content width inside any box
//	banner := components.Banner(..., w, ...)
//	panel := components.Panel("Title", body, w, ...)
//
// Never hardcode widths; never compute width minus constants inline. Go
// through PageWidth + InnerWidth.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// BoxPadding is the lipgloss horizontal padding applied inside every
	// box (both borders get this same padding, so the right gutter looks
	// exactly like the left gutter). Keep in sync with the Padding(0, 1)
	// baked into BannerBorder / PanelBorder in theme.go.
	BoxPadding = 1

	// BoxBorderWidth counts the left + right border characters drawn by
	// lipgloss for a bordered box.
	BoxBorderWidth = 2

	// BoxChrome is the total horizontal overhead of a box: two border
	// chars + BoxPadding on each side. Used for InnerWidth calculations.
	BoxChrome = BoxBorderWidth + 2*BoxPadding

	// PageMargin is the blank gutter left between the outer-most box and
	// the terminal's right edge, so double-line borders never abut the
	// terminal chrome (looks much cleaner on most emulators).
	PageMargin = 2

	// PageMinWidth is the absolute minimum width we clamp to. Narrower
	// than this and dotted leaders / key alignments stop looking right.
	PageMinWidth = 60

	// PageMaxWidth caps how wide we let boxes grow on ultra-wide
	// terminals -- keeps line lengths readable and banners centered.
	PageMaxWidth = 160

	// CursorCellWidth is the fixed column width reserved for the nav
	// cursor glyph. Must always render at this width whether the row is
	// selected or not, so columns downstream line up.
	CursorCellWidth = 2

	// BadgeRowGap is the number of blank lines to insert between stacked
	// badges to stop them from visually merging into a single bar.
	BadgeRowGap = 1
)

// PageWidth returns the outer width every box on a single screen should
// use. All boxes (banner, panels, activity, notices) on the same page
// MUST pass this same return value as their width parameter so they all
// render at the uniform box width the prompt calls for.
func PageWidth(termWidth int) int {
	if termWidth <= 0 {
		termWidth = 80
	}
	w := termWidth - PageMargin
	if w < PageMinWidth {
		w = PageMinWidth
	}
	if w > PageMaxWidth {
		w = PageMaxWidth
	}
	return w
}

// InnerWidth returns the content-area width inside a box of outer width
// outerW, i.e. after the borders and the lipgloss padding on each side
// have been subtracted. Use this to size dotted-leader rows, ruled
// dividers, lipgloss.PlaceHorizontal centering, etc.
func InnerWidth(outerW int) int {
	w := outerW - BoxChrome
	if w < 20 {
		w = 20
	}
	return w
}

// CursorCell renders the fixed-width navigation cursor cell. Selected
// rows get the chevron rendered in cursorStyle; unselected rows get an
// equal-width blank so the column downstream (bracket tag, label, ...)
// lines up byte-for-byte across both states. Always returns a string
// whose visible width equals CursorCellWidth.
func CursorCell(selected bool, cursorStyle lipgloss.Style) string {
	if selected {
		return cursorStyle.Render("\u25b8 ")
	}
	return strings.Repeat(" ", CursorCellWidth)
}

// StackBadges joins already-rendered badge strings with a single-blank
// gap between rows so the coloured backgrounds never visually touch.
// Use when you'd otherwise concatenate badge strings with "\n" and end
// up with a single green/red bar.
func StackBadges(rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	sep := strings.Repeat("\n", BadgeRowGap+1)
	return strings.Join(rows, sep)
}

// PageAlign records the shared column widths computed once per page so
// every row across every box on that page aligns to the same column
// offsets, not just rows within a single box. Build one in View() and
// pass it (or its fields) to every row renderer on the page.
//
//	align := components.NewPageAlign(innerW)
//	align.NoteKey("main", "Stable")
//	align.NoteKey("dev",  "Bleeding")
//	row1 := myRow("main", ..., align)
//	row2 := myRow("dev",  ..., align)
type PageAlign struct {
	// TotalInner is the minimum content-area width of any box on the
	// page (the narrowest box's InnerWidth). Rows should size their
	// dotted leaders to fill up to TotalInner so rhs values right-align
	// to the same column regardless of which box a row lives in.
	TotalInner int

	// KeyWidth is the width of the widest bracketed key (`[main]`,
	// `[ESC]`, ...) encountered on the page, used to right-pad every
	// [key] so closing `]` lines up page-wide.
	KeyWidth int

	// LabelWidth is the width of the widest row label on the page.
	LabelWidth int

	// RhsWidth is the width of the widest right-hand value on the page.
	RhsWidth int
}

// NewPageAlign starts a page-wide alignment tracker, seeded with the
// narrowest box's inner width. The KeyWidth field is initialized from
// the TUI's GlobalKeyWidth so even single-row boxes have a sane default.
func NewPageAlign(totalInner int) *PageAlign {
	return &PageAlign{TotalInner: totalInner, KeyWidth: GlobalKeyWidth}
}

// NoteKey updates KeyWidth/LabelWidth if the supplied values exceed
// the tracker's current maxima. Call once per row before rendering.
func (a *PageAlign) NoteKey(key, label string) {
	if kw := lipgloss.Width(key); kw > a.KeyWidth {
		a.KeyWidth = kw
	}
	if lw := lipgloss.Width(label); lw > a.LabelWidth {
		a.LabelWidth = lw
	}
}

// NoteRhs updates RhsWidth if rhs is wider than anything recorded so far.
func (a *PageAlign) NoteRhs(rhs string) {
	if rw := lipgloss.Width(rhs); rw > a.RhsWidth {
		a.RhsWidth = rw
	}
}
