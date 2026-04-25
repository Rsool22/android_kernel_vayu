package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/state"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// BuildOptionsScreen mirrors the bash `run_build_menu` screen: kernel-name
// prompt, [I]ncremental and [C]cache toggles, [X] reset build counter,
// [S]tart build, [B]ack to features. Bubbles textinput drives the kernel-
// name editor.
type BuildOptionsScreen struct {
	app *App

	ti       textinput.Model
	editing  bool
	dirty    bool
	ccacheOK bool
}

// NewBuildOptionsScreen constructs the screen with a textinput for kernel
// name and probes ccache availability for the [C] toggle's N/A state.
func NewBuildOptionsScreen(a *App) BuildOptionsScreen {
	ti := textinput.New()
	ti.Placeholder = "AnyMore-v2.1   (LOCALVERSION suffix; leave empty for default)"
	ti.CharLimit = 64
	ti.Width = 60
	ti.Prompt = "  Kernel-Name: "
	ti.SetValue(a.Cfg.KernelName)
	ti.PromptStyle = LabelStyle
	ti.TextStyle = ValueStyle
	ti.PlaceholderStyle = MutedText
	_, ccacheErr := exec.LookPath("ccache")
	return BuildOptionsScreen{app: a, ti: ti, ccacheOK: ccacheErr == nil}
}

func (s BuildOptionsScreen) Init() tea.Cmd { return nil }

// Update handles all the build-options keys and the textinput sub-model.
func (s BuildOptionsScreen) Update(msg tea.Msg) (BuildOptionsScreen, tea.Cmd) {
	if s.editing {
		switch m := msg.(type) {
		case tea.KeyMsg:
			switch m.String() {
			case "enter":
				v := strings.TrimSpace(s.ti.Value())
				s.app.Cfg.KernelName = v
				s.dirty = true
				_ = s.app.Cfg.Save()
				s.editing = false
				s.ti.Blur()
				if v == "" {
					s.app.Toast = "Kernel-Name cleared"
				} else {
					s.app.Toast = "Kernel-Name set to " + v
				}
				s.app.ToastErr = false
				return s, nil
			case "esc":
				s.editing = false
				s.ti.Blur()
				s.ti.SetValue(s.app.Cfg.KernelName)
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.ti, cmd = s.ti.Update(msg)
		return s, cmd
	}

	switch m := msg.(type) {
	case tea.KeyMsg:
		switch strings.ToLower(m.String()) {
		case "n":
			s.editing = true
			s.ti.SetValue(s.app.Cfg.KernelName)
			s.ti.Focus()
			return s, textinput.Blink
		case "i":
			s.app.Builder.Incremental = !s.app.Builder.Incremental
			if s.app.Builder.ForceCleanReason != "" && s.app.Builder.Incremental {
				s.app.Toast = fmt.Sprintf("Incremental locked off — %s requires clean", s.app.Builder.ForceCleanReason)
				s.app.ToastErr = true
				s.app.Builder.Incremental = false
				return s, nil
			}
			if err := s.app.Builder.Save(s.app.Paths.Kernel); err != nil {
				s.app.Toast = "Save failed: " + err.Error()
				s.app.ToastErr = true
				return s, nil
			}
			if s.app.Builder.Incremental {
				s.app.Toast = "Incremental: ON (skip clean)"
			} else {
				s.app.Toast = "Incremental: OFF (full clean)"
			}
			s.app.ToastErr = false
		case "c":
			if !s.ccacheOK {
				s.app.Toast = "ccache not installed — install ccache via Setup → Dependencies"
				s.app.ToastErr = true
				return s, nil
			}
			s.app.Builder.UseCcache = !s.app.Builder.UseCcache
			if err := s.app.Builder.Save(s.app.Paths.Kernel); err != nil {
				s.app.Toast = "Save failed: " + err.Error()
				s.app.ToastErr = true
				return s, nil
			}
			if s.app.Builder.UseCcache {
				s.app.Toast = "ccache: ON"
			} else {
				s.app.Toast = "ccache: OFF"
			}
			s.app.ToastErr = false
		case "x":
			if err := state.ResetBuildNumber(s.app.Paths.Kernel, s.app.Paths.Output); err != nil {
				s.app.Toast = "Reset failed: " + err.Error()
				s.app.ToastErr = true
				return s, nil
			}
			s.app.Toast = "Build counter reset → next build is #1"
			s.app.ToastErr = false
		case "s":
			if s.app.Paths.Clang == "" || s.app.Paths.AnyKernel == "" {
				s.app.Toast = "Cannot start — fix Clang / AnyKernel3 paths first"
				s.app.ToastErr = true
				return s, nil
			}
			s.app.Screen = ScreenBuild
			return s, s.app.build.Init()
		case "b", "esc":
			// Back to Mode Select — the linear pipeline is
			//   Main → BuildOptions → [S] → do_build (compile → package).
			// Feature toggles are reachable here through [F] but never
			// gate this back-arrow.
			s.app.Screen = ScreenMain
			return s, nil
		case "f":
			// Optional jump to Feature Configuration (KSU / SuSFS / KPM
			// toggles + Menuconfig). Returning from there with [C] or
			// [B] lands back here — it never auto-advances to Build.
			s.app.Screen = ScreenFeatures
			return s, s.app.features.Init()
		case "q":
			return s, tea.Quit
		}
	}
	return s, nil
}

// View renders the screen.
func (s BuildOptionsScreen) View() string {
	w := panelWidth(s.app.Width)
	inner := innerContentWidth(w)

	banner := components.Banner(
		"BUILD  OPTIONS",
		"kernel-name · incremental · ccache · counter",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Active Features summary panel ───────────────────────────────────────
	st := readDefconfigState(s.app)
	var sm strings.Builder
	sm.WriteString(components.KV("Capabilities", st.CapTag(s.app.Builder.KSUBranch), 14, LabelStyle, ValueStyle) + "\n")
	sm.WriteString(components.KV("Features", st.ExtTag(), 14, LabelStyle, ValueStyle) + "\n")
	sm.WriteString(components.KV("Hook Mode", st.HookMode(), 14, LabelStyle, AccentText))
	summaryPanel := components.Panel("Active Features", sm.String(), w, PanelBorder, TitleStyle)

	// ── Toggles + actions panel ─────────────────────────────────────────────
	var body strings.Builder
	body.WriteString(buildOptRow("N", "Set Kernel-Name", knameValue(s.app.Cfg.KernelName), inner) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")

	// Incremental — locked when ForceCleanReason set
	incVal := "OFF (full clean)"
	incStyle := WarnText
	if s.app.Builder.Incremental {
		incVal = "ON  (skip clean)"
		incStyle = OKText
	}
	if reason := s.app.Builder.ForceCleanReason; reason != "" {
		incVal = "LOCKED — " + reason
		incStyle = ErrText
	}
	body.WriteString(buildOptRowStyled("I", "Toggle Incremental", incVal, incStyle, inner) + "\n")

	// ccache
	ccVal := "OFF"
	ccStyle := WarnText
	if !s.ccacheOK {
		ccVal = "N/A — ccache not installed"
		ccStyle = MutedText
	} else if s.app.Builder.UseCcache {
		ccVal = "ON"
		ccStyle = OKText
	}
	body.WriteString(buildOptRowStyled("C", "Toggle ccache", ccVal, ccStyle, inner) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")

	// Counter
	next := state.ReadBuildNumber(s.app.Paths.Kernel)
	body.WriteString(buildOptRowStyled("X", "Reset Build Counter",
		fmt.Sprintf("next #%s → #1", strconv.Itoa(next)), AccentText, inner) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")

	body.WriteString(buildOptRowStyled("F", "Feature Configuration",
		"KSU / SuSFS / KPM toggles", AccentText, inner) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")
	body.WriteString(buildOptRowStyled("S", "Start Build",
		fmt.Sprintf("compile + package as #%s", strconv.Itoa(next)), OKText, inner) + "\n")
	body.WriteString(buildOptRowStyled("B", "Back", "to Mode Select", LabelStyle, inner))
	togglesPanel := components.Panel("Build Options", body.String(), w, PanelBorder, TitleStyle)

	// ── Inline kernel-name editor ───────────────────────────────────────────
	var editor string
	if s.editing {
		editor = "\n" + components.Panel("Edit Kernel-Name",
			s.ti.View()+"\n  "+HelpStyle.Render("[Enter] save · [Esc] cancel"),
			w, PanelBorder.BorderForeground(ColorBanner), TitleStyle.Foreground(ColorBanner)) + "\n"
	}

	out := banner + "\n" + summaryPanel + "\n" + togglesPanel + editor + "\n" +
		"  " + HelpStyle.Render("Select [N/I/C/X/F/S/B]\u00a0\u00b7\u00a0esc to return") + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// buildOptRow renders a `[K]  Label .................. value` row using
// ValueStyle for the value.
func buildOptRow(key, label, value string, width int) string {
	return buildOptRowStyled(key, label, value, ValueStyle, width)
}

// buildOptRowStyled is buildOptRow with explicit value style.
func buildOptRowStyled(key, label, value string, valStyle lipgloss.Style, width int) string {
	tag := components.BracketTag(key, 1, HotKeyStyle)
	prefix := tag + "  " + ValueStyle.Render(label)
	rhs := valStyle.Render(value)
	pad := width - lipgloss.Width(prefix) - lipgloss.Width(rhs) - 2
	if pad < 1 {
		pad = 1
	}
	leader := MutedText.Render(" " + strings.Repeat("·", pad-2) + " ")
	return prefix + leader + rhs
}

func knameValue(v string) string {
	if v == "" {
		return "(default — no LOCALVERSION suffix)"
	}
	return v
}

// readDefconfigState reads the live defconfig and returns the feature state
// used by the Build Options summary panel. Returns a zero state on error.
func readDefconfigState(a *App) features.State {
	if a == nil || a.Paths.Kernel == "" {
		return features.State{}
	}
	dc := filepath.Join(a.Paths.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
	st, err := features.Read(dc)
	if err != nil {
		return features.State{}
	}
	st.DriverPresent = features.DriverPresent(a.Paths.Kernel)
	return st
}
