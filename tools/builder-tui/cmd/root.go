// Package cmd is the cobra entry point for vayu-builder.
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	v "github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/version"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/ui"
)

var rootStyle string

// rootCmd, when run with no subcommand, opens the interactive TUI.
var rootCmd = &cobra.Command{
	Use:   "vayu-builder",
	Short: "Vayu kernel builder TUI",
	Long: `Vayu kernel builder.

Run with no arguments to open the interactive TUI. Subcommands let CI and
power users perform individual actions headlessly:

  vayu-builder build         compile + package the kernel
  vayu-builder fetch-clang   download Google AOSP or ZyC clang
  vayu-builder probe         show ReSukiSU branch availability
  vayu-builder paths         dump autodiscovered paths
  vayu-builder version       print build identity and exit
`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: load config:", err)
			cfg = config.Defaults()
		}
		// Theme precedence: --style flag → persisted Cfg.Theme → "bash".
		// Unknown values fall back to bash with a warning so a stale
		// config never leaves the screen unstyled.
		theme := rootStyle
		if theme == "" {
			theme = cfg.Theme
		}
		known := false
		for _, n := range ui.StyleNames {
			if n == theme {
				known = true
				break
			}
		}
		if !known && theme != "" {
			fmt.Fprintf(os.Stderr, "warning: unknown theme %q (using bash)\n", theme)
			theme = "bash"
		}
		ui.ApplyStyle(ui.ParseStyle(theme))
		app := ui.NewApp(cfg)
		p := tea.NewProgram(app, tea.WithAltScreen())
		ui.Program = p
		_, err = p.Run()
		return err
	},
}

// Execute runs the root command. Called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootStyle, "style", "bash",
		"visual variant: bash (double-line, magenta/cyan/yellow) | modern (rounded, soft palette)")
	rootCmd.AddCommand(buildCmd, fetchClangCmd, probeCmd, pathsCmd, versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and exit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(v.String())
	},
}
