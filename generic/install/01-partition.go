package install

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/durantschoon/guix-platform-install/lib"
)

// Step01Partition handles universal hardware-safe partitioning (NVMe, SATA, VirtIO, eMMC; UEFI & BIOS)
type Step01Partition struct{}

func (s *Step01Partition) RunWarnings(state *State) error {
	lib.PrintStepHeader(1, "Universal Hardware Partition Setup")
	fmt.Println()
	fmt.Println("Detected Hardware Profile:")
	if state.Hardware != nil {
		fmt.Printf("  Architecture : %s\n", state.Hardware.Arch)
		fmt.Printf("  CPU          : %s (%s, %d logical cores)\n", state.Hardware.CPUModel, state.Hardware.CPUVendor, state.Hardware.CPUCores)
		fmt.Printf("  Memory       : %d MiB RAM (recommended swap: %s, max-jobs: %d)\n", state.Hardware.TotalRAMMiB, state.SwapSize, state.Hardware.BuildMaxJobs)
		fmt.Printf("  Boot Mode    : %s\n", state.BootMode)
		if state.UseNonguix {
			fmt.Println("  Kernel Mode  : Opt-in Non-Free Hardware Overlay (USE_NONGUIX=yes)")
		} else {
			fmt.Println("  Kernel Mode  : 100% Pure Libre GNU Guix (linux-libre, default)")
		}
	}
	fmt.Println()
	fmt.Println("This step will:")
	fmt.Println("  1. Select target disk safely (honoring DEVICE, TARGET_MODEL, TARGET_SERIAL, or multi-disk prompt)")
	fmt.Println("  2. Display current partition table")
	fmt.Println("  3. Create GPT partition table")
	fmt.Println("  4. Create boot + root partitions matching detected firmware:")
	fmt.Println("     - UEFI mode: EFI System Partition (512MiB FAT32) + GUIX_ROOT (remaining ext4)")
	fmt.Println("     - BIOS mode: BIOS Boot (1MiB bios_grub) + EFI (512MiB FAT32) + GUIX_ROOT (remaining ext4)")
	fmt.Println("  5. Format partitions with filesystem labels (EFI, GUIX_ROOT)")
	fmt.Println()
	fmt.Println("WARNING: This will DESTROY ALL DATA on the selected target disk!")
	fmt.Println("         Secondary drives are never touched.")
	fmt.Println()
	return nil
}

func (s *Step01Partition) RunClean(state *State) error {
	if state.Device == "" {
		device, err := lib.SelectTargetDevice("generic")
		if err != nil {
			return err
		}
		state.Device = device
	}

	if err := lib.CheckDeviceSpace(state.Device, 20.0); err != nil {
		return err
	}

	if err := lib.CheckDiskHasActiveHolders(state.Device); err != nil {
		return err
	}

	if state.BootMode == "" {
		state.BootMode = lib.DetectBootMode()
	}

	// Idempotency: if partitions already exist and are formatted with expected labels, reuse them
	if lib.HasExistingPartitions(state.Device) {
		part1 := lib.MakePartitionPath(state.Device, "1")
		if lib.IsPartitionType(part1, "bios_grub") {
			state.EFI = lib.MakePartitionPath(state.Device, "2")
			state.Root = lib.MakePartitionPath(state.Device, "3")
			state.BootMode = "bios"
		} else {
			state.EFI = lib.MakePartitionPath(state.Device, "1")
			state.Root = lib.MakePartitionPath(state.Device, "2")
		}
		if lib.IsPartitionFormatted(state.EFI, "vfat") && lib.IsPartitionFormatted(state.Root, "ext4") {
			if err := lib.VerifyLabelsExist("GUIX_ROOT", "EFI"); err == nil {
				fmt.Println("[OK] Existing formatted GUIX_ROOT and EFI partitions detected on", state.Device)
				fmt.Println("     Skipping repartitioning and formatting (idempotent rerun)")
				return nil
			}
		}
	}

	fmt.Printf("Current partition table for %s:\n", state.Device)
	lib.RunCommand("parted", "-s", state.Device, "print")
	fmt.Println()

	fmt.Printf("WARNING: About to partition %s (%s mode). This will DESTROY ALL DATA on %s!\n", state.Device, state.BootMode, state.Device)
	fmt.Print("Type 'yes' to continue: ")

	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close()

	reader := bufio.NewReader(tty)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(answer) != "yes" {
		return fmt.Errorf("partitioning aborted by user")
	}

	// Unmount if currently mounted
	_ = lib.UnmountDevice(state.Device)

	if state.BootMode == "bios" {
		fmt.Println("Creating GPT partition table with BIOS boot + EFI + GUIX_ROOT layout...")
		if err := lib.RunCommand("parted", "-s", state.Device,
			"mklabel", "gpt",
			"mkpart", "BIOSBOOT", "1MiB", "2MiB",
			"set", "1", "bios_grub", "on",
			"mkpart", "EFI", "fat32", "2MiB", "514MiB",
			"set", "2", "esp", "on",
			"mkpart", "GUIX_ROOT", "ext4", "514MiB", "100%"); err != nil {
			return fmt.Errorf("parted BIOS layout failed: %w", err)
		}
		state.EFI = lib.MakePartitionPath(state.Device, "2")
		state.Root = lib.MakePartitionPath(state.Device, "3")
	} else {
		fmt.Println("Creating GPT partition table with UEFI (ESP + GUIX_ROOT) layout...")
		if err := lib.RunCommand("parted", "-s", state.Device,
			"mklabel", "gpt",
			"mkpart", "EFI", "fat32", "1MiB", "513MiB",
			"set", "1", "esp", "on",
			"mkpart", "GUIX_ROOT", "ext4", "513MiB", "100%"); err != nil {
			return fmt.Errorf("parted UEFI layout failed: %w", err)
		}
		state.EFI = lib.MakePartitionPath(state.Device, "1")
		state.Root = lib.MakePartitionPath(state.Device, "2")
	}

	fmt.Println("Waiting for kernel to recognize partitions...")
	lib.RunCommand("partprobe", state.Device)
	lib.RunCommand("sleep", "2")

	fmt.Printf("Formatting EFI partition: %s\n", state.EFI)
	if err := lib.RunCommand("mkfs.vfat", "-F", "32", "-n", "EFI", state.EFI); err != nil {
		return err
	}

	fmt.Printf("Formatting root partition: %s\n", state.Root)
	if err := lib.RunCommand("mkfs.ext4", "-F", "-L", "GUIX_ROOT", state.Root); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Verifying partition labels...")
	lib.RunCommand("fatlabel", state.EFI)
	lib.RunCommand("e2label", state.Root)

	fmt.Println()
	fmt.Println("[OK] Partition setup complete:")
	fmt.Printf("  DEVICE=%s\n", state.Device)
	fmt.Printf("  EFI=%s\n", state.EFI)
	fmt.Printf("  ROOT=%s\n", state.Root)
	fmt.Printf("  BOOT_MODE=%s\n", state.BootMode)
	return nil
}
