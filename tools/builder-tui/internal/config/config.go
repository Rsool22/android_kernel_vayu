// Package config persists user-visible toolchain selections so the next
// session opens at the same defaults. File lives at
// $XDG_CONFIG_HOME/vayu_builder/config (typically ~/.config/vayu_builder/config).
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClangSource is the user-selected clang origin.
type ClangSource string

const (
	ClangAuto   ClangSource = "auto"   // Google primary, ZyC fallback
	ClangGoogle ClangSource = "google" // AOSP gitiles
	ClangZyC    ClangSource = "zyc"    // ZyCromerZ/Clang releases
)

// Config holds the persisted user preferences.
type Config struct {
	// KernelDir, when set, overrides autodiscovery for the kernel root.
	KernelDir string
	// ClangDir, when set, overrides autodiscovery for the clang install.
	ClangDir string
	// AnyKernelDir, when set, overrides autodiscovery for AnyKernel3.
	AnyKernelDir string
	// OutputDir, when set, overrides KBUILD_OUTPUT.
	OutputDir string
	// GCC64Dir is the directory containing aarch64-linux-gnu-* tools (default /usr/bin).
	GCC64Dir string
	// GCC32Dir is the directory containing arm-linux-gnueabi-* tools (default /usr/bin).
	GCC32Dir string

	// KernelName, when set, becomes the LOCALVERSION suffix on the next build.
	// Cleared at the end of every build cycle (session-only persistence).
	KernelName string

	// ClangSource selects the upstream for fetches.
	ClangSource ClangSource
	// ZyCTarget is "latest", "23", "15", etc.
	ZyCTarget string
	// GoogleTarget is "latest", "r596125", etc.
	GoogleTarget string

	// KSUBranch is the active ReSukiSU branch ("main" or "dev").
	KSUBranch string
}

// Defaults returns a sensible starting Config.
func Defaults() Config {
	return Config{
		ClangSource:  ClangAuto,
		ZyCTarget:    "latest",
		GoogleTarget: "latest",
		KSUBranch:    "main",
	}
}

// Path returns the config file path, creating its directory if missing.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "vayu_builder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

// Load reads the config file. Missing file returns Defaults() with no error.
func Load() (Config, error) {
	c := Defaults()
	p, err := Path()
	if err != nil {
		return c, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
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
		case "kernel_dir":
			c.KernelDir = v
		case "clang_dir":
			c.ClangDir = v
		case "anykernel_dir":
			c.AnyKernelDir = v
		case "output_dir":
			c.OutputDir = v
		case "gcc64_dir":
			c.GCC64Dir = v
		case "gcc32_dir":
			c.GCC32Dir = v
		case "kernel_name":
			c.KernelName = v
		case "clang_source":
			c.ClangSource = ClangSource(v)
		case "zyc_target":
			c.ZyCTarget = v
		case "google_target":
			c.GoogleTarget = v
		case "ksu_branch":
			c.KSUBranch = v
		}
	}
	if err := s.Err(); err != nil {
		return c, err
	}
	c.normalize()
	return c, nil
}

// Save writes the config atomically.
func (c Config) Save() error {
	c.normalize()
	p, err := Path()
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "# vayu_builder config -- managed by tools/builder-tui")
	fmt.Fprintln(w, "# Lines beginning with '#' are ignored.")
	fmt.Fprintln(w)
	writeOpt := func(k, v string) {
		if v != "" {
			fmt.Fprintf(w, "%s=%s\n", k, v)
		}
	}
	writeOpt("kernel_dir", c.KernelDir)
	writeOpt("clang_dir", c.ClangDir)
	writeOpt("anykernel_dir", c.AnyKernelDir)
	writeOpt("output_dir", c.OutputDir)
	writeOpt("gcc64_dir", c.GCC64Dir)
	writeOpt("gcc32_dir", c.GCC32Dir)
	writeOpt("kernel_name", c.KernelName)
	writeOpt("clang_source", string(c.ClangSource))
	writeOpt("zyc_target", c.ZyCTarget)
	writeOpt("google_target", c.GoogleTarget)
	writeOpt("ksu_branch", c.KSUBranch)
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (c *Config) normalize() {
	switch c.ClangSource {
	case ClangAuto, ClangGoogle, ClangZyC:
	default:
		c.ClangSource = ClangAuto
	}
	if c.ZyCTarget == "" {
		c.ZyCTarget = "latest"
	}
	if c.GoogleTarget == "" {
		c.GoogleTarget = "latest"
	}
	if c.KSUBranch != "main" && c.KSUBranch != "dev" {
		c.KSUBranch = "main"
	}
}
