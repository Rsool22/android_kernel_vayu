package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/resukisu"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// KSUScreen surfaces ReSukiSU branch availability and install/update actions.
// Branch probes run on entry and on demand (key 'p').
type KSUScreen struct {
	app    *App
	main   resukisu.Probe
	dev    resukisu.Probe
	probed bool
	busy   bool
	stage  string
}

func NewKSUScreen(a *App) KSUScreen { return KSUScreen{app: a} }

func (s KSUScreen) Init() tea.Cmd { return s.probeCmd() }

type ksuProbedMsg struct{ main, dev resukisu.Probe }
type ksuActionDoneMsg struct {
	stage string
	err   error
}
type ksuGuardDoneMsg struct {
	report pipeline.GuardReport
	err    error
}
type ksuRemoveDoneMsg struct {
	rc       int
	cFiles   int
	err      error
	dirGone  bool
	guardRep pipeline.GuardReport
}

func (s KSUScreen) probeCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		m, d := resukisu.ProbeAll(ctx)
		return ksuProbedMsg{main: m, dev: d}
	}
}

func (s KSUScreen) Update(msg tea.Msg) (KSUScreen, tea.Cmd) {
	switch m := msg.(type) {
	case ksuProbedMsg:
		s.main = m.main
		s.dev = m.dev
		s.probed = true
		s.busy = false
		s.app.Toast = "Upstream probed."
		s.app.ToastErr = false
		return s, nil
	case ksuActionDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = m.stage + " failed: " + m.err.Error()
			s.app.ToastErr = true
		} else {
			s.app.Toast = m.stage + " complete."
			s.app.ToastErr = false
		}
		return s, nil
	case tea.KeyMsg:
		if s.busy {
			return s, nil
		}
		switch strings.ToLower(m.String()) {
		case "p":
			s.busy = true
			s.stage = "probing upstream"
			return s, s.probeCmd()
		case "s":
			if s.app.Cfg.KSUBranch == "main" {
				s.app.Cfg.KSUBranch = "dev"
			} else {
				s.app.Cfg.KSUBranch = "main"
			}
			s.app.PersistConfig()
		case "i", "u":
			selected := s.app.Cfg.KSUBranch
			brstate := s.branchState(selected)
			if brstate != resukisu.StatePresent {
				s.app.Toast = "Cannot install: " + selected + " branch is " + string(brstate)
				s.app.ToastErr = true
				return s, nil
			}
			s.busy = true
			s.stage = "installing " + selected
			return s, s.installCmd(selected)
		case "v":
			s.busy = true
			s.stage = "verifying KSU hook guards"
			return s, s.guardCmd()
		case "x":
			s.busy = true
			s.stage = "removing driver (setup.sh --cleanup)"
			return s, s.removeCmd()
		}
	case ksuGuardDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = "Guards: " + m.err.Error()
			s.app.ToastErr = true
		} else if m.report.Summary != "" {
			s.app.Toast = "Guards: " + m.report.Summary
			s.app.ToastErr = m.report.HasFixes()
		} else {
			s.app.Toast = "Guards verified"
			s.app.ToastErr = false
		}
		return s, nil
	case ksuRemoveDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.app.Toast = "Remove failed: " + m.err.Error()
			s.app.ToastErr = true
			return s, nil
		}
		if m.rc == 0 && m.dirGone {
			s.disableKSUInDefconfig()
			s.app.Builder.ForceCleanReason = "Driver removed"
			s.app.Builder.Incremental = false
			_ = s.app.Builder.Save(s.app.Paths.Kernel)
			s.app.Toast = "Driver removed -- defconfig reset"
			s.app.ToastErr = false
		} else {
			s.app.Toast = "Cleanup failed or driver already gone (exit " + itoa(m.rc) + ")"
			s.app.ToastErr = true
		}
		return s, nil
	}
	return s, nil
}

// disableKSUInDefconfig sets CONFIG_KSU/SUSFS/MANUAL_HOOK/KPM all to "is not
// set" in arch/arm64/configs/vayu_defconfig (mirrors bash post-removal).
func (s KSUScreen) disableKSUInDefconfig() {
	if s.app.Paths.Kernel == "" {
		return
	}
	dc := filepath.Join(s.app.Paths.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
	st, err := features.Read(dc)
	if err != nil {
		return
	}
	st.KSU = false
	st.SUSFS = false
	st.KPM = false
	st.ManualHook = false
	_ = features.Apply(dc, st)
}

// guardCmd runs apply_ksu_guards.py and parses the output.
func (s KSUScreen) guardCmd() tea.Cmd {
	return func() tea.Msg {
		guardPath := filepath.Join(s.app.Paths.Kernel, "scripts", "apply_ksu_guards.py")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "python3", guardPath)
		cmd.Dir = s.app.Paths.Kernel
		cmd.Env = append(cmd.Env, "TERM_W=9999", "KERNEL_DIR="+s.app.Paths.Kernel)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return ksuGuardDoneMsg{err: err}
		}
		return ksuGuardDoneMsg{report: pipeline.ParseGuardReport(string(out))}
	}
}

// removeCmd runs setup.sh --cleanup and verifies driver removal.
func (s KSUScreen) removeCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		rc, _, err := resukisu.Cleanup(ctx, s.app.Paths.Kernel, s.app.Paths.Output, nil)
		if err != nil {
			return ksuRemoveDoneMsg{rc: rc, err: err}
		}
		ksuDir := filepath.Join(s.app.Paths.Kernel, "drivers", "kernelsu")
		gone := !pathExists(ksuDir)
		// Run guard verification post-removal (expect mostly skips).
		var rep pipeline.GuardReport
		guardPath := filepath.Join(s.app.Paths.Kernel, "scripts", "apply_ksu_guards.py")
		if pathExists(guardPath) {
			gctx, gcancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer gcancel()
			gc := exec.CommandContext(gctx, "python3", guardPath)
			gc.Dir = s.app.Paths.Kernel
			gc.Env = append(gc.Env, "TERM_W=9999", "KERNEL_DIR="+s.app.Paths.Kernel)
			out, _ := gc.CombinedOutput()
			rep = pipeline.ParseGuardReport(string(out))
		}
		return ksuRemoveDoneMsg{rc: rc, dirGone: gone, guardRep: rep}
	}
}

func (s KSUScreen) branchState(b string) resukisu.State {
	switch b {
	case "main":
		return s.main.State
	case "dev":
		return s.dev.State
	}
	return resukisu.StateAbsent
}

func (s KSUScreen) installCmd(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := resukisu.Install(ctx, s.app.Paths.Kernel, branch, nil)
		return ksuActionDoneMsg{stage: "Install (" + branch + ")", err: err}
	}
}

func (s KSUScreen) View() string {
	w := panelWidth(s.app.Width)

	banner := components.Banner(
		"ReSukiSU  DRIVER  MANAGER",
		"github.com/ReSukiSU/ReSukiSU  ·  branch probing",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Active selection ────────────────────────────────────────────────────
	activeBadge := components.Badge(strings.ToUpper(s.app.Cfg.KSUBranch), BadgeAccent)
	var sel strings.Builder
	sel.WriteString(components.KV("Active", activeBadge, 9, LabelStyle, ValueStyle) + "\n")
	sel.WriteString(components.KV("Target", s.app.Paths.Kernel+"/drivers/kernelsu", 9, LabelStyle, AccentText))
	selPanel := components.Panel("Selection", sel.String(), w, PanelBorder, TitleStyle)

	// ── Upstream branch states ──────────────────────────────────────────────
	// Right-pad branch label to the widest value so badges align in a column.
	branchW := components.GlobalKeyWidth // shares the global bracket column with [main]/[dev]
	innerW := innerContentWidth(w)
	var ups strings.Builder
	ups.WriteString(probeRow("main", branchW, s.main, s.probed, innerW, s.app.Cfg.KSUBranch == "main") + "\n")
	ups.WriteString(probeRow("dev", branchW, s.dev, s.probed, innerW, s.app.Cfg.KSUBranch == "dev"))
	upsPanel := components.Panel("Upstream branches", ups.String(), w, PanelBorder, TitleStyle)

	// ── Action strip ────────────────────────────────────────────────────────
	actions := components.HotkeyStrip([]components.Hotkey{
		{Key: "I", Desc: "Install / update", Sub: "from active branch"},
		{Key: "S", Desc: "Switch", Sub: "main ↔ dev"},
		{Key: "V", Desc: "Verify guards"},
		{Key: "X", Desc: "Remove driver"},
		{Key: "P", Desc: "Re-probe"},
		{Key: "ESC", Desc: "Back"},
	}, HotKeyStyle, ValueStyle, DimText, MutedText)

	divider := "  " + components.Separator(innerContentWidth(w), MutedText) + "\n"
	out := banner + "\n" + selPanel + "\n" + upsPanel + "\n" + divider + "  " + actions + "\n"

	if s.busy {
		out += "\n  " + AccentText.Render(s.stage+" …") + "\n"
	}
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// pathExists returns true when p exists (file or dir).
func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// itoa returns the decimal string representation of i.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// padTo right-pads a string with spaces to a fixed visible width.
func padTo(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// probeRow renders a single branch probe row with all columns aligned
// regardless of branch label or status length:
//
//	▸ [main]  PRESENT  ····························· HEAD@7adffacb
//	  [dev ]  ABSENT   ······· branch removed upstream
//	  [foo ]  NETWORK  ······· could not reach upstream
//
// branchW pads the bracketed branch label to a fixed width (shared
// global so [main]/[dev]/[ESC] all close at the same column). The
// badge column is padded to 7 chars (max of "present"/"absent"/"network").
// active=true draws a navigation chevron in the left gutter so the
// currently selected branch is visually distinct from the bullet styles
// used elsewhere.
func probeRow(branch string, branchW int, p resukisu.Probe, probed bool, width int, active bool) string {
	const badgeW = 7
	tag := components.BracketTag(strings.TrimSpace(branch), branchW, HotKeyStyle)
	arrow := components.NavArrow(active, SelText)
	left := arrow + tag + "  "
	if !probed {
		prefix := left + components.Badge(padTo("PROBE", badgeW), BadgeAccent)
		return components.LeaderRow(prefix, DimText.Render("(probing …)"), width, MutedText)
	}
	var badge, detail string
	switch p.State {
	case resukisu.StatePresent:
		short := p.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		badge = components.Badge(padTo("PRESENT", badgeW), BadgeOK)
		detail = DimText.Render("HEAD@") + AccentText.Bold(true).Render(short)
	case resukisu.StateAbsent:
		badge = components.Badge(padTo("ABSENT", badgeW), BadgeWarn)
		detail = DimText.Render("branch removed or merged upstream")
	case resukisu.StateNetworkFail:
		badge = components.Badge(padTo("NETWORK", badgeW), BadgeErr)
		detail = DimText.Render("could not reach upstream — check connection")
	default:
		return ""
	}
	return components.LeaderRow(left+badge, detail, width, MutedText)
}
