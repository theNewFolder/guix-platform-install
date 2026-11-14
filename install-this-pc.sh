#!/run/current-system/profile/bin/bash
# Universal Hardware Launcher (supports any PC/Workstation/Laptop/Server).
# Run from the Guix live ISO as root:
#   bash /path/to/guix-platform-install/install-this-pc.sh
#
# Defaults:
#   - GUIX_PLATFORM=generic (auto-detects UEFI/BIOS, CPU vendor, NVMe/SATA/VirtIO/eMMC, RAM/cores)
#   - TARGET_MODEL="${TARGET_MODEL:-}" (optional model pin, e.g., "Samsung SSD 990 PRO 2TB";
#     if unset and multiple disks exist, the installer opens an interactive multi-disk selector
#     so secondary "cold"/"work" drives are never touched)
#   - USE_NONGUIX="${USE_NONGUIX:-no}" (100% pure GNU Guix linux-libre by default;
#     set USE_NONGUIX=yes for hardware requiring non-free Wi-Fi/GPU firmware)
# ASCII only: the Guix ISO terminal cannot render Unicode.
set -euo pipefail

TARGET_MODEL="${TARGET_MODEL:-}"
TARGET_SERIAL="${TARGET_SERIAL:-}"
USE_NONGUIX="${USE_NONGUIX:-no}"
GUIX_PLATFORM="${GUIX_PLATFORM:-generic}"
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK=/tmp/gpi

if [[ $EUID -ne 0 ]]; then
    echo "[ERROR] Run as root (sudo -i first)."
    exit 1
fi

if [[ ! -e /run/current-system/profile/bin/guix && ! -x "$(command -v guix || true)" ]]; then
    echo "[ERROR] guix not found. Boot the Guix live ISO first."
    exit 1
fi

echo "== Block Devices =="
lsblk -d -o NAME,SIZE,TRAN,MODEL,SERIAL
echo

if [[ -z "${DEVICE:-}" && -n "$TARGET_MODEL" ]]; then
    mapfile -t matches < <(lsblk -d -n -o NAME,MODEL | awk -v m="$TARGET_MODEL" 'index($0, m) {print $1}')
    if [[ ${#matches[@]} -ne 1 ]]; then
        echo "[ERROR] Expected exactly one '$TARGET_MODEL', found ${#matches[@]}. Aborting."
        exit 1
    fi
    DEVICE="/dev/${matches[0]}"
    echo "[OK] Target disk pinned by model: $DEVICE ($TARGET_MODEL)"
    echo "     All other drives (cold/work) will NOT be touched."
    echo
fi

echo "== Network check =="
if ping -c1 -W3 bordeaux.guix.gnu.org >/dev/null 2>&1 || ping -c1 -W3 ci.guix.gnu.org >/dev/null 2>&1; then
    echo "[OK] Online (Guix substitute mirrors reachable)"
else
    echo "[ERROR] No network. Plug in Ethernet or USB tethering."
    exit 1
fi
echo

read -r -p "Username (lowercase) [${USER_NAME:-guix}]: " input_user </dev/tty
USER_NAME="${input_user:-${USER_NAME:-guix}}"
read -r -p "Full name [${FULL_NAME:-Guix User}]: " input_name </dev/tty
FULL_NAME="${input_name:-${FULL_NAME:-Guix User}}"
read -r -p "Timezone (e.g. Asia/Dubai) [${TIMEZONE:-Asia/Dubai}]: " input_tz </dev/tty
TIMEZONE="${input_tz:-${TIMEZONE:-Asia/Dubai}}"
read -r -p "Hostname [${HOST_NAME:-guix}]: " input_host </dev/tty
HOST_NAME="${input_host:-${HOST_NAME:-guix}}"

if [[ ! -f "/run/current-system/profile/share/zoneinfo/$TIMEZONE" && ! -f "/usr/share/zoneinfo/$TIMEZONE" ]]; then
    echo "[WARN] Could not verify timezone '$TIMEZONE' locally; continuing anyway."
fi

echo
echo "Summary:"
echo "  GUIX_PLATFORM = $GUIX_PLATFORM"
echo "  DEVICE        = ${DEVICE:-auto-select (multi-disk safe)}"
echo "  TARGET_MODEL  = ${TARGET_MODEL:-none}"
echo "  USE_NONGUIX   = $USE_NONGUIX (default: no = pure linux-libre)"
echo "  USER_NAME     = $USER_NAME"
echo "  FULL_NAME     = $FULL_NAME"
echo "  TIMEZONE      = $TIMEZONE"
echo "  HOST_NAME     = $HOST_NAME"
read -r -p "Proceed? [y/N] " ok </dev/tty
[[ "$ok" == "y" || "$ok" == "Y" ]] || { echo "Aborted."; exit 1; }

# Copy repo off the USB stick (skip macOS ._ junk) and build the installer.
rm -rf "$WORK"
mkdir -p "$WORK" /tmp/go-cache /tmp/go-tmp
tar -C "$SRC" --exclude='._*' --exclude='.Spotlight-V100' --exclude='.git' -cf - . | tar -C "$WORK" -xf -
cd "$WORK"
export GOCACHE=/tmp/go-cache GOTMPDIR=/tmp/go-tmp
echo "Building installer (fetching Go via guix shell)..."
guix shell go -- go build -o run-remote-steps .

export GUIX_PLATFORM USE_NONGUIX TARGET_MODEL TARGET_SERIAL
export DEVICE="${DEVICE:-}"
export USER_NAME FULL_NAME TIMEZONE HOST_NAME
export SWAP_SIZE="${SWAP_SIZE:-8G}"

echo
echo "Starting installer..."
echo
./run-remote-steps
