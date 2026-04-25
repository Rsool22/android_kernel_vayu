#!/usr/bin/env bash
# .github/scripts/ci_build.sh -- thin wrapper that invokes the vayu-builder
# binary's `build` subcommand from CI. The binary handles autodiscovery,
# clang/cross-compiler resolution, and AnyKernel3 packaging. Everything that
# used to live as bash here now lives in tools/builder-tui/internal/*.
#
# Inputs (env, optional):
#   KERNEL_DIR             override autodiscovered kernel root
#   CLANG_DIR              override autodiscovered clang install
#   ANYKERNEL_DIR          override AnyKernel3 location
#   OUTPUT_DIR             override KBUILD_OUTPUT
#   DEFCONFIG              defconfig name (default: vayu_defconfig)
#   KSU_BRANCH_AVAILABLE   "true" / "false" / unset; if "false" we soft-skip
#
# Outputs (when running under GitHub Actions):
#   ZIP_PATH, ZIP_NAME, ELAPSED appended to $GITHUB_ENV
#
# Exit codes:
#   0   build + package OK
#   77  soft-skip (KSU branch unavailable)
#   1   hard failure
set -euo pipefail

if [ "${KSU_BRANCH_AVAILABLE:-true}" != "true" ]; then
    echo "[ci_build] KSU branch unavailable -- skipping (exit 77)"
    exit 77
fi

# Locate the vayu-builder binary. Order:
#   1. $VAYU_BUILDER_BIN if it points to an executable
#   2. $REPO/bin/vayu-builder (built fresh in CI)
#   3. fallback: build it now from tools/builder-tui
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VAYU_BIN="${VAYU_BUILDER_BIN:-}"
if [ -z "$VAYU_BIN" ] || [ ! -x "$VAYU_BIN" ]; then
    if [ -x "$REPO_ROOT/bin/vayu-builder" ]; then
        VAYU_BIN="$REPO_ROOT/bin/vayu-builder"
    else
        echo "[ci_build] building vayu-builder from source"
        ( cd "$REPO_ROOT/tools/builder-tui" && go build -o "$REPO_ROOT/bin/vayu-builder" . )
        VAYU_BIN="$REPO_ROOT/bin/vayu-builder"
    fi
fi

[ -x "$VAYU_BIN" ] || { echo "[ci_build] vayu-builder binary not found"; exit 1; }
echo "[ci_build] using $($VAYU_BIN version)"

start_ts=$(date +%s)
ARGS=(build --package)
[ -n "${DEFCONFIG:-}" ] && ARGS+=(--defconfig "$DEFCONFIG")
[ -n "${OUTPUT_ZIP:-}" ] && ARGS+=(--output "$OUTPUT_ZIP")

ZIP_PATH=$("$VAYU_BIN" "${ARGS[@]}")
end_ts=$(date +%s)
ELAPSED=$((end_ts - start_ts))

ZIP_NAME=$(basename "$ZIP_PATH")
echo "[ci_build] OK in ${ELAPSED}s"
echo "[ci_build] zip: $ZIP_PATH"

if [ -n "${GITHUB_ENV:-}" ]; then
    {
        echo "ZIP_PATH=$ZIP_PATH"
        echo "ZIP_NAME=$ZIP_NAME"
        echo "ELAPSED=$ELAPSED"
    } >> "$GITHUB_ENV"
fi
