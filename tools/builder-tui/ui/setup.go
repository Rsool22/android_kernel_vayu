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

// SetupScreen renders the resolved paths panel + distro install hints
// and exposes inline path editing via bubbles textinput.
//
// Rows are navigable by hotkey (1-6) OR ↑/↓ + Enter. Long values wrap to a
// continuation line indented under the value column so paths never overflow
// off-screen.
type SetupScreen struct {
	app     *App
	editing pathField
	input   textinput.Model
	cursor  int // 0..5, indexes into pathField list for arrow-nav
}

func NewSetupScreen(a *App) SetupScreen {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 4096
	ti.Width = 60
	ti.Prompt = "▶ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorValue)
	return SetupScreen{app: a, input: ti}
}

func (s SetupScreen) Init() tea.Cmd { return nil }

// fields returns all path fields in display order.
func (s SetupScreen) fields() []pathField {
	return []pathField{fieldKernel, fieldClang, fieldAnyKernel, fieldOutput, fieldGcc64, fieldGcc32}
}

// fieldValue returns the current persisted value for a given field.
func (s SetupScreen) fieldValue(f pathField) string {
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
func (s SetupScreen) resolvedValue(f pathField) string {
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

func (s *SetupScreen) setFieldValue(f pathField, v string) {
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
func (s *SetupScreen) rescan() {
	p, _ := discover.Resolve(".",
		s.app.Cfg.KernelDir, s.app.Cfg.ClangDir, s.app.Cfg.AnyKernelDir, s.app.Cfg.OutputDir,
		s.app.Cfg.GCC64Dir, s.app.Cfg.GCC32Dir)
	s.app.Paths = p
}

func (s SetupScreen) Update(msg tea.Msg) (SetupScreen, tea.Cmd) {
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
		}
	}
	return s, nil
}

func (s SetupScreen) View() string {
	w := panelWidth(s.app.Width)
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"SETUP  ·  PATHS",
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
	// label column width = max(label) + tag (4) + separators
	labelW := 12
	prefixW := 4 /*"  [N]"*/ + 2 + labelW + 2 // marker + tag + space + label + " : "
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
		// Cursor marker: chevron when hovered, otherwise blank.
		mark := "  "
		if i == s.cursor {
			mark = AccentText.Render(" ›")
		}
		tag := components.BracketTag(r.key, 1, HotKeyStyle)
		// Wrap long values with continuation indent under the value column.
		valLines := wrapValue(r.value, valWrap)
		head := mark + " " + tag + " " + LabelStyle.Render(padRight(r.label, labelW)) + LabelStyle.Render(" : ") + v.Render(valLines[0])
		p.WriteString(head)
		indent := strings.Repeat(" ", prefixW)
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

	// ── Install hints panel (distro-aware) ──────────────────────────────────
	var hintsPanel string
	if s.app.Paths.Distro != discover.PMUnknown {
		var h strings.Builder
		hints := installHints(s.app.Paths)
		for i, line := range hints {
			lines := wrapValue(line, innerW-4)
			h.WriteString("  " + AccentText.Render("$ ") + ValueStyle.Render(lines[0]))
			for _, l := range lines[1:] {
				h.WriteString("\n    " + ValueStyle.Render(l))
			}
			if i < len(hints)-1 {
				h.WriteString("\n")
			}
		}
		hintsPanel = components.Panel("Install hints ("+string(s.app.Paths.Distro)+")", h.String(),
			w, PanelBorder, TitleStyle) + "\n"
	}

	var actions string
	if s.editing != 0 {
		actions = components.HotkeyStripWrap([]components.Hotkey{
			{Key: "Enter", Desc: "Save"},
			{Key: "ESC", Desc: "Cancel"},
		}, stripWidth(s.app.Width), HotKeyStyle, ValueStyle, DimText, MutedText)
	} else {
		actions = components.HotkeyStripWrap([]components.Hotkey{
			{Key: "1-6", Desc: "Edit", Sub: "path slot"},
			{Key: "↑/↓", Desc: "Navigate"},
			{Key: "Enter", Desc: "Edit selected"},
			{Key: "R", Desc: "Re-scan"},
			{Key: "ESC", Desc: "Back"},
		}, stripWidth(s.app.Width), HotKeyStyle, ValueStyle, DimText, MutedText)
	}

	divider := components.Separator(w, MutedText) + "\n"
	out := banner + "\n" + pathsPanel + "\n" + editorPanel + hintsPanel + divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// wrapValue is a thin wrapper over components.wrapPlain that's exposed for
// callers in this file. Keeps long path values on a single line until they
// exceed `width`, then breaks on path separators preferentially.
func wrapValue(s string, width int) []string {
	if width <= 0 || len(s) <= width {
		return []string{s}
	}
	return wrapPlainPaths(s, width)
}

// wrapPlainPaths breaks s at /, space, or hyphen boundaries near the width
// limit; falls back to a hard cut.
func wrapPlainPaths(s string, width int) []string {
	var lines []string
	for len(s) > width {
		cut := width
		for i := width; i > width/2 && i < len(s); i-- {
			c := s[i]
			if c == '/' || c == ' ' || c == '-' || c == '_' {
				cut = i + 1
				break
			}
		}
		if cut > len(s) {
			cut = len(s)
		}
		lines = append(lines, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

// padRight right-pads s to the given visual width with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// defaultOr returns d when d is non-empty, otherwise alt. Used to surface
// the *configured default* when discovery fails so the user sees what the
// builder is going to try, instead of "(not found)" alone.
func defaultOr(d, alt string) string {
	if d == "" {
		return alt
	}
	return d
}

func installHints(p discover.Paths) []string {
	pkgs := []string{"git", "ccache", "make", "bc", "bison", "flex", "zip", "unzip", "rsync", "python3", "build-essential"}
	if p.GccArm64 == "" {
		pkgs = append(pkgs, "gcc-aarch64-linux-gnu")
	}
	if p.GccArm == "" {
		pkgs = append(pkgs, "gcc-arm-linux-gnueabi")
	}
	switch p.Distro {
	case discover.PMApt:
		return []string{"sudo apt-get install -y " + strings.Join(pkgs, " ")}
	case discover.PMDnf:
		return []string{"sudo dnf install -y " + strings.Join(pkgs, " ")}
	case discover.PMPacman:
		return []string{"sudo pacman -S --needed " + strings.Join(pkgs, " ")}
	case discover.PMZypper:
		return []string{"sudo zypper install " + strings.Join(pkgs, " ")}
	case discover.PMApk:
		return []string{"sudo apk add " + strings.Join(pkgs, " ")}
	}
	return []string{"(distro unknown -- install: " + strings.Join(pkgs, ", ") + ")"}
}
