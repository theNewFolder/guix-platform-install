package lib

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// BlockDeviceInfo describes a physical block device discovered via lsblk / sysfs
type BlockDeviceInfo struct {
	Path       string // e.g., "/dev/nvme0n1"
	Name       string // e.g., "nvme0n1"
	Size       string // e.g., "1.8T"
	Model      string // e.g., "Samsung SSD 990 PRO 2TB"
	Serial     string // e.g., "S7H..."
	WWN        string // e.g., "eui.002538..."
	Transport  string // e.g., "nvme", "sata", "virtio", "usb", "mmc"
	Removable  bool
	Mountpoint string
}

// HardwareProfile captures the detected hardware capabilities of the target machine
type HardwareProfile struct {
	Arch             string   // "x86_64" or "aarch64"
	CPUVendor        string   // "GenuineIntel", "AuthenticAMD", "ARM", or "unknown"
	CPUModel         string   // Human-readable CPU model string
	CPUCores         int      // Logical CPU count
	TotalRAMMiB      int      // Total system memory in MiB
	RecommendedSwap  string   // e.g., "4G", "8G", "16G"
	BootMode         string   // "uefi" or "bios"
	HasNVMe          bool     // Whether any NVMe controller/device is present
	IsVM             bool     // Hypervisor / VM guest detected
	UseNonguix       bool     // Explicit opt-in via USE_NONGUIX=yes (defaults to false / pure linux-libre)
	InitrdModules    []string // Extra initrd kernel modules needed for boot
	KernelArguments  []string // Extra kernel command-line arguments
	BuildMaxJobs     int      // Recommended --max-jobs based on RAM and cores
	SubstituteMirrors string  // Space-separated substitute mirror list
}

// NeedsPartitionSeparatorP implements the Linux kernel disk_name() partition naming rule:
// if a block device name ends with a digit (nvme0n1, mmcblk0, loop0, nbd0, md0),
// Linux inserts a 'p' before the partition number (e.g., /dev/nvme0n1p1).
func NeedsPartitionSeparatorP(device string) bool {
	base := filepath.Base(strings.TrimSpace(device))
	if base == "" {
		return false
	}
	if strings.HasPrefix(base, "nvme") ||
		strings.HasPrefix(base, "mmcblk") ||
		strings.HasPrefix(base, "loop") ||
		strings.HasPrefix(base, "nbd") ||
		strings.HasPrefix(base, "md") {
		return true
	}
	last := base[len(base)-1]
	return last >= '0' && last <= '9'
}

// HasGRUBEFIBinary checks whether any valid GRUB EFI binary (x86_64 or aarch64) exists on /mnt/boot/efi
func HasGRUBEFIBinary() bool {
	candidates := []string{
		"/mnt/boot/efi/EFI/Guix/grubx64.efi",
		"/mnt/boot/efi/EFI/guix/grubx64.efi",
		"/mnt/boot/efi/EFI/Guix/grubaa64.efi",
		"/mnt/boot/efi/EFI/guix/grubaa64.efi",
		"/mnt/boot/efi/EFI/BOOT/BOOTX64.EFI",
		"/mnt/boot/efi/EFI/BOOT/BOOTAA64.EFI",
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// IsNonguixEnabled checks whether the user explicitly opted into non-free firmware/kernel
// via USE_NONGUIX=yes|1|true. By default, generic installs use 100% pure GNU Guix (linux-libre).
func IsNonguixEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("USE_NONGUIX")))
	return val == "yes" || val == "1" || val == "true"
}

// DiscoverBlockDevices enumerates all candidate physical disks on the host,
// filtering out loop, zram, optical (sr/rom), eMMC boot/rpmb partitions, and live ISO backing devices.
func DiscoverBlockDevices() ([]BlockDeviceInfo, error) {
	cmd := exec.Command("lsblk", "-d", "-P", "-n", "-o", "NAME,SIZE,TYPE,TRAN,RM,MOUNTPOINT,MODEL,SERIAL,WWN")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run lsblk: %w", err)
	}

	return ParseLsblkPairs(string(output)), nil
}

// ParseLsblkPairs parses `lsblk -d -P -n -o ...` key="value" lines into BlockDeviceInfo records.
// Exported for deterministic unit testing without requiring root or Linux block devices.
func ParseLsblkPairs(raw string) []BlockDeviceInfo {
	var devices []BlockDeviceInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := parseKeyValueLine(line)
		devType := fields["TYPE"]
		name := fields["NAME"]
		if devType != "disk" || name == "" {
			continue
		}
		if strings.HasPrefix(name, "loop") ||
			strings.HasPrefix(name, "zram") ||
			strings.HasPrefix(name, "sr") ||
			strings.HasPrefix(name, "ram") ||
			strings.Contains(name, "boot") ||
			strings.Contains(name, "rpmb") {
			continue
		}
		mountpoint := fields["MOUNTPOINT"]
		if mountpoint == "/iso" || strings.HasPrefix(mountpoint, "/run/initramfs") {
			continue
		}
		rm := fields["RM"] == "1" || strings.EqualFold(fields["RM"], "true")
		devices = append(devices, BlockDeviceInfo{
			Path:       "/dev/" + name,
			Name:       name,
			Size:       fields["SIZE"],
			Model:      strings.TrimSpace(fields["MODEL"]),
			Serial:     strings.TrimSpace(fields["SERIAL"]),
			WWN:        strings.TrimSpace(fields["WWN"]),
			Transport:  strings.TrimSpace(fields["TRAN"]),
			Removable:  rm,
			Mountpoint: mountpoint,
		})
	}
	return devices
}

// CheckDiskHasActiveHolders checks /sys/block/<disk>/holders and partition holders
// to prevent wiping disks that back active LVM, LUKS/dm-crypt, or mdadm RAID volumes.
func CheckDiskHasActiveHolders(device string) error {
	base := filepath.Base(strings.TrimSpace(device))
	if base == "" {
		return nil
	}
	holdersDir := filepath.Join("/sys/block", base, "holders")
	if entries, err := os.ReadDir(holdersDir); err == nil && len(entries) > 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return fmt.Errorf("refusing to wipe %s: active device-mapper/RAID holders detected (%s)", device, strings.Join(names, ", "))
	}
	return nil
}

func parseKeyValueLine(line string) map[string]string {
	res := make(map[string]string)
	i := 0
	n := len(line)
	for i < n {
		for i < n && line[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}
		eq := strings.IndexByte(line[i:], '=')
		if eq < 0 {
			break
		}
		key := line[i : i+eq]
		i += eq + 1
		if i < n && line[i] == '"' {
			i++
			start := i
			for i < n && line[i] != '"' {
				i++
			}
			res[key] = line[start:i]
			if i < n && line[i] == '"' {
				i++
			}
		} else {
			start := i
			for i < n && line[i] != ' ' {
				i++
			}
			res[key] = line[start:i]
		}
	}
	return res
}

// MatchBlockDeviceByModelOrSerial filters discovered devices by TARGET_MODEL and/or TARGET_SERIAL.
// Requires an unambiguous single match so secondary drives (e.g. "cold"/"work" data disks) are never touched.
func MatchBlockDeviceByModelOrSerial(devices []BlockDeviceInfo, targetModel, targetSerial string) (BlockDeviceInfo, error) {
	targetModel = strings.TrimSpace(targetModel)
	targetSerial = strings.TrimSpace(targetSerial)
	var matches []BlockDeviceInfo

	for _, d := range devices {
		modelOK := targetModel == "" || strings.Contains(strings.ToLower(d.Model), strings.ToLower(targetModel))
		serialOK := targetSerial == "" || strings.EqualFold(d.Serial, targetSerial)
		if modelOK && serialOK {
			matches = append(matches, d)
		}
	}

	if len(matches) == 0 {
		return BlockDeviceInfo{}, fmt.Errorf("no disk matched TARGET_MODEL=%q TARGET_SERIAL=%q (discovered %d disk(s))", targetModel, targetSerial, len(devices))
	}
	if len(matches) > 1 {
		var names []string
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%s (%s %s)", m.Path, m.Model, m.Size))
		}
		return BlockDeviceInfo{}, fmt.Errorf("ambiguous disk match for TARGET_MODEL=%q TARGET_SERIAL=%q: matched %d disks [%s]; set DEVICE or TARGET_SERIAL explicitly",
			targetModel, targetSerial, len(matches), strings.Join(names, ", "))
	}
	return matches[0], nil
}

// SelectTargetDevice safely resolves the target disk for installation:
//  1. Explicit DEVICE env var (if valid block device)
//  2. Explicit TARGET_MODEL and/or TARGET_SERIAL env vars (requires exact 1 match)
//  3. Single non-removable disk auto-selection
//  4. Interactive selection prompt via /dev/tty when multiple disks are present (multi-disk safety gate)
func SelectTargetDevice(platform string) (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("DEVICE")); explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("specified DEVICE=%s does not exist: %w", explicit, err)
		}
		fmt.Printf("[OK] Using explicitly specified DEVICE=%s\n", explicit)
		return explicit, nil
	}

	devices, err := DiscoverBlockDevices()
	if err != nil || len(devices) == 0 {
		return DetectDevice(platform)
	}

	targetModel := os.Getenv("TARGET_MODEL")
	targetSerial := os.Getenv("TARGET_SERIAL")
	if targetModel != "" || targetSerial != "" {
		matched, err := MatchBlockDeviceByModelOrSerial(devices, targetModel, targetSerial)
		if err != nil {
			return "", err
		}
		fmt.Printf("[OK] Pinned target disk: %s (%s, %s)\n", matched.Path, matched.Model, matched.Size)
		fmt.Println("     All other drives are protected and will NOT be touched.")
		return matched.Path, nil
	}

	// Filter non-removable disks first
	var fixedDisks []BlockDeviceInfo
	for _, d := range devices {
		if !d.Removable && d.Transport != "usb" {
			fixedDisks = append(fixedDisks, d)
		}
	}
	if len(fixedDisks) == 0 {
		fixedDisks = devices
	}

	if len(fixedDisks) == 1 {
		d := fixedDisks[0]
		fmt.Printf("[OK] Auto-detected single target disk: %s (%s, %s)\n", d.Path, d.Model, d.Size)
		return d.Path, nil
	}

	// Multi-disk safety gate: never guess when multiple disks exist!
	fmt.Println()
	fmt.Println("=== MULTI-DISK SAFETY GATE ===")
	fmt.Printf("Detected %d disks on this machine. Refusing to guess which disk to wipe!\n", len(fixedDisks))
	fmt.Println("Tip: You can also pre-pin by setting TARGET_MODEL=\"...\", TARGET_SERIAL=\"...\", or DEVICE=\"/dev/...\"")
	fmt.Println()
	for idx, d := range fixedDisks {
		model := d.Model
		if model == "" {
			model = "generic-disk"
		}
		tran := d.Transport
		if tran == "" {
			tran = "local"
		}
		fmt.Printf("  [%d] %-14s  %-8s  %-30s  (bus: %s)\n", idx+1, d.Path, d.Size, model, tran)
	}
	fmt.Println()
	fmt.Printf("Select target disk number [1-%d] (or 'q' to abort): ", len(fixedDisks))

	tty, err := os.Open("/dev/tty")
	if err != nil {
		return "", fmt.Errorf("multiple disks detected (%d) and /dev/tty is unavailable; set DEVICE or TARGET_MODEL explicitly: %w", len(fixedDisks), err)
	}
	defer tty.Close()

	reader := bufio.NewReader(tty)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read disk selection from /dev/tty: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" || strings.EqualFold(line, "q") {
		return "", fmt.Errorf("disk selection aborted by user")
	}
	choice, err := strconv.Atoi(line)
	if err != nil || choice < 1 || choice > len(fixedDisks) {
		return "", fmt.Errorf("invalid disk selection %q (expected 1..%d)", line, len(fixedDisks))
	}

	selected := fixedDisks[choice-1]
	fmt.Printf("[OK] Selected target disk: %s (%s, %s)\n", selected.Path, selected.Model, selected.Size)
	return selected.Path, nil
}

// DetectHardwareProfile inspects CPU, architecture, RAM, storage bus, and firmware mode
// to generate an optimal hardware configuration and build-speed profile.
func DetectHardwareProfile() *HardwareProfile {
	arch := detectMachineArch()
	vendor, model := detectCPUInfo()
	cores := runtime.NumCPU()
	if cores < 1 {
		cores = 1
	}
	ramMiB := detectTotalRAMMiB()
	bootMode := os.Getenv("BOOT_MODE")
	if bootMode == "" {
		bootMode = DetectBootMode()
	}

	hasNVMe := false
	if entries, err := filepath.Glob("/dev/nvme*"); err == nil && len(entries) > 0 {
		hasNVMe = true
	}

	isVM := detectVirtualMachine()
	useNonguix := IsNonguixEnabled()

	// Recommended swap size based on RAM
	swapSize := os.Getenv("SWAP_SIZE")
	if swapSize == "" {
		switch {
		case ramMiB <= 0:
			swapSize = "4G"
		case ramMiB < 4096:
			swapSize = "4G"
		case ramMiB <= 16384:
			swapSize = "8G"
		default:
			swapSize = "8G"
		}
	}

	// Compute safe parallel build job count (--max-jobs) based on RAM (approx 4 GiB per heavy derivation)
	maxJobs := 2
	if ramMiB >= 32768 {
		maxJobs = 8
	} else if ramMiB >= 16384 {
		maxJobs = 4
	} else if ramMiB < 4096 && ramMiB > 0 {
		maxJobs = 1
	}
	if envJobs := os.Getenv("GUIX_MAX_JOBS"); envJobs != "" {
		if parsed, err := strconv.Atoi(envJobs); err == nil && parsed > 0 {
			maxJobs = parsed
		}
	}

	mirrors := "https://bordeaux.guix.gnu.org https://ci.guix.gnu.org"
	if useNonguix {
		mirrors = "https://substitutes.nonguix.org " + mirrors
	}
	if customMirrors := strings.TrimSpace(os.Getenv("GUIX_SUBSTITUTE_URLS")); customMirrors != "" {
		mirrors = customMirrors
	}

	initrdMods := []string{}
	if isVM {
		initrdMods = append(initrdMods, "virtio_pci", "virtio_blk", "virtio_scsi", "virtio_net")
	}

	kernelArgs := []string{"quiet"}
	if envArgs := strings.TrimSpace(os.Getenv("GUIX_KERNEL_ARGS")); envArgs != "" {
		kernelArgs = strings.Fields(envArgs)
	}

	return &HardwareProfile{
		Arch:              arch,
		CPUVendor:         vendor,
		CPUModel:          model,
		CPUCores:          cores,
		TotalRAMMiB:       ramMiB,
		RecommendedSwap:   swapSize,
		BootMode:          bootMode,
		HasNVMe:           hasNVMe,
		IsVM:              isVM,
		UseNonguix:        useNonguix,
		InitrdModules:     initrdMods,
		KernelArguments:   kernelArgs,
		BuildMaxJobs:      maxJobs,
		SubstituteMirrors: mirrors,
	}
}

// BuildAccelerationArgs returns the optimal guix build / system init flags for the detected hardware.
func (hp *HardwareProfile) BuildAccelerationArgs() []string {
	coresFlag := "--cores=0"
	if envCores := strings.TrimSpace(os.Getenv("GUIX_BUILD_CORES")); envCores != "" {
		coresFlag = "--cores=" + envCores
	}
	return []string{
		coresFlag,
		fmt.Sprintf("--max-jobs=%d", hp.BuildMaxJobs),
		"--substitute-urls=" + hp.SubstituteMirrors,
	}
}

func detectMachineArch() string {
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		m := strings.TrimSpace(string(out))
		if m != "" {
			return m
		}
	}
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return "x86_64"
}

func detectCPUInfo() (vendor string, model string) {
	vendor = "unknown"
	model = "Generic Processor"
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return vendor, model
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		switch k {
		case "vendor_id":
			if v != "" {
				vendor = v
			}
		case "model name", "Processor", "Model":
			if v != "" && model == "Generic Processor" {
				model = v
			}
		case "CPU implementer":
			if vendor == "unknown" {
				vendor = "ARM"
			}
		}
	}
	return vendor, model
}

func detectTotalRAMMiB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 8192
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kib, err := strconv.Atoi(fields[1]); err == nil && kib > 0 {
					return kib / 1024
				}
			}
		}
	}
	return 8192
}

func detectVirtualMachine() bool {
	for _, dmiPath := range []string{
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/board_vendor",
	} {
		if data, err := os.ReadFile(dmiPath); err == nil {
			s := strings.ToLower(string(data))
			for _, sig := range []string{"qemu", "kvm", "virtualbox", "vmware", "bochs", "bhyve", "xen", "cloudzy", "oracle", "digitalocean", "hetzner", "linode"} {
				if strings.Contains(s, sig) {
					return true
				}
			}
		}
	}
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), "hypervisor") {
			return true
		}
	}
	return false
}
