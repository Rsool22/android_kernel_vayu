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
// a softer rounded-corner aesthetic with a subdued palette; "ocean" /
// "forest" / "mono" are alternative palettes accessible through the theme
// picker on the Setup screen ([C] = Customize Theme). Set via the --style
// flag in cmd/root.go *or* the persisted Cfg.Theme before App starts.
type StyleVariant int

const (
	StyleBash StyleVariant = iota
	StyleModern
	StyleOcean
	StyleForest
	StyleMono
)

// StyleNames is the ordered set of preset theme names exposed in the
// theme picker. Index matches the StyleVariant values above.
var StyleNames = []string{"bash", "modern", "ocean", "forest", "mono"}

// ParseStyle maps a name back to a StyleVariant; returns StyleBash for
// unknown names so a stale config never breaks the UI.
func ParseStyle(name string) StyleVariant {
	for i, n := range StyleNames {
		if n == name {
			return StyleVariant(i)
		}
	}
	return StyleBash
}

// CurrentStyle is the active variant. Changed at most once during init.
var CurrentStyle StyleVariant = StyleBash

// ApplyStyle swaps the package-level styles to match v. Call once before
// tea.NewProgram.Run() — and again from the Setup theme picker when the
// user switches themes at runtime.
func ApplyStyle(v StyleVariant) {
	CurrentStyle = v
	// Each branch sets the palette colours; the border + style derivations
	// at the bottom are shared so we only have one place where the actual
	// lipgloss.Style values are constructed (consistency across themes).
	border := lipgloss.DoubleBorder()
	switch v {
	case StyleModern:
		ColorBanner = lipgloss.Color("99")
		ColorPanel = lipgloss.Color("75")
		ColorTitle = lipgloss.Color("117")
		ColorValue = lipgloss.Color("231")
		ColorAccent = lipgloss.Color("141")
		ColorHotKey = lipgloss.Color("215")
		ColorBuild = lipgloss.Color("117")
		border = lipgloss.RoundedBorder()
	case StyleOcean:
		ColorBanner = lipgloss.Color("39")  // azure
		ColorPanel = lipgloss.Color("45")   // teal
		ColorTitle = lipgloss.Color("159")  // pale cyan
		ColorValue = lipgloss.Color("231")
		ColorAccent = lipgloss.Color("87")  // soft cyan
		ColorHotKey = lipgloss.Color("226") // lemon
		ColorBuild = lipgloss.Color("39")
	case StyleForest:
		ColorBanner = lipgloss.Color("28")  // pine
		ColorPanel = lipgloss.Color("34")   // grass
		ColorTitle = lipgloss.Color("190")  // chartreuse
		ColorValue = lipgloss.Color("231")
		ColorAccent = lipgloss.Color("154") // lime
		ColorHotKey = lipgloss.Color("214") // amber
		ColorBuild = lipgloss.Color("34")
	case StyleMono:
		ColorBanner = lipgloss.Color("250")
		ColorPanel = lipgloss.Color("245")
		ColorTitle = lipgloss.Color("255")
		ColorValue = lipgloss.Color("255")
		ColorAccent = lipgloss.Color("250")
		ColorHotKey = lipgloss.Color("231")
		ColorBuild = lipgloss.Color("245")
	default: // StyleBash
		ColorBanner = lipgloss.Color("201")
		ColorPanel = lipgloss.Color("51")
		ColorTitle = lipgloss.Color("220")
		ColorValue = lipgloss.Color("231")
		ColorAccent = lipgloss.Color("87")
		ColorHotKey = lipgloss.Color("214")
		ColorBuild = lipgloss.Color("117")
	}

	BannerBorder = lipgloss.NewStyle().Border(border, true).
		BorderForeground(ColorBanner).Padding(0, 1)
	PanelBorder = lipgloss.NewStyle().Border(border, true).
		BorderForeground(ColorPanel).Padding(0, 1)
	PanelOK = PanelBorder.BorderForeground(ColorOK)
	PanelWarn = PanelBorder.BorderForeground(ColorWarn)
	PanelErr = PanelBorder.BorderForeground(ColorErr)
	PanelDim = PanelBorder.BorderForeground(ColorMuted)
	TitleStyle = lipgloss.NewStyle().Foreground(ColorTitle).Bold(true)
	BannerTitle = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	BannerSubtle = lipgloss.NewStyle().Foreground(ColorAccent).Italic(true)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	HotKeyStyle = lipgloss.NewStyle().Foreground(ColorHotKey).Bold(true)
	AccentText = lipgloss.NewStyle().Foreground(ColorAccent)
	ValueStyle = lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
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
