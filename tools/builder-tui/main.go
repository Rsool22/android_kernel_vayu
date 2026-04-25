// vayu-builder is the entry point binary for the Vayu kernel builder TUI.
//
// Run with no arguments to open the interactive Bubble Tea UI; subcommands
// expose the same engine for CI use.
package main

import "github.com/Rsool22/android_kernel_vayu/tools/builder-tui/cmd"

func main() {
	cmd.Execute()
}
