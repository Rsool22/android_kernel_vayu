#!/usr/bin/env bash
# build.sh — bootstrap wrapper for the vayu-builder Bubble Tea TUI.
#
# Resolution order (first match wins):
#   1. $VAYU_BUILDER_BIN if set and executable.
#   2. $REPO/bin/vayu-builder built earlier in this checkout.
#   3. Compile from source: cd tools/builder-tui && go build … (preferred,
#      always uses the live source tree — no version skew, no network).
#   4. Cached prebuilt binary at $XDG_CACHE_HOME/vayu-builder/.
#   5. Download the prebuilt release artefact from GitHub.
#
# Steps 3 and 5 only run if the previous fallbacks failed. The default flow
# on a fresh checkout is: detect Go → compile → cache → exec. Subsequent
# runs short-circuit at step 2 (instant). Network is only required when
# Go is missing AND we cannot offer to install it.
#
# Environment overrides:
#   VAYU_BUILDER_BIN       use a pre-existing binary at this path
#   VAYU_BUILDER_FORCE     "rebuild" forces step 3 even if step 2 cached
#   VAYU_BUILDER_VERSION   pin a release tag for step 5 (default: latest)
#   VAYU_BUILDER_REPO      override the owner/repo for step 5
#   VAYU_BUILDER_NO_BUILD  "1" disables step 3 (force download path)

set -euo pipefail

REPO="${VAYU_BUILDER_REPO:-Rsool22/android_kernel_vayu}"
VERSION="${VAYU_BUILDER_VERSION:-latest}"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/vayu-builder"
mkdir -p "$CACHE_DIR"

c_red=$'\e[31m'; c_grn=$'\e[32m'; c_ylw=$'\e[33m'; c_cyn=$'\e[36m'; c_dim=$'\e[2m'; c_off=$'\e[0m'
say()  { printf '%s%s%s\n' "$c_dim" "[vayu-builder] $*" "$c_off"; }
err()  { printf '%s%s%s\n' "$c_red" "[vayu-builder] $*" "$c_off" >&2; }
ok()   { printf '%s%s%s\n' "$c_grn" "[vayu-builder] $*" "$c_off"; }
warn() { printf '%s%s%s\n' "$c_ylw" "[vayu-builder] $*" "$c_off"; }
info() { printf '%s%s%s\n' "$c_cyn" "[vayu-builder] $*" "$c_off"; }

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              err "unsupported arch: $(uname -m)"; exit 1 ;;
    esac
}

detect_os() {
    case "$(uname -s)" in
        Linux)   echo "linux" ;;
        Darwin)  echo "darwin" ;;
        *)       err "unsupported OS: $(uname -s)"; exit 1 ;;
    esac
}

asset_name() { printf 'vayu-builder-%s-%s\n' "$(detect_os)" "$(detect_arch)"; }

resolve_url() {
    local tag="$1" asset; asset=$(asset_name)
    if [ "$tag" = "latest" ]; then
        printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$asset"
    else
        printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$tag" "$asset"
    fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="${SCRIPT_DIR}/tools/builder-tui"
LOCAL_BIN="${SCRIPT_DIR}/bin/vayu-builder"

# Step 1 — explicit override
if [ -n "${VAYU_BUILDER_BIN:-}" ] && [ -x "${VAYU_BUILDER_BIN}" ]; then
    exec "${VAYU_BUILDER_BIN}" "$@"
fi

# Step 2 — same-tree dev binary (skip if user asked for a forced rebuild)
if [ -x "${LOCAL_BIN}" ] && [ "${VAYU_BUILDER_FORCE:-}" != "rebuild" ]; then
    # Stale-check: rebuild if any source file is newer than the binary.
    if [ -d "${SRC_DIR}" ]; then
        if [ -n "$(find "${SRC_DIR}" -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "${LOCAL_BIN}" -print -quit 2>/dev/null)" ]; then
            warn "source newer than ${LOCAL_BIN##*/}; rebuilding."
        else
            exec "${LOCAL_BIN}" "$@"
        fi
    else
        exec "${LOCAL_BIN}" "$@"
    fi
fi

# Step 3 — build from source (default for first run)
if [ "${VAYU_BUILDER_NO_BUILD:-0}" != "1" ] && [ -d "${SRC_DIR}" ]; then
    if command -v go >/dev/null 2>&1; then
        info "first-run bootstrap: compiling vayu-builder from source"
        say  "  go: $(go version | awk '{print $3, $4}')"
        say  "  src: ${SRC_DIR}"
        say  "  out: ${LOCAL_BIN}"
        mkdir -p "$(dirname "${LOCAL_BIN}")"
        if ( cd "${SRC_DIR}" && go build -trimpath -ldflags '-s -w' -o "${LOCAL_BIN}" . ); then
            ok "compiled $(${LOCAL_BIN} version 2>/dev/null || echo vayu-builder)"
            exec "${LOCAL_BIN}" "$@"
        else
            err "go build failed; falling through to release download."
        fi
    else
        warn "go(1) not found; cannot self-compile. Falling back to release download."
        warn "  install with:  apt-get install -y golang-go   (or your distro's equivalent)"
    fi
fi

# Step 4/5 — cached or downloaded prebuilt
TARGET="${CACHE_DIR}/$(asset_name)-${VERSION}"
if [ -x "$TARGET" ] && [ "${VAYU_BUILDER_FORCE:-}" != "rebuild" ]; then
    exec "$TARGET" "$@"
fi

URL="$(resolve_url "$VERSION")"
info "downloading $(asset_name) (version=${VERSION})"
say  "  from: ${URL}"
if ! curl -fSL --connect-timeout 30 --retry 3 --progress-bar -o "${TARGET}.dl" "$URL"; then
    err "download failed and no local source-build option succeeded."
    err "to recover, do one of:"
    err "  1) install Go (apt/dnf/pacman/zypper/apk → 'golang' or 'golang-go') and re-run $0"
    err "  2) run 'cd tools/builder-tui && go build -o ../../bin/vayu-builder .' yourself"
    err "  3) connect to the internet and re-run $0"
    err "  4) point VAYU_BUILDER_BIN at a local copy of the binary"
    exit 1
fi
chmod +x "${TARGET}.dl"
mv "${TARGET}.dl" "$TARGET"
ok "installed at ${TARGET}"
exec "$TARGET" "$@"
