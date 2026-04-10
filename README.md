# android_kernel_xiaomi_vayu
### Linux 4.14 · NonGKI · Android 16 · Xiaomi Poco X3 Pro

Custom kernel source for the Poco X3 Pro (`vayu`, SM8150) based on the [AnymoreProject](https://github.com/AnymoreProject) 4.14 tree targeting Android 16. Ships with ReSukiSU v4.1.0, SUSFS v2.1.0 in inline-hook mode, and KPM support — all working on a non-GKI 4.14 kernel where kprobes are completely broken.

Includes an interactive `build.sh` TUI and a GitHub Actions CI pipeline that builds both the ReSukiSU `main` and `dev` branches in parallel and publishes a rolling release automatically.

**[→ Download Latest Release](https://github.com/Rsool22/android_kernel_vayu/releases/latest)**

---

## What's in here

**ReSukiSU v4.1.0** integrated via the official `setup.sh`. The driver lives at `drivers/kernelsu/` and is wired into Kconfig and the build system. Because kprobes are completely broken on this kernel, SUSFS inline-hook is the primary hook mode. Manual-hook mode is also supported from the same source tree — controlled entirely by which defconfig flag is active.

The two hook modes are **mutually exclusive**. `CONFIG_KSU_SUSFS` and `CONFIG_KSU_MANUAL_HOOK` cannot both be set. Every call site in the kernel source is guarded accordingly so you can switch modes by changing the defconfig without touching any source files.

**SUSFS v2.1.0** patches were cherry-picked from the [sidex15 SM8150 reference tree](https://github.com/sidex15/android_kernel_lge_sm8150) — 29 commits in total, three of which required manual conflict resolution. The critical fix for root not working on 4.14 non-GKI is a `ksu_handle_setresuid` call in `kernel/sys.c` guarded with `#if defined(CONFIG_KSU_SUSFS) || defined(CONFIG_KSU_MANUAL_HOOK)`. Without this, the kernel boots fine but root is never granted.

**KPM** is enabled via `CONFIG_KPM=y` with `KALLSYMS` and `KALLSYMS_ALL`. Works for loading KPM modules; the KPM card in the manager UI requires uname spoofing via SUSFS which is already configured.

**Docker and Droidspaces** — all Generally Necessary Docker configs pass the Moby check-config script. Droidspaces container configs are fully enabled. See the relevant sections below for details and known 4.14 QCOM limitations.

SELinux enforcing. `KBUILD_BUILD_USER=OmegaR01`, `KBUILD_BUILD_HOST=Vayu`.

---

## Download

Pre-built zips are published by the GitHub Actions CI after every successful build. Flash via [OrangeFox Recovery](https://orangefox.download/device/61310755bb6a91af6a656d0d) or any AnyKernel3-compatible recovery.

| File | Description |
|------|-------------|
| `*MAIN*.zip` | Kernel zip — ReSukiSU `main` (stable) + SUSFS + KPM |
| `*DEV*.zip` | Kernel zip — ReSukiSU `dev` (latest) + SUSFS + KPM |
| `ReSukiSU-Manager-MAIN.zip` | Standard Manager APK (MAIN branch) |
| `ReSukiSU-Spoofed-Manager-MAIN.zip` | GMS-spoofed Manager APK (MAIN branch) |
| `ReSukiSU-Manager-DEV.zip` | Standard Manager APK (DEV branch) |
| `ReSukiSU-Spoofed-Manager-DEV.zip` | GMS-spoofed Manager APK (DEV branch) |

Use the Spoofed Manager if your ROM fails Play Integrity with the standard one. Always match the Manager APK branch to the kernel zip you flashed.

**Flash steps:**
1. Boot into OrangeFox (or any TWRP-based recovery).
2. Flash the kernel zip.
3. Reboot, install the matching Manager APK, grant root on first launch.

---

## Building locally

### Requirements

| | |
|--|--|
| OS | Linux x86\_64 (Ubuntu 22.04+ recommended) |
| RAM | 8 GB minimum, 16 GB comfortable |
| Disk | ~30 GB for source + toolchain + out/ |
| Shell | Bash 4.x+ |
| Toolchain | ZyC Clang 23.x (fetched automatically by build.sh) |

```bash
sudo apt-get install -y make zip curl git python3 bc flex bison perl \
    libssl-dev libelf-dev gcc-aarch64-linux-gnu gcc-arm-linux-gnueabi ccache
```

### Quick start

```bash
git clone https://github.com/Rsool22/android_kernel_xiaomi_vayu \
    ~/kernel-builds/vayu_a16_kernel
cd ~/kernel-builds/vayu_a16_kernel

git clone https://github.com/osm0sis/AnyKernel3

chmod +x build.sh
./build.sh
```

On first launch without Clang installed the build option will be blocked. Use `[T] Toolchain Manager` to download Clang 23.x first.

### Manual compile (no TUI)

```bash
export PATH=~/kernel-builds/vayu_a16_kernel/clang/bin:$PATH

make O=out ARCH=arm64 vayu_defconfig

make O=out ARCH=arm64 \
    LLVM=1 LLVM_IAS=1 \
    CC=clang \
    CROSS_COMPILE=aarch64-linux-gnu- \
    CROSS_COMPILE_ARM32=arm-linux-gnueabi- \
    KBUILD_BUILD_USER=OmegaR01 \
    KBUILD_BUILD_HOST=Vayu \
    -j$(nproc)
```

Output lands at `out/arch/arm64/boot/` — copy `Image`, `dtb.img`, `dtbo.img` into AnyKernel3 and zip.

---

## build.sh

An interactive TUI build script included in the source. Handles toolchain management, defconfig patching, compilation, and AnyKernel3 packaging. Built for Bash 4.x and tested over Termius SSH.

### Directory layout

```
~/kernel-builds/vayu_a16_kernel/
├── build.sh
├── arch/arm64/configs/vayu_defconfig
├── drivers/kernelsu/              ← ReSukiSU driver
├── clang/                         ← ZyC Clang (fetched by [T])
├── AnyKernel3/
├── out/
│   └── arch/arm64/boot/
│       ├── Image
│       ├── dtb.img
│       └── dtbo.img
└── Anykernel-Builds/              ← final zips + logs

~/.config/vayu_builder/config      ← persisted path overrides
<kernel_dir>/.builder_state        ← session state
```

### Main menu

```
[B]  Full Build
[P]  Package Existing Image   (only shown when out/Image exists)
[T]  Toolchain Manager
[M]  ReSukiSU Driver Manager
[S]  Setup
[Q]  Quit
```

`[B]` is blocked with a warning if Clang or AnyKernel3 paths are missing. The previous build info (number, date, mode, features) is shown at the top of the menu.

### Feature configuration

```
[1]  Toggle ReSukiSU
[2]  Toggle SuSFS
[3]  Toggle KPM          (requires ReSukiSU)
[M]  menuconfig
[C]  Continue
[B]  Back
```

Hook mode is set automatically based on the feature flags:

| ReSukiSU | SuSFS | Mode |
|----------|-------|------|
| ON | ON | SuSFS Inline-Hook — `CONFIG_KSU_SUSFS=y`, `CONFIG_KSU_MANUAL_HOOK` unset |
| ON | OFF | Manual-Hook — `CONFIG_KSU_MANUAL_HOOK=y`, `CONFIG_KSU_SUSFS` unset |
| OFF | — | Vanilla |

All toggles write directly to `vayu_defconfig` before `make defconfig` runs.

### Build options

```
[N]  Set Kernel Name     → optional LOCALVERSION suffix (e.g. AnyMore-v2.1)
[I]  Incremental Build   → keeps previous out/ objects; OFF runs mrproper
[C]  ccache              → toggle compiler cache (greyed if not installed)
[S]  Start Build
[X]  Reset Build Counter
[B]  Back to Features
```

Switching the ReSukiSU branch or changing the hook mode between builds automatically locks incremental mode and forces a full clean on the next run.

### Build stages

| Stage | What happens |
|-------|-------------|
| 1 – Clean | `make mrproper` — skipped in incremental mode |
| 2 – Defconfig | `make vayu_defconfig` — skipped if menuconfig produced a fresh .config this session |
| 3 – Compile | `make -j$(nproc)` with ZyC Clang + LLVM/LLD, streamed live |
| 4 – Verify | Checks Image exists; shows relevant linker and compiler errors on failure |
| 5 – Package | Copies `Image`, `dtb.img`, `dtbo.img` into AnyKernel3, zips, moves to output dir |

Ctrl+C is caught via a `mkfifo`-based PID tracker — make is killed cleanly, a CANCEL log is written, and you are returned to the prompt with no orphan processes.

### Toolchain Manager `[T]`

Queries the [ZyCromerZ/Clang](https://github.com/ZyCromerZ/Clang) GitHub API, downloads with a progress bar, verifies `bin/clang`, then installs to `clang/`. Clang 23.x is the version this tree was tested with. The GitHub unauthenticated API allows 60 requests per hour — if you hit the limit, download the tarball manually and extract it to `CLANG_DIR`.

### ReSukiSU Driver Manager `[M]`

```
[I]  Install driver        (first-time setup)
[U]  Update driver         (checks remote SHA before pulling)
[T]  Switch branch         (main ↔ dev, forces clean build)
[V]  Verify hook guards
[X]  Remove driver
```

`[V] Verify Guards` checks all manual hook call sites and confirms each is wrapped in the correct `#if defined(CONFIG_KSU_...)` guard. Switching branches forces a full clean on the next build.

### Menuconfig workflow

After closing menuconfig the script detects whether `.config` was saved and presents options:

| Key | Action |
|-----|--------|
| `[V]` | Preserve `.config` for the next `./build.sh` run (saved to `.menuconfig_saved_config`) |
| `[D]` | Write back to `vayu_defconfig` permanently via `make savedefconfig` |
| `[T]` or `[I]` | Build immediately with the current `.config` as-is |

### Output files

```
Anykernel-Builds/
├── [VAYU-AnyMore-Project]-[ReSukiSU=SuSFS-Inline-Hook]-(Features=SuSFS+KPM)-(2026-04-06)-{Build-#5}.zip
├── [VAYU-AnyMore-Project]-[ReSukiSU=SuSFS-Inline-Hook]-(Features=SuSFS+KPM)-(2026-04-06)-Build-#5.log
└── [VAYU-AnyMore-Project]-FAIL.log    ← only present on failure
```

Only the most recent zip and log are kept. The counter only increments on successful packaging.

### State files

| File | Purpose |
|------|---------|
| `~/.config/vayu_builder/config` | Path overrides, persisted across runs |
| `.builder_state` | Incremental toggle, ccache, KSU branch, force-clean flag |
| `.builder_prev_state` | Last build info shown in main menu |
| `.build_number` | Monotonic build counter |
| `.menuconfig_saved_config` | Preserved menuconfig .config — delete to cancel |
| `out/.version` | Kernel internal version counter (reset with `[X]` in Build Options) |

---

## Docker Support

All Generally Necessary configs from the Moby `check-config.sh` script pass on this kernel. Optional configs that are viable on 4.14 QCOM are included.

**Known limitations:**
- `CFS_BANDWIDTH` is blocked by `SCHED_WALT=y` — Qualcomm's scheduler and CFS bandwidth enforcement are mutually exclusive in this tree
- `NFT_FIB` has no selectable symbol in 4.14
- Docker must use `iptables-legacy` and the `cgroupfs` driver — `BPF_CGROUP_DEVICE` attach type is not supported on 4.14

To run Docker inside Termux follow [this guide](https://gist.github.com/FreddieOliveira/efe850df7ff3951cb62d74bd770dce27#23-docker).

| Category | Configs |
|---|---|
| IPv6 NAT | `IP6_NF_NAT`, `IP6_NF_TARGET_MASQUERADE` |
| IPVS | `NETFILTER_XT_MATCH_IPVS`, `IP_VS`, `IP_VS_NFCT`, `IP_VS_RR`, `IP_VS_PROTO_TCP/UDP` |
| nftables | `NF_TABLES`, `NFT_CT`, `NFT_MASQ`, `NFT_NAT` |
| Network drivers | `MACVLAN`, `IPVLAN`, `VXLAN`, `BRIDGE_VLAN_FILTERING` |
| cgroup controls | `BLK_DEV_THROTTLING`, `CGROUP_PERF`, `NET_CLS_CGROUP`, `CGROUP_NET_PRIO` |

---

## Droidspaces / LXC Container Support

Full Droidspaces container support is enabled. Get the rootfs from the [official releases page](https://github.com/ravindu644/Droidspaces-rootfs-builder/releases/latest) — the [Droidspaces repository](https://github.com/ravindu644/Droidspaces-rootfs-builder) has setup instructions.

| Feature | Status |
|---|---|
| PID / IPC / UTS / NET / Mount namespaces | enabled |
| cgroup v2 — cpu, cpuset, io, memory, pids | enabled |
| OverlayFS with redirect + index | enabled |
| VETH / BRIDGE / BRIDGE_NETFILTER | enabled |
| TUN / FUSE | enabled |
| POSIX_MQUEUE / SYSVIPC | enabled |

---

## GitHub Actions CI

Builds both `main` and `dev` ReSukiSU branches in parallel on every upstream change and publishes a single rolling `latest` release. Workflow at `.github/workflows/build-dual.yml`.

**`check-updates`** — fetches HEAD SHAs from both ReSukiSU branches. If the combined SHA matches the cached value from the last successful run and the trigger is scheduled, the build is skipped entirely. Manual dispatch always builds regardless of the cache.

**`build` (matrix: dev, main)** — two runners in parallel, each shallow-clones the source, restores ccache (keyed on branch + defconfig hash), downloads ZyC Clang 23.x via `fetch_clang.sh`, installs the ReSukiSU driver, applies KSU hook guards via `scripts/apply_ksu_guards.py`, then compiles. `fail-fast: false` so one branch failing does not cancel the other. The kernel zip is uploaded as a GitHub Actions artifact.

**`release`** — downloads both kernel zips, fetches matching Manager APKs from the upstream ReSukiSU CI (searches the last 30 successful push-triggered runs per branch), deletes the existing `latest` release and recreates it with all files attached.

**`save-cache`** — persists the combined SHA cache key after a successful release so the next scheduled run can skip if nothing changed.

### ccache flags for CI

- `CCACHE_DIRECT=1` — avoids broken-pipe errors from the preprocessor being closed early on cache hits
- `CCACHE_SLOPPINESS=random_seed,include_file_mtime,include_file_ctime` — prevents cache misses from the kernel's per-file `-frandom-seed` and fresh-checkout timestamps
- `CCACHE_BASEDIR` + `CCACHE_NOHASHDIR` — makes cache entries portable across runners with different workspace paths

### Repo layout for CI

```
.github/
├── workflows/build-dual.yml
└── scripts/
    ├── ci_build.sh
    └── fetch_clang.sh
scripts/
└── apply_ksu_guards.py
AnyKernel3/          ← must be committed into the repo
```

Optional repo variable: `BUILD_COUNTER_OFFSET` — subtracted from `GITHUB_RUN_NUMBER` to produce a clean sequential build number. Set under Settings → Variables → Actions variables. No secrets needed — the workflow uses the built-in `GITHUB_TOKEN`.

**Triggers:** daily at 03:00 UTC (skipped if no upstream changes), or manual via Actions → Run workflow.

---

## Troubleshooting

**Root not working after flash** — most likely the Manager APK branch doesn't match the kernel zip. If that's correct, verify `ksu_handle_setresuid` is present in `kernel/sys.c` — this is the critical call site for root grant on 4.14 non-GKI.

**SUSFS not working** — confirm `CONFIG_KSU_SUSFS=y` and `CONFIG_KSU_MANUAL_HOOK` is explicitly not set. Having both set simultaneously causes silent failures.

**Incremental build locked** — the ReSukiSU branch or hook mode changed since the last build. A full clean is required; the lock clears automatically after it completes.

**Modules not loading after reboot** — ensure no hook guards fire twice in SUSFS mode. Call sites guarded with `CONFIG_KSU_SUSFS || CONFIG_KSU_MANUAL_HOOK` must not overlap with the inline hooks already baked in by the SUSFS cherry-picks.

**GitHub API rate limited** — 60 unauthenticated requests per hour. Manually download the tarball from [ZyCromerZ/Clang/releases](https://github.com/ZyCromerZ/Clang/releases) and extract to `clang/`.

**Box drawing broken in Termius** — confirm terminal encoding is UTF-8 and the font includes box-drawing characters (╔ ╗ ╚ ╝ ║ ═).

**Manager APK missing from CI release** — the release job searches the last 30 push-triggered upstream runs. If all 30 have expired (GitHub retains artifacts for 90 days by default), the APK is skipped with a warning and the release is published without it.

---

## Reference Links

| | |
|--|--|
| ReSukiSU | https://github.com/ReSukiSU/ReSukiSU |
| SUSFS for KSU | https://gitlab.com/simonpunk/susfs4ksu |
| ZyC Clang releases | https://github.com/ZyCromerZ/Clang/releases |
| AnyKernel3 | https://github.com/osm0sis/AnyKernel3 |
| KernelPatch / KPM | https://github.com/bmax121/KernelPatch |
| OrangeFox Recovery (vayu) | https://orangefox.download/device/61310755bb6a91af6a656d0d |
| Droidspaces rootfs | https://github.com/ravindu644/Droidspaces-rootfs-builder/releases/latest |
| Docker on Termux | https://gist.github.com/FreddieOliveira/efe850df7ff3951cb62d74bd770dce27#23-docker |

---

> Poco X3 Pro (`vayu`, SM8150) · Linux 4.14 NonGKI · Android 16 · ZyC Clang 23.x · ReSukiSU v4.1.0 · SUSFS v2.1.0 · KPM
