#!/usr/bin/env bash
# build.sh -- visually-polished launcher for the vayu-builder Bubble Tea TUI.
#
# Resolution order:
#   1. $VAYU_BUILDER_BIN   (explicit override -- takes precedence over all)
#   2. ./bin/vayu-builder  (local dev build relative to this script)
#   3. Build from source   (when Go is installed AND tools/builder-tui exists)
#   4. Cached release       ($XDG_CACHE_HOME/vayu-builder/<asset>-<tag>)
#   5. Download release     (from GitHub Releases of $VAYU_BUILDER_REPO)
#
# Environment overrides:
#   VAYU_BUILDER_VERSION   pin a release tag (default: latest)
#   VAYU_BUILDER_REPO      override the owner/repo to fetch from
#   VAYU_BUILDER_BIN       use a pre-existing binary at this path
#   VAYU_BUILDER_NO_BUILD  set to 1 to skip the from-source build path
#   VAYU_BUILDER_NO_NET    set to 1 to disable downloads (offline mode)

set -euo pipefail

REPO="${VAYU_BUILDER_REPO:-Rsool22/android_kernel_vayu}"
VERSION="${VAYU_BUILDER_VERSION:-latest}"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/vayu-builder"
mkdir -p "$CACHE_DIR"

# ── colour palette (matches the TUI's bash theme) ─────────────────────────────
if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
    c_mag=$'\e[1;35m'  c_cyn=$'\e[1;36m'  c_yel=$'\e[1;33m'
    c_grn=$'\e[1;32m'  c_red=$'\e[1;31m'  c_org=$'\e[38;5;214m'
    c_dim=$'\e[2m'     c_bld=$'\e[1m'     c_wht=$'\e[1;37m'  c_off=$'\e[0m'
else
    c_mag= c_cyn= c_yel= c_grn= c_red= c_org= c_dim= c_bld= c_wht= c_off=
fi

# ── width of the boxed banners; clamp to 64..96 like the TUI ──────────────────
COLS=$(tput cols 2>/dev/null || echo 80)
W=$(( COLS - 2 ))
if [ "$W" -lt 64 ]; then W=64; fi
if [ "$W" -gt 96 ]; then W=96; fi

repeat() { local n="$1" ch="$2" out=""; while [ "$n" -gt 0 ]; do out="$out$ch"; n=$((n-1)); done; printf '%s' "$out"; }

# Boxed banner with magenta double-line border, centred title.
banner() {
    local title="$1" sub="${2:-}"
    local inner=$(( W - 2 ))
    local pad=$(( (inner - ${#title}) / 2 ))
    local rest=$(( inner - pad - ${#title} ))
    printf '%s╔%s╗%s\n' "$c_mag" "$(repeat $inner '═')" "$c_off"
    printf '%s║%s%s%s%s%s║%s\n' "$c_mag" "$(repeat $pad ' ')" "$c_wht" "$title" "$c_off$c_mag" "$(repeat $rest ' ')" "$c_off"
    if [ -n "$sub" ]; then
        local p=$(( (inner - ${#sub}) / 2 ))
        local r=$(( inner - p - ${#sub} ))
        printf '%s║%s%s%s%s%s║%s\n' "$c_mag" "$(repeat $p ' ')" "$c_dim" "$sub" "$c_off$c_mag" "$(repeat $r ' ')" "$c_off"
    fi
    printf '%s╚%s╝%s\n' "$c_mag" "$(repeat $inner '═')" "$c_off"
}

# Cyan double-line panel header (closing line printed by `panel_close`).
# Total width is W to match the banner / step rows.
panel_open() {
    local title="$1"
    # Total width budget: W. Layout: ╔ + left + ' ' + title + ' ' + right + ╗
    local fill=$(( W - 4 - ${#title} ))
    if [ "$fill" -lt 4 ]; then fill=4; fi
    local left=2
    local right=$(( fill - left ))
    printf '%s╔%s %s%s%s %s╗%s\n' "$c_cyn" "$(repeat $left '═')" "$c_yel$c_bld" "$title" "$c_off$c_cyn" "$(repeat $right '═')" "$c_off"
}
panel_close() { printf '%s╚%s╝%s\n' "$c_cyn" "$(repeat $((W - 2)) '═')" "$c_off"; }

# Step row: `║ [#]  label ............... detail ║`
# All math in *visible* characters; detail is truncated when too long so
# the right border never wraps. Inner width = W - 4 (2 borders + 2 gutters).
step() {
    local n="$1" label="$2" detail="${3:-}"
    local inner=$(( W - 4 ))
    local prefix="[$n]  ${label}"
    # Reserve at minimum 8 cells for a leader; truncate detail if needed.
    local max_detail=$(( inner - ${#prefix} - 8 ))
    if [ "$max_detail" -lt 4 ]; then max_detail=4; fi
    if [ "${#detail}" -gt "$max_detail" ]; then
        detail="…${detail: -$((max_detail - 1))}"
    fi
    local pad=$(( inner - ${#prefix} - ${#detail} - 2 ))
    if [ "$pad" -lt 2 ]; then pad=2; fi
    local leader_n=$(( pad - 2 ))
    if [ "$leader_n" -lt 0 ]; then leader_n=0; fi
    # Build the styled inner segment, then sandwich it between borders.
    local inner_str
    inner_str=$(printf '[%s%s%s]  %s%s%s %s%s%s %s' \
        "$c_org$c_bld" "$n" "$c_off" \
        "$c_wht" "$label" "$c_off" \
        "$c_dim" "$(repeat $leader_n '·')" "$c_off" \
        "$detail")
    # Pad to inner width based on visible length (prefix + 1 space + leader + 1 space + detail).
    local visible=$(( ${#prefix} + 1 + leader_n + 1 + ${#detail} ))
    local trail=$(( inner - visible ))
    if [ "$trail" -lt 0 ]; then trail=0; fi
    printf '%s║%s %s%s %s║%s\n' "$c_cyn" "$c_off" "$inner_str" "$(repeat $trail ' ')" "$c_cyn" "$c_off"
}

# Single-line message rows used outside panels.
say()  { printf '%s%s[i]%s %s\n' "$c_cyn" "$c_bld" "$c_off" "$*"; }
ok()   { printf '%s%s[✓]%s %s\n' "$c_grn" "$c_bld" "$c_off" "$*"; }
warn() { printf '%s%s[!]%s %s\n' "$c_yel" "$c_bld" "$c_off" "$*"; }
err()  { printf '%s%s[×]%s %s\n' "$c_red" "$c_bld" "$c_off" "$*" >&2; }

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

# ── 1. start banner ──────────────────────────────────────────────────────────
banner "VAYU  KERNEL  BUILDER  --  LAUNCHER" "$(asset_name)  ·  ${REPO} @ ${VERSION}"
echo

panel_open "Resolving runtime"

# 1) Explicit override.
if [ -n "${VAYU_BUILDER_BIN:-}" ] && [ -x "${VAYU_BUILDER_BIN}" ]; then
    step 1 "Using \$VAYU_BUILDER_BIN" "${VAYU_BUILDER_BIN}"
    panel_close
    echo
    ok "launching ${VAYU_BUILDER_BIN}"
    exec "${VAYU_BUILDER_BIN}" "$@"
fi
step 1 "Override env (\$VAYU_BUILDER_BIN)" "${VAYU_BUILDER_BIN:-(unset)}"

# 2) Local dev binary -- only used as-is when the source tree is older
#    than the cached binary. Otherwise we fall through to the from-source
#    rebuild in step 3 so the user always runs against the latest TUI.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_BIN="${SCRIPT_DIR}/bin/vayu-builder"
SRC_DIR="${SCRIPT_DIR}/tools/builder-tui"

# sources_newer_than <binary>
#   Returns 0 (true) when any *.go file under $SRC_DIR is newer than the
#   given binary (or when the binary doesn't exist). Returns 1 (false)
#   when the binary is up-to-date relative to source. Falls back to "true"
#   on any error so we err on the side of rebuilding.
sources_newer_than() {
    local bin="$1"
    [ -x "$bin" ] || return 0
    [ -d "$SRC_DIR" ] || return 1
    if find "$SRC_DIR" \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
            -newer "$bin" -print -quit 2>/dev/null | grep -q .; then
        return 0
    fi
    return 1
}

if [ -x "$LOCAL_BIN" ] && [ "${VAYU_BUILDER_FORCE_REBUILD:-0}" != "1" ] \
        && ! sources_newer_than "$LOCAL_BIN"; then
    step 2 "Local binary"          "$LOCAL_BIN  (up to date)"
    panel_close
    echo
    ok "launching ${LOCAL_BIN}"
    exec "$LOCAL_BIN" "$@"
fi
if [ -x "$LOCAL_BIN" ]; then
    step 2 "Local binary"          "$LOCAL_BIN  (stale -- rebuilding)"
else
    step 2 "Local binary"          "(none at ${LOCAL_BIN})"
fi

# 3) From-source build (if Go is available and tools/ exists).
if [ "${VAYU_BUILDER_NO_BUILD:-0}" != "1" ] && [ -d "$SRC_DIR" ] && [ -f "${SRC_DIR}/go.mod" ]; then
    if command -v go >/dev/null 2>&1; then
        GO_VER="$(go version 2>/dev/null | awk '{print $3}')"
        step 3 "Go toolchain"        "${GO_VER}  (build from source)"
        panel_close
        echo
        say "compiling vayu-builder from ${SRC_DIR}"
        mkdir -p "${SCRIPT_DIR}/bin"
        # Pipe go build output through a `[go]` prefix so it visually
        # matches the rest of the launcher's output stream.
        if ( cd "$SRC_DIR" && go build -trimpath -ldflags '-s -w' -o "$LOCAL_BIN" . ) 2>&1 \
            | sed -e "s/^/$(printf '%s[go]%s ' "$c_cyn" "$c_off")/"; then
            ok "built ${LOCAL_BIN}"
            echo
            ok "launching ${LOCAL_BIN}"
            exec "$LOCAL_BIN" "$@"
        else
            err "go build failed -- falling back to release download"
            echo
        fi
    else
        step 3 "Go toolchain"        "$(printf '%s(not installed)%s -- skipping source build' "$c_dim" "$c_off")"
    fi
else
    step 3 "Source tree"             "(skipped -- VAYU_BUILDER_NO_BUILD=1 or tools/ missing)"
fi

# 4) Cached release binary.
TARGET="${CACHE_DIR}/$(asset_name)-${VERSION}"
if [ -x "$TARGET" ]; then
    step 4 "Cached release"          "${TARGET}  (use)"
    panel_close
    echo
    ok "launching ${TARGET}"
    exec "$TARGET" "$@"
fi
step 4 "Cached release"              "(none at ${TARGET})"

# 5) Download.
if [ "${VAYU_BUILDER_NO_NET:-0}" = "1" ]; then
    step 5 "Download"                "$(printf '%sdisabled (VAYU_BUILDER_NO_NET=1)%s' "$c_dim" "$c_off")"
    panel_close
    echo
    err "no usable runtime found and downloads are disabled"
    err "  set VAYU_BUILDER_BIN=/path/to/binary, or unset VAYU_BUILDER_NO_NET"
    exit 1
fi

URL="$(resolve_url "$VERSION")"
step 5 "Download"                    "from GitHub Releases"
panel_close
echo

say "fetching $(asset_name) (version=${VERSION})"
say "  url: ${URL}"
say "  dst: ${TARGET}"
echo

if ! curl -fSL --connect-timeout 30 --retry 3 --progress-bar \
        -o "${TARGET}.dl" "$URL"; then
    echo
    err "download failed -- here's how to recover:"
    err "  1) check your internet connection and retry"
    err "  2) build from source manually:"
    err "       cd tools/builder-tui && go build -o ../../bin/vayu-builder ."
    err "  3) set VAYU_BUILDER_BIN=/path/to/prebuilt/vayu-builder"
    exit 1
fi
chmod +x "${TARGET}.dl"
mv "${TARGET}.dl" "$TARGET"
echo
ok "installed at ${TARGET}"
echo
ok "launching ${TARGET}"

exec "$TARGET" "$@"
