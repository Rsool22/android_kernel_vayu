# vayu-builder PR #3 — test plan

PR: https://github.com/Rsool22/android_kernel_vayu/pull/3
Local binary under test: `/tmp/vayu-builder` (built from this branch with `go build`)

## What the PR claims to fix

1. Google AOSP clang is the new primary toolchain; ZyC is fallback.
2. Path autodiscovery works regardless of directory name (kernel root, clang, AnyKernel3).
3. ReSukiSU branch probe distinguishes `present` / `absent` / `network_fail` so CI no longer silently skips when `dev` is gone.

## Tests

### T1 — ReSukiSU dual-branch probe returns concrete SHAs (refutes the pre-rewrite "always-empty" bug)

`/tmp/vayu-builder probe --json`

* **Pass**: stdout is valid JSON; both `.main.state` and `.dev.state` are `present`; both `.main.sha` and `.dev.sha` are 40-char hex strings; exit code is `0`.
* **Fail** if either state is `absent` while the branch actually exists on github.com/ReSukiSU/ReSukiSU, or if either SHA is empty when state is `present`, or if exit code != 0.
* **Adversarial**: a broken implementation that returned the same JSON regardless of git output would not pass, because we cross-check the SHAs against `git ls-remote https://github.com/ReSukiSU/ReSukiSU refs/heads/main refs/heads/dev` outside the binary and require equality.

Cross-check command: `git ls-remote https://github.com/ReSukiSU/ReSukiSU refs/heads/{main,dev}`

### T2 — `probe` returns exit 2 when the branch genuinely doesn't exist

`/tmp/vayu-builder probe-test-only` is not exposed; instead, run a forced negative inside a subshell by pointing the prober at a known-bad ref via env override. Because the upstream URL is hard-coded in `internal/resukisu/resukisu.go` (line 1 — `repoURL = "https://github.com/ReSukiSU/ReSukiSU"`), there is no env knob to redirect it. We exercise the absent path differently:

`git ls-remote --exit-code https://github.com/ReSukiSU/ReSukiSU refs/heads/__nonexistent__ ; echo $?`

* **Pass**: exit code is `2`. This proves the underlying mechanism (`--exit-code` returning 2 for missing refs) the prober relies on actually behaves that way on this network and git version, so the prober's `case 2 -> StateAbsent` branch is wired to a real signal.
* **Fail** if exit code is anything other than 2.

### T3 — Path autodiscovery survives directory rename

```
ln -sfT /home/ubuntu/repos/android_kernel_vayu /tmp/some-other-name
cd /tmp/some-other-name
/tmp/vayu-builder paths --json
```

* **Pass**: the `Kernel` field in the JSON points to the kernel root (resolves to `/home/ubuntu/repos/android_kernel_vayu` via realpath) and `Distro` is `apt`. Even though the working directory is `/tmp/some-other-name`, the resolver walks up by markers and identifies the kernel.
* **Fail** if `Kernel` is empty, points to `/tmp/some-other-name` literally without resolving, or is missing.
* **Adversarial**: a hardcoded-path implementation (the bug we're fixing) would either return the literal `/tmp/some-other-name` (no realpath resolution) or return an empty kernel root because the dir name doesn't match the old hardcoded `vayu_a16_kernel`.

### T4 — Google AOSP gitiles clang resolution returns a real `clang-r*` revision

`/tmp/vayu-builder fetch-clang --check --source google`

* **Pass**: stdout contains `clang-r` followed by digits, the resolved `URL` ends in `.tar.gz`, `Source: google`, exit 0. We do **not** download — `--check` resolves only.
* **Fail** if the output mentions ZyC or doesn't contain a `clang-r` revision.
* **Adversarial**: stripping the gitiles XSSI prefix (`)]}'`) is the most fragile part. A broken parser would either fail to parse JSON or list 0 revisions, producing an error instead of a `clang-r*` line.

### T5 — ZyC fallback still works (`--source zyc`)

`/tmp/vayu-builder fetch-clang --check --source zyc`

* **Pass**: stdout shows a `Tag` like `15.0.7-...` or `19.x.x-...`, `Source: zyc`, `URL` ends with `.tar.gz` or `.tar.zst`, exit 0.
* **Fail** if exit non-zero or the URL points to gitiles.

### T6 — Visual TUI walkthrough (recorded)

Launch `/tmp/vayu-builder` interactively in a maximized terminal and walk through each main-menu screen:

* Main menu renders the orange/cyan banner and four hotkey lines (B/T/K/S/Q).
* Press `T` → Toolchain Manager renders A/G/Z source selector + F/C hotkeys.
* Press `Esc` then `K` → ReSukiSU Manager shows main+dev with `present` state and a SHA prefix on each.
* Press `Esc` then `S` → Setup screen lists kernel/clang/anykernel/distro paths.
* Press `Esc` then `B` → Build screen renders viewport + "Press [B]" prompt (we do NOT trigger an actual kernel compile — that takes ~30 min).
* Press `Esc` then `Q` → exit cleanly with no panic.

* **Pass**: every screen renders without garbage, no Bubble Tea panic on resize/keypress, exit code 0.
* **Fail**: any panic, blank screen, or hotkey that doesn't transition to the right screen.

## Out of scope

- Actual kernel compilation (T6 stops short of triggering build). Compilation is a known long path; CI will exercise it on merge.
- AnyKernel3 zip packaging — same reason.
- The `release-builder-tui.yml` cross-compile job — it doesn't trigger on the PR and we can't run an Actions workflow locally; will exercise on merge to `16-ReSukiSU`.

## CI status note

PR #3 currently shows 0 checks because both new workflows (`build-dual.yml`, `release-builder-tui.yml`) are gated on `push` to `16-ReSukiSU` (and the daily schedule), not on `pull_request`. This is intentional — we don't want every PR push to consume an hour of Actions time on a kernel build. Mentioned in the report.
