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
	// focus is the ↑/↓ cursor on the build-options rows. Indexes into
	// buildOptKeys() so Enter can dispatch the focused action.
	focus int
}

// buildOptKeys is the hotkey order shown on the screen. Kept in sync
// with the body rendering order so arrow nav and Enter fire on the
// right action.
func (s BuildOptionsScreen) buildOptKeys() []string {
	return []string{"n", "i", "c", "x", "s", "b"}
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
				msgText := "Kernel-Name cleared"
				if v != "" {
					msgText = "Kernel-Name set to " + v
				}
				return s, s.app.SetToast(msgText, false)
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
		key := strings.ToLower(m.String())
		// Arrow-nav over the build-options rows. Enter / Space fires
		// the action the cursor is on. (prompt #5 — extend to every
		// menu screen, not just main / toolchain / features.)
		keys := s.buildOptKeys()
		n := len(keys)
		switch key {
		case "up", "k":
			s.focus = (s.focus + n - 1) % n
			return s, nil
		case "down", "j":
			s.focus = (s.focus + 1) % n
			return s, nil
		case "enter", " ":
			key = keys[s.focus]
		}
		switch key {
		case "n":
			s.editing = true
			s.ti.SetValue(s.app.Cfg.KernelName)
			s.ti.Focus()
			return s, textinput.Blink
		case "i":
			s.app.Builder.Incremental = !s.app.Builder.Incremental
			if s.app.Builder.ForceCleanReason != "" && s.app.Builder.Incremental {
				s.app.Builder.Incremental = false
				return s, s.app.SetToast(fmt.Sprintf("Incremental locked off — %s requires clean", s.app.Builder.ForceCleanReason), true)
			}
			if err := s.app.Builder.Save(s.app.Paths.Kernel); err != nil {
				return s, s.app.SetToast("Save failed: "+err.Error(), true)
			}
			msgText := "Incremental: OFF (full clean)"
			if s.app.Builder.Incremental {
				msgText = "Incremental: ON (skip clean)"
			}
			return s, s.app.SetToast(msgText, false)
		case "c":
			if !s.ccacheOK {
				return s, s.app.SetToast("ccache not installed — install ccache via Setup → Dependencies", true)
			}
			s.app.Builder.UseCcache = !s.app.Builder.UseCcache
			if err := s.app.Builder.Save(s.app.Paths.Kernel); err != nil {
				return s, s.app.SetToast("Save failed: "+err.Error(), true)
			}
			msgText := "ccache: OFF"
			if s.app.Builder.UseCcache {
				msgText = "ccache: ON"
			}
			return s, s.app.SetToast(msgText, false)
		case "x":
			if err := state.ResetBuildNumber(s.app.Paths.Kernel, s.app.Paths.Output); err != nil {
				return s, s.app.SetToast("Reset failed: "+err.Error(), true)
			}
			return s, s.app.SetToast("Build counter reset → next build is #1", false)
		case "s":
			if s.app.Paths.Clang == "" || s.app.Paths.AnyKernel == "" {
				return s, s.app.SetToast("Cannot start — fix Clang / AnyKernel3 paths first", true)
			}
			s.app.Screen = ScreenBuild
			return s, s.app.build.Init()
		case "b":
			s.app.Screen = ScreenFeatures
			return s, nil
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
	keys := s.buildOptKeys()
	isSel := func(k string) bool {
		if s.focus < 0 || s.focus >= len(keys) {
			return false
		}
		return keys[s.focus] == strings.ToLower(k)
	}
	body.WriteString(buildOptRow("N", "Set Kernel-Name", knameValue(s.app.Cfg.KernelName), inner, isSel("n")) + "\n")
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
	body.WriteString(buildOptRowStyled("I", "Toggle Incremental", incVal, incStyle, inner, isSel("i")) + "\n")

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
	body.WriteString(buildOptRowStyled("C", "Toggle ccache", ccVal, ccStyle, inner, isSel("c")) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")

	// Counter
	next := state.ReadBuildNumber(s.app.Paths.Kernel)
	body.WriteString(buildOptRowStyled("X", "Reset Build Counter",
		fmt.Sprintf("next #%s → #1", strconv.Itoa(next)), AccentText, inner, isSel("x")) + "\n")
	body.WriteString(components.Separator(inner, MutedText) + "\n")

	body.WriteString(buildOptRowStyled("S", "Start Build",
		fmt.Sprintf("compile + package as #%s", strconv.Itoa(next)), OKText, inner, isSel("s")) + "\n")
	body.WriteString(buildOptRowStyled("B", "Back", "to Features", LabelStyle, inner, isSel("b")))
	togglesPanel := components.Panel("Build Options", body.String(), w, PanelBorder, TitleStyle)

	// ── Inline kernel-name editor ───────────────────────────────────────────
	var editor string
	if s.editing {
		editor = "\n" + components.Panel("Edit Kernel-Name",
			s.ti.View()+"\n  "+HelpStyle.Render("[Enter] save · [Esc] cancel"),
			w, PanelBorder.BorderForeground(ColorBanner), TitleStyle.Foreground(ColorBanner)) + "\n"
	}

	// ── Keys panel (replaces the old footer strip) ──────────────────────────
	keysPanel := components.KeysPanel([]components.Hotkey{
		{Key: "N", Desc: "Name"},
		{Key: "I", Desc: "Incremental"},
		{Key: "C", Desc: "ccache"},
		{Key: "X", Desc: "Reset counter"},
		{Key: "S", Desc: "Start build"},
		{Key: "B", Desc: "Back", Sub: "features"},
		{Key: "ESC", Desc: "Main"},
	}, w, PanelDim, TitleStyle.Foreground(ColorDim),
		HotKeyStyle, ValueStyle, DimText, MutedText)

	out := banner + "\n" + summaryPanel + "\n" + togglesPanel + editor + "\n" + keysPanel + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	_ = inner
	return out
}

// buildOptRow renders a `[K]  Label .................. value` row using
// ValueStyle for the value.
func buildOptRow(key, label, value string, width int, selected bool) string {
	return buildOptRowStyled(key, label, value, ValueStyle, width, selected)
}

// buildOptRowStyled is buildOptRow with explicit value style. When
// selected is true, the row gets a `›` cursor cell prefix so arrow-nav
// is visible to the user.
func buildOptRowStyled(key, label, value string, valStyle lipgloss.Style, width int, selected bool) string {
	cursor := components.CursorCell(selected, AccentText)
	tag := components.GlobalBracketTag(key, HotKeyStyle)
	prefix := cursor + tag + "  " + ValueStyle.Render(label)
	rhs := valStyle.Render(value)
	pad := width - lipgloss.Width(prefix) - lipgloss.Width(rhs) - 2
	if pad < 1 {
		pad = 1
	}
	return prefix + components.Leader(pad, MutedText) + rhs
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
