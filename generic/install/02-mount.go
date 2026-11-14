package install

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/durantschoon/guix-platform-install/lib"
)

// Step02Mount mounts partitions and initializes the target disk store
type Step02Mount struct{}

func (s *Step02Mount) RunWarnings(state *State) error {
	if state.Device == "" {
		device, err := lib.SelectTargetDevice("generic")
		if err != nil {
			return err
		}
		state.Device = device
	}

	if state.EFI == "" || state.Root == "" {
		part1 := lib.MakePartitionPath(state.Device, "1")
		if lib.IsPartitionType(part1, "bios_grub") {
			state.EFI = lib.MakePartitionPath(state.Device, "2")
			state.Root = lib.MakePartitionPath(state.Device, "3")
			if state.BootMode == "" {
				state.BootMode = "bios"
			}
		} else {
			efi, root, err := lib.DetectPartitions(state.Device)
			if err != nil {
				return err
			}
			state.EFI = efi
			state.Root = root
		}
	}

	if state.Root == "" || state.EFI == "" {
		return fmt.Errorf("required variables not set (ROOT, EFI); run Step01 first")
	}

	lib.PrintStepHeader(2, "Mount and Store Setup")
	fmt.Println()
	fmt.Println("This step will:")
	fmt.Println("  1. Mount GUIX_ROOT partition to /mnt")
	fmt.Println("  2. Copy Guix store from ISO to /mnt/gnu/store (if not already populated)")
	fmt.Println("  3. Copy /var/guix database to /mnt/var/guix")
	fmt.Println("  4. Clean up ISO filesystem artifacts (symlinks for /var/run, /var/lock, /etc/mtab)")
	fmt.Println("  5. Mount EFI partition to /mnt/boot/efi")
	fmt.Println()
	return nil
}

func (s *Step02Mount) RunClean(state *State) error {
	if state.Root == "" || state.EFI == "" || state.Device == "" {
		return fmt.Errorf("required variables not set (ROOT, EFI, DEVICE)")
	}

	storeAlreadyPopulated := lib.IsMounted("/mnt") && lib.IsStorePopulated()
	if storeAlreadyPopulated {
		fmt.Println("[OK] /mnt is already mounted and /mnt/gnu/store is populated (idempotent)")
	}

	if err := lib.VerifyLabelsExist("GUIX_ROOT", "EFI"); err != nil {
		return err
	}

	if !lib.IsMounted("/mnt") {
		fmt.Println("Mounting GUIX_ROOT to /mnt...")
		if err := lib.MountByLabel("GUIX_ROOT", "/mnt"); err != nil {
			return err
		}
	}

	avail := lib.GetMountFreeSpaceGiB("/mnt")
	if avail > 0 && avail < 30 {
		fmt.Printf("[WARN] Only %.1f GiB free on /mnt. Recommended: 40+ GiB.\n", avail)
	}

	if !storeAlreadyPopulated {
		for _, dir := range []string{"/mnt/gnu/store", "/mnt/var/guix"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dir, err)
			}
		}

		fmt.Println("Stopping guix-daemon temporarily for store sync...")
		if lib.CommandExists("herd") {
			lib.RunCommand("herd", "stop", "guix-daemon")
		} else {
			exec.Command("pkill", "-TERM", "-x", "guix-daemon").Run()
		}

		start := time.Now()
		fmt.Println("Syncing /gnu/store to /mnt/gnu/store...")
		if lib.CommandExists("rsync") {
			if err := lib.RunCommand("rsync", "-aHAX", "--info=progress2,stats", "/gnu/store/.", "/mnt/gnu/store/."); err != nil {
				return fmt.Errorf("rsync failed: %w", err)
			}
		} else {
			if err := lib.RunCommand("cp", "-a", "/gnu/store/.", "/mnt/gnu/store/"); err != nil {
				return fmt.Errorf("cp failed: %w", err)
			}
		}
		fmt.Printf("[OK] Store synced in %.0f seconds\n", time.Since(start).Seconds())

		fmt.Println("Copying /var/guix to /mnt/var/guix...")
		if err := lib.RunCommand("cp", "-a", "/var/guix", "/mnt/var/"); err != nil {
			return fmt.Errorf("failed to copy /var/guix: %w", err)
		}
	}

	if err := lib.CleanupISOArtifacts(); err != nil {
		return fmt.Errorf("ISO artifact cleanup failed: %w", err)
	}

	fmt.Println("Restarting guix-daemon...")
	if lib.CommandExists("herd") {
		if err := lib.RunCommand("herd", "start", "guix-daemon"); err != nil {
			return fmt.Errorf("failed to start guix-daemon: %w", err)
		}
	}

	if err := os.MkdirAll("/mnt/boot/efi", 0755); err != nil {
		return err
	}
	if !lib.IsMounted("/mnt/boot/efi") {
		fmt.Println("Mounting EFI partition to /mnt/boot/efi...")
		if err := lib.MountByLabel("EFI", "/mnt/boot/efi"); err != nil {
			return err
		}
	}

	fmt.Println("[OK] Step 2 (Mount & Store Setup) complete")
	return nil
}
