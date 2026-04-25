package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/config"
	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/discover"
)

var pathsJSON bool

var pathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "Dump autodiscovered paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		p, err := discover.Resolve(".",
			cfg.KernelDir, cfg.ClangDir, cfg.AnyKernelDir, cfg.OutputDir,
			cfg.GCC64Dir, cfg.GCC32Dir)
		if err != nil {
			return err
		}
		if pathsJSON {
			return json.NewEncoder(os.Stdout).Encode(p)
		}
		fmt.Printf("kernel    : %s\n", p.Kernel)
		fmt.Printf("clang     : %s\n", p.Clang)
		fmt.Printf("anykernel : %s\n", p.AnyKernel)
		fmt.Printf("output    : %s\n", p.Output)
		fmt.Printf("aarch64gcc: %s\n", p.GccArm64)
		fmt.Printf("armgcc    : %s\n", p.GccArm)
		fmt.Printf("distro    : %s\n", p.Distro)
		return nil
	},
}

func init() {
	pathsCmd.Flags().BoolVar(&pathsJSON, "json", false, "emit machine-readable JSON")
}
