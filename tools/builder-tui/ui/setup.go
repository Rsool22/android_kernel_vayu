package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
type SetupScreen struct {
	app   *App
	editing pathField
	input textinput.Model
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

func (s *SetupScreen) setFieldValue(f pathField, v string) {
	v = strings.TrimSpace(v)
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

func (s SetupScreen) Update(msg tea.Msg) (SetupScreen, tea.Cmd) {
	if s.editing != 0 {
		switch m := msg.(type) {
		case tea.KeyMsg:
			switch m.String() {
			case "enter":
				s.setFieldValue(s.editing, s.input.Value())
				s.app.PersistConfig()
				p, _ := discover.Resolve(".",
					s.app.Cfg.KernelDir, s.app.Cfg.ClangDir, s.app.Cfg.AnyKernelDir, s.app.Cfg.OutputDir)
				s.app.Paths = p
				label := fieldLabel(s.editing)
				s.editing = 0
				return s, s.app.SetToast(label+" saved.", false)
			case "esc":
				s.editing = 0
				return s, s.app.SetToast("Edit cancelled.", false)
			}
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(m.String()) {
		case "r":
			p, _ := discover.Resolve(".",
				s.app.Cfg.KernelDir, s.app.Cfg.ClangDir, s.app.Cfg.AnyKernelDir, s.app.Cfg.OutputDir)
			s.app.Paths = p
			return s, s.app.SetToast("Paths re-scanned.", false)
		case "1", "2", "3", "4", "5", "6":
			f := pathField(m.String()[0] - '0')
			s.editing = f
			s.input.SetValue(s.fieldValue(f))
			s.input.Focus()
			return s, textinput.Blink
		}
	}
	return s, nil
}

func (s SetupScreen) View() string {
	w := panelWidth(s.app.Width)

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
		{"1", "Kernel", okOr(s.app.Paths.Kernel, "(not found)"), s.app.Paths.Kernel == ""},
		{"2", "Clang", okOr(s.app.Paths.Clang, "(not found)"), s.app.Paths.Clang == ""},
		{"3", "AnyKernel3", okOr(s.app.Paths.AnyKernel, "(not found)"), s.app.Paths.AnyKernel == ""},
		{"4", "Output", okOr(s.app.Paths.Output, "(unset)"), s.app.Paths.Output == ""},
		{"5", "aarch64-gcc", okOr(s.app.Paths.GccArm64, "(not found)"), s.app.Paths.GccArm64 == ""},
		{"6", "arm-gcc", okOr(s.app.Paths.GccArm, "(not found)"), s.app.Paths.GccArm == ""},
	}
	innerW := innerContentWidth(w)
	const labelW = 11 // widest label ("AnyKernel3") + a little breathing room
	var p strings.Builder
	for i, r := range rows {
		v := ValueStyle
		if r.bad {
			v = ErrText
		}
		tag := components.GlobalBracketTag(r.key, HotKeyStyle)
		label := LabelStyle.Render(padTo(r.label, labelW))
		prefix := tag + "  " + label
		p.WriteString(components.LeaderRow(prefix, v.Render(r.value), innerW, MutedText))
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
		editorPanel = components.Panel("Edit path", b.String(), w, PanelWarn, lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)) + "\n"
	}

	// ── Install hints panel (distro-aware) ──────────────────────────────────
	var hintsPanel string
	if s.app.Paths.Distro != discover.PMUnknown {
		var h strings.Builder
		hints := installHints(s.app.Paths)
		for i, line := range hints {
			h.WriteString("  " + AccentText.Render("$ ") + ValueStyle.Render(line))
			if i < len(hints)-1 {
				h.WriteString("\n")
			}
		}
		hintsPanel = components.Panel("Install hints ("+string(s.app.Paths.Distro)+")", h.String(), w, PanelBorder, TitleStyle) + "\n"
	}

	var actions string
	if s.editing != 0 {
		actions = components.HotkeyStrip([]components.Hotkey{
			{Key: "Enter", Desc: "Save"},
			{Key: "ESC", Desc: "Cancel"},
		}, HotKeyStyle, ValueStyle, DimText, MutedText)
	} else {
		actions = components.HotkeyStrip([]components.Hotkey{
			{Key: "1-6", Desc: "Edit", Sub: "path slot"},
			{Key: "R", Desc: "Re-scan"},
			{Key: "ESC", Desc: "Back"},
		}, HotKeyStyle, ValueStyle, DimText, MutedText)
	}

	divider := "  " + components.Separator(innerContentWidth(w), MutedText) + "\n"
	out := banner + "\n" + pathsPanel + "\n" + editorPanel + hintsPanel
	if s.app.Activity != nil {
		out += components.ActivityPanel(s.app.Activity, w, 5, PanelDim, TitleStyle.Foreground(ColorDim)) + "\n"
	}
	out += divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
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
