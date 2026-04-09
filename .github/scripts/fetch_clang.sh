#!/bin/bash
# =============================================================================
#  fetch_clang.sh -- downloads ZyC Clang 23.x for CI
# =============================================================================
set -e

TARGET_VER="23"
API="https://api.github.com/repos/ZyCromerZ/Clang/releases?per_page=50"
CLANG_DIR="${GITHUB_WORKSPACE}/clang"

echo "  [CI] Querying ZyC Clang releases for version ${TARGET_VER}.x..."

JSON=$(curl -fsSL --connect-timeout 30 --max-time 60 \
    -H "Accept: application/vnd.github+json" \
    "$API")

if [ -z "$JSON" ]; then
    echo "ERROR: Empty response from GitHub API"
    exit 1
fi

INFO=$(echo "$JSON" | python3 -c "
import sys, json
target = '${TARGET_VER}'
data = json.load(sys.stdin)
for rel in data:
    for asset in rel.get('assets', []):
        name = asset.get('name', '')
        if name.endswith('.tar.gz') and ('Clang-' + target + '.') in name:
            size_mb = round(asset['size'] / 1024 / 1024, 1)
            print(rel['tag_name'] + '|' + asset['browser_download_url'] + '|' + str(size_mb) + '|' + name)
            sys.exit(0)
print('fail')
")

if [ "$INFO" = "fail" ] || [ -z "$INFO" ]; then
    echo "ERROR: Could not find ZyC Clang ${TARGET_VER}.x release"
    exit 1
fi

TAG=$(echo "$INFO"  | cut -d'|' -f1)
URL=$(echo "$INFO"  | cut -d'|' -f2)
SIZE=$(echo "$INFO" | cut -d'|' -f3)
ASSET=$(echo "$INFO"| cut -d'|' -f4)

echo "  [CI] Found: ${TAG} -- ${ASSET} (~${SIZE} MB)"
echo "  [CI] Downloading..."

TMP_FILE=$(mktemp /tmp/zyc_clang_XXXXXX.tar.gz)
TMP_DIR=$(mktemp -d /tmp/zyc_extract_XXXXXX)

curl -L --progress-bar -o "$TMP_FILE" "$URL"

echo "  [CI] Extracting..."
tar -xzf "$TMP_FILE" -C "$TMP_DIR"
rm -f "$TMP_FILE"

# Handle single subdirectory inside tarball
ENTRIES=$(ls -A "$TMP_DIR" | wc -l)
SRC_DIR="$TMP_DIR"
if [ "$ENTRIES" -eq 1 ]; then
    SUBDIR=$(ls -A "$TMP_DIR")
    [ -d "$TMP_DIR/$SUBDIR" ] && SRC_DIR="$TMP_DIR/$SUBDIR"
fi

if [ ! -f "${SRC_DIR}/bin/clang" ]; then
    echo "ERROR: bin/clang not found in archive"
    rm -rf "$TMP_DIR"
    exit 1
fi

rm -rf "$CLANG_DIR"
mv "$SRC_DIR" "$CLANG_DIR"
rm -rf "$TMP_DIR"

echo "  [CI] ZyC Clang ${TAG} installed at ${CLANG_DIR}"
