// Package kbuild runs the actual kernel build with sensible defaults
// (LLVM=1, ccache, KBUILD_OUTPUT). It streams compile output line-by-line so
// the TUI can show live progress.
package kbuild

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/proc"
)

// Options configures a build run.
type Options struct {
	KernelDir string // source root
	OutputDir string // KBUILD_OUTPUT
	ClangDir  string // dir containing bin/clang
	GccArm64  string // path to aarch64 cross-compiler binary (resolved CROSS_COMPILE)
	GccArm    string // path to arm cross-compiler binary (resolved CROSS_COMPILE_ARM32)

	Defconfig string // e.g. "vayu_defconfig"
	Arch      string // default "arm64"
	Jobs      int    // <=0 -> runtime.NumCPU()
	UseCcache bool

	ExtraEnv []string
}

// Result reports a build outcome.
type Result struct {
	ExitCode    int
	Image       string // path to the built kernel image (Image.gz-dtb if present)
	OutputDir   string
	Elapsed     time.Duration
	CombinedLog string
}

// Defaults returns a sensibly populated Options for vayu.
func Defaults(kernelRoot string) Options {
	return Options{
		KernelDir: kernelRoot,
		OutputDir: filepath.Join(kernelRoot, "out"),
		Defconfig: "vayu_defconfig",
		Arch:      "arm64",
		Jobs:      runtime.NumCPU(),
		UseCcache: true,
	}
}

// crossPrefix derives the GNU cross-compile prefix from a binary path.
// "/usr/bin/aarch64-linux-gnu-gcc" -> "/usr/bin/aarch64-linux-gnu-".
func crossPrefix(p string) string {
	if p == "" {
		return ""
	}
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	cut := strings.LastIndex(base, "gcc")
	if cut == -1 {
		return ""
	}
	return filepath.Join(dir, base[:cut])
}

// Run executes a clean defconfig + Image.gz-dtb build.
func Run(ctx context.Context, o Options, line proc.LineFunc) Result {
	start := time.Now()
	if o.Jobs <= 0 {
		o.Jobs = runtime.NumCPU()
	}
	if o.Arch == "" {
		o.Arch = "arm64"
	}
	if o.OutputDir == "" {
		o.OutputDir = filepath.Join(o.KernelDir, "out")
	}
	_ = os.MkdirAll(o.OutputDir, 0o755)

	env := buildEnv(o)
	makeArgs := makeBaseArgs(o)

	defcfg := append([]string{}, makeArgs...)
	defcfg = append(defcfg, o.Defconfig)
	if r := proc.Run(ctx, o.KernelDir, env, line, "make", defcfg...); !r.Ok() {
		return Result{ExitCode: r.ExitCode, OutputDir: o.OutputDir, Elapsed: time.Since(start), CombinedLog: r.Combined}
	}

	build := append([]string{}, makeArgs...)
	build = append(build, "-j"+strconv.Itoa(o.Jobs), "Image.gz-dtb")
	r := proc.Run(ctx, o.KernelDir, env, line, "make", build...)
	res := Result{
		ExitCode:    r.ExitCode,
		OutputDir:   o.OutputDir,
		Elapsed:     time.Since(start),
		CombinedLog: r.Combined,
	}
	if r.Ok() {
		// Locate produced image
		img := filepath.Join(o.OutputDir, "arch", o.Arch, "boot", "Image.gz-dtb")
		if _, err := os.Stat(img); err == nil {
			res.Image = img
		}
	}
	return res
}

func makeBaseArgs(o Options) []string {
	args := []string{
		"O=" + o.OutputDir,
		"ARCH=" + o.Arch,
		"LLVM=1",
		"LLVM_IAS=1",
	}
	if o.GccArm64 != "" {
		args = append(args, "CROSS_COMPILE="+crossPrefix(o.GccArm64))
	}
	if o.GccArm != "" {
		args = append(args, "CROSS_COMPILE_ARM32="+crossPrefix(o.GccArm))
	}
	if o.UseCcache {
		args = append(args, "CC=ccache clang", "HOSTCC=ccache clang", "HOSTCXX=ccache clang++")
	}
	return args
}

func buildEnv(o Options) []string {
	env := os.Environ()
	if o.ClangDir != "" {
		// Prepend clang's bin so PATH-based lookups for clang/llvm-* work.
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
	env = append(env, o.ExtraEnv...)
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

// FormatElapsed pretty-prints a duration as MM:SS.
func FormatElapsed(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	return fmt.Sprintf("%02d:%02d", m, s)
}
