package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/clang"
)

var (
	fetchSource string
	fetchTarget string
	fetchInto   string
	fetchCheck  bool
)

var fetchClangCmd = &cobra.Command{
	Use:   "fetch-clang",
	Short: "Fetch a clang toolchain (Google AOSP or ZyC)",
	Long: `Fetch and install a clang toolchain for kernel builds.

Sources:
  auto   try Google AOSP first, fall back to ZyC (default)
  google android.googlesource.com prebuilts/clang/host/linux-x86 (main-kernel)
  zyc    github.com/ZyCromerZ/Clang releases

Targets:
  Google: "latest" or a specific revision like "r596125"
  ZyC:    "latest", "23", "15", or any tag substring

If --check is set, only resolves the latest release without downloading.

Outputs (when running in CI, useful via $GITHUB_ENV):
  CLANG_DIR=<install dir>
  CLANG_SOURCE=<google|zyc>
  CLANG_VERSION=<resolved tag>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		var src clang.Source
		switch fetchSource {
		case "google":
			src = &clang.Google{}
		case "zyc":
			src = &clang.ZyC{}
		case "auto", "":
			src = &clang.Auto{}
		default:
			return fmt.Errorf("unknown --source %q", fetchSource)
		}
		rel, err := src.Latest(ctx, fetchTarget)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[fetch-clang] %s : %s\n", rel.Source, rel.Tag)
		if fetchCheck {
			fmt.Println(rel.Tag)
			return nil
		}
		if fetchInto == "" {
			fetchInto = "./clang"
		}
		fmt.Fprintf(os.Stderr, "[fetch-clang] downloading to %s\n", fetchInto)
		if err := clang.Install(ctx, rel, fetchInto, nil); err != nil {
			return err
		}
		// Emit GitHub Actions-style env updates if running under CI.
		if envFile := os.Getenv("GITHUB_ENV"); envFile != "" {
			f, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				fmt.Fprintf(f, "CLANG_DIR=%s\nCLANG_SOURCE=%s\nCLANG_VERSION=%s\n",
					fetchInto, rel.Source, rel.Tag)
				f.Close()
			}
		}
		fmt.Println(rel.Tag)
		return nil
	},
}

func init() {
	fetchClangCmd.Flags().StringVar(&fetchSource, "source", "auto", "auto | google | zyc")
	fetchClangCmd.Flags().StringVar(&fetchTarget, "target", "latest", "release/revision target")
	fetchClangCmd.Flags().StringVar(&fetchInto, "into", "", "install directory (default ./clang)")
	fetchClangCmd.Flags().BoolVar(&fetchCheck, "check", false, "resolve latest only; do not download")
}
