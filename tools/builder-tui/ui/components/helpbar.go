// Global keybinding help bar (bubbles/help, prompt item S3).
//
// Every screen appends HelpBar(width, showAll) to its View() so the same
// key hints show at the bottom of every page. Short view is the default;
// expanded view is toggled by `?` (bubbles/help handles the state).
package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel is the persistent bubbles/help instance used by the root
// App to render the global status bar. Kept as a mutable package-level
// value so the root model can toggle ShowAll via App.Update.
var HelpModel = help.New()

// HelpBar renders the one-line help strip for the current screen at the
// supplied width. Caller passes the active Keys.ShortHelp / FullHelp
// via the bubbles/help instance; the returned string is padded to
// `width` so the strip fills the gutter consistently.
//
// leftSuffix is an optional prefix drawn at the start of the strip
// (e.g. a breadcrumb path) before the key hints. Pass "" to omit it.
func HelpBar(leftSuffix string, width int, sepStyle lipgloss.Style) string {
	HelpModel.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	HelpModel.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	HelpModel.Styles.ShortSeparator = sepStyle
	HelpModel.Styles.FullKey = HelpModel.Styles.ShortKey
	HelpModel.Styles.FullDesc = HelpModel.Styles.ShortDesc
	HelpModel.Styles.FullSeparator = sepStyle
	hints := HelpModel.View(Keys)
	if leftSuffix != "" {
		hints = leftSuffix + "   " + hints
	}
	// Right-pad so the strip always spans the full page width (lines up
	// with box edges above it).
	pad := width - lipgloss.Width(hints)
	if pad > 0 {
		hints += strings.Repeat(" ", pad)
	}
	return hints
}
