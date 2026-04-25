package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
	w := panelWidth(s.app.Width)

	banner := components.Banner(
		"SETUP  ·  PATHS",
		"autodiscovered, persisted in ~/.config/vayu_builder",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Paths panel ─────────────────────────────────────────────────────────
	var p strings.Builder
	rows := []struct {
		k, v string
		bad  bool
	}{
		{"Distro", string(s.app.Paths.Distro), s.app.Paths.Distro == discover.PMUnknown},
		{"Kernel", okOr(s.app.Paths.Kernel, "(not found)"), s.app.Paths.Kernel == ""},
		{"Clang", okOr(s.app.Paths.Clang, "(not found)"), s.app.Paths.Clang == ""},
		{"AnyKernel3", okOr(s.app.Paths.AnyKernel, "(not found)"), s.app.Paths.AnyKernel == ""},
		{"Output", okOr(s.app.Paths.Output, "(unset)"), s.app.Paths.Output == ""},
		{"aarch64-gcc", okOr(s.app.Paths.GccArm64, "(not found)"), s.app.Paths.GccArm64 == ""},
		{"arm-gcc", okOr(s.app.Paths.GccArm, "(not found)"), s.app.Paths.GccArm == ""},
	}
	for i, r := range rows {
		v := ValueStyle
		if r.bad {
			v = ErrText
		}
		p.WriteString(components.KV(r.k, r.v, 12, LabelStyle, v))
		if i < len(rows)-1 {
			p.WriteString("\n")
		}
	}
	pathsPanel := components.Panel("Resolved paths", p.String(), w, PanelBorder, TitleStyle)

	// ── Install hints panel (distro-aware) ──────────────────────────────────
	var hintsPanel string
	if s.app.Paths.Distro != discover.PMUnknown {
		var h strings.Builder
		for i, line := range installHints(s.app.Paths) {
			h.WriteString("  " + AccentText.Render("$ ") + ValueStyle.Render(line))
			if i < len(installHints(s.app.Paths))-1 {
				h.WriteString("\n")
			}
		}
		hintsPanel = components.Panel("Install hints ("+string(s.app.Paths.Distro)+")", h.String(), w, PanelBorder, TitleStyle) + "\n"
	}

	actions := components.HotkeyStrip([]string{
		components.Hotkey("R", "Re-scan", "re-run autodiscovery", HotKeyStyle, OKText, DimText),
		components.Hotkey("ESC", "Back", "", HotKeyStyle, DimText, DimText),
	}, MutedText)

	out := banner + "\n" + pathsPanel + "\n" + hintsPanel + "  " + actions + "\n"
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
