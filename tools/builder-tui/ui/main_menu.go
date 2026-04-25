package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// MainMenu is the entry screen, the strict 1:1 port of the bash
// `run_mode_menu`. Hotkeys: B / [P] / T / M / S / Q. ↑/↓ + Enter also
// navigate; Enter activates the highlighted row. The Features and
// Dependencies sub-screens are reachable only inside the Build flow and the
// Setup wrapper respectively (matching the bash structure), not from the
// top-level menu.
type MainMenu struct {
	app    *App
	cursor int
}

func NewMainMenu(a *App) MainMenu { return MainMenu{app: a} }

// menuActionKey returns the hotkey letter for the row at index i in the
// dynamic menu list (which omits [P] when no image exists).
func (m MainMenu) menuActionKey(i int) string {
	keys := []string{"b"}
	if m.app.HasImage {
		keys = append(keys, "p")
	}
	keys = append(keys, "t", "m", "s", "q")
	if i >= 0 && i < len(keys) {
		return keys[i]
	}
	return ""
}

func (m MainMenu) menuLen() int {
	n := 5 // B + T M S Q
	if m.app.HasImage {
		n++
	}
	return n
}

func (m MainMenu) Init() tea.Cmd { return nil }

// pathsLocked reports whether building should be blocked because either
// Clang or AnyKernel3 is missing.
func (m MainMenu) pathsLocked() bool {
	return m.app.Paths.Clang == "" || m.app.Paths.AnyKernel == ""
}

func (m MainMenu) Update(msg tea.Msg) (MainMenu, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(msg.String())
		// Arrow-key + Enter navigation. j/k vim aliases were dropped here
		// because lowercasing them collided with the M (ReSukiSU) hotkey
		// in the bash original.
		switch key {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down":
			if m.cursor < m.menuLen()-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			key = m.menuActionKey(m.cursor)
		}
		switch key {
		case "b":
			if m.pathsLocked() {
				m.app.Toast = "Cannot build — fix Clang / AnyKernel3 paths in Setup first"
				m.app.ToastErr = true
				return m, nil
			}
			// B drives directly into Build Options; from there [S] starts
			// the linear compile → package pipeline. Feature toggles live
			// behind [F] inside Build Options, not in this linear path.
			m.app.Screen = ScreenBuildOptions
			return m, m.app.buildOpts.Init()
		case "p":
			// Package-only — only valid when Image already exists in out/.
			if !m.app.HasImage {
				m.app.Toast = "No previous Image in out/ — run a full build first"
				m.app.ToastErr = true
				return m, nil
			}
			if m.pathsLocked() {
				m.app.Toast = "Cannot package — AnyKernel3 path missing"
				m.app.ToastErr = true
				return m, nil
			}
			m.app.PackageOnly = true
			m.app.Screen = ScreenBuild
			b, cmd := m.app.build.startBuild()
			m.app.build = b
			return m, cmd
		case "t":
			m.app.Screen = ScreenToolchain
			return m, m.app.toolchain.Init()
		case "m":
			m.app.Screen = ScreenKSU
			return m, m.app.ksu.Init()
		case "s":
			m.app.Screen = ScreenSetup
			return m, m.app.setup.Init()
		}
	}
	return m, nil
}

func (m MainMenu) View() string {
	// Refresh persisted state every time we paint the main menu so the
	// "Previous Build" panel and HasImage gating stay live after returning
	// from a sub-screen (a build, a counter reset, a menuconfig session).
	m.app.refreshPersistence()

	w := panelWidth(m.app.Width)
	innerW := innerContentWidth(w)

	// ── Banner ───────────────────────────────────────────────────────────────
	banner := components.Banner(
		"VAYU  KERNEL  BUILDER",
		"Linux 4.14 NonGKI · Poco X3 Pro (vayu) · Android 16 · ReSukiSU",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Previous Build panel ─────────────────────────────────────────────────
	// Mirrors the bash mode menu's grey "Previous Build" block. Empty body
	// shows a single-line "No previous build recorded" centered.
	var prevBody strings.Builder
	pb := m.app.PrevBuild
	if pb.Empty() {
		prevBody.WriteString(lipgloss.PlaceHorizontal(innerW, lipgloss.Center,
			DimText.Render("No previous build recorded")))
	} else {
		const lblW = 14
		valW := innerW - lblW - 3 // " : "
		if valW < 8 {
			valW = 8
		}
		buildLine := fmt.Sprintf("#%d", pb.Num)
		if pb.Date != "" {
			buildLine = buildLine + "   " + pb.Date
		}
		prevBody.WriteString(components.KVWrap("Build", buildLine, lblW, valW, LabelStyle, DimText))
		if pb.KernelName != "" {
			prevBody.WriteString("\n")
			prevBody.WriteString(components.KVWrap("Kernel-Name", pb.KernelName, lblW, valW, LabelStyle, DimText))
		}
		if pb.Mode != "" {
			prevBody.WriteString("\n")
			prevBody.WriteString(components.KVWrap("Mode", pb.Mode, lblW, valW, LabelStyle, DimText))
		}
		prevBody.WriteString("\n")
		prevBody.WriteString(components.Separator(innerW, MutedText))
		prevBody.WriteString("\n")
		cap := pb.Cap
		if cap == "" {
			cap = "Unknown"
		}
		prevBody.WriteString(components.KVWrap("Capabilities", cap, lblW, valW, LabelStyle, DimText))
		ext := pb.ExtFeat
		if ext == "" {
			ext = "[None]"
		}
		prevBody.WriteString("\n")
		prevBody.WriteString(components.KVWrap("Features", ext, lblW, valW, LabelStyle, DimText))
	}
	prevPanel := components.Panel("Previous Build", prevBody.String(), w, PanelDim, DimText.Bold(true))

	// ── Menu panel ───────────────────────────────────────────────────────────
	type menuItem struct {
		key, label, rhs string
		fg              lipgloss.Color
		blocked         bool
	}
	branchTag := m.app.Builder.KSUBranch
	if branchTag == "" {
		branchTag = "main"
	}
	items := []menuItem{
		{"B", "Full Build", "configure → compile → package", ColorOK, m.pathsLocked()},
	}
	if m.app.HasImage {
		items = append(items, menuItem{"P", "Package Existing Image", "skip compile", ColorOK, m.pathsLocked()})
	}
	items = append(items,
		menuItem{"T", "Toolchain Manager", "fetch/update ZyC Clang", ColorAccent, false},
		menuItem{"M", "ReSukiSU Driver Manager", "branch: " + branchTag, ColorWarn, false},
		menuItem{"S", "Setup", "paths + dependencies", ColorAccent, false},
		menuItem{"Q", "Quit", "", ColorMuted, false},
	)
	var menu strings.Builder
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	maxKey := 0
	for _, it := range items {
		if len(it.key) > maxKey {
			maxKey = len(it.key)
		}
	}
	for i, it := range items {
		labelStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
		rhs := it.rhs
		rhsStyle := lipgloss.NewStyle().Foreground(it.fg)
		if it.blocked {
			labelStyle = lipgloss.NewStyle().Foreground(ColorMuted).Strikethrough(true)
			rhs = "BLOCKED — fix paths first"
			rhsStyle = ErrText
		}
		// 2-char marker before the row so highlighted row gets a chevron.
		mark := "  "
		if i == m.cursor {
			mark = AccentText.Render(" ›")
		}
		row := components.MenuRow(it.key, it.label, rhs, innerW-2, maxKey,
			HotKeyStyle, labelStyle, rhsStyle, leader,
		)
		menu.WriteString(mark + row)
		if i < len(items)-1 {
			menu.WriteString("\n")
		}
	}
	menuPanel := components.Panel("Select Mode", menu.String(), w, PanelBorder, TitleStyle)

	// ── Conditional banners ──────────────────────────────────────────────────
	var extras strings.Builder
	if m.pathsLocked() {
		var msgs []string
		if m.app.Paths.Clang == "" {
			msgs = append(msgs, "[!] Clang not found — use [T] to fetch or [S] to fix path")
		}
		if m.app.Paths.AnyKernel == "" {
			msgs = append(msgs, "[!] AnyKernel3 not found — vendor it or fix path in [S]")
		}
		body := ErrText.Render(strings.Join(msgs, "\n"))
		extras.WriteString(components.Panel("Path Issues", body, w, PanelErr, ErrText))
		extras.WriteString("\n")
	}
	if m.app.MenuconfigPreserved {
		body := lipgloss.PlaceHorizontal(innerW, lipgloss.Center,
			lipgloss.NewStyle().Foreground(ColorBanner).Bold(true).
				Render("Preserved menuconfig .config — will be restored on next build"))
		extras.WriteString(components.Panel("", body, w,
			PanelBorder.BorderForeground(ColorBanner), TitleStyle))
		extras.WriteString("\n")
	}

	// ── Toast + help ─────────────────────────────────────────────────────────
	var toast string
	if m.app.Toast != "" {
		toast = "  " + components.Toast(m.app.Toast, m.app.ToastErr) + "\n"
	}

	// Build the option string dynamically: [B/P/T/M/S/Q]
	opts := []string{"B"}
	if m.app.HasImage {
		opts = append(opts, "P")
	}
	opts = append(opts, "T", "M", "S", "Q")
	help := HelpStyle.Render(fmt.Sprintf(
		"  Select [%s] · esc/q to quit · terminal %dx%d",
		strings.Join(opts, "/"), m.app.Width, m.app.Height,
	))

	return banner + "\n" +
		prevPanel + "\n" +
		menuPanel + "\n" +
		extras.String() +
		toast +
		help
}

func okOr(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

// clampWidth returns the outer panel/banner width to use given the terminal
// columns. We clamp to a minimum so very narrow terminals still produce
// readable output, and to a generous maximum so ultra-wide terminals
// (>200 cols) don't render absurdly stretched dotted leaders. Pass max <= 0
// to disable the upper bound.
func clampWidth(w, min, max int) int {
	if w < min {
		return min
	}
	if max > 0 && w > max {
		return max
	}
	return w
}

// panelWidth returns the outer width used for banners and panels. The
// returned value is the full rendered box width, including the left and
// right border columns; callers pass this directly to components.Banner /
// components.Panel without subtracting anything else.
//
// Behaviour:
//   - On a normal terminal we use the full width with no upper clamp so
//     wide SSH terminals get to use the space (the bash original does the
//     same via `set_width`).
//   - We always reserve a single-column right gutter, so the right border
//     never hugs the very last terminal column — some emulators (notably
//     macOS Terminal and PuTTY) will wrap the next character to a new
//     line if a glyph lands on the final column.
//   - On very narrow screens (mobile SSH) we clamp to 24 to keep the box
//     drawable. Below that the UI degrades to label-only rows.
func panelWidth(termWidth int) int {
	if termWidth <= 0 {
		termWidth = 80
	}
	w := termWidth - 1
	if w < 24 {
		w = 24
	}
	return w
}

// innerContentWidth is the visible content area inside a panel, i.e. the
// number of columns available for body text after lipgloss subtracts both
// borders (2) and the horizontal padding (2 — one column on each side).
// Used by callers that need to size MenuRow leaders, KV padding, etc.
func innerContentWidth(outerWidth int) int {
	w := outerWidth - 4
	if w < 8 {
		w = 8
	}
	return w
}


