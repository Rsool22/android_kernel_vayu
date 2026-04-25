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

func (s ToolchainScreen) Init() tea.Cmd {
	// ZyC is the default, but Google AOSP clang is also offered now;
	// preserve any previously-saved Google selection. Migrate the legacy
	// "Auto" mode to ZyC silently since the bash original only supports
	// the two concrete vendors.
	if s.app.Cfg.ClangSource == config.ClangAuto {
		s.app.Cfg.ClangSource = config.ClangZyC
		_ = s.app.Cfg.Save()
	}
	if s.app.Cfg.ZyCTarget == "" {
		s.app.Cfg.ZyCTarget = "23"
		_ = s.app.Cfg.Save()
	}
	if s.app.Cfg.GoogleTarget == "" {
		s.app.Cfg.GoogleTarget = "latest"
		_ = s.app.Cfg.Save()
	}
	return nil
}

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
	source config.ClangSource
	zycTag string // e.g. "23", "15", "latest" — only used when source==ClangZyC
}

// sourceItems returns the rows currently rendered in the Source panel,
// in display order. Two vendors are exposed: Google AOSP (single row) and
// ZyC Clang (three version slots — 23.x / 15.x / latest).
func (s ToolchainScreen) sourceItems() []sourceItem {
	return []sourceItem{
		{hotkey: "G", desc: "Google AOSP Clang  (main-kernel/clang)", source: config.ClangGoogle},
		{hotkey: "2", desc: "ZyC Clang 23.x  (current)", source: config.ClangZyC, zycTag: "23"},
		{hotkey: "1", desc: "ZyC Clang 15.x  (legacy)", source: config.ClangZyC, zycTag: "15"},
		{hotkey: "L", desc: "ZyC Clang latest  (any version)", source: config.ClangZyC, zycTag: "latest"},
	}
}

// applySource picks the row at cursor and persists it.
func (s *ToolchainScreen) applySource(it sourceItem) {
	s.app.Cfg.ClangSource = it.source
	if it.source == config.ClangZyC && it.zycTag != "" {
		s.app.Cfg.ZyCTarget = it.zycTag
	}
	s.app.PersistConfig()
	s.log.OK("target → " + sourceLabel(s.app.Cfg))
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
		case "g":
			s.applySource(sourceItem{source: config.ClangGoogle})
			s.cursor = 0
		case "2":
			s.applySource(sourceItem{source: config.ClangZyC, zycTag: "23"})
			s.cursor = 1
		case "1":
			s.applySource(sourceItem{source: config.ClangZyC, zycTag: "15"})
			s.cursor = 2
		case "l":
			s.applySource(sourceItem{source: config.ClangZyC, zycTag: "latest"})
			s.cursor = 3
		case "r", "b":
			s.app.Screen = ScreenMain
			return s, nil
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
		"github.com/ZyCromerZ/Clang  ·  Linux 4.14 NonGKI build",
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
		var selected bool
		switch it.source {
		case config.ClangGoogle:
			selected = s.app.Cfg.ClangSource == config.ClangGoogle
		case config.ClangZyC:
			selected = s.app.Cfg.ClangSource == config.ClangZyC && s.app.Cfg.ZyCTarget == it.zycTag
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

	// ── Actions panel ───────────────────────────────────────────────────────
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	labelSt := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	var act strings.Builder
	// "  " prefix keeps the [F/C/R] bracket column aligned with the rest
	// of the script (Source rows, Build Options, Main Menu) so brackets
	// line up vertically across pages, not just within one panel.
	mw := inner - 2
	act.WriteString("  " + components.MenuRow("F", "Fetch",
		"download + install selected target", mw, 1,
		HotKeyStyle, labelSt, MutedText, leader) + "\n")
	act.WriteString("  " + components.MenuRow("C", "Check Latest",
		"query upstream, no install", mw, 1,
		HotKeyStyle, labelSt, MutedText, leader) + "\n")
	act.WriteString("  " + components.MenuRow("R", "Return", "to Mode Select", mw, 1,
		HotKeyStyle, labelSt, MutedText, leader))
	actPanel := components.Panel("Actions", act.String(), w, PanelBorder, TitleStyle)

	out := strings.Join([]string{
		banner, statePanel, srcPanel, actPanel, logPanel,
		"  " + HelpStyle.Render("Select [G/2/1/L/F/C/R]\u00a0\u00b7\u00a0esc to return"),
	}, "\n")
	return strings.TrimRight(out, "\n ")
}

// srcRow renders one source-selector row. selected = persisted active source;
// hovered = cursor position. They render distinctly: selected gets the
// `●` dot, hovered gets the `›` chevron, and a row that is both shows
// both markers (`›●`) so the navigation arrow stays visible even when
// it's sitting on the active source. The label is dot-padded out to the
// panel's inner width so an ACTIVE pill sits flush against the right
// edge — matching the dot-leader pattern used by the main menu and
// Build Options rows.
func srcRow(key, label string, selected, hovered bool, width int) string {
	// Two-char marker column so the [X] bracket sits at content
	// column 2 across every page. Slot 0 holds the selection dot
	// `●`, slot 1 holds the cursor chevron `›` (immediately left of
	// the bracket, matching the main menu pattern). When the cursor
	// is sitting on the active source, both are shown.
	a, b := " ", " "
	if selected {
		a = SelText.Render("●")
	}
	if hovered {
		b = AccentText.Render("›")
	}
	mark := a + b
	tag := components.BracketTag(key, 1, HotKeyStyle)
	prefix := mark + tag + "  " +
		lipgloss.NewStyle().Foreground(ColorValue).Render(label)
	rhs := ""
	if selected {
		rhs = components.Badge("ACTIVE", BadgeAccent)
	}
	if rhs == "" {
		return prefix
	}
	pad := width - lipgloss.Width(prefix) - lipgloss.Width(rhs)
	if pad < 3 {
		return prefix + "  " + rhs
	}
	return prefix + components.DotLeader(pad, MutedText) + rhs
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
