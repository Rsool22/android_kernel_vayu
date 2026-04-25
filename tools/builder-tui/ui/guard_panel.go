package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// renderGuardPanel turns a parsed pipeline.GuardReport into a bash-styled
// panel with three vertically-stacked sections: APPLIED (FIX, orange),
// SKIPPED (SKP, grey) and VERIFIED OK (OK, green). Each row uses a fixed
// label column so file paths line up across the three sections.
//
// The bash original used coloured box rows; we keep the same hierarchy
// but render with table-like alignment so very long file paths/details
// don't break panel borders.
func renderGuardPanel(g pipeline.GuardReport, w int) string {
	if g.Total() == 0 && g.Summary == "" {
		return ""
	}
	border := PanelOK
	title := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	if g.HasFixes() {
		border = PanelWarn
		title = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)
	}

	inner := innerContentWidth(w)
	fileColW := guardFileColumnWidth(g, inner/2)
	if fileColW < 12 {
		fileColW = 12
	}

	var b strings.Builder
	if len(g.Fixed) > 0 {
		b.WriteString(WarnText.Bold(true).Render("APPLIED") + "\n")
		b.WriteString(components.Separator(inner, MutedText) + "\n")
		for _, e := range g.Fixed {
			b.WriteString(guardRow(e, fileColW, WarnText, ValueStyle))
			b.WriteString("\n")
		}
	}
	if len(g.Skipped) > 0 {
		if b.Len() > 0 {
			b.WriteString(components.Separator(inner, MutedText) + "\n")
		}
		b.WriteString(MutedText.Bold(true).Render("SKIPPED") + "\n")
		b.WriteString(components.Separator(inner, MutedText) + "\n")
		for _, e := range g.Skipped {
			detail := e.File
			if detail == "" {
				detail = e.Detail
			}
			b.WriteString("  " + DimText.Render(truncate(detail, inner-4)) + "\n")
		}
	}
	if len(g.OK) > 0 {
		if b.Len() > 0 {
			b.WriteString(components.Separator(inner, MutedText) + "\n")
		}
		b.WriteString(OKText.Bold(true).Render("VERIFIED  OK") + "\n")
		b.WriteString(components.Separator(inner, MutedText) + "\n")
		for _, e := range g.OK {
			b.WriteString(guardRow(e, fileColW, DimText, MutedText))
			b.WriteString("\n")
		}
	}
	if g.Summary != "" {
		b.WriteString(components.Separator(inner, MutedText) + "\n")
		st := OKText
		if g.HasFixes() {
			st = WarnText
		}
		b.WriteString(lipgloss.PlaceHorizontal(inner, lipgloss.Center,
			st.Render("Summary : "+g.Summary)) + "\n")
	}

	return components.Panel("HOOK  GUARD  VERIFICATION", b.String(), w, border, title)
}

// guardRow renders one guard row as `<file>   <detail>`, padding the file
// column to fileColW.
func guardRow(e pipeline.GuardEntry, fileColW int, fileSt, detailSt lipgloss.Style) string {
	file := truncate(e.File, fileColW)
	pad := fileColW - lipgloss.Width(file)
	if pad < 0 {
		pad = 0
	}
	prefix := "  " + fileSt.Render(file) + strings.Repeat(" ", pad) + "  "
	return prefix + detailSt.Render(truncate(e.Detail, 80))
}

// guardFileColumnWidth returns the longest file column across all entries
// (clipped to maxW) so values line up.
func guardFileColumnWidth(g pipeline.GuardReport, maxW int) int {
	w := 0
	for _, e := range append(append([]pipeline.GuardEntry{}, g.Fixed...), g.OK...) {
		if lw := lipgloss.Width(e.File); lw > w {
			w = lw
		}
	}
	if w > maxW {
		w = maxW
	}
	return w
}
