// Package features models the kernel-config feature toggles used by the
// vayu Bubble Tea builder: ReSukiSU (CONFIG_KSU), SuSFS
// (CONFIG_KSU_SUSFS), KPM (CONFIG_KPM) and the related manual-hook key
// (CONFIG_KSU_MANUAL_HOOK). It reads/writes vayu_defconfig in the kernel
// source tree using the same semantics the original bash build script
// used (sed-style edit-or-append).
package features

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Keys edited by this package. Exposed so callers can render labels.
const (
	KeyKSU        = "CONFIG_KSU"
	KeySUSFS      = "CONFIG_KSU_SUSFS"
	KeyKPM        = "CONFIG_KPM"
	KeyManualHook = "CONFIG_KSU_MANUAL_HOOK"
)

// State captures the on-disk feature toggles read from defconfig plus
// whether the kernel currently has the ReSukiSU driver source tree
// installed (drivers/kernelsu).
type State struct {
	KSU           bool
	SUSFS         bool
	KPM           bool
	ManualHook    bool
	DriverPresent bool
}

// HookMode returns the human-readable hook mode implied by the state:
// "SuSFS-Inline-Hook" when SuSFS is on, "Manual-Hook" when KSU is on
// without SuSFS, "N/A" otherwise.
func (s State) HookMode() string {
	switch {
	case !s.DriverPresent || !s.KSU:
		return "N/A"
	case s.SUSFS:
		return "SuSFS-Inline-Hook"
	default:
		return "Manual-Hook"
	}
}

// CapTag returns the bash script's [DEV-ReSukiSU | Hook-Mode=…] style
// caption used for build artefact naming.
func (s State) CapTag(branch string) string {
	if !s.KSU {
		return "[Vanilla]"
	}
	tag := strings.ToUpper(branch)
	if tag == "" {
		tag = "MAIN"
	}
	return fmt.Sprintf("[%s-ReSukiSU | Hook-Mode=%s]", tag, s.HookMode())
}

// ExtTag returns "[SuSFS + KPM]" / "[SuSFS]" / "[KPM]" / "[None]" -- the
// extra-feature summary used in zip names.
func (s State) ExtTag() string {
	if !s.KSU {
		return "[None]"
	}
	parts := []string{}
	if s.SUSFS {
		parts = append(parts, "SuSFS")
	}
	if s.KPM {
		parts = append(parts, "KPM")
	}
	if len(parts) == 0 {
		return "[None]"
	}
	return "[" + strings.Join(parts, " + ") + "]"
}

// Read parses the supplied defconfig file and returns the current
// feature state. Missing keys are treated as disabled.
func Read(defconfig string) (State, error) {
	st := State{}
	f, err := os.Open(defconfig)
	if err != nil {
		return st, err
	}
	defer f.Close()

	on := func(line, key string) bool {
		return line == key+"=y"
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case on(line, KeyKSU):
			st.KSU = true
		case on(line, KeySUSFS):
			st.SUSFS = true
		case on(line, KeyKPM):
			st.KPM = true
		case on(line, KeyManualHook):
			st.ManualHook = true
		}
	}
	return st, sc.Err()
}

// DriverPresent returns true if the ReSukiSU driver source tree is
// installed inside the kernel (drivers/kernelsu/Kconfig present).
func DriverPresent(kernelDir string) bool {
	if kernelDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(kernelDir, "drivers", "kernelsu", "Kconfig"))
	return err == nil
}

// ToggleConfig flips a single CONFIG_* key in the supplied defconfig.
// Mirrors bash:
//
//	sed -i -e "s|^# ${cfg} is not set|<new>|" -e "s|^${cfg}=.*|<new>|" defconfig
//	[ ! found ] && echo "$new_line" >> defconfig
//
// `enabled=true`  -> writes `KEY=y`.
// `enabled=false` -> writes `# KEY is not set`.
func ToggleConfig(defconfig, key string, enabled bool) error {
	if defconfig == "" {
		return errors.New("defconfig path is empty")
	}
	if key == "" {
		return errors.New("config key is empty")
	}
	newLine := "# " + key + " is not set"
	if enabled {
		newLine = key + "=y"
	}

	in, err := os.ReadFile(defconfig)
	if err != nil {
		return err
	}
	lines := strings.Split(string(in), "\n")
	reEnabled := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `=`)
	reDisabled := regexp.MustCompile(`^# ` + regexp.QuoteMeta(key) + ` is not set\s*$`)

	found := false
	for i, l := range lines {
		switch {
		case reEnabled.MatchString(l):
			lines[i] = newLine
			found = true
		case reDisabled.MatchString(l):
			lines[i] = newLine
			found = true
		}
	}
	if !found {
		// Append before the final empty line if any.
		lines = append(lines, newLine)
	}
	out := strings.Join(lines, "\n")
	return os.WriteFile(defconfig, []byte(out), 0o644)
}

// Apply writes all toggles from the supplied state to defconfig in one
// pass, deriving CONFIG_KSU_MANUAL_HOOK from the (KSU, SUSFS) pair. This
// is what the bash build runner does at Stage 2 just before make.
func Apply(defconfig string, s State) error {
	hook := s.KSU && !s.SUSFS
	for _, p := range []struct {
		k string
		v bool
	}{
		{KeyKSU, s.KSU},
		{KeySUSFS, s.SUSFS},
		{KeyKPM, s.KPM},
		{KeyManualHook, hook},
	} {
		if err := ToggleConfig(defconfig, p.k, p.v); err != nil {
			return fmt.Errorf("toggle %s: %w", p.k, err)
		}
	}
	return nil
}

// ToggleKSU flips ReSukiSU; cascading rule mirrors the bash build:
// disabling KSU also disables SUSFS, KPM and ManualHook; enabling KSU
// turns ManualHook on when SUSFS is off.
func ToggleKSU(defconfig string, s State) (State, error) {
	if !s.DriverPresent {
		return s, errors.New("ReSukiSU driver not installed (use ReSukiSU manager [I] first)")
	}
	if s.KSU {
		s.KSU, s.SUSFS, s.KPM, s.ManualHook = false, false, false, false
	} else {
		s.KSU = true
		if !s.SUSFS {
			s.ManualHook = true
		}
	}
	return s, Apply(defconfig, s)
}

// ToggleSUSFS flips SuSFS. SuSFS requires KSU; toggling SuSFS off when
// KSU is on auto-enables ManualHook (and vice-versa).
func ToggleSUSFS(defconfig string, s State) (State, error) {
	if !s.DriverPresent {
		return s, errors.New("ReSukiSU driver not installed")
	}
	if !s.KSU {
		return s, errors.New("enable ReSukiSU first -- SuSFS requires it")
	}
	if s.SUSFS {
		s.SUSFS = false
		s.ManualHook = true
	} else {
		s.SUSFS = true
		s.ManualHook = false
	}
	return s, Apply(defconfig, s)
}

// ToggleKPM flips KPM. KPM requires KSU but is otherwise independent
// of SuSFS.
func ToggleKPM(defconfig string, s State) (State, error) {
	if !s.DriverPresent {
		return s, errors.New("ReSukiSU driver not installed")
	}
	if !s.KSU {
		return s, errors.New("enable ReSukiSU first -- KPM requires it")
	}
	s.KPM = !s.KPM
	return s, Apply(defconfig, s)
}
