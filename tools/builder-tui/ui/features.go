package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/state"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// FeaturesScreen mirrors the bash build's "FEATURE CONFIGURATION" menu.
// Toggles ReSukiSU / SuSFS / KPM in vayu_defconfig with the same
// cross-feature cascade rules as the original script.
//
// focus holds the arrow-nav cursor row (0 = KSU, 1 = SuSFS, 2 = KPM).
// Enter/space toggles the focused row; the 1/2/3 hotkeys still work
// as direct jumps. Prompt item #5 (universal arrow-nav).
type FeaturesScreen struct {
	app   *App
	state features.State
	read  bool
	focus int
}

func NewFeaturesScreen(a *App) FeaturesScreen { return FeaturesScreen{app: a} }

func (s FeaturesScreen) Init() tea.Cmd { return nil }

// defconfigPath returns the resolved path to arch/arm64/configs/vayu_defconfig
// or "" when the kernel root isn't known yet.
func (s FeaturesScreen) defconfigPath() string {
	if s.app.Paths.Kernel == "" {
		return ""
	}
	return filepath.Join(s.app.Paths.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
}

// refresh re-reads the defconfig and refreshes driver-presence flag.
func (s *FeaturesScreen) refresh() {
	s.read = false
	dc := s.defconfigPath()
	if dc == "" {
		return
	}
	st, err := features.Read(dc)
	if err != nil {
		return
	}
	st.DriverPresent = features.DriverPresent(s.app.Paths.Kernel)
	s.state = st
	s.read = true
}

func (s FeaturesScreen) Update(msg tea.Msg) (FeaturesScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if !s.read {
			s.refresh()
		}
		dc := s.defconfigPath()
		if dc == "" {
			return s, s.app.SetToast("Kernel root not resolved -- run Setup first", true)
		}
		key := strings.ToLower(m.String())
		// Arrow-nav cursor across the 3 feature rows (prompt #5).
		switch key {
		case "up", "k":
			s.focus = (s.focus + 2) % 3
			return s, nil
		case "down", "j":
			s.focus = (s.focus + 1) % 3
			return s, nil
		case "enter", " ":
			// Toggle the focused row. Translate focus -> hotkey and fall
			// through into the original per-row switch below.
			key = []string{"1", "2", "3"}[s.focus]
		}
		var cmd tea.Cmd
		switch key {
		case "1":
			st, err := features.ToggleKSU(dc, s.state)
			s.state = st
			if err != nil {
				cmd = s.app.SetToast(err.Error(), true)
			} else if st.KSU {
				cmd = s.app.SetToast("ReSukiSU enabled (Manual-Hook on)", false)
			} else {
				cmd = s.app.SetToast("ReSukiSU disabled (SuSFS + KPM + ManualHook cleared)", false)
			}
		case "2":
			st, err := features.ToggleSUSFS(dc, s.state)
			s.state = st
			if err != nil {
				cmd = s.app.SetToast(err.Error(), true)
			} else if st.SUSFS {
				cmd = s.app.SetToast("SuSFS enabled (SuSFS-Inline-Hook active)", false)
			} else {
				cmd = s.app.SetToast("SuSFS disabled (Manual-Hook auto-enabled)", false)
			}
		case "3":
			st, err := features.ToggleKPM(dc, s.state)
			s.state = st
			if err != nil {
				cmd = s.app.SetToast(err.Error(), true)
			} else if st.KPM {
				cmd = s.app.SetToast("KPM enabled", false)
			} else {
				cmd = s.app.SetToast("KPM disabled", false)
			}
		case "r":
			s.refresh()
			cmd = s.app.SetToast("Defconfig re-read.", false)
		case "m":
			return s, s.runMenuconfig()
		}
		return s, cmd
	case menuconfigDoneMsg:
		cmd := s.handleMenuconfigDone(m)
		s.refresh()
		return s, cmd
	}
	return s, nil
}

// menuconfigDoneMsg carries the result of an interactive `make menuconfig`
// session: whether .config mtime advanced (changes saved), exit code, and
// any error from the wrapper.
type menuconfigDoneMsg struct {
	saved   bool
	rc      int
	err     error
	cfgPath string
}

// runMenuconfig suspends the TUI, runs `make menuconfig` interactively,
// then resumes and emits a menuconfigDoneMsg with the outcome.
func (s FeaturesScreen) runMenuconfig() tea.Cmd {
	if s.app.Paths.Kernel == "" || s.app.Paths.Output == "" {
		return s.app.SetToast("Kernel/output paths unresolved -- run Setup first", true)
	}
	cfg := filepath.Join(s.app.Paths.Output, ".config")
	mtimeBefore := configMtime(cfg)

	cc := "clang"
	if s.app.Builder.UseCcache {
		cc = "ccache clang"
	}
	cmd := exec.Command(
		"make", "-C", s.app.Paths.Kernel,
		"O="+s.app.Paths.Output,
		"ARCH=arm64", "LLVM=1", "LLVM_IAS=1", "CC="+cc,
		"menuconfig",
	)
	if s.app.Paths.Clang != "" {
		path := os.Getenv("PATH")
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(s.app.Paths.Clang, "bin")+":"+path)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		rc := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			} else {
				rc = -1
			}
		}
		return menuconfigDoneMsg{
			saved:   configMtime(cfg) > mtimeBefore && rc == 0,
			rc:      rc,
			err:     err,
			cfgPath: cfg,
		}
	})
}

// handleMenuconfigDone processes the menuconfig result: mark MenuconfigUsed
// when changes were saved, clear preserved file (mtime advance overrides
// any pending preserved .config), and return an auto-dismiss toast Cmd.
func (s FeaturesScreen) handleMenuconfigDone(m menuconfigDoneMsg) tea.Cmd {
	if m.err != nil && m.rc < 0 {
		return s.app.SetToast("Menuconfig failed: "+m.err.Error(), true)
	}
	if m.rc != 0 {
		return s.app.SetToast("Menuconfig aborted -- no changes applied", false)
	}
	if !m.saved {
		return s.app.SetToast("Menuconfig closed without saving -- no changes", false)
	}
	s.app.MenuconfigUsed = true
	state.ClearMenuconfigPreserve(s.app.Paths.Kernel)
	s.app.MenuconfigPreserved = false
	return s.app.SetToast("Menuconfig saved (Stage 2 defconfig regen skipped this session)", false)
}

// configMtime returns the file mtime in unix seconds (or 0 when missing).
func configMtime(p string) int64 {
	if st, err := os.Stat(p); err == nil {
		return st.ModTime().Unix()
	}
	return 0
}

// featureRow renders one feature row with [N] tag, label, dotted leader
// and a status badge that right-aligns inside the panel. All four badge
// variants (ENABLED/DISABLED/N/A/NO DRIVER) are padded to a fixed width
// so they line up across rows; the leader fills the gap between label
// and badge so badges sit at the same column regardless of label width.
// When selected is true the ▸ cursor glyph is rendered in the dedicated
// cursor cell (prompt #5 + #6).
func featureRow(num, name string, enabled, available, driver, selected bool, width int) string {
	const badgeW = 10
	cursor := components.CursorCell(selected, SelText)
	tag := components.GlobalBracketTag(num, HotKeyStyle)
	var label, rhs string
	switch {
	case !driver:
		label = ValueStyle.Render(name)
		rhs = components.Badge(padTo("NO DRIVER", badgeW), BadgeErr)
	case !available:
		label = DimText.Render(name)
		rhs = components.Badge(padTo("N/A", badgeW), BadgeWarn)
	case enabled:
		label = ValueStyle.Bold(true).Render(name)
		rhs = components.Badge(padTo("ENABLED", badgeW), BadgeOK)
	default:
		label = DimText.Render(name)
		rhs = components.Badge(padTo("DISABLED", badgeW), BadgeAccent)
	}
	prefix := cursor + tag + "  " + label
	return components.LeaderRow(prefix, rhs, width, MutedText)
}

func (s FeaturesScreen) View() string {
	if !s.read {
		s.refresh()
	}
	w := panelWidth(s.app.Width)
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"FEATURE  CONFIGURATION",
		"vayu_defconfig  ·  CONFIG_KSU / SUSFS / KPM",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	st := s.state
	driver := st.DriverPresent

	// ── Toggles panel ───────────────────────────────────────────────────────
	// Separate rows with a blank line so ENABLED / DISABLED badges stacked
	// on adjacent lines don't bleed into a single coloured rectangle
	// (see components/layout.go:BadgeRowGap).
	var p strings.Builder
	p.WriteString(featureRow("1", "ReSukiSU  (CONFIG_KSU)", st.KSU, true, driver, s.focus == 0, innerW) + "\n\n")
	p.WriteString(featureRow("2", "SuSFS     (CONFIG_KSU_SUSFS)", st.SUSFS, st.KSU, driver, s.focus == 1, innerW) + "\n\n")
	p.WriteString(featureRow("3", "KPM       (CONFIG_KPM)", st.KPM, st.KSU, driver, s.focus == 2, innerW))
	togPanel := components.Panel("Toggles", p.String(), w, PanelBorder, TitleStyle)

	// ── Summary panel ───────────────────────────────────────────────────────
	var sm strings.Builder
	sm.WriteString(components.KV("Hook Mode", st.HookMode(), 11, LabelStyle, AccentText) + "\n")
	sm.WriteString(components.KV("Caption", st.CapTag(s.app.Cfg.KSUBranch), 11, LabelStyle, ValueStyle) + "\n")
	sm.WriteString(components.KV("Extras", st.ExtTag(), 11, LabelStyle, ValueStyle) + "\n")
	dc := s.defconfigPath()
	if dc == "" {
		sm.WriteString(components.KV("Defconfig", "(kernel root unresolved)", 11, LabelStyle, ErrText))
	} else {
		sm.WriteString(components.KV("Defconfig", dc, 11, LabelStyle, DimText))
	}
	sumPanel := components.Panel("Active features", sm.String(), w, PanelBorder, TitleStyle)

	// ── Action strip ────────────────────────────────────────────────────────
	actions := components.HotkeyStrip([]components.Hotkey{
		{Key: "1", Desc: "ReSukiSU", Sub: "toggle"},
		{Key: "2", Desc: "SuSFS", Sub: "toggle"},
		{Key: "3", Desc: "KPM", Sub: "toggle"},
		{Key: "M", Desc: "Menuconfig", Sub: "interactive"},
		{Key: "R", Desc: "Reload"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := "  " + components.Separator(innerW, MutedText) + "\n"
	out := banner + "\n" + togPanel + "\n" + sumPanel + "\n"
	if s.app.Activity != nil {
		out += components.ActivityPanel(s.app.Activity, w, 5, PanelDim, TitleStyle.Foreground(ColorDim)) + "\n"
	}
	out += divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}
