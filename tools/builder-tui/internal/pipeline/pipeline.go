// Package pipeline drives the 5-stage bash 1:1 build pipeline:
//
//	Stage 1: Clean    (make clean / mrproper, rescued via .config preserve)
//	Stage 2: Defconfig (toggle CONFIG_KSU_*/MANUAL_HOOK, then make <defconfig>
//	                    or olddefconfig when in-session menuconfig is in use)
//	Stage 3: Guard    (apply_ksu_guards.py invocation; output parsed by Wave 5)
//	Stage 4: Compile  (setsid make … LLVM=1 LLVM_IAS=1 CC=clang …)
//	Stage 5: Package  (copy Image/dtbo/dtb to AnyKernel3 + zip -r9)
//
// The pipeline is a state machine that emits a stream of Events (StageStart,
// StageEnd, Line, Done, Cancelled) over a channel, which the TUI converts
// into tea.Msgs via Program.Send so the BuildScreen can render staged
// progress with a spinner / progress bar / stopwatch.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
)

// Stage discriminates the five pipeline stages.
type Stage int

const (
	StageClean Stage = iota + 1
	StageDefconfig
	StageGuard
	StageCompile
	StagePackage
)

func (s Stage) String() string {
	switch s {
	case StageClean:
		return "Clean"
	case StageDefconfig:
		return "Defconfig"
	case StageGuard:
		return "Guards"
	case StageCompile:
		return "Compile"
	case StagePackage:
		return "Package"
	}
	return "?"
}

// Status is the running result of a stage.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusOK
	StatusFailed
	StatusSkipped
	StatusCancelled
)

// Event is the tagged-union stream the pipeline emits while running.
type Event struct {
	Kind   EventKind
	Stage  Stage
	Status Status
	Detail string
	Line   string
	IsErr  bool
	// Final summary attached to KindDone events:
	Result *Result
}

// EventKind is the discriminator on Event.
type EventKind int

const (
	KindStageStart EventKind = iota
	KindStageEnd
	KindLine // compile/log line; carry isErr for stderr classification
	KindDone
)

// Options configures one pipeline run.
type Options struct {
	KernelDir string // source root
	OutputDir string // KBUILD_OUTPUT (e.g. <kernelDir>/out)
	ClangDir  string // dir containing bin/clang
	GccArm64  string // resolved aarch64 cross-compiler binary path
	GccArm    string // resolved arm cross-compiler binary path
	AnyKernel string // AnyKernel3 dir for stage 5

	Defconfig string // e.g. "vayu_defconfig"

	// Builder state
	Incremental    bool   // skip clean stage when true
	UseCcache      bool   // CC=ccache clang
	KernelName     string // LOCALVERSION suffix (-${KernelName})
	SkipDefconfig  bool   // in-session menuconfig: olddefconfig instead of make <defcfg>
	PreservedCfg   string // path to .config to restore after mrproper (Wave 6)
	ForceClean     string // non-empty -> log a "forced clean: <reason>" warning on Stage 1
	GuardScriptRel string // path under KernelDir, default "scripts/apply_ksu_guards.py"

	// Stage 5 packaging metadata (caller fills this from naming.ZipName etc.)
	ZipPath string

	// Number of make jobs; <=0 -> runtime.NumCPU().
	Jobs int
}

// Result is the final summary attached to the KindDone event.
type Result struct {
	ExitCode int
	Cancelled bool
	Image    string // path to out/arch/arm64/boot/Image when present
	ZipPath  string // path to packaged AnyKernel3 zip when stage 5 ran
	ZipSize  int64
	ImgSize  int64
	Elapsed  time.Duration

	// Per-stage outcome (status + duration) for the result panel.
	Stages map[Stage]StageResult

	// Last fail log path (combined log written via tee while compiling).
	FailLog string

	// Guard is the parsed apply_ksu_guards.py report (Stage 3). Empty when
	// the guard script was not present.
	Guard GuardReport
}

// StageResult records one stage outcome.
type StageResult struct {
	Status   Status
	Detail   string
	Elapsed  time.Duration
}

// Run executes the full pipeline. Cancel via ctx; returns the final Result
// once Done has fired. Events are sent on `events`; the channel is closed
// when the pipeline returns. Caller is responsible for draining the channel.
func Run(ctx context.Context, o Options, st features.State, events chan<- Event) Result {
	if events == nil {
		ch := make(chan Event, 1024)
		go drain(ch)
		events = ch
		defer close(ch)
	}

	if o.Jobs <= 0 {
		o.Jobs = runtime.NumCPU()
	}
	if o.Defconfig == "" {
		o.Defconfig = "vayu_defconfig"
	}
	if o.GuardScriptRel == "" {
		o.GuardScriptRel = filepath.Join("scripts", "apply_ksu_guards.py")
	}

	r := Result{Stages: map[Stage]StageResult{}}
	t0 := time.Now()
	defer func() {
		r.Elapsed = time.Since(t0)
		events <- Event{Kind: KindDone, Result: &r}
	}()

	// ── Stage 1: Clean ──────────────────────────────────────────────────────
	emit := func(stage Stage, st Status, detail string, dur time.Duration) {
		r.Stages[stage] = StageResult{Status: st, Detail: detail, Elapsed: dur}
		events <- Event{Kind: KindStageEnd, Stage: stage, Status: st, Detail: detail}
	}
	stageStart := func(stage Stage, detail string) time.Time {
		events <- Event{Kind: KindStageStart, Stage: stage, Detail: detail}
		return time.Now()
	}

	// Stage 1
	{
		t := stageStart(StageClean, "")
		if o.Incremental && o.ForceClean == "" {
			emit(StageClean, StatusSkipped, "incremental — keeping previous objects", time.Since(t))
		} else {
			detail := "make clean && make mrproper"
			if o.ForceClean != "" {
				detail = "forced clean: " + o.ForceClean
			}
			rescue := ""
			if o.SkipDefconfig {
				if cfg := filepath.Join(o.OutputDir, ".config"); fileExists(cfg) {
					if tmp, err := tempCopy(cfg); err == nil {
						rescue = tmp
					}
				}
			}
			line := func(l string, isErr bool) {
				events <- Event{Kind: KindLine, Stage: StageClean, Line: l, IsErr: isErr}
			}
			rc := runMake(ctx, o.KernelDir, o, []string{"O=" + o.OutputDir, "clean"}, line)
			if rc == 0 {
				rc = runMake(ctx, o.KernelDir, o, []string{"O=" + o.OutputDir, "mrproper"}, line)
			}
			if rescue != "" {
				_ = os.MkdirAll(o.OutputDir, 0o755)
				_ = copyFile(rescue, filepath.Join(o.OutputDir, ".config"))
				_ = os.Remove(rescue)
			}
			if rc != 0 {
				emit(StageClean, StatusFailed, fmt.Sprintf("make clean exit %d", rc), time.Since(t))
				r.ExitCode = rc
				return r
			}
			emit(StageClean, StatusOK, detail, time.Since(t))
		}
	}
	if ctx.Err() != nil {
		r.Cancelled = true
		return r
	}

	// ── Stage 2: Defconfig ───────────────────────────────────────────────────
	{
		t := stageStart(StageDefconfig, "")
		dcPath := filepath.Join(o.KernelDir, "arch", "arm64", "configs", o.Defconfig)
		if err := features.Apply(dcPath, st); err != nil {
			emit(StageDefconfig, StatusFailed, "defconfig edit: "+err.Error(), time.Since(t))
			r.ExitCode = 2
			return r
		}
		line := func(l string, isErr bool) {
			events <- Event{Kind: KindLine, Stage: StageDefconfig, Line: l, IsErr: isErr}
		}
		var rc int
		if o.SkipDefconfig {
			if o.PreservedCfg != "" && fileExists(o.PreservedCfg) {
				_ = os.MkdirAll(o.OutputDir, 0o755)
				_ = copyFile(o.PreservedCfg, filepath.Join(o.OutputDir, ".config"))
			}
			rc = runMake(ctx, o.KernelDir, o, append(makeBaseArgs(o), "olddefconfig"), line)
		} else {
			rc = runMake(ctx, o.KernelDir, o, append(makeBaseArgs(o), o.Defconfig), line)
		}
		if rc != 0 {
			emit(StageDefconfig, StatusFailed, fmt.Sprintf("make defconfig exit %d", rc), time.Since(t))
			r.ExitCode = rc
			return r
		}
		// LOCALVERSION cleanup when KERNEL_NAME is in use.
		if o.KernelName != "" {
			cfgPath := filepath.Join(o.OutputDir, ".config")
			if fileExists(cfgPath) {
				_ = stripLocalversion(cfgPath)
			}
		}
		emit(StageDefconfig, StatusOK, "applied: "+o.Defconfig, time.Since(t))
	}
	if ctx.Err() != nil {
		r.Cancelled = true
		return r
	}

	// ── Stage 3: Guard verification ──────────────────────────────────────────
	{
		t := stageStart(StageGuard, "")
		guardPath := filepath.Join(o.KernelDir, o.GuardScriptRel)
		if !fileExists(guardPath) {
			emit(StageGuard, StatusSkipped, "guard script not found", time.Since(t))
		} else {
			cmd := exec.CommandContext(ctx, "python3", guardPath)
			cmd.Dir = o.KernelDir
			cmd.Env = append(os.Environ(), "TERM_W=9999", "KERNEL_DIR="+o.KernelDir)
			out, err := cmd.CombinedOutput()
			for _, l := range strings.Split(string(out), "\n") {
				if l == "" {
					continue
				}
				events <- Event{Kind: KindLine, Stage: StageGuard, Line: l}
			}
			r.Guard = ParseGuardReport(string(out))
			if err != nil {
				emit(StageGuard, StatusFailed, "guard script: "+err.Error(), time.Since(t))
			} else {
				summary := r.Guard.Summary
				if summary == "" {
					summary = "guards verified"
				}
				emit(StageGuard, StatusOK, summary, time.Since(t))
			}
		}
	}
	if ctx.Err() != nil {
		r.Cancelled = true
		return r
	}

	// ── Stage 4: Compile ─────────────────────────────────────────────────────
	failLog := filepath.Join(o.OutputDir, "BUILD-FAIL.log")
	r.FailLog = failLog
	_ = os.MkdirAll(o.OutputDir, 0o755)
	logF, _ := os.Create(failLog)
	if logF != nil {
		defer logF.Close()
	}
	{
		t := stageStart(StageCompile, fmt.Sprintf("%d threads", o.Jobs))
		args := makeBaseArgs(o)
		args = append(args,
			"CLANG_TRIPLE=aarch64-linux-gnu-",
			"CROSS_COMPILE=aarch64-linux-gnu-",
			"CROSS_COMPILE_ARM32=arm-linux-gnueabi-",
		)
		if o.KernelName != "" {
			args = append(args, "LOCALVERSION=-"+o.KernelName, "CONFIG_LOCALVERSION=")
			scmv := filepath.Join(o.KernelDir, ".scmversion")
			backup, _ := os.ReadFile(scmv)
			_ = os.WriteFile(scmv, []byte(""), 0o644)
			defer func() {
				if len(backup) > 0 {
					_ = os.WriteFile(scmv, backup, 0o644)
				} else {
					_ = os.Remove(scmv)
				}
			}()
		}
		args = append(args, "-j"+strconv.Itoa(o.Jobs))
		// Note: bash adds "Image.gz-dtb" or omits target; vayu uses the default
		// all target which produces arch/arm64/boot/Image.
		line := func(l string, isErr bool) {
			if logF != nil {
				logF.WriteString(l + "\n")
			}
			events <- Event{Kind: KindLine, Stage: StageCompile, Line: l, IsErr: isErr}
		}
		rc, cancelled := runMakeSetsid(ctx, o.KernelDir, o, args, line)
		if cancelled {
			r.Cancelled = true
			emit(StageCompile, StatusCancelled, "Ctrl-C", time.Since(t))
			return r
		}
		img := filepath.Join(o.OutputDir, "arch", "arm64", "boot", "Image")
		if rc != 0 || !fileExists(img) {
			r.ExitCode = rc
			emit(StageCompile, StatusFailed, fmt.Sprintf("make exit %d", rc), time.Since(t))
			return r
		}
		r.Image = img
		if fi, err := os.Stat(img); err == nil {
			r.ImgSize = fi.Size()
		}
		emit(StageCompile, StatusOK, "kernel image built", time.Since(t))
	}

	// ── Stage 5: Package ─────────────────────────────────────────────────────
	{
		t := stageStart(StagePackage, "")
		if o.AnyKernel == "" {
			emit(StagePackage, StatusSkipped, "AnyKernel3 dir not set", time.Since(t))
			return r
		}
		zipPath := o.ZipPath
		if zipPath == "" {
			zipPath = filepath.Join(o.OutputDir, "kernel.zip")
		}
		// Copy Image / dtbo.img / dtb.img into AnyKernel3.
		_ = copyFile(filepath.Join(o.OutputDir, "arch", "arm64", "boot", "Image"),
			filepath.Join(o.AnyKernel, "Image"))
		_ = copyFile(filepath.Join(o.OutputDir, "arch", "arm64", "boot", "dtbo.img"),
			filepath.Join(o.AnyKernel, "dtbo.img"))
		_ = copyFile(filepath.Join(o.OutputDir, "arch", "arm64", "boot", "dtb.img"),
			filepath.Join(o.AnyKernel, "dtb.img"))

		line := func(l string, isErr bool) {
			events <- Event{Kind: KindLine, Stage: StagePackage, Line: l, IsErr: isErr}
		}
		zipBin, err := exec.LookPath("zip")
		if err != nil {
			emit(StagePackage, StatusFailed, "zip not on PATH", time.Since(t))
			return r
		}
		cmd := exec.CommandContext(ctx, zipBin, "-r9", zipPath, ".", "-x", "*.git*")
		cmd.Dir = o.AnyKernel
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			emit(StagePackage, StatusFailed, "zip start: "+err.Error(), time.Since(t))
			return r
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); streamLines(stdout, false, line) }()
		go func() { defer wg.Done(); streamLines(stderr, true, line) }()
		wg.Wait()
		if err := cmd.Wait(); err != nil {
			emit(StagePackage, StatusFailed, "zip: "+err.Error(), time.Since(t))
			return r
		}
		if fi, err := os.Stat(zipPath); err == nil {
			r.ZipPath = zipPath
			r.ZipSize = fi.Size()
		}
		emit(StagePackage, StatusOK, "AnyKernel3 zip created", time.Since(t))
	}

	r.ExitCode = 0
	return r
}

// makeBaseArgs builds the common make argv used in stages 2 and 4.
func makeBaseArgs(o Options) []string {
	args := []string{
		"-C", o.KernelDir,
		"O=" + o.OutputDir,
		"ARCH=arm64",
		"LLVM=1",
		"LLVM_IAS=1",
	}
	cc := "clang"
	if o.UseCcache {
		cc = "ccache clang"
	}
	args = append(args, "CC="+cc)
	return args
}

// runMake runs a make invocation with stage 1/2 default streaming (no setsid).
func runMake(ctx context.Context, dir string, o Options, args []string, line func(l string, isErr bool)) int {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "make", full...)
	cmd.Dir = dir
	cmd.Env = buildEnv(o)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		line("make start: "+err.Error(), true)
		return 127
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); streamLines(stdout, false, line) }()
	go func() { defer wg.Done(); streamLines(stderr, true, line) }()
	wg.Wait()
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if asExit(err, &ee) {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// runMakeSetsid runs make in its own session (setsid) so a single SIGTERM
// to the negative PID kills the entire compile pgroup. Returns (exitCode,
// cancelled).
func runMakeSetsid(ctx context.Context, dir string, o Options, args []string, line func(l string, isErr bool)) (int, bool) {
	cmd := exec.Command("make", args...)
	cmd.Dir = dir
	cmd.Env = buildEnv(o)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		line("make start: "+err.Error(), true)
		return 127, false
	}

	cancelled := false
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cancelled = true
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				_ = syscall.Kill(-pgid, syscall.SIGTERM)
			}
			_ = cmd.Process.Kill()
		case <-done:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); streamLines(stdout, false, line) }()
	go func() { defer wg.Done(); streamLines(stderr, true, line) }()
	wg.Wait()
	err := cmd.Wait()
	close(done)
	if cancelled {
		return -1, true
	}
	if err != nil {
		var ee *exec.ExitError
		if asExit(err, &ee) {
			return ee.ExitCode(), false
		}
		return 1, false
	}
	return 0, false
}

func buildEnv(o Options) []string {
	env := os.Environ()
	if o.ClangDir != "" {
		path := os.Getenv("PATH")
		env = replaceOrAppend(env, "PATH="+filepath.Join(o.ClangDir, "bin")+":"+path)
	}
	env = append(env, "KBUILD_OUTPUT="+o.OutputDir)
	if o.UseCcache {
		env = append(env,
			"CCACHE_DIR="+ccacheDir(),
			"CCACHE_MAXSIZE=10G",
			"CCACHE_COMPRESS=1",
			"CCACHE_SLOPPINESS=time_macros,include_file_mtime,locale,include_file_ctime,file_macro,system_headers,pch_defines",
		)
	}
	return env
}

func ccacheDir() string {
	if d := os.Getenv("CCACHE_DIR"); d != "" {
		return d
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".cache", "ccache")
	}
	return "/tmp/ccache"
}

func replaceOrAppend(env []string, kv string) []string {
	idx := strings.IndexByte(kv, '=')
	if idx == -1 {
		return env
	}
	prefix := kv[:idx+1]
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = kv
			return env
		}
	}
	return append(env, kv)
}

func asExit(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
