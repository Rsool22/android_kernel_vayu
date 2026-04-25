package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/resukisu"
)

var probeJSON bool

var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Probe ReSukiSU branch availability",
	Long: `Probes the ReSukiSU upstream for both 'main' and 'dev' branches.

Output:
  default text: human-readable two-line report.
  --json:       machine-readable for CI consumption.

Exit codes:
  0  at least one branch is present
  2  both branches absent (upstream merged/removed)
  3  network failure (cannot reach upstream)
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		main, dev := resukisu.ProbeAll(ctx)
		if probeJSON {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"main": map[string]any{"state": main.State, "sha": main.SHA},
				"dev":  map[string]any{"state": dev.State, "sha": dev.SHA},
			})
		}
		fmt.Printf("main : %s %s\n", main.State, main.SHA)
		fmt.Printf("dev  : %s %s\n", dev.State, dev.SHA)
		switch {
		case main.State == resukisu.StateNetworkFail && dev.State == resukisu.StateNetworkFail:
			os.Exit(3)
		case main.State == resukisu.StateAbsent && dev.State == resukisu.StateAbsent:
			os.Exit(2)
		}
		return nil
	},
}

func init() {
	probeCmd.Flags().BoolVar(&probeJSON, "json", false, "emit machine-readable JSON")
}
