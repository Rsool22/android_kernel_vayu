#!/bin/bash
# =============================================================================
#  VAYU KERNEL BUILDER  ─  ReSukiSU + SuSFS + KPM  ─  AnyMore Project
#  Linux 4.14 NonGKI  │  Xiaomi Poco X3 Pro (vayu)  │  Android 16
# =============================================================================

# ── Paths ────────────────────────────────────────────────────────────────────
kernel_dir="${HOME}/kernel-builds/vayu_a16_kernel"
objdir="${kernel_dir}/out"
anykernel="${HOME}/kernel-builds/AnyKernel3"
CLANG_DIR="${HOME}/kernel-builds/clang"
GCC64_DIR="/usr/bin"
GCC32_DIR="/usr/bin"
CONFIG_FILE="vayu_defconfig"
OUTPUT_DIR="${HOME}/Anykernel-Builds"
DEFCONFIG_PATH="${kernel_dir}/arch/arm64/configs/${CONFIG_FILE}"

# ── Persistent state files ───────────────────────────────────────────────────
# .builder_state     — incremental preference
# .builder_prev_state — last successful build summary
# .menuconfig_saved_config — preserved .config for next ./build.sh run
STATE_FILE="${HOME}/kernel-builds/.builder_state"
PREV_STATE_FILE="${HOME}/kernel-builds/.builder_prev_state"
MENUCONFIG_PRESERVE_FILE="${HOME}/kernel-builds/.menuconfig_saved_config"

# ── Session-only build overrides — cleared on each fresh ./build.sh run ──────
#   KERNEL_NAME  — appended as LOCALVERSION suffix; also suppresses the git
#                  +N dirty-tree count via .scmversion so uname -r stays clean
KERNEL_NAME=""

# ── Naming — single source of truth ──────────────────────────────────────────
# ALL zip filenames, log filenames, and box labels derive from _build_log_base().
# Format: [VAYU-AnyMore-Project]-[Kernel:"Name"]-[ReSukiSU:Hook]-(Features:X+Y)-[date]
#   Zip → …-{Build-#N}.zip     Log → …-Build-#N.log / …-FAIL.log
PROJECT_NAME="VAYU-AnyMore"

_build_log_base() {
    local bdate; bdate=$(date +%Y-%m-%d)
    local name="[${PROJECT_NAME}-Project]"
    [ -n "$KERNEL_NAME" ] && name="${name}-[Kernel-Name=${KERNEL_NAME}]"
    if $FEAT_KSU; then
        local hook; $FEAT_SUSFS && hook="SuSFS-Inline-Hook" || hook="Manual-Hook"
        name="${name}-[ReSukiSU=${hook}]"
        local extras=""
        $FEAT_SUSFS && extras="SuSFS"
        $FEAT_KPM   && extras="${extras:+${extras}+}KPM"
        [ -n "$extras" ] && name="${name}-(Features=${extras})"
    fi
    echo "${name}-(${bdate})"
}

_success_log_name() { echo "$(_build_log_base)-Build-#${1}.log"; }
_fail_log_name()    { echo "[${PROJECT_NAME}-Project]-FAIL.log"; }

_rm_old_success_log() {
    find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-Build-#*.log" \
        -delete 2>/dev/null || true
}

_rm_old_zip() {
    local keep="${1:-}"
    find "$OUTPUT_DIR" -maxdepth 1 -name "*.zip" | while IFS= read -r f; do
        [ "$f" != "$keep" ] && rm -f "$f"
    done
}

# ── Build counter ────────────────────────────────────────────────────────────
BUILD_NUM_FILE="${HOME}/kernel-builds/.build_number"

_read_build_num() {
    if [ -f "$BUILD_NUM_FILE" ]; then
        BUILD_NUM=$(( $(cat "$BUILD_NUM_FILE") + 1 ))
    else
        BUILD_NUM=1
    fi
}

_commit_build_num() { echo "$BUILD_NUM" > "$BUILD_NUM_FILE"; }

reset_build_num() {
    echo "0" > "$BUILD_NUM_FILE"
    _read_build_num
}

_read_build_num

# ── Persistent state: save/load incremental preference ───────────────────────
_save_state() { printf "INCREMENTAL=%s\nUSE_CCACHE=%s\n" "$INCREMENTAL" "$USE_CCACHE" > "$STATE_FILE"; }

_load_state() {
    INCREMENTAL=false
    # shellcheck source=/dev/null
    [ -f "$STATE_FILE" ] && source "$STATE_FILE" 2>/dev/null || true
    # If ccache binary is absent, force USE_CCACHE off regardless of saved state
    [ -z "$CCACHE_BIN" ] && USE_CCACHE=false
    _update_make_cc
}

_load_state

# ── Menuconfig session state ─────────────────────────────────────────────────
# These are always reset to clean state on each fresh ./build.sh invocation.
#
#   MENUCONFIG_USED  — true when menuconfig ran and user saved changes this session
#   _SKIP_DEFCONFIG  — true when Stage 3 should skip defconfig regen (use .config)
#   _PRESERVE_ACTIVE — true when a preserved .config exists and will be restored
#
# Lifecycle:
#   [M] in feat menu → saves, sets MENUCONFIG_USED=true, _SKIP_DEFCONFIG=true
#   [V] in post-build → copies .config to MENUCONFIG_PRESERVE_FILE (survives exit)
#   [D] in post-build → runs savedefconfig, writes vayu_defconfig permanently
#   On next ./build.sh → _PRESERVE_ACTIVE=true, Stage 3 restores the saved .config
MENUCONFIG_USED=false
_SKIP_DEFCONFIG=false
_PRESERVE_ACTIVE=false
[ -f "$MENUCONFIG_PRESERVE_FILE" ] && _PRESERVE_ACTIVE=true

_save_prev_state() {
    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    local bmode; $INCREMENTAL && bmode="Incremental" || bmode="Full Clean"
    {
        printf "PREV_FEAT='%s'\n"  "$fs"
        printf "PREV_MODE='%s'\n"  "$bmode"
        printf "PREV_NUM='%s'\n"   "$BUILD_NUM"
        printf "PREV_KNAME='%s'\n" "$KERNEL_NAME"
        printf "PREV_DATE='%s'\n"  "$(date '+%Y-%m-%d %H:%M')"
    } > "$PREV_STATE_FILE"
}

_load_prev_state() {
    PREV_FEAT=""; PREV_MODE=""; PREV_NUM=""; PREV_KNAME=""; PREV_DATE=""
    # shellcheck source=/dev/null
    [ -f "$PREV_STATE_FILE" ] && source "$PREV_STATE_FILE" 2>/dev/null || true
}

# ── Environment ──────────────────────────────────────────────────────────────
export ARCH="arm64"
export KBUILD_BUILD_USER="OmegaR01"
export KBUILD_BUILD_HOST="Vayu"
export PATH="${CLANG_DIR}/bin:${GCC64_DIR}:${GCC32_DIR}:${PATH}"
CCACHE_BIN=$(command -v ccache 2>/dev/null)

# USE_CCACHE — toggled from build menu; defaults on if binary is available
USE_CCACHE=true
[ -z "$CCACHE_BIN" ] && USE_CCACHE=false

# MAKE_CC — computed from USE_CCACHE; used in every make call
MAKE_CC="clang"
_update_make_cc() {
    if $USE_CCACHE && [ -n "$CCACHE_BIN" ]; then
        MAKE_CC="ccache clang"
    else
        MAKE_CC="clang"
    fi
}
_update_make_cc

# ── Colors ───────────────────────────────────────────────────────────────────
NC='\033[0m'
LRD='\033[1;31m'; RED='\033[0;31m'
LGR='\033[1;32m'; YEL='\033[1;33m'
CYN='\033[1;36m'; MAG='\033[1;35m'
BLU='\033[1;34m'; WHT='\033[1;37m'
GRY='\033[0;37m'; DIM='\033[2m'
ORG='\033[0;33m'

# ── Sanity checks ────────────────────────────────────────────────────────────
[ ! -d "$CLANG_DIR" ] && printf "${LRD}ERR: Clang not found: %s${NC}\n" "$CLANG_DIR" && exit 1
[ ! -d "$anykernel"  ] && printf "${LRD}ERR: AnyKernel3 not found: %s${NC}\n" "$anykernel" && exit 1

# =============================================================================
# TERMINAL HELPERS
# =============================================================================
W=80

do_clear() {
    printf '\033[?25l\033[2J\033[H\033[?25h'
}

_cursor_hide() { printf '\033[?25l'; }
_cursor_show() { printf '\033[?25h'; }

set_width() {
    local c
    c=$(stty size 2>/dev/null | awk '{print $2}')
    [ -z "$c" ] && c=$(tput cols 2>/dev/null)
    [ -z "$c" ] && c="${COLUMNS:-0}"
    c=$(( c + 0 ))
    [ "$c" -lt 40  ] && c=40
    [ "$c" -gt 160 ] && c=160
    W="$c"
}

_winch_enable()  { trap '_handle_winch' WINCH; }
_winch_disable() { trap - WINCH; _MENU_REDRAW_FN=""; }

# Drain paste overflow — only the first char acts as a keypress
_drain_input() {
    local _junk
    while IFS= read -r -s -t 0.05 -n256 _junk 2>/dev/null && [ -n "$_junk" ]; do :; done
}

_term_cleanup() {
    stty echo 2>/dev/null
    printf '\033[?25h\033[2J\033[H'
}
trap '_term_cleanup' EXIT

# ── Box drawing primitives ────────────────────────────────────────────────────
_hbar() { printf '═%.0s' $(seq 1 "$1"); }
_tbar() { local _i _o=''; for _i in $(seq 1 "$1"); do _o="${_o}─"; done; printf '%s' "$_o"; }
_sbar() { local _i _o=''; for _i in $(seq 1 "$1"); do _o="${_o} "; done; printf '%s' "$_o"; }

box_top()  { printf "${1}╔$(_hbar $((W-2)))╗${NC}\n"; }
box_bot()  { printf "${1}╚$(_hbar $((W-2)))╝${NC}\n"; }
box_div()  { printf "${1}╠$(_hbar $((W-2)))╣${NC}\n"; }
box_blank(){ printf "${1}║$(_sbar $((W-2)))${1}║${NC}\n"; }

box_ctr() {
    local bc=$1 tc=$2 text=$3
    local inner=$((W-2))
    [ ${#text} -gt $inner ] && text="${text:0:$(( inner-1 ))}…"
    local len=${#text}
    local lp=$(( (inner-len)/2 )) rp=$(( inner-len-(inner-len)/2 ))
    [ $lp -lt 0 ] && lp=0; [ $rp -lt 0 ] && rp=0
    printf "${bc}║${tc}%${lp}s%s%${rp}s${bc}║${NC}\n" '' "$text" ''
}

box_row() {
    local bc=$1 tc=$2 text=$3
    local inner=$(( W-2 ))
    [ ${#text} -gt $inner ] && text="${text:0:$(( inner-1 ))}…"
    local pad=$(( inner-${#text} ))
    [ $pad -lt 0 ] && pad=0
    printf "${bc}║${tc}%s%${pad}s${bc}║${NC}\n" "$text" ''
}

box_kv() {
    local bc=$1 kc=$2 vc=$3 key=$4 val=$5
    local inner=$(( W-2 ))
    local total=$(( ${#key}+${#val}+1 ))
    if [ $total -gt $inner ]; then
        # Try shrinking value first to keep the key label readable
        local maxval=$(( inner-${#key}-2 ))
        if [ $maxval -ge 4 ]; then
            [ ${#val} -gt $maxval ] && val="${val:0:$(( maxval-1 ))}…"
        else
            # Both are long — split budget evenly
            local half=$(( (inner-2)/2 ))
            [ ${#key} -gt $half ] && key="${key:0:$(( half-1 ))}…"
            local maxval2=$(( inner-${#key}-2 ))
            [ $maxval2 -lt 4 ] && maxval2=4
            [ ${#val} -gt $maxval2 ] && val="${val:0:$(( maxval2-1 ))}…"
        fi
    fi
    local gap=$(( inner-${#key}-${#val} ))
    [ $gap -lt 1 ] && gap=1
    printf "${bc}║${kc}%s%${gap}s${vc}%s${bc}║${NC}\n" "$key" '' "$val"
}

# Inline separator within a box — thin rule with optional label
box_rule() {
    local bc=$1 lc=${2:-} label=${3:-}
    local inner=$(( W-2 ))
    if [ -n "$label" ]; then
        local rest=$(( inner-${#label}-4 ))
        [ $rest -lt 1 ] && rest=1
        printf "${bc}║${lc} ─ %s $(_tbar $rest)${bc}║${NC}\n" "$label"
    else
        printf "${bc}║${DIM}$(_tbar $inner)${bc}║${NC}\n"
    fi
}

# box_wrap — renders a labelled row then wraps overflow onto continuation rows.
# Continuation rows are automatically indented to align with the value start,
# i.e. they start at column (${#label} + 3) to match the "label : " prefix.
# Usage: box_wrap border_color text_color label value
box_wrap() {
    local bclr=$1 tclr=$2 label=$3 value=$4
    local inner=$(( W-2 ))
    # Continuation indent = label width + 3 chars for " : "
    local voff=$(( ${#label} + 3 ))
    local cont; printf -v cont '%*s' "$voff" ''
    local first="${label} : ${value}"
    if [ ${#first} -le $inner ]; then
        box_row "$bclr" "$tclr" "$first"
        return
    fi
    # First row: label + " : " + as much of value as fits
    local avail=$(( inner - voff ))
    if [ $avail -le 4 ]; then
        box_row "$bclr" "$tclr" "${first:0:$(( inner-1 ))}…"
        return
    fi
    # Find best break point — prefer ] } space -[ boundaries
    local best=$avail
    local i=$(( avail - 1 ))
    while [ $i -gt $(( avail / 2 )) ]; do
        local ch="${value:$i:1}" nx="${value:$(( i+1 )):1}"
        if [ "$ch" = "]" ] || [ "$ch" = "}" ] || [ "$ch" = " " ] || \
           ( [ "$ch" = "-" ] && [ "$nx" = "[" ] ); then
            best=$(( i + 1 ))
            break
        fi
        i=$(( i - 1 ))
    done
    box_row "$bclr" "$tclr" "${label} : ${value:0:$best}"
    # Continuation rows — aligned with value start
    local rem="${cont}${value:$best}"
    while [ ${#rem} -gt $inner ]; do
        local bi=$(( inner - 1 ))
        while [ $bi -gt $(( inner / 2 )) ]; do
            local ch2="${rem:$bi:1}" nx2="${rem:$(( bi+1 )):1}"
            if [ "$ch2" = "]" ] || [ "$ch2" = "}" ] || [ "$ch2" = " " ] || \
               ( [ "$ch2" = "-" ] && [ "$nx2" = "[" ] ); then
                bi=$(( bi + 1 ))
                break
            fi
            bi=$(( bi - 1 ))
        done
        [ $bi -le $(( inner / 2 )) ] && bi=$inner
        box_row "$bclr" "$tclr" "${rem:0:$bi}"
        rem="${cont}${rem:$bi}"
    done
    [ -n "$rem" ] && box_row "$bclr" "$tclr" "$rem"
}

# Stage separator (between build stages in build output)
log_sep() {
    local label="${1:-}"
    if [ -n "$label" ]; then
        local rest=$(( W-${#label}-5 ))
        [ $rest -lt 1 ] && rest=1
        printf "\n${CYN}── %s ${DIM}$(_tbar $rest)${NC}\n\n" "$label"
    else
        printf "${DIM}$(_tbar $W)${NC}\n"
    fi
}

# =============================================================================
# FEATURE STATE
# =============================================================================
FEAT_KSU=false; FEAT_SUSFS=false; FEAT_KPM=false

read_features() {
    FEAT_KSU=false; FEAT_SUSFS=false; FEAT_KPM=false
    grep -q "^CONFIG_KSU=y"       "$DEFCONFIG_PATH" 2>/dev/null && FEAT_KSU=true
    grep -q "^CONFIG_KSU_SUSFS=y" "$DEFCONFIG_PATH" 2>/dev/null && FEAT_SUSFS=true
    grep -q "^CONFIG_KPM=y"       "$DEFCONFIG_PATH" 2>/dev/null && FEAT_KPM=true
}

toggle_config() {
    local cfg=$1 en=$2
    sed -i "/^# ${cfg} is not set/d; /^${cfg}=/d" "$DEFCONFIG_PATH"
    if [ "$en" = true ]; then echo "${cfg}=y"            >> "$DEFCONFIG_PATH"
    else                       echo "# ${cfg} is not set" >> "$DEFCONFIG_PATH"
    fi
}

# feat_str — display string for all boxes and saved state.
feat_str() {
    if $FEAT_KSU; then
        local hook; $FEAT_SUSFS && hook="SuSFS-Inline-Hook" || hook="Manual-Hook"
        local s="[ReSukiSU:${hook}]"
        local extras=""
        $FEAT_SUSFS && extras="SuSFS"
        $FEAT_KPM   && extras="${extras:+${extras}+}KPM"
        [ -n "$extras" ] && s="${s}  (Features:${extras})"
        echo "$s"
    else
        echo "Vanilla"
    fi
}

# _build_zip_name — derives from _build_log_base (single source of truth)
_build_zip_name() { echo "$(_build_log_base)-{Build-#${BUILD_NUM}}.zip"; }

# =============================================================================
# WINCH HANDLER
# =============================================================================
_MENU_REDRAW_FN=""
_handle_winch() { set_width; [ -n "$_MENU_REDRAW_FN" ] && "$_MENU_REDRAW_FN"; }

# =============================================================================
# SHARED TITLE
# =============================================================================
draw_title() {
    local clang_ver=""
    clang_ver=$(clang --version 2>/dev/null | head -1 | grep -oP '\d+\.\d+\.\d+' | head -1)
    [ -z "$clang_ver" ] && clang_ver="unknown"

    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "VAYU  KERNEL  BUILDER  ─  AnyMore Project"
    box_rule "$MAG" "$DIM"
    box_ctr "$MAG" "$DIM" "Linux 4.14 NonGKI  │  Poco X3 Pro (vayu)  │  Android 16  │  Clang ${clang_ver}"
    box_bot "$MAG"
}

# draw_title_static — skips clang version lookup for menus that redraw on resize
draw_title_static() {
    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "VAYU  KERNEL  BUILDER  ─  AnyMore Project"
    box_rule "$MAG" "$DIM"
    box_ctr "$MAG" "$DIM" "Linux 4.14 NonGKI  │  Poco X3 Pro (vayu)  │  Android 16"
    box_bot "$MAG"
}

# =============================================================================
# STEP 1 — FEATURE CONFIGURATION
# =============================================================================
FEAT_MSG=""
FEAT_MSG_C="$LGR"

_draw_feat_full() {
    _MENU_REDRAW_FN="_draw_feat_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    # ── Features ─────────────────────────────────────────────────────────────
    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "FEATURE  CONFIGURATION"
    box_div "$CYN"

    if $FEAT_KSU; then
        box_kv "$CYN" "$WHT" "$LGR" "  [1]  ReSukiSU" "● ENABLED   "
    else
        box_kv "$CYN" "$GRY" "$GRY" "  [1]  ReSukiSU" "○ DISABLED  "
    fi
    box_rule "$CYN" "$DIM"

    if ! $FEAT_KSU; then
        box_kv "$CYN" "$GRY" "$GRY" "  [2]  SuSFS   " "─ N/A       "
    elif $FEAT_SUSFS; then
        box_kv "$CYN" "$WHT" "$LGR" "  [2]  SuSFS   " "● ENABLED   "
    else
        box_kv "$CYN" "$GRY" "$GRY" "  [2]  SuSFS   " "○ DISABLED  "
    fi
    box_rule "$CYN" "$DIM"

    if ! $FEAT_KSU; then
        box_kv "$CYN" "$GRY" "$GRY" "  [3]  KPM     " "─ N/A       "
    elif $FEAT_KPM; then
        box_kv "$CYN" "$WHT" "$LGR" "  [3]  KPM     " "● ENABLED   "
    else
        box_kv "$CYN" "$GRY" "$GRY" "  [3]  KPM     " "○ DISABLED  "
    fi
    box_div "$CYN"

    # Hook mode inline
    if $FEAT_KSU && $FEAT_SUSFS; then
        box_kv "$CYN" "$DIM" "$CYN" "  Hook Mode" "⇒ SuSFS-Inline-Hook  "
    elif $FEAT_KSU; then
        box_kv "$CYN" "$WHT" "$LGR" "  Hook Mode" "⇒ Manual-Hook  "
    else
        box_kv "$CYN" "$GRY" "$GRY" "  Hook Mode" "─ N/A  "
    fi
    box_bot "$CYN"

    # ── Actions ──────────────────────────────────────────────────────────────
    box_top "$CYN"
    box_row "$CYN" "$MAG" "  [M]  Open Menuconfig"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$LGR" "  [C]  Confirm & Continue"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "  [B]  Back to Mode Select"
    box_row "$CYN" "$GRY" "  [Q]  Quit"

    # ── Session status (menuconfig) ───────────────────────────────────────────
    if $MENUCONFIG_USED; then
        box_rule "$CYN" "$YEL" "Menuconfig"
        box_row "$CYN" "$YEL" "  ⚑  Active this session — defconfig regen skipped at Stage 2"
        box_row "$CYN" "$GRY" "     Use [V] or [D] after build to persist or discard"
    elif $_PRESERVE_ACTIVE; then
        box_rule "$CYN" "$MAG" "Menuconfig"
        box_row "$CYN" "$MAG" "  ⚑  Preserved .config will be restored on next build"
        box_row "$CYN" "$GRY" "     Use [D] after build to make it permanent, or ignore to expire"
    fi
    box_bot "$CYN"

    # ── Feedback message ──────────────────────────────────────────────────────
    if [ -n "$FEAT_MSG" ]; then
        box_top "$FEAT_MSG_C"
        box_ctr "$FEAT_MSG_C" "$FEAT_MSG_C" "$FEAT_MSG"
        box_bot "$FEAT_MSG_C"
    fi

    printf '\033[J'
    _cursor_show
    printf "\n${WHT}  Select [1/2/3/M/C/B/Q]: ${NC}"
}

# Returns 0=confirmed, 1=back
run_feat_menu() {
    FEAT_MSG=""
    _draw_feat_full

    while true; do
        IFS= read -r -s -n1 choice
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input
            FEAT_MSG=""; _draw_feat_full; continue
        fi
        _drain_input
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            1)  if $FEAT_KSU; then
                    FEAT_KSU=false;   toggle_config "CONFIG_KSU"             false
                    FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS"       false
                    FEAT_KPM=false;   toggle_config "CONFIG_KPM"             false
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" false
                    FEAT_MSG="ReSukiSU disabled — SuSFS + KPM + ManualHook also cleared"
                    FEAT_MSG_C="$YEL"
                else
                    FEAT_KSU=true; toggle_config "CONFIG_KSU" true
                    if ! $FEAT_SUSFS; then toggle_config "CONFIG_KSU_MANUAL_HOOK" true; fi
                    FEAT_MSG="ReSukiSU enabled"; FEAT_MSG_C="$LGR"
                fi
                _draw_feat_full ;;

            2)  if ! $FEAT_KSU; then
                    FEAT_MSG="Enable ReSukiSU first — SuSFS requires it"
                    FEAT_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if $FEAT_SUSFS; then
                    FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS"       false
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" true
                    FEAT_MSG="SuSFS disabled — Manual-Hook auto-enabled"; FEAT_MSG_C="$LGR"
                else
                    FEAT_SUSFS=true;  toggle_config "CONFIG_KSU_SUSFS"       true
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" false
                    FEAT_MSG="SuSFS enabled — SuSFS-Inline-Hook active"; FEAT_MSG_C="$YEL"
                fi
                _draw_feat_full ;;

            3)  if ! $FEAT_KSU; then
                    FEAT_MSG="Enable ReSukiSU first — KPM requires it"
                    FEAT_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if $FEAT_KPM; then
                    FEAT_KPM=false; toggle_config "CONFIG_KPM" false
                    FEAT_MSG="KPM disabled"; FEAT_MSG_C="$GRY"
                else
                    FEAT_KPM=true;  toggle_config "CONFIG_KPM" true
                    FEAT_MSG="KPM enabled";  FEAT_MSG_C="$LGR"
                fi
                _draw_feat_full ;;

            m)  do_clear
                # Capture .config mtime before opening menuconfig
                local _mc_mtime_before
                _mc_mtime_before=$(stat -c %Y "${objdir}/.config" 2>/dev/null || echo "0")
                # Ignore SIGINT in parent — Ctrl+C cancels menuconfig only
                trap '' INT
                make -C "$kernel_dir" O="$objdir" \
                    ARCH=arm64 LLVM=1 LLVM_IAS=1 \
                    CC="$MAKE_CC" \
                    menuconfig
                local _mc_rc=$?
                trap - INT
                tput reset
                local _mc_mtime_after
                _mc_mtime_after=$(stat -c %Y "${objdir}/.config" 2>/dev/null || echo "0")

                if [ "$_mc_rc" -ne 0 ]; then
                    FEAT_MSG="Menuconfig aborted — no changes applied"
                    FEAT_MSG_C="$YEL"
                elif [ "$_mc_mtime_after" != "$_mc_mtime_before" ]; then
                    # User saved — activate skip-defconfig for this session
                    MENUCONFIG_USED=true
                    _SKIP_DEFCONFIG=true
                    _PRESERVE_ACTIVE=false
                    rm -f "$MENUCONFIG_PRESERVE_FILE"
                    read_features
                    FEAT_MSG="Menuconfig saved — Stage 2 defconfig regen skipped this session"
                    FEAT_MSG_C="$CYN"
                else
                    FEAT_MSG="Menuconfig closed without saving — no changes"
                    FEAT_MSG_C="$YEL"
                fi
                _draw_feat_full ;;

            c)  return 0 ;;
            b)  return 1 ;;
            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   FEAT_MSG="Unknown key — use 1/2/3/M/C/B/Q"; FEAT_MSG_C="$LRD"
                 _draw_feat_full ;;
        esac
    done
}

# =============================================================================
# STEP 2 — BUILD OPTIONS
# =============================================================================
BUILD_MSG=""
BUILD_MSG_C="$LGR"

_draw_build_full() {
    _MENU_REDRAW_FN="_draw_build_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"

    # ── Active feature summary ────────────────────────────────────────────────
    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "ACTIVE  FEATURES"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$WHT" "  ${fs}"
    box_bot "$CYN"

    # ── Build options — order: Name / Incremental / ccache / Start ─────────────
    box_top "$BLU"
    box_ctr "$BLU" "$YEL" "BUILD  OPTIONS"
    box_div "$BLU"

    # 1 — Kernel Name
    if [ -n "$KERNEL_NAME" ]; then
        box_wrap "$BLU" "$WHT" "  [N]  Kernel Name" "${KERNEL_NAME}"
    else
        box_row "$BLU" "$GRY" "  [N]  Set Kernel Name    (optional — appended to LOCALVERSION)"
    fi
    box_rule "$BLU" "$DIM"

    # 2 — Incremental
    if $INCREMENTAL; then
        box_kv "$BLU" "$WHT" "$YEL" "  [I]  Incremental Build" "● ON  "
    else
        box_kv "$BLU" "$GRY" "$GRY" "  [I]  Incremental Build" "○ OFF "
    fi
    box_rule "$BLU" "$DIM"

    # 3 — ccache
    if [ -z "$CCACHE_BIN" ]; then
        box_kv "$BLU" "$GRY" "$GRY" "  [C]  ccache" "─ N/A "
    elif $USE_CCACHE; then
        box_kv "$BLU" "$WHT" "$YEL" "  [C]  ccache" "● ON  "
    else
        box_kv "$BLU" "$GRY" "$GRY" "  [C]  ccache" "○ OFF "
    fi
    box_rule "$BLU" "$DIM"

    # 4 — Start
    box_row "$BLU" "$LGR" "  [S]  Start Build"

    box_div "$BLU"
    box_row "$BLU" "$GRY" "  [B]  Back to Features"
    box_row "$BLU" "$LRD" "  [X]  Reset Build Counter  (#${BUILD_NUM} → #1)"
    box_row "$BLU" "$GRY" "  [Q]  Quit"
    box_bot "$BLU"

    if [ -n "$BUILD_MSG" ]; then
        box_top "$BUILD_MSG_C"
        box_ctr "$BUILD_MSG_C" "$BUILD_MSG_C" "$BUILD_MSG"
        box_bot "$BUILD_MSG_C"
    fi
    printf '\033[J'
    _cursor_show
    printf "\n${WHT}  Select [N/I/C/S/B/X/Q]: ${NC}"
}

prompt_kernel_name() {
    local MAX_NAME_LEN=50
    _winch_disable
    set_width
    do_clear
    draw_title_static
    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "SET  KERNEL  NAME"
    box_div "$CYN"
    if [ -n "$KERNEL_NAME" ]; then
        box_wrap "$CYN" "$WHT" "  Current    " "${KERNEL_NAME}"
    else
        box_row "$CYN" "$GRY" "  Current     : (none — base version used as-is)"
    fi
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "  Sets the LOCALVERSION suffix appended after the base kernel version."
    box_row "$CYN" "$GRY" "  Example input : AnyMore-v2.1"
    box_row "$CYN" "$GRY" "  Result        : uname -r shows  4.14.356-AnyMore-v2.1"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "  Used in the zip / log filename as the kernel label."
    box_row "$CYN" "$GRY" "  Max ${MAX_NAME_LEN} chars. Quotes, slashes, brackets stripped."
    box_row "$CYN" "$GRY" "  Press Enter with no input to clear and restore the default."
    box_bot "$CYN"
    printf "\n${WHT}  Kernel Name: ${NC}"
    _cursor_show
    IFS= read -r _kname_input
    _kname_input=$(printf '%s' "$_kname_input" | tr -d '"'"'"'\/[]{}\\')
    if [ ${#_kname_input} -gt $MAX_NAME_LEN ]; then
        _kname_input="${_kname_input:0:$MAX_NAME_LEN}"
    fi
    KERNEL_NAME="$_kname_input"
    if [ -n "$KERNEL_NAME" ]; then
        local _short_name="$KERNEL_NAME"
        [ ${#_short_name} -gt 30 ] && _short_name="${_short_name:0:27}…"
        BUILD_MSG="Kernel name set: \"${_short_name}\""
    else
        BUILD_MSG="Kernel name cleared — default naming active"
    fi
    BUILD_MSG_C="$LGR"
    _winch_enable
}


# Returns 0=build started, 1=back to features
run_build_menu() {
    BUILD_MSG=""
    _draw_build_full

    while true; do
        IFS= read -r -s -n1 choice
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input
            BUILD_MSG=""; _draw_build_full; continue
        fi
        _drain_input
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            n)  prompt_kernel_name; _draw_build_full ;;
            i)  if $INCREMENTAL; then
                    INCREMENTAL=false
                    BUILD_MSG="Incremental OFF — full clean build"; BUILD_MSG_C="$GRY"
                else
                    INCREMENTAL=true
                    BUILD_MSG="Incremental ON — keeps previous objects"; BUILD_MSG_C="$YEL"
                fi
                _save_state
                _draw_build_full ;;
            c)  if [ -z "$CCACHE_BIN" ]; then
                    BUILD_MSG="ccache binary not found — install ccache to enable"; BUILD_MSG_C="$LRD"
                elif $USE_CCACHE; then
                    USE_CCACHE=false
                    BUILD_MSG="ccache OFF — builds will be slower"; BUILD_MSG_C="$GRY"
                    _save_state; _update_make_cc
                else
                    USE_CCACHE=true
                    BUILD_MSG="ccache ON — compiler cache active"; BUILD_MSG_C="$YEL"
                    _save_state; _update_make_cc
                fi
                _draw_build_full ;;
            s)  _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build; return 0 ;;
            x)  reset_build_num
                BUILD_MSG="Build counter reset — next successful build will be #1"
                BUILD_MSG_C="$LRD"
                _draw_build_full ;;
            b)  return 1 ;;
            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   BUILD_MSG="Unknown key — use N/I/C/S/B/X/Q"; BUILD_MSG_C="$LRD"
                 _draw_build_full ;;
        esac
    done
}

# =============================================================================
# BUILD SUMMARY
# =============================================================================
draw_summary() {
    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    local mode="Full Clean Build"; $INCREMENTAL && mode="Incremental"
    local cc_label
    if [ -z "$CCACHE_BIN" ]; then cc_label="not found"
    elif $USE_CCACHE; then       cc_label="enabled"
    else                          cc_label="disabled"
    fi

    box_top "$YEL"
    box_ctr "$YEL" "$YEL" "BUILD  SUMMARY"
    box_div "$YEL"
    box_row "$YEL" "$WHT" "  Target      : vayu_a16_kernel (Linux 4.14 NonGKI)"
    box_row "$YEL" "$WHT" "  Build       : #${BUILD_NUM}"
    [ -n "$KERNEL_NAME" ] && \
        box_wrap "$YEL" "$WHT" "  Kernel-Name" "${KERNEL_NAME}"
    box_row "$YEL" "$WHT" "  Features    : ${fs}"
    box_row "$YEL" "$WHT" "  Mode        : ${mode}"
    box_row "$YEL" "$WHT" "  ccache      : ${cc_label}"
    box_row "$YEL" "$WHT" "  Output      : ${OUTPUT_DIR}"
    if $_SKIP_DEFCONFIG; then
        box_rule "$YEL" "$YEL" "Menuconfig"
        if $_PRESERVE_ACTIVE; then
            box_row "$YEL" "$YEL" "  ⚑  Restoring preserved .config from previous session"
        else
            box_row "$YEL" "$YEL" "  ⚑  Using in-session menuconfig .config  (defconfig skipped)"
        fi
    fi
    box_bot "$YEL"
}

# =============================================================================
# ASCII ART
# =============================================================================
print_success_art() {
    printf "${LGR}"
    printf '  ░█▀▀░█░█░█▀▀░█▀▀░█▀▀░█▀▀░█▀▀░█░░\n'
    printf '  ░▀▀█░█░█░█░░░█░░░█▀▀░▀▀█░▀▀█░▀░░\n'
    printf '  ░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀░░\n'
    printf "${NC}\n"
}

print_failed_art() {
    printf "${LRD}"
    printf '  ░█▀▀░█▀█░▀█▀░█░░░█▀▀░█▀▄░▀▀█\n'
    printf '  ░█▀▀░█▀█░░█░░█░░░█▀▀░█░█░░▀░\n'
    printf '  ░▀░░░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░░▀░\n'
    printf "${NC}\n"
}

print_cancelled_art() {
    printf "${YEL}"
    printf '  ░█▀▀░▀█▀░█▀█░█▀█░█▀█░█▀▀░█░░░█░░░█▀▀░█▀▄\n'
    printf '  ░█░░░░█░░█▀█░█░█░█░░░█▀▀░█░░░█░░░█▀▀░█░█\n'
    printf '  ░▀▀▀░▀▀▀░▀░▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░\n'
    printf "${NC}\n"
}

# =============================================================================
# RESULT BOXES
# =============================================================================
print_success_box() {
    local elapsed=$1 zippath=${2:-""}
    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    local zipname="${zippath##*/}"
    local zip_size; zip_size=$(du -h "$zippath" 2>/dev/null | cut -f1)
    local cc_label
    if [ -z "$CCACHE_BIN" ]; then cc_label="not found"
    elif $USE_CCACHE; then       cc_label="enabled"
    else                          cc_label="disabled"
    fi
    log_sep "OUTPUT"
    print_success_art
    box_top "$LGR"
    box_row "$LGR" "$WHT" "  Build       : #${BUILD_NUM}"
    [ -n "$KERNEL_NAME" ] && \
        box_wrap "$LGR" "$WHT" "  Kernel-Name" "${KERNEL_NAME}"
    box_row "$LGR" "$WHT" "  Features    : ${fs}"
    box_row "$LGR" "$WHT" "  ccache      : ${cc_label}"
    box_row "$LGR" "$WHT" "  Time        : ${elapsed}"
    box_row "$LGR" "$WHT" "  When        : $(date '+%Y-%m-%d %H:%M')"
    box_rule "$LGR" "$LGR"
    box_wrap "$LGR" "$LGR" "  Zip        " "$zipname"
    [ -n "$zip_size" ] && box_row "$LGR" "$GRY" "  Size        : ${zip_size}"
    box_row "$LGR" "$GRY" "  Output      : ${OUTPUT_DIR}"
    box_bot "$LGR"
}

print_fail_box() {
    local logfile=$1
    log_sep "OUTPUT"
    print_failed_art
    box_top "$LRD"
    box_ctr "$LRD" "$WHT" "Build failed — see errors below"
    box_div "$LRD"
    if [ -f "$logfile" ]; then
        local maxw=$(( W-6 ))
        # Linker errors (most actionable — undefined symbols, etc.)
        local ld_errs; ld_errs=$(grep "ld\.lld:.*error:" "$logfile" 2>/dev/null | tail -4)
        # Compiler errors
        local cc_errs; cc_errs=$(grep -E "error:" "$logfile" 2>/dev/null | grep -v "ld\.lld:" | tail -6)
        local shown=false
        if [ -n "$ld_errs" ]; then
            box_ctr "$LRD" "$YEL" "LINKER"
            box_rule "$LRD" "$DIM"
            while IFS= read -r line; do
                [ ${#line} -gt $maxw ] && line="${line:0:$(( maxw-1 ))}…"
                box_row "$LRD" "$YEL" "  ${line}"
            done <<< "$ld_errs"
            shown=true
        fi
        if [ -n "$cc_errs" ]; then
            $shown && box_rule "$LRD" "$DIM"
            box_ctr "$LRD" "$RED" "COMPILER"
            box_rule "$LRD" "$DIM"
            while IFS= read -r line; do
                [ ${#line} -gt $maxw ] && line="${line:0:$(( maxw-1 ))}…"
                box_row "$LRD" "$RED" "  ${line}"
            done <<< "$cc_errs"
            shown=true
        fi
        if ! $shown; then
            box_ctr "$LRD" "$GRY" "(no error: lines found — check log for details)"
        fi
    else
        box_ctr "$LRD" "$GRY" "(log file not found)"
    fi
    box_rule "$LRD" "$GRY"
    box_wrap "$LRD" "$GRY" "  Log        " "${logfile##*/}"
    box_bot "$LRD"
}

print_cancelled_box() {
    local elapsed=$1
    log_sep "OUTPUT"
    print_cancelled_art
    box_top "$YEL"
    box_ctr "$YEL" "$WHT" "Build cancelled by user"
    box_rule "$YEL" "$GRY"
    box_row "$YEL" "$GRY" "  Elapsed     : ${elapsed}"
    box_row "$YEL" "$GRY" "  Objects in out/ are intact for incremental retry"
    box_bot "$YEL"
}

# =============================================================================
# POST-BUILD PROMPT
# Returns: 0=menu  1=retry Full Clean  2=retry Incremental
# [V] and [D] only shown when MENUCONFIG_USED=true.
# =============================================================================
post_build_prompt() {
    printf "\n"
    box_top "$WHT"
    box_ctr "$WHT" "$YEL" "WHAT  NEXT?"
    box_div "$WHT"
    box_row "$WHT" "$YEL" "  [T]  Retry — Full Clean"
    box_row "$WHT" "$CYN" "  [I]  Retry — Incremental"
    if $MENUCONFIG_USED; then
        box_rule "$WHT" "$MAG" "Menuconfig"
        box_row "$WHT" "$MAG" "  [V]  Preserve for next ./build.sh run"
        box_row "$WHT" "$GRY" "       (saves .config to disk, restored automatically on relaunch)"
        box_row "$WHT" "$LGR" "  [D]  Write permanently to vayu_defconfig"
        box_row "$WHT" "$GRY" "       (runs savedefconfig — becomes the new build default)"
    fi
    box_rule "$WHT" "$DIM"
    box_row "$WHT" "$WHT" "  [R]  Return to menu"
    box_row "$WHT" "$WHT" "  [E]  Exit script"
    box_bot "$WHT"

    if $MENUCONFIG_USED; then
        printf "\n${WHT}  Select [T/I/V/D/R/E]: ${NC}"
    else
        printf "\n${WHT}  Select [T/I/R/E]: ${NC}"
    fi

    while true; do
        IFS= read -r -s -n1 key
        if [ "$key" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input; continue
        fi
        _drain_input
        key=$(printf '%s' "$key" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        case "$key" in
            r)  return 0 ;;
            t)  return 1 ;;
            i)  return 2 ;;
            e)  printf "\n${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            v)  if $MENUCONFIG_USED; then
                    if [ -f "${objdir}/.config" ]; then
                        cp "${objdir}/.config" "$MENUCONFIG_PRESERVE_FILE"
                        printf "\n${LGR}  ✓ .config preserved — will be restored on next ./build.sh run.${NC}\n"
                        printf "${GRY}    Delete %s to cancel.${NC}\n" "$MENUCONFIG_PRESERVE_FILE"
                    else
                        printf "\n${YEL}  No .config found in out/ — nothing to preserve.${NC}\n"
                    fi
                    sleep 1
                fi
                continue ;;
            d)  if $MENUCONFIG_USED; then
                    printf "\n${CYN}  Running savedefconfig...${NC}\n"
                    make -C "$kernel_dir" O="$objdir" \
                        ARCH=arm64 LLVM=1 LLVM_IAS=1 \
                        CC="$MAKE_CC" \
                        savedefconfig 2>&1 | \
                        while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
                    if [ -f "${objdir}/defconfig" ]; then
                        cp "${objdir}/defconfig" "$DEFCONFIG_PATH"
                        MENUCONFIG_USED=false
                        _SKIP_DEFCONFIG=false
                        rm -f "$MENUCONFIG_PRESERVE_FILE"
                        printf "${LGR}  ✓ vayu_defconfig updated — changes are now permanent defaults.${NC}\n"
                        printf "${GRY}    [V]/[D] options hidden (no pending menuconfig changes).${NC}\n"
                    else
                        printf "${LRD}  savedefconfig failed — defconfig not written.${NC}\n"
                    fi
                    sleep 2
                fi
                continue ;;
            '') continue ;;
        esac
    done
}

# =============================================================================
# SIGINT HANDLER
# =============================================================================
_MAKE_PID=""
_CANCELLED=false
_build_sigint() {
    _CANCELLED=true
    if [ -n "$_MAKE_PID" ]; then
        kill -TERM -- "-$_MAKE_PID" 2>/dev/null
        kill -TERM "$_MAKE_PID"     2>/dev/null
    fi
}

# =============================================================================
# DO-PACKAGE — copy images into AnyKernel3 and zip
# =============================================================================
_LAST_ZIP=""

do_package() {
    local zipname; zipname=$(_build_zip_name)
    local zippath="${OUTPUT_DIR}/${zipname}"

    log_sep "STAGE 4 — PACKAGE"
    box_top "$MAG"
    box_ctr "$MAG" "$MAG" "Packaging AnyKernel3 zip..."
    box_rule "$MAG" "$DIM"
    box_row "$MAG" "$GRY" "  Output      : ${zipname}"
    box_bot "$MAG"
    printf "\n"

    cp "${objdir}/arch/arm64/boot/Image"    "${anykernel}/Image"
    cp "${objdir}/arch/arm64/boot/dtbo.img" "${anykernel}/dtbo.img" 2>/dev/null || true
    cp "${objdir}/arch/arm64/boot/dtb.img"  "${anykernel}/dtb.img"  2>/dev/null || true

    local _zip_rc_file; _zip_rc_file=$(mktemp /tmp/vkb_zip_XXXXXX)
    (
        cd "$anykernel" && \
        zip -r9 "$zippath" . -x '*.git*' 2>&1 | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        echo "${PIPESTATUS[0]}" > "$_zip_rc_file"
    )
    local ZIP_EXIT; ZIP_EXIT=$(cat "$_zip_rc_file" 2>/dev/null); rm -f "$_zip_rc_file"
    ZIP_EXIT=$(( ${ZIP_EXIT:-1} + 0 ))

    if [ "$ZIP_EXIT" -eq 0 ] && [ -f "$zippath" ]; then
        local zip_size; zip_size=$(du -h "$zippath" 2>/dev/null | cut -f1)
        printf "\n  ${LGR}✓ Zip created${NC}  ${GRY}(${zip_size})${NC}\n"
        _LAST_ZIP="$zippath"
        return 0
    else
        _LAST_ZIP=""
        return 1
    fi
}

# =============================================================================
# UPDATE RESUKISU DRIVER
# =============================================================================
do_update_resukisu() {
    _winch_disable
    set_width
    do_clear

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "UPDATE  ReSukiSU  DRIVER"
    box_div "$CYN"
    box_row "$CYN" "$WHT" "  Source      : github.com/ReSukiSU/ReSukiSU"
    box_row "$CYN" "$WHT" "  Branch      : main"
    box_row "$CYN" "$WHT" "  Target      : ${kernel_dir}/drivers/kernelsu"
    box_bot "$CYN"
    printf "\n"

    log_sep "RUNNING SETUP.SH"

    local setup_url="https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh"
    local _rc_file; _rc_file=$(mktemp /tmp/vkb_upd_XXXXXX)

    # If drivers/kernelsu is a real directory (not a symlink), setup.sh's
    # ln -sf will silently place the symlink inside it instead of replacing it.
    # Move it out of the way so ln -sf can create the symlink correctly.
    local _ksu_dir="${kernel_dir}/drivers/kernelsu"
    if [ -d "$_ksu_dir" ] && [ ! -L "$_ksu_dir" ]; then
        printf "  ${YEL}[!] drivers/kernelsu is a real dir — moving to .bak for symlink creation${NC}\n"
        mv "$_ksu_dir" "${_ksu_dir}.bak"
    fi

    (
        cd "$kernel_dir" || exit 1
        bash <(curl -LSs "$setup_url")
        echo "$?" > "$_rc_file"
    ) 2>&1 | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done

    local UPDATE_EXIT; UPDATE_EXIT=$(cat "$_rc_file" 2>/dev/null); rm -f "$_rc_file"
    UPDATE_EXIT=$(( ${UPDATE_EXIT:-1} + 0 ))

    log_sep "VERIFY"

    local ksu_c_count
    ksu_c_count=$(find -L "${kernel_dir}/drivers/kernelsu" -name "*.c" 2>/dev/null | wc -l)
    local kbuild_ok=false
    [ -f "${kernel_dir}/drivers/kernelsu/Kbuild" ] && kbuild_ok=true

    printf "\n"
    if [ "$UPDATE_EXIT" -eq 0 ] && [ "$ksu_c_count" -gt 5 ] && $kbuild_ok; then
        box_top "$LGR"
        box_ctr "$LGR" "$LGR" "ReSukiSU driver updated successfully"
        box_rule "$LGR" "$DIM"
        box_row "$LGR" "$WHT" "  .c files    : ${ksu_c_count} found"
        box_row "$LGR" "$WHT" "  Kbuild      : present"
        box_bot "$LGR"
    else
        box_top "$LRD"
        box_ctr "$LRD" "$LRD" "Update failed or source tree incomplete"
        box_rule "$LRD" "$DIM"
        box_row "$LRD" "$WHT" "  .c files found  : ${ksu_c_count}  (expect > 5)"
        box_row "$LRD" "$WHT" "  Kbuild present  : $( $kbuild_ok && echo YES || echo NO )"
        box_row "$LRD" "$YEL" "  Check curl / network and retry"
        box_bot "$LRD"
    fi

    printf "\n"
    box_top "$WHT"
    box_row "$WHT" "$WHT" "  [R]  Return to menu"
    box_bot "$WHT"
    printf "\n${WHT}  Press [R] to return: ${NC}"

    while true; do
        IFS= read -r -s -n1 key
        [ "$key" = $'\033' ] && { IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null; _drain_input; continue; }
        _drain_input
        key=$(printf '%s' "$key" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        [ "$key" = "r" ] && break
        [ "$key" = "" ] && continue
    done
}

# =============================================================================
# BUILD
# =============================================================================
do_build() {
    _read_build_num
    _winch_disable
    set_width
    do_clear

    draw_summary
    printf "\n"

    # ── Stage 1: Clean ───────────────────────────────────────────────────────
    log_sep "STAGE 1 — CLEAN"
    if $INCREMENTAL; then
        box_top "$GRY"; box_ctr "$GRY" "$GRY" "Incremental — keeping previous objects"; box_bot "$GRY"
        printf "\n"
    else
        # Rescue .config before mrproper wipes out/ (needed when menuconfig was used)
        local _rescued_cfg=""
        if $_SKIP_DEFCONFIG && [ -f "${objdir}/.config" ]; then
            _rescued_cfg=$(mktemp /tmp/vkb_cfg_XXXXXX)
            cp "${objdir}/.config" "$_rescued_cfg"
        fi
        box_top "$CYN"; box_ctr "$CYN" "$CYN" "Cleaning previous build output..."; box_bot "$CYN"
        printf "\n"
        make -C "$kernel_dir" O="$objdir" clean    2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        make -C "$kernel_dir" O="$objdir" mrproper 2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        # Restore rescued .config so Stage 3 can skip defconfig regen
        if [ -n "$_rescued_cfg" ] && [ -f "$_rescued_cfg" ]; then
            mkdir -p "$objdir"
            cp "$_rescued_cfg" "${objdir}/.config"
            rm -f "$_rescued_cfg"
        fi
        printf "\n"
    fi

    # ── Stage 2: Defconfig ──────────────────────────────────────────────────
    toggle_config "CONFIG_KSU_SUSFS" "$FEAT_SUSFS"
    toggle_config "CONFIG_KPM"       "$FEAT_KPM"
    if $FEAT_KSU && ! $FEAT_SUSFS; then
        toggle_config "CONFIG_KSU_MANUAL_HOOK"                  true
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INPUT_HOOK"  false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_SETUID_HOOK" false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INITRC_HOOK" false
    else
        toggle_config "CONFIG_KSU_MANUAL_HOOK"                  false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INPUT_HOOK"  false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_SETUID_HOOK" false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INITRC_HOOK" false
    fi

    log_sep "STAGE 2 — DEFCONFIG"
    if $_SKIP_DEFCONFIG; then
        if $_PRESERVE_ACTIVE && [ -f "$MENUCONFIG_PRESERVE_FILE" ]; then
            box_top "$MAG"
            box_ctr "$MAG" "$WHT" "Restoring preserved menuconfig .config"
            box_bot "$MAG"
            printf "\n"
            mkdir -p "$objdir"
            cp "$MENUCONFIG_PRESERVE_FILE" "${objdir}/.config"
            _PRESERVE_ACTIVE=false
        else
            box_top "$YEL"
            box_ctr "$YEL" "$WHT" "Defconfig skipped — in-session menuconfig .config in use"
            box_bot "$YEL"
            printf "\n"
        fi
        # Resolve any new Kconfig symbols absent from the saved .config
        make -C "$kernel_dir" O="$objdir" \
            ARCH=arm64 LLVM=1 LLVM_IAS=1 \
            CC="$MAKE_CC" \
            olddefconfig 2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        printf "\n"
    else
        box_top "$CYN"
        box_ctr "$CYN" "$WHT" "Applying: arch/arm64/configs/${CONFIG_FILE}"
        box_bot "$CYN"
        printf "\n"
        make -C "$kernel_dir" O="$objdir" \
            ARCH=arm64 LLVM=1 LLVM_IAS=1 \
            CC="$MAKE_CC" \
            "$CONFIG_FILE" 2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        printf "\n"
    fi

    mkdir -p "$OUTPUT_DIR"
    local fail_log="${OUTPUT_DIR}/$(_fail_log_name)"

    # ── Stage 3: Compile ─────────────────────────────────────────────────────
    log_sep "STAGE 3 — COMPILE"
    box_top "$CYN"
    box_ctr "$CYN" "$CYN" "COMPILING  KERNEL"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "  $(nproc --all) threads  │  $(date '+%H:%M')"
    box_bot "$CYN"
    printf "\n"

    local t0; t0=$(date +%s)

    # tee writes build output to fail_log in real time.
    # On success the log is renamed to the success log.
    # On failure it stays as FAIL.log.
    local _fifo; _fifo=$(mktemp -u /tmp/vkb_XXXXXX)
    mkfifo "$_fifo"
    tee "$fail_log" < "$_fifo" &
    local _TEE_PID=$!

    _MAKE_PID=""; _CANCELLED=false
    trap '_build_sigint' INT

    # ── Resolve LOCALVERSION ──────────────────────────────────────────────
    # When KERNEL_NAME is set, pass it as LOCALVERSION so uname -r shows
    # 4.14.356-<KERNEL_NAME>.  Also overwrite .scmversion with empty content
    # so setlocalversion exits early and does NOT append the git +N dirty count
    # or inject CONFIG_LOCALVERSION from defconfig.  Existing .scmversion
    # content is backed up and restored after the build completes.
    local _make_localver=()
    local _scmver_created=false
    local _scmver_backup=""
    if [ -n "$KERNEL_NAME" ]; then
        _make_localver=(
            "LOCALVERSION=-${KERNEL_NAME}"
            "CONFIG_LOCALVERSION="
        )
        _scmver_backup=$(cat "${kernel_dir}/.scmversion" 2>/dev/null || true)
        printf '' > "${kernel_dir}/.scmversion"
        _scmver_created=true
    fi

    setsid make -C "$kernel_dir" O="$objdir" \
        ARCH=arm64 \
        LLVM=1 LLVM_IAS=1 \
        CC="$MAKE_CC" \
        CLANG_TRIPLE=aarch64-linux-gnu- \
        CROSS_COMPILE=aarch64-linux-gnu- \
        CROSS_COMPILE_ARM32=arm-linux-gnueabi- \
        "${_make_localver[@]}" \
        -j"$(nproc --all)" \
        > "$_fifo" 2>&1 &
    _MAKE_PID=$!
    wait "$_MAKE_PID"
    BUILD_EXIT=$?
    wait "$_TEE_PID" 2>/dev/null
    rm -f "$_fifo"

    # Restore .scmversion to its pre-build state
    if $_scmver_created; then
        if [ -n "$_scmver_backup" ]; then
            printf '%s' "$_scmver_backup" > "${kernel_dir}/.scmversion"
        else
            rm -f "${kernel_dir}/.scmversion"
        fi
    fi

    trap - INT

    local t1; t1=$(date +%s)
    local s=$(( t1 - t0 ))
    local elapsed
    [ "$s" -ge 60 ] && elapsed="$(( s/60 ))m $(( s%60 ))s" || elapsed="${s}s"

    # ── Cancelled ────────────────────────────────────────────────────────────
    if $_CANCELLED; then
        rm -f "$fail_log" 2>/dev/null || true
        print_cancelled_box "$elapsed"
        post_build_prompt; local _pbrc=$?
        if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        fi
        return
    fi

    # ── Failed ───────────────────────────────────────────────────────────────
    if [ "$BUILD_EXIT" -ne 0 ] || [ ! -f "${objdir}/arch/arm64/boot/Image" ]; then
        print_fail_box "$fail_log"
        post_build_prompt; local _pbrc=$?
        if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        fi
        return
    fi

    # ── Image info ───────────────────────────────────────────────────────────
    local img_path="${objdir}/arch/arm64/boot/Image"
    local img_size; img_size=$(du -h "$img_path" 2>/dev/null | cut -f1)
    printf "\n  ${LGR}✓ Image built${NC}  ${GRY}(${img_size})${NC}\n"

    # ── Stage 5: Package ─────────────────────────────────────────────────────
    if do_package; then
        _commit_build_num
        _save_prev_state
        _rm_old_success_log
        local slog_name; slog_name=$(_success_log_name "$BUILD_NUM")
        cp "$fail_log" "${OUTPUT_DIR}/${slog_name}"
        rm -f "$fail_log"
        find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-CANCEL.log" \
            -delete 2>/dev/null || true
        _rm_old_zip "$_LAST_ZIP"
        print_success_box "$elapsed" "$_LAST_ZIP"
    else
        print_fail_box "$fail_log"
    fi

    post_build_prompt; local _pbrc=$?
    if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
    elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
    fi
}

# =============================================================================
# PACKAGE-ONLY — zip existing images without recompiling
# =============================================================================
do_package_only() {
    _read_build_num
    _winch_disable
    set_width
    do_clear

    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    mkdir -p "$OUTPUT_DIR"

    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "PACKAGE  EXISTING  IMAGES"
    box_div "$MAG"
    box_row "$MAG" "$WHT" "  Image       : ${objdir}/arch/arm64/boot/Image"
    box_row "$MAG" "$WHT" "  Feat        : ${fs}"
    box_bot "$MAG"
    printf "\n"

    if do_package; then
        _commit_build_num
        _save_prev_state
        _rm_old_success_log
        local slog_name; slog_name=$(_success_log_name "$BUILD_NUM")
        printf "[%s-Project] Package-only build #%s  %s\n" \
            "$PROJECT_NAME" "$BUILD_NUM" "$(date)" > "${OUTPUT_DIR}/${slog_name}"
        find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-CANCEL.log" \
            -delete 2>/dev/null || true
        _rm_old_zip "$_LAST_ZIP"
        print_success_box "N/A (package only)" "$_LAST_ZIP"
    else
        log_sep "OUTPUT"
        print_failed_art
        box_top "$LRD"; box_ctr "$LRD" "$LRD" "Packaging failed — check zip tool"; box_bot "$LRD"
    fi

    post_build_prompt; local _pbrc=$?
    if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
    elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
    fi
}

# =============================================================================
# STARTUP MODE MENU
# =============================================================================
_draw_mode_menu() {
    _MENU_REDRAW_FN="_draw_mode_menu"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    local has_image=false
    [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true

    # ── Previous build ────────────────────────────────────────────────────────
    _load_prev_state
    box_top "$GRY"
    box_ctr "$GRY" "$DIM" "Previous Build"
    box_div "$GRY"
    if [ -z "$PREV_NUM" ]; then
        box_ctr "$GRY" "$GRY" "No previous build recorded"
    else
        # Build # and date on their own row — always short, never clashes
        box_kv  "$GRY" "$GRY" "$GRY" \
            "  Build       : #${PREV_NUM}" \
            "${PREV_DATE:+${PREV_DATE} }"
        [ -n "$PREV_KNAME" ] && \
            box_wrap "$GRY" "$GRY" "  Kernel-Name" "${PREV_KNAME}"
        box_row "$GRY" "$GRY" "  Mode        : ${PREV_MODE}"
        box_rule "$GRY" "$DIM"
        # Features can be long — wrap onto next row if needed
        box_wrap "$GRY" "$GRY" "  Features   " "${PREV_FEAT}"
    fi
    box_bot "$GRY"

    # ── Mode selection ────────────────────────────────────────────────────────
    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "SELECT  MODE"
    box_div "$CYN"
    box_row "$CYN" "$WHT" "  [B]  Full Build  (configure → compile → package)"
    if $has_image; then
        box_row "$CYN" "$LGR" "  [P]  Package Existing Image  (skip compile)"
    else
        box_row "$CYN" "$GRY" "       Package Only — no Image found in out/"
    fi
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$CYN" "  [U]  Update ReSukiSU Driver"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "  [Q]  Quit"
    box_bot "$CYN"

    # ── Session indicators ────────────────────────────────────────────────────
    if $_PRESERVE_ACTIVE; then
        box_top "$MAG"
        box_ctr "$MAG" "$MAG" "⚑  Preserved menuconfig .config — will be restored on next build"
        box_bot "$MAG"
    fi

    printf '\033[J'
    _cursor_show
    if $has_image; then
        printf "\n${WHT}  Select [B/P/U/Q]: ${NC}"
    else
        printf "\n${WHT}  Select [B/U/Q]: ${NC}"
    fi
}

run_mode_menu() {
    _draw_mode_menu
    local has_image=false
    [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true

    while true; do
        IFS= read -r -s -n1 choice
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input
            _draw_mode_menu
            has_image=false; [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true
            continue
        fi
        _drain_input
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        case "$choice" in
            b)  return 0 ;;
            p)  $has_image && return 2 ;;
            u)  do_update_resukisu
                _draw_mode_menu
                has_image=false; [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true
                continue ;;
            q)  printf "\n${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '') continue ;;
        esac
    done
}

# =============================================================================
# MAIN LOOP
# =============================================================================
do_clear
read_features

while true; do
    run_mode_menu
    MODE_RC=$?

    if [ "$MODE_RC" -eq 2 ]; then
        do_package_only
        do_clear
        KERNEL_NAME=""
        read_features
        continue
    fi

    # Full build: feat menu ↔ build menu inner loop
    # [B] in feat menu  → exit inner loop → back to mode select
    # [B] in build menu → loop back to feat menu (NOT mode select)
    # [S] in build menu → build runs, KERNEL_NAME resets, return to mode select
    _do_build=false
    while true; do
        run_feat_menu || break          # [B] in feat → exit inner loop
        run_build_menu && { _do_build=true; break; }
        # [B] in build → loop back to feat menu
    done
    if ! $_do_build; then
        do_clear; read_features; continue
    fi
    do_clear
    KERNEL_NAME=""
    read_features
done
