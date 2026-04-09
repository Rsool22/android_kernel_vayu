# VAYU Kernel Builder
### ReSukiSU · SUSFS · KPM — AnyMore Project

> An interactive TUI build script for the **Xiaomi Poco X3 Pro (vayu)** running a Linux 4.14 NonGKI kernel targeting Android 16.
> Handles everything from the first `git clone` through to a flashable AnyKernel3 zip — dependencies, toolchain management, driver setup, defconfig patching, compilation, and packaging — all from a single script.

---

## Table of Contents

- [Download from Release](#download-from-release)
- [Build Requirements](#build-requirements)
- [Quick Start](#quick-start)
- [Directory Layout](#directory-layout)
- [First-Time Setup](#first-time-setup)
  - [1. Install Dependencies](#1-install-dependencies)
  - [2. Download ZyC Clang](#2-download-zyc-clang)
  - [3. Install the ReSukiSU Driver](#3-install-the-resukisu-driver)
  - [4. Clone AnyKernel3](#4-clone-anykernel3)
- [How the Script Works](#how-the-script-works)
  - [Startup & State Loading](#startup--state-loading)
  - [Main Menu](#main-menu)
  - [Feature Configuration](#feature-configuration)
  - [Build Options](#build-options)
  - [Build Internals — Five Stages](#build-internals--five-stages)
  - [Toolchain Manager](#toolchain-manager)
  - [ReSukiSU Driver Manager](#resukisu-driver-manager)
  - [Setup Menu](#setup-menu)
  - [Menuconfig Workflow](#menuconfig-workflow)
- [Output Files](#output-files)
- [State & Config Files](#state--config-files)
- [GitHub Actions CI Build](#github-actions-ci-build)
  - [What the Workflow Does](#what-the-workflow-does)
  - [Job Breakdown](#job-breakdown)
  - [Repository Setup for GitHub Builds](#repository-setup-for-github-builds)
  - [Triggering a Build](#triggering-a-build)
  - [Build Artifacts & Releases](#build-artifacts--releases)
  - [How the CI Build Script Works](#how-the-ci-build-script-works)
  - [How the Clang Fetcher Works](#how-the-clang-fetcher-works)
- [Troubleshooting](#troubleshooting)
- [Reference Links](#reference-links)

---

## Download from Release

Pre-built flashable zips are published automatically by the GitHub Actions CI workflow after every successful build. No compilation required — just download and flash via [OrangeFox Recovery](https://orangefox.download/device/61310755bb6a91af6a656d0d) or any AnyKernel3-compatible recovery.

**[→ Download Latest Release](https://github.com/Rsool22/android_kernel_xiaomi_vayu/releases/latest)**

> The release page always contains exactly one release (rolling `latest` strategy). The previous release is replaced on every successful CI run, so the link above always points to the most recent build.

### Available Files

| File | Description |
|------|-------------|
| `*MAIN*.zip` | Kernel zip — stable ReSukiSU (`main` branch) + SUSFS + KPM |
| `*DEV*.zip` | Kernel zip — latest ReSukiSU (`dev` branch) + SUSFS + KPM |
| `ReSukiSU-Manager-MAIN.zip` | Standard Manager APK (MAIN branch) |
| `ReSukiSU-Spoofed-Manager-MAIN.zip` | GMS-spoofed Manager APK (MAIN branch) |
| `ReSukiSU-Manager-DEV.zip` | Standard Manager APK (DEV branch) |
| `ReSukiSU-Spoofed-Manager-DEV.zip` | GMS-spoofed Manager APK (DEV branch) |

> **Which Manager APK should I use?** Use the **standard** variant unless your device or ROM fails SafetyNet / Play Integrity checks, in which case try the **Spoofed** variant. Always match the Manager APK branch (`MAIN`/`DEV`) to the kernel zip you flashed.

### Flash Instructions

1. Download the kernel zip (`MAIN` or `DEV`) from the release page.
2. Boot into **OrangeFox Recovery** (or any TWRP-based recovery that supports AnyKernel3).
3. Flash the zip.
4. Reboot to system.
5. Install the matching **ReSukiSU Manager APK**.
6. Grant root on first launch.

---

## Build Requirements

| Item | Minimum |
|------|---------|
| OS | Linux (Ubuntu 22.04+ recommended) |
| Shell | Bash 4.x+ |
| RAM | 8 GB (16 GB recommended) |
| Disk | 30 GB free — kernel source + toolchain + `out/` |
| Arch | x86\_64 host |
| Python | python3 (for GitHub API queries) |

---

## Quick Start

```bash
# Clone your kernel source (if not already done)
git clone https://github.com/Rsool22/android_kernel_xiaomi_vayu \
    ~/kernel-builds/vayu_a16_kernel
cd ~/kernel-builds/vayu_a16_kernel

# Clone AnyKernel3 alongside it
git clone https://github.com/osm0sis/AnyKernel3 \
    ~/kernel-builds/vayu_a16_kernel/AnyKernel3

# Make the script executable and launch
chmod +x build.sh
./build.sh
```

On first run without a Clang installation present, the main menu will display a red warning and block the **[B] Build** option. Use **[T] Toolchain Manager** to download Clang before attempting a build.

---

## Directory Layout

The script uses these default paths, all of which are configurable via **[S] Setup → [P] Paths**:

```
~/kernel-builds/vayu_a16_kernel/          ← kernel_dir  (kernel source root)
├── build.sh                              ← this script
├── arch/arm64/configs/vayu_defconfig     ← active defconfig
├── drivers/kernelsu/                     ← ReSukiSU driver (installed by [M])
├── clang/                                ← CLANG_DIR (ZyC Clang toolchain)
├── AnyKernel3/                           ← anykernel  (packaging template)
├── out/                                  ← objdir     (build output)
│   └── arch/arm64/boot/
│       ├── Image
│       ├── dtb.img
│       └── dtbo.img
└── Anykernel-Builds/                     ← OUTPUT_DIR (final zips + logs)

~/.config/vayu_builder/config             ← persisted path overrides
<kernel_dir>/.builder_state               ← persisted session state
```

---

## First-Time Setup

### 1. Install Dependencies

Launch the script and navigate to **[S] Setup → [D] Check Dependencies**.

The dependency checker automatically detects your package manager (`apt` / `dnf` / `pacman` / `zypper` / `apk`), reports the status of every required and optional package, and can install all missing ones automatically when you press **[I]**.

**Required packages:**

| Package | Purpose |
|---------|---------|
| `make` | Build system |
| `zip` | AnyKernel3 packaging |
| `curl` | Toolchain and driver downloads |
| `git` | ReSukiSU driver setup |
| `python3` | GitHub API JSON parsing |
| `bc`, `flex`, `bison`, `perl` | Kernel build prerequisites |
| `libssl-dev` | Kernel crypto headers |
| `libelf-dev` | Kernel BTF/ELF support |
| `gcc-aarch64-linux-gnu` | GCC 64-bit cross-compiler |
| `gcc-arm-linux-gnueabi` | GCC 32-bit cross-compiler |

**Optional:**

| Package | Purpose |
|---------|---------|
| `ccache` | Compiler cache — dramatically speeds up incremental rebuilds |

**Ubuntu/Debian one-liner:**
```bash
sudo apt-get install -y make zip curl git python3 bc flex bison perl \
    libssl-dev libelf-dev gcc-aarch64-linux-gnu gcc-arm-linux-gnueabi ccache
```

**Arch Linux:**
```bash
sudo pacman -S --needed base-devel aarch64-linux-gnu-gcc arm-linux-gnueabi-gcc \
    bc flex bison perl openssl libelf ccache zip curl git python
```

---

### 2. Download ZyC Clang

From the main menu, press **[T] Toolchain Manager**, then:

1. Select your target version: `23` for ZyC Clang 23.x (**recommended**), `15` for 15.x, or `L` for the absolute latest release.
2. Press **[C]** to check the latest release information without downloading anything.
3. Press **[F]** to fetch and install — the script queries the [ZyCromerZ/Clang](https://github.com/ZyCromerZ/Clang) GitHub API, shows the download size, asks for confirmation, then downloads with a live progress bar and extracts the toolchain to `clang/`.

The toolchain archive is roughly 300–500 MB. Extraction and installation are fully automatic; no manual steps are needed after pressing **[F]**.

> **Note:** GitHub's unauthenticated API allows 60 requests per hour per IP. If you hit the rate limit, wait a minute and retry, or download the tarball manually from [ZyCromerZ/Clang/releases](https://github.com/ZyCromerZ/Clang/releases) and extract it to `CLANG_DIR`.

---

### 3. Install the ReSukiSU Driver

From the main menu, press **[M] ReSukiSU Driver Manager**, then press **[I] Install Driver**.

The script runs the official `setup.sh` from [ReSukiSU/ReSukiSU](https://github.com/ReSukiSU/ReSukiSU) against your kernel source tree. This places the driver source at `drivers/kernelsu/` and wires it into the kernel's Kconfig and Makefile automatically. You can select the target branch (`main` = stable, `dev` = latest features) from the same menu.

> ⚠️ A **full clean build is required** after switching branches or reinstalling the driver. The script enforces this automatically by locking incremental mode.

---

### 4. Clone AnyKernel3

The script packages the compiled kernel using [AnyKernel3](https://github.com/osm0sis/AnyKernel3). The repo may already include it, but if not, clone it to the default location or set a custom path via **[S] Setup → [P] Paths → [3]**:

```bash
git clone https://github.com/osm0sis/AnyKernel3 \
    ~/kernel-builds/vayu_a16_kernel/AnyKernel3
```

Make sure the `anykernel.sh` inside the clone is configured for vayu before building.

---

## How the Script Works

This section explains the internals of `build.sh` so you understand what is happening at every stage — not just what buttons to press.

### Startup & State Loading

When `build.sh` launches, it does the following before drawing any UI:

1. **Loads the path config** from `~/.config/vayu_builder/config` if it exists. This file is written whenever you save changes in the Path Configurator and lets your custom paths survive script restarts.
2. **Loads the session state** from `<kernel_dir>/.builder_state`. This restores the incremental/full-clean toggle, ccache setting, active ReSukiSU branch, and whether a forced-clean reason is pending.
3. **Checks for a preserved menuconfig** at `<kernel_dir>/.menuconfig_saved_config`. If found, a flag is set so the main menu can show a magenta notice.
4. **Sets up the environment** — exports `ARCH=arm64`, `KBUILD_BUILD_USER`, `KBUILD_BUILD_HOST`, and prepends the Clang and GCC paths to `$PATH`.
5. **Reads the build counter** from `<kernel_dir>/.build_number` to know what build number the next successful zip will receive.
6. **Performs soft checks** — determines whether Clang and AnyKernel3 directories exist, storing the results in `_CLANG_MISSING` and `_ANYKERNEL_MISSING` flags. These gate the build option in the UI without hard-exiting.

---

### Main Menu

The main menu is the top-level control panel. It redraws itself whenever the terminal is resized (using a `SIGWINCH` trap) and shows a **Previous Build** info box at the top with the last build's number, date, mode, and feature set.

```
[B]  Full Build             → configure features → compile → package
[P]  Package Existing Image → skip compilation, re-package out/ artifacts only
[T]  Toolchain Manager      → fetch/update ZyC Clang
[M]  ReSukiSU Driver Mgr    → install/update/switch driver branch
[S]  Setup                  → paths + dependency checker
[Q]  Quit
```

- **[B]** is blocked with a red warning if Clang or AnyKernel3 paths are missing.
- **[P]** only appears when `out/arch/arm64/boot/Image` already exists from a previous compile.
- A red warning box lists any missing paths with a suggestion to use **[T]** or **[S]**.
- A magenta notice box appears if a preserved menuconfig `.config` is active.

The menu reads a single keypress (no Enter required) and dispatches immediately.

---

### Feature Configuration

Reached from **[B] Full Build**. This screen lets you toggle the three feature flags before any compilation begins.

```
[1]  Toggle ReSukiSU   → enables CONFIG_KSU; sets hook mode automatically
[2]  Toggle SuSFS      → CONFIG_KSU_SUSFS; switches hook mode if toggled
[3]  Toggle KPM        → CONFIG_KPM; requires ReSukiSU to be enabled
[M]  menuconfig        → opens interactive kernel config editor
[C]  Continue to Build → proceed with current settings
[B]  Back              → return to main menu
```

**Hook mode rules — enforced automatically:**

| ReSukiSU | SuSFS | Result |
|----------|-------|--------|
| ON | ON | SuSFS Inline-Hook mode — `CONFIG_KSU_SUSFS=y`, `CONFIG_KSU_MANUAL_HOOK` unset |
| ON | OFF | Manual-Hook mode — `CONFIG_KSU_MANUAL_HOOK=y`, `CONFIG_KSU_SUSFS` unset |
| OFF | — | Vanilla kernel — both flags unset |

> `CONFIG_KSU_SUSFS` and `CONFIG_KSU_MANUAL_HOOK` are **mutually exclusive** on Linux 4.14 NonGKI. The script enforces this constraint automatically and never enables both simultaneously.

All toggles write directly to `arch/arm64/configs/vayu_defconfig` using in-place `sed` substitutions before `make defconfig` is run.

---

### Build Options

The second configuration screen in the full build flow.

```
[N]  Set Kernel Name     → optional LOCALVERSION suffix (e.g. AnyMore-v2.1)
                           result: uname -r shows 4.14.356-AnyMore-v2.1
[I]  Incremental Build   → ON keeps previous out/ objects; OFF runs mrproper first
[C]  ccache              → toggle compiler cache (greyed if ccache not installed)
[S]  Start Build         → begin compilation
[X]  Reset Build Counter → resets zip/log counter and out/.version to #1
[B]  Back                → return to Feature Configuration
[Q]  Quit
```

**Incremental lock:** if you switch the ReSukiSU branch or change the hook mode between builds, the script automatically locks incremental mode off and forces a full clean build on the next run. The lock clears automatically after a successful clean build completes.

---

### Build Internals — Five Stages

Once **[S] Start Build** is confirmed, the script runs five sequential stages, printing a clearly labelled status line for each:

| Stage | What happens |
|-------|-------------|
| **1 – Clean** | Runs `make mrproper` to wipe the `out/` directory. Skipped entirely in incremental mode. |
| **2 – Defconfig** | Runs `make vayu_defconfig` to generate `out/.config` from your defconfig. Skipped if a menuconfig session already produced a fresh `.config` this run. |
| **3 – Compile** | Runs `make` with all threads (`nproc`) using ZyC Clang, LLVM/LLD, and optionally ccache. Output is streamed live and also written to a log file for error extraction. |
| **4 – Verify** | Checks that `out/arch/arm64/boot/Image` was produced. On failure, extracts and prints the most relevant linker and compiler error lines inline rather than making you scroll through the raw log. |
| **5 – Package** | Copies `Image`, `dtb.img`, and `dtbo.img` into the AnyKernel3 directory, runs `zip -r9` to create the flashable zip, and moves it to `OUTPUT_DIR`. |

**Make flags used:**
```
ARCH=arm64
LLVM=1
LLVM_IAS=1
CC=clang  (or "ccache clang" when ccache is enabled)
CROSS_COMPILE=aarch64-linux-gnu-
CROSS_COMPILE_ARM32=arm-linux-gnueabi-
O=<out/>
```

**Build identity baked into the kernel:**
```
KBUILD_BUILD_USER=OmegaR01
KBUILD_BUILD_HOST=Vayu
```

**Ctrl+C handling:** if you interrupt compilation with Ctrl+C, the script catches the signal via a `mkfifo`-based PID tracker, kills the `make` process cleanly, writes a CANCEL log to `OUTPUT_DIR`, and returns you to the post-build prompt instead of leaving orphan processes or a corrupted `out/`.

---

### Toolchain Manager

Accessed from the main menu via **[T]**.

```
[F]   Fetch Selected Version   → download + install to CLANG_DIR
[C]   Check Latest Release     → API query only, no download
[23]  Target ZyC Clang 23.x    ← recommended for this kernel
[15]  Target ZyC Clang 15.x
[L]   Latest (any version)
[R]   Return
```

The fetch process:
1. Queries the ZyCromerZ/Clang GitHub Releases API for a `.tar.gz` asset matching the selected version prefix.
2. Shows the release tag, asset filename, and download size, then asks for confirmation.
3. Downloads with a live MB progress bar using `curl`.
4. Extracts to a temp directory, verifies `bin/clang` is present, then moves the result to `CLANG_DIR`.
5. If an existing toolchain is already in `CLANG_DIR`, it is replaced after confirmation.

---

### ReSukiSU Driver Manager

Accessed from the main menu via **[M]**.

```
[U]  Update Driver     → pull latest commits from the active branch (checks remote SHA first)
[I]  Install Driver    → first-time setup via ReSukiSU's setup.sh (shown when driver is absent)
[S]  Switch Branch     → toggles main ↔ dev, pulls immediately, forces a clean build
[V]  Verify Guards     → checks that all KSU/SuSFS hook-guard #ifdefs are correct in source
[X]  Remove Driver     → runs setup.sh --cleanup and removes drivers/kernelsu/
[R]  Return
```

- The active branch (`main` / `dev`) is stored in `.builder_state` and survives restarts.
- **[V] Verify Guards** checks all 9 manual hook call sites in the kernel source and confirms each is correctly wrapped in `#if defined(CONFIG_KSU_MANUAL_HOOK)`. This is the guard pattern required on 4.14 NonGKI so the same source tree can build in either Manual-Hook or SuSFS-Inline-Hook mode without source changes.
- **[S] Switch Branch** automatically pulls the new branch and locks incremental mode to guarantee a clean rebuild, since the driver source has changed.
- **[U] Update Driver** compares the current local HEAD SHA against the remote branch SHA before pulling — if they already match it reports "already up to date" without running git.

---

### Setup Menu

Accessed from the main menu via **[S]**.

```
[P]  Configure Paths    → edit all directory paths
[D]  Check Dependencies → detect and optionally install missing packages
[R]  Return
```

#### Path Configurator

```
[1]  Kernel Dir    default: ~/kernel-builds/vayu_a16_kernel
[2]  Clang Dir     default: <kernel_dir>/clang
[3]  AnyKernel3    default: <kernel_dir>/AnyKernel3
[4]  Output Dir    default: <kernel_dir>/Anykernel-Builds
[5]  GCC64 Dir     default: /usr/bin  (aarch64-linux-gnu-* binaries)
[6]  GCC32 Dir     default: /usr/bin  (arm-linux-gnueabi-* binaries)
[S]  Save & Apply  → writes to ~/.config/vayu_builder/config, applies immediately
[R]  Return        → discard unsaved changes
```

All paths support tilde (`~`) expansion. Changes take effect immediately after **[S]** without restarting the script, because `_recompute_paths` re-derives all dependent variables and updates `$PATH` in-place.

#### Dependency Checker

Scans for every required and optional package and shows OK / MISSING status for each. Press **[I]** to run the appropriate install command for your detected package manager automatically.

Supported managers: `apt`, `dnf`, `yum`, `pacman`, `zypper`, `apk`.

---

### Menuconfig Workflow

Press **[M]** in the Feature Configuration screen to open the standard kernel `menuconfig` TUI before building.

After you close menuconfig, the script detects whether a `.config` was saved and presents three options in the post-menuconfig prompt:

| Key | Action |
|-----|--------|
| **[V]** | Preserve the `.config` for the next `./build.sh` run — saved to `.menuconfig_saved_config` and auto-restored on relaunch. A magenta notice on the main menu reminds you it is active. |
| **[D]** | Write changes permanently back to `vayu_defconfig` via `make savedefconfig` — this becomes the new default for all future builds. |
| **[T] / [I]** | Proceed to build immediately — defconfig generation is automatically skipped and the menuconfig `.config` is used as-is. |

> To cancel a preserved config, delete `<kernel_dir>/.menuconfig_saved_config` manually.

---

## Output Files

All final outputs land in `OUTPUT_DIR` (`Anykernel-Builds/` by default):

```
Anykernel-Builds/
├── [VAYU-AnyMore-Project]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-(2025-01-01)-{Build-#3}.zip
├── [VAYU-AnyMore-Project]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-(2025-01-01)-Build-#3.log
└── [VAYU-AnyMore-Project]-FAIL.log    ← only on failure; replaced on next success
```

- Only the **most recent** zip and success log are kept. Previous zips and logs are deleted automatically on each successful build.
- The build counter increments only on successful packaging — failed or cancelled builds do not advance the counter.
- CANCEL logs are written when Ctrl+C interrupts compilation. They are cleaned up on the next successful build.

---

## State & Config Files

| File | Purpose |
|------|---------|
| `~/.config/vayu_builder/config` | Persisted path overrides — sourced automatically on every launch |
| `<kernel_dir>/.builder_state` | Session state: incremental mode, ccache toggle, KSU branch, force-clean reason |
| `<kernel_dir>/.builder_prev_state` | Last build snapshot shown in the Previous Build info box |
| `<kernel_dir>/.build_number` | Monotonic build counter |
| `<kernel_dir>/.menuconfig_saved_config` | Preserved menuconfig `.config` — delete this file to cancel it |
| `<kernel_dir>/out/.version` | Kernel's own internal version counter (reset with [X] in Build Options) |

All state files are plain text and safe to delete if you want a completely clean slate.

---

## GitHub Actions CI Build

For automated, reproducible builds without a local Linux machine, this project includes a full GitHub Actions CI/CD pipeline. It compiles both the `main` (stable) and `dev` (latest) ReSukiSU driver branches in parallel, then publishes a combined rolling release with all kernels and manager APKs.

### What the Workflow Does

The workflow file is `.github/workflows/build-dual.yml`. It:

1. **Checks for new ReSukiSU commits** on both `dev` and `main` branches. If nothing has changed since the last scheduled run, the build is skipped entirely.
2. **Compiles both branches in parallel** (as a matrix job), each on a fresh Ubuntu runner, using identical toolchains and build flags.
3. **Bundles the matching ReSukiSU Manager APKs** from the upstream ReSukiSU CI, so each release includes both the kernel zip and its companion Manager app.
4. **Publishes a single rolling `latest` release** that is deleted and recreated each run, so the Releases page always shows exactly one entry with the most recent builds.
5. **Caches both the SHA fingerprints and ccache** across runs to keep builds fast and avoid redundant work.

---

### Job Breakdown

The workflow consists of four jobs that run in sequence:

#### `check-updates` — Should we build at all?

This job runs first and gates everything else. It:
- Uses `git ls-remote` to fetch the current HEAD SHA for both the `dev` and `main` branches of the ReSukiSU repository.
- Tries to restore a cached file keyed on the combined SHAs (`ksu-dual-<dev_sha>-<main_sha>`).
- If the cache key is a hit **and** the trigger is a scheduled run, it sets `has_updates=false` and all downstream jobs are skipped.
- If the SHAs are new (cache miss) **or** the build was triggered manually via `workflow_dispatch`, it sets `has_updates=true` and the build proceeds.

This means daily scheduled runs are completely free when no upstream changes have been made.

#### `build` — Compile both branches (matrix)

This job runs with a strategy matrix of `[dev, main]`, spawning two identical runners in parallel — one per branch. Each runner:

1. **Checks out** the kernel source at `fetch-depth: 1` (shallow clone for speed).
2. **Installs build dependencies** via `apt-get` — the same packages listed in the [Requirements](#1-install-dependencies) section above.
3. **Restores the ccache** for this specific branch and defconfig combination from the Actions cache, keyed on `ccache-<branch>-<defconfig-hash>-<run-number>`. The restore-keys fallback finds the best partial match from prior runs.
4. **Configures ccache** with `--max-size=2G`, content-based compiler checking, and compression to maximise cache efficiency.
5. **Downloads ZyC Clang 23.x** by running `.github/scripts/fetch_clang.sh` — see [How the Clang Fetcher Works](#how-the-clang-fetcher-works).
6. **Installs the ReSukiSU driver** by piping the upstream `setup.sh` directly from GitHub for the target branch.
7. **Applies KSU hook guards** — runs `scripts/apply_ksu_guards.py` to patch the required `#if defined(CONFIG_KSU_MANUAL_HOOK)` guards into all 9 manual hook call sites in the kernel source.
8. **Compiles the kernel** by running `.github/scripts/ci_build.sh` — see [How the CI Build Script Works](#how-the-ci-build-script-works).
9. **Saves the updated ccache** even if the build fails (`if: always()`), so a partial cache is preserved for the next attempt.
10. **Uploads the kernel zip** as a GitHub Actions artifact (`kernel-dev` or `kernel-main`) with a 1-day retention period.

`fail-fast: false` is set so that a failure in one branch does not cancel the other.

#### `release` — Assemble and publish

This job runs after both build jobs succeed. It:

1. **Downloads** both kernel zip artifacts from the `build` jobs.
2. **Fetches the ReSukiSU Manager APKs** (standard and spoofed variants for both branches) from the upstream ReSukiSU repository's Actions artifacts. It queries the last 30 successful push-triggered workflow runs on each branch and downloads the first non-expired artifact set it finds. This strategy ensures the Manager always matches a signed release build, never a scheduled or unsigned run.
3. **Reads the ReSukiSU version codes** from `ksu_version_DEV.txt` and `ksu_version_MAIN.txt` — small text files written by the CI build script that capture the exact version code printed by the ReSukiSU Makefile during compilation.
4. **Deletes the existing `latest` release and tag** (if any), then creates a fresh one with all zips attached and a formatted release body containing the build number, date, trigger reason, driver versions, and installation instructions.

#### `save-cache` — Persist the SHA fingerprint

Runs after a successful release. Saves the combined SHA cache key (`ksu-dual-<dev_sha>-<main_sha>`) so the next scheduled run knows that both branches were successfully built at these SHAs and can skip the build if nothing has changed.

---

### Repository Setup for GitHub Builds

To use the GitHub Actions workflow in your own fork, you need the following files in place:

```
.github/
├── workflows/
│   └── build-dual.yml       ← the main workflow
└── scripts/
    ├── ci_build.sh           ← headless build script (no TUI)
    └── fetch_clang.sh        ← downloads ZyC Clang 23.x

scripts/
└── apply_ksu_guards.py       ← patches hook guards into kernel source

AnyKernel3/                   ← must be committed into the repo
```

**Optional repository variable:**

| Variable | Purpose |
|----------|---------|
| `BUILD_COUNTER_OFFSET` | Integer offset subtracted from `GITHUB_RUN_NUMBER` to produce a clean sequential build number. For example, if your first real build is run #47, set `BUILD_COUNTER_OFFSET=46` so the zip is labelled `Build-1`. Set via **Settings → Variables and secrets → Actions variables**. |

No secrets need to be configured manually — the workflow uses the built-in `GITHUB_TOKEN` which is automatically provided by Actions.

---

### Triggering a Build

The workflow runs on two triggers:

| Trigger | When |
|---------|------|
| **Scheduled** | Daily at 03:00 UTC via cron (`0 3 * * *`). Skipped automatically if no upstream changes are detected. |
| **Manual** | Go to **Actions → Vayu-AnyMore ReSukiSU+SuSFS+KPM (DEV&MAIN) → Run workflow**. Optionally enter a reason string (e.g. `"Testing KPM fix"`). Manual triggers always build regardless of the SHA cache. |

---

### Build Artifacts & Releases

After a successful run, the following files are attached to the `latest` release:

| File | Description |
|------|-------------|
| `*MAIN*.zip` | Kernel zip built with the `main` (stable) ReSukiSU driver |
| `*DEV*.zip` | Kernel zip built with the `dev` (latest) ReSukiSU driver |
| `ReSukiSU-Manager-MAIN.zip` | Standard Manager APK for the MAIN branch |
| `ReSukiSU-Spoofed-Manager-MAIN.zip` | GMS-spoofed Manager APK for MAIN |
| `ReSukiSU-Manager-DEV.zip` | Standard Manager APK for the DEV branch |
| `ReSukiSU-Spoofed-Manager-DEV.zip` | GMS-spoofed Manager APK for DEV |

All releases use the **rolling `latest` strategy** — the previous release is deleted and replaced each run, so there is always exactly one release on the page. This keeps storage clean and makes the download link for the latest build stable and predictable.

> **Spoofed Manager** is only needed if your device or ROM fails SafetyNet/Play Integrity checks with the standard manager. Use the standard variant unless you have a specific reason to do otherwise.

---

### How the CI Build Script Works

`.github/scripts/ci_build.sh` is a headless (no TUI) version of the build logic from `build.sh`. It is invoked by the `build` job and runs inside the GitHub Actions environment.

**What it does, step by step:**

1. **Sets up the environment** — exports `ARCH=arm64`, `KBUILD_BUILD_USER=OmegaR01`, `KBUILD_BUILD_HOST=GitHub-CI`, and prepends the Clang `bin/` directory to `$PATH`.

2. **Configures ccache for CI** — applies several ccache settings that are critical for correctness on Actions runners:
   - `CCACHE_DIRECT=1` — hashes source files directly rather than running `clang -E`, eliminating the broken-pipe errors that occur when ccache closes a preprocessor pipe early after a cache hit.
   - `CCACHE_SLOPPINESS=random_seed,include_file_mtime,include_file_ctime` — prevents cache misses caused by the kernel's per-file `-frandom-seed` flag and by timestamp differences from fresh git checkouts on new runners.
   - `CCACHE_BASEDIR` and `CCACHE_NOHASHDIR` — normalise absolute paths before hashing so cache entries are portable across different runner machines with different workspace paths.

3. **Patches the defconfig** — calls the internal `toggle_config` function to enable `CONFIG_KSU`, `CONFIG_KSU_SUSFS`, and `CONFIG_KPM`, and to explicitly disable `CONFIG_KSU_MANUAL_HOOK`. This is equivalent to the Feature Configuration toggles in the interactive script.

4. **Generates `.config`** — runs `make vayu_defconfig` then `make olddefconfig` to produce a complete build configuration with all defconfig options resolved.

5. **Compiles the kernel** — runs `make -j$(nproc --all)` with `--output-sync=line` to keep interleaved output from parallel jobs readable. A `tail -f` process streams the build log live to the Actions console, with a `grep -v "LLVM ERROR:"` filter to suppress cosmetic broken-pipe messages that are non-fatal.

6. **Extracts the ReSukiSU version code** — greps the build log for the line `ReSukiSU version code: XXXXX` that the ReSukiSU Makefile prints during compilation. This is more reliable than the GitHub API because it reflects the exact code compiled into this kernel, not an approximation.

7. **Packages the zip** — copies `Image`, `dtbo.img`, and `dtb.img` into the `AnyKernel3/` directory and creates the flashable zip with `zip -r9`.

8. **Writes `ksu_version_<BRANCH>.txt`** alongside the zip so the `release` job can read the version code and include it in the release notes.

---

### How the Clang Fetcher Works

`.github/scripts/fetch_clang.sh` downloads and installs ZyC Clang 23.x for use by the CI runner.

**What it does:**

1. **Queries the GitHub Releases API** for the `ZyCromerZ/Clang` repository, fetching the 50 most recent releases as JSON.
2. **Finds the correct asset** — uses a short Python3 inline script to scan releases for a `.tar.gz` asset whose filename contains `Clang-23.` and returns the release tag, download URL, size, and asset name.
3. **Downloads to a temp file** with a visible progress bar via `curl -L --progress-bar`.
4. **Extracts to a temp directory** and handles both flat archives and archives that contain a single subdirectory.
5. **Verifies** that `bin/clang` is present in the extracted content. If it is not, the script exits with an error before touching `CLANG_DIR`.
6. **Moves** the verified toolchain directory to `${GITHUB_WORKSPACE}/clang`, replacing any previous installation.

---

## Troubleshooting

**Build button greyed out or blocked**
→ Clang or AnyKernel3 path is missing. Use **[T]** to download Clang, or **[S] → [P]** to fix paths.

**`ccache` shows N/A**
→ Install ccache: `sudo apt-get install ccache` (or equivalent). Restart the script after installing.

**GitHub API rate limited**
→ Unauthenticated GitHub API allows 60 requests/hour per IP. Wait a minute and retry, or download the toolchain tarball manually from [ZyCromerZ/Clang/releases](https://github.com/ZyCromerZ/Clang/releases) and extract it to `CLANG_DIR`.

**`bin/clang not found in archive`**
→ The downloaded tarball may be corrupted or incomplete. Delete the partial file and retry the fetch.

**Incremental build locked**
→ The ReSukiSU branch was switched or the hook mode was changed since the last build. A full clean build is required; the lock clears automatically after the next successful clean build.

**`Menuconfig aborted — no changes applied`**
→ You closed menuconfig with **Q** or Ctrl+C without saving. No `.config` changes were made.

**Kernel boots but root is not working**
→ Verify the `ksud` binary on the device matches the driver version in the Manager APK. If using the `dev` branch driver, ensure the Manager APK is also from the `dev` branch.

**Display corrupted in SSH / Termius**
→ The script uses `printf '\033[2J\033[H'` for screen clears, which is compatible with most terminal emulators. If box borders are broken, confirm your terminal is set to UTF-8 and the font supports box-drawing characters (╔ ╗ ╚ ╝ ║ ═).

**GitHub Actions build skips even after a manual trigger**
→ Manual `workflow_dispatch` runs always build regardless of the SHA cache. If the build is still being skipped, check the `check-updates` job log to confirm `has_updates` was set to `true`.

**Manager APK download step fails in CI**
→ The release job searches the last 30 successful push-triggered runs on the upstream ReSukiSU repository. If artifacts from all 30 runs have expired (GitHub retains action artifacts for 90 days by default), the download is skipped with a warning and the release is published without that APK.

---

## Reference Links

| Resource | URL |
|----------|-----|
| ReSukiSU project | https://github.com/ReSukiSU/ReSukiSU |
| ReSukiSU setup script | https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/kernel/setup.sh |
| ZyC Clang releases | https://github.com/ZyCromerZ/Clang/releases |
| AnyKernel3 | https://github.com/osm0sis/AnyKernel3 |
| SUSFS for KSU | https://gitlab.com/simonpunk/susfs4ksu |
| Vayu kernel source | https://github.com/Rsool22/android_kernel_xiaomi_vayu |
| OrangeFox Recovery (vayu) | https://orangefox.download/device/61310755bb6a91af6a656d0d |
| KernelPatch / KPM docs | https://github.com/bmax121/KernelPatch |
| Linux 4.14 kernel docs | https://www.kernel.org/doc/html/v4.14/ |
| Android kernel build guide | https://source.android.com/docs/setup/build/building-kernels |
| Clang cross-compilation | https://clang.llvm.org/docs/CrossCompilation.html |
| ccache manual | https://ccache.dev/manual/latest.html |

---

> **Project:** AnyMore Project — VAYU Kernel Builder
> **Device:** Xiaomi Poco X3 Pro (`vayu`, SM8150)
> **Kernel:** Linux 4.14 NonGKI · arm64 · Android 16
> **Compiler:** ZyC Clang 23.x · LLVM/LLD
> **Features:** ReSukiSU · SUSFS Inline Hook · KPM
