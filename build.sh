#!/bin/bash
# =============================================================================
#  VAYU KERNEL BUILDER  ─  ReSukiSU + SuSFS + KPM  ─  AnyMore Project
#  Linux 4.14 NonGKI  │  Xiaomi Poco X3 Pro (vayu)  │  Android 16
# =============================================================================

# ── Paths ────────────────────────────────────────────────────────────────────
kernel_dir="${HOME}/kernel-builds/vayu_a16_kernel"
objdir="${kernel_dir}/out"
anykernel="${kernel_dir}/AnyKernel3"
CLANG_DIR="${kernel_dir}/clang"
GCC64_DIR="/usr/bin"
GCC32_DIR="/usr/bin"
CONFIG_FILE="vayu_defconfig"
OUTPUT_DIR="${kernel_dir}/Anykernel-Builds"
DEFCONFIG_PATH="${kernel_dir}/arch/arm64/configs/${CONFIG_FILE}"

# ── Persistent state files ───────────────────────────────────────────────────
STATE_FILE="${kernel_dir}/.builder_state"
PREV_STATE_FILE="${kernel_dir}/.builder_prev_state"
MENUCONFIG_PRESERVE_FILE="${kernel_dir}/.menuconfig_saved_config"

# ── Session-only overrides ───────────────────────────────────────────────────
KERNEL_NAME=""

# ── Naming ───────────────────────────────────────────────────────────────────
PROJECT_NAME="VAYU-AnyMore"

_build_log_base() {
    local bdate; bdate=$(date +%Y-%m-%d)
    local name="[${PROJECT_NAME}-Project]"
    [ -n "$KERNEL_NAME" ] && name="${name}-[Kernel-Name=${KERNEL_NAME}]"
    if $FEAT_KSU; then
        local btag; [ "$KSU_BRANCH" = "dev" ] && btag="DEV" || btag="MAIN"
        local hook; $FEAT_SUSFS && hook="SuSFS-Inline-Hook" || hook="Manual-Hook"
        name="${name}-[${btag}-ReSukiSU=${hook}]"
        local extras=""
        $FEAT_SUSFS && extras="SuSFS"
        $FEAT_KPM   && extras="${extras:+${extras}+}KPM"
        [ -n "$extras" ] && name="${name}-(+${extras})"
    fi
    echo "${name}-(${bdate})"
}

_success_log_name() { echo "$(_build_log_base)-Build-#${1}.log"; }
_fail_log_name()    { echo "[${PROJECT_NAME}-Project]-FAIL.log"; }

_rm_old_success_log() {
    find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-Build-#*.log" -delete 2>/dev/null || true
}

_rm_old_zip() {
    local keep="${1:-}"
    find "$OUTPUT_DIR" -maxdepth 1 -name "*.zip" | while IFS= read -r f; do
        [ "$f" != "$keep" ] && rm -f "$f"
    done
}

# ── Build counter ────────────────────────────────────────────────────────────
BUILD_NUM_FILE="${kernel_dir}/.build_number"
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
    echo "0" > "${objdir}/.version" 2>/dev/null || true
    _read_build_num
}
_read_build_num

# ── State Variables ───────────────────────────────────────────────────────────
KSU_BRANCH="main"
TOAST_MSG=""
TOAST_MSG_C=""
TOAST_SUBMSG=""
_LAST_PULL_STATUS=0
MENUCONFIG_USED=false
_SKIP_DEFCONFIG=false
_PRESERVE_ACTIVE=false
GUARD_TOAST=""
[ -f "$MENUCONFIG_PRESERVE_FILE" ] && _PRESERVE_ACTIVE=true

# ── Persistent state: save/load ──────────────────────────────────────────────
_save_state() { 
    {
        printf "INCREMENTAL=%s\n" "$INCREMENTAL"
        printf "USE_CCACHE=%s\n" "$USE_CCACHE"
        printf "KSU_BRANCH=\"%s\"\n" "$KSU_BRANCH"
        printf "FORCE_CLEAN_REASON=\"%s\"\n" "$FORCE_CLEAN_REASON"
    } > "$STATE_FILE"
}

_load_state() {
    INCREMENTAL=false
    KSU_BRANCH="main"
    FORCE_CLEAN_REASON=""
    [ -f "$STATE_FILE" ] && source "$STATE_FILE" 2>/dev/null || true
    [ -z "$CCACHE_BIN" ] && USE_CCACHE=false
    [ "$KSU_BRANCH" != "main" ] && [ "$KSU_BRANCH" != "dev" ] && KSU_BRANCH="main"
    _update_make_cc
}

_save_prev_state() {
    local bmode; $INCREMENTAL && bmode="Incremental" || bmode="Full Clean"
    {
        printf "PREV_CAP='%s'\n"  "$(get_cap_str)"
        printf "PREV_EXT_FEAT='%s'\n" "$(get_ext_feat_str)"
        printf "PREV_MODE='%s'\n"  "$bmode"
        printf "PREV_NUM='%s'\n"   "$BUILD_NUM"
        printf "PREV_KNAME='%s'\n" "$KERNEL_NAME"
        printf "PREV_DATE='%s'\n"  "$(date '+%Y-%m-%d %H:%M')"
    } > "$PREV_STATE_FILE"
}

_load_prev_state() {
    PREV_CAP=""; PREV_EXT_FEAT=""; PREV_MODE=""; PREV_NUM=""; PREV_KNAME=""; PREV_DATE=""
    [ -f "$PREV_STATE_FILE" ] && source "$PREV_STATE_FILE" 2>/dev/null || true
}

# ── Environment ──────────────────────────────────────────────────────────────
export ARCH="arm64"
export KBUILD_BUILD_USER="OmegaR01"
export KBUILD_BUILD_HOST="Vayu"
export PATH="${CLANG_DIR}/bin:${GCC64_DIR}:${GCC32_DIR}:${PATH}"
CCACHE_BIN=$(command -v ccache 2>/dev/null)

USE_CCACHE=true
[ -z "$CCACHE_BIN" ] && USE_CCACHE=false
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

_load_state
[ ! -d "$CLANG_DIR" ] && printf "${LRD}ERR: Clang not found: %s${NC}\n" "$CLANG_DIR" && exit 1
[ ! -d "$anykernel"  ] && printf "${LRD}ERR: AnyKernel3 not found: %s${NC}\n" "$anykernel" && exit 1

# =============================================================================
# TERMINAL HELPERS & GLOBAL BOX PADDING
# =============================================================================
W=80
PAD_STR="   "
PAD_W=3

do_clear() { printf '\033[?25l\033[2J\033[H\033[?25h'; }
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
_drain_input() { local _junk; while IFS= read -r -s -t 0.05 -n256 _junk 2>/dev/null && [ -n "$_junk" ]; do :; done; }
_term_cleanup() { stty echo 2>/dev/null; printf '\033[?25h\033[2J\033[H'; }
trap '_term_cleanup' EXIT

_hbar() { printf '═%.0s' $(seq 1 "$1"); }
_tbar() { local _i _o=''; for _i in $(seq 1 "$1"); do _o="${_o}─"; done; printf '%s' "$_o"; }

box_top() { printf "${1}╔$(_hbar $((W-2)))╗${NC}\n"; }
box_bot() { printf "${1}╚$(_hbar $((W-2)))╝${NC}\n"; }
box_div() { printf "${1}╠$(_hbar $((W-2)))╣${NC}\n"; }

box_ctr() {
    local bc=$1 tc=$2 text=$3
    while [[ "$text" == " "* ]]; do text="${text# }"; done
    while [[ "$text" == *" " ]]; do text="${text% }"; done
    local inner=$((W-2))
    local max_text=$(( inner - 4 ))
    [ ${#text} -gt $max_text ] && text="${text:0:$(( max_text - 1 ))}…"
    local len=${#text}
    local lp=$(( (inner-len)/2 )) 
    local rp=$(( inner - len - lp ))
    printf "${bc}║${tc}%*s%s%*s${bc}║${NC}\n" "$lp" "" "$text" "$rp" ""
}

box_row() {
    local bc=$1 tc=$2 text=$3
    while [[ "$text" == " "* ]]; do text="${text# }"; done
    local inner=$(( W - 2 ))
    text="  ${text}"
    local max_text=$(( inner - 2 ))
    [ ${#text} -gt $max_text ] && text="${text:0:$(( max_text - 1 ))}…"
    local right_pad=$(( inner - ${#text} ))
    printf "${bc}║${tc}%s%*s${bc}║${NC}\n" "$text" "$right_pad" ""
}

box_lbl() {
    local bc=$1 tc=$2 lbl=$3 val=$4 vc=${5:-$2}
    local LBL_W=14
    local inner=$(( W - 2 ))
    
    local formatted_lbl
    printf -v formatted_lbl "%-*s" "$LBL_W" "$lbl"
    local left_part="${PAD_STR}${formatted_lbl} : "
    local left_len=${#left_part}
    local val_max=$(( inner - left_len - PAD_W )) 

    if [ ${#val} -le $val_max ]; then
        local right_pad=$(( inner - left_len - ${#val} ))
        printf "${bc}║${tc}%s${vc}%s%*s${bc}║${NC}\n" "$left_part" "$val" "$right_pad" ""
        return
    fi

    local rem="$val"
    local first_line=true
    local cont_pad
    printf -v cont_pad "%*s" "$left_len" ""

    while [ ${#rem} -gt 0 ]; do
        if [ ${#rem} -le $val_max ]; then
            local right_pad=$(( inner - left_len - ${#rem} ))
            if $first_line; then
                printf "${bc}║${tc}%s${vc}%s%*s${bc}║${NC}\n" "$left_part" "$rem" "$right_pad" ""
            else
                printf "${bc}║${tc}%s${vc}%s%*s${bc}║${NC}\n" "$cont_pad" "$rem" "$right_pad" ""
            fi
            break
        fi
        local break_idx=$val_max
        local i=$val_max
        while [ $i -gt $(( val_max / 2 )) ]; do
            local ch="${rem:$i:1}"
            if [[ "$ch" == " " || "$ch" == "-" || "$ch" == "]" || "$ch" == "}" ]]; then
                break_idx=$(( i + 1 ))
                break
            fi
            i=$(( i - 1 ))
        done
        local chunk="${rem:0:$break_idx}"
        rem="${rem:$break_idx}"
        local right_pad=$(( inner - left_len - ${#chunk} ))
        if $first_line; then
            printf "${bc}║${tc}%s${vc}%s%*s${bc}║${NC}\n" "$left_part" "$chunk" "$right_pad" ""
            first_line=false
        else
            printf "${bc}║${tc}%s${vc}%s%*s${bc}║${NC}\n" "$cont_pad" "$chunk" "$right_pad" ""
        fi
    done
}

box_menu() {
    local bc=$1 tc=$2 key=$3 lbl=$4 val=$5 val_color=${6:-$2}
    local inner=$(( W - 2 ))
    local left_str="${PAD_STR}[${key}]  ${lbl}"
    local right_str="${val}${PAD_STR}"
    [ -z "$val" ] && right_str="${PAD_STR}"
    
    local total_len=$(( ${#left_str} + ${#right_str} ))
    if [ $total_len -gt $inner ]; then
        local avail=$(( inner - ${#right_str} - 7 )) 
        lbl="${lbl:0:$(( avail - 1 ))}…"
        left_str="${PAD_STR}[${key}]  ${lbl}"
    fi
    
    local pad=$(( inner - ${#left_str} - ${#right_str} ))
    [ $pad -lt 0 ] && pad=0
    printf "${bc}║${tc}%s%*s${val_color}%s${bc}║${NC}\n" "$left_str" "$pad" "" "$right_str"
}

box_rule() {
    local bc=$1 lc=${2:-} label=${3:-}
    local inner=$(( W - 2 ))
    if [ -n "$label" ]; then
        local rest=$(( inner - ${#label} - 8 ))
        [ $rest -lt 1 ] && rest=1
        printf "${bc}║${lc} ── %s $(_tbar $rest)${bc}║${NC}\n" "$label"
    else
        printf "${bc}║${DIM}$(_tbar $inner)${bc}║${NC}\n"
    fi
}

log_sep() {
    local label="${1:-}"
    if [ -n "$label" ]; then
        local rest=$(( W - ${#label} - 5 ))
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
    local cfg=$1 en=$2 new_line
    if [ "$en" = true ]; then new_line="${cfg}=y"; else new_line="# ${cfg} is not set"; fi
    if grep -qE "^(# )?${cfg}[= ]" "$DEFCONFIG_PATH" 2>/dev/null; then
        sed -i -e "s|^# ${cfg} is not set|${new_line}|" -e "s|^${cfg}=.*|${new_line}|" "$DEFCONFIG_PATH"
    else
        echo "$new_line" >> "$DEFCONFIG_PATH"
    fi
}

get_cap_str() {
    if $FEAT_KSU; then
        local btag; [ "$KSU_BRANCH" = "dev" ] && btag="DEV" || btag="MAIN"
        local hook; $FEAT_SUSFS && hook="SuSFS-Inline-Hook" || hook="Manual-Hook"
        echo "[${btag}-ReSukiSU | Hook-Mode=${hook}]"
    else
        echo "[Vanilla]"
    fi
}

get_ext_feat_str() {
    if $FEAT_KSU; then
        local extras=""
        $FEAT_SUSFS && extras="SuSFS"
        $FEAT_KPM   && extras="${extras:+${extras} + }KPM"
        [ -n "$extras" ] && echo "[${extras}]" || echo "[None]"
    else
        echo "[None]"
    fi
}

_build_zip_name() { echo "$(_build_log_base)-{Build-#${BUILD_NUM}}.zip"; }
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
    box_ctr "$MAG" "$WHT" "VAYU  KERNEL  BUILDER  --  AnyMore Project"
    box_rule "$MAG" "$DIM"
    box_ctr "$MAG" "$DIM" "Linux 4.14 NonGKI  |  Poco X3 Pro (vayu)  |  Android 16  |  Clang ${clang_ver}"
    box_bot "$MAG"
}

draw_title_static() {
    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "VAYU  KERNEL  BUILDER  --  AnyMore Project"
    box_rule "$MAG" "$DIM"
    box_ctr "$MAG" "$DIM" "Linux 4.14 NonGKI  |  Poco X3 Pro (vayu)  |  Android 16"
    box_bot "$MAG"
}

# =============================================================================
# TOAST UI
# =============================================================================
draw_toast() {
    if [ -n "$TOAST_MSG" ]; then
        printf "\n"
        box_top "$TOAST_MSG_C"
        box_ctr "$TOAST_MSG_C" "$TOAST_MSG_C" "$TOAST_MSG"
        if [ -n "$TOAST_SUBMSG" ]; then
            box_ctr "$TOAST_MSG_C" "$WHT" "$TOAST_SUBMSG"
        fi
        box_bot "$TOAST_MSG_C"
    fi
}

# =============================================================================
# STEP 1 — FEATURE CONFIGURATION
# =============================================================================
_draw_feat_full() {
    _MENU_REDRAW_FN="_draw_feat_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "FEATURE  CONFIGURATION"
    box_div "$CYN"

    if ! _drivers_present; then
        box_menu "$CYN" "$GRY" "1" "ReSukiSU" "⊘ NO DRIVER" "$GRY"
        box_rule "$CYN" "$DIM"
        box_menu "$CYN" "$GRY" "2" "SuSFS" "⊘ NO DRIVER" "$GRY"
        box_rule "$CYN" "$DIM"
        box_menu "$CYN" "$GRY" "3" "KPM" "⊘ NO DRIVER" "$GRY"
    else
        if $FEAT_KSU; then
            box_menu "$CYN" "$WHT" "1" "ReSukiSU" "● ENABLED" "$LGR"
        else
            box_menu "$CYN" "$GRY" "1" "ReSukiSU" "○ DISABLED" "$GRY"
        fi
        box_rule "$CYN" "$DIM"

        if ! $FEAT_KSU; then
            box_menu "$CYN" "$GRY" "2" "SuSFS" "─ N/A" "$GRY"
        elif $FEAT_SUSFS; then
            box_menu "$CYN" "$WHT" "2" "SuSFS" "● ENABLED" "$LGR"
        else
            box_menu "$CYN" "$GRY" "2" "SuSFS" "○ DISABLED" "$GRY"
        fi
        box_rule "$CYN" "$DIM"

        if ! $FEAT_KSU; then
            box_menu "$CYN" "$GRY" "3" "KPM" "─ N/A" "$GRY"
        elif $FEAT_KPM; then
            box_menu "$CYN" "$WHT" "3" "KPM" "● ENABLED" "$LGR"
        else
            box_menu "$CYN" "$GRY" "3" "KPM" "○ DISABLED" "$GRY"
        fi
    fi 

    box_div "$CYN"
    if ! _drivers_present; then
        box_lbl "$CYN" "$GRY" "Hook Mode" "─ N/A"
    elif $FEAT_KSU && $FEAT_SUSFS; then
        box_lbl "$CYN" "$DIM" "Hook Mode" "⇒ SuSFS-Inline-Hook" "$CYN"
    elif $FEAT_KSU; then
        box_lbl "$CYN" "$WHT" "Hook Mode" "⇒ Manual-Hook" "$LGR"
    else
        box_lbl "$CYN" "$GRY" "Hook Mode" "─ N/A"
    fi
    box_bot "$CYN"

    box_top "$CYN"
    box_menu "$CYN" "$MAG" "M" "Open Menuconfig" "" ""
    box_rule "$CYN" "$DIM"
    box_menu "$CYN" "$LGR" "C" "Confirm & Continue" "" ""
    box_rule "$CYN" "$DIM"
    box_menu "$CYN" "$GRY" "B" "Back to Mode Select" "" ""
    box_menu "$CYN" "$GRY" "Q" "Quit" "" ""

    if $MENUCONFIG_USED; then
        box_rule "$CYN" "$YEL" "Menuconfig"
        box_row "$CYN" "$YEL" "⚑ Active this session — defconfig regen skipped at Stage 2"
        box_row "$CYN" "$GRY" "Use [V] or [D] after build to persist or discard"
    elif $_PRESERVE_ACTIVE; then
        box_rule "$CYN" "$MAG" "Menuconfig"
        box_row "$CYN" "$MAG" "⚑ Preserved .config will be restored on next build"
        box_row "$CYN" "$GRY" "Use [D] after build to make it permanent, or ignore to expire"
    fi
    box_bot "$CYN"

    draw_toast
    printf '\033[J'
    _cursor_show
    printf "\n${WHT}  Select [1/2/3/M/C/B/Q]: ${NC}"
}

run_feat_menu() {
    TOAST_MSG=""
    _draw_feat_full

    while true; do
        IFS= read -r -s -n1 choice
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input
            TOAST_MSG=""; _draw_feat_full; continue
        fi
        _drain_input
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            1)  if ! _drivers_present; then
                    TOAST_MSG="No driver installed"
                    TOAST_SUBMSG="Use [M] in Mode Select to install ReSukiSU first"
                    TOAST_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if $FEAT_KSU; then
                    FEAT_KSU=false;   toggle_config "CONFIG_KSU"             false
                    FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS"       false
                    FEAT_KPM=false;   toggle_config "CONFIG_KPM"             false
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" false
                    TOAST_MSG="ReSukiSU disabled"
                    TOAST_SUBMSG="SuSFS + KPM + ManualHook also cleared"
                    TOAST_MSG_C="$YEL"
                else
                    FEAT_KSU=true; toggle_config "CONFIG_KSU" true
                    if ! $FEAT_SUSFS; then toggle_config "CONFIG_KSU_MANUAL_HOOK" true; fi
                    TOAST_MSG="ReSukiSU enabled"; TOAST_SUBMSG=""; TOAST_MSG_C="$LGR"
                fi
                _draw_feat_full ;;
            2)  if ! _drivers_present; then
                    TOAST_MSG="No driver installed"
                    TOAST_SUBMSG="Use [M] in Mode Select to install ReSukiSU first"
                    TOAST_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if ! $FEAT_KSU; then
                    TOAST_MSG="Enable ReSukiSU first — SuSFS requires it"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if $FEAT_SUSFS; then
                    FEAT_SUSFS=false; toggle_config "CONFIG_KSU_SUSFS"       false
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" true
                    TOAST_MSG="SuSFS disabled"
                    TOAST_SUBMSG="Manual-Hook auto-enabled"
                    TOAST_MSG_C="$LGR"
                else
                    FEAT_SUSFS=true;  toggle_config "CONFIG_KSU_SUSFS"       true
                                      toggle_config "CONFIG_KSU_MANUAL_HOOK" false
                    TOAST_MSG="SuSFS enabled"
                    TOAST_SUBMSG="SuSFS-Inline-Hook active"
                    TOAST_MSG_C="$YEL"
                fi
                _draw_feat_full ;;
            3)  if ! _drivers_present; then
                    TOAST_MSG="No driver installed"
                    TOAST_SUBMSG="Use [M] in Mode Select to install ReSukiSU first"
                    TOAST_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if ! $FEAT_KSU; then
                    TOAST_MSG="Enable ReSukiSU first — KPM requires it"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"; _draw_feat_full; continue
                fi
                if $FEAT_KPM; then
                    FEAT_KPM=false; toggle_config "CONFIG_KPM" false
                    TOAST_MSG="KPM disabled"; TOAST_SUBMSG=""; TOAST_MSG_C="$GRY"
                else
                    FEAT_KPM=true;  toggle_config "CONFIG_KPM" true
                    TOAST_MSG="KPM enabled"; TOAST_SUBMSG=""; TOAST_MSG_C="$LGR"
                fi
                _draw_feat_full ;;
            m)  do_clear
                local _mc_mtime_before
                _mc_mtime_before=$(stat -c %Y "${objdir}/.config" 2>/dev/null || echo "0")
                trap '' INT
                make -C "$kernel_dir" O="$objdir" ARCH=arm64 LLVM=1 LLVM_IAS=1 CC="$MAKE_CC" menuconfig
                local _mc_rc=$?
                trap - INT
                tput reset
                local _mc_mtime_after
                _mc_mtime_after=$(stat -c %Y "${objdir}/.config" 2>/dev/null || echo "0")

                if [ "$_mc_rc" -ne 0 ]; then
                    TOAST_MSG="Menuconfig aborted — no changes applied"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$YEL"
                elif [ "$_mc_mtime_after" != "$_mc_mtime_before" ]; then
                    MENUCONFIG_USED=true
                    _SKIP_DEFCONFIG=true
                    _PRESERVE_ACTIVE=false
                    rm -f "$MENUCONFIG_PRESERVE_FILE"
                    read_features
                    TOAST_MSG="Menuconfig saved"
                    TOAST_SUBMSG="Stage 2 defconfig regen skipped this session"
                    TOAST_MSG_C="$CYN"
                else
                    TOAST_MSG="Menuconfig closed without saving — no changes"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$YEL"
                fi
                _draw_feat_full ;;
            c)  return 0 ;;
            b)  return 1 ;;
            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   TOAST_MSG="Unknown key — use 1/2/3/M/C/B/Q"; TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"
                 _draw_feat_full ;;
        esac
    done
}

# =============================================================================
# STEP 2 — BUILD OPTIONS
# =============================================================================
_draw_build_full() {
    _MENU_REDRAW_FN="_draw_build_full"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "ACTIVE  FEATURES"
    box_div "$CYN"
    box_lbl "$CYN" "$WHT" "Capabilities" "$(get_cap_str)"
    box_lbl "$CYN" "$WHT" "Features" "$(get_ext_feat_str)"
    box_bot "$CYN"

    box_top "$BLU"
    box_ctr "$BLU" "$YEL" "BUILD  OPTIONS"
    box_div "$BLU"

    if [ -n "$KERNEL_NAME" ]; then
        box_lbl "$BLU" "$WHT" "Kernel Name" "${KERNEL_NAME}"
    else
        box_menu "$BLU" "$GRY" "N" "Set Kernel Name" "optional — appended to LOCALVERSION" "$GRY"
    fi
    box_rule "$BLU" "$DIM"

    if [ -n "$FORCE_CLEAN_REASON" ]; then
        box_menu "$BLU" "$GRY" "I" "Incremental Build" "⊘ LOCKED" "$GRY"
        box_row "$BLU" "$GRY" "Reason: ${FORCE_CLEAN_REASON} — full clean required"
    elif $INCREMENTAL; then
        box_menu "$BLU" "$WHT" "I" "Incremental Build" "● ON" "$YEL"
    else
        box_menu "$BLU" "$GRY" "I" "Incremental Build" "○ OFF" "$GRY"
    fi
    box_rule "$BLU" "$DIM"

    if [ -z "$CCACHE_BIN" ]; then
        box_menu "$BLU" "$GRY" "C" "ccache" "─ N/A" "$GRY"
    elif $USE_CCACHE; then
        box_menu "$BLU" "$WHT" "C" "ccache" "● ON" "$YEL"
    else
        box_menu "$BLU" "$GRY" "C" "ccache" "○ OFF" "$GRY"
    fi
    box_rule "$BLU" "$DIM"

    box_menu "$BLU" "$LGR" "S" "Start Build" "" ""
    box_div "$BLU"
    box_menu "$BLU" "$GRY" "B" "Back to Features" "" ""
    box_menu "$BLU" "$LRD" "X" "Reset Build Counter" "(#${BUILD_NUM} → #1)" "$LRD"
    box_menu "$BLU" "$GRY" "Q" "Quit" "" ""
    box_bot "$BLU"

    draw_toast
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
        box_lbl "$CYN" "$WHT" "Current" "${KERNEL_NAME}"
    else
        box_lbl "$CYN" "$GRY" "Current" "(none — base version used as-is)"
    fi
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "Sets the LOCALVERSION suffix appended after the base kernel version."
    box_row "$CYN" "$GRY" "Input        : AnyMore-v2.1"
    box_row "$CYN" "$GRY" "Result       : uname -r shows  4.14.356-AnyMore-v2.1"
    box_rule "$CYN" "$DIM"
    box_row "$CYN" "$GRY" "Used in the zip / log filename as the kernel label."
    box_row "$CYN" "$GRY" "Max ${MAX_NAME_LEN} chars. Quotes, slashes, brackets stripped."
    box_row "$CYN" "$GRY" "Press Enter with no input to clear and restore the default."
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
        TOAST_MSG="Kernel name set: \"${_short_name}\""
    else
        TOAST_MSG="Kernel name cleared — default naming active"
    fi
    TOAST_SUBMSG=""
    TOAST_MSG_C="$LGR"
    _winch_enable
}

run_build_menu() {
    TOAST_MSG=""
    _draw_build_full

    while true; do
        IFS= read -r -s -n1 choice
        if [ "$choice" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input
            TOAST_MSG=""; _draw_build_full; continue
        fi
        _drain_input
        choice=$(printf '%s' "$choice" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')

        case "$choice" in
            n)  prompt_kernel_name; _draw_build_full ;;
            i)  if [ -n "$FORCE_CLEAN_REASON" ]; then
                    TOAST_MSG="Incremental locked"
                    TOAST_SUBMSG="${FORCE_CLEAN_REASON}"
                    TOAST_MSG_C="$YEL"; _draw_build_full
                elif $INCREMENTAL; then
                    INCREMENTAL=false
                    TOAST_MSG="Incremental OFF — full clean build"
                    TOAST_SUBMSG=""
                    TOAST_MSG_C="$GRY"
                    _save_state; _draw_build_full
                else
                    INCREMENTAL=true
                    TOAST_MSG="Incremental ON — keeps previous objects"
                    TOAST_SUBMSG=""
                    TOAST_MSG_C="$YEL"
                    _save_state; _draw_build_full
                fi ;;
            c)  if [ -z "$CCACHE_BIN" ]; then
                    TOAST_MSG="ccache binary not found — install ccache to enable"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"
                elif $USE_CCACHE; then
                    USE_CCACHE=false
                    TOAST_MSG="ccache OFF — builds will be slower"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$GRY"
                    _save_state; _update_make_cc
                else
                    USE_CCACHE=true
                    TOAST_MSG="ccache ON — compiler cache active"
                    TOAST_SUBMSG=""; TOAST_MSG_C="$YEL"
                    _save_state; _update_make_cc
                fi
                _draw_build_full ;;
            s)  _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build; return 0 ;;
            x)  reset_build_num
                TOAST_MSG="Both counters reset"
                TOAST_SUBMSG="zip/log #1 · kernel uname #1 on next build"
                TOAST_MSG_C="$LRD"
                _draw_build_full ;;
            b)  return 1 ;;
            q)  do_clear; printf "${WHT}  Goodbye.${NC}\n\n"; exit 0 ;;
            '')  continue ;;
            *)   TOAST_MSG="Unknown key — use N/I/C/S/B/X/Q"
                 TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"
                 _draw_build_full ;;
        esac
    done
}

# =============================================================================
# BUILD SUMMARY
# =============================================================================
draw_summary() {
    local mode="Full Clean Build"; $INCREMENTAL && mode="Incremental"
    local cc_label
    if [ -z "$CCACHE_BIN" ]; then cc_label="not found"
    elif $USE_CCACHE; then       cc_label="enabled"
    else                          cc_label="disabled"
    fi

    box_top "$YEL"
    box_ctr "$YEL" "$YEL" "BUILD  SUMMARY"
    box_div "$YEL"
    box_lbl "$YEL" "$WHT" "Target" "vayu_a16_kernel (Linux 4.14 NonGKI)"
    box_lbl "$YEL" "$WHT" "Build" "#${BUILD_NUM}"
    [ -n "$KERNEL_NAME" ] && box_lbl "$YEL" "$WHT" "Kernel-Name" "${KERNEL_NAME}"
    box_lbl "$YEL" "$WHT" "Capabilities" "$(get_cap_str)"
    box_lbl "$YEL" "$WHT" "Features" "$(get_ext_feat_str)"
    box_lbl "$YEL" "$WHT" "Mode" "${mode}"
    box_lbl "$YEL" "$WHT" "ccache" "${cc_label}"
    box_lbl "$YEL" "$WHT" "Output" "${OUTPUT_DIR}"
    if $_SKIP_DEFCONFIG; then
        box_rule "$YEL" "$YEL" "Menuconfig"
        if $_PRESERVE_ACTIVE; then
            box_row "$YEL" "$YEL" "⚑ Restoring preserved .config from previous session"
        else
            box_row "$YEL" "$YEL" "⚑ Using in-session menuconfig .config (defconfig skipped)"
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
    printf '  ░▀▀▀░▀▀▀░▀░▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░\n'
    printf "${NC}\n"
}

# =============================================================================
# RESULT BOXES
# =============================================================================
print_success_box() {
    local elapsed=$1 zippath=${2:-""}
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
    box_lbl "$LGR" "$WHT" "Build" "#${BUILD_NUM}"
    [ -n "$KERNEL_NAME" ] && box_lbl "$LGR" "$WHT" "Kernel-Name" "${KERNEL_NAME}"
    box_lbl "$LGR" "$WHT" "Capabilities" "$(get_cap_str)"
    box_lbl "$LGR" "$WHT" "Features" "$(get_ext_feat_str)"
    box_lbl "$LGR" "$WHT" "ccache" "${cc_label}"
    box_lbl "$LGR" "$WHT" "Time" "${elapsed}"
    box_lbl "$LGR" "$WHT" "When" "$(date '+%Y-%m-%d %H:%M')"
    box_rule "$LGR" "$LGR"
    box_lbl "$LGR" "$LGR" "Zip" "$zipname"
    [ -n "$zip_size" ] && box_lbl "$LGR" "$GRY" "Size" "${zip_size}"
    box_lbl "$LGR" "$GRY" "Output" "${OUTPUT_DIR}"
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
        local maxw=$(( W-8 ))
        local ld_errs; ld_errs=$(grep "ld\.lld:.*error:" "$logfile" 2>/dev/null | tail -4)
        local cc_errs; cc_errs=$(grep -E "error:" "$logfile" 2>/dev/null | grep -v "ld\.lld:" | tail -6)
        local shown=false
        if [ -n "$ld_errs" ]; then
            box_ctr "$LRD" "$YEL" "LINKER"
            box_rule "$LRD" "$DIM"
            while IFS= read -r line; do
                [ ${#line} -gt $maxw ] && line="${line:0:$(( maxw-1 ))}…"
                box_row "$LRD" "$YEL" "${line}"
            done <<< "$ld_errs"
            shown=true
        fi
        if [ -n "$cc_errs" ]; then
            $shown && box_rule "$LRD" "$DIM"
            box_ctr "$LRD" "$RED" "COMPILER"
            box_rule "$LRD" "$DIM"
            while IFS= read -r line; do
                [ ${#line} -gt $maxw ] && line="${line:0:$(( maxw-1 ))}…"
                box_row "$LRD" "$RED" "${line}"
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
    box_lbl "$LRD" "$GRY" "Log" "${logfile##*/}"
    box_bot "$LRD"
}

print_cancelled_box() {
    local elapsed=$1
    log_sep "OUTPUT"
    print_cancelled_art
    box_top "$YEL"
    box_ctr "$YEL" "$WHT" "Build cancelled by user"
    box_rule "$YEL" "$GRY"
    box_lbl "$YEL" "$GRY" "Elapsed" "${elapsed}"
    box_row "$YEL" "$GRY" "Objects in out/ are intact for incremental retry"
    box_bot "$YEL"
}

# =============================================================================
# POST-BUILD PROMPT
# =============================================================================
post_build_prompt() {
    printf "\n"
    box_top "$WHT"
    box_ctr "$WHT" "$YEL" "WHAT  NEXT?"
    box_div "$WHT"
    box_menu "$WHT" "$YEL" "T" "Retry — Full Clean" "" ""
    box_menu "$WHT" "$CYN" "I" "Retry — Incremental" "" ""
    if $MENUCONFIG_USED; then
        box_rule "$WHT" "$MAG" "Menuconfig"
        box_menu "$WHT" "$MAG" "V" "Preserve for next ./build.sh run" "" ""
        box_row "$WHT" "$GRY" "(saves .config to disk, restored automatically on relaunch)"
        box_menu "$WHT" "$LGR" "D" "Write permanently to vayu_defconfig" "" ""
        box_row "$WHT" "$GRY" "(runs savedefconfig — becomes the new build default)"
    fi
    box_rule "$WHT" "$DIM"
    box_menu "$WHT" "$WHT" "R" "Return to menu" "" ""
    box_menu "$WHT" "$WHT" "E" "Exit script" "" ""
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
                    make -C "$kernel_dir" O="$objdir" ARCH=arm64 LLVM=1 LLVM_IAS=1 CC="$MAKE_CC" savedefconfig 2>&1 | \
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

    log_sep "STAGE 5 — PACKAGE"
    box_top "$MAG"
    box_ctr "$MAG" "$MAG" "Packaging AnyKernel3 zip..."
    box_rule "$MAG" "$DIM"
    box_lbl "$MAG" "$GRY" "Output" "${zipname}"
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
_ksu_local_commit() {
    local _ksu_dir="${kernel_dir}/drivers/kernelsu"
    local real_path
    real_path=$(readlink -f "$_ksu_dir" 2>/dev/null)
    [ -z "$real_path" ] && real_path="$_ksu_dir"
    [ ! -e "$real_path" ] && { echo ""; return; }
    local dir="$real_path"
    while [ "$dir" != "/" ]; do
        if [ -d "${dir}/.git" ] || [ -f "${dir}/.git" ]; then
            git -C "$dir" rev-parse HEAD 2>/dev/null
            return
        fi
        dir=$(dirname "$dir")
    done
    echo ""
}

_check_resukisu_update() {
    local branch="$KSU_BRANCH"
    local remote_commit
    remote_commit=$(git ls-remote "https://github.com/ReSukiSU/ReSukiSU.git" "refs/heads/${branch}" 2>/dev/null | awk '{print $1; exit}')
    if [ -z "$remote_commit" ]; then echo "network_fail"; return; fi
    local local_commit; local_commit=$(_ksu_local_commit)
    if [ -z "$local_commit" ]; then echo "no_local"; return; fi
    if [ "$local_commit" = "$remote_commit" ]; then echo "up_to_date:${local_commit:0:8}"; else echo "update_available:${remote_commit:0:8}"; fi
}

_drivers_present() { [ -e "${kernel_dir}/drivers/kernelsu" ]; }

_draw_update_menu() {
    _MENU_REDRAW_FN="_draw_update_menu"
    _winch_enable
    set_width
    _cursor_hide
    printf '\033[H'
    draw_title_static

    box_top "$ORG"
    box_ctr "$ORG" "$YEL" "ReSukiSU  DRIVER  MANAGER"
    box_div "$ORG"
    box_lbl "$ORG" "$WHT" "Source" "github.com/ReSukiSU/ReSukiSU"
    if [ "$KSU_BRANCH" = "dev" ]; then
        box_lbl "$ORG" "$WHT" "Branch" "dev  (development)" "$YEL"
    else
        box_lbl "$ORG" "$WHT" "Branch" "main  (stable)" "$LGR"
    fi
    box_lbl "$ORG" "$GRY" "Target" "${kernel_dir}/drivers/kernelsu"
    box_rule "$ORG" "$DIM"
    if _drivers_present; then
        box_menu "$ORG" "$WHT" "U" "Update Driver" "pull from active branch" "$GRY"
    else
        box_menu "$ORG" "$LGR" "I" "Install Driver" "run setup.sh from active branch" "$GRY"
    fi
    box_menu "$ORG" "$CYN" "S" "Switch Branch" "main <-> dev, auto-pulls" "$GRY"
    box_menu "$ORG" "$YEL" "V" "Verify Guards" "check KSU/SuSFS hooks in code" "$GRY"
    box_rule "$ORG" "$DIM"
    box_menu "$ORG" "$LRD" "X" "Remove Driver" "runs setup.sh --cleanup" "$GRY"
    box_rule "$ORG" "$DIM"
    box_menu "$ORG" "$GRY" "R" "Return to menu" "" ""
    box_bot "$ORG"

    draw_toast
    printf '\033[J'
    _cursor_show
    
    local opts="U/S/V/X/R"
    if ! _drivers_present; then
        opts="I/S/V/X/R"
    fi
    printf "\n${ORG}  Select [%s]: ${NC}" "$opts"
}

do_update_resukisu() {
    _winch_disable
    set_width
    do_clear
    TOAST_MSG=""
    TOAST_SUBMSG=""
    _draw_update_menu

    while true; do
        IFS= read -r -s -n1 key
        if [ "$key" = $'\033' ]; then
            IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null
            _drain_input; _draw_update_menu; continue
        fi
        _drain_input
        key=$(printf '%s' "$key" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        case "$key" in
            v)  do_clear
                TOAST_MSG="Verifying KSU/SuSFS hook guards..."
                TOAST_SUBMSG=""
                TOAST_MSG_C="$CYN"
                _draw_update_menu
                _apply_ksu_guards > /dev/null
                TOAST_MSG="Guard Verification Complete"
                TOAST_SUBMSG="${GUARD_TOAST}"
                TOAST_MSG_C="$LGR"
                do_clear
                _draw_update_menu
                continue ;;
            s)  [ "$KSU_BRANCH" = "main" ] && KSU_BRANCH="dev" || KSU_BRANCH="main"
                _save_state
                TOAST_MSG="Pulling ${KSU_BRANCH} branch..."
                TOAST_SUBMSG=""; TOAST_MSG_C="$CYN"
                _draw_update_menu
                _do_resukisu_pull
                if [ "$_LAST_PULL_STATUS" -eq 0 ]; then
                    FORCE_CLEAN_REASON="Branch switched"
                    INCREMENTAL=false; _save_state
                    TOAST_MSG="✓ Switched to ${KSU_BRANCH} — clean build required"
                    TOAST_SUBMSG="${GUARD_TOAST}"
                    TOAST_MSG_C="$YEL"
                else
                    TOAST_MSG="✗ Switch to ${KSU_BRANCH} failed — check output"
                    TOAST_SUBMSG="${GUARD_TOAST}"
                    TOAST_MSG_C="$LRD"
                fi
                set_width; do_clear; _draw_update_menu
                continue ;;
            u|i) TOAST_MSG="Checking upstream status..."
                TOAST_SUBMSG=""; TOAST_MSG_C="$CYN"
                _draw_update_menu
                local _chk; _chk=$(_check_resukisu_update)
                case "$_chk" in
                    up_to_date:*)
                        TOAST_MSG="● Up-to-date  (${_chk#up_to_date:})"
                        TOAST_SUBMSG=""; TOAST_MSG_C="$LGR"
                        _draw_update_menu ;;
                    update_available:*)
                        local _rsha="${_chk#update_available:}"
                        TOAST_MSG=""; TOAST_MSG_C="$LGR"
                        _do_resukisu_pull
                        if [ "$_LAST_PULL_STATUS" -eq 0 ]; then
                            TOAST_MSG="✓ Updated to ${_rsha}"
                            TOAST_SUBMSG="${GUARD_TOAST}"
                            TOAST_MSG_C="$LGR"
                        else
                            TOAST_MSG="✗ Update failed — see output above"
                            TOAST_SUBMSG="${GUARD_TOAST}"
                            TOAST_MSG_C="$LRD"
                        fi
                        set_width; do_clear; _draw_update_menu ;;
                    no_local)
                        TOAST_MSG=""; TOAST_MSG_C="$LGR"
                        _do_resukisu_pull
                        if [ "$_LAST_PULL_STATUS" -eq 0 ]; then
                            TOAST_MSG="✓ Driver installed successfully"
                            TOAST_SUBMSG="${GUARD_TOAST}"
                            TOAST_MSG_C="$LGR"
                        else
                            TOAST_MSG="✗ Install failed — see output above"
                            TOAST_SUBMSG="${GUARD_TOAST}"
                            TOAST_MSG_C="$LRD"
                        fi
                        set_width; do_clear; _draw_update_menu ;;
                    network_fail)
                        TOAST_MSG="✗ Network error — check connection"
                        TOAST_SUBMSG=""; TOAST_MSG_C="$LRD"
                        _draw_update_menu ;;
                    *)
                        TOAST_MSG=""; _do_resukisu_pull
                        set_width; do_clear; _draw_update_menu ;;
                esac
                continue ;;
            x)  _do_resukisu_cleanup
                set_width; do_clear; _draw_update_menu
                continue ;;
            r)  TOAST_MSG=""; TOAST_SUBMSG=""; TOAST_MSG_C="$LGR"; break ;;
            '') continue ;;
        esac
    done
    _winch_disable
}

_do_resukisu_cleanup() {
    _winch_disable
    set_width
    do_clear

    local setup_url="https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh"

    box_top "$LRD"
    box_ctr "$LRD" "$YEL" "REMOVE  ReSukiSU  DRIVER"
    box_div "$LRD"
    box_lbl "$LRD" "$WHT" "Runs" "setup.sh --cleanup"
    box_lbl "$LRD" "$WHT" "Removes" "drivers/kernelsu/"
    box_lbl "$LRD" "$WHT" "Also clears" "out/drivers/kernelsu/  KernelSU/"
    box_rule "$LRD" "$YEL"
    box_row "$LRD" "$YEL" "This will remove KSU from the kernel source tree."
    box_row "$LRD" "$YEL" "A full clean build is required afterward."
    box_bot "$LRD"
    printf "\n${LRD}  Confirm removal? [y/N]: ${NC}"

    local _confirm
    IFS= read -r -s -n1 _confirm
    _drain_input
    _confirm=$(printf '%s' "$_confirm" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
    if [ "$_confirm" != "y" ]; then
        TOAST_MSG="Removal cancelled"
        TOAST_SUBMSG=""
        TOAST_MSG_C="$GRY"
        return
    fi

    printf "\n"
    log_sep "RUNNING SETUP.SH --CLEANUP"

    local _rc_file; _rc_file=$(mktemp /tmp/vkb_cln_XXXXXX)
    (
        cd "$kernel_dir" || exit 1
        bash <(curl -LSs "$setup_url") --cleanup
        echo "$?" > "$_rc_file"
    ) 2>&1 | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done

    local CLEANUP_EXIT; CLEANUP_EXIT=$(cat "$_rc_file" 2>/dev/null); rm -f "$_rc_file"
    CLEANUP_EXIT=$(( ${CLEANUP_EXIT:-1} + 0 ))

    if [ -d "${kernel_dir}/KernelSU" ]; then
        printf "  ${CYN}●${NC} Removing KernelSU/ clone...\n"
        rm -rf "${kernel_dir}/KernelSU"
    fi
    if [ -d "${objdir}/drivers/kernelsu" ]; then
        printf "  ${CYN}●${NC} Clearing out/drivers/kernelsu objects...\n"
        rm -rf "${objdir}/drivers/kernelsu"
    fi
    local _int_link="${kernel_dir}/drivers/kernelsu/kernel"
    if [ -L "$_int_link" ] && [ ! -e "$_int_link" ]; then
        rm -f "$_int_link"
    fi

    log_sep "VERIFY & CHECK"
    printf "  ${CYN}●${NC} Verifying KSU hook guards (expect skips):\n"
    _apply_ksu_guards

    printf "\n"
    if [ "$CLEANUP_EXIT" -eq 0 ] && [ ! -d "${kernel_dir}/drivers/kernelsu" ]; then
        toggle_config "CONFIG_KSU"              false
        toggle_config "CONFIG_KSU_SUSFS"        false
        toggle_config "CONFIG_KSU_MANUAL_HOOK"  false
        toggle_config "CONFIG_KPM"              false
        read_features
        FORCE_CLEAN_REASON="Driver removed"
        INCREMENTAL=false; _save_state
        box_top "$LGR"
        box_ctr "$LGR" "$LGR" "KSU driver removed successfully"
        box_rule "$LGR" "$DIM"
        box_lbl "$LGR" "$WHT" "Removes" "drivers/kernelsu/"
        box_lbl "$LGR" "$WHT" "KernelSU" "removed"
        box_lbl "$LGR" "$WHT" "out/ objects" "cleared"
        box_lbl "$LGR" "$WHT" "defconfig" "KSU options disabled"
        box_lbl "$LGR" "$YEL" "Note" "Full clean build required"
        box_bot "$LGR"
        TOAST_MSG="✓ Driver removed — defconfig reset"
        TOAST_SUBMSG="${GUARD_TOAST}"
        TOAST_MSG_C="$LGR"
    else
        box_top "$LRD"
        box_ctr "$LRD" "$LRD" "Cleanup may have failed or driver already absent"
        box_rule "$LRD" "$DIM"
        box_lbl "$LRD" "$WHT" "Exit code" "${CLEANUP_EXIT}"
        box_row "$LRD" "$WHT" "Check output above and retry if needed"
        box_bot "$LRD"
        TOAST_MSG="✗ Cleanup failed or driver already gone"
        TOAST_SUBMSG="${GUARD_TOAST}"
        TOAST_MSG_C="$LRD"
    fi

    printf "\n"
    box_top "$ORG"
    box_menu "$ORG" "$WHT" "R" "Return" "" ""
    box_bot "$ORG"
    printf "\n${ORG}  Press [R] to return: ${NC}"

    while true; do
        IFS= read -r -s -n1 key2
        [ "$key2" = $'\033' ] && { IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null; _drain_input; continue; }
        _drain_input
        key2=$(printf '%s' "$key2" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        [ "$key2" = "r" ] && break
        [ "$key2" = "" ] && continue
    done
}

# _guard_entry_row bc tc_label tc_detail file_part detail
#
# Renders one guard entry inside the box.  The file:line column is
# fixed-width (GUARD_LC chars); the detail wraps onto continuation
# lines that are indented to align exactly under the first detail char.
#
#   ║  fs/stat.c:398          | ksu_handle_stat(&dfd, &filename,     ║
#   ║                         |   &flag);                             ║
#
_guard_entry_row() {
    local bc=$1 tc_lbl=$2 tc_det=$3 file_part=$4 detail=$5

    local GUARD_LC=24          # width of the file:line column (excl. leading 2-space pad)
    local SEP=" | "            # column separator
    local SEP_W=${#SEP}        # 3
    local LPAD=2               # leading spaces inside the box
    local inner=$(( W - 2 ))   # usable width between the box borders

    # Truncate file:line if absurdly long (e.g. long driver path)
    if [ ${#file_part} -gt $GUARD_LC ]; then
        file_part="${file_part:0:$(( GUARD_LC - 1 ))}~"
    fi

    # Width available for the detail text on each line
    local det_w=$(( inner - LPAD - GUARD_LC - SEP_W ))
    [ $det_w -lt 10 ] && det_w=10

    # Continuation indent string: LPAD + GUARD_LC spaces + SEP
    local cont_indent
    printf -v cont_indent "%*s%s" $(( LPAD + GUARD_LC )) "" "$SEP"

    # Pad file_part to GUARD_LC
    local fp_padded
    printf -v fp_padded "%-*s" "$GUARD_LC" "$file_part"

    # Split detail into chunks of det_w, breaking on spaces when possible
    local rem="$detail"
    local first=true

    while [ ${#rem} -gt 0 ]; do
        local chunk
        if [ ${#rem} -le $det_w ]; then
            chunk="$rem"
            rem=""
        else
            # Find last space at or before det_w
            local brk=$det_w
            local i=$det_w
            while [ $i -gt $(( det_w / 2 )) ]; do
                [ "${rem:$i:1}" = " " ] && { brk=$i; break; }
                i=$(( i - 1 ))
            done
            chunk="${rem:0:$brk}"
            rem="${rem:$brk}"
            # Strip leading space from remainder
            while [ "${rem:0:1}" = " " ]; do rem="${rem:1}"; done
        fi

        local right_pad
        if $first; then
            right_pad=$(( inner - LPAD - GUARD_LC - SEP_W - ${#chunk} ))
            [ $right_pad -lt 0 ] && right_pad=0
            printf "${bc}║${tc_lbl}%*s%s${SEP}${tc_det}%s%*s${bc}║${NC}\n" \
                "$LPAD" "" "$fp_padded" "$chunk" "$right_pad" ""
            first=false
        else
            right_pad=$(( inner - LPAD - GUARD_LC - SEP_W - ${#chunk} ))
            [ $right_pad -lt 0 ] && right_pad=0
            printf "${bc}║${tc_det}%s%s%*s${bc}║${NC}\n" \
                "$cont_indent" "$chunk" "$right_pad" ""
        fi
    done
}

_stage_guard_verify() {
    log_sep "STAGE 3 — GUARD VERIFICATION"

    local guard_script="${kernel_dir}/scripts/apply_ksu_guards.py"

    box_top "$ORG"
    box_ctr "$ORG" "$YEL" "HOOK  GUARD  VERIFICATION"
    box_rule "$ORG" "$DIM"
    box_row  "$ORG" "$GRY" "Dual-guard: CONFIG_KSU_SUSFS || CONFIG_KSU_MANUAL_HOOK"
    box_row  "$ORG" "$GRY" "Files: fs/{exec,open,stat,read_write}.c  kernel/{reboot,sys}.c"
    box_row  "$ORG" "$GRY" "       drivers/input/input.c  drivers/kernelsu/runtime/ksud_integration.c"
    box_bot  "$ORG"
    printf "\n"

    if [ ! -f "$guard_script" ]; then
        box_top "$YEL"
        box_ctr "$YEL" "$YEL" "Guard script not found — skipping verification"
        box_lbl "$YEL" "$GRY" "Expected" "scripts/apply_ksu_guards.py"
        box_bot "$YEL"
        GUARD_TOAST="Script not found"
        printf "\n"
        return
    fi

    local out
    out=$(TERM_W=9999 KERNEL_DIR="$kernel_dir" python3 "$guard_script" 2>&1)

    # ── Parse output into typed buckets ──────────────────────────────────────
    # Python format: "  [TAG] path:lineno | detail text"
    # We keep the FULL detail (no semicolon truncation).
    local -a fix_files fix_details ok_files ok_details skp_details
    local fixed_count=0 ok_count=0 skp_count=0

    while IFS= read -r l; do
        local bare="${l#  }"
        if [[ "$bare" == \[FIX\]* ]]; then
            local body="${bare#\[FIX\] }"
            local fp="${body%% |*}"
            local dt="${body#*| }"
            fix_files+=("$fp"); fix_details+=("$dt")
            fixed_count=$(( fixed_count + 1 ))
        elif [[ "$bare" == \[\ OK\]* ]]; then
            local body="${bare#\[\ OK\] }"
            local fp="${body%% |*}"
            local dt="${body#*| }"
            ok_files+=("$fp"); ok_details+=("$dt")
            ok_count=$(( ok_count + 1 ))
        elif [[ "$bare" == \[SKP\]* ]]; then
            skp_details+=("${bare#\[SKP\] }")
            skp_count=$(( skp_count + 1 ))
        fi
    done <<< "$out"

    local summary_str
    summary_str=$(printf '%s\n' "$out" | grep "=> Summary:" | sed 's/.*=> Summary: //')

    # Border colour: orange if fixes applied, green if all-ok, red on failure
    local bc
    if   [ "$fixed_count" -gt 0 ]; then bc="$ORG"
    elif [ -n "$summary_str"    ]; then bc="$LGR"
    else                                bc="$LRD"
    fi

    box_top "$bc"

    # ── APPLIED section ───────────────────────────────────────────────────────
    if [ "${#fix_files[@]}" -gt 0 ]; then
        box_ctr "$bc" "$ORG" "APPLIED"
        box_rule "$bc" "$DIM"
        local idx=0
        for fp in "${fix_files[@]}"; do
            _guard_entry_row "$bc" "$ORG" "$WHT" "$fp" "${fix_details[$idx]}"
            idx=$(( idx + 1 ))
        done
    fi

    # ── SKIPPED section ───────────────────────────────────────────────────────
    if [ "${#skp_details[@]}" -gt 0 ]; then
        [ "${#fix_files[@]}" -gt 0 ] && box_rule "$bc" "$DIM"
        box_ctr "$bc" "$GRY" "SKIPPED"
        box_rule "$bc" "$DIM"
        for sd in "${skp_details[@]}"; do
            box_row "$bc" "$GRY" "$sd"
        done
    fi

    # ── VERIFIED OK section ───────────────────────────────────────────────────
    if [ "${#ok_files[@]}" -gt 0 ]; then
        [ $(( ${#fix_files[@]} + ${#skp_details[@]} )) -gt 0 ] && box_rule "$bc" "$DIM"
        box_ctr "$bc" "$LGR" "VERIFIED  OK"
        box_rule "$bc" "$DIM"
        local idx=0
        for fp in "${ok_files[@]}"; do
            _guard_entry_row "$bc" "$DIM" "$GRY" "$fp" "${ok_details[$idx]}"
            idx=$(( idx + 1 ))
        done
    fi

    # ── Summary ───────────────────────────────────────────────────────────────
    box_rule "$bc" "$DIM"
    if [ -n "$summary_str" ]; then
        GUARD_TOAST="Guards: ${summary_str}"
        local fix_c; fix_c=$(printf '%s' "$summary_str" | grep -oP '^\d+')
        if [ "${fix_c:-0}" -gt 0 ]; then
            box_ctr "$bc" "$ORG" "Summary : ${summary_str}"
        else
            box_ctr "$bc" "$LGR" "Summary : ${summary_str}"
        fi
    else
        GUARD_TOAST="Guards: Verification failed"
        box_ctr "$bc" "$LRD" "Verification failed — check guard script"
    fi
    box_bot "$bc"
    printf "\n"
}

_apply_ksu_guards() {
    local guard_script="${kernel_dir}/scripts/apply_ksu_guards.py"
    if [ ! -f "$guard_script" ]; then
        printf "  ${YEL}● KSU guard fixer not found — skipping${NC}\n"
        GUARD_TOAST="Script not found"
        return
    fi
    local out
    out=$(TERM_W=$W KERNEL_DIR="$kernel_dir" python3 "$guard_script" 2>&1)
    
    while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done <<< "$out"
    
    local summary
    summary=$(echo "$out" | grep "=> Summary:" | sed 's/.*=> Summary: //')
    if [ -n "$summary" ]; then
        GUARD_TOAST="Guards: ${summary}"
    else
        GUARD_TOAST="Guards: Verification failed"
    fi
}

_do_resukisu_pull() {
    _winch_disable
    set_width
    do_clear

    local branch="$KSU_BRANCH"
    local setup_url="https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh"

    box_top "$ORG"
    box_ctr "$ORG" "$YEL" "UPDATE  ReSukiSU  DRIVER"
    box_div "$ORG"
    box_lbl "$ORG" "$WHT" "Source" "github.com/ReSukiSU/ReSukiSU"
    if [ "$branch" = "dev" ]; then
        box_lbl "$ORG" "$WHT" "Branch" "dev  (development)" "$YEL"
    else
        box_lbl "$ORG" "$WHT" "Branch" "main  (stable)" "$LGR"
    fi
    box_lbl "$ORG" "$GRY" "Target" "${kernel_dir}/drivers/kernelsu"
    box_bot "$ORG"
    printf "\n"

    log_sep "RUNNING SETUP.SH"

    local _rc_file; _rc_file=$(mktemp /tmp/vkb_upd_XXXXXX)
    local _ksu_dir="${kernel_dir}/drivers/kernelsu"
    if [ -d "$_ksu_dir" ] && [ ! -L "$_ksu_dir" ]; then
        printf "  ${YEL}[!] drivers/kernelsu is a real dir — moving to .bak for symlink creation${NC}\n"
        mv "$_ksu_dir" "${_ksu_dir}.bak"
    fi

    (
        cd "$kernel_dir" || exit 1
        bash <(curl -LSs "$setup_url") "$branch"
        echo "$?" > "$_rc_file"
    ) 2>&1 | while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done

    local UPDATE_EXIT; UPDATE_EXIT=$(cat "$_rc_file" 2>/dev/null); rm -f "$_rc_file"
    UPDATE_EXIT=$(( ${UPDATE_EXIT:-1} + 0 ))

    log_sep "VERIFY & PATCH"

    local ksu_c_count
    ksu_c_count=$(find -L "${kernel_dir}/drivers/kernelsu" -name "*.c" 2>/dev/null | wc -l)
    local kbuild_ok=false
    [ -f "${kernel_dir}/drivers/kernelsu/Kbuild" ] && kbuild_ok=true

    printf "\n"
    if [ "$UPDATE_EXIT" -eq 0 ] && [ "$ksu_c_count" -gt 5 ] && $kbuild_ok; then
        _LAST_PULL_STATUS=0
        local _int_link="${kernel_dir}/drivers/kernelsu/kernel"
        if [ -L "$_int_link" ] && [ ! -e "$_int_link" ]; then
            printf "  ${CYN}●${NC} Removed dangling kernel symlink\n"
            rm -f "$_int_link"
        fi
        local _out_ksu="${objdir}/drivers/kernelsu"
        if [ -d "$_out_ksu" ]; then
            printf "  ${CYN}●${NC} Cleared stale out/ objects\n"
            rm -rf "$_out_ksu"
        fi

        printf "  ${CYN}●${NC} Verifying and applying KSU hook guards:\n"
        _apply_ksu_guards

        log_sep "OUTPUT"
        box_top "$LGR"
        box_ctr "$LGR" "$LGR" "ReSukiSU driver updated successfully"
        box_rule "$LGR" "$DIM"
        box_lbl "$LGR" "$WHT" "Branch" "${branch}"
        box_lbl "$LGR" "$WHT" ".c files" "${ksu_c_count} found"
        box_lbl "$LGR" "$WHT" "Kbuild" "present"
        box_lbl "$LGR" "$YEL" "Note" "Full clean build required after branch switch"
        box_bot "$LGR"
    else
        _LAST_PULL_STATUS=1
        log_sep "OUTPUT"
        box_top "$LRD"
        box_ctr "$LRD" "$LRD" "Update failed or source tree incomplete"
        box_rule "$LRD" "$DIM"
        box_lbl "$LRD" "$WHT" "Branch" "${branch}"
        box_lbl "$LRD" "$WHT" ".c files" "${ksu_c_count}  (expect > 5)"
        box_lbl "$LRD" "$WHT" "Kbuild" "$( $kbuild_ok && echo YES || echo NO )"
        box_lbl "$LRD" "$YEL" "Note" "Check curl / network and retry"
        box_bot "$LRD"
    fi

    printf "\n"
    box_top "$ORG"
    box_menu "$ORG" "$WHT" "R" "Return" "" ""
    box_bot "$ORG"
    printf "\n${ORG}  Press [R] to return: ${NC}"

    while true; do
        IFS= read -r -s -n1 key2
        [ "$key2" = $'\033' ] && { IFS= read -r -s -t 0.05 -n5 _esc 2>/dev/null; _drain_input; continue; }
        _drain_input
        key2=$(printf '%s' "$key2" | tr 'ABCDEFGHIJKLMNOPQRSTUVWXYZ' 'abcdefghijklmnopqrstuvwxyz')
        [ "$key2" = "r" ] && break
        [ "$key2" = "" ] && continue
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
    if [ -n "$FORCE_CLEAN_REASON" ]; then
        INCREMENTAL=false
        box_top "$YEL"
        box_ctr "$YEL" "$YEL" "${FORCE_CLEAN_REASON} — forcing full clean build"
        box_bot "$YEL"
        printf "\n"
        FORCE_CLEAN_REASON=""
        _save_state
    fi
    if $INCREMENTAL; then
        box_top "$GRY"; box_ctr "$GRY" "$GRY" "Incremental — keeping previous objects"; box_bot "$GRY"
        printf "\n"
    else
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
        make -C "$kernel_dir" O="$objdir" ARCH=arm64 LLVM=1 LLVM_IAS=1 CC="$MAKE_CC" olddefconfig 2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        printf "\n"
    else
        box_top "$CYN"
        box_ctr "$CYN" "$WHT" "Applying: arch/arm64/configs/${CONFIG_FILE}"
        box_bot "$CYN"
        printf "\n"
        make -C "$kernel_dir" O="$objdir" ARCH=arm64 LLVM=1 LLVM_IAS=1 CC="$MAKE_CC" "$CONFIG_FILE" 2>&1 | grep -v "^-- " | \
            while IFS= read -r l; do printf "  ${GRY}%s${NC}\n" "$l"; done
        printf "\n"
    fi

    mkdir -p "$OUTPUT_DIR"
    local fail_log="${OUTPUT_DIR}/$(_fail_log_name)"

    if [ -n "$KERNEL_NAME" ] && [ -f "${objdir}/.config" ]; then
        sed -i 's/^CONFIG_LOCALVERSION=.*/CONFIG_LOCALVERSION=""/' "${objdir}/.config"
    fi

    # ── Stage 3: Guard Verification ─────────────────────────────────────────
    _stage_guard_verify

    # ── Stage 4: Compile ─────────────────────────────────────────────────────
    log_sep "STAGE 4 — COMPILE"
    box_top "$CYN"
    box_ctr "$CYN" "$CYN" "COMPILING  KERNEL"
    box_rule "$CYN" "$DIM"
    box_ctr "$CYN" "$GRY" "$(nproc --all) threads  │  $(date '+%H:%M')"
    box_bot "$CYN"
    printf "\n"

    local t0; t0=$(date +%s)

    local _fifo; _fifo=$(mktemp -u /tmp/vkb_XXXXXX)
    mkfifo "$_fifo"
    tee "$fail_log" < "$_fifo" &
    local _TEE_PID=$!

    _MAKE_PID=""; _CANCELLED=false
    trap '_build_sigint' INT

    local _make_localver=()
    local _scmver_created=false
    local _scmver_backup=""
    if [ -n "$KERNEL_NAME" ]; then
        _make_localver=("LOCALVERSION=-${KERNEL_NAME}" "CONFIG_LOCALVERSION=")
        _scmver_backup=$(cat "${kernel_dir}/.scmversion" 2>/dev/null || true)
        printf '' > "${kernel_dir}/.scmversion"
        _scmver_created=true
    fi

    setsid make -C "$kernel_dir" O="$objdir" ARCH=arm64 LLVM=1 LLVM_IAS=1 CC="$MAKE_CC" \
        CLANG_TRIPLE=aarch64-linux-gnu- CROSS_COMPILE=aarch64-linux-gnu- CROSS_COMPILE_ARM32=arm-linux-gnueabi- \
        "${_make_localver[@]}" -j"$(nproc --all)" > "$_fifo" 2>&1 &
    _MAKE_PID=$!
    wait "$_MAKE_PID"
    BUILD_EXIT=$?
    wait "$_TEE_PID" 2>/dev/null
    rm -f "$_fifo"

    if $_scmver_created; then
        if [ -n "$_scmver_backup" ]; then printf '%s' "$_scmver_backup" > "${kernel_dir}/.scmversion"; else rm -f "${kernel_dir}/.scmversion"; fi
    fi

    trap - INT

    local t1; t1=$(date +%s)
    local s=$(( t1 - t0 ))
    local elapsed
    [ "$s" -ge 60 ] && elapsed="$(( s/60 ))m $(( s%60 ))s" || elapsed="${s}s"

    if $_CANCELLED; then
        rm -f "$fail_log" 2>/dev/null || true
        print_cancelled_box "$elapsed"
        post_build_prompt; local _pbrc=$?
        if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        fi
        return
    fi

    if [ "$BUILD_EXIT" -ne 0 ] || [ ! -f "${objdir}/arch/arm64/boot/Image" ]; then
        print_fail_box "$fail_log"
        post_build_prompt; local _pbrc=$?
        if   [ "$_pbrc" -eq 1 ]; then INCREMENTAL=false; _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        elif [ "$_pbrc" -eq 2 ]; then INCREMENTAL=true;  _save_state; _SKIP_DEFCONFIG=$MENUCONFIG_USED; do_build
        fi
        return
    fi

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
        find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-CANCEL.log" -delete 2>/dev/null || true
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
# PACKAGE-ONLY
# =============================================================================
do_package_only() {
    _read_build_num
    _winch_disable
    set_width
    do_clear

    mkdir -p "$OUTPUT_DIR"
    box_top "$MAG"
    box_ctr "$MAG" "$WHT" "PACKAGE  EXISTING  IMAGES"
    box_div "$MAG"
    box_lbl "$MAG" "$WHT" "Image" "${objdir}/arch/arm64/boot/Image"
    box_lbl "$MAG" "$WHT" "Capabilities" "$(get_cap_str)"
    box_lbl "$MAG" "$WHT" "Features" "$(get_ext_feat_str)"
    box_bot "$MAG"
    printf "\n"

    if do_package; then
        _commit_build_num
        _save_prev_state
        _rm_old_success_log
        local slog_name; slog_name=$(_success_log_name "$BUILD_NUM")
        printf "[%s-Project] Package-only build #%s  %s\n" \
            "$PROJECT_NAME" "$BUILD_NUM" "$(date)" > "${OUTPUT_DIR}/${slog_name}"
        find "$OUTPUT_DIR" -maxdepth 1 -name "\[${PROJECT_NAME}-Project\]*-CANCEL.log" -delete 2>/dev/null || true
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

    _load_prev_state
    box_top "$GRY"
    box_ctr "$GRY" "$DIM" "Previous Build"
    box_div "$GRY"
    if [ -z "$PREV_NUM" ]; then
        box_ctr "$GRY" "$GRY" "No previous build recorded"
    else
        local pb_val="#${PREV_NUM}"
        [ -n "$PREV_DATE" ] && pb_val="${pb_val}   ${PREV_DATE}"
        box_lbl "$GRY" "$GRY" "Build" "$pb_val"
        [ -n "$PREV_KNAME" ] && box_lbl "$GRY" "$GRY" "Kernel-Name" "${PREV_KNAME}"
        box_lbl "$GRY" "$GRY" "Mode" "${PREV_MODE}"
        box_rule "$GRY" "$DIM"
        box_lbl "$GRY" "$GRY" "Capabilities" "${PREV_CAP:-${PREV_FEAT:-Unknown}}"
        box_lbl "$GRY" "$GRY" "Features" "${PREV_EXT_FEAT:-[None]}"
    fi
    box_bot "$GRY"

    box_top "$CYN"
    box_ctr "$CYN" "$YEL" "SELECT  MODE"
    box_div "$CYN"
    box_menu "$CYN" "$WHT" "B" "Full Build" "configure → compile → package" "$GRY"
    if $has_image; then
        box_menu "$CYN" "$LGR" "P" "Package Existing Image" "skip compile" "$GRY"
    else
        box_row "$CYN" "$GRY" "Package Only — no Image found in out/"
    fi
    box_rule "$CYN" "$DIM"
    if [ "$KSU_BRANCH" = "dev" ]; then
        box_menu "$CYN" "$WHT" "M" "ReSukiSU Driver Manager" "branch: dev" "$YEL"
    else
        box_menu "$CYN" "$WHT" "M" "ReSukiSU Driver Manager" "branch: main" "$LGR"
    fi
    box_rule "$CYN" "$DIM"
    box_menu "$CYN" "$GRY" "Q" "Quit" "" ""
    box_bot "$CYN"

    if $_PRESERVE_ACTIVE; then
        box_top "$MAG"
        box_ctr "$MAG" "$MAG" "Preserved menuconfig .config — will be restored on next build"
        box_bot "$MAG"
    fi

    printf '\033[J'
    _cursor_show
    if $has_image; then
        printf "\n${WHT}  Select [B/P/M/Q]: ${NC}"
    else
        printf "\n${WHT}  Select [B/M/Q]: ${NC}"
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
            m)  do_update_resukisu
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

    _do_build=false
    while true; do
        run_feat_menu || break
        run_build_menu && { _do_build=true; break; }
    done
    if ! $_do_build; then
        do_clear; read_features; continue
    fi
    do_clear
    KERNEL_NAME=""
    read_features
done
