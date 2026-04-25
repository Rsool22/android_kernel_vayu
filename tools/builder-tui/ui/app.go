package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
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
)

// Program is set by the entrypoint (cmd.Execute) once tea.NewProgram returns,
// so background goroutines can post messages back into the event loop via
// Program.Send. nil before Init().
var Program *tea.Program

// App is the root tea.Model. It holds shared state (config, paths, terminal
// size) and delegates render/update to the active child screen.
type App struct {
	Cfg       config.Config
	Paths     discover.Paths
	Width     int
	Height    int
	Screen    Screen
	Toast     string
	ToastErr  bool

	main      MainMenu
	toolchain ToolchainScreen
	ksu       KSUScreen
	build     BuildScreen
	setup     SetupScreen
	features  FeaturesScreen
}

// NewApp constructs the root app and pre-runs path autodiscovery.
func NewApp(cfg config.Config) *App {
	p, _ := discover.Resolve(".", cfg.KernelDir, cfg.ClangDir, cfg.AnyKernelDir, cfg.OutputDir)
	a := &App{
		Cfg:    cfg,
		Paths:  p,
		Screen: ScreenMain,
		Width:  80,
		Height: 24,
	}
	a.main = NewMainMenu(a)
	a.toolchain = NewToolchainScreen(a)
	a.ksu = NewKSUScreen(a)
	a.build = NewBuildScreen(a)
	a.setup = NewSetupScreen(a)
	a.features = NewFeaturesScreen(a)
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

// Bg returns a fresh detachable context for background ops launched via Cmd.
func (a *App) Bg() context.Context { return context.Background() }
