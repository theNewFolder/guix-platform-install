# Universal Hardware Installer (`GUIX_PLATFORM=generic`)

The `generic` platform generalizes `guix-platform-install` across arbitrary desktops, workstations, laptops, servers, and virtual machines (`x86_64` and `aarch64`) without hardcoded per-device silos.

## Key Capabilities

1. **Multi-Disk Blast-Radius Protection (`SelectTargetDevice`)**:
   - Honors `DEVICE=/dev/nvmeXn1`, `TARGET_MODEL="Samsung SSD 990 PRO 2TB"`, and `TARGET_SERIAL="..."`.
   - Refuses to guess when multiple disks exist on a workstation: displays an interactive `/dev/tty` selection table so secondary data drives (`cold`, `work`, NAS arrays) are never touched.
2. **Universal Storage Bus & Partition Naming (`NeedsPartitionSeparatorP`)**:
   - Implements the Linux kernel `disk_name()` rule: automatically inserts `p` before partition numbers when the device name ends with a digit (`nvme0n1p1`, `mmcblk0p1`, `loop0p1`, `md0p1`, `nbd0p1`) and omits `p` for alphabetic devices (`sda1`, `vda1`, `xvda1`).
3. **Automatic Firmware & Architecture Matrix**:
   - Auto-detects `UEFI` (`/sys/firmware/efi`) vs `Legacy BIOS` (`bios_grub` + MBR GRUB target).
   - Supports both `x86_64` (`grubx64.efi`) and `aarch64` (`grubaa64.efi`).
   - Detects CPU vendor (`GenuineIntel`, `AuthenticAMD`, `ARM`), logical core count, and total RAM to tune swap size and `guix-daemon` parallelism (`--cores=0 --max-jobs=N`).
4. **Pure Libre GNU Guix Default (`linux-libre`) + Explicit Opt-In Hardware Firmware (`USE_NONGUIX=yes`)**:
   - Defaults to 100% FSF/GNU Free Software (`linux-libre`) and official GNU Guix substitute servers (`bordeaux.guix.gnu.org` and `ci.guix.gnu.org`).
   - Set `USE_NONGUIX=yes` only when hardware requires proprietary Wi-Fi/GPU firmware or CPU microcode (`linux`, `linux-firmware`, `microcode-initrd`).

## Usage from the Guix Live ISO

```bash
# Pure Free Software install (default) with multi-disk interactive selector:
GUIX_PLATFORM=generic ./run-remote-steps

# Pin target disk by model (never touches secondary drives):
GUIX_PLATFORM=generic TARGET_MODEL="Samsung SSD 990 PRO 2TB" ./run-remote-steps

# Opt into non-free kernel/firmware overlay for hardware requiring blobs:
GUIX_PLATFORM=generic USE_NONGUIX=yes TARGET_MODEL="Samsung SSD 990 PRO 2TB" ./run-remote-steps
```
