#!/usr/bin/env bash
# .github/scripts/fetch_clang.sh -- thin wrapper around the vayu-builder
# binary's `fetch-clang` subcommand for CI use.
#
# Behavior:
#   • Resolves and downloads a clang toolchain into $INSTALL_DIR (default:
#     $REPO_ROOT/clang).
#   • Tries Google AOSP first when --source=auto, falls back to ZyC.
#   • Appends CLANG_DIR, CLANG_SOURCE, CLANG_VERSION to $GITHUB_ENV when run
#     under Actions.
#
# Flags accepted (forwarded verbatim):
#   --source auto|google|zyc       (default: auto)
#   --target latest|<rev|tag>      (default: latest)
#   --into <dir>                   (default: $REPO_ROOT/clang)
#   --check                        resolve only; do not download

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VAYU_BIN="${VAYU_BUILDER_BIN:-}"
if [ -z "$VAYU_BIN" ] || [ ! -x "$VAYU_BIN" ]; then
    if [ -x "$REPO_ROOT/bin/vayu-builder" ]; then
        VAYU_BIN="$REPO_ROOT/bin/vayu-builder"
    else
        echo "[fetch_clang] building vayu-builder from source"
        ( cd "$REPO_ROOT/tools/builder-tui" && go build -o "$REPO_ROOT/bin/vayu-builder" . )
        VAYU_BIN="$REPO_ROOT/bin/vayu-builder"
    fi
fi

ARGS=()
INTO_PROVIDED=false
for a in "$@"; do
    ARGS+=("$a")
    [ "$a" = "--into" ] && INTO_PROVIDED=true
done
if ! $INTO_PROVIDED; then
    ARGS+=("--into" "$REPO_ROOT/clang")
fi

exec "$VAYU_BIN" fetch-clang "${ARGS[@]}"
