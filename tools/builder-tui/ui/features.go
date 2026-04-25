package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
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
		}
	}
	return s, nil
}

// featureRow renders one feature row with [N] tag, label, and a status
// badge. All four badge variants (ENABLED/DISABLED/N/A/NO DRIVER) are
// padded to a fixed width so they line up across rows.
func featureRow(num, name string, enabled, available, driver bool) string {
	const badgeW = 10
	tag := components.BracketTag(num, 1, HotKeyStyle)
	switch {
	case !driver:
		return "  " + tag + "  " +
			ValueStyle.Render(name) + "  " +
			components.Badge(padTo("NO DRIVER", badgeW), BadgeErr)
	case !available:
		return "  " + tag + "  " +
			DimText.Render(name) + "  " +
			components.Badge(padTo("N/A", badgeW), BadgeWarn)
	case enabled:
		return "  " + tag + "  " +
			ValueStyle.Bold(true).Render(name) + "  " +
			components.Badge(padTo("ENABLED", badgeW), BadgeOK)
	default:
		return "  " + tag + "  " +
			DimText.Render(name) + "  " +
			components.Badge(padTo("DISABLED", badgeW), BadgeAccent)
	}
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
	var p strings.Builder
	p.WriteString(featureRow("1", "ReSukiSU  (CONFIG_KSU)", st.KSU, true, driver) + "\n")
	p.WriteString(featureRow("2", "SuSFS     (CONFIG_KSU_SUSFS)", st.SUSFS, st.KSU, driver) + "\n")
	p.WriteString(featureRow("3", "KPM       (CONFIG_KPM)", st.KPM, st.KSU, driver))
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
		{Key: "R", Desc: "Reload", Sub: "re-read defconfig"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := "  " + components.Separator(innerW, MutedText) + "\n"
	out := banner + "\n" + togPanel + "\n" + sumPanel + "\n" + divider + "  " + actions + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}
