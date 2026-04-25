// Package discover provides path autodiscovery for the kernel root, clang
// install, AnyKernel3, and the host's package manager. The kernel-root walker
// finds the source tree regardless of directory name, by sniffing well-known
// markers (Makefile with "VERSION =", arch/arm64/, kernel/).
package discover

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths is the resolved set of locations the builder needs.
type Paths struct {
	Kernel    string
	Clang     string
	AnyKernel string
	Output    string

	GccArm64    string // path to aarch64-linux-gnu-gcc (or similar)
	GccArm      string // path to arm-linux-gnueabi-gcc (or similar)

	Distro PackageManager
}

// PackageManager identifies the host distro family.
type PackageManager string

const (
	PMUnknown PackageManager = "unknown"
	PMApt     PackageManager = "apt"     // Debian, Ubuntu
	PMDnf     PackageManager = "dnf"     // Fedora, RHEL
	PMPacman  PackageManager = "pacman"  // Arch
	PMZypper  PackageManager = "zypper"  // openSUSE
	PMApk     PackageManager = "apk"     // Alpine
)

// DetectPackageManager picks the first pkg manager binary present in PATH.
func DetectPackageManager() PackageManager {
	for _, p := range []struct {
		bin string
		pm  PackageManager
	}{
		{"apt-get", PMApt},
		{"dnf", PMDnf},
		{"yum", PMDnf},
		{"pacman", PMPacman},
		{"zypper", PMZypper},
		{"apk", PMApk},
	} {
		if _, err := exec.LookPath(p.bin); err == nil {
			return p.pm
		}
	}
	return PMUnknown
}

// FindKernelRoot walks up from start, returning the directory that looks like
// a Linux kernel source tree (arch/arm64/, kernel/, Makefile w/ "VERSION =").
// If start is empty, uses cwd.
func FindKernelRoot(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for cur := abs; cur != "/" && cur != ""; cur = filepath.Dir(cur) {
		if isKernelRoot(cur) {
			return cur, nil
		}
	}
	return "", errors.New("kernel root not found (looked for Makefile + arch/arm64 + kernel/)")
}

func isKernelRoot(dir string) bool {
	mk := filepath.Join(dir, "Makefile")
	if _, err := os.Stat(mk); err != nil {
		return false
	}
	if !dirExists(filepath.Join(dir, "arch", "arm64")) {
		return false
	}
	if !dirExists(filepath.Join(dir, "kernel")) {
		return false
	}
	// Sniff first ~30 lines for "VERSION = " to confirm.
	f, err := os.Open(mk)
	if err != nil {
		return false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for i := 0; i < 30 && s.Scan(); i++ {
		if strings.HasPrefix(strings.TrimSpace(s.Text()), "VERSION =") {
			return true
		}
	}
	return false
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// FindClang resolves a clang install in priority order:
//  1. override (when non-empty and bin/clang executable)
//  2. $kernel/clang
//  3. $kernel/toolchain/clang
//  4. ~/toolchains/clang
//  5. /opt/google-clang* or /opt/zyc-clang*
//  6. /usr/lib/llvm-*/bin/clang (system)
//  7. PATH lookup of `clang`
//
// Returns the directory containing bin/clang.
func FindClang(override, kernelRoot string) string {
	if override != "" && hasClangBinary(override) {
		return override
	}
	candidates := []string{
		filepath.Join(kernelRoot, "clang"),
		filepath.Join(kernelRoot, "toolchain", "clang"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "toolchains", "clang"),
			filepath.Join(home, "kernel-builds", "vayu_a16_kernel", "clang"),
		)
	}
	for _, glob := range []string{"/opt/google-clang*", "/opt/zyc-clang*", "/usr/lib/llvm-*"} {
		matches, _ := filepath.Glob(glob)
		candidates = append(candidates, matches...)
	}
	for _, c := range candidates {
		if hasClangBinary(c) {
			return c
		}
	}
	if p, err := exec.LookPath("clang"); err == nil {
		// $clang_root is the parent of bin/clang
		return filepath.Dir(filepath.Dir(p))
	}
	return ""
}

func hasClangBinary(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "bin", "clang"))
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// FindAnyKernel resolves an AnyKernel3 dir.
func FindAnyKernel(override, kernelRoot string) string {
	if override != "" && isAnyKernel(override) {
		return override
	}
	candidates := []string{
		filepath.Join(kernelRoot, "AnyKernel3"),
		filepath.Join(filepath.Dir(kernelRoot), "AnyKernel3"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "AnyKernel3"))
	}
	for _, c := range candidates {
		if isAnyKernel(c) {
			return c
		}
	}
	return ""
}

func isAnyKernel(dir string) bool {
	if !dirExists(dir) {
		return false
	}
	for _, m := range []string{"anykernel.sh", "META-INF"} {
		if _, err := os.Stat(filepath.Join(dir, m)); err != nil {
			return false
		}
	}
	return true
}

// FindCrossCompiler returns the absolute path to a cross-compiler binary,
// or "" if not found. names are checked in order (first hit wins).
func FindCrossCompiler(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

// Resolve runs the full discovery pass and returns Paths populated for the
// builder.
//
// overrides come from config.Config. When kernelOverride is set, it wins
// outright; otherwise we walk up from start to find a kernel root. If no
// root is detected (TUI started outside any kernel tree), we DON'T return
// an error — instead we leave Kernel="" and let the caller display the
// configured override / default for the user to fix on the Setup screen.
func Resolve(start, kernelOverride, clangOverride, anyOverride, outputOverride,
	gcc64Override, gcc32Override string) (Paths, error) {
	p := Paths{
		Output: outputOverride,
		Distro: DetectPackageManager(),
	}
	root := kernelOverride
	if root == "" {
		if r, err := FindKernelRoot(start); err == nil {
			root = r
		}
	}
	p.Kernel = root
	if p.Output == "" && root != "" {
		p.Output = filepath.Join(root, "Anykernel-Builds")
	}
	p.Clang = FindClang(clangOverride, root)
	p.AnyKernel = FindAnyKernel(anyOverride, root)

	// GCC: prefer the user-configured directory (matches bash GCC{64,32}_DIR),
	// falling back to PATH lookup. Stored as a directory so Setup can show
	// it as an editable bin-dir, mirroring the bash UX.
	p.GccArm64 = resolveGccDir(gcc64Override,
		"aarch64-linux-gnu-gcc",
		"aarch64-linux-android-gcc",
		"aarch64-none-linux-gnu-gcc",
	)
	p.GccArm = resolveGccDir(gcc32Override,
		"arm-linux-gnueabi-gcc",
		"arm-linux-gnueabihf-gcc",
		"arm-linux-androideabi-gcc",
	)
	return p, nil
}

// resolveGccDir returns the directory containing one of the named gcc binaries.
// Honours `override` first; falls back to PATH lookup. Returns "" if nothing
// is found anywhere.
func resolveGccDir(override string, names ...string) string {
	if override != "" {
		for _, n := range names {
			if _, err := os.Stat(filepath.Join(override, n)); err == nil {
				return override
			}
		}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return filepath.Dir(p)
		}
	}
	return override
}
