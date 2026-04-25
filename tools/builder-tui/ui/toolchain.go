package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/clang"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// ToolchainScreen lets the user pick Google/ZyC/Auto, query upstream, and
// fetch+install a clang toolchain. It also shows the local clang library
// inventory and lets the user switch between installed versions.
type ToolchainScreen struct {
	app       *App
	prog      progress.Model
	busy      bool
	stage     string
	last      string
	inventory []clang.Entry
	invCursor int
}

func NewToolchainScreen(a *App) ToolchainScreen {
	p := progress.New(progress.WithGradient("#5fafff", "#5fffd7"))
	p.Width = 60
	return ToolchainScreen{app: a, prog: p}
}

func (s ToolchainScreen) Init() tea.Cmd {
	return func() tea.Msg { return tcInventoryRefreshMsg{} }
}

// Messages used by the toolchain screen.
type tcQueryDoneMsg struct {
	rel clang.Release
	err error
}
type tcDownloadProgressMsg struct{ done, total int64 }
type tcInstallDoneMsg struct{ err error }
type tcInventoryRefreshMsg struct{}

func (s ToolchainScreen) Update(msg tea.Msg) (ToolchainScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(m.String())
		if s.busy {
			return s, nil
		}
		switch key {
		case "a":
			s.app.Cfg.ClangSource = config.ClangAuto
			s.app.PersistConfig()
		case "g":
			s.app.Cfg.ClangSource = config.ClangGoogle
			s.app.PersistConfig()
		case "z":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.PersistConfig()
		case "l":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "latest"
			s.app.PersistConfig()
		case "1":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "15"
			s.app.PersistConfig()
		case "2":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "23"
			s.app.PersistConfig()
		case "c":
			s.busy = true
			s.stage = "querying upstream"
			return s, s.queryCmd()
		case "f":
			s.busy = true
			s.stage = "querying upstream"
			return s, s.queryThenFetchCmd()
		case "n":
			if len(s.inventory) > 0 {
				s.invCursor = (s.invCursor + 1) % len(s.inventory)
			}
		case "x":
			if len(s.inventory) > 0 {
				tgt := s.inventory[s.invCursor]
				lib := clang.LibraryDir(s.app.Paths.Clang)
				if err := clang.MakeActive(lib, tgt.Dir); err != nil {
					s.app.Toast = "Switch failed: " + err.Error()
					s.app.ToastErr = true
				} else {
					s.app.Toast = "Active clang \u2192 " + tgt.Tag
					s.app.ToastErr = false
				}
				s = s.refreshInventory()
			}
		case "r":
			if len(s.inventory) > 0 {
				tgt := s.inventory[s.invCursor]
				lib := clang.LibraryDir(s.app.Paths.Clang)
				if err := clang.Remove(lib, tgt.Dir); err != nil {
					s.app.Toast = "Remove failed: " + err.Error()
					s.app.ToastErr = true
				} else {
					s.app.Toast = "Removed " + tgt.Tag
					s.app.ToastErr = false
				}
				s = s.refreshInventory()
			}
		}
	case tcQueryDoneMsg:
		s.busy = false
		if m.err != nil {
			msg := m.err.Error()
			if strings.Contains(msg, "rate limited") || strings.Contains(msg, "rate limit") {
				msg += "  (set $ZYC_GH_TOKEN to lift the 60/h limit)"
			}
			s.app.Toast = "Query failed: " + msg
			s.app.ToastErr = true
			return s, nil
		}
		s.last = fmt.Sprintf("%s : %s (%s)", m.rel.Source, m.rel.Tag, sizeStr(m.rel.SizeBytes))
		s.app.Toast = "Latest " + s.last
		s.app.ToastErr = false
	case tcDownloadProgressMsg:
		if m.total > 0 {
			cmd := s.prog.SetPercent(float64(m.done) / float64(m.total))
			return s, cmd
		}
	case tcInstallDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = "Install failed: " + m.err.Error()
			s.app.ToastErr = true
		} else {
			s.app.Toast = "Clang installed at " + s.app.Paths.Clang
			s.app.ToastErr = false
		}
		s = s.refreshInventory()
		return s, nil
	case tcInventoryRefreshMsg:
		s = s.refreshInventory()
		return s, nil
	case progress.FrameMsg:
		var cmd tea.Cmd
		var pm tea.Model
		pm, cmd = s.prog.Update(m)
		s.prog = pm.(progress.Model)
		return s, cmd
	}
	return s, nil
}

func (s ToolchainScreen) View() string {
	w := panelWidth(s.app.Width)
	inner := innerContentWidth(w)

	// ── Banner ───────────────────────────────────────────────────────────────
	banner := components.Banner(
		"TOOLCHAIN  MANAGER",
		"Google AOSP clang  ·  ZyC Clang fallback",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Current state panel ─────────────────────────────────────────────────
	var st strings.Builder
	st.WriteString(components.KV("Source", sourceLabel(s.app.Cfg), 11, LabelStyle, ValueStyle) + "\n")
	st.WriteString(components.KV("Origin", sourceOrigin(s.app.Cfg.ClangSource), 11, LabelStyle, AccentText) + "\n")
	local := clang.LocalVersion(s.app.Paths.Clang)
	if local == "" {
		st.WriteString(components.KV("Local", "(not installed)", 11, LabelStyle, ErrText) + "\n")
	} else {
		st.WriteString(components.KV("Local", local, 11, LabelStyle, OKText) + "\n")
	}
	st.WriteString(components.KV("Install", okOr(s.app.Paths.Clang, "(unset)"), 11, LabelStyle, MutedText))
	if s.last != "" {
		st.WriteString("\n" + components.KV("Latest", s.last, 11, LabelStyle, OKText))
	}
	statePanel := components.Panel("State", st.String(), w, PanelBorder, TitleStyle)

	// ── Source selector panel ───────────────────────────────────────────────
	innerW := inner
	var src strings.Builder
	src.WriteString(srcRow("A", "Auto: Google primary, ZyC fallback", s.app.Cfg.ClangSource == config.ClangAuto, innerW) + "\n")
	src.WriteString(srcRow("G", "Google AOSP clang (android.googlesource.com)", s.app.Cfg.ClangSource == config.ClangGoogle, innerW) + "\n")
	src.WriteString(srcRow("Z", "ZyC Clang (community / GitHub releases)", s.app.Cfg.ClangSource == config.ClangZyC, innerW))
	if s.app.Cfg.ClangSource == config.ClangZyC {
		src.WriteString("\n" + components.Rule("ZyC target", innerW, MutedText) + "\n")
		src.WriteString(srcRow("2", "ZyC Clang 23.x (current)", s.app.Cfg.ZyCTarget == "23", innerW) + "\n")
		src.WriteString(srcRow("1", "ZyC Clang 15.x (legacy)", s.app.Cfg.ZyCTarget == "15", innerW) + "\n")
		src.WriteString(srcRow("L", "Latest (any version)", s.app.Cfg.ZyCTarget == "latest", innerW))
	}
	srcPanel := components.Panel("Source", src.String(), w, PanelBorder, TitleStyle)

	// ── Action strip ────────────────────────────────────────────────────────
	actions := components.HotkeyStrip([]components.Hotkey{
		{Key: "F", Desc: "Fetch", Sub: "versioned install"},
		{Key: "C", Desc: "Check latest", Sub: "query upstream"},
		{Key: "N", Desc: "Next", Sub: "cycle inventory"},
		{Key: "X", Desc: "Switch", Sub: "activate selected"},
		{Key: "R", Desc: "Remove", Sub: "delete selected"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	// Inventory panel: lists installed clang versions side-by-side
	// with the active one and allows the user to switch.
	invPanel := s.renderInventoryPanel(w, innerW)

	divider := "  " + components.Separator(innerContentWidth(w), MutedText) + "\n"
	out := banner + "\n" + statePanel + "\n" + srcPanel + "\n" + invPanel + "\n" + divider + "  " + actions + "\n"

	if s.busy {
		out += "\n  " + AccentText.Render(s.stage+" …") + "\n  " + s.prog.View() + "\n"
	}
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// srcRow renders one source-selector row with a navigation chevron in
// the left gutter for the active selection, [key] right-padded to the
// global bracket column so closing brackets align across pages, and a
// dotted leader pulling the ACTIVE badge to the right edge of the panel.
//
//	▸ [A]  Auto: Google primary, ZyC fallback ················· ACTIVE
//	  [G]  Google AOSP clang (android.googlesource.com)
//	  [Z]  ZyC Clang (community / GitHub releases)
func srcRow(key, label string, selected bool, width int) string {
	arrow := components.NavArrow(selected, SelText)
	tag := components.GlobalBracketTag(key, HotKeyStyle)
	prefix := arrow + tag + "  " +
		lipgloss.NewStyle().Foreground(ColorValue).Render(label)
	if selected {
		return components.LeaderRow(prefix, components.Badge("ACTIVE", BadgeAccent), width, MutedText)
	}
	return prefix
}

func sourceLabel(c config.Config) string {
	switch c.ClangSource {
	case config.ClangGoogle:
		return "Google AOSP clang  (" + c.GoogleTarget + ")"
	case config.ClangZyC:
		return "ZyC Clang  (" + c.ZyCTarget + ".x)"
	default:
		return "Auto: Google → ZyC fallback"
	}
}

func sourceOrigin(s config.ClangSource) string {
	switch s {
	case config.ClangGoogle:
		return "android.googlesource.com (main-kernel)"
	case config.ClangZyC:
		return "github.com/ZyCromerZ/Clang"
	default:
		return "auto: AOSP gitiles + ZyC GitHub"
	}
}

func sizeStr(b int64) string {
	if b <= 0 {
		return "?"
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("%d MB", (b+mb-1)/mb)
}

func (s ToolchainScreen) queryCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var src clang.Source
		var target string
		switch s.app.Cfg.ClangSource {
		case config.ClangGoogle:
			src = &clang.Google{}
			target = s.app.Cfg.GoogleTarget
		case config.ClangZyC:
			src = &clang.ZyC{}
			target = s.app.Cfg.ZyCTarget
		default:
			src = &clang.Auto{}
			target = s.app.Cfg.GoogleTarget
		}
		rel, err := src.Latest(ctx, target)
		return tcQueryDoneMsg{rel: rel, err: err}
	}
}

func (s ToolchainScreen) queryThenFetchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var src clang.Source
		var target string
		switch s.app.Cfg.ClangSource {
		case config.ClangGoogle:
			src = &clang.Google{}
			target = s.app.Cfg.GoogleTarget
		case config.ClangZyC:
			src = &clang.ZyC{}
			target = s.app.Cfg.ZyCTarget
		default:
			src = &clang.Auto{}
			target = s.app.Cfg.GoogleTarget
		}
		rel, err := src.Latest(ctx, target)
		if err != nil {
			return tcInstallDoneMsg{err: err}
		}
		dest := s.app.Paths.Clang
		if dest == "" && s.app.Paths.Kernel != "" {
			dest = s.app.Paths.Kernel + "/clang"
		}
		if dest == "" {
			return tcInstallDoneMsg{err: fmt.Errorf("install dir unknown (configure Setup > Paths first)")}
		}
		// Pump bytes-downloaded progress back into the tea event loop via
		// Program.Send so the bubbles progress.Model animates in real time.
		var lastSent time.Time
		cb := clang.ProgressFunc(func(done, total int64) {
			if Program == nil {
				return
			}
			if time.Since(lastSent) < 100*time.Millisecond && done < total {
				return
			}
			lastSent = time.Now()
			Program.Send(tcDownloadProgressMsg{done: done, total: total})
		})
		// Multi-version install: drop the new build into a versioned
		// inventory directory next to the active slot, then atomically
		// flip the active symlink. This means the previous version is
		// still on disk (visible in the [N]ext list) and the user can
		// roll back at any time.
		lib := clang.LibraryDir(dest)
		if lib == "" {
			err = clang.Install(ctx, rel, dest, cb)
			return tcInstallDoneMsg{err: err}
		}
		installed, err := clang.InstallVersioned(ctx, rel, lib, cb)
		if err != nil {
			return tcInstallDoneMsg{err: err}
		}
		if err := clang.MakeActive(lib, installed); err != nil {
			return tcInstallDoneMsg{err: fmt.Errorf("activate %s: %w", installed, err)}
		}
		return tcInstallDoneMsg{}
	}
}

// refreshInventory rescans the library directory next to Paths.Clang
// and returns a new screen value with the updated inventory list.
func (s ToolchainScreen) refreshInventory() ToolchainScreen {
	lib := clang.LibraryDir(s.app.Paths.Clang)
	if lib == "" {
		s.inventory = nil
		s.invCursor = 0
		return s
	}
	inv, err := clang.Inventory(lib, s.app.Paths.Clang)
	if err != nil {
		s.inventory = nil
		s.invCursor = 0
		return s
	}
	s.inventory = inv
	if s.invCursor >= len(inv) {
		s.invCursor = 0
	}
	return s
}

// renderInventoryPanel paints the local clang library list. When the
// library directory is empty or unset, it renders a friendly empty-state
// notice instead.
func (s ToolchainScreen) renderInventoryPanel(w, innerW int) string {
	lib := clang.LibraryDir(s.app.Paths.Clang)
	if lib == "" {
		body := DimText.Render("(set Setup → Clang path first)")
		return components.Panel("Library", body, w, PanelDim, TitleStyle.Foreground(ColorDim))
	}
	if len(s.inventory) == 0 {
		body := DimText.Render("No additional clang versions installed.\n" +
			"Use [F] to fetch a new version — it will be saved alongside the active install.")
		return components.Panel("Library  ·  "+lib, body, w, PanelDim, TitleStyle.Foreground(ColorDim))
	}
	var b strings.Builder
	for i, e := range s.inventory {
		b.WriteString(invRow(i == s.invCursor, e, innerW))
		if i < len(s.inventory)-1 {
			b.WriteString("\n")
		}
	}
	return components.Panel("Library  ·  "+lib, b.String(), w, PanelBorder, TitleStyle)
}

// invRow renders one library entry: ▸ tag · source ·········· ACTIVE
func invRow(cursor bool, e clang.Entry, width int) string {
	arrow := components.NavArrow(cursor, SelText)
	srcLabel := e.Source
	if srcLabel == "" {
		srcLabel = "unknown"
	}
	prefix := arrow + ValueStyle.Render(e.Tag) + "  " +
		DimText.Render("("+srcLabel+")")
	var rhs string
	if e.Active {
		rhs = components.Badge("ACTIVE", BadgeOK)
	} else {
		rhs = DimText.Render("idle")
	}
	return components.LeaderRow(prefix, rhs, width, MutedText)
}
