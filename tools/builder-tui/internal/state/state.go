// Package state persists per-kernel-tree builder state across runs.
//
// Files kept (all under the kernel source tree):
//
//	.build_number                 — rolling integer, incremented on success
//	.builder_state                — INCREMENTAL / USE_CCACHE / KSU_BRANCH / FORCE_CLEAN_REASON
//	.builder_prev_state           — last-build summary metadata (rendered on the Mode menu)
//	.menuconfig_saved_config      — preserved out/.config from a previous menuconfig session
//
// The bash builder used dot-prefixed shell-source-able files; this Go
// implementation keeps the same on-disk format so a user can switch
// between the two implementations without losing state.
package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileNames in the kernel root.
const (
	FileBuildNumber       = ".build_number"
	FileBuilderState      = ".builder_state"
	FileBuilderPrevState  = ".builder_prev_state"
	FileMenuconfigPreserve = ".menuconfig_saved_config"
)

// Builder holds the live builder state (`.builder_state`).
type Builder struct {
	Incremental      bool
	UseCcache        bool
	KSUBranch        string
	ForceCleanReason string
}

// DefaultBuilder returns the bash defaults: full clean build, ccache on,
// main branch, no force-clean reason.
func DefaultBuilder() Builder {
	return Builder{
		Incremental: false,
		UseCcache:   true,
		KSUBranch:   "main",
	}
}

// LoadBuilder reads `.builder_state` from kernelDir. Missing file or
// parse errors return DefaultBuilder() with no error.
func LoadBuilder(kernelDir string) Builder {
	b := DefaultBuilder()
	p := filepath.Join(kernelDir, FileBuilderState)
	f, err := os.Open(p)
	if err != nil {
		return b
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch k {
		case "INCREMENTAL":
			b.Incremental = parseBool(v)
		case "USE_CCACHE":
			b.UseCcache = parseBool(v)
		case "KSU_BRANCH":
			if v == "main" || v == "dev" {
				b.KSUBranch = v
			}
		case "FORCE_CLEAN_REASON":
			b.ForceCleanReason = v
		}
	}
	return b
}

// Save writes `.builder_state` atomically.
func (b Builder) Save(kernelDir string) error {
	p := filepath.Join(kernelDir, FileBuilderState)
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "INCREMENTAL=%s\n", boolStr(b.Incremental))
	fmt.Fprintf(w, "USE_CCACHE=%s\n", boolStr(b.UseCcache))
	fmt.Fprintf(w, "KSU_BRANCH=%q\n", b.KSUBranch)
	fmt.Fprintf(w, "FORCE_CLEAN_REASON=%q\n", b.ForceCleanReason)
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// PrevBuild captures a rendered summary of the last successful build,
// shown on the Mode menu's "Previous Build" panel.
type PrevBuild struct {
	Cap        string
	ExtFeat    string
	Mode       string
	Num        int
	KernelName string
	Date       string // "YYYY-MM-DD HH:MM"
}

// LoadPrevBuild reads `.builder_prev_state`. Missing file returns a zero
// PrevBuild (no error) so callers can render an "(empty)" panel.
func LoadPrevBuild(kernelDir string) PrevBuild {
	pb := PrevBuild{}
	p := filepath.Join(kernelDir, FileBuilderPrevState)
	f, err := os.Open(p)
	if err != nil {
		return pb
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch k {
		case "PREV_CAP":
			pb.Cap = v
		case "PREV_EXT_FEAT":
			pb.ExtFeat = v
		case "PREV_MODE":
			pb.Mode = v
		case "PREV_NUM":
			n, _ := strconv.Atoi(v)
			pb.Num = n
		case "PREV_KNAME":
			pb.KernelName = v
		case "PREV_DATE":
			pb.Date = v
		}
	}
	return pb
}

// Save writes `.builder_prev_state` atomically. Date is filled with the
// current time if blank.
func (pb PrevBuild) Save(kernelDir string) error {
	if pb.Date == "" {
		pb.Date = time.Now().Format("2006-01-02 15:04")
	}
	p := filepath.Join(kernelDir, FileBuilderPrevState)
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "PREV_CAP=%q\n", pb.Cap)
	fmt.Fprintf(w, "PREV_EXT_FEAT=%q\n", pb.ExtFeat)
	fmt.Fprintf(w, "PREV_MODE=%q\n", pb.Mode)
	fmt.Fprintf(w, "PREV_NUM=%q\n", strconv.Itoa(pb.Num))
	fmt.Fprintf(w, "PREV_KNAME=%q\n", pb.KernelName)
	fmt.Fprintf(w, "PREV_DATE=%q\n", pb.Date)
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Empty reports whether the prev-build record is blank (no run captured
// yet), so callers can show "No previous build recorded".
func (pb PrevBuild) Empty() bool {
	return pb.Num == 0 && pb.Cap == "" && pb.Mode == "" && pb.Date == ""
}

// ReadBuildNumber returns the rolling counter value for the next build:
// `cat .build_number + 1`, or `1` if the file is missing/invalid. This
// matches the bash `_read_build_num` semantics where the on-disk value
// is the *last successful* build number.
func ReadBuildNumber(kernelDir string) int {
	p := filepath.Join(kernelDir, FileBuildNumber)
	b, err := os.ReadFile(p)
	if err != nil {
		return 1
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 {
		return 1
	}
	return n + 1
}

// CommitBuildNumber writes n to `.build_number` after a successful build.
func CommitBuildNumber(kernelDir string, n int) error {
	if n < 0 {
		n = 0
	}
	p := filepath.Join(kernelDir, FileBuildNumber)
	return atomicWrite(p, []byte(strconv.Itoa(n)+"\n"))
}

// ResetBuildNumber zeros `.build_number` and `out/.version` (the kernel
// build's own counter). Both writes are best-effort.
func ResetBuildNumber(kernelDir, outDir string) error {
	if err := atomicWrite(filepath.Join(kernelDir, FileBuildNumber), []byte("0\n")); err != nil {
		return err
	}
	if outDir != "" {
		_ = os.MkdirAll(outDir, 0o755)
		_ = os.WriteFile(filepath.Join(outDir, ".version"), []byte("0\n"), 0o644)
	}
	return nil
}

// MenuconfigPreservePath returns the on-disk path used to stash a
// preserved menuconfig .config across runs.
func MenuconfigPreservePath(kernelDir string) string {
	return filepath.Join(kernelDir, FileMenuconfigPreserve)
}

// MenuconfigPreserved reports whether a preserved .config exists.
func MenuconfigPreserved(kernelDir string) bool {
	_, err := os.Stat(MenuconfigPreservePath(kernelDir))
	return err == nil
}

// ClearMenuconfigPreserve removes any preserved .config (best-effort).
func ClearMenuconfigPreserve(kernelDir string) {
	_ = os.Remove(MenuconfigPreservePath(kernelDir))
}

// SaveMenuconfigPreserve copies srcDotConfig → `.menuconfig_saved_config`.
func SaveMenuconfigPreserve(kernelDir, srcDotConfig string) error {
	dst := MenuconfigPreservePath(kernelDir)
	in, err := os.ReadFile(srcDotConfig)
	if err != nil {
		return err
	}
	return atomicWrite(dst, in)
}

// ── helpers ─────────────────────────────────────────────────────────────

func parseBool(v string) bool {
	switch strings.ToLower(v) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func atomicWrite(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
