package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// SetupScreen displays autodiscovered paths and gives the user a chance to
// re-run discovery or jump to environment-specific install hints.
type SetupScreen struct {
	app *App
}

func NewSetupScreen(a *App) SetupScreen { return SetupScreen{app: a} }

func (s SetupScreen) Init() tea.Cmd { return nil }

func (s SetupScreen) Update(msg tea.Msg) (SetupScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(m.String()) {
		case "r":
			p, _ := discover.Resolve(".",
				s.app.Cfg.KernelDir, s.app.Cfg.ClangDir, s.app.Cfg.AnyKernelDir, s.app.Cfg.OutputDir)
			s.app.Paths = p
			s.app.Toast = "Paths re-scanned."
			s.app.ToastErr = false
		}
	}
	return s, nil
}

func (s SetupScreen) View() string {
	w := s.app.Width
	if w < 60 {
		w = 60
	}
	if w > 100 {
		w = 100
	}

	var b strings.Builder
	b.WriteString(components.Banner("SETUP / PATHS", "autodiscovered", w, ColorTitle, ColorAccent) + "\n")
	b.WriteString(components.Rule("", w, MutedText) + "\n\n")

	statusKey := lipgloss.NewStyle().Foreground(ColorDim)
	statusVal := lipgloss.NewStyle().Foreground(ColorAccent)
	b.WriteString(components.KV("Distro", string(s.app.Paths.Distro), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Kernel", okOr(s.app.Paths.Kernel, "(not found)"), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Clang", okOr(s.app.Paths.Clang, "(not found)"), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("AnyKernel3", okOr(s.app.Paths.AnyKernel, "(not found)"), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("Output", okOr(s.app.Paths.Output, "(unset)"), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("aarch64-gcc", okOr(s.app.Paths.GccArm64, "(not found)"), 14, statusKey, statusVal) + "\n")
	b.WriteString(components.KV("arm-gcc", okOr(s.app.Paths.GccArm, "(not found)"), 14, statusKey, statusVal) + "\n\n")

	if s.app.Paths.Distro != discover.PMUnknown {
		b.WriteString(components.Rule("Install hints", w, MutedText) + "\n")
		for _, line := range installHints(s.app.Paths) {
			b.WriteString("  " + DimText.Render(line) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(components.Rule("Action", w, MutedText) + "\n")
	b.WriteString(components.Hotkey("R", "Re-scan paths", "", HotKeyStyle, lipgloss.NewStyle().Foreground(ColorOK), DimText) + "\n")
	b.WriteString(components.Hotkey("ESC", "Return to main", "", HotKeyStyle, DimText, DimText) + "\n")

	if s.app.Toast != "" {
		st := OKText
		if s.app.ToastErr {
			st = ErrText
		}
		b.WriteString("\n" + st.Render("  "+s.app.Toast))
	}
	return b.String()
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
