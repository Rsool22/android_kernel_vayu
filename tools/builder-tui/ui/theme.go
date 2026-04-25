// Package ui implements the Bubble Tea TUI for the vayu kernel builder.
//
// Visual conventions (mirrored from the original bash build.sh aesthetic):
//   - Magenta double-line frames around the title banner.
//   - Cyan double-line frames around panels.
//   - Yellow centered titles inside panels.
//   - Bright white for values/emphasis, dim grey for labels and rules,
//     green/yellow/red for ok/warn/err status.
//   - Boxes auto-resize to terminal width with a sane min/max.
//
// All styling lives in this file or in components/. Models never write
// raw ANSI -- they compose styles by name.
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Width is the active draw width; resized live by App on tea.WindowSizeMsg.
var Width = 80

// StyleVariant selects the visual mode. "bash" (default) replicates the
// chunky double-line bordered look of the original build.sh; "modern" uses
// a softer rounded-corner aesthetic with a subdued palette; "mono" is a
// minimal single-colour palette suitable for low-colour terminals.
//
// Set via the --style flag in cmd/root.go before the App starts rendering,
// or live via the Settings screen.
type StyleVariant int

const (
	StyleBash StyleVariant = iota
	StyleModern
	StyleMono
)

// String returns the lowercase name used in the persisted config file.
func (v StyleVariant) String() string {
	switch v {
	case StyleModern:
		return "modern"
	case StyleMono:
		return "mono"
	default:
		return "bash"
	}
}

// ParseStyleVariant returns the StyleVariant for a config string, with
// "bash" as the safe default for unknown values.
func ParseStyleVariant(s string) StyleVariant {
	switch s {
	case "modern":
		return StyleModern
	case "mono":
		return StyleMono
	default:
		return StyleBash
	}
}

// CurrentStyle is the active variant. Changed by ApplyStyle.
var CurrentStyle StyleVariant = StyleBash

// AccentOverride is a non-empty 256-colour or hex string that takes
// precedence over the theme's default accent / hotkey colours when set.
// Empty means "use the theme default". Updated via ApplyAccentOverride.
var AccentOverride = ""

// ApplyStyle swaps the package-level styles to match v. Safe to call
// multiple times -- always restores from the canonical bash palette
// before applying the requested variant so previous overrides don't leak.
func ApplyStyle(v StyleVariant) {
	CurrentStyle = v
	resetBashPalette()
	switch v {
	case StyleModern:
		ColorBanner = lipgloss.Color("99")  // soft purple
		ColorPanel = lipgloss.Color("75")   // periwinkle
		ColorTitle = lipgloss.Color("117")  // sky
		ColorAccent = lipgloss.Color("141") // lavender
		ColorHotKey = lipgloss.Color("215") // peach
		BannerBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorBanner).
			Padding(0, 1)
		PanelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorPanel).
			Padding(0, 1)
	case StyleMono:
		mono := lipgloss.Color("250")
		ColorBanner = mono
		ColorPanel = mono
		ColorTitle = mono
		ColorAccent = mono
		ColorHotKey = lipgloss.Color("231")
		ColorBuild = mono
		ColorSel = lipgloss.Color("231")
		BannerBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(mono).
			Padding(0, 1)
		PanelBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(mono).
			Padding(0, 1)
	}
	if AccentOverride != "" {
		applyAccentColor(AccentOverride)
	}
	rebuildDerivedStyles()
}

// ApplyAccentOverride records and applies a user-chosen accent colour.
// Pass "" to clear the override and revert to the theme default.
func ApplyAccentOverride(color string) {
	AccentOverride = color
	ApplyStyle(CurrentStyle)
}

func applyAccentColor(c string) {
	col := lipgloss.Color(c)
	ColorAccent = col
	ColorHotKey = col
	ColorTitle = col
}

// resetBashPalette restores the canonical bash colour palette.
func resetBashPalette() {
	ColorBanner = lipgloss.Color("201")
	ColorPanel = lipgloss.Color("51")
	ColorTitle = lipgloss.Color("220")
	ColorValue = lipgloss.Color("231")
	ColorLabel = lipgloss.Color("250")
	ColorOK = lipgloss.Color("46")
	ColorWarn = lipgloss.Color("214")
	ColorErr = lipgloss.Color("196")
	ColorDim = lipgloss.Color("245")
	ColorMuted = lipgloss.Color("240")
	ColorAccent = lipgloss.Color("87")
	ColorSel = lipgloss.Color("213")
	ColorHotKey = lipgloss.Color("214")
	ColorBuild = lipgloss.Color("117")
	BannerBorder = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder(), true).
		BorderForeground(ColorBanner).
		Padding(0, 1)
	PanelBorder = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder(), true).
		BorderForeground(ColorPanel).
		Padding(0, 1)
}

// rebuildDerivedStyles refreshes every style derived from the colour
// palette so a live theme change takes effect on the next render.
func rebuildDerivedStyles() {
	PanelOK = PanelBorder.BorderForeground(ColorOK)
	PanelWarn = PanelBorder.BorderForeground(ColorWarn)
	PanelErr = PanelBorder.BorderForeground(ColorErr)
	PanelDim = PanelBorder.BorderForeground(ColorMuted)
	TitleStyle = lipgloss.NewStyle().Foreground(ColorTitle).Bold(true)
	BannerTitle = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	BannerSubtle = lipgloss.NewStyle().Foreground(ColorAccent).Italic(true)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	HelpStyle = lipgloss.NewStyle().Foreground(ColorDim).Italic(true)
	HotKeyStyle = lipgloss.NewStyle().Foreground(ColorHotKey).Bold(true)
	LabelStyle = lipgloss.NewStyle().Foreground(ColorLabel)
	ValueStyle = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	AccentText = lipgloss.NewStyle().Foreground(ColorAccent)
	OKText = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	WarnText = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)
	ErrText = lipgloss.NewStyle().Foreground(ColorErr).Bold(true)
	DimText = lipgloss.NewStyle().Foreground(ColorDim)
	MutedText = lipgloss.NewStyle().Foreground(ColorMuted)
	SelText = lipgloss.NewStyle().Foreground(ColorSel).Bold(true)
	BadgeOK = lipgloss.NewStyle().
		Foreground(lipgloss.Color("16")).
		Background(ColorOK).
		Bold(true).
		Padding(0, 1)
	BadgeWarn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("16")).
		Background(ColorWarn).
		Bold(true).
		Padding(0, 1)
	BadgeErr = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(ColorErr).
		Bold(true).
		Padding(0, 1)
	BadgeAccent = lipgloss.NewStyle().
		Foreground(lipgloss.Color("16")).
		Background(ColorPanel).
		Bold(true).
		Padding(0, 1)
}

// Palette adapted from the bash TUI:
//
//	MAG = bold magenta (banner)
//	CYN = bold cyan (panels)
//	YEL = bold yellow (titles, warn)
//	WHT = bold white (values)
//	LGR = bold green (success)
//	LRD = bold red (error)
//	GRY = grey (labels)
//	DIM = dim/muted (rules, secondary)
//	ORG = orange (highlight)
var (
	ColorBanner = lipgloss.Color("201") // bright magenta
	ColorPanel  = lipgloss.Color("51")  // bright cyan
	ColorTitle  = lipgloss.Color("220") // bright yellow
	ColorValue  = lipgloss.Color("231") // bright white
	ColorLabel  = lipgloss.Color("250") // soft grey
	ColorOK     = lipgloss.Color("46")  // bright green
	ColorWarn   = lipgloss.Color("214") // amber
	ColorErr    = lipgloss.Color("196") // bright red
	ColorDim    = lipgloss.Color("245")
	ColorMuted  = lipgloss.Color("240")
	ColorAccent = lipgloss.Color("87")  // soft cyan (subtitles, accent text)
	ColorSel    = lipgloss.Color("213") // pink-ish for selected list rows
	ColorHotKey = lipgloss.Color("214") // orange hotkey letters
	ColorBuild  = lipgloss.Color("117") // sky blue for build viewport border
)

var (
	// Border styles -- DoubleBorder mirrors the ╔═╗ chars used in bash build.sh.
	// BannerBorder uses the same padding as PanelBorder so the inner content
	// columns line up across the banner and the panels stacked below it.
	BannerBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder(), true).
			BorderForeground(ColorBanner).
			Padding(0, 1)

	PanelBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder(), true).
			BorderForeground(ColorPanel).
			Padding(0, 1)

	PanelOK   = PanelBorder.BorderForeground(ColorOK)
	PanelWarn = PanelBorder.BorderForeground(ColorWarn)
	PanelErr  = PanelBorder.BorderForeground(ColorErr)
	PanelDim  = PanelBorder.BorderForeground(ColorMuted)

	// Inline text roles.
	TitleStyle    = lipgloss.NewStyle().Foreground(ColorTitle).Bold(true)
	BannerTitle   = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	BannerSubtle  = lipgloss.NewStyle().Foreground(ColorAccent).Italic(true)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	HelpStyle     = lipgloss.NewStyle().Foreground(ColorDim).Italic(true)

	HotKeyStyle = lipgloss.NewStyle().Foreground(ColorHotKey).Bold(true)
	LabelStyle  = lipgloss.NewStyle().Foreground(ColorLabel)
	ValueStyle  = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	AccentText  = lipgloss.NewStyle().Foreground(ColorAccent)
	OKText      = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	WarnText    = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)
	ErrText     = lipgloss.NewStyle().Foreground(ColorErr).Bold(true)
	DimText     = lipgloss.NewStyle().Foreground(ColorDim)
	MutedText   = lipgloss.NewStyle().Foreground(ColorMuted)
	SelText     = lipgloss.NewStyle().Foreground(ColorSel).Bold(true)

	// Badge styles -- inline pill/tag looks for status words.
	BadgeOK = lipgloss.NewStyle().
		Foreground(lipgloss.Color("16")).
		Background(ColorOK).
		Bold(true).
		Padding(0, 1)
	BadgeWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("16")).
			Background(ColorWarn).
			Bold(true).
			Padding(0, 1)
	BadgeErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("231")).
			Background(ColorErr).
			Bold(true).
			Padding(0, 1)
	BadgeAccent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("16")).
			Background(ColorPanel).
			Bold(true).
			Padding(0, 1)
)
