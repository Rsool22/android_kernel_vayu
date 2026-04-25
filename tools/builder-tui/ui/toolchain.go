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
// fetch+install a clang toolchain.
//
// The screen exposes a unified source-row list that can be navigated by
// hotkey OR arrow keys. Operation progress (download bytes, query results,
// install status) is appended to the activity log panel — replacing the old
// transient toast that was prone to duplicate / stale rendering.
type ToolchainScreen struct {
	app   *App
	prog  progress.Model
	busy  bool
	stage string

	// Cursor position within the source list (sources + ZyC sub-target rows
	// when ZyC is the active source). Driven by ↑/↓ keys.
	cursor int

	// Most-recent download progress for the activity log "X of Y" line.
	dlBytes int64
	dlTotal int64
	dlAt    time.Time

	log *components.Activity
}

func NewToolchainScreen(a *App) ToolchainScreen {
	p := progress.New(progress.WithGradient("#5fafff", "#5fffd7"))
	p.Width = 60
	return ToolchainScreen{
		app:  a,
		prog: p,
		log:  components.NewActivity(),
	}
}

func (s ToolchainScreen) Init() tea.Cmd { return nil }

// Messages used by the toolchain screen.
type tcQueryDoneMsg struct {
	rel clang.Release
	err error
}
type tcDownloadProgressMsg struct{ done, total int64 }
type tcInstallDoneMsg struct{ err error }

// sourceItem describes one row in the navigable source list.
type sourceItem struct {
	hotkey string
	desc   string
	isZyC  bool   // true = a ZyC-target sub-row, only enabled when ZyC active
	zycTag string // e.g. "23", "15", "latest" — only when isZyC
}

// sourceItems returns the rows currently rendered in the Source panel,
// in display order. The list shrinks/grows depending on whether ZyC is
// the active root selection (its sub-targets are only listed then).
func (s ToolchainScreen) sourceItems() []sourceItem {
	out := []sourceItem{
		{hotkey: "A", desc: "Auto: Google primary, ZyC fallback"},
		{hotkey: "G", desc: "Google AOSP clang (android.googlesource.com)"},
		{hotkey: "Z", desc: "ZyC Clang (community / GitHub releases)"},
	}
	if s.app.Cfg.ClangSource == config.ClangZyC {
		out = append(out,
			sourceItem{hotkey: "2", desc: "ZyC Clang 23.x (current)", isZyC: true, zycTag: "23"},
			sourceItem{hotkey: "1", desc: "ZyC Clang 15.x (legacy)", isZyC: true, zycTag: "15"},
			sourceItem{hotkey: "L", desc: "Latest (any version)", isZyC: true, zycTag: "latest"},
		)
	}
	return out
}

// applySource picks the source row at cursor and persists it.
func (s *ToolchainScreen) applySource(it sourceItem) {
	switch it.hotkey {
	case "A":
		s.app.Cfg.ClangSource = config.ClangAuto
	case "G":
		s.app.Cfg.ClangSource = config.ClangGoogle
	case "Z":
		s.app.Cfg.ClangSource = config.ClangZyC
	}
	if it.isZyC {
		s.app.Cfg.ClangSource = config.ClangZyC
		s.app.Cfg.ZyCTarget = it.zycTag
	}
	s.app.PersistConfig()
	s.log.OK("source → " + sourceLabel(s.app.Cfg))
}

func (s ToolchainScreen) Update(msg tea.Msg) (ToolchainScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(m.String())
		if s.busy {
			return s, nil
		}
		items := s.sourceItems()
		switch key {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil
		case "down", "j":
			if s.cursor < len(items)-1 {
				s.cursor++
			}
			return s, nil
		case "enter", " ":
			if s.cursor >= 0 && s.cursor < len(items) {
				it := items[s.cursor]
				s.applySource(it)
			}
			return s, nil
		case "a":
			s.app.Cfg.ClangSource = config.ClangAuto
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
			s.cursor = 0
		case "g":
			s.app.Cfg.ClangSource = config.ClangGoogle
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
			s.cursor = 1
		case "z":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
			s.cursor = 2
		case "l":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "latest"
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
		case "1":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "15"
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
		case "2":
			s.app.Cfg.ClangSource = config.ClangZyC
			s.app.Cfg.ZyCTarget = "23"
			s.app.PersistConfig()
			s.log.OK("source → " + sourceLabel(s.app.Cfg))
		case "c":
			s.busy = true
			s.stage = "querying upstream"
			s.log.Info("querying " + sourceLabel(s.app.Cfg) + " …")
			return s, s.queryCmd()
		case "f":
			s.busy = true
			s.stage = "querying upstream"
			s.dlBytes, s.dlTotal = 0, 0
			s.log.Info("fetch: querying " + sourceLabel(s.app.Cfg) + " …")
			return s, s.queryThenFetchCmd()
		}
	case tcQueryDoneMsg:
		s.busy = false
		if m.err != nil {
			msg := m.err.Error()
			if strings.Contains(msg, "rate limited") || strings.Contains(msg, "rate limit") {
				msg += "  (set $ZYC_GH_TOKEN to lift the 60/h limit)"
			}
			s.log.Err("query failed: " + msg)
			return s, nil
		}
		s.log.OK(fmt.Sprintf("latest %s : %s (%s)", m.rel.Source, m.rel.Tag, sizeStr(m.rel.SizeBytes)))
	case tcDownloadProgressMsg:
		s.dlBytes = m.done
		s.dlTotal = m.total
		s.dlAt = time.Now()
		// Log a progress line at most once per second so the panel doesn't
		// fill up; ActProgress replaces the previous progress line in place.
		if m.total > 0 {
			pct := float64(m.done) / float64(m.total)
			s.log.Progress(fmt.Sprintf("downloading: %s / %s  (%d%%)",
				humanBytes(m.done), humanBytes(m.total), int(pct*100)))
			cmd := s.prog.SetPercent(pct)
			return s, cmd
		}
		// Indeterminate (chunked) mode — surface bytes downloaded.
		s.log.Progress(fmt.Sprintf("downloading: %s (size unknown)", humanBytes(m.done)))
		return s, nil
	case tcInstallDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.log.Err("install failed: " + m.err.Error())
		} else {
			s.log.OK("clang installed at " + s.app.Paths.Clang)
		}
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
	valW := inner - 13 // 11 (label col) + " : "
	var st strings.Builder
	st.WriteString(components.KVWrap("Source", sourceLabel(s.app.Cfg), 11, valW, LabelStyle, ValueStyle) + "\n")
	st.WriteString(components.KVWrap("Origin", sourceOrigin(s.app.Cfg.ClangSource), 11, valW, LabelStyle, AccentText) + "\n")
	local := clang.LocalVersion(s.app.Paths.Clang)
	if local == "" {
		st.WriteString(components.KVWrap("Local", "(not installed)", 11, valW, LabelStyle, ErrText) + "\n")
	} else {
		st.WriteString(components.KVWrap("Local", local, 11, valW, LabelStyle, OKText) + "\n")
	}
	st.WriteString(components.KVWrap("Install", okOr(s.app.Paths.Clang, "(unset)"), 11, valW, LabelStyle, MutedText))
	statePanel := components.Panel("State", st.String(), w, PanelBorder, TitleStyle)

	// ── Source selector panel (cursor-navigable) ────────────────────────────
	items := s.sourceItems()
	if s.cursor >= len(items) {
		s.cursor = len(items) - 1
	}
	var src strings.Builder
	for i, it := range items {
		// Insert a sub-rule before the first ZyC sub-target row.
		if it.isZyC && (i == 0 || !items[i-1].isZyC) {
			src.WriteString(components.Rule("ZyC target", inner, MutedText) + "\n")
		}
		selected := false
		if it.isZyC {
			selected = s.app.Cfg.ClangSource == config.ClangZyC && s.app.Cfg.ZyCTarget == it.zycTag
		} else {
			switch it.hotkey {
			case "A":
				selected = s.app.Cfg.ClangSource == config.ClangAuto
			case "G":
				selected = s.app.Cfg.ClangSource == config.ClangGoogle
			case "Z":
				selected = s.app.Cfg.ClangSource == config.ClangZyC
			}
		}
		src.WriteString(srcRow(it.hotkey, it.desc, selected, i == s.cursor, inner) + "\n")
	}
	srcPanel := components.Panel("Source", strings.TrimRight(src.String(), "\n"),
		w, PanelBorder, TitleStyle)

	// ── Activity log ────────────────────────────────────────────────────────
	logBody := s.log.Render(8)
	if s.busy {
		logBody += "\n" + AccentText.Render(s.stage+" …")
		if s.dlBytes > 0 && s.dlTotal > 0 {
			logBody += "\n  " + s.prog.View()
		}
	}
	logPanel := components.Panel("Activity", logBody, w, PanelBorder, TitleStyle)

	// ── Action strip + ↑/↓/Enter hint ──────────────────────────────────────
	actions := components.HotkeyStripWrap([]components.Hotkey{
		{Key: "F", Desc: "Fetch", Sub: "check + download"},
		{Key: "C", Desc: "Check latest", Sub: "query upstream"},
		{Key: "↑/↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Apply"},
		{Key: "ESC", Desc: "Back"},
	}, stripWidth(s.app.Width), HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := components.Separator(w, MutedText) + "\n"
	return banner + "\n" + statePanel + "\n" + srcPanel + "\n" + logPanel + "\n" +
		divider + "  " + actions + "\n"
}

// srcRow renders one source-selector row. selected = persisted active source;
// hovered = cursor position. They render distinctly: selected gets the dot
// marker + ACTIVE pill; hovered gets a `›` left chevron.
func srcRow(key, label string, selected, hovered bool, width int) string {
	mark := "  "
	if hovered {
		mark = AccentText.Render(" ›")
	}
	if selected {
		mark = SelText.Render(" ●")
	}
	tag := components.BracketTag(key, 1, HotKeyStyle)
	row := mark + " " + tag + "  " +
		lipgloss.NewStyle().Foreground(ColorValue).Render(label)
	if selected {
		row += "  " + components.Badge("ACTIVE", BadgeAccent)
	}
	return row
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
	return humanBytes(b)
}

// humanBytes formats a byte count as a short MiB / GiB string.
func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kb))
	}
	return fmt.Sprintf("%d B", b)
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
			if time.Since(lastSent) < 100*time.Millisecond && (total <= 0 || done < total) {
				return
			}
			lastSent = time.Now()
			Program.Send(tcDownloadProgressMsg{done: done, total: total})
		})
		err = clang.Install(ctx, rel, dest, cb)
		return tcInstallDoneMsg{err: err}
	}
}
