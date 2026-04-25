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
type FeaturesScreen struct {
	app   *App
	state features.State
	read  bool
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
			s.app.Toast = "Kernel root not resolved -- run Setup first"
			s.app.ToastErr = true
			return s, nil
		}
		switch strings.ToLower(m.String()) {
		case "1":
			st, err := features.ToggleKSU(dc, s.state)
			s.state = st
			if err != nil {
				s.app.Toast = err.Error()
				s.app.ToastErr = true
			} else if st.KSU {
				s.app.Toast = "ReSukiSU enabled (Manual-Hook on)"
				s.app.ToastErr = false
			} else {
				s.app.Toast = "ReSukiSU disabled (SuSFS + KPM + ManualHook cleared)"
				s.app.ToastErr = false
			}
		case "2":
			st, err := features.ToggleSUSFS(dc, s.state)
			s.state = st
			if err != nil {
				s.app.Toast = err.Error()
				s.app.ToastErr = true
			} else if st.SUSFS {
				s.app.Toast = "SuSFS enabled (SuSFS-Inline-Hook active)"
				s.app.ToastErr = false
			} else {
				s.app.Toast = "SuSFS disabled (Manual-Hook auto-enabled)"
				s.app.ToastErr = false
			}
		case "3":
			st, err := features.ToggleKPM(dc, s.state)
			s.state = st
			if err != nil {
				s.app.Toast = err.Error()
				s.app.ToastErr = true
			} else if st.KPM {
				s.app.Toast = "KPM enabled"
				s.app.ToastErr = false
			} else {
				s.app.Toast = "KPM disabled"
				s.app.ToastErr = false
			}
		case "r":
			s.refresh()
			s.app.Toast = "Defconfig re-read."
			s.app.ToastErr = false
		case "m":
			return s, s.runMenuconfig()
		}
	case menuconfigDoneMsg:
		s.handleMenuconfigDone(m)
		s.refresh()
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
		s.app.Toast = "Kernel/output paths unresolved -- run Setup first"
		s.app.ToastErr = true
		return nil
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
// any pending preserved .config), and surface a toast.
func (s FeaturesScreen) handleMenuconfigDone(m menuconfigDoneMsg) {
	if m.err != nil && m.rc < 0 {
		s.app.Toast = "Menuconfig failed: " + m.err.Error()
		s.app.ToastErr = true
		return
	}
	if m.rc != 0 {
		s.app.Toast = "Menuconfig aborted -- no changes applied"
		s.app.ToastErr = false
		return
	}
	if !m.saved {
		s.app.Toast = "Menuconfig closed without saving -- no changes"
		s.app.ToastErr = false
		return
	}
	s.app.MenuconfigUsed = true
	state.ClearMenuconfigPreserve(s.app.Paths.Kernel)
	s.app.MenuconfigPreserved = false
	s.app.Toast = "Menuconfig saved (Stage 2 defconfig regen skipped this session)"
	s.app.ToastErr = false
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
func featureRow(num, name string, enabled, available, driver bool, width int) string {
	const badgeW = 10
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
	prefix := tag + "  " + label
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
	p.WriteString(featureRow("1", "ReSukiSU  (CONFIG_KSU)", st.KSU, true, driver, innerW) + "\n\n")
	p.WriteString(featureRow("2", "SuSFS     (CONFIG_KSU_SUSFS)", st.SUSFS, st.KSU, driver, innerW) + "\n\n")
	p.WriteString(featureRow("3", "KPM       (CONFIG_KPM)", st.KPM, st.KSU, driver, innerW))
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
	out := banner + "\n" + togPanel + "\n" + sumPanel + "\n" + divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}
