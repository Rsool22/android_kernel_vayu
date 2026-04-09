#!/bin/bash
# =============================================================================
#  VAYU CI BUILD -- headless, no TUI
#  Reuses core logic from build.sh; run by GitHub Actions only.
#  Env vars:
#    KSU_BRANCH  -- dev (default) or main
# =============================================================================
set -e

KERNEL_DIR="${GITHUB_WORKSPACE}"
CLANG_DIR="${KERNEL_DIR}/clang"
ANYKERNEL_DIR="${KERNEL_DIR}/AnyKernel3"
OUTPUT_DIR="${KERNEL_DIR}/Anykernel-Builds"
OBJDIR="${KERNEL_DIR}/out"
DEFCONFIG="arch/arm64/configs/vayu_defconfig"
OFFSET="${BUILD_COUNTER_OFFSET:-0}"
BUILD_NUM=$(( ${GITHUB_RUN_NUMBER:-1} - OFFSET ))
[ "$BUILD_NUM" -lt 1 ] && BUILD_NUM=1   # guard against a bad offset value
KSU_BRANCH="${KSU_BRANCH:-dev}"

export ARCH=arm64
export KBUILD_BUILD_USER="OmegaR01"
export KBUILD_BUILD_HOST="GitHub-CI"
export PATH="${CLANG_DIR}/bin:${PATH}"

# ccache -- directory is set by the workflow; fall back to a local dir if run
# outside CI so the script stays usable for local headless builds too.
export CCACHE_DIR="${CCACHE_DIR:-${HOME}/.ccache}"
export CCACHE_COMPILERCHECK="content"   # re-check when clang binary changes
export CCACHE_COMPRESS="1"

MAKE=(make
    -C "${KERNEL_DIR}"
    O="${OBJDIR}"
    ARCH=arm64
    LLVM=1
    LLVM_IAS=1
    CC="ccache clang"
    CLANG_TRIPLE=aarch64-linux-gnu-
    CROSS_COMPILE=aarch64-linux-gnu-
    CROSS_COMPILE_ARM32=arm-linux-gnueabi-
    --output-sync=line
)

# ── Helpers ───────────────────────────────────────────────────────────────────
log() { echo "  [CI] $*"; }

toggle_config() {
    local cfg=$1 en=$2 new_line
    if [ "$en" = "true" ]; then
        new_line="${cfg}=y"
    else
        new_line="# ${cfg} is not set"
    fi
    if grep -qE "^(# )?${cfg}[= ]" "${KERNEL_DIR}/${DEFCONFIG}" 2>/dev/null; then
        sed -i \
            -e "s|^# ${cfg} is not set|${new_line}|" \
            -e "s|^${cfg}=.*|${new_line}|" \
            "${KERNEL_DIR}/${DEFCONFIG}"
    else
        echo "$new_line" >> "${KERNEL_DIR}/${DEFCONFIG}"
    fi
}

# ── Verify toolchain ──────────────────────────────────────────────────────────
log "Checking Clang..."
if [ ! -f "${CLANG_DIR}/bin/clang" ]; then
    echo "ERROR: Clang not found at ${CLANG_DIR}/bin/clang"
    exit 1
fi
# Use || true to avoid SIGPIPE / broken-pipe from head closing the stream early
clang --version 2>&1 | head -1 || true

# ── Stage 1: Set features in defconfig ───────────────────────────────────────
log "Configuring features: KSU=y  SuSFS=y  KPM=y  ManualHook=n  Branch=${KSU_BRANCH}"
toggle_config "CONFIG_KSU"              "true"
toggle_config "CONFIG_KSU_SUSFS"        "true"
toggle_config "CONFIG_KPM"             "true"
toggle_config "CONFIG_KSU_MANUAL_HOOK" "false"

# ── Stage 2: Generate .config ─────────────────────────────────────────────────
log "Running defconfig..."
mkdir -p "${OBJDIR}"

# Force kernel build number to always be #1 in uname
echo "0" > "${OBJDIR}/.version"

"${MAKE[@]}" vayu_defconfig
log "Running olddefconfig..."
"${MAKE[@]}" olddefconfig

# ── Stage 3: Compile ──────────────────────────────────────────────────────────
log "Building kernel with $(nproc --all) threads..."
START_TIME=$(date +%s)

# Write build output to a file instead of a pipe.
#
# Root cause of "LLVM ERROR: IO failure on output stream: Broken pipe":
#   "2>&1 | cat" still uses a Unix pipe. When the GitHub Actions log runner
#   briefly closes that pipe, cat receives SIGPIPE and dies. That breaks
#   make's write-end, so every clang subprocess trying to write a diagnostic
#   gets EPIPE -> LLVM fatal error. Files cannot receive SIGPIPE, so a file
#   redirect is the only reliable fix.
#
# `tail -f` streams the log live to the Actions console while make runs.
BUILD_LOG="/tmp/kernel_build.log"
: > "${BUILD_LOG}"   # truncate / create
tail -f "${BUILD_LOG}" &
TAIL_PID=$!

"${MAKE[@]}" -j"$(nproc --all)" > "${BUILD_LOG}" 2>&1
BUILD_STATUS=$?

# Give tail a moment to flush the last lines, then stop it
sleep 1
kill "${TAIL_PID}" 2>/dev/null || true
wait "${TAIL_PID}" 2>/dev/null || true

if [ "${BUILD_STATUS}" -ne 0 ]; then
    echo "ERROR: make failed (exit ${BUILD_STATUS})"
    exit 1
fi

END_TIME=$(date +%s)
ELAPSED=$(( END_TIME - START_TIME ))
ELAPSED_FMT="$(( ELAPSED / 60 ))m $(( ELAPSED % 60 ))s"

if [ ! -f "${OBJDIR}/arch/arm64/boot/Image" ]; then
    echo "ERROR: Image not produced -- build failed"
    exit 1
fi
log "Image built in ${ELAPSED_FMT}"

# ── ccache stats ──────────────────────────────────────────────────────────────
log "ccache stats:"
ccache -s

# ── Stage 4: Package ──────────────────────────────────────────────────────────
log "Packaging AnyKernel3 zip..."
mkdir -p "${OUTPUT_DIR}"

cp "${OBJDIR}/arch/arm64/boot/Image"    "${ANYKERNEL_DIR}/Image"
cp "${OBJDIR}/arch/arm64/boot/dtbo.img" "${ANYKERNEL_DIR}/dtbo.img" 2>/dev/null || true
cp "${OBJDIR}/arch/arm64/boot/dtb.img"  "${ANYKERNEL_DIR}/dtb.img"  2>/dev/null || true

[ "${KSU_BRANCH}" = "dev" ] && BRANCH_TAG="DEV" || BRANCH_TAG="MAIN"

ZIPNAME="[VAYU-AnyMore-Project]-[${BRANCH_TAG}-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-($(date +%Y-%m-%d))-{Build-#${BUILD_NUM}}.zip"
ZIPPATH="${OUTPUT_DIR}/${ZIPNAME}"

cd "${ANYKERNEL_DIR}"
zip -r9 "${ZIPPATH}" . -x '*.git*'
cd "${KERNEL_DIR}"

ZIP_SIZE=$(du -h "${ZIPPATH}" | cut -f1)
log "Zip created: ${ZIPNAME} (${ZIP_SIZE})"
log "Build complete -- elapsed: ${ELAPSED_FMT}"

echo "ZIP_PATH=${ZIPPATH}"     >> "${GITHUB_ENV}"
echo "ZIP_NAME=${ZIPNAME}"     >> "${GITHUB_ENV}"
echo "ELAPSED=${ELAPSED_FMT}" >> "${GITHUB_ENV}"
