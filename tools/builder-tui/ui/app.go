package ui

import (
	"context"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/state"
)

// Screen is a discriminator for which child model is active.
type Screen int

const (
	ScreenMain Screen = iota
	ScreenSetup
	ScreenToolchain
	ScreenKSU
	ScreenBuild
	ScreenFeatures
	ScreenBuildOptions
	ScreenDeps
)

// Program is set by the entrypoint (cmd.Execute) once tea.NewProgram returns,
// so background goroutines can post messages back into the event loop via
// Program.Send. nil before Init().
var Program *tea.Program

// App is the root tea.Model. It holds shared state (config, paths, terminal
// size) and delegates render/update to the active child screen.
type App struct {
	Cfg      config.Config
	Paths    discover.Paths
	Width    int
	Height   int
	Screen   Screen
	Toast    string
	ToastErr bool

	// Builder is the persisted .builder_state (incremental, ccache, branch,
	// force-clean reason). Reloaded after every persistence-affecting action.
	Builder state.Builder
	// PrevBuild is the last successful build summary (rendered on the Mode
	// menu's gray "Previous Build" panel).
	PrevBuild state.PrevBuild
	// HasImage is true when out/arch/arm64/boot/Image exists, gating the
	// package-only entry on the mode menu.
	HasImage bool
	// MenuconfigPreserved is true when .menuconfig_saved_config exists and
	// will be restored on the next build (banner shown on Mode menu).
	MenuconfigPreserved bool
	// MenuconfigUsed is true when the user ran [M] menuconfig in the
	// current session and the .config mtime advanced -- next build's
	// Stage 2 will use olddefconfig instead of make <defconfig>.
	MenuconfigUsed bool
	// PackageOnly arms the next BuildScreen run as Stage-5-only (skip
	// clean/defconfig/guard/compile). Set by [P] on the main menu and
	// cleared after the run starts.
	PackageOnly bool

	main       MainMenu
	toolchain  ToolchainScreen
	ksu        KSUScreen
	build      BuildScreen
	setup      SetupScreen
	features   FeaturesScreen
	buildOpts  BuildOptionsScreen
	deps       DepsScreen
}

// NewApp constructs the root app and pre-runs path autodiscovery.
func NewApp(cfg config.Config) *App {
	p, _ := discover.Resolve(".",
		cfg.KernelDir, cfg.ClangDir, cfg.AnyKernelDir, cfg.OutputDir,
		cfg.GCC64Dir, cfg.GCC32Dir)
	a := &App{
		Cfg:    cfg,
		Paths:  p,
		Screen: ScreenMain,
		Width:  80,
		Height: 24,
	}
	a.refreshPersistence()
	a.main = NewMainMenu(a)
	a.toolchain = NewToolchainScreen(a)
	a.ksu = NewKSUScreen(a)
	a.build = NewBuildScreen(a)
	a.setup = NewSetupScreen(a)
	a.features = NewFeaturesScreen(a)
	a.buildOpts = NewBuildOptionsScreen(a)
	a.deps = NewDepsScreen(a)
	return a
}

// Init kicks off background commands the active screen wants on entry.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.main.Init(),
		a.toolchain.Init(),
		a.ksu.Init(),
		a.setup.Init(),
	)
}

// Update routes messages to the active child screen, intercepting global
// keys (q/ctrl+c quits; esc returns to main from sub-screens). Window-resize
// events are broadcast to *every* child so screens which weren't active at
// startup (build, ksu, …) still see the correct terminal dimensions when the
// user finally navigates to them.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.Width = m.Width
		a.Height = m.Height
		Width = m.Width
		var cmds []tea.Cmd
		var cmd tea.Cmd
		a.main, cmd = a.main.Update(msg)
		cmds = append(cmds, cmd)
		a.toolchain, cmd = a.toolchain.Update(msg)
		cmds = append(cmds, cmd)
		a.ksu, cmd = a.ksu.Update(msg)
		cmds = append(cmds, cmd)
		a.build, cmd = a.build.Update(msg)
		cmds = append(cmds, cmd)
		a.setup, cmd = a.setup.Update(msg)
		cmds = append(cmds, cmd)
		a.features, cmd = a.features.Update(msg)
		cmds = append(cmds, cmd)
		a.buildOpts, cmd = a.buildOpts.Update(msg)
		cmds = append(cmds, cmd)
		a.deps, cmd = a.deps.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)
	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.Screen == ScreenMain {
				return a, tea.Quit
			}
			a.Screen = ScreenMain
			return a, nil
		case "esc":
			if a.Screen != ScreenMain {
				a.Screen = ScreenMain
				return a, nil
			}
		}
	}
	switch a.Screen {
	case ScreenMain:
		var cmd tea.Cmd
		a.main, cmd = a.main.Update(msg)
		return a, cmd
	case ScreenToolchain:
		var cmd tea.Cmd
		a.toolchain, cmd = a.toolchain.Update(msg)
		return a, cmd
	case ScreenKSU:
		var cmd tea.Cmd
		a.ksu, cmd = a.ksu.Update(msg)
		return a, cmd
	case ScreenBuild:
		var cmd tea.Cmd
		a.build, cmd = a.build.Update(msg)
		return a, cmd
	case ScreenSetup:
		var cmd tea.Cmd
		a.setup, cmd = a.setup.Update(msg)
		return a, cmd
	case ScreenFeatures:
		var cmd tea.Cmd
		a.features, cmd = a.features.Update(msg)
		return a, cmd
	case ScreenBuildOptions:
		var cmd tea.Cmd
		a.buildOpts, cmd = a.buildOpts.Update(msg)
		return a, cmd
	case ScreenDeps:
		var cmd tea.Cmd
		a.deps, cmd = a.deps.Update(msg)
		return a, cmd
	}
	return a, nil
}

// View renders the active screen.
func (a *App) View() string {
	switch a.Screen {
	case ScreenToolchain:
		return a.toolchain.View()
	case ScreenKSU:
		return a.ksu.View()
	case ScreenBuild:
		return a.build.View()
	case ScreenSetup:
		return a.setup.View()
	case ScreenFeatures:
		return a.features.View()
	case ScreenBuildOptions:
		return a.buildOpts.View()
	case ScreenDeps:
		return a.deps.View()
	default:
		return a.main.View()
	}
}

// PersistConfig writes the current Config to disk and best-effort updates the
// in-app cache. Non-fatal on error -- shows a toast.
func (a *App) PersistConfig() {
	if err := a.Cfg.Save(); err != nil {
		a.Toast = "Config save failed: " + err.Error()
		a.ToastErr = true
		return
	}
	a.Toast = "Config saved."
	a.ToastErr = false
}

// refreshPersistence reloads the per-kernel-tree state files plus the
// out/Image / preserved-menuconfig flags. Call after any action that may
// have mutated them (build, branch switch, counter reset, menuconfig).
func (a *App) refreshPersistence() {
	if a.Paths.Kernel != "" {
		a.Builder = state.LoadBuilder(a.Paths.Kernel)
		a.PrevBuild = state.LoadPrevBuild(a.Paths.Kernel)
		a.MenuconfigPreserved = state.MenuconfigPreserved(a.Paths.Kernel)
	}
	a.HasImage = false
	if a.Paths.Output != "" {
		if _, err := os.Stat(filepath.Join(a.Paths.Output, "arch", "arm64", "boot", "Image")); err == nil {
			a.HasImage = true
		}
	}
}

// Bg returns a fresh detachable context for background ops launched via Cmd.
func (a *App) Bg() context.Context { return context.Background() }
