#!/usr/bin/env bash
# build.sh -- thin wrapper around the vayu-builder Bubble Tea TUI.
#
# This script is **not** the build system. It downloads the prebuilt
# vayu-builder binary from GitHub Releases (matching the host's OS/arch) and
# execs it. The binary contains the entire TUI plus all CLI subcommands
# (`build`, `fetch-clang`, `probe`, `paths`).
#
# To run from source instead (e.g. for local development on the TUI):
#     cd tools/builder-tui && go build -o ../../bin/vayu-builder . && ./bin/vayu-builder
#
# Environment overrides:
#   VAYU_BUILDER_VERSION   pin a release tag (default: latest)
#   VAYU_BUILDER_REPO      override the owner/repo to fetch from
#   VAYU_BUILDER_BIN       use a pre-existing binary at this path

set -euo pipefail

REPO="${VAYU_BUILDER_REPO:-Rsool22/android_kernel_vayu}"
VERSION="${VAYU_BUILDER_VERSION:-latest}"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/vayu-builder"
mkdir -p "$CACHE_DIR"

c_red=$'\e[31m'; c_grn=$'\e[32m'; c_ylw=$'\e[33m'; c_dim=$'\e[2m'; c_off=$'\e[0m'
say()  { printf '%s%s%s\n' "$c_dim" "[vayu-builder] $*" "$c_off"; }
err()  { printf '%s%s%s\n' "$c_red" "[vayu-builder] $*" "$c_off" >&2; }
ok()   { printf '%s%s%s\n' "$c_grn" "[vayu-builder] $*" "$c_off"; }
warn() { printf '%s%s%s\n' "$c_ylw" "[vayu-builder] $*" "$c_off"; }

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

asset_name() {
    printf 'vayu-builder-%s-%s\n' "$(detect_os)" "$(detect_arch)"
}

resolve_url() {
    local tag="$1" asset; asset=$(asset_name)
    if [ "$tag" = "latest" ]; then
        printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$asset"
    else
        printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$tag" "$asset"
    fi
}

# If the user supplied a path or there's a same-tree dev binary, use it.
if [ -n "${VAYU_BUILDER_BIN:-}" ] && [ -x "${VAYU_BUILDER_BIN}" ]; then
    exec "${VAYU_BUILDER_BIN}" "$@"
fi

# Local dev binary in repo
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -x "${SCRIPT_DIR}/bin/vayu-builder" ]; then
    exec "${SCRIPT_DIR}/bin/vayu-builder" "$@"
fi

# Cached binary
TARGET="${CACHE_DIR}/$(asset_name)-${VERSION}"
if [ ! -x "$TARGET" ]; then
    URL="$(resolve_url "$VERSION")"
    say "downloading $(asset_name) (version=${VERSION})"
    say "  from: ${URL}"
    if ! curl -fSL --connect-timeout 30 --retry 3 --progress-bar -o "${TARGET}.dl" "$URL"; then
        err "download failed."
        err "Either:"
        err "  1) connect to the internet and retry,"
        err "  2) build from source: cd tools/builder-tui && go build -o ../../bin/vayu-builder . "
        err "  3) point VAYU_BUILDER_BIN at a local copy of the binary."
        exit 1
    fi
    chmod +x "${TARGET}.dl"
    mv "${TARGET}.dl" "$TARGET"
    ok "installed at ${TARGET}"
fi

exec "$TARGET" "$@"
