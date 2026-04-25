package ui

import (
	"testing"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/pipeline"
)

// TestStageRowNarrow ensures stageRow doesn't panic on narrow widths
// (regression for the `strings: negative Repeat count` panic that hit
// the Build screen on small SSH terminals when prefix + rhs > inner).
func TestStageRowNarrow(t *testing.T) {
	sv := stageView{
		Stage:  pipeline.StageDefconfig,
		Status: pipeline.StatusRunning,
		Detail: "arch/arm64/configs/vayu_defconfig (this is a long detail)",
	}
	for _, w := range []int{-5, 0, 1, 5, 10, 20, 40, 60, 80, 120, 200} {
		// Should not panic at any width.
		_ = stageRow(sv, "⠼", w)
	}
}

// TestSrcRowNarrow ensures srcRow handles narrow widths cleanly.
func TestSrcRowNarrow(t *testing.T) {
	for _, w := range []int{-5, 0, 1, 5, 10, 20, 40, 80, 120} {
		_ = srcRow("G", "Google AOSP Clang  (main-kernel/clang)", true, false, w)
		_ = srcRow("L", "ZyC Clang latest  (any version)", false, true, w)
	}
}

// TestBuildOptRowNarrow ensures buildOptRow handles narrow widths.
func TestBuildOptRowNarrow(t *testing.T) {
	for _, w := range []int{-5, 0, 1, 5, 10, 20, 40, 80, 120} {
		_ = buildOptRow("N", "Build label", "/very/long/path/to/some/output/file.zip", w)
	}
}
