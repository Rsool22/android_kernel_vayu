// Centralized key bindings for the TUI. Every screen imports Keys and
// reuses these bindings so both the active-key detection and the help
// text stay in one place (see S3 help bar + S6 central registry in
// devin_prompt_buildsh_overhaul.md).
package components

import "github.com/charmbracelet/bubbles/key"

// KeyMap groups every hotkey the TUI uses. New bindings added here are
// automatically available to every screen and picked up by the help bar
// component (Help view derived from this map).
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	Enter  key.Binding
	Back   key.Binding
	Quit   key.Binding
	Help   key.Binding
	Filter key.Binding
	Refresh key.Binding
}

// Keys is the package-wide key map every screen consumes. Initialized
// at import time; never mutated at runtime so concurrent reads are safe.
var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("\u2191/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("\u2193/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("\u2190/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("\u2192/l", "right"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "ctrl+u"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "ctrl+d"),
		key.WithHelp("pgdn", "page down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("home/g", "top"),
	),
	End: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("end/G", "bottom"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}

// ShortHelp returns the small subset of bindings shown in the one-line
// help strip at the bottom of every screen. Used via bubbles/help.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Back, k.Quit}
}

// FullHelp returns all bindings grouped for the expanded help view
// (triggered by `?`). bubbles/help expects a slice of binding slices so
// each inner slice becomes a column in the expanded help.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.PageUp, k.PageDown, k.Home, k.End},
		{k.Enter, k.Back, k.Quit, k.Help},
		{k.Filter, k.Refresh},
	}
}
