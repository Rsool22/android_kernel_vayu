#!/bin/bash
# =============================================================================
#  VAYU KERNEL BUILDER — ReSukiSU + SUSFS + KPM — AnyMore Project
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

# ── Log / zip naming ─────────────────────────────────────────────────────────
# Two fixed-name logs in OUTPUT_DIR (never accumulate):
#   LAST-SUCCESS-{#N}.log  — log of the last successful build (renamed on commit)
#   LAST-FAIL.log          — log of the last failed/cancelled make (always same name)
# One zip kept: previous successful zip is deleted when a new one is produced.
LOG_FAIL_NAME="[VAYU-Anymore]-LAST-FAIL.log"

# Compute the success log name for a given build number
_success_log_name() { echo "[VAYU-Anymore]-LAST-SUCCESS-{#${1}}.log"; }

# Delete any previous success log (pattern: LAST-SUCCESS-*)
_rm_old_success_log() {
    find "$OUTPUT_DIR" -maxdepth 1 -name "[VAYU-Anymore]-LAST-SUCCESS-*.log" -delete 2>/dev/null || true
}

# Delete any previous zip (pattern: [VAYU-Anymore]-*.zip)
_rm_old_zip() {
    find "$OUTPUT_DIR" -maxdepth 1 -name "[VAYU-Anymore]-*.zip" -delete 2>/dev/null || true
}

# ── Build counter ────────────────────────────────────────────────────────────
# BUILD_NUM is only written to disk on a successful zip.
# Failed or aborted builds do not consume a number.
BUILD_NUM_FILE="${HOME}/kernel-builds/.build_number"

_read_build_num() {
    if [ -f "$BUILD_NUM_FILE" ]; then
        BUILD_NUM=$(( $(cat "$BUILD_NUM_FILE") + 1 ))
    else
        BUILD_NUM=1
    fi
}

_commit_build_num() {
    echo "$BUILD_NUM" > "$BUILD_NUM_FILE"
}

reset_build_num() {
    echo "0" > "$BUILD_NUM_FILE"
    _read_build_num
}

_read_build_num

# ── Environment ──────────────────────────────────────────────────────────────
export ARCH="arm64"
export KBUILD_BUILD_USER="OmegaR01"
export KBUILD_BUILD_HOST="Vayu"
export PATH="${CLANG_DIR}/bin:${GCC64_DIR}:${GCC32_DIR}:${PATH}"
CCACHE_BIN=$(command -v ccache 2>/dev/null)
INCREMENTAL=false

# ── Colors ───────────────────────────────────────────────────────────────────
NC='\033[0m'
LRD='\033[1;31m'; RED='\033[0;31m'
LGR='\033[1;32m'; YEL='\033[1;33m'
CYN='\033[1;36m'; MAG='\033[1;35m'
BLU='\033[1;34m'; WHT='\033[1;37m'
GRY='\033[0;37m'

# ── Sanity checks ────────────────────────────────────────────────────────────
[ ! -d "$CLANG_DIR" ] && printf "${LRD}ERR: Clang not found: %s${NC}\n" "$CLANG_DIR" && exit 1
[ ! -d "$anykernel"  ] && printf "${LRD}ERR: AnyKernel3 not found: %s${NC}\n" "$anykernel" && exit 1

# =============================================================================
# TERMINAL HELPERS
# do_clear      — erase visible screen + cursor home
# set_width     — read current terminal width into W
# WINCH is armed only while a menu is on screen (_winch_enable/_winch_disable).
# During build output WINCH is fully removed so no menu escape codes can fire
# into the scrolling compile log (Termius-safe — no alternate screen).
# =============================================================================
W=80

do_clear() {
    printf '\033[?25l'
    printf '\033[2J\033[H'
    printf '\033[?25h'
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

# Arm WINCH redraws — call at the top of every menu draw function.
_winch_enable()  { trap '_handle_winch' WINCH; }
# Disarm WINCH completely — call before streaming any build output.
_winch_disable() { trap - WINCH; _MENU_REDRAW_FN=""; }

# On any exit: restore echo, cursor, and clear the screen so the user's
# shell prompt appears on a clean terminal (not buried under build output).
_term_cleanup() {
    stty echo 2>/dev/null
    printf '\033[?25h'
    printf '\033[2J\033[H'
}
trap '_term_cleanup' EXIT

_hbar() { printf '═%.0s' $(seq 1 "$1"); }
_tbar() { local _i _o=''; for _i in $(seq 1 "$1"); do _o="${_o}-"; done; printf '%s' "$_o"; }

box_top() { printf "${1}╔$(_hbar $((W-2)))╗${NC}\n"; }
box_bot() { printf "${1}╚$(_hbar $((W-2)))╝${NC}\n"; }
box_div() { printf "${1}╠$(_hbar $((W-2)))╣${NC}\n"; }

# Centered plain-text row — truncates if wider than inner width
box_ctr() {
    local bc=$1 tc=$2 text=$3
    local inner=$((W-2))
    if [ ${#text} -gt $inner ]; then text="${text:0:$(( inner-1 ))}…"; fi
    local len=${#text}
    local lp=$(( (inner-len)/2 )) rp=$(( inner-len-(inner-len)/2 ))
    [ $lp -lt 0 ] && lp=0; [ $rp -lt 0 ] && rp=0
    printf "${bc}║${tc}%${lp}s%s%${rp}s${bc}║${NC}\n" '' "$text" ''
}

# Left-aligned plain-text row — truncates if wider than inner width
box_row() {
    local bc=$1 tc=$2 text=$3
    local inner=$(( W-2 ))
    if [ ${#text} -gt $inner ]; then text="${text:0:$(( inner-1 ))}…"; fi
    local pad=$(( inner-${#text} ))
    [ $pad -lt 0 ] && pad=0
    printf "${bc}║${tc}%s%${pad}s${bc}║${NC}\n" "$text" ''
}

# Key (left) + Value (right) — truncates key if combined length overflows
box_kv() {
    local bc=$1 kc=$2 vc=$3 key=$4 val=$5
    local inner=$(( W-2 ))
    if [ $(( ${#key}+${#val}+1 )) -gt $inner ]; then
        local maxkey=$(( inner-${#val}-2 ))
        [ $maxkey -lt 1 ] && maxkey=1
        [ ${#key} -gt $maxkey ] && key="${key:0:$(( maxkey-1 ))}…"
    fi
    local gap=$(( inner-${#key}-${#val} ))
    [ $gap -lt 1 ] && gap=1
    printf "${bc}║${kc}%s%${gap}s${vc}%s${bc}║${NC}\n" "$key" '' "$val"
}

log_sep() {
    local label="${1:-}"
    if [ -n "$label" ]; then
        local rest=$(( W-${#label}-4 ))
        [ $rest -lt 1 ] && rest=1
        printf "${CYN}-- %s $(_tbar $rest)${NC}\n" "$label"
    else
        printf "${CYN}$(_tbar $W)${NC}\n"
    fi
}

# =============================================================================
# FEATURE STATE
# Dependency rules:
#   SUSFS requires ReSukiSU (CONFIG_KSU_SUSFS depends on CONFIG_KSU)
#   KPM    requires ReSukiSU
# When ReSukiSU is disabled, SUSFS and KPM are force-disabled with it.
# =============================================================================
FEAT_KSU=false; FEAT_SUSFS=false; FEAT_KPM=false

read_features() {
    FEAT_KSU=false; FEAT_SUSFS=false; FEAT_KPM=false
    grep -q "^CONFIG_KSU=y"              "$DEFCONFIG_PATH" 2>/dev/null && FEAT_KSU=true
    grep -q "^CONFIG_KSU_SUSFS=y"        "$DEFCONFIG_PATH" 2>/dev/null && FEAT_SUSFS=true
    grep -q "^CONFIG_KPM=y"              "$DEFCONFIG_PATH" 2>/dev/null && FEAT_KPM=true
    # MANUAL_HOOK is derived: active when KSU=on and SUSFS=off
    # (mirrors Kconfig rule: depends on !KSU_SUSFS)
}

toggle_config() {
    local cfg=$1 en=$2
    sed -i "/^# ${cfg} is not set/d; /^${cfg}=/d" "$DEFCONFIG_PATH"
    if [ "$en" = true ]; then echo "${cfg}=y"            >> "$DEFCONFIG_PATH"
    else                       echo "# ${cfg} is not set" >> "$DEFCONFIG_PATH"
    fi
}

feat_str() {
    local s=""
    if $FEAT_KSU; then
        s="ReSukiSU+"
        if $FEAT_SUSFS; then
            s="${s}InlineHook+SUSFS+"
        else
            s="${s}ManualHook+"
        fi
    fi
    $FEAT_KPM && s="${s}KPM+"
    local out="${s%+}"
    [ -z "$out" ] && out="Vanilla"
    echo "$out"
}

# =============================================================================
# WINCH — recalculate width and redraw active menu on terminal resize
# Armed/disarmed per-phase via _winch_enable / _winch_disable.
# =============================================================================
_MENU_REDRAW_FN=""
_handle_winch() { set_width; [ -n "$_MENU_REDRAW_FN" ] && "$_MENU_REDRAW_FN"; }
# Initial state: no WINCH trap — menus arm it when they draw.

# =============================================================================
# SHARED TITLE  — printed at top of every screen
# =============================================================================
draw_title() {
    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "VAYU  KERNEL  BUILDER"
    box_bot "$MAG"
}

# =============================================================================
# STEP 1 — FEATURE CONFIGURATION
# Full initial draw then partial updates on toggle.
# =============================================================================
FEAT_MSG=""
FEAT_MSG_C="$LGR"

_draw_feat_full() {
    _MENU_REDRAW_FN="_draw_feat_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'    # cursor home on visible screen — no scrollback jump, no flash
    draw_title

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "FEATURE  CONFIGURATION"
    box_div "$CYN"

    # ReSukiSU (always available)
    if $FEAT_KSU; then box_kv "$CYN" "$WHT" "$LGR" "  [1]  ReSukiSU" "* ENABLED   "
    else               box_kv "$CYN" "$GRY" "$GRY" "  [1]  ReSukiSU" "o DISABLED  "; fi

    # SUSFS — dimmed if ReSukiSU off
    if ! $FEAT_KSU;    then box_kv "$CYN" "$GRY" "$GRY" "  [2]  SUSFS" "- N/A       "
    elif $FEAT_SUSFS;  then box_kv "$CYN" "$WHT" "$LGR" "  [2]  SUSFS" "* ENABLED   "
    else                    box_kv "$CYN" "$GRY" "$GRY" "  [2]  SUSFS" "o DISABLED  "; fi

    # KPM — dimmed if ReSukiSU off
    if ! $FEAT_KSU;  then box_kv "$CYN" "$GRY" "$GRY" "  [3]  KPM" "- N/A       "
    elif $FEAT_KPM;  then box_kv "$CYN" "$WHT" "$LGR" "  [3]  KPM" "* ENABLED   "
    else                  box_kv "$CYN" "$GRY" "$GRY" "  [3]  KPM" "o DISABLED  "; fi

    # Manual Hook — derived (auto), not user-toggled
    if $FEAT_KSU && ! $FEAT_SUSFS; then
        box_kv "$CYN" "$WHT" "$LGR" "       Manual Hook" "* AUTO-ON   "
    elif $FEAT_KSU && $FEAT_SUSFS; then
        box_kv "$CYN" "$GRY" "$GRY" "       Manual Hook" "- SUSFS mode "
    else
        box_kv "$CYN" "$GRY" "$GRY" "       Manual Hook" "- N/A        "
    fi

    box_div "$CYN"
    box_row "$CYN" "$WHT" "  [C]  Confirm Features"
    box_row "$CYN" "$MAG" "  [M]  Open Menuconfig"
    box_row "$CYN" "$WHT" "  [Q]  Quit"
    box_bot "$CYN"

    # Message box + prompt (MSG_ROW=15)
    if [ -n "$FEAT_MSG" ]; then
        box_top "$FEAT_MSG_C"
        box_ctr "$FEAT_MSG_C" "$FEAT_MSG_C" "$FEAT_MSG"
        box_bot "$FEAT_MSG_C"
    fi
    printf '\033[J'
    _cursor_show
    printf "\n${WHT}  Select [1/2/3/C/M/Q]: ${NC}"
}

run_feat_menu() {
    FEAT_MSG=""
    _draw_feat_full

    while true; do
        IFS= read -r -s -n1 choice

        # Drain escape sequences
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            FEAT_MSG=""; _draw_feat_full; continue
        fi

        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            1)  if $FEAT_KSU; then
                    # Disabling ReSukiSU — force-disable SUSFS, KPM, and Manual Hook too
                    FEAT_KSU=false;   toggle_config "CONFIG_KSU"              false
                    FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS"        false
                    FEAT_KPM=false;   toggle_config "CONFIG_KPM"              false
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK"  false
                    FEAT_MSG="ReSukiSU disabled  ->  SUSFS + KPM + ManualHook also disabled"; FEAT_MSG_C="$YEL"
                else
                    FEAT_KSU=true; toggle_config "CONFIG_KSU" true
                    # KSU on + SUSFS currently off → auto-enable Manual Hook
                    if ! $FEAT_SUSFS; then toggle_config "CONFIG_KSU_MANUAL_HOOK" true; fi
                    FEAT_MSG="ReSukiSU enabled"; FEAT_MSG_C="$LGR"
                fi
                _draw_feat_full ;;

            2)  if ! $FEAT_KSU; then
                    FEAT_MSG="[!] Enable ReSukiSU first -- SUSFS requires it"; FEAT_MSG_C="$LRD"
                    _draw_feat_full; continue
                fi
                if $FEAT_SUSFS; then FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS" false
                                     toggle_config "CONFIG_KSU_MANUAL_HOOK" true
                                     FEAT_MSG="SUSFS disabled  ->  Manual Hook auto-enabled"; FEAT_MSG_C="$LGR"
                else                 FEAT_SUSFS=true;  toggle_config "CONFIG_KSU_SUSFS" true
                                     toggle_config "CONFIG_KSU_MANUAL_HOOK" false
                                     FEAT_MSG="SUSFS enabled  ->  Manual Hook auto-disabled"; FEAT_MSG_C="$YEL"
                fi
                _draw_feat_full ;;

            3)  if ! $FEAT_KSU; then
                    FEAT_MSG="[!] Enable ReSukiSU first -- KPM requires it"; FEAT_MSG_C="$LRD"
                    _draw_feat_full; continue
                fi
                if $FEAT_KPM; then FEAT_KPM=false; toggle_config "CONFIG_KPM" false
                                   FEAT_MSG="KPM disabled"; FEAT_MSG_C="$GRY"
                else               FEAT_KPM=true;  toggle_config "CONFIG_KPM" true
                                   FEAT_MSG="KPM enabled";  FEAT_MSG_C="$LGR"
                fi
                _draw_feat_full ;;

            c)  return 0 ;;

            m)  do_clear
                make -C "$kernel_dir" O="$objdir" \
                    ARCH=arm64 LLVM=1 LLVM_IAS=1 \
                    CC="${CCACHE_BIN:+ccache }clang" \
                    menuconfig
                read_features
                FEAT_MSG="Menuconfig done -- features refreshed from defconfig"; FEAT_MSG_C="$CYN"
                _draw_feat_full ;;

            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   FEAT_MSG="Unknown option: [${choice}]"; FEAT_MSG_C="$LRD"
                 _draw_feat_full ;;
        esac
    done
}

# =============================================================================
# STEP 2 — BUILD OPTIONS
# Full initial draw then partial update on incremental toggle.
# =============================================================================
BUILD_MSG=""
BUILD_MSG_C="$LGR"

_draw_build_full() {
    _MENU_REDRAW_FN="_draw_build_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'    # cursor home on visible screen — no scrollback jump, no flash
    draw_title

    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "FEATURES"
    box_div "$CYN"
    box_row "$CYN" "$GRY" "  ${fs}"
    box_bot "$CYN"

    box_top "$BLU"
    box_ctr "$BLU" "$YEL" "BUILD  OPTIONS"
    box_div "$BLU"
    box_row "$BLU" "$WHT" "  [S]  Start Build"
    if $INCREMENTAL; then box_kv "$BLU" "$WHT" "$YEL" "  [I]  Incremental Build" "* ON        "
    else                  box_kv "$BLU" "$GRY" "$GRY" "  [I]  Incremental Build" "o OFF       "; fi
    box_div "$BLU"
    box_row "$BLU" "$WHT" "  [B]  Back to Features"
    box_row "$BLU" "$LRD" "  [X]  Reset Build Counter  (#${BUILD_NUM} -> #1)"
    box_row "$BLU" "$WHT" "  [Q]  Quit"
    box_bot "$BLU"

    if [ -n "$BUILD_MSG" ]; then
        box_top "$BUILD_MSG_C"
        box_ctr "$BUILD_MSG_C" "$BUILD_MSG_C" "$BUILD_MSG"
        box_bot "$BUILD_MSG_C"
    fi
    printf '\033[J'
    _cursor_show
    printf "\n${WHT}  Select [S/I/B/X/Q]: ${NC}"
}

run_build_menu() {
    BUILD_MSG=""
    _draw_build_full

    while true; do
        IFS= read -r -s -n1 choice

        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            BUILD_MSG=""; _draw_build_full; continue
        fi

        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            i)  if $INCREMENTAL; then INCREMENTAL=false
                                      BUILD_MSG="Incremental OFF -- full clean build"; BUILD_MSG_C="$GRY"
                else                  INCREMENTAL=true
                                      BUILD_MSG="Incremental ON -- keeps previous objects"; BUILD_MSG_C="$YEL"
                fi
                _draw_build_full ;;
            s)  do_build; return 0 ;;
            x)  reset_build_num
                BUILD_MSG="Build counter reset -- next successful build will be #1"; BUILD_MSG_C="$LRD"
                _draw_build_full ;;   # full redraw so the counter in [X] row updates
            b)  return 1 ;;
            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   BUILD_MSG="Unknown option: [${choice}]"; BUILD_MSG_C="$LRD"
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
    box_top  "$YEL"
    box_ctr  "$YEL" "$YEL" "BUILD  SUMMARY"
    box_div  "$YEL"
    box_row  "$YEL" "$WHT" "  Kernel   : vayu_a16_kernel"
    box_row  "$YEL" "$WHT" "  Build #  : #${BUILD_NUM}"
    box_row  "$YEL" "$WHT" "  Features : ${fs}"
    box_row  "$YEL" "$WHT" "  Mode     : ${mode}"
    box_row  "$YEL" "$WHT" "  Output   : ${OUTPUT_DIR}"
    box_bot  "$YEL"
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

# =============================================================================
# RESULT BOXES  — art above, no redundant title row in box
# =============================================================================
print_success_box() {
    local elapsed=$1 zippath=${2:-""}
    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    printf "\n"
    print_success_art
    box_top "$LGR"
    box_row "$LGR" "$WHT" "  Build #  : #${BUILD_NUM}"
    box_row "$LGR" "$WHT" "  Features : ${fs}"
    box_row "$LGR" "$WHT" "  Time     : ${elapsed}"
    box_row "$LGR" "$WHT" "  Zip      : ${zippath##*/}"
    box_row "$LGR" "$WHT" "  Output   : ${OUTPUT_DIR}"
    box_bot "$LGR"
}

print_fail_box() {
    local logfile=$1
    printf "\n"
    print_failed_art
    box_top "$LRD"
    box_ctr "$LRD" "$WHT" "Compiler returned a non-zero exit code."
    box_div "$LRD"
    box_ctr "$LRD" "$YEL" "COMPILER ERRORS"
    box_div "$LRD"
    if [ -f "$logfile" ]; then
        local maxw=$(( W-6 ))
        grep -E "error:" "$logfile" 2>/dev/null | head -5 | while IFS= read -r line; do
            [ ${#line} -gt $maxw ] && line="${line:0:$(( maxw-1 ))}…"
            box_row "$LRD" "$RED" "  ${line}"
        done
    fi
    box_div "$LRD"
    box_row "$LRD" "$GRY" "  Log: ${logfile##*/}"
    box_bot "$LRD"
}

# =============================================================================
# POST-BUILD PROMPT — printed inline after build output, no screen clear.
# WINCH stays disabled until the next menu draw re-enables it.
# =============================================================================
post_build_prompt() {
    printf "\n"
    box_top "$WHT"
    box_row "$WHT" "$WHT" "  [R]  Return to menu"
    box_row "$WHT" "$WHT" "  [E]  Exit script"
    box_bot "$WHT"
    printf "\n${WHT}  Select [R/E]: ${NC}"

    while true; do
        IFS= read -r -s -n1 key
        if [ "$key" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null; continue
        fi
        key=$(printf '%s' "$key" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        case "$key" in
            r)  return 0 ;;
            e)  printf "\n${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
        esac
    done
}

# =============================================================================
# SIGINT HANDLER FOR COMPILE STAGE
# Must be top-level — nested function definitions break on bash 4.x.
# _MAKE_PID and _CANCELLED are globals set in do_build before the trap.
# =============================================================================
_MAKE_PID=""
_CANCELLED=false
_build_sigint() {
    _CANCELLED=true
    if [ -n "$_MAKE_PID" ]; then
        # Kill the entire setsid process group (pgid == make's pid) so all
        # spawned compilers are terminated, not just the top-level make.
        kill -TERM -- "-$_MAKE_PID" 2>/dev/null
        kill -TERM "$_MAKE_PID"     2>/dev/null  # fallback if setsid unavailable
    fi
}

# =============================================================================
# DO-PACKAGE — copy images into AnyKernel3 and zip
# Called from do_build (after compile) and from do_package_only.
# Sets ZIP_EXIT and zip (filename) in caller's scope via globals.
# =============================================================================
_LAST_ZIP=""   # set to the produced zip path on success

do_package() {
    local fs=$1 elapsed=${2:-"N/A"}
    local dtag; dtag=$(date '+%Y-%m-%d')

    printf "\n"; log_sep "PACKAGE"; printf "\n"
    box_top "$MAG"; box_ctr "$MAG" "$MAG" "Packaging AnyKernel3 zip..."; box_bot "$MAG"
    printf "\n"

    cp "${objdir}/arch/arm64/boot/Image"    "${anykernel}/Image"
    cp "${objdir}/arch/arm64/boot/dtbo.img" "${anykernel}/dtbo.img" 2>/dev/null || true

    # Compute next build number (preview — not committed yet)
    local zipnum="$BUILD_NUM"
    local zipname="[VAYU-Anymore]-(${fs})-[${dtag}]-{#${zipnum}}.zip"
    local zippath="${OUTPUT_DIR}/${zipname}"

    local _zip_rc_file; _zip_rc_file=$(mktemp /tmp/vkb_zip_XXXXXX)
    (
        cd "$anykernel" && \
        zip -r9 "$zippath" . -x '*.git*' 2>&1 | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        echo "${PIPESTATUS[0]}" > "$_zip_rc_file"
    )
    local ZIP_EXIT; ZIP_EXIT=$(cat "$_zip_rc_file" 2>/dev/null); rm -f "$_zip_rc_file"
    ZIP_EXIT=$(( ${ZIP_EXIT:-1} + 0 ))

    printf "\n"; log_sep

    if [ "$ZIP_EXIT" -eq 0 ] && [ -f "$zippath" ]; then
        _LAST_ZIP="$zippath"
        return 0
    else
        _LAST_ZIP=""
        return 1
    fi
}

# =============================================================================
# UPDATE RESUKISU DRIVER
# Pulls latest from ReSukiSU/ReSukiSU main via official setup.sh.
# Verifies drivers/kernelsu/ source tree afterwards.
# =============================================================================
do_update_resukisu() {
    _winch_disable
    set_width
    do_clear

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "UPDATE  ReSukiSU  DRIVER"
    box_div "$CYN"
    box_row "$CYN" "$WHT" "  Source : github.com/ReSukiSU/ReSukiSU"
    box_row "$CYN" "$WHT" "  Branch : main"
    box_row "$CYN" "$WHT" "  Target : ${kernel_dir}/drivers/kernelsu"
    box_bot "$CYN"
    printf "\n"

    log_sep "RUNNING SETUP.SH"
    printf "\n"

    local setup_url="https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh"
    local _rc_file; _rc_file=$(mktemp /tmp/vkb_upd_XXXXXX)

    (
        cd "$kernel_dir" || exit 1
        curl -LSs "$setup_url" | bash
        echo "${PIPESTATUS[0]}" > "$_rc_file"
    ) 2>&1 | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done

    local UPDATE_EXIT; UPDATE_EXIT=$(cat "$_rc_file" 2>/dev/null); rm -f "$_rc_file"
    UPDATE_EXIT=$(( ${UPDATE_EXIT:-1} + 0 ))

    printf "\n"; log_sep "VERIFY"; printf "\n"

    # Verify source tree — new layout: drivers/kernelsu/ with subdirs
    local ksu_c_count
    ksu_c_count=$(find "${kernel_dir}/drivers/kernelsu" -name "*.c" 2>/dev/null | wc -l)
    local kbuild_ok=false
    [ -f "${kernel_dir}/drivers/kernelsu/Kbuild" ] && kbuild_ok=true

    if [ "$UPDATE_EXIT" -eq 0 ] && [ "$ksu_c_count" -gt 5 ] && $kbuild_ok; then
        box_top "$LGR"
        box_ctr "$LGR" "$LGR" "ReSukiSU driver updated successfully"
        box_div "$LGR"
        box_row "$LGR" "$WHT" "  Source files : ${ksu_c_count} .c files found"
        box_row "$LGR" "$WHT" "  Kbuild       : present"
        box_bot "$LGR"
    else
        box_top "$LRD"
        box_ctr "$LRD" "$LRD" "Update failed or source tree incomplete"
        box_div "$LRD"
        box_row "$LRD" "$WHT" "  .c files found : ${ksu_c_count}  (expect > 5)"
        box_row "$LRD" "$WHT" "  Kbuild present : $( $kbuild_ok && echo YES || echo NO )"
        box_row "$LRD" "$YEL" "  Check curl/network and retry"
        box_bot "$LRD"
    fi

    printf "\n"
    box_top "$WHT"
    box_row "$WHT" "$WHT" "  [R]  Return to menu"
    box_bot "$WHT"
    printf "\n${WHT}  Press R to return: ${NC}"

    while true; do
        IFS= read -r -s -n1 key
        [ "$key" = $'\033' ] && { IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null; continue; }
        key=$(printf '%s' "$key" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        [ "$key" = "r" ] && break
        [ "$key" = "" ] && continue
    done
}

# =============================================================================
# BUILD
# =============================================================================
do_build() {
    # Disable WINCH before any output — no menu redraws during build scroll.
    _winch_disable
    set_width
    do_clear

    draw_summary

    # ── Stage 1: Clean / Incremental ─────────────────────────────────────────
    printf "\n"; log_sep "STAGE 1 -- CLEAN"; printf "\n"
    if $INCREMENTAL; then
        box_top "$GRY"; box_ctr "$GRY" "$GRY" "Incremental -- keeping previous objects"; box_bot "$GRY"
        printf "\n"
    else
        box_top "$CYN"; box_ctr "$CYN" "$CYN" "Cleaning previous build..."; box_bot "$CYN"
        printf "\n"
        make -C "$kernel_dir" O="$objdir" clean    2>&1 | grep -v "^-- " | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        make -C "$kernel_dir" O="$objdir" mrproper 2>&1 | grep -v "^-- " | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        printf "\n"
    fi

    # ── Stage 2: Feature config ───────────────────────────────────────────────
    # New ReSukiSU Kconfig: KSU_MANUAL_HOOK depends on !KSU_SUSFS — mutually exclusive.
    # SUSFS mode  → LSM hooks only, all MANUAL_HOOK configs must be off.
    # Manual mode → inline fs/ hooks active; AUTO_SETUID_HOOK disabled because
    #               kernel/sys.c already calls ksu_handle_setresuid directly.
    toggle_config "CONFIG_KSU"       "$FEAT_KSU"
    toggle_config "CONFIG_KSU_SUSFS" "$FEAT_SUSFS"
    toggle_config "CONFIG_KPM"       "$FEAT_KPM"
    if $FEAT_KSU && ! $FEAT_SUSFS; then
        toggle_config "CONFIG_KSU_MANUAL_HOOK"                      true
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INPUT_HOOK"      true
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_SETUID_HOOK"     false  # sys.c has manual hook
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INITRC_HOOK"     true
    else
        toggle_config "CONFIG_KSU_MANUAL_HOOK"                      false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INPUT_HOOK"      false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_SETUID_HOOK"     false
        toggle_config "CONFIG_KSU_MANUAL_HOOK_AUTO_INITRC_HOOK"     false
    fi

    # ── Stage 3: Defconfig ────────────────────────────────────────────────────
    printf "\n"; log_sep "STAGE 2 -- DEFCONFIG"; printf "\n"
    box_top "$CYN"
    box_ctr "$CYN" "$WHT" "Applying: arch/arm64/configs/${CONFIG_FILE}"
    box_bot "$CYN"
    printf "\n"
    make -C "$kernel_dir" O="$objdir" \
        ARCH=arm64 LLVM=1 LLVM_IAS=1 \
        CC="${CCACHE_BIN:+ccache }clang" \
        "$CONFIG_FILE" 2>&1 | grep -v "^-- " | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
    printf "\n"

    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    mkdir -p "$OUTPUT_DIR"
    local fail_log="${OUTPUT_DIR}/${LOG_FAIL_NAME}"

    # ── Stage 4: Compile ──────────────────────────────────────────────────────
    printf "\n"; log_sep "STAGE 3 -- COMPILE"; printf "\n"
    box_top "$CYN"; box_ctr "$CYN" "$CYN" "COMPILING  KERNEL"; box_bot "$CYN"
    printf "\n"

    local t0; t0=$(date +%s)

    # Pipe make output through tee into the fail log (overwritten each run).
    # On success, this temp log is renamed to the success log.
    # On failure, it stays as LAST-FAIL.log.
    local _fifo; _fifo=$(mktemp -u /tmp/vkb_XXXXXX)
    mkfifo "$_fifo"
    tee "$fail_log" < "$_fifo" &
    local _TEE_PID=$!

    _MAKE_PID=""; _CANCELLED=false
    trap '_build_sigint' INT

    setsid make -C "$kernel_dir" O="$objdir" \
        ARCH=arm64 \
        LLVM=1 LLVM_IAS=1 \
        CC="${CCACHE_BIN:+ccache }clang" \
        CLANG_TRIPLE=aarch64-linux-gnu- \
        CROSS_COMPILE=aarch64-linux-gnu- \
        CROSS_COMPILE_ARM32=arm-linux-gnueabi- \
        -j"$(nproc --all)" \
        > "$_fifo" 2>&1 &
    _MAKE_PID=$!
    wait "$_MAKE_PID"
    BUILD_EXIT=$?
    rm -f "$_fifo"
    wait "$_TEE_PID" 2>/dev/null

    trap - INT

    local t1; t1=$(date +%s)
    local s=$(( t1 - t0 ))
    local elapsed
    [ "$s" -ge 60 ] && elapsed="$(( s/60 ))m $(( s%60 ))s" || elapsed="${s}s"

    if $_CANCELLED; then
        printf "\n\n!!! BUILD CANCELLED (Ctrl+C) elapsed: %s !!!\n" "$elapsed" >> "$fail_log"
        printf "\n"; log_sep
        box_top "$YEL"; box_ctr "$YEL" "$YEL" "Build cancelled -- see LAST-FAIL.log"; box_bot "$YEL"
        printf "\n"
        post_build_prompt
        return
    fi

    if [ "$BUILD_EXIT" -ne 0 ] || [ ! -f "${objdir}/arch/arm64/boot/Image" ]; then
        # fail_log already written — leave it as LAST-FAIL.log
        printf "\n"; log_sep
        print_fail_box "$fail_log"
        post_build_prompt
        return
    fi

    # ── Stage 5: Package ──────────────────────────────────────────────────────
    if do_package "$fs" "$elapsed"; then
        # Commit build number, rotate logs and zips
        _commit_build_num
        _rm_old_success_log
        local slog_name; slog_name=$(_success_log_name "$BUILD_NUM")
        cp "$fail_log" "${OUTPUT_DIR}/${slog_name}"  # save success log copy
        rm -f "$fail_log"                             # no failure this run
        _rm_old_zip  # remove previous zip before counting new one
        # Move the new zip into place (it was already written by do_package)
        print_success_box "$elapsed" "$_LAST_ZIP"
    else
        print_fail_box "$fail_log"
    fi

    post_build_prompt
}

# =============================================================================
# PACKAGE-ONLY — zip existing images without recompiling
# =============================================================================
do_package_only() {
    _winch_disable
    set_width
    do_clear

    local fs; fs=$(feat_str); [ -z "$fs" ] && fs="None"
    mkdir -p "$OUTPUT_DIR"

    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "PACKAGE  EXISTING  IMAGES"
    box_div "$MAG"
    box_row "$MAG" "$WHT" "  Image : ${objdir}/arch/arm64/boot/Image"
    box_row "$MAG" "$WHT" "  Feat  : ${fs}"
    box_bot "$MAG"
    printf "\n"

    if do_package "$fs"; then
        _commit_build_num
        _rm_old_success_log
        local slog_name; slog_name=$(_success_log_name "$BUILD_NUM")
        # No compile log to copy for package-only — create a brief note
        printf "[VAYU-Anymore] Package-only build #%s  %s\n" "$BUILD_NUM" "$(date)" > "${OUTPUT_DIR}/${slog_name}"
        _rm_old_zip
        print_success_box "N/A" "$_LAST_ZIP"
    else
        box_top "$LRD"; box_ctr "$LRD" "$LRD" "Packaging failed — check zip tool"; box_bot "$LRD"
    fi

    post_build_prompt
}

# =============================================================================
# STARTUP MODE MENU
# Three options at launch:
#   [B] Full build  — configure features → build options → make → package
#   [P] Package only — zip existing Image/dtbo without recompiling
#   [Q] Quit
# If no Image exists in objdir, [P] is hidden and auto-falls through to [B].
# =============================================================================
_draw_mode_menu() {
    _MENU_REDRAW_FN="_draw_mode_menu"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title

    local has_image=false
    [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "SELECT  MODE"
    box_div "$CYN"
    box_row "$CYN" "$WHT" "  [B]  Full Build  (clean + compile + package)"
    if $has_image; then
        box_row "$CYN" "$LGR" "  [P]  Package Existing Images  (skip compile)"
    else
        box_row "$CYN" "$GRY" "       Package Only — no Image found in out/"
    fi
    box_div "$CYN"
    box_row "$CYN" "$CYN" "  [U]  Update ReSukiSU Driver"
    box_div "$CYN"
    box_row "$CYN" "$WHT" "  [Q]  Quit"
    box_bot "$CYN"

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
            _draw_mode_menu
            has_image=false; [ -f "${objdir}/arch/arm64/boot/Image" ] && has_image=true
            continue
        fi
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        case "$choice" in
            b)  return 0 ;;   # full build flow
            p)  $has_image && return 2 ;;   # package only (only if image exists)
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
        # Package-only path
        do_package_only
        do_clear
        read_features
        continue
    fi

    # Full build path
    run_feat_menu
    run_build_menu || continue
    do_clear
    read_features
done
