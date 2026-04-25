package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/ascii"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/naming"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/state"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// BuildScreen drives the 5-stage build pipeline with live stage rendering,
// a bubbles stopwatch for elapsed time, a spinner for the running stage,
// and a viewport for streamed compile output. After completion it renders
// the matching SUCCESS / FAILED / CANCELLED ASCII art with an animated
// reveal and a result panel, then offers the bash post-build prompt
// ([T] full retry, [I] incremental retry, [R] return, [E] exit, plus
// menuconfig V/D when applicable).
type BuildScreen struct {
	app *App

	vp        viewport.Model
	spin      spinner.Model
	sw        stopwatch.Model
	lines     []string
	running   bool
	cancelFn  context.CancelFunc
	stages    []stageView
	result    *pipeline.Result
	revealed  int // ascii art rows revealed
	startTime time.Time

	// post-build mode
	postBuild bool
}

// stageView mirrors a pipeline.Stage's live state for rendering.
type stageView struct {
	Stage   pipeline.Stage
	Status  pipeline.Status
	Detail  string
	Started time.Time
	Ended   time.Time
}

// NewBuildScreen constructs the screen with its bubbles sub-models.
func NewBuildScreen(a *App) BuildScreen {
	vp := viewport.New(80, 14)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(ColorBuild).
		Padding(0, 1)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)

	sw := stopwatch.NewWithInterval(time.Second)
	return BuildScreen{app: a, vp: vp, spin: sp, sw: sw}
}

func (s BuildScreen) Init() tea.Cmd { return nil }

// Pipeline event messages — flowed in from the goroutine via Program.Send.
type stageStartMsg struct{ stage pipeline.Stage; detail string }
type stageEndMsg struct{ stage pipeline.Stage; status pipeline.Status; detail string }
type compileLineMsg struct{ stage pipeline.Stage; line string; isErr bool }
type pipelineDoneMsg struct{ res pipeline.Result }

func (s BuildScreen) Update(msg tea.Msg) (BuildScreen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		w := panelWidth(m.Width)
		s.vp.Width = w - 4
		if s.vp.Width < 40 {
			s.vp.Width = 40
		}
		s.vp.Height = m.Height - 22
		if s.vp.Height < 8 {
			s.vp.Height = 8
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(m)
		return s, cmd
	case stopwatch.TickMsg, stopwatch.StartStopMsg:
		var cmd tea.Cmd
		s.sw, cmd = s.sw.Update(m)
		return s, cmd
	case stageStartMsg:
		s.stages = append(s.stages, stageView{Stage: m.stage, Status: pipeline.StatusRunning, Detail: m.detail, Started: time.Now()})
	case stageEndMsg:
		for i := len(s.stages) - 1; i >= 0; i-- {
			if s.stages[i].Stage == m.stage {
				s.stages[i].Status = m.status
				s.stages[i].Detail = m.detail
				s.stages[i].Ended = time.Now()
				break
			}
		}
	case compileLineMsg:
		s.lines = append(s.lines, m.line)
		if len(s.lines) > 5000 {
			s.lines = s.lines[len(s.lines)-5000:]
		}
		s.vp.SetContent(strings.Join(s.lines, "\n"))
		s.vp.GotoBottom()
	case pipelineDoneMsg:
		s.running = false
		s.result = &m.res
		s.postBuild = true
		s.revealed = 0
		// Persist state on success / failure / cancel.
		if m.res.Cancelled {
			// no commit; keep build counter
		} else if m.res.ExitCode == 0 && m.res.Image != "" {
			next := state.ReadBuildNumber(s.app.Paths.Kernel)
			_ = state.CommitBuildNumber(s.app.Paths.Kernel, next)
			pb := state.PrevBuild{
				Cap:        readDefconfigState(s.app).CapTag(s.app.Builder.KSUBranch),
				ExtFeat:    readDefconfigState(s.app).ExtTag(),
				Mode:       buildMode(m.res),
				Num:        next,
				KernelName: s.app.Cfg.KernelName,
				Date:       time.Now().Format("2006-01-02 15:04"),
			}
			_ = pb.Save(s.app.Paths.Kernel)
		}
		seq := append([]tea.Cmd{s.sw.Stop()}, ascii.AnimateCmds(asciiKindFor(m.res))...)
		return s, tea.Sequence(seq...)
	case ascii.RevealMsg:
		if m.Row > s.revealed {
			s.revealed = m.Row
		}
	case savedefconfigDoneMsg:
		if m.err != nil {
			s.app.Toast = "savedefconfig failed: " + m.err.Error()
			s.app.ToastErr = true
		} else {
			s.app.Toast = "vayu_defconfig updated from in-session menuconfig"
			s.app.ToastErr = false
		}
	case tea.KeyMsg:
		if !s.postBuild && !s.running {
			switch strings.ToLower(m.String()) {
			case "b":
				return s.startBuild()
			}
		}
		if s.running {
			switch m.String() {
			case "ctrl+c":
				if s.cancelFn != nil {
					s.cancelFn()
				}
				return s, nil
			}
		}
		if s.postBuild {
			return s.handlePostBuild(m)
		}
		var cmd tea.Cmd
		s.vp, cmd = s.vp.Update(msg)
		return s, cmd
	}
	return s, nil
}

// handlePostBuild routes the post-build prompt keys.
func (s BuildScreen) handlePostBuild(m tea.KeyMsg) (BuildScreen, tea.Cmd) {
	switch strings.ToLower(m.String()) {
	case "t":
		s.app.Builder.Incremental = false
		_ = s.app.Builder.Save(s.app.Paths.Kernel)
		return s.startBuild()
	case "i":
		s.app.Builder.Incremental = true
		_ = s.app.Builder.Save(s.app.Paths.Kernel)
		return s.startBuild()
	case "v":
		if s.app.MenuconfigUsed {
			cfg := filepath.Join(s.app.Paths.Output, ".config")
			if err := state.SaveMenuconfigPreserve(s.app.Paths.Kernel, cfg); err != nil {
				s.app.Toast = "Preserve failed: " + err.Error()
				s.app.ToastErr = true
			} else {
				s.app.MenuconfigPreserved = true
				s.app.Toast = "Menuconfig .config preserved -- restored on next build"
				s.app.ToastErr = false
			}
		}
		return s, nil
	case "d":
		if s.app.MenuconfigUsed {
			return s, s.runSavedefconfig()
		}
		return s, nil
	case "r":
		s.postBuild = false
		s.app.Screen = ScreenMain
		return s, nil
	case "e":
		return s, tea.Quit
	}
	return s, nil
}

// savedefconfigDoneMsg reports the result of a `make savedefconfig` run
// triggered by post-build [D].
type savedefconfigDoneMsg struct {
	err error
}

// runSavedefconfig regenerates a defconfig from the in-tree .config and
// copies it over arch/arm64/configs/vayu_defconfig (mirrors bash [D]).
func (s BuildScreen) runSavedefconfig() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cc := "clang"
		if s.app.Builder.UseCcache {
			cc = "ccache clang"
		}
		args := []string{
			"-C", s.app.Paths.Kernel,
			"O=" + s.app.Paths.Output,
			"ARCH=arm64", "LLVM=1", "LLVM_IAS=1", "CC=" + cc,
			"savedefconfig",
		}
		_ = ctx
		cmd := exec.Command("make", args...)
		cmd.Dir = s.app.Paths.Kernel
		out, err := cmd.CombinedOutput()
		_ = out
		if err != nil {
			return savedefconfigDoneMsg{err: err}
		}
		src := filepath.Join(s.app.Paths.Output, "defconfig")
		dst := filepath.Join(s.app.Paths.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
		if err := copyToFile(src, dst); err != nil {
			return savedefconfigDoneMsg{err: err}
		}
		s.app.MenuconfigUsed = false
		state.ClearMenuconfigPreserve(s.app.Paths.Kernel)
		s.app.MenuconfigPreserved = false
		return savedefconfigDoneMsg{}
	}
}

func copyToFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

// startBuild kicks off a new pipeline run on a goroutine, hands back a Cmd
// that starts the spinner and stopwatch and clears prior state.
func (s BuildScreen) startBuild() (BuildScreen, tea.Cmd) {
	if s.app.Paths.Clang == "" || s.app.Paths.AnyKernel == "" {
		s.app.Toast = "Cannot build — fix Clang / AnyKernel3 paths first"
		s.app.ToastErr = true
		return s, nil
	}
	s.running = true
	s.postBuild = false
	s.revealed = 0
	s.result = nil
	s.lines = nil
	s.stages = nil
	s.startTime = time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel

	dc := filepath.Join(s.app.Paths.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
	st, err := features.Read(dc)
	if err == nil {
		st.DriverPresent = features.DriverPresent(s.app.Paths.Kernel)
	}

	// Compose ZipPath via naming.
	branch := s.app.Builder.KSUBranch
	if branch == "" {
		branch = "main"
	}
	num := state.ReadBuildNumber(s.app.Paths.Kernel)
	args := naming.Args{
		KernelName: s.app.Cfg.KernelName,
		State:      st,
		Branch:     branch,
		Date:       time.Now(),
		Build:      num,
	}
	zipName := naming.ZipName(args)
	zipPath := filepath.Join(s.app.Paths.Output, zipName)

	preservedCfg := ""
	if s.app.MenuconfigPreserved {
		preservedCfg = filepath.Join(s.app.Paths.Kernel, ".menuconfig_saved_config")
	}
	opts := pipeline.Options{
		KernelDir:     s.app.Paths.Kernel,
		OutputDir:     s.app.Paths.Output,
		ClangDir:      s.app.Paths.Clang,
		GccArm64:      s.app.Paths.GccArm64,
		GccArm:        s.app.Paths.GccArm,
		AnyKernel:     s.app.Paths.AnyKernel,
		Defconfig:     "vayu_defconfig",
		Incremental:   s.app.Builder.Incremental,
		UseCcache:     s.app.Builder.UseCcache,
		KernelName:    s.app.Cfg.KernelName,
		ForceClean:    s.app.Builder.ForceCleanReason,
		SkipDefconfig: s.app.MenuconfigUsed || s.app.MenuconfigPreserved,
		PreservedCfg:  preservedCfg,
		PackageOnly:   s.app.PackageOnly,
		ZipPath:       zipPath,
	}
	s.app.PackageOnly = false

	go runPipelineGoroutine(ctx, opts, st)

	return s, tea.Batch(s.spin.Tick, s.sw.Reset(), s.sw.Start())
}

// runPipelineGoroutine bridges the pipeline.Run event channel onto
// Program.Send so the BuildScreen receives stageStart/stageEnd/line/done
// messages on the tea event loop.
func runPipelineGoroutine(ctx context.Context, o pipeline.Options, st features.State) {
	ch := make(chan pipeline.Event, 1024)
	done := make(chan struct{})
	go func() {
		_ = pipeline.Run(ctx, o, st, ch)
		close(ch)
		close(done)
	}()
	for ev := range ch {
		switch ev.Kind {
		case pipeline.KindStageStart:
			if Program != nil {
				Program.Send(stageStartMsg{stage: ev.Stage, detail: ev.Detail})
			}
		case pipeline.KindStageEnd:
			if Program != nil {
				Program.Send(stageEndMsg{stage: ev.Stage, status: ev.Status, detail: ev.Detail})
			}
		case pipeline.KindLine:
			if Program != nil {
				Program.Send(compileLineMsg{stage: ev.Stage, line: ev.Line, isErr: ev.IsErr})
			}
		case pipeline.KindDone:
			if Program != nil && ev.Result != nil {
				Program.Send(pipelineDoneMsg{res: *ev.Result})
			}
		}
	}
	<-done
}

func asciiKindFor(r pipeline.Result) ascii.Kind {
	switch {
	case r.Cancelled:
		return ascii.Cancelled
	case r.ExitCode != 0 || r.Image == "":
		return ascii.Failed
	default:
		return ascii.Success
	}
}

func buildMode(r pipeline.Result) string {
	if _, ok := r.Stages[pipeline.StageClean]; ok {
		if r.Stages[pipeline.StageClean].Status == pipeline.StatusSkipped {
			return "Incremental"
		}
		return "Full Clean"
	}
	return "Unknown"
}

// View renders the build screen.
func (s BuildScreen) View() string {
	w := panelWidth(s.app.Width)
	inner := innerContentWidth(w)

	// Banner shows the current run header with stopwatch when running.
	subtitle := s.app.Paths.Kernel
	if s.running {
		subtitle = "elapsed " + s.sw.View() + "  ·  " + s.app.Paths.Kernel
	}
	banner := components.Banner("BUILD  KERNEL", subtitle, w, BannerBorder, BannerTitle, BannerSubtle)

	// Stages panel.
	var stages strings.Builder
	for i, sv := range s.stages {
		stages.WriteString(stageRow(sv, s.spin.View(), inner))
		if i < len(s.stages)-1 {
			stages.WriteString("\n")
		}
	}
	if stages.Len() == 0 {
		stages.WriteString(lipgloss.PlaceHorizontal(inner, lipgloss.Center,
			HelpStyle.Render("press [B] to start the build")))
	}
	stagesPanel := components.Panel("Pipeline", stages.String(), w, PanelBorder, TitleStyle)

	// Compile output viewport (header rule + framed).
	headLabel := lipgloss.NewStyle().Foreground(ColorBuild).Bold(true).Render(" Compiler output ")
	headFillW := w - lipgloss.Width(headLabel)
	if headFillW < 0 {
		headFillW = 0
	}
	vpHeader := headLabel + MutedText.Render(strings.Repeat("─", headFillW)) + "\n"

	// Result panel (after pipeline finishes).
	var resultPanel string
	if s.result != nil {
		resultPanel = "\n" + s.renderResultPanel(w, inner) + "\n"
	}

	out := banner + "\n" + stagesPanel + "\n" + vpHeader + s.vp.View() + resultPanel + "\n" +
		"  " + HelpStyle.Render(s.helpLine()) + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}

// stageRow renders one pipeline stage line: `[#] Name      RUNNING  detail`.
func stageRow(sv stageView, spinFrame string, width int) string {
	num := fmt.Sprintf("%d", sv.Stage)
	name := sv.Stage.String()
	badge := ""
	switch sv.Status {
	case pipeline.StatusRunning:
		badge = lipgloss.NewStyle().Foreground(ColorAccent).Render(spinFrame + " RUNNING")
	case pipeline.StatusOK:
		badge = components.Badge("OK", BadgeOK)
	case pipeline.StatusFailed:
		badge = components.Badge("FAIL", BadgeErr)
	case pipeline.StatusSkipped:
		badge = components.Badge("SKIP", BadgeAccent)
	case pipeline.StatusCancelled:
		badge = components.Badge("CANCEL", BadgeWarn)
	default:
		badge = MutedText.Render("…")
	}
	tag := components.BracketTag(num, 1, HotKeyStyle)
	// 2-col leading indent so the [#] bracket column lines up with menu
	// rows on every other screen.
	prefix := "  " + tag + "  " + ValueStyle.Render(padTo(name, 10))
	rhs := badge
	if sv.Detail != "" {
		rhs += "  " + DimText.Render(sv.Detail)
	}
	pad := width - lipgloss.Width(prefix) - lipgloss.Width(rhs)
	return prefix + components.DotLeader(pad, MutedText) + rhs
}

// renderResultPanel renders ASCII art + summary panel after the pipeline
// completes (success / fail / cancel paths).
func (s BuildScreen) renderResultPanel(w, inner int) string {
	r := s.result
	kind := asciiKindFor(*r)
	art := ascii.Render(kind, s.revealed)

	var b strings.Builder
	b.WriteString(art)
	b.WriteString("\n")

	switch kind {
	case ascii.Success:
		b.WriteString(s.renderSuccessSummary(inner))
	case ascii.Failed:
		b.WriteString(s.renderFailSummary(inner))
	case ascii.Cancelled:
		b.WriteString(s.renderCancelSummary(inner))
	}

	border := PanelOK
	title := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	switch kind {
	case ascii.Failed:
		border = PanelErr
		title = lipgloss.NewStyle().Foreground(ColorErr).Bold(true)
	case ascii.Cancelled:
		border = PanelWarn
		title = lipgloss.NewStyle().Foreground(ColorWarn).Bold(true)
	}
	out := components.Panel(strings.ToUpper(kind.String()), b.String(), w, border, title)
	if gp := renderGuardPanel(r.Guard, w); gp != "" {
		out += "\n" + gp
	}
	return out
}

func (s BuildScreen) renderSuccessSummary(inner int) string {
	r := s.result
	num := state.ReadBuildNumber(s.app.Paths.Kernel) - 1
	if num < 1 {
		num = 1
	}
	st := readDefconfigState(s.app)
	cc := "disabled"
	if s.app.Builder.UseCcache {
		cc = "enabled"
	}
	var b strings.Builder
	const lblW = 14
	valW := inner - lblW - 3 // " : "
	if valW < 8 {
		valW = 8
	}
	add := func(k, v string) {
		b.WriteString(components.KVWrap(k, v, lblW, valW, LabelStyle, ValueStyle))
		b.WriteString("\n")
	}
	add("Build", fmt.Sprintf("#%d", num))
	if s.app.Cfg.KernelName != "" {
		add("Kernel-Name", s.app.Cfg.KernelName)
	}
	add("Capabilities", st.CapTag(s.app.Builder.KSUBranch))
	add("Features", st.ExtTag())
	add("ccache", cc)
	add("Time", pipeline.FormatElapsed(int(r.Elapsed.Seconds())))
	add("When", time.Now().Format("2006-01-02 15:04"))
	b.WriteString(components.Separator(inner, MutedText))
	b.WriteString("\n")
	if r.ZipPath != "" {
		add("Zip", filepath.Base(r.ZipPath))
		add("Size", humanSize(r.ZipSize))
	}
	if r.Image != "" {
		add("Image", filepath.Base(r.Image))
	}
	add("Output", s.app.Paths.Output)
	return b.String()
}

func (s BuildScreen) renderFailSummary(inner int) string {
	r := s.result
	var b strings.Builder
	b.WriteString(lipgloss.PlaceHorizontal(inner, lipgloss.Center,
		ErrText.Render("Build failed — see errors below")) + "\n")
	b.WriteString(components.Separator(inner, MutedText) + "\n")

	linker, compiler := pipeline.FailErrors(r.FailLog)
	maxw := inner - 8
	if maxw < 20 {
		maxw = 20
	}
	if len(linker) > 0 {
		b.WriteString(WarnText.Render("  LINKER") + "\n")
		for _, l := range linker {
			b.WriteString("  " + WarnText.Render(truncate(l, maxw)) + "\n")
		}
	}
	if len(compiler) > 0 {
		b.WriteString(ErrText.Render("  COMPILER") + "\n")
		for _, l := range compiler {
			b.WriteString("  " + ErrText.Render(truncate(l, maxw)) + "\n")
		}
	}
	if len(linker) == 0 && len(compiler) == 0 {
		b.WriteString(DimText.Render("  (no error: lines found — check the log)") + "\n")
	}
	b.WriteString(components.Separator(inner, MutedText) + "\n")
	wrapW := inner - 14 - 3
	if wrapW < 8 {
		wrapW = 8
	}
	b.WriteString(components.KVWrap("Log", filepath.Base(r.FailLog), 14, wrapW, LabelStyle, DimText) + "\n")
	return b.String()
}

func (s BuildScreen) renderCancelSummary(inner int) string {
	var b strings.Builder
	r := s.result
	b.WriteString(lipgloss.PlaceHorizontal(inner, lipgloss.Center,
		WarnText.Render("Build cancelled by user")) + "\n")
	b.WriteString(components.Separator(inner, MutedText) + "\n")
	wrapW := inner - 14 - 3
	if wrapW < 8 {
		wrapW = 8
	}
	b.WriteString(components.KVWrap("Elapsed", pipeline.FormatElapsed(int(r.Elapsed.Seconds())), 14, wrapW, LabelStyle, DimText) + "\n")
	b.WriteString("  " + DimText.Render("Objects in out/ are intact for incremental retry") + "\n")
	return b.String()
}

// helpLine returns a single-line description of the keys available in the
// build screen depending on its current state (running / post-build / idle).
func (s BuildScreen) helpLine() string {
	if s.running {
		return "Ctrl+C cancels · ↑/↓ scroll · esc to return"
	}
	if s.postBuild {
		opts := []string{"T", "I"}
		if s.app.MenuconfigUsed {
			opts = append(opts, "V", "D")
		}
		opts = append(opts, "R", "E")
		return "Select [" + strings.Join(opts, "/") + "] · esc to return"
	}
	return "Press [B] to build · esc to return"
}

// truncate returns s clipped to maxRunes with an ellipsis when needed.
func truncate(s string, maxRunes int) string {
	if maxRunes <= 1 || lipgloss.Width(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	if len(r) > maxRunes-1 {
		r = r[:maxRunes-1]
	}
	return string(r) + "…"
}

func humanSize(b int64) string {
	const k = 1024
	if b < k {
		return fmt.Sprintf("%dB", b)
	}
	if b < k*k {
		return fmt.Sprintf("%.1fK", float64(b)/k)
	}
	if b < k*k*k {
		return fmt.Sprintf("%.1fM", float64(b)/(k*k))
	}
	return fmt.Sprintf("%.1fG", float64(b)/(k*k*k))
}


