package pipeline

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// fileExists is a thin os.Stat wrapper that ignores errors.
func fileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// copyFile mirrors `cp src dst`. Best-effort: missing src returns nil so
// optional artefacts (dtbo.img, dtb.img) are skipped without aborting.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		// Treat missing source as a no-op so optional artefacts are silent.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// tempCopy copies src to a fresh /tmp file and returns its path.
func tempCopy(src string) (string, error) {
	f, err := os.CreateTemp("", "vkb_cfg_*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	f.Close()
	if err := copyFile(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

// stripLocalversion replaces the `CONFIG_LOCALVERSION=` line in a
// generated .config with an empty value (when KERNEL_NAME is in use the
// LOCALVERSION come from the make argv instead).
func stripLocalversion(cfgPath string) error {
	in, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`(?m)^CONFIG_LOCALVERSION=.*$`)
	out := re.ReplaceAll(in, []byte(`CONFIG_LOCALVERSION=""`))
	return os.WriteFile(cfgPath, out, 0o644)
}

// streamLines pumps r line-by-line into the LineFunc-style callback.
func streamLines(r io.Reader, isErr bool, line func(string, bool)) {
	if r == nil {
		return
	}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for s.Scan() {
		line(s.Text(), isErr)
	}
}

// drain reads and discards all events on the channel until it's closed.
// Used as a fallback when the caller didn't supply an events channel.
func drain(ch <-chan Event) {
	for range ch {
	}
}

// FailErrors returns up to 4 last "ld.lld:.*error:" lines and up to 6 last
// "error:" lines (excluding ld.lld) from a fail log, mirroring the bash
// print_fail_box() filter. Both slices are returned in original (top-to-
// bottom) order so the most-recent errors render at the bottom.
func FailErrors(failLogPath string) (linker, compiler []string) {
	in, err := os.ReadFile(failLogPath)
	if err != nil {
		return nil, nil
	}
	for _, l := range strings.Split(string(in), "\n") {
		switch {
		case strings.Contains(l, "ld.lld:") && strings.Contains(l, "error:"):
			linker = append(linker, l)
		case strings.Contains(l, "error:") && !strings.Contains(l, "ld.lld:"):
			compiler = append(compiler, l)
		}
	}
	if n := len(linker); n > 4 {
		linker = linker[n-4:]
	}
	if n := len(compiler); n > 6 {
		compiler = compiler[n-6:]
	}
	return linker, compiler
}

// guardSummary extracts the `=> Summary: …` line emitted by
// scripts/apply_ksu_guards.py. Returns "" when the line is missing.
func guardSummary(stdout string) string {
	for _, l := range strings.Split(stdout, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "=> Summary:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "=> Summary:"))
		}
	}
	return ""
}

// FormatElapsed pretty-prints d as bash does in the build script:
// `<sec>s` when under a minute; `<m>m <s>s` otherwise.
func FormatElapsed(secs int) string {
	if secs < 60 {
		return itoa(secs) + "s"
	}
	return itoa(secs/60) + "m " + itoa(secs%60) + "s"
}

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
