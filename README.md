<!--
  GITHUB REPO DESCRIPTION (Settings → About → Description):
  "Linux 4.14 NonGKI kernel for Xiaomi Poco X3 Pro (vayu · SM8150) · Android 11~16 · ReSukiSU + SUSFS v2.1.0 + KPM · based on LineageOS/android_kernel_qcom_sm8150"
-->

<div align="center">
  <a href="https://keepandroidopen.org">
    <img src="https://keepandroidopen.org/banner/" alt="Keep Android Open" width="100%"/>
  </a>
</div>

<div align="center">

# 🐧 AnyMore-Project Kernel Source

**(Vayu/Bhima) Perf Kernel | v4.14.356+13 nonGKI • Android 11 ~ 16**

[![CI](https://img.shields.io/github/actions/workflow/status/Rsool22/android_kernel_vayu/build-dual.yml?branch=16&style=for-the-badge&logo=github-actions&logoColor=white&label=CI)](https://github.com/Rsool22/android_kernel_vayu/actions/workflows/build-dual.yml) [![Release](https://img.shields.io/github/v/release/Rsool22/android_kernel_vayu?style=for-the-badge&logo=github&logoColor=white&label=Release)](https://github.com/Rsool22/android_kernel_vayu/releases/latest) [![Last Commit](https://img.shields.io/github/last-commit/Rsool22/android_kernel_vayu/16?style=for-the-badge&logo=git&logoColor=white&label=Last%20Commit)](https://github.com/Rsool22/android_kernel_vayu/commits/16) [![License](https://img.shields.io/badge/License-GPL--2.0-blue?style=for-the-badge&logo=gnu&logoColor=white)](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html)

[![Awesome Android Root](https://img.shields.io/badge/Awesome%20Android%20Root-Root%20Apps%20%26%20Modules-3DDC84?style=flat-square&logo=android&logoColor=white)](https://github.com/awesome-android-root/awesome-android-root#root-apps-and-modules) [![ReSukiSU Telegram](https://img.shields.io/badge/ReSukiSU-Telegram-2CA5E0?style=flat-square&logo=telegram&logoColor=white)](https://t.me/ReSukisu) [![AnyMore Discuss](https://img.shields.io/badge/AnyMore-Discuss-7B2FBE?style=flat-square&logo=telegram&logoColor=white)](https://t.me/+fh4NvUnmcWM4ODE1) [![Poco X3 Pro Updates](https://img.shields.io/badge/Poco%20X3%20Pro-Updates-E91E63?style=flat-square&logo=telegram&logoColor=white)](https://t.me/PocoX3ProUpdates)

</div>

---

A fork of the [AnymoreProject](https://github.com/AnymoreProject/android_kernel_vayu) vayu kernel tree, itself based on the [LineageOS SM8150 kernel](https://github.com/LineageOS/android_kernel_qcom_sm8150) — the original Android 4.14 source base for Qualcomm SM8150 devices. Targets Android 11 through 16 and is compatible with most AOSP-based ROMs.

&nbsp;

This kernel source includes an interactive `build.sh` TUI for local builds, alongside a GitHub Actions CI pipeline that builds both the ReSukiSU `main` and `dev` branches in parallel — automatically publishing a rolling release on every upstream change.

---

## Table of Contents

- [Download](#download)
- [Features](#features)
- [Docker Support](#docker-support)
- [Droidspaces / LXC Container Support](#droidspaces--lxc-container-support)
- [Building Locally](#building-locally)
- [The vayu-builder TUI](#the-vayu-builder-tui)
- [GitHub Actions CI](#github-actions-ci)
- [Credits](#credits)
- [Reference Links](#reference-links)

---

## Download

Pre-built kernel zips are published automatically by the GitHub Actions CI pipeline after every successful build and are available on the [releases page](https://github.com/Rsool22/android_kernel_vayu/releases/latest). Flash via [OrangeFox Recovery](https://orangefox.download/device/61310755bb6a91af6a656d0d) or any AnyKernel3-compatible recovery.

> [!NOTE]
> All CI builds use **SUSFS Inline-Hook mode with KPM kernel support** exclusively. Manual-Hook mode is not built by the CI — fork the repo and edit the workflow, or build locally via `build.sh` if you need it.

| File | Description |
|------|-------------|
| `*MAIN*.zip` | Kernel zip — ReSukiSU `main` (stable) + SUSFS v2.1.0 Inline-Hook + KPM |
| `*DEV*.zip` | Kernel zip — ReSukiSU `dev` (latest features) + SUSFS v2.1.0 Inline-Hook + KPM |
| `ReSukiSU-Manager-MAIN.zip` | Standard Manager APK (MAIN branch) |
| `ReSukiSU-Spoofed-Manager-MAIN.zip` | GMS-spoofed Manager APK (MAIN branch) |
| `ReSukiSU-Manager-DEV.zip` | Standard Manager APK (DEV branch) |
| `ReSukiSU-Spoofed-Manager-DEV.zip` | GMS-spoofed Manager APK (DEV branch) |

> [!TIP]
> Use the **Spoofed Manager** variant if your ROM fails Play Integrity checks with the standard APK. Always match the Manager APK branch to the kernel zip you flashed — do not mix `MAIN` and `DEV`.

> [!WARNING]
> Do **not** flash this kernel on **HyperOS/MIUI**. This kernel is built from the AnymoreProject Android 16 branch which is incompatible with HyperOS/MIUI — flashing it will result in a bootloop. HyperOS/MIUI uses a heavily modified kernel source maintained on a separate branch. If you want a custom kernel for HyperOS/MIUI, fork the HyperOS/MIUI branch from the [AnymoreProject kernel](https://github.com/AnymoreProject/android_kernel_vayu) instead.

**Flash steps:**
1. Boot into OrangeFox Recovery or any TWRP-based recovery
2. Flash the kernel zip for your preferred branch (`MAIN` = stable, `DEV` = latest features)
3. Reboot to system
4. Install the matching Manager APK and grant root access when prompted
5. *(Optional)* To activate KPM: open the ReSukiSU Manager app, flash the KernelPatch package from within the app, and select the KPM patch when prompted

---

## Features

### ReSukiSU

<div align="left">
  <img src="https://raw.githubusercontent.com/ReSukiSU/ReSukiSU/main/docs/ReSukiSU_blue.svg" width="180" align="right"/>
</div>

ReSukiSU / SukiSU Ultra is a kernel-based root solution forked from SukiSU Ultra, focused on non-GKI kernel compatibility and enhanced stability. The driver is integrated via the official `setup.sh` script and lives at `drivers/kernelsu/`, wired into Kconfig and the build system. Root is granted via direct manual source hook call sites, and ReSukiSU's hook abstraction handles the kernel-side logic automatically once those call sites are in place. See the [ReSukiSU documentation](https://resukisu.github.io) for full integration details.

> [!NOTE]
> ReSukiSU does not publish versioned GitHub releases — the driver version codes shown in the **Build Info** table on this repo's [releases page](https://github.com/Rsool22/android_kernel_vayu/releases/latest) (e.g. `34781`) are internal build numbers generated by the upstream ReSukiSU CI pipeline.

[![CI Builds](https://img.shields.io/badge/CI-Builds-2CA5E0?style=flat-square&logo=github-actions&logoColor=white)](https://github.com/ReSukiSU/ReSukiSU/actions?query=event%3Apush) [![SukiSU Ultra Latest](https://img.shields.io/badge/SukiSU%20Ultra-Latest%20Release-E91E63?style=flat-square&logo=github&logoColor=white)](https://github.com/tiann/KernelSU/releases/latest) [![GPL-2.0](https://img.shields.io/badge/License-GPL--2.0-blue?style=flat-square&logo=gnu&logoColor=white)](https://www.gnu.org/licenses/old-licenses/gpl-2.0.en.html) [![GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square&logo=gnu&logoColor=white)](https://github.com/ReSukiSU/ReSukiSU/blob/main/LICENSE)

&nbsp;

<details>
<summary><strong>Available Hook Modes</strong></summary>
<br>

Two hook modes are supported from the same source tree, switchable via `build.sh` or by manually editing `vayu_defconfig` — no source changes are required, as all hook call sites are already integrated into the kernel source.

| Mode | Config flag | When to use |
|:----:|------------|------------|
| **SUSFS Inline-Hook** | `CONFIG_KSU_SUSFS=y` | Primary mode — pairs with SUSFS for root concealment and hooks |
| **Manual-Hook** | `CONFIG_KSU_MANUAL_HOOK=y` | Standalone mode — hooks only, no SUSFS |

> [!NOTE]
> Manual hook call sites are already integrated directly into the kernel source — no additional patching is needed to use Manual-Hook mode. The following files contain manual hooks (`ksu_handle_*`):
> - `fs/exec.c` — `ksu_handle_execveat`
> - `fs/read_write.c` — `ksu_handle_sys_read`
> - `fs/devpts/inode.c` — `ksu_handle_devpts`
> - `kernel/reboot.c` — `ksu_handle_sys_reboot`
> - `drivers/input/input.c` — `ksu_handle_input_handle_event`
> - `fs/stat.c` — `ksu_handle_stat`, `ksu_handle_newfstat_ret`, `ksu_handle_fstat64_ret`

> [!IMPORTANT]
> These two modes are **mutually exclusive**. `CONFIG_KSU_SUSFS` and `CONFIG_KSU_MANUAL_HOOK` must never both be set at the same time. SUSFS already provides its own inline hooks at the kernel level — enabling Manual-Hook mode simultaneously would cause every hook call site to fire twice, leading to unpredictable behavior. The only way to run both is to either strip the SUSFS inline hooks from the patches manually, or edit Kconfig to allow both flags — neither is recommended. Switching modes is done entirely via `build.sh` or by manually editing `vayu_defconfig`, with no source changes required.

</details>

---

### SUSFS v2.1.0

[SUSFS](https://gitlab.com/simonpunk/susfs4ksu) is a filesystem-level root concealment layer for KernelSU-based roots. It hides root from apps and system services at the kernel level, making detection significantly harder than userspace-only approaches. ReSukiSU ships with a built-in SUSFS control panel in the manager app — no separate module is required to configure SUSFS.

Patches were applied via 29 cherry-picks from the [sidex15 SM8150 reference tree](https://github.com/sidex15/android_kernel_lge_sm8150).

&nbsp;

<div align="center">
  <img src="https://raw.githubusercontent.com/sidex15/susfs4ksu-binaries/refs/heads/new/susfsbanner.png" width="400"/>
</div>

&nbsp;

**susfs4ksu** — A KernelSU module for SUSFS patched kernels. Installs the userspace helper tools `ksu_susfs` and `sus_su` into `/data/adb/ksu` and provides a script to communicate with the SUSFS kernel layer, enabling root hiding at the kernel level.

[![Download](https://img.shields.io/badge/Download-Latest%20CI-3DDC84?style=flat-square&logo=github-actions&logoColor=white)](https://nightly.link/sidex15/susfs4ksu-module/workflows/build/v1.5.2%2B?preview)

> [!NOTE]
> - Requires a kernel with SUSFS patches applied — this kernel qualifies
> - Kernel must use SUSFS v1.5.2 or later for effective hiding
> - Shamiko v1.2.1 or later is compatible but optional
> - HideMyApplist is compatible
> - ReVanced root module compatible
> - Recommended to use [bindhosts](https://github.com/bindhosts/bindhosts) for systemless hosts support
>
> 📖 [susfs4ksu module documentation](https://github.com/sidex15/susfs4ksu-module/wiki)

> [!NOTE]
> SUSFS patch sites in the kernel source (`susfs_*`):
> `kernel/sys.c`, `fs/open.c`, `fs/namei.c`, `fs/namespace.c`, `fs/stat.c`, `fs/readdir.c`, `fs/statfs.c`, `fs/proc/base.c`, `fs/proc/fd.c`, `fs/proc/task_mmu.c`, `fs/proc_namespace.c`, `fs/proc/cmdline.c`, `security/selinux/avc.c`, `kernel/kallsyms.c`

> [!IMPORTANT]
> The critical call site for root to function on 4.14 non-GKI with SUSFS Inline-Hook is `ksu_handle_setresuid` in `kernel/sys.c`, guarded with `#if defined(CONFIG_KSU_SUSFS) || defined(CONFIG_KSU_MANUAL_HOOK)`. Without this call site the kernel compiles and boots without errors, but root grant fails silently — the manager app will display an error indicating it failed to grant root to any app.

---

### KPM (KernelPatch Modules)

[KernelPatch](https://github.com/bmax121/KernelPatch) module support is enabled in the kernel config via `CONFIG_KPM=y`, with `KALLSYMS` and `KALLSYMS_ALL` also set. This allows loading kernel patch modules at runtime without recompiling the kernel. A backport of `set_memory.h` from Linux 4.19 is also included to ensure KPM compatibility on this 4.14 tree.

> [!NOTE]
> KPM support is **experimental** on Linux 4.14 and may not be stable on all ROM configurations.

> [!IMPORTANT]
> KPM is **not active by default** — the kernel supports it, but activation requires a one-time setup after flashing:
> 1. Flash the kernel zip and boot into system normally
> 2. Open the ReSukiSU Manager app
> 3. Flash the KernelPatch package from within the app
> 4. Select the KPM patch when prompted

---

### Droidspaces / LXC Container Support

[Droidspaces](https://github.com/ravindu644/Droidspaces-OSS) is a lightweight, portable Linux containerization tool that lets you run full Linux distributions natively on Android — with complete init system support including systemd, OpenRC, runit, s6, and more — at zero performance penalty.

Full Droidspaces container support is enabled on this kernel. Download a rootfs from the [official rootfs releases page](https://github.com/ravindu644/Droidspaces-rootfs-builder/releases/latest) and follow the [Android installation guide](https://github.com/ravindu644/Droidspaces-OSS/blob/main/Documentation/Installation-Android.md) for setup instructions.

[![Download](https://img.shields.io/badge/Download-Latest%20Release-3DDC84?style=flat-square&logo=github&logoColor=white)](https://github.com/ravindu644/Droidspaces-OSS/releases/latest) [![CI Builds](https://img.shields.io/badge/CI-Builds-2CA5E0?style=flat-square&logo=github-actions&logoColor=white)](https://github.com/ravindu644/Droidspaces-OSS/actions?query=event%3Apush) [![Telegram](https://img.shields.io/badge/Droidspaces-Telegram-2CA5E0?style=flat-square&logo=telegram&logoColor=white)](https://t.me/Droidspaces)

&nbsp;

| Feature | Status |
|---------|:------:|
| PID / IPC / UTS / NET / Mount namespaces | ✅ |
| cgroup v1 — cpu, cpuacct, memory, blkio, pids, devices | ✅ |
| cgroup v2 — cpu, cpuset, io, memory, pids | ✅ |
| OverlayFS with redirect + index | ✅ |
| VETH / BRIDGE / BRIDGE\_NETFILTER | ✅ |
| TUN / FUSE | ✅ |
| POSIX\_MQUEUE / SYSVIPC | ✅ |
| Docker inside container (NAT mode) | ✅ |

&nbsp;

<details>
<summary><strong>Droidspaces Container Configurations</strong></summary>
<br>

### Network Mode

| Mode | Description | Use when |
|------|-------------|----------|
| **Host (Default)** | Container shares Android's network namespace directly | General use, simple internet access |
| **NAT (Isolated)** | Container gets its own network namespace routed via upstream interface | Network isolation, port forwarding, or tools that manipulate iptables |
| **None (Air-gapped)** | No network access | Offline workloads only |

**NAT Settings** *(visible only when NAT mode is selected):*

| Setting | Description |
|---------|-------------|
| **Static IP** | Set a static IPv4 within the `172.28.x.x` subnet via Octet 3 & 4 fields |
| **Upstream Interfaces** | Mandatory — add `wlan0` (WiFi) or `rmnet_data0` (mobile data) to route traffic |
| **Port Forwarding** | Map host ports to container ports (single e.g. `8080` or range e.g. `8000-8010`) |
| **DNS Servers** | Configure custom DNS for the container |
| **IPv6** | Always disabled in NAT and None modes — only available in Host mode |

### Integration & Hardware

| Setting | Description |
|---------|-------------|
| **Android Storage** | Mount Android storage directly into the container |
| **Hardware Access** | Grant full hardware access to the container |
| **Configure Termux X11** | Enable X11 socket mounting for Termux-X11 GUI support |

### Security & Boot

| Setting | Default | Notes |
|---------|---------|-------|
| **SELinux Permissive** | OFF | Enable only if you hit confirmed SELinux denials inside the container |
| **Volatile Mode** | OFF | RAM-only mode — all changes are discarded on container stop |
| **Force Cgroup V1** | OFF | Enable on 4.14 NonGKI kernels for correct cgroup operation |
| **Manual Deadlock Shield** | OFF | Prevents VFS `grab_super()` deadlocks on legacy kernels — **enabling this blocks Docker, Podman, LXC, and systemd sandboxing inside the container** |
| **Run at Boot** | OFF | Automatically start the container on device boot |

### Advanced Options

| Setting | Description |
|---------|-------------|
| **Environment Variables** | Set custom env vars in `KEY=VALUE` format, one per line |
| **Bind Mounts** | Mount host paths directly into the container |

### Storage Configuration

| Setting | Description |
|---------|-------------|
| **Sparse Image** | Creates a dedicated ext4 filesystem image mounted as a loop device — recommended for encrypted storage and F2FS `/data` partitions to avoid SELinux and keyring compatibility issues |

### Global Settings

| Setting | Description |
|---------|-------------|
| **Reinstall Backend** | Reinstalls the Droidspaces binaries and boot module |
| **Daemon Mode** | Runs Droidspaces as a persistent system-level daemon to bypass seccomp blocks from Magisk/APatch, GrapheneOS, etc. — enables high-privilege operations by fetching and running commands from userspace. Reboot required |
| **Integrate Droidspaces to System Path** | Creates a symlink at `/system/bin` to access the `droidspaces` command in Android root shell. ⚠️ **Causes bootloop if your mount system uses OverlayFS meta modules** — only use with Magic Mount-based mount systems. Reboot required |
| **Requirements** | Built-in kernel compatibility checker — verifies that all required kernel configs for Droidspaces are present on your device |

</details>

---

### Docker Support

All **Generally Necessary** configs required for Docker support are included and pass the [Moby `check-config.sh`](https://github.com/moby/moby/blob/master/contrib/check-config.sh) validation script. Optional configs viable on this source tree are also included where possible.

**Known limitations in this source tree:**
- `CFS_BANDWIDTH` is blocked by `SCHED_WALT=y` — Qualcomm's WALT scheduler and CFS bandwidth enforcement are mutually exclusive in this tree.
- `NFT_FIB` has no selectable Kconfig symbol in this tree.

&nbsp;

#### Running in Droidspaces (Recommended)

The recommended and fully supported way to run Docker on this kernel is inside a [Droidspaces](https://github.com/ravindu644/Droidspaces-OSS) container using **NAT (Isolated)** network mode. This gives Docker a proper isolated Linux environment with full iptables control, bridge networking, and internet access — with no additional configuration or workarounds required.

**Step 1 — Configure the Droidspaces Container**

Before installing Docker, configure the container in the Droidspaces app:

| Setting | Value | Why |
|---------|-------|-----|
| **Network Mode** | NAT (Isolated) | Provides the container with its own isolated network namespace and full iptables control |
| **Upstream Interface** | `wlan0` (WiFi) or `rmnet_data0` (mobile data) | Routes all container internet traffic through the active Android network interface |
| **Manual Deadlock Shield** | **OFF** ⚠️ | Must be disabled — this setting blocks nested namespace creation, which Docker fundamentally requires to function |
| **Force Cgroup V1** | **ON** | Required for correct cgroup operation on 4.14 NonGKI kernels |

**Step 2 — Install Docker**

Inside the Droidspaces container as root, use the official install script:

```bash
curl -fsSL https://get.docker.com -o install-docker.sh
sh install-docker.sh
```

No `daemon.json` configuration is needed. In NAT (Isolated) mode Docker runs with its defaults — iptables works fully, bridge networking functions correctly, and `overlay2` is the storage driver automatically.

**Step 3 — Verify Docker is running:**

```bash
docker run hello-world
```

```bash
docker run --rm alpine ping -c 3 8.8.8.8
```

Both should succeed. The ping test confirms full container internet access via the upstream interface.

**If Docker doesn't start automatically:**

Docker has no systemd service enabled by default inside the container. Start it manually:

```bash
dockerd &> /var/log/dockerd.log & disown
```

```bash
sleep 5 && tail -5 /var/log/dockerd.log
```

A healthy start shows:
```
level=info msg="Daemon has completed initialization"
level=info msg="API listen on /var/run/docker.sock"
```

Then re-run the verification:

```bash
docker run hello-world
```

```bash
docker run --rm alpine ping -c 3 8.8.8.8
```

To auto-start `dockerd` on every container launch, add it to your shell profile:

**For systemd-based distros (Ubuntu, Debian, Fedora):**
```bash
systemctl enable docker
```

**For non-systemd / OpenRC / all other cases:**
```bash
echo 'dockerd &> /var/log/dockerd.log &' >> /etc/profile
```

> [!NOTE]
> If your Droidspaces container is running in **Host** network mode, Docker requires `--iptables=false` to be set — Android's host network namespace blocks netfilter writes from userspace, causing iptables chain creation to fail. Add the following to `/etc/docker/daemon.json`:
> ```json
> {
>     "iptables": false,
>     "ip6tables": false,
>     "storage-driver": "overlay2"
> }
> ```
> Note that in Host mode, containers share Android's network namespace and have no network isolation. **NAT (Isolated) mode is strongly recommended for full Docker functionality.**

&nbsp;

##### Docker Troubleshooting — Droidspaces (Ubuntu/Debian)

**`Cannot connect to the Docker daemon at unix:///var/run/docker.sock`**

The daemon is not running. Start it:

```bash
dockerd &> /var/log/dockerd.log & disown
```

```bash
sleep 5 && tail -5 /var/log/dockerd.log
```

**`failed to start daemon: process with PID X is still running`**

A previous session left a stale pid file:

```bash
kill <PID> 2>/dev/null
rm -f /var/run/docker.pid
dockerd &> /var/log/dockerd.log & disown
```

**`Error initializing network controller` / bridge driver failure**

The Droidspaces container is in **Host (Default)** network mode. Switch to **NAT (Isolated)** in the container editor, save, restart the container, and retry.

**Daemon exits immediately / no output**

Check the log:

```bash
cat /var/log/dockerd.log
```

If the log mentions namespace or clone errors, **Manual Deadlock Shield** is ON. Disable it under **Security & Boot** in the Droidspaces container editor and restart the container.

**`APT::Sandbox::User` error when running apt inside a container**

Ubuntu/Debian requires the root sandbox to be disabled inside Docker containers:

```bash
echo 'APT::Sandbox::User "root";' > /etc/apt/apt.conf
```

**Docker not starting automatically on container launch**

```bash
systemctl enable docker
systemctl start docker
```

&nbsp;

#### Running in Termux (Native)

Running Docker directly in Termux — without any chroot or container — is possible on this kernel. To build Docker for Termux, follow the **[Docker on Android guide by FreddieOliveira](https://gist.github.com/FreddieOliveira/efe850df7ff3951cb62d74bd770dce27)** for the full compilation procedure, then apply the configuration below to run it correctly on Termux.

> [!IMPORTANT]
> When running Docker natively in Termux, `--iptables=false` and `--ip6tables=false` are always required. Android's host network namespace does not permit userspace netfilter modifications — Docker's attempt to create iptables chains for bridge networking fails at the kernel level regardless of root access or kernel capabilities. As a result, Docker bridge networking and container network isolation are not available in Termux; all containers share Android's host network namespace. For full Docker functionality with proper bridge networking and container isolation, use the [Droidspaces path](#running-in-droidspaces-recommended) above.

Configure `/data/docker/run/docker/daemon.json`:

```json
{
    "data-root": "/data/docker/lib/docker",
    "exec-root": "/data/docker/run/docker",
    "pidfile": "/data/docker/run/docker.pid",
    "hosts": [
        "unix:///data/docker/run/docker.sock"
    ],
    "iptables": false,
    "ip6tables": false,
    "storage-driver": "overlay2"
}
```

> [!NOTE]
> Docker stores all data — containers, images, volumes — under `/data/docker`. This location is intentional: Android mounts `/data/data` with options that prevent `overlay2` from working correctly. Be aware that formatting your device or flashing a ROM will erase this directory entirely.

**Starting Docker and auto-start on boot**

Install a terminal multiplexer to run the daemon and containers in separate panes:

```bash
pkg install tmux
```

Start the daemon in one tmux pane:

```bash
sudo dockerd --iptables=false
```

Or run it in the background:

```bash
sudo dockerd &>/dev/null &
```

To auto-start on every Termux launch, add to `~/.bashrc` or `~/.profile`:

```bash
# Start Docker daemon automatically if not running
RUNNING=`ps aux | grep dockerd | grep -v grep`
if [ -z "$RUNNING" ]; then
    sudo dockerd &>/dev/null &
fi
```

> [!NOTE]
> Since Termux does not use systemd, `systemctl enable docker` is not available. The profile-based approach above is the recommended method for auto-starting `dockerd` in Termux. The Docker socket is located at `/data/docker/run/docker.sock` — if you get a connection error, set the host explicitly:
> ```bash
> export DOCKER_HOST=unix:///data/docker/run/docker.sock
> ```

&nbsp;

##### Docker Troubleshooting — Termux (Native)

**`iptables: No chain/target/match by that name`**

Add `"iptables": false` and `"ip6tables": false` to `/data/docker/run/docker/daemon.json` and restart `dockerd`.

**Cannot connect to Docker socket**

The socket path in Termux is non-standard. Set the host explicitly:

```bash
export DOCKER_HOST=unix:///data/docker/run/docker.sock
```

Add it to `~/.bashrc` to make it permanent:

```bash
echo 'export DOCKER_HOST=unix:///data/docker/run/docker.sock' >> ~/.bashrc
```

**`dockerd` not starting automatically**

Add to `~/.bashrc` or `~/.profile`:

```bash
RUNNING=`ps aux | grep dockerd | grep -v grep`
if [ -z "$RUNNING" ]; then
    sudo dockerd &>/dev/null &
fi
```

&nbsp;

<details>
<summary><strong>Enabled Docker Kernel Configs</strong></summary>
<br>

| Category | Config Symbols |
|----------|----------------|
| Core netfilter | `NETFILTER=y`, `NETFILTER_ADVANCED=y`, `NETFILTER_XTABLES=y`, `NETFILTER_INGRESS=y` |
| IPv4 tables | `IP_NF_IPTABLES=y`, `IP_NF_FILTER=y`, `IP_NF_TARGET_REJECT=y`, `IP_NF_NAT=y`, `IP_NF_TARGET_MASQUERADE=y`, `IP_NF_MANGLE=y`, `IP_NF_RAW=y`, `IP_NF_SECURITY=y` |
| IPv6 NAT | `IP6_NF_IPTABLES=y`, `IP6_NF_NAT=y`, `IP6_NF_TARGET_MASQUERADE=y` |
| IPVS | `NETFILTER_XT_MATCH_IPVS=y`, `IP_VS=y`, `IP_VS_NFCT=y`, `IP_VS_RR=y`, `IP_VS_PROTO_TCP=y`, `IP_VS_PROTO_UDP=y` |
| nftables | `NF_TABLES=y`, `NFT_CT=y`, `NFT_MASQ=y`, `NFT_NAT=y` |
| Network drivers | `MACVLAN=y`, `IPVLAN=y`, `VXLAN=y`, `BRIDGE_VLAN_FILTERING=y` |
| Namespaces | `NAMESPACES=y`, `PID_NS=y`, `NET_NS=y`, `IPC_NS=y`, `UTS_NS=y`, `USER_NS=y` |
| cgroup controls | `CGROUPS=y`, `BLK_DEV_THROTTLING=y`, `CGROUP_PERF=y`, `NET_CLS_CGROUP=y`, `CGROUP_NET_PRIO=y` |
| Overlay storage | `OVERLAY_FS=y`, `OVERLAY_FS_REDIRECT_DIR=y`, `OVERLAY_FS_INDEX=y` |
| Misc | `POSIX_MQUEUE=y`, `SYSVIPC=y`, `TUN=y`, `FUSE_FS=y` |

</details>

---

## Building Locally

### Requirements

| Requirement | Details |
|-------------|---------|
| **OS** | Any modern Linux. Tested on Ubuntu 22.04+, Fedora 40, Arch, openSUSE Tumbleweed |
| **RAM / disk** | 8 GB RAM minimum (16 GB recommended) — ~30 GB free disk |
| **Tools** | `git`, `make`, `bash`, `curl`, `tar`, `python3`, `ccache`, plus an ARM cross-compiler |

The builder picks the right install command for your distro. Examples:

```bash
# Debian / Ubuntu
sudo apt-get install -y git make bc bison flex zip unzip rsync python3 \
    build-essential ccache gcc-aarch64-linux-gnu gcc-arm-linux-gnueabi libssl-dev

# Fedora / RHEL
sudo dnf install -y git make bc bison flex zip unzip rsync python3 \
    @development-tools ccache gcc-aarch64-linux-gnu gcc-arm-linux-gnueabi openssl-devel

# Arch
sudo pacman -S --needed git make bc bison flex zip unzip rsync python3 \
    base-devel ccache aarch64-linux-gnu-gcc arm-linux-gnueabihf-gcc
```

The exact command is also surfaced inside the TUI under **Setup ▸ Install hints**, populated for the package manager that was detected on the host.

### Quick start

```bash
git clone -b 16-ReSukiSU https://github.com/Rsool22/android_kernel_vayu
cd android_kernel_vayu
chmod +x build.sh
./build.sh
```

`build.sh` is a ~90-line wrapper. On first invocation it downloads the prebuilt
`vayu-builder` binary that matches your OS and CPU (linux-amd64 or linux-arm64)
into `~/.cache/vayu-builder/`, then execs it. Re-runs reuse the cached binary;
delete the cache directory to force a refresh, or pin a specific release with
`VAYU_BUILDER_VERSION=tui-v1.2.3 ./build.sh`.

> [!NOTE]
> The directory name doesn't matter. The builder finds the kernel root by
> looking for the standard markers (`Makefile` with `VERSION =`, `arch/arm64/`,
> `kernel/`), so renaming or moving the source tree won't break anything.

If you'd rather run the builder from source (e.g. while iterating on the TUI):

```bash
cd tools/builder-tui
go build -o ../../bin/vayu-builder .
../../build.sh   # the wrapper auto-detects bin/vayu-builder
```

---

## The vayu-builder TUI

`vayu-builder` is a [Bubble Tea](https://github.com/charmbracelet/bubbletea)-based
TUI written in Go. It replaces the old 3,000-line bash script wholesale; every
piece of logic the bash version did (autodiscovery, clang fetching, ReSukiSU
management, build orchestration, AnyKernel3 packaging) now lives in
`tools/builder-tui/`.

### Main menu

```
VAYU  KERNEL  BUILDER         Android 16 / NonGKI / SM8150
─────────────────────────────────────────────────────────────
  Kernel       /home/.../android_kernel_vayu
  Clang        /home/.../clang        (clang version 19.x)
  AnyKernel3   /home/.../AnyKernel3
  Distro       apt

  [B]  Build kernel       (compile + package)
  [T]  Toolchain manager  (Google / ZyC clang)
  [K]  ReSukiSU driver    (install / update)
  [S]  Setup / paths      (deps + config)
  [Q]  Quit
```

Each non-trivial action is a separate screen reachable from the main menu.

### Toolchain Manager `[T]`

Pick where clang comes from:

| Source | Origin | When to use |
|---|---|---|
| **Auto (default)** | Google AOSP first; falls back to ZyC if gitiles is unreachable | recommended — best of both worlds |
| **Google** | `android.googlesource.com/.../clang/host/linux-x86` (`main-kernel`) | match what AOSP itself uses |
| **ZyC** | `github.com/ZyCromerZ/Clang` releases | older revisions, e.g. ZyC 23.x for compatibility |

Hotkeys: `[A]` Auto, `[G]` Google, `[Z]` ZyC, `[F]` Fetch (download), `[C]` Check (resolve only). Selection persists at `~/.config/vayu_builder/config`.

For ZyC, an additional sub-menu lets you target a specific major version
(`[2]` 23.x, `[1]` 15.x, `[L]` latest of any version).

### ReSukiSU Driver Manager `[K]`

The screen probes both the `main` and `dev` branches of
[ReSukiSU/ReSukiSU](https://github.com/ReSukiSU/ReSukiSU) on entry, distinguishing:

| State | Meaning | Action |
|---|---|---|
| `present` (with SHA) | branch exists | install/update OK |
| `absent` (branch removed or merged) | upstream removed it | refuse to install; CI skips this leg |
| `network unreachable` | no connectivity | retry later |

Hotkeys: `[I]` Install/update from active branch, `[S]` Switch (main ⇄ dev),
`[P]` Re-probe upstream, `[ESC]` Return.

> [!NOTE]
> The probe uses `git ls-remote --exit-code`, so we get distinct exit codes
> for "ref missing" (2) vs "git failed" (128). When `dev` looks unreachable we
> double-check by probing `main` -- only if both fail do we declare a network
> failure.

### Setup `[S]`

Shows everything autodiscovery resolved (kernel root, clang dir, AnyKernel3,
output dir, `aarch64-` and `arm-` cross compilers) plus the install command
appropriate for your distro for any missing tools. `[R]` re-runs discovery.

### Build `[B]`

Runs `make O=$out vayu_defconfig` then `make Image.gz-dtb`. Compile output
streams live into a scrollable viewport. On success the resulting kernel image
is packaged into an AnyKernel3 zip. Internally:

* `LLVM=1 LLVM_IAS=1` always set
* `CC=ccache clang` when ccache is on `$PATH`
* `CROSS_COMPILE` and `CROSS_COMPILE_ARM32` derived from the resolved cross
  compilers (`aarch64-linux-gnu-`, `arm-linux-gnueabi-`)
* `KBUILD_OUTPUT=$out` so the source tree stays clean

### CLI subcommands

The same binary serves CI:

```bash
vayu-builder build [--defconfig vayu_defconfig] [--package] [--output kernel.zip]
vayu-builder fetch-clang --source auto|google|zyc --target latest --into ./clang [--check]
vayu-builder probe [--json]
vayu-builder paths [--json]
vayu-builder version
```

Exit codes for `probe`: `0` at least one branch present; `2` both absent;
`3` network failure.

---

## GitHub Actions CI

`.github/workflows/build-dual.yml` runs the kernel build daily and on manual
dispatch. The pipeline is a four-stage DAG:

```
reset-counter (manual)  →  probe  →  build (matrix: main, dev)  →  release
```

### Probe job

Calls `vayu-builder probe --json` to find out which ReSukiSU branches are
available upstream. Outputs `main_available`, `dev_available`, `main_sha`,
`dev_sha`, and a top-level `should_build`.

* If both branches are absent, `should_build=false` and the rest of the
  pipeline short-circuits — **no empty release is published**.
* If the network probe outright fails (vayu-builder exits 3), the job fails
  loudly so the run is visibly broken rather than silently empty.

### Build matrix

```yaml
strategy:
  matrix:
    ksu_branch: [main, dev]
```

Each leg is gated on its own availability:

```yaml
env:
  KSU_BRANCH_AVAILABLE: ${{ matrix.ksu_branch == 'main'
      && needs.probe.outputs.main_available
      || needs.probe.outputs.dev_available }}
```

Steps inside each leg are wrapped in `if: env.KSU_BRANCH_AVAILABLE == 'true'`,
so an absent branch produces a `::warning::` and skips cleanly without breaking
the matrix.

For each available leg the job:

1. Checks out the source.
2. Builds `vayu-builder` from `tools/builder-tui/` (`go build -trimpath`).
3. Installs host deps (`build-essential`, `gcc-aarch64-linux-gnu`,
   `gcc-arm-linux-gnueabi`, `jq`, etc.).
4. Restores ccache.
5. Calls `.github/scripts/fetch_clang.sh --source "$CLANG_SOURCE"` (default
   `auto` → Google with ZyC fallback).
6. Runs ReSukiSU `setup.sh` for the matrix branch.
7. Runs `.github/scripts/ci_build.sh`, which delegates to
   `vayu-builder build --package`.
8. Uploads the produced zip as artifact `kernel-${ksu_branch}`.

### Release job

Runs only if `probe.should_build == 'true'` and at least one branch was
available. It downloads whatever artifacts the build matrix produced
(one or both branches), composes release notes that explicitly call out which
branch was skipped (if any), and publishes a rolling pre-release tagged
`rolling-build` plus an immutable numbered tag `rolling-build-<run_number>`.

### Behaviour summary

| Upstream state | What CI does | What's published |
|---|---|---|
| both `main` and `dev` present | full matrix builds | release with both zips |
| only `main` present | dev leg skipped (warning) | release with main zip only, dev marked _skipped_ in notes |
| only `dev` present | main leg skipped (warning) | release with dev zip only, main marked _skipped_ in notes |
| both absent (upstream merged dev into main and removed `main` too) | pipeline short-circuits at probe | no release; previous release stays as latest |
| network failure | probe job fails | no release; run is visibly red |

### Toolchain selection in CI

The clang source defaults to `auto` (Google → ZyC fallback). To force a
specific source on a manual run, use the workflow_dispatch input
`clang_source`:

* `auto` (default) — Google AOSP, ZyC fallback
* `google` — Google AOSP only
* `zyc` — ZyC Clang only (handy if AOSP gitiles is having a bad day)

### Releasing the builder binary itself

`.github/workflows/release-builder-tui.yml` builds the `vayu-builder` binary
for `linux-amd64` and `linux-arm64` and publishes them as release assets.
Triggers:

* push to `16-ReSukiSU` touching `tools/builder-tui/**` → rolling
  `tui-latest` pre-release
* tag `tui-vX.Y.Z` → versioned release
* `workflow_dispatch` for manual rebuilds

The `build.sh` wrapper resolves `tui-latest` by default, which is why a fresh
clone of the repo always picks up the newest TUI without needing `go` locally.

---

## Credits

| Project | Author(s) | Role |
|---------|-----------|------|
| [LineageOS SM8150 kernel](https://github.com/LineageOS/android_kernel_qcom_sm8150) | LineageOS | Original Android 4.14 kernel source base for Qualcomm SM8150 devices |
| [AnymoreProject kernel (vayu)](https://github.com/AnymoreProject/android_kernel_vayu) | AnymoreProject | Device-specific kernel tree for vayu, forked from the LineageOS SM8150 base |
| [ReSukiSU](https://github.com/ReSukiSU/ReSukiSU) | ReSukiSU Team | Kernel-based root solution forked from SukiSU Ultra, focused on non-GKI stability |
| [SukiSU Ultra](https://github.com/SukiSU-Ultra/SukiSU-Ultra) | SukiSU Ultra Team | Upstream root solution that ReSukiSU is forked from |
| [SUSFS for KSU](https://gitlab.com/simonpunk/susfs4ksu) | simonpunk | Kernel-level filesystem patches for root concealment |
| [susfs4ksu module](https://github.com/sidex15/susfs4ksu-module) | sidex15 | Userspace companion module for SUSFS-patched kernels |
| [sidex15 SM8150 reference](https://github.com/sidex15/android_kernel_lge_sm8150) | sidex15 | Reference SM8150 kernel tree used for SUSFS cherry-picks |
| [KernelPatch / KPM](https://github.com/bmax121/KernelPatch) | bmax121 | Runtime kernel patching framework and KPM module support |
| [ZyC Clang](https://github.com/ZyCromerZ/Clang) | ZyCromerZ | Prebuilt LLVM/Clang toolchain used for kernel compilation |
| [AnyKernel3](https://github.com/osm0sis/AnyKernel3) | osm0sis | Ramdisk-less kernel flashing framework |
| [Droidspaces](https://github.com/ravindu644/Droidspaces-OSS) | ravindu644 | Lightweight, portable LXC-inspired container runtime for Android |
| [OrangeFox Recovery](https://orangefox.download/device/61310755bb6a91af6a656d0d) | OrangeFox Team | Feature-rich custom recovery for the Poco X3 Pro |
| [Docker on Android](https://gist.github.com/FreddieOliveira/efe850df7ff3951cb62d74bd770dce27) | FreddieOliveira | Comprehensive guide and patches for running Docker natively in Termux |
| [Moby check-config.sh](https://github.com/moby/moby/blob/master/contrib/check-config.sh) | Moby Project | Official Docker kernel configuration validation script |
| [Awesome Android Root](https://github.com/awesome-android-root/awesome-android-root#root-apps-and-modules) | Community | Curated collection of root apps, modules, and resources |
| [bindhosts](https://github.com/bindhosts/bindhosts) | bindhosts | Systemless hosts solution for KernelSU-based roots |
| [Keep Android Open](https://keepandroidopen.org) | Community | Initiative advocating for an open and unlocked Android ecosystem |

---

<details>
<summary><strong>Reference Links</strong></summary>
<br>

| Resource | URL |
|----------|-----|
| ReSukiSU Repository | https://github.com/ReSukiSU/ReSukiSU |
| ReSukiSU Documentation | https://resukisu.github.io |
| ReSukiSU Telegram Channel | https://t.me/ReSukisu |
| ReSukiSU CI Builds | https://github.com/ReSukiSU/ReSukiSU/actions?query=event%3Apush |
| SukiSU Ultra Repository | https://github.com/SukiSU-Ultra/SukiSU-Ultra |
| SukiSU Ultra Latest Release | https://github.com/tiann/KernelSU/releases/latest |
| SukiSU Ultra License | https://github.com/SukiSU-Ultra/SukiSU-Ultra/blob/main/LICENSE |
| SUSFS for KSU | https://gitlab.com/simonpunk/susfs4ksu |
| susfs4ksu Companion Module | https://github.com/sidex15/susfs4ksu-module |
| susfs4ksu Module CI Builds | https://nightly.link/sidex15/susfs4ksu-module/workflows/build/v1.5.2%2B?preview |
| susfs4ksu Module Documentation | https://github.com/sidex15/susfs4ksu-module/wiki |
| sidex15 SM8150 Reference Tree | https://github.com/sidex15/android_kernel_lge_sm8150 |
| KernelPatch / KPM Framework | https://github.com/bmax121/KernelPatch |
| ZyC Clang Releases | https://github.com/ZyCromerZ/Clang/releases |
| AnyKernel3 | https://github.com/osm0sis/AnyKernel3 |
| AnymoreProject Vayu Kernel Tree | https://github.com/AnymoreProject/android_kernel_vayu |
| LineageOS SM8150 Kernel | https://github.com/LineageOS/android_kernel_qcom_sm8150 |
| OrangeFox Recovery (vayu) | https://orangefox.download/device/61310755bb6a91af6a656d0d |
| Droidspaces Repository | https://github.com/ravindu644/Droidspaces-OSS |
| Droidspaces Latest Release | https://github.com/ravindu644/Droidspaces-OSS/releases/latest |
| Droidspaces CI Builds | https://github.com/ravindu644/Droidspaces-OSS/actions?query=event%3Apush |
| Droidspaces Rootfs Releases | https://github.com/ravindu644/Droidspaces-rootfs-builder/releases/latest |
| Droidspaces Android Setup Guide | https://github.com/ravindu644/Droidspaces-OSS/blob/main/Documentation/Installation-Android.md |
| Droidspaces Telegram Channel | https://t.me/Droidspaces |
| Docker on Android — FreddieOliveira | https://gist.github.com/FreddieOliveira/efe850df7ff3951cb62d74bd770dce27 |
| Moby Docker Kernel Check Script | https://github.com/moby/moby/blob/master/contrib/check-config.sh |
| Awesome Android Root | https://github.com/awesome-android-root/awesome-android-root#root-apps-and-modules |
| bindhosts — Systemless Hosts | https://github.com/bindhosts/bindhosts |
| Keep Android Open | https://keepandroidopen.org |
| GPL-2.0 License | https://www.gnu.org/licenses/old-licenses/gpl-2.0.en.html |
| ReSukiSU GPL-3.0 License | https://github.com/ReSukiSU/ReSukiSU/blob/main/LICENSE |
| Poco X3 Pro — XDA Forums | https://xdaforums.com/f/xiaomi-poco-x3-pro.12163/ |
| KernelSU Non-GKI Integration Guide | https://kernelsu.org/guide/how-to-integrate-for-non-gki.html |
| AnyMore Discuss — Telegram | https://t.me/+fh4NvUnmcWM4ODE1 |
| Poco X3 Pro Updates — Telegram | https://t.me/PocoX3ProUpdates |

</details>

---

> [!WARNING]
> **Disclaimer:** Flashing custom kernels may void your device warranty. I am not responsible for bricked devices, boot loops, data loss, or any other damage. Always back up your data and boot partition before flashing. **You do this entirely at your own risk.**

---

<div align="center">

---

**VAYU-AnyMore-Project** · Poco X3 Pro (`vayu` · `bhima`) · SM8150 · Linux 4.14 NonGKI

ReSukiSU · SUSFS v2.1.0 Inline-Hook · KPM · Docker · Droidspaces

Android 11 ~ 16 · ZyC Clang 23.x · LLVM/LLD · SELinux Enforcing

*Built with ❤️ for the Poco X3 Pro community*

</div>
