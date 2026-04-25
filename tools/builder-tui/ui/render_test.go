package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
)

// TestRenderSnapshots builds an App, drives each screen's View() at a
// representative terminal size, and writes the rendered, ANSI-stripped
// output to /tmp/vayu-render-<screen>.txt so we can eyeball alignment
// without launching the live TUI.
func TestRenderSnapshots(t *testing.T) {
	if os.Getenv("RENDER_SNAP") == "" {
		t.Skip("set RENDER_SNAP=1 to dump snapshots")
	}
	cfg := config.Defaults()
	a := NewApp(cfg)
	a.Paths = discover.Paths{
		Kernel:    "/home/ubuntu/rsool-16",
		Clang:     "/home/ubuntu/rsool-16/clang",
		AnyKernel: "/home/ubuntu/rsool-16/AnyKernel3",
		Output:    "/home/ubuntu/rsool-16/out",
		GccArm64:  "/usr/bin/aarch64-linux-gnu-",
		GccArm:    "/usr/bin/arm-linux-gnueabi-",
		Distro:    discover.PMApt,
	}
	a.Width = 120
	a.Height = 50
	if w := os.Getenv("RENDER_W"); w != "" {
		fmt.Sscanf(w, "%d", &a.Width)
	}

	dump := func(name, body string) {
		path := fmt.Sprintf("/tmp/vayu-render-%s.txt", name)
		if err := os.WriteFile(path, []byte(stripANSI(body)), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d bytes)", path, len(body))
	}

	a.Screen = ScreenMain
	dump("main", a.View())
	a.Screen = ScreenBuildOptions
	dump("build_options", a.View())
	a.Screen = ScreenFeatures
	a.features.refresh()
	dump("features", a.View())
	a.Screen = ScreenSetup
	dump("setup", a.View())
	a.Screen = ScreenPaths
	dump("paths", a.View())
	a.Screen = ScreenDeps
	dump("deps", a.View())
	a.Screen = ScreenToolchain
	dump("toolchain", a.View())
	a.Screen = ScreenKSU
	dump("ksu", a.View())
	a.Screen = ScreenBuild
	dump("build", a.View())
}

func stripANSI(s string) string {
	var b strings.Builder
	skip := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			skip = true
			continue
		}
		if skip {
			if c >= '@' && c <= '~' {
				skip = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var _ = lipgloss.Width
