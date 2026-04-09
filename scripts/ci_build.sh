#!/bin/bash
# =============================================================================
#  VAYU CI BUILD -- headless, no TUI
#  Reuses core logic from build.sh; run by GitHub Actions only.
# =============================================================================
set -e

KERNEL_DIR="${GITHUB_WORKSPACE}"
CLANG_DIR="${KERNEL_DIR}/clang"
ANYKERNEL_DIR="${KERNEL_DIR}/AnyKernel3"
OUTPUT_DIR="${KERNEL_DIR}/Anykernel-Builds"
OBJDIR="${KERNEL_DIR}/out"
DEFCONFIG="arch/arm64/configs/vayu_defconfig"
BUILD_NUM="${GITHUB_RUN_NUMBER:-1}"

export ARCH=arm64
export KBUILD_BUILD_USER="OmegaR01"
export KBUILD_BUILD_HOST="GitHub-CI"
export PATH="${CLANG_DIR}/bin:/usr/lib/ccache:${PATH}"

MAKE="make -C ${KERNEL_DIR} O=${OBJDIR} ARCH=arm64 LLVM=1 LLVM_IAS=1 \
    CC=clang \
    CLANG_TRIPLE=aarch64-linux-gnu- \
    CROSS_COMPILE=aarch64-linux-gnu- \
    CROSS_COMPILE_ARM32=arm-linux-gnueabi-"

# ── Helpers ──────────────────────────────────────────────────────────────────
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
clang --version | head -1

# ── Stage 1: Set features in defconfig ───────────────────────────────────────
log "Configuring features: KSU=y  SuSFS=y  KPM=y  ManualHook=n"
toggle_config "CONFIG_KSU"              "true"
toggle_config "CONFIG_KSU_SUSFS"        "true"
toggle_config "CONFIG_KPM"             "true"
# SuSFS active → use inline hook, not manual hook
toggle_config "CONFIG_KSU_MANUAL_HOOK" "false"

# ── Stage 2: Generate .config ─────────────────────────────────────────────────
log "Running defconfig..."
mkdir -p "${OBJDIR}"
$MAKE vayu_defconfig

log "Running olddefconfig..."
$MAKE olddefconfig

# ── Stage 3: Compile ──────────────────────────────────────────────────────────
log "Building kernel with $(nproc --all) threads..."
START_TIME=$(date +%s)
$MAKE -j"$(nproc --all)"
END_TIME=$(date +%s)
ELAPSED=$(( END_TIME - START_TIME ))
ELAPSED_FMT="$(( ELAPSED / 60 ))m $(( ELAPSED % 60 ))s"

if [ ! -f "${OBJDIR}/arch/arm64/boot/Image" ]; then
    echo "ERROR: Image not produced -- build failed"
    exit 1
fi
log "Image built in ${ELAPSED_FMT}"

# ── Stage 4: Package ──────────────────────────────────────────────────────────
log "Packaging AnyKernel3 zip..."
mkdir -p "${OUTPUT_DIR}"

cp "${OBJDIR}/arch/arm64/boot/Image"    "${ANYKERNEL_DIR}/Image"
cp "${OBJDIR}/arch/arm64/boot/dtbo.img" "${ANYKERNEL_DIR}/dtbo.img" 2>/dev/null || true
cp "${OBJDIR}/arch/arm64/boot/dtb.img"  "${ANYKERNEL_DIR}/dtb.img"  2>/dev/null || true

ZIPNAME="[VAYU-AnyMore-Project]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-($(date +%Y-%m-%d))-{Build-#${BUILD_NUM}}.zip"
ZIPPATH="${OUTPUT_DIR}/${ZIPNAME}"

cd "${ANYKERNEL_DIR}"
zip -r9 "${ZIPPATH}" . -x '*.git*'
cd "${KERNEL_DIR}"

ZIP_SIZE=$(du -h "${ZIPPATH}" | cut -f1)
log "Zip created: ${ZIPNAME} (${ZIP_SIZE})"
log "Build complete -- elapsed: ${ELAPSED_FMT}"

# Export zip path for the release step
echo "ZIP_PATH=${ZIPPATH}" >> "${GITHUB_ENV}"
echo "ZIP_NAME=${ZIPNAME}" >> "${GITHUB_ENV}"
echo "ELAPSED=${ELAPSED_FMT}" >> "${GITHUB_ENV}"
