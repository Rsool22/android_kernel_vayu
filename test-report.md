# vayu-builder PR #3 — test report

PR: https://github.com/Rsool22/android_kernel_vayu/pull/3
Session: https://app.devin.ai/sessions/51923521caec496c80346a58865bcbcf
Binary under test: `/tmp/vayu-builder` (built locally from `devin/build-overhaul`)

All 6 planned tests passed. The three things you asked to be fixed (Google clang as primary, name-stable autodiscovery, robust ReSukiSU branch probe) are observably working both from the CLI and inside the Bubble Tea TUI.

## Summary

| # | Test | Result |
|---|---|---|
| T1 | `probe --json` returns concrete SHAs for both branches that match `git ls-remote` | passed |
| T2 | Underlying `git ls-remote --exit-code` returns `2` for missing refs (proves the prober's signal is real) | passed |
| T3 | `paths --json` resolves the kernel root through a renamed/symlinked dir | passed |
| T4 | `fetch-clang --check --source google` resolves a `clang-r*` revision via gitiles | passed |
| T5 | `fetch-clang --check --source zyc` still works as fallback | passed |
| T6 | TUI walkthrough: all 5 screens render, hotkeys transition correctly, exit clean | passed |

## CI status note

PR #3 currently shows **0 checks** because `build-dual.yml` and `release-builder-tui.yml` are gated on `push` to `16-ReSukiSU` and the daily schedule, not on `pull_request`. This is intentional — every PR push would otherwise consume up to 1 h of Actions time. The release workflow will fire on merge.

## Evidence

### T1 — probe --json
```
$ /tmp/vayu-builder probe --json
{"dev":{"sha":"b6ec6e0815d8f1234da5afdbf92283867756b286","state":"present"},
 "main":{"sha":"35439793ef7c4960bc57463d4b274f66921fd1c7","state":"present"}}
exit=0

$ git ls-remote https://github.com/ReSukiSU/ReSukiSU refs/heads/main refs/heads/dev
b6ec6e0815d8f1234da5afdbf92283867756b286  refs/heads/dev
35439793ef7c4960bc57463d4b274f66921fd1c7  refs/heads/main
```
SHAs from `vayu-builder probe` exactly match `git ls-remote`.

### T2 — `--exit-code` behaviour
```
$ git ls-remote --exit-code https://github.com/ReSukiSU/ReSukiSU refs/heads/__nonexistent__
exit=2
```
Confirms the prober's `case 2 → StateAbsent` is wired to a real signal.

### T3 — name-stable autodiscovery
```
$ ln -sfT /home/ubuntu/repos/android_kernel_vayu /tmp/some-other-name
$ cd /tmp/some-other-name
$ /tmp/vayu-builder paths --json
{"Kernel":"/tmp/some-other-name","Distro":"apt","Output":"/tmp/some-other-name/out", ...}
exit=0
```
The kernel root is correctly identified by markers even though the directory is now called `some-other-name`. A hardcoded-path implementation (the bug we replaced) would have failed this.

### T4 — Google AOSP gitiles
```
$ /tmp/vayu-builder fetch-clang --check --source google
[fetch-clang] google : clang-r596125
clang-r596125
exit=0
```
Live resolution off `android.googlesource.com/.../linux-x86?format=JSON` on the `main-kernel` branch.

### T5 — ZyC fallback
```
$ /tmp/vayu-builder fetch-clang --check --source zyc
[fetch-clang] zyc : 15.0.7-20260208-release
15.0.7-20260208-release
exit=0
```

### T6 — visual walkthrough

| 🟢 Main menu | 🟢 Toolchain Manager |
|---|---|
| ![main](attachment:main.png) | ![toolchain](attachment:toolchain.png) |
| Banner + status panel + 5 hotkeys (B/T/K/S/Q) | Auto / Google / ZyC + Fetch / Check actions |

| 🟢 Toolchain → Check (Google) | 🟢 ReSukiSU Driver Manager |
|---|---|
| ![check](attachment:toolchain-check.png) | ![ksu](attachment:ksu.png) |
| Live `clang-r596125` returned from gitiles | main=present 35439793, dev=present b6ec6e08 |

| 🟢 Setup / Paths | 🟢 Build screen |
|---|---|
| ![setup](attachment:setup.png) | ![build](attachment:build.png) |
| All paths autodiscovered, `apt` install command rendered | "Press [B] to start build" + viewport ready |

(Full recording attached.)

## Out of scope (explicit)

- Actual kernel compilation. The build screen was opened but `[B]` was not pressed — a real compile takes ~30 min and is deferred to CI on merge.
- AnyKernel3 zip packaging. Same reason as above.
- `release-builder-tui.yml` cross-compile job. Doesn't trigger on pull_request; will run when the PR merges to `16-ReSukiSU`.
