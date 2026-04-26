package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui/components"
)

// dep is one host-dependency probe row.
type dep struct {
	name   string // human label (e.g. "git")
	probe  string // command to look up
	pkg    map[discover.PackageManager]string
	header string // optional path probe (e.g. "/usr/include/openssl/ssl.h")
}

// hostDeps lists every dep the build needs. Mirrors bash deps_check.
var hostDeps = []dep{
	{name: "git", probe: "git", pkg: pkgEverywhere("git")},
	{name: "make", probe: "make", pkg: pkgEverywhere("make")},
	{name: "ccache", probe: "ccache", pkg: pkgEverywhere("ccache")},
	{name: "bc", probe: "bc", pkg: pkgEverywhere("bc")},
	{name: "bison", probe: "bison", pkg: pkgEverywhere("bison")},
	{name: "flex", probe: "flex", pkg: pkgEverywhere("flex")},
	{name: "zip", probe: "zip", pkg: pkgEverywhere("zip")},
	{name: "unzip", probe: "unzip", pkg: pkgEverywhere("unzip")},
	{name: "rsync", probe: "rsync", pkg: pkgEverywhere("rsync")},
	{name: "python3", probe: "python3", pkg: pkgEverywhere("python3")},
	{name: "curl", probe: "curl", pkg: pkgEverywhere("curl")},
	{name: "tar", probe: "tar", pkg: pkgEverywhere("tar")},
	{name: "build-essential", probe: "gcc", pkg: map[discover.PackageManager]string{
		discover.PMApt: "build-essential", discover.PMDnf: "gcc gcc-c++",
		discover.PMPacman: "base-devel", discover.PMZypper: "gcc gcc-c++",
		discover.PMApk: "build-base",
	}},
	{name: "aarch64-gcc", probe: "aarch64-linux-gnu-gcc", pkg: map[discover.PackageManager]string{
		discover.PMApt: "gcc-aarch64-linux-gnu", discover.PMDnf: "gcc-aarch64-linux-gnu",
		discover.PMPacman: "aarch64-linux-gnu-gcc", discover.PMZypper: "cross-aarch64-gcc11",
		discover.PMApk: "binutils-aarch64",
	}},
	{name: "arm-gcc", probe: "arm-linux-gnueabi-gcc", pkg: map[discover.PackageManager]string{
		discover.PMApt: "gcc-arm-linux-gnueabi", discover.PMDnf: "gcc-arm-linux-gnu",
		discover.PMPacman: "arm-linux-gnueabihf-gcc", discover.PMZypper: "cross-arm-gcc11",
		discover.PMApk: "binutils-arm",
	}},
	{name: "openssl-dev", probe: "openssl", header: "/usr/include/openssl/ssl.h", pkg: map[discover.PackageManager]string{
		discover.PMApt: "libssl-dev", discover.PMDnf: "openssl-devel",
		discover.PMPacman: "openssl", discover.PMZypper: "libopenssl-devel",
		discover.PMApk: "openssl-dev",
	}},
	{name: "libelf-dev", probe: "", header: "/usr/include/libelf.h", pkg: map[discover.PackageManager]string{
		discover.PMApt: "libelf-dev", discover.PMDnf: "elfutils-libelf-devel",
		discover.PMPacman: "libelf", discover.PMZypper: "libelf-devel",
		discover.PMApk: "elfutils-dev",
	}},
}

func pkgEverywhere(name string) map[discover.PackageManager]string {
	return map[discover.PackageManager]string{
		discover.PMApt:    name,
		discover.PMDnf:    name,
		discover.PMPacman: name,
		discover.PMZypper: name,
		discover.PMApk:    name,
	}
}

// depRow is one rendered row.
type depRow struct {
	dep     dep
	present bool
	resolved string // path or version
}

// DepsScreen displays the host-dependency probe table and offers an
// install command for whichever rows are missing.
type DepsScreen struct {
	app    *App
	rows   []depRow
	probed bool
	busy   bool
	stage  string
}

func NewDepsScreen(a *App) DepsScreen { return DepsScreen{app: a} }

func (s DepsScreen) Init() tea.Cmd { return s.probeCmd() }

type depsProbedMsg struct{ rows []depRow }
type depsInstallDoneMsg struct{ err error }

func (s DepsScreen) probeCmd() tea.Cmd {
	return func() tea.Msg {
		var rows []depRow
		for _, d := range hostDeps {
			r := depRow{dep: d}
			if d.probe != "" {
				if p, err := exec.LookPath(d.probe); err == nil {
					r.present = true
					r.resolved = p
				}
			}
			if !r.present && d.header != "" {
				if _, err := execStat(d.header); err == nil {
					r.present = true
					r.resolved = d.header
				}
			}
			rows = append(rows, r)
		}
		return depsProbedMsg{rows: rows}
	}
}

func execStat(p string) (interface{}, error) {
	cmd := exec.Command("test", "-e", p)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (s DepsScreen) missingPkgs() []string {
	var out []string
	for _, r := range s.rows {
		if !r.present {
			if pkg := r.dep.pkg[s.app.Paths.Distro]; pkg != "" {
				out = append(out, pkg)
			}
		}
	}
	return out
}

// installCmd runs the host package manager to install the missing deps
// and streams stdout/stderr line-by-line to the shared Activity log so
// the user sees apt/dnf progress without having to leave the TUI. The
// final depsInstallDoneMsg carries any error from Wait/Run.
func (s DepsScreen) installCmd(pkgs []string) tea.Cmd {
	return func() tea.Msg {
		if len(pkgs) == 0 {
			return depsInstallDoneMsg{}
		}
		var args []string
		switch s.app.Paths.Distro {
		case discover.PMApt:
			// Update first so new packages resolve; chain with &&.
			args = []string{"sh", "-c",
				"sudo apt-get update && sudo apt-get install -y " + strings.Join(pkgs, " "),
			}
		case discover.PMDnf:
			args = append([]string{"sudo", "dnf", "install", "-y"}, pkgs...)
		case discover.PMPacman:
			args = append([]string{"sudo", "pacman", "-S", "--needed", "--noconfirm"}, pkgs...)
		case discover.PMZypper:
			args = append([]string{"sudo", "zypper", "--non-interactive", "install"}, pkgs...)
		case discover.PMApk:
			args = append([]string{"sudo", "apk", "add"}, pkgs...)
		default:
			return depsInstallDoneMsg{err: fmt.Errorf("unsupported distro: %s", s.app.Paths.Distro)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return depsInstallDoneMsg{err: err}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return depsInstallDoneMsg{err: err}
		}
		if err := cmd.Start(); err != nil {
			return depsInstallDoneMsg{err: err}
		}
		log := s.app.Activity
		go streamTo(stdout, log, components.ActInfo)
		go streamTo(stderr, log, components.ActWarn)
		if err := cmd.Wait(); err != nil {
			return depsInstallDoneMsg{err: err}
		}
		return depsInstallDoneMsg{}
	}
}

// streamTo copies a reader line-by-line into an ActivityLog. Used for
// piping child-process stdout / stderr to the UI without blocking the
// tea event loop.
func streamTo(r io.Reader, log *components.ActivityLog, lvl components.ActivityLevel) {
	if log == nil {
		_, _ = io.Copy(io.Discard, r)
		return
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		log.Append(lvl, line)
	}
}

func (s DepsScreen) Update(msg tea.Msg) (DepsScreen, tea.Cmd) {
	switch m := msg.(type) {
	case depsProbedMsg:
		s.rows = m.rows
		s.probed = true
		s.busy = false
		s.stage = ""
	case depsInstallDoneMsg:
		s.busy = false
		s.stage = ""
		if m.err != nil {
			return s, s.app.SetToast("Install failed: "+m.err.Error(), true)
		}
		return s, tea.Batch(
			s.app.SetToast("Install complete -- re-probing", false),
			s.probeCmd(),
		)
	case tea.KeyMsg:
		if s.busy {
			return s, nil
		}
		switch strings.ToLower(m.String()) {
		case "p":
			s.busy = true
			s.stage = "probing host dependencies"
			return s, s.probeCmd()
		case "i":
			pkgs := s.missingPkgs()
			if len(pkgs) == 0 {
				return s, s.app.SetToast("All dependencies present -- nothing to install", false)
			}
			if s.app.Paths.Distro == discover.PMUnknown {
				return s, s.app.SetToast("Unknown distro -- install manually", true)
			}
			s.busy = true
			s.stage = "installing " + fmt.Sprint(len(pkgs)) + " packages"
			if s.app.Activity != nil {
				s.app.Activity.Info("$ " + s.installPreview(pkgs))
			}
			return s, s.installCmd(pkgs)
		case "c":
			pkgs := s.missingPkgs()
			if len(pkgs) == 0 {
				return s, s.app.SetToast("Nothing missing", false)
			}
			return s, s.app.SetToast("$ "+s.installPreview(pkgs), false)
		}
	}
	return s, nil
}

func (s DepsScreen) installPreview(pkgs []string) string {
	switch s.app.Paths.Distro {
	case discover.PMApt:
		return "sudo apt-get install -y " + strings.Join(pkgs, " ")
	case discover.PMDnf:
		return "sudo dnf install -y " + strings.Join(pkgs, " ")
	case discover.PMPacman:
		return "sudo pacman -S --needed " + strings.Join(pkgs, " ")
	case discover.PMZypper:
		return "sudo zypper install " + strings.Join(pkgs, " ")
	case discover.PMApk:
		return "sudo apk add " + strings.Join(pkgs, " ")
	}
	return "(unknown distro)"
}

func (s DepsScreen) View() string {
	w := panelWidth(s.app.Width)

	banner := components.Banner(
		"DEPENDENCY  CHECK",
		"host packages required to build the kernel",
		w, BannerBorder, BannerTitle, BannerSubtle,
	)

	// Status / distro panel
	var hdr strings.Builder
	hdr.WriteString(components.KV("Distro", string(s.app.Paths.Distro), 12, LabelStyle, ValueStyle) + "\n")
	missing := s.missingPkgs()
	if !s.probed {
		hdr.WriteString(components.KV("State", "probing…", 12, LabelStyle, AccentText))
	} else if len(missing) == 0 {
		hdr.WriteString(components.KV("State", "all good", 12, LabelStyle, OKText))
	} else {
		hdr.WriteString(components.KV("State", fmt.Sprintf("%d missing", len(missing)), 12, LabelStyle, ErrText))
	}
	statusPanel := components.Panel("Host status", hdr.String(), w, PanelBorder, TitleStyle)

	// Probe table — build a simple aligned ASCII table (no bubbles/table for
	// readability with our color theme).
	var tbl strings.Builder
	colName := 22
	colStatus := 12
	tbl.WriteString(LabelStyle.Render(padTo("PACKAGE", colName)) +
		"  " + LabelStyle.Render(padTo("STATE", colStatus)) +
		"  " + LabelStyle.Render("RESOLVED / PKG NAME") + "\n")
	for _, r := range s.rows {
		state := ErrText.Render(padTo("MISSING", colStatus))
		resolved := r.dep.pkg[s.app.Paths.Distro]
		if r.present {
			state = OKText.Render(padTo("PRESENT", colStatus))
			resolved = r.resolved
		}
		tbl.WriteString(ValueStyle.Render(padTo(r.dep.name, colName)) +
			"  " + state +
			"  " + DimText.Render(resolved) + "\n")
	}
	tablePanel := components.Panel("Probes", strings.TrimRight(tbl.String(), "\n"), w, PanelBorder, TitleStyle)

	// Keys panel
	keysPanel := components.KeysPanel([]components.Hotkey{
		{Key: "P", Desc: "Re-probe"},
		{Key: "C", Desc: "Show install cmd"},
		{Key: "I", Desc: "Install missing", Sub: "via " + string(s.app.Paths.Distro)},
		{Key: "ESC", Desc: "Back"},
	}, w, PanelDim, TitleStyle.Foreground(ColorDim),
		HotKeyStyle, ValueStyle, DimText, MutedText)

	var busyLine string
	if s.busy {
		busyLine = "\n  " + lipgloss.NewStyle().Foreground(ColorAccent).Render("⟳  "+s.stage)
	}

	out := banner + "\n" + statusPanel + "\n" + tablePanel + "\n"
	if s.app.Activity != nil {
		out += components.ActivityPanel(s.app.Activity, w, 5, PanelDim, TitleStyle.Foreground(ColorDim)) + "\n"
	}
	out += keysPanel + busyLine + "\n"
	if s.app.Toast != "" {
		out += "\n  " + components.Toast(s.app.Toast, s.app.ToastErr) + "\n"
	}
	return out
}
