# VAYU Kernel Builder
### ReSukiSU + SuSFS + KPM — AnyMore Project

> Interactive TUI build script for the **Xiaomi Poco X3 Pro (vayu)** — Linux 4.14 NonGKI kernel targeting Android 16.  
> Handles everything: dependencies, toolchain, driver setup, defconfig, compilation, and AnyKernel3 packaging — from a single script.

---

## Table of Contents

- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Directory Layout](#directory-layout)
- [First-Time Setup](#first-time-setup)
  - [1. Install Dependencies](#1-install-dependencies)
  - [2. Download ZyC Clang](#2-download-zyc-clang)
  - [3. Install the ReSukiSU Driver](#3-install-the-resukisu-driver)
  - [4. Clone AnyKernel3](#4-clone-anykernel3)
- [Main Menu](#main-menu)
- [Feature Configuration](#feature-configuration)
- [Build Options](#build-options)
- [Toolchain Manager](#toolchain-manager)
- [ReSukiSU Driver Manager](#resukisu-driver-manager)
- [Setup Menu](#setup-menu)
  - [Path Configurator](#path-configurator)
  - [Dependency Checker](#dependency-checker)
- [Build Internals](#build-internals)
- [Output Files](#output-files)
- [Menuconfig Workflow](#menuconfig-workflow)
- [State & Config Files](#state--config-files)
- [Troubleshooting](#troubleshooting)
- [Reference Links](#reference-links)

---

## Requirements

| Item | Minimum |
|------|---------|
| OS | Linux (Ubuntu 22.04+ IS Recommended) |
| Shell | Bash 4.x+ |
| RAM | 8 GB (16 GB Is recommended) |
| Disk | 30 GB free (kernel source + toolchain + out/) |
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

On first run with no Clang present, the main menu will show a red warning and block the **Build** option. Use **[T] Toolchain Manager** to download Clang before attempting a build.

---

## Directory Layout

The script uses these default paths, all configurable via **[S] Setup → [P] Paths**:

```
~/kernel-builds/vayu_a16_kernel/      ← kernel_dir  (kernel source root)
├── build.sh                          ← this script
├── arch/arm64/configs/vayu_defconfig ← active defconfig
├── drivers/kernelsu/                 ← ReSukiSU driver (installed by [M])
├── clang/                            ← CLANG_DIR  (ZyC Clang toolchain)
├── AnyKernel3/                       ← anykernel  (packaging template)
├── out/                              ← objdir  (build output)
│   └── arch/arm64/boot/
│       ├── Image
│       ├── dtb.img
│       └── dtbo.img
└── Anykernel-Builds/                 ← OUTPUT_DIR  (final zips + logs)

~/.config/vayu_builder/config         ← persisted path overrides
~/.kernel-builds/.build_state         ← persisted session state (branch, ccache, mode)
```

---

## First-Time Setup

### 1. Install Dependencies

Launch the script and go to **[S] Setup → [D] Check Dependencies**.

The dependency checker detects your package manager (apt / dnf / pacman / zypper / apk) and shows the status of every required and optional package. Press **[I]** to install all missing packages automatically.

**Required packages:**

| Package | Purpose |
|---------|---------|
| `make` | Build system |
| `zip` | AnyKernel3 packaging |
| `curl` | Toolchain + driver downloads |
| `git` | ReSukiSU driver setup |
| `python3` | GitHub API JSON parsing |
| `bc`, `flex`, `bison`, `perl` | Kernel build prerequisites |
| `libssl-dev` | Kernel crypto headers |
| `libelf-dev` | Kernel BTF/ELF support |
| `gcc-aarch64-linux-gnu` | GCC 64-bit cross compiler |
| `gcc-arm-linux-gnueabi` | GCC 32-bit cross compiler |

**Optional:**

| Package | Purpose |
|---------|---------|
| `ccache` | Compiler cache — dramatically speeds up rebuilds |

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

From the main menu press **[T] Toolchain Manager**, then:

1. Select the target version (`23` for ZyC Clang 23.x — recommended, `15` for 15.x, `L` for absolute latest)
2. Press **[C]** to check the latest release info without downloading
3. Press **[F]** to fetch and install — the script queries the [ZyCromerZ/Clang](https://github.com/ZyCromerZ/Clang) GitHub API, shows download size, asks for confirmation, then downloads with a live progress bar and extracts to `clang/`

The toolchain is ~300–500 MB. Extraction and install are automatic; no manual steps required.

> If GitHub API rate limits you (60 req/hr unauthenticated), wait a minute and try again.

---

### 3. Install the ReSukiSU Driver

From the main menu press **[M] ReSukiSU Driver Manager**, then press **[I] Install Driver**.

The script runs the official `setup.sh` from [ReSukiSU/ReSukiSU](https://github.com/ReSukiSU/ReSukiSU) against your kernel source tree, which places the driver at `drivers/kernelsu/`. You can choose the branch (`main` = stable, `dev` = latest features) from this same menu.

> ⚠️ A **full clean build is required** after switching branches or reinstalling the driver.

---

### 4. AnyKernel3

The script packages the kernel using [AnyKernel3](https://github.com/osm0sis/AnyKernel3). The  Repo already has it included or you can Clone it either to the default location or configure a custom path via **[S] Setup → [P] Paths → [3]**:

```bash
git clone https://github.com/osm0sis/AnyKernel3 \
    ~/kernel-builds/vayu_a16_kernel/AnyKernel3
```

Make sure your `anykernel.sh` inside the clone is configured for vayu before building.

---

## Main Menu

```
[B]  Full Build             → configure → compile → package
[P]  Package Existing Image → skip compile, re-package out/ artifacts only
[T]  Toolchain Manager      → fetch/update ZyC Clang
[M]  ReSukiSU Driver Mgr    → install/update/switch driver branch
[S]  Setup                  → paths + dependency checker
[Q]  Quit
```

- **[B]** is blocked with a red warning if Clang or AnyKernel3 paths are missing.
- **[P]** only appears if `out/arch/arm64/boot/Image` already exists.
- The **Previous Build** box at the top shows the last build's number, date, mode, and feature set.

---

## Feature Configuration

Reached from **[B] Full Build** → Feature Configuration screen.

```
[1]  Toggle ReSukiSU       → enables CONFIG_KSU; enables SuSFS-Inline-Hook mode
[2]  Toggle SuSFS          → CONFIG_KSU_SUSFS; switches hook mode to Manual if off
[3]  Toggle KPM            → CONFIG_KPM; requires ReSukiSU to be enabled
[M]  menuconfig            → opens interactive kernel config editor
[C]  Continue to Build     → proceed with current settings
[B]  Back                  → return to main menu
```

**Hook mode rules (enforced automatically):**

| KSU | SuSFS | Result |
|-----|-------|--------|
| ON  | ON    | SuSFS-Inline-Hook mode (`CONFIG_KSU_SUSFS=y`) |
| ON  | OFF   | Manual-Hook mode (`CONFIG_KSU_MANUAL_HOOK=y`) |
| OFF | —     | Vanilla kernel |

> `CONFIG_KSU_SUSFS` and `CONFIG_KSU_MANUAL_HOOK` are mutually exclusive. The script enforces this automatically — never enable both manually.

All toggles write directly to `arch/arm64/configs/vayu_defconfig` using `sed` in-place.

---

## Build Options

Second step of the full build flow.

```
[N]  Set Kernel Name    → optional LOCALVERSION suffix (e.g. AnyMore-v2.1)
                          uname -r will show: 4.14.356-AnyMore-v2.1
[I]  Incremental Build  → ON keeps previous out/ objects; OFF does mrproper first
[C]  ccache             → toggle compiler cache (greyed out if ccache not installed)
[S]  Start Build        → begin compilation
[X]  Reset Build Counter → resets zip/log counter and out/.version to #1
[B]  Back               → return to Feature Configuration
[Q]  Quit
```

**Incremental lock:** after switching ReSukiSU branch or changing hook mode, incremental is automatically locked and a full clean build is forced on the next run.

---

## Toolchain Manager

Accessed from the main menu via **[T]**.

```
[F]   Fetch Selected Version   → download + install to CLANG_DIR
[C]   Check Latest Release     → API query only, no download
[23]  Target ZyC Clang 23.x    ← recommended
[15]  Target ZyC Clang 15.x
[L]   Latest (any version)
[R]   Return
```

- Source: [github.com/ZyCromerZ/Clang](https://github.com/ZyCromerZ/Clang)
- Downloads a `.tar.gz` release asset, extracts to a temp dir, verifies `bin/clang` is present, then moves to `CLANG_DIR`
- Shows a live download progress bar with MB counter
- Shows a spinner during extraction
- If an existing toolchain is present it is replaced after confirmation

---

## ReSukiSU Driver Manager

Accessed from the main menu via **[M]**.

```
[U]  Update Driver       → pull latest from active branch (checks remote SHA first)
[I]  Install Driver      → first-time install via setup.sh (shown when not installed)
[S]  Switch Branch       → toggles main ↔ dev, pulls immediately, forces clean build
[V]  Verify Guards       → checks KSU/SuSFS manual hook guards in source files
[X]  Remove Driver       → runs setup.sh --cleanup, removes drivers/kernelsu/
[R]  Return
```

- Source: [github.com/ReSukiSU/ReSukiSU](https://github.com/ReSukiSU/ReSukiSU)
- Branch selection (`main` / `dev`) is persisted to `.builder_state`
- **[V] Verify Guards** runs the internal `_apply_ksu_guards` check which validates that all 9 manual hook sites are correctly guarded with `#if defined(CONFIG_KSU_MANUAL_HOOK)`
- **[S] Switch Branch** automatically triggers a pull and locks incremental mode

---

## Setup Menu

Accessed from the main menu via **[S]**.

```
[P]  Configure Paths      → edit all directory paths
[D]  Check Dependencies   → detect + install missing packages
[R]  Return
```

### Path Configurator

```
[1]  Kernel Dir     default: ~/kernel-builds/vayu_a16_kernel
[2]  Clang Dir      default: <kernel_dir>/clang
[3]  AnyKernel3     default: <kernel_dir>/AnyKernel3
[4]  Output Dir     default: <kernel_dir>/Anykernel-Builds
[5]  GCC64 Dir      default: /usr/bin  (aarch64-linux-gnu-* location)
[6]  GCC32 Dir      default: /usr/bin  (arm-linux-gnueabi-* location)
[S]  Save & Apply   → writes to ~/.config/vayu_builder/config, hot-applies
[R]  Return (discard unsaved changes)
```

Paths support tilde (`~`) expansion. Changes take effect immediately after **[S]**.  
The config file is sourced automatically on every subsequent `./build.sh` invocation.

### Dependency Checker

Lists every required and optional package with OK / MISSING status, detects your package manager, and optionally runs the install command for you with **[I]**.

Supported package managers: `apt`, `dnf`, `yum`, `pacman`, `zypper`, `apk`.

---

## Build Internals

A full build runs five sequential stages:

| Stage | What happens |
|-------|-------------|
| **1 – Clean** | `make mrproper` (full) or skipped (incremental) |
| **2 – Defconfig** | `make vayu_defconfig` → generates `out/.config`; skipped if menuconfig was used this session |
| **3 – Compile** | `make` with ZyC Clang + LLVM/LLD, 4.14 nonGKI flags, optional ccache. Thread count = `nproc` |
| **4 – Verify** | Checks `out/arch/arm64/boot/Image` exists; on failure shows extracted linker and compiler errors inline |
| **5 – Package** | Copies `Image`, `dtb.img`, `dtbo.img` into AnyKernel3 dir, runs `zip -r9`, moves to `OUTPUT_DIR` |

**Make flags used:**
```
ARCH=arm64
LLVM=1
LLVM_IAS=1
CC=clang  (or ccache clang)
CROSS_COMPILE=aarch64-linux-gnu-
CROSS_COMPILE_ARM32=arm-linux-gnueabi-
O=<out/>
```

**Build identity:**
```
KBUILD_BUILD_USER=OmegaR01
KBUILD_BUILD_HOST=Vayu
```

---

## Output Files

All final outputs land in `OUTPUT_DIR` (`Anykernel-Builds/` by default):

```
Anykernel-Builds/
├── [VAYU-AnyMore-Project]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-(2025-01-01)-{Build-#3}.zip
├── [VAYU-AnyMore-Project]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-(2025-01-01)-Build-#3.log
└── [VAYU-AnyMore-Project]-FAIL.log   ← only on failure; replaced on next success
```

- Only the **most recent** zip and success log are kept; old ones are deleted automatically on each successful build
- The build counter increments only on successful packaging
- CANCEL logs are written if Ctrl+C is used during compilation, and are cleaned on the next successful build

---

## Menuconfig Workflow

Press **[M]** in the Feature Configuration screen to open the standard kernel `menuconfig` UI.

After closing menuconfig you have three options shown in the post-build **What Next?** prompt:

| Key | Action |
|-----|--------|
| **[V]** | Preserve `.config` for next `./build.sh` run — saved to `.menuconfig_saved_config`, auto-restored on relaunch |
| **[D]** | Write changes permanently to `vayu_defconfig` via `make savedefconfig` — becomes the new build default |
| **[T/I]** | Retry build (defconfig regen is automatically skipped; the menuconfig `.config` is reused) |

> If you choose **[V]**, a magenta notice appears on the main menu reminding you a preserved `.config` is active. Delete the file `<kernel_dir>/.menuconfig_saved_config` to cancel it.

---

## State & Config Files

| File | Purpose |
|------|---------|
| `~/.config/vayu_builder/config` | Persisted path overrides — sourced on every launch |
| `<kernel_dir>/.builder_state` | Session state: incremental mode, ccache toggle, KSU branch, force-clean reason |
| `<kernel_dir>/.builder_prev_state` | Last build snapshot shown in the Previous Build box |
| `<kernel_dir>/.build_number` | Monotonic build counter |
| `<kernel_dir>/.menuconfig_saved_config` | Preserved menuconfig `.config` (delete to cancel) |
| `<kernel_dir>/out/.version` | Kernel's own version counter (reset with [X]) |

All state files are plain text and safe to delete if you want a clean slate.

---

## Troubleshooting

**Build button greyed out / blocked**  
→ Clang or AnyKernel3 path is missing. Use **[T]** to download Clang or **[S] → [P]** to fix paths.

**`ccache` shows N/A**  
→ Install ccache: `sudo apt-get install ccache` (or equivalent). Restart the script after installing.

**`GitHub API rate limited`**  
→ Unauthenticated GitHub API allows 60 requests/hour per IP. Wait ~1 minute and retry. Alternatively, download the toolchain tarball manually from [ZyCromerZ/Clang/releases](https://github.com/ZyCromerZ/Clang/releases) and extract it to `CLANG_DIR`.

**`bin/clang not found in archive`**  
→ The downloaded tarball may be corrupted. Delete the partial file and retry fetch.

**Incremental build locked**  
→ The ReSukiSU branch was switched or hook mode changed. A full clean build is required. The lock clears automatically after the next successful full build.

**`Menuconfig aborted — no changes applied`**  
→ You closed menuconfig with **Q** or Ctrl+C without saving. No `.config` changes were made.

**Kernel boots but root not working**  
→ Verify the `ksud` binary on device matches the driver version. If using ReSukiSU dev branch, ensure the Manager APK is also from the dev branch.

**Display corrupted in SSH/Termius**  
→ The script uses `printf '\033[2J\033[H'` for screen clears, which is compatible with most terminal emulators. If box borders are broken, ensure your terminal is set to UTF-8 and the font supports box-drawing characters (╔╗╚╝║═).

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
