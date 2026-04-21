#!/usr/bin/env python3
"""
Idempotent KSU hook guard fixer.
Run automatically after every ReSukiSU branch switch or update.

Ensures all ksu_handle_* call sites and extern declarations in kernel
source are wrapped with the dual guard. Fully supports multi-line C statements.
"""
import re, os, sys

_file_dir = os.path.dirname(os.path.abspath(__file__))
if os.path.basename(_file_dir) == 'scripts':
    _fallback = os.path.abspath(os.path.join(_file_dir, '..'))
else:
    _fallback = _file_dir

KERNEL_DIR = os.environ.get('KERNEL_DIR', _fallback)
TERM_W = int(os.environ.get('TERM_W', '80'))
MAX_TEXT = max(20, TERM_W - 35)

DUAL_GUARD  = '#if defined(CONFIG_KSU_SUSFS) || defined(CONFIG_KSU_MANUAL_HOOK)'
GUARD_CLOSE = '#endif'

_fixed = 0
_ok    = 0
_skip  = 0

def _log_fixed(path, lineno, text):
    global _fixed; _fixed += 1
    print(f'  [FIX] {path}:{lineno:<4} | {text[:MAX_TEXT].strip()}')

def _log_ok(path, lineno, text):
    global _ok; _ok += 1
    print(f'  [ OK] {path}:{lineno:<4} | {text[:MAX_TEXT].strip()}')

def _log_skip(path, reason):
    global _skip; _skip += 1
    print(f'  [SKP] {path: <15} | {reason}')

def wrap_bare_calls(rel_path):
    full_path = os.path.join(KERNEL_DIR, rel_path)
    if not os.path.isfile(full_path):
        _log_skip(rel_path, 'file not found')
        return

    with open(full_path, 'r') as f:
        lines = f.readlines()

    new_lines        = []
    changed          = False
    depth            = 0
    dual_depth       = -1
    pending_wrap     = False
    wrap_start_line  = 0

    for i, line in enumerate(lines):
        stripped = line.strip()

        # ── preprocessor open (#if / #ifdef / #ifndef) ────────────────────────
        if re.match(r'^#\s*if(?:def|ndef)?\b', stripped):
            # A ksu_handle call can never span across a #if boundary in valid C.
            # If we're mid-wrap here something already went wrong; close it
            # defensively so we don't emit an unterminated #if.
            if pending_wrap:
                dual_depth = -1
                depth      = max(0, depth - 1)
                new_lines.append(GUARD_CLOSE + '\n')
                _log_fixed(rel_path, wrap_start_line,
                           f'[defensive close before #if at line {i+1}]')
                pending_wrap = False
                changed = True
            depth += 1
            if stripped == DUAL_GUARD and dual_depth == -1:
                dual_depth = depth
            new_lines.append(line)
            continue

        # ── preprocessor close (#endif) ───────────────────────────────────────
        if re.match(r'^#\s*endif\b', stripped):
            # Same defensive close.
            if pending_wrap:
                dual_depth = -1
                depth      = max(0, depth - 1)
                new_lines.append(GUARD_CLOSE + '\n')
                _log_fixed(rel_path, wrap_start_line,
                           f'[defensive close before #endif at line {i+1}]')
                pending_wrap = False
                changed = True
            if dual_depth == depth:
                dual_depth = -1
            depth = max(0, depth - 1)
            new_lines.append(line)
            continue

        # ── #else / #elif are depth-neutral — pass straight through ───────────
        if re.match(r'^#\s*(else|elif)\b', stripped):
            new_lines.append(line)
            continue

        # ── content line ──────────────────────────────────────────────────────
        inside_dual = (dual_depth != -1)
        is_call   = (bool(re.search(r'\bksu_handle_\w+\s*\(', stripped))
                     and not stripped.startswith('//')
                     and not stripped.startswith('/*'))
        is_extern = ('extern' in stripped and 'ksu_handle_' in stripped
                     and not stripped.startswith('//')
                     and not stripped.startswith('/*'))

        if not inside_dual and (is_call or is_extern or pending_wrap):
            if not pending_wrap:
                pending_wrap    = True
                wrap_start_line = i + 1
                # Inject the guard and immediately track it in depth/dual_depth
                # so that a second run sees it as already-guarded (idempotency).
                new_lines.append(DUAL_GUARD + '\n')
                depth     += 1
                dual_depth = depth

            new_lines.append(line)

            if stripped.endswith(';') or stripped.endswith('}'):
                # Un-track before writing #endif so depth stays balanced.
                dual_depth = -1
                depth      = max(0, depth - 1)
                new_lines.append(GUARD_CLOSE + '\n')
                pending_wrap = False
                if is_call or is_extern:
                    _log_fixed(rel_path, wrap_start_line, stripped)
                else:
                    _log_fixed(rel_path, wrap_start_line,
                               f'Multi-line block ending at line {i+1}')
                changed = True
            continue

        if is_call or is_extern:
            _log_ok(rel_path, i + 1, stripped)
        new_lines.append(line)

    # Safety net: pending_wrap survived to EOF (shouldn't happen in valid C).
    if pending_wrap:
        dual_depth = -1
        depth      = max(0, depth - 1)
        new_lines.append(GUARD_CLOSE + '\n')
        _log_fixed(rel_path, wrap_start_line, '[EOF safety close]')
        changed = True

    if changed:
        with open(full_path, 'w') as f:
            f.writelines(new_lines)

def fix_ksud_integration():
    rel  = 'drivers/kernelsu/runtime/ksud_integration.c'
    full = os.path.join(KERNEL_DIR, rel)
    if not os.path.isfile(full):
        _log_skip(rel, 'file not found (driver not installed)')
        return

    with open(full, 'r', encoding='utf-8', errors='ignore') as f:
        lines = f.readlines()

    new_guard = '#if defined(CONFIG_KSU_MANUAL_HOOK) || defined(CONFIG_KSU_SUSFS)\n'
    
    for i, line in enumerate(lines):
        if line.strip() == new_guard.strip():
            _log_ok(rel, i + 1, 'ksud_integration dual-guard already applied')
            return

    changed = False
    for i, line in enumerate(lines):
        if line.strip() == '#ifdef CONFIG_KSU_MANUAL_HOOK':
            lookahead = "".join(lines[i:i+4])
            if 'KernelSU' in lookahead or 'ksu_handle_newfstat_ret' in lookahead:
                lines[i] = new_guard
                _log_fixed(rel, i + 1, 'ksud_integration guard -> dual')
                changed = True
                break

    if changed:
        with open(full, 'w', encoding='utf-8') as f:
            f.writelines(lines)
    else:
        _log_skip(rel, 'ksud_integration target block not found')


SOURCE_FILES = [
    'fs/exec.c',
    'fs/open.c',
    'fs/stat.c',
    'fs/read_write.c',
    'kernel/reboot.c',
    'drivers/input/input.c',
    'kernel/sys.c',
]

for rel in SOURCE_FILES:
    wrap_bare_calls(rel)

fix_ksud_integration()

print(f'\n  => Summary: {_fixed} fixed, {_ok} already-ok, {_skip} skipped')
sys.exit(0)

