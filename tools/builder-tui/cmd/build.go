// Package cmd's `build` sub-command runs the full bash-1:1 build pipeline
// (Stages 1-5 incl. apply_ksu_guards.py) headlessly. CI invokes this via
// .github/scripts/ci_build.sh.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/naming"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/state"
)

var (
	buildDefconfig string
	buildPackage   bool
	buildOutput    string
	buildBranch    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run the full bash-1:1 pipeline (clean → defconfig → guards → compile → package)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		p, err := discover.Resolve(".",
			cfg.KernelDir, cfg.ClangDir, cfg.AnyKernelDir, cfg.OutputDir,
			cfg.GCC64Dir, cfg.GCC32Dir)
		if err != nil {
			return err
		}
		if p.Kernel == "" {
			return fmt.Errorf("kernel root not found (use --kernel-dir or run from inside the source tree)")
		}
		if p.Clang == "" {
			return fmt.Errorf("clang not found; run `vayu-builder fetch-clang` or set $CLANG_DIR")
		}

		// Read defconfig feature state so Stage 2 / Stage 3 / naming all
		// agree on KSU/SuSFS/KPM/MANUAL_HOOK.
		dcPath := filepath.Join(p.Kernel, "arch", "arm64", "configs", "vayu_defconfig")
		fst, err := features.Read(dcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[build] WARN: features.Read(%s): %v\n", dcPath, err)
		}

		defcfg := buildDefconfig
		if defcfg == "" {
			defcfg = "vayu_defconfig"
		}

		// Default zip path: <kernel>/<naming.ZipName(...)> when
		// --output not supplied.
		num := state.ReadBuildNumber(p.Kernel)
		branch := buildBranch
		if branch == "" {
			branch = cfg.KSUBranch
		}
		if branch == "" {
			branch = "main"
		}
		zipName := naming.ZipName(naming.Args{
			KernelName: cfg.KernelName,
			State:      fst,
			Branch:     branch,
			Build:      num,
		})
		zipPath := buildOutput
		if zipPath == "" {
			zipPath = filepath.Join(p.Kernel, "..", zipName)
		}

		opts := pipeline.Options{
			KernelDir:  p.Kernel,
			OutputDir:  p.Output,
			ClangDir:   p.Clang,
			GccArm64:   p.GccArm64,
			GccArm:     p.GccArm,
			AnyKernel:  p.AnyKernel,
			Defconfig:  defcfg,
			KernelName: cfg.KernelName,
			ZipPath:    zipPath,
		}

		fmt.Fprintf(os.Stderr, "[build] kernel    : %s\n", p.Kernel)
		fmt.Fprintf(os.Stderr, "[build] output    : %s\n", p.Output)
		fmt.Fprintf(os.Stderr, "[build] clang     : %s\n", p.Clang)
		fmt.Fprintf(os.Stderr, "[build] anykernel : %s\n", p.AnyKernel)
		fmt.Fprintf(os.Stderr, "[build] defconfig : %s\n", opts.Defconfig)
		fmt.Fprintf(os.Stderr, "[build] features  : KSU=%v SuSFS=%v KPM=%v ManualHook=%v\n",
			fst.KSU, fst.SUSFS, fst.KPM, fst.ManualHook)

		// Stream events to stderr; print final zip path to stdout for
		// CI consumption (ci_build.sh captures it).
		ch := make(chan pipeline.Event, 1024)
		done := make(chan struct{})
		var res pipeline.Result
		go func() {
			res = pipeline.Run(context.Background(), opts, fst, ch)
			close(ch)
			close(done)
		}()
		statusName := func(s pipeline.Status) string {
			switch s {
			case pipeline.StatusOK:
				return "OK"
			case pipeline.StatusFailed:
				return "FAIL"
			case pipeline.StatusSkipped:
				return "SKIP"
			case pipeline.StatusCancelled:
				return "CANCEL"
			}
			return "?"
		}
		for ev := range ch {
			switch ev.Kind {
			case pipeline.KindStageStart:
				fmt.Fprintf(os.Stderr, "::group::Stage %s %s\n", ev.Stage, ev.Detail)
			case pipeline.KindStageEnd:
				fmt.Fprintf(os.Stderr, "[stage %s] %s -- %s\n", ev.Stage, statusName(ev.Status), ev.Detail)
				fmt.Fprintln(os.Stderr, "::endgroup::")
			case pipeline.KindLine:
				if ev.IsErr {
					fmt.Fprintln(os.Stderr, "!! "+ev.Line)
				} else {
					fmt.Fprintln(os.Stderr, ev.Line)
				}
			}
		}
		<-done

		if res.ExitCode != 0 {
			return fmt.Errorf("build failed: exit %d (fail-log: %s)", res.ExitCode, res.FailLog)
		}
		fmt.Fprintf(os.Stderr, "[build] OK in %s\n", res.Elapsed.Truncate(1e9))
		if buildPackage && res.ZipPath != "" {
			fmt.Println(res.ZipPath)
		} else if res.Image != "" {
			fmt.Println(res.Image)
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildDefconfig, "defconfig", "", "override defconfig (default: vayu_defconfig)")
	buildCmd.Flags().BoolVar(&buildPackage, "package", false, "package output zip via AnyKernel3")
	buildCmd.Flags().StringVar(&buildOutput, "output", "", "output zip path (with --package)")
	buildCmd.Flags().StringVar(&buildBranch, "branch", "", "ReSukiSU branch tag (main / dev) for naming")
}
