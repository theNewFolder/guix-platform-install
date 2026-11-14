package install

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/durantschoon/guix-platform-install/lib"
)

// Step04SystemInit performs the final system initialization for the universal generic platform
type Step04SystemInit struct{}

func (s *Step04SystemInit) RunWarnings(state *State) error {
	lib.PrintStepHeader(4, "System Initialization (Final Step)")
	fmt.Println()
	fmt.Println("This step will:")
	fmt.Printf("  1. Create and activate /mnt/swapfile (%s)\n", lib.GetEnvOrDefault(state.SwapSize, "8G"))
	fmt.Println("  2. Start cow-store to redirect /gnu/store writes to target disk")
	if state.UseNonguix {
		fmt.Println("  3. Build and initialize Guix system with opt-in hardware firmware overlay")
	} else {
		fmt.Println("  3. Build and initialize 100% pure libre Guix system (linux-libre)")
	}
	fmt.Println("  4. Verify kernel, initrd, and GRUB bootloader installation")
	fmt.Println("  5. Set root and user passwords and install ~/guix-customize tools")
	fmt.Println()
	return nil
}

func (s *Step04SystemInit) RunClean(state *State) error {
	lib.EnableCommandLogging("/tmp/guix-install.log")

	if !lib.IsMounted("/mnt") {
		if err := lib.MountByLabel("GUIX_ROOT", "/mnt"); err != nil {
			return fmt.Errorf("failed to mount /mnt: %w", err)
		}
	}

	tmpDir := "/mnt/var/tmp"
	if err := os.MkdirAll(tmpDir, 0777); err != nil {
		return err
	}
	os.Setenv("TMPDIR", tmpDir)

	cacheDir := "/mnt/var/cache"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	os.Setenv("XDG_CACHE_HOME", cacheDir)

	exec.Command("rm", "-rf", "/var/guix/substitute-cache/").Run()

	if err := lib.CreateSwapFile(state.SwapSize); err != nil {
		return err
	}

	configPath := "/mnt/etc/config.scm"
	bootMode := lib.DetectBootModeFromConfig(configPath)
	if bootMode == "uefi" {
		if err := lib.VerifyESP(); err != nil {
			return err
		}
	} else {
		fmt.Println("[OK] BIOS boot mode detected - skipping ESP vfat check")
	}

	if err := lib.StartCowStore(); err != nil {
		return err
	}

	recoveryPath := "/root/recovery-complete-install.sh"
	if err := lib.WriteRecoveryScript(recoveryPath, state.GuixPlatform); err == nil {
		fmt.Printf("Recovery script written: %s\n", recoveryPath)
	}

	if state.UseNonguix {
		if err := lib.RunGuixSystemInit(state.GuixPlatform); err != nil {
			return err
		}
		if err := lib.VerifyAndRecoverKernelFiles(3, state.GuixPlatform, "non-libre"); err != nil {
			return err
		}
	} else {
		if err := lib.RunGuixSystemInitFreeSoftware(state.GuixPlatform); err != nil {
			return err
		}
	}

	if err := lib.InstallVerificationScript(); err != nil {
		fmt.Printf("[WARN] Failed to install verification script: %v\n", err)
	}

	if err := lib.SetUserPassword(state.UserName); err != nil {
		fmt.Printf("[WARN] Failed to set user password: %v\n", err)
	}

	if err := lib.DownloadCustomizationTools(state.GuixPlatform, state.UserName); err != nil {
		fmt.Printf("[WARN] Failed to download customization tools: %v\n", err)
	}

	if err := lib.WriteInstallReceipt(state.GuixPlatform, state.UserName); err != nil {
		fmt.Printf("[WARN] Failed to write installation receipt: %v\n", err)
	}

	if err := lib.RunComprehensiveVerification(recoveryPath); err != nil {
		return fmt.Errorf("installation incomplete - run %s before rebooting: %w", recoveryPath, err)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  INSTALLATION COMPLETE AND VERIFIED")
	fmt.Println("========================================")
	fmt.Println()
	return nil
}
