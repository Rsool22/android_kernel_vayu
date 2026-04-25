package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// pathField identifies which of the 6 path slots is being edited.
type pathField int

const (
	fieldKernel pathField = iota + 1
	fieldClang
	fieldAnyKernel
	fieldOutput
	fieldGcc64
	fieldGcc32
)

// PathsScreen renders the resolved-paths panel + distro install hints
// and exposes inline path editing. This is the strict 1:1 port of the
// bash `do_paths_config` / `_draw_paths_menu`. Reachable only from the
// Setup wrapper menu (`S → P`).
//
// Rows are navigable by hotkey (1-6) OR ↑/↓ + Enter. Long values wrap to a
// continuation line indented under the value column so paths never overflow
// off-screen.
type PathsScreen struct {
	app     *App
	editing pathField
	input   textinput.Model
	cursor  int // 0..5, indexes into pathField list for arrow-nav
}

func NewPathsScreen(a *App) PathsScreen {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 4096
	ti.Width = 60
	ti.Prompt = "▶ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorValue)
	return PathsScreen{app: a, input: ti}
}

func (s PathsScreen) Init() tea.Cmd { return nil }

// fields returns all path fields in display order.
func (s PathsScreen) fields() []pathField {
	return []pathField{fieldKernel, fieldClang, fieldAnyKernel, fieldOutput, fieldGcc64, fieldGcc32}
}

// fieldValue returns the current persisted value for a given field.
func (s PathsScreen) fieldValue(f pathField) string {
	switch f {
	case fieldKernel:
		return s.app.Cfg.KernelDir
	case fieldClang:
		return s.app.Cfg.ClangDir
	case fieldAnyKernel:
		return s.app.Cfg.AnyKernelDir
	case fieldOutput:
		return s.app.Cfg.OutputDir
	case fieldGcc64:
		return s.app.Cfg.GCC64Dir
	case fieldGcc32:
		return s.app.Cfg.GCC32Dir
	}
	return ""
}

// resolvedValue returns the autodiscovered value for a given field, used as
// the starting placeholder when the persisted value is empty.
func (s PathsScreen) resolvedValue(f pathField) string {
	switch f {
	case fieldKernel:
		return s.app.Paths.Kernel
	case fieldClang:
		return s.app.Paths.Clang
	case fieldAnyKernel:
		return s.app.Paths.AnyKernel
	case fieldOutput:
		return s.app.Paths.Output
	case fieldGcc64:
		return s.app.Paths.GccArm64
	case fieldGcc32:
		return s.app.Paths.GccArm
	}
	return ""
}

func (s *PathsScreen) setFieldValue(f pathField, v string) {
	v = config.ExpandTilde(strings.TrimSpace(v))
	switch f {
	case fieldKernel:
		s.app.Cfg.KernelDir = v
	case fieldClang:
		s.app.Cfg.ClangDir = v
	case fieldAnyKernel:
		s.app.Cfg.AnyKernelDir = v
	case fieldOutput:
		s.app.Cfg.OutputDir = v
	case fieldGcc64:
		s.app.Cfg.GCC64Dir = v
	case fieldGcc32:
		s.app.Cfg.GCC32Dir = v
	}
}

// fieldLabel returns the human-readable name for the row.
func fieldLabel(f pathField) string {
	switch f {
	case fieldKernel:
		return "Kernel directory"
	case fieldClang:
		return "Clang directory"
	case fieldAnyKernel:
		return "AnyKernel3 directory"
	case fieldOutput:
		return "Output directory"
	case fieldGcc64:
		return "aarch64 GCC directory"
	case fieldGcc32:
		return "arm GCC directory"
	}
	return ""
}

// rescan re-runs discovery using the current config overrides and updates
// app.Paths in-place. Used after every save / on key 'R' / on tilde expansion.
func (s *PathsScreen) rescan() {
	p, _ := discover.Resolve(".",
		s.app.Cfg.KernelDir, s.app.Cfg.ClangDir, s.app.Cfg.AnyKernelDir, s.app.Cfg.OutputDir,
		s.app.Cfg.GCC64Dir, s.app.Cfg.GCC32Dir)
	s.app.Paths = p
}

func (s PathsScreen) Update(msg tea.Msg) (PathsScreen, tea.Cmd) {
	if s.editing != 0 {
		switch m := msg.(type) {
		case tea.KeyMsg:
			switch m.String() {
			case "enter":
				s.setFieldValue(s.editing, s.input.Value())
				s.app.PersistConfig()
				s.rescan()
				s.app.Toast = fieldLabel(s.editing) + " saved."
				s.app.ToastErr = false
				s.editing = 0
				return s, nil
			case "esc":
				s.editing = 0
				s.app.Toast = "Edit cancelled."
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		fields := s.fields()
		switch strings.ToLower(m.String()) {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil
		case "down", "j":
			if s.cursor < len(fields)-1 {
				s.cursor++
			}
			return s, nil
		case "enter", " ":
			if s.cursor >= 0 && s.cursor < len(fields) {
				f := fields[s.cursor]
				s.editing = f
				v := s.fieldValue(f)
				if v == "" {
					v = s.resolvedValue(f)
				}
				s.input.SetValue(v)
				s.input.Focus()
				return s, textinput.Blink
			}
		case "r":
			s.rescan()
			s.app.Toast = "Paths re-scanned."
			s.app.ToastErr = false
		case "1", "2", "3", "4", "5", "6":
			f := pathField(m.String()[0] - '0')
			s.cursor = int(f) - 1
			s.editing = f
			v := s.fieldValue(f)
			if v == "" {
				v = s.resolvedValue(f)
			}
			s.input.SetValue(v)
			s.input.Focus()
			return s, textinput.Blink
		case "esc", "b":
			// Return to Setup wrapper menu (NOT the main menu) — matches
			// the bash structure where Setup → P → R returns to Setup.
			s.app.Screen = ScreenSetup
			return s, nil
		}
	}
	return s, nil
}

func (s PathsScreen) View() string {
	w := panelWidth(s.app.Width)
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"PATHS  CONFIG",
		"autodiscovered, persisted in ~/.config/vayu_builder",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Paths panel ─────────────────────────────────────────────────────────
	type row struct {
		key, label, value string
		bad               bool
	}
	rows := []row{
		{"1", "Kernel    ", okOr(s.app.Paths.Kernel, defaultOr(s.app.Cfg.KernelDir, "(not found)")), s.app.Paths.Kernel == ""},
		{"2", "Clang     ", okOr(s.app.Paths.Clang, defaultOr(s.app.Cfg.ClangDir, "(not found)")), s.app.Paths.Clang == ""},
		{"3", "AnyKernel3", okOr(s.app.Paths.AnyKernel, defaultOr(s.app.Cfg.AnyKernelDir, "(not found)")), s.app.Paths.AnyKernel == ""},
		{"4", "Output    ", okOr(s.app.Paths.Output, defaultOr(s.app.Cfg.OutputDir, "(unset)")), s.app.Paths.Output == ""},
		{"5", "aarch64-gcc", okOr(s.app.Paths.GccArm64, defaultOr(s.app.Cfg.GCC64Dir, "(not found)")), s.app.Paths.GccArm64 == ""},
		{"6", "arm-gcc   ", okOr(s.app.Paths.GccArm, defaultOr(s.app.Cfg.GCC32Dir, "(not found)")), s.app.Paths.GccArm == ""},
	}
	labelW := 12
	// "  [N]  Label         : " — mark(2) + tag(3) + 2sp + labelW + " : "(3)
	prefixW := 2 + 3 + 2 + labelW + 3
	valWrap := innerW - prefixW
	if valWrap < 16 {
		valWrap = 16
	}
	var p strings.Builder
	for i, r := range rows {
		v := ValueStyle
		if r.bad {
			v = ErrText
		}
		mark := "  "
		if i == s.cursor {
			mark = AccentText.Render(" ›")
		}
		tag := components.BracketTag(r.key, 1, HotKeyStyle)
		valLines := wrapValue(r.value, valWrap)
		head := mark + tag + "  " + LabelStyle.Render(padRight(r.label, labelW)) + LabelStyle.Render(" : ") + v.Render(valLines[0])
		p.WriteString(head)
		indent := components.SafeRepeat(" ", prefixW)
		for _, l := range valLines[1:] {
			p.WriteString("\n" + indent + v.Render(l))
		}
		if i < len(rows)-1 {
			p.WriteString("\n")
		}
	}
	pathsPanel := components.Panel("Resolved paths", p.String(), w, PanelBorder, TitleStyle)

	// ── Editor panel ────────────────────────────────────────────────────────
	var editorPanel string
	if s.editing != 0 {
		var b strings.Builder
		b.WriteString(LabelStyle.Render("Editing: ") + ValueStyle.Render(fieldLabel(s.editing)) + "\n")
		b.WriteString(s.input.View())
		editorPanel = components.Panel("Edit path", b.String(), w, PanelWarn,
			lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)) + "\n"
	}

	// Path-issue warnings (bash parity: shown inside the Paths screen too).
	var warnPanel string
	if s.app.Paths.Clang == "" || s.app.Paths.AnyKernel == "" {
		var msgs []string
		if s.app.Paths.Clang == "" {
			msgs = append(msgs, "[!] Clang not found: "+defaultOr(s.app.Cfg.ClangDir, "(unset)"))
		}
		if s.app.Paths.AnyKernel == "" {
			msgs = append(msgs, "[!] AnyKernel3 not found: "+defaultOr(s.app.Cfg.AnyKernelDir, "(unset)"))
		}
		body := ErrText.Render(strings.Join(msgs, "\n"))
		warnPanel = components.Panel("Path Issues", body, w, PanelErr, ErrText) + "\n"
	}

	var helpText string
	if s.editing != 0 {
		helpText = "Enter saves · esc cancels"
	} else {
		helpText = "Select [1-6/Enter/R/B] · esc to return"
	}
	out := banner + "\n" + pathsPanel + "\n" + editorPanel + warnPanel +
		"  " + HelpStyle.Render(helpText) + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}
