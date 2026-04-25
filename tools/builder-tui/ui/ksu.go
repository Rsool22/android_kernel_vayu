package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/resukisu"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// KSUScreen surfaces ReSukiSU branch availability and install/update actions.
// Branch probes run on entry and on demand (key 'p').
//
// All operation feedback (probe, install, verify, remove) goes into the live
// activity log panel rendered above the action strip — no more transient
// one-line toasts that overwrite each other or get re-printed.
type KSUScreen struct {
	app    *App
	main   resukisu.Probe
	dev    resukisu.Probe
	probed bool
	busy   bool
	stage  string
	log    *components.Activity
}

func NewKSUScreen(a *App) KSUScreen {
	return KSUScreen{app: a, log: components.NewActivity()}
}

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
		s.log.OK("upstream probed: main=" + string(m.main.State) + " dev=" + string(m.dev.State))
		return s, nil
	case ksuActionDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.log.Err(m.stage + " failed: " + m.err.Error())
		} else {
			s.log.OK(m.stage + " complete")
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
			s.log.Info("probing ReSukiSU branches …")
			return s, s.probeCmd()
		case "s":
			if s.app.Cfg.KSUBranch == "main" {
				s.app.Cfg.KSUBranch = "dev"
			} else {
				s.app.Cfg.KSUBranch = "main"
			}
			s.app.PersistConfig()
			s.log.OK("active branch → " + s.app.Cfg.KSUBranch)
		case "i", "u":
			selected := s.app.Cfg.KSUBranch
			brstate := s.branchState(selected)
			if brstate != resukisu.StatePresent {
				s.log.Err("cannot install: " + selected + " branch is " + string(brstate))
				return s, nil
			}
			s.busy = true
			s.stage = "installing " + selected
			s.log.Info("installing ReSukiSU [" + selected + "] …")
			return s, s.installCmd(selected)
		case "v":
			s.busy = true
			s.stage = "verifying KSU hook guards"
			s.log.Info("running apply_ksu_guards.py …")
			return s, s.guardCmd()
		case "x":
			s.busy = true
			s.stage = "removing driver (setup.sh --cleanup)"
			s.log.Warn("removing driver via setup.sh --cleanup …")
			return s, s.removeCmd()
		}
	case ksuGuardDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.log.Err("guards: " + m.err.Error())
		} else if m.report.Summary != "" {
			if m.report.HasFixes() {
				s.log.Warn("guards: " + m.report.Summary)
			} else {
				s.log.OK("guards: " + m.report.Summary)
			}
		} else {
			s.log.OK("guards verified")
		}
		return s, nil
	case ksuRemoveDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			s.log.Err("remove failed: " + m.err.Error())
			return s, nil
		}
		if m.rc == 0 && m.dirGone {
			s.disableKSUInDefconfig()
			s.app.Builder.ForceCleanReason = "Driver removed"
			s.app.Builder.Incremental = false
			_ = s.app.Builder.Save(s.app.Paths.Kernel)
			s.log.OK("driver removed; defconfig reset")
		} else {
			s.log.Err("cleanup failed or driver already gone (exit " + itoa(m.rc) + ")")
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
	innerW := innerContentWidth(w)

	banner := components.Banner(
		"ReSukiSU  DRIVER  MANAGER",
		"github.com/ReSukiSU/ReSukiSU  ·  branch probing",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// ── Active selection ────────────────────────────────────────────────────
	activeBadge := components.Badge(strings.ToUpper(s.app.Cfg.KSUBranch), BadgeAccent)
	var sel strings.Builder
	sel.WriteString(components.KV("Active", activeBadge, 9, LabelStyle, ValueStyle) + "\n")
	sel.WriteString(components.KVWrap("Target",
		s.app.Paths.Kernel+"/drivers/kernelsu",
		9, innerW-12, LabelStyle, AccentText))
	selPanel := components.Panel("Selection", sel.String(), w, PanelBorder, TitleStyle)

	// ── Upstream branch states ──────────────────────────────────────────────
	branchW := 4 // max(len("main"), len("dev"))
	var ups strings.Builder
	ups.WriteString(probeRow("main", branchW, s.main, s.probed) + "\n")
	ups.WriteString(probeRow("dev", branchW, s.dev, s.probed))
	upsPanel := components.Panel("Upstream branches", ups.String(), w, PanelBorder, TitleStyle)

	// ── Activity log ────────────────────────────────────────────────────────
	logBody := s.log.Render(8)
	if s.busy {
		logBody += "\n" + AccentText.Render(s.stage+" …")
	}
	logPanel := components.Panel("Activity", logBody, w, PanelBorder, TitleStyle)

	// ── Actions panel ───────────────────────────────────────────────────────
	leader := lipgloss.NewStyle().Foreground(ColorMuted)
	labelStyle := lipgloss.NewStyle().Foreground(ColorValue).Bold(true)
	var act strings.Builder
	act.WriteString(components.MenuRow("I", "Install / update",
		"from active branch", innerW, 1,
		HotKeyStyle, labelStyle, MutedText, leader) + "\n")
	act.WriteString(components.MenuRow("S", "Switch", "main ↔ dev", innerW, 1,
		HotKeyStyle, labelStyle, MutedText, leader) + "\n")
	act.WriteString(components.MenuRow("V", "Verify guards", "lint hook patterns", innerW, 1,
		HotKeyStyle, labelStyle, MutedText, leader) + "\n")
	act.WriteString(components.MenuRow("X", "Remove driver", "uninstall + clean", innerW, 1,
		HotKeyStyle, labelStyle, MutedText, leader) + "\n")
	act.WriteString(components.MenuRow("P", "Re-probe", "refresh upstream state", innerW, 1,
		HotKeyStyle, labelStyle, MutedText, leader))
	actPanel := components.Panel("Actions", act.String(), w, PanelBorder, TitleStyle)

	return banner + "\n" + selPanel + "\n" + upsPanel + "\n" +
		actPanel + "\n" + logPanel + "\n" +
		"  " + HelpStyle.Render("Select [I/S/V/X/P]\u00a0\u00b7\u00a0esc to return") + "\n"
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
//	  [main]  PRESENT  HEAD@7adffacb
//	  [dev ]  ABSENT   branch removed upstream
//	  [foo ]  NETWORK  could not reach upstream
//
// branchW is the widest branch label across all rows; we pad AFTER the
// closing bracket (instead of inside it) so [main] and [dev] both hug
// their labels rather than rendering as [main] / [dev ]. The trailing
// padding still keeps the badge column aligned.
// badge text is padded to 7 chars (max of "present"/"absent"/"network").
func probeRow(branch string, branchW int, p resukisu.Probe, probed bool) string {
	const badgeW = 7
	b := strings.TrimSpace(branch)
	tag := components.BracketTag(b, len(b), HotKeyStyle)
	tagPad := strings.Repeat(" ", branchW-len(b))
	prefix := "  " + tag + tagPad + "  "
	if !probed {
		return prefix + DimText.Render("(probing …)")
	}
	switch p.State {
	case resukisu.StatePresent:
		short := p.SHA
		if len(short) > 8 {
			short = short[:8]
		}
		return prefix +
			components.Badge(padTo("PRESENT", badgeW), BadgeOK) + "  " +
			DimText.Render("HEAD@") + AccentText.Bold(true).Render(short)
	case resukisu.StateAbsent:
		return prefix +
			components.Badge(padTo("ABSENT", badgeW), BadgeWarn) + "  " +
			DimText.Render("branch removed or merged upstream")
	case resukisu.StateNetworkFail:
		return prefix +
			components.Badge(padTo("NETWORK", badgeW), BadgeErr) + "  " +
			DimText.Render("could not reach upstream — check connection")
	}
	return ""
}
