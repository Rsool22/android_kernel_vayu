package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/kbuild"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pkg3"
)

var (
	buildDefconfig string
	buildPackage   bool
	buildOutput    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile kernel (LLVM=1, ccache); optionally package as AnyKernel3 zip",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		p, err := discover.Resolve(".", cfg.KernelDir, cfg.ClangDir, cfg.AnyKernelDir, cfg.OutputDir)
		if err != nil {
			return err
		}
		if p.Kernel == "" {
			return fmt.Errorf("kernel root not found (use --kernel-dir or run from inside the source tree)")
		}
		if p.Clang == "" {
			return fmt.Errorf("clang not found; run `vayu-builder fetch-clang` or set $CLANG_DIR")
		}
		o := kbuild.Defaults(p.Kernel)
		if buildDefconfig != "" {
			o.Defconfig = buildDefconfig
		}
		o.OutputDir = p.Output
		o.ClangDir = p.Clang
		o.GccArm64 = p.GccArm64
		o.GccArm = p.GccArm

		fmt.Fprintf(os.Stderr, "[build] kernel    : %s\n", p.Kernel)
		fmt.Fprintf(os.Stderr, "[build] clang     : %s\n", p.Clang)
		fmt.Fprintf(os.Stderr, "[build] defconfig : %s\n", o.Defconfig)
		fmt.Fprintf(os.Stderr, "[build] jobs      : %d\n", o.Jobs)

		ctx := context.Background()
		res := kbuild.Run(ctx, o, func(line string, _ bool) {
			fmt.Fprintln(os.Stderr, line)
		})
		if res.ExitCode != 0 {
			return fmt.Errorf("build failed: exit %d", res.ExitCode)
		}
		fmt.Fprintf(os.Stderr, "[build] OK in %s\n", kbuild.FormatElapsed(res.Elapsed))

		if buildPackage {
			if p.AnyKernel == "" {
				return fmt.Errorf("--package requested but AnyKernel3 dir not found")
			}
			zip := buildOutput
			if zip == "" {
				zip = filepath.Join(p.Kernel, "..", "vayu-kernel.zip")
			}
			if err := pkg3.Package(ctx, p.AnyKernel, res.Image, zip); err != nil {
				return err
			}
			fmt.Println(zip)
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildDefconfig, "defconfig", "", "override defconfig (default: vayu_defconfig)")
	buildCmd.Flags().BoolVar(&buildPackage, "package", false, "package output zip via AnyKernel3")
	buildCmd.Flags().StringVar(&buildOutput, "output", "", "output zip path (with --package)")
}
