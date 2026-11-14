package install

import (
	"fmt"
	"os"
	"strings"

	"github.com/durantschoon/guix-platform-install/lib"
)

// Step03Config generates a hardware-aware Guix system configuration (/mnt/etc/config.scm)
// Defaults to 100% pure GNU Guix (linux-libre) with build-acceleration daemon settings,
// while supporting explicit opt-in USE_NONGUIX=yes for non-free hardware firmware.
type Step03Config struct{}

func (s *Step03Config) RunWarnings(state *State) error {
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
			state.BootMode = "bios"
		} else {
			efi, root, err := lib.DetectPartitions(state.Device)
			if err != nil {
				return err
			}
			state.EFI = efi
			state.Root = root
		}
	}

	lib.PrintStepHeader(3, "Hardware-Aware Guix Configuration Generation")
	fmt.Println()
	fmt.Println("This step will:")
	fmt.Println("  1. Detect CPU vendor/arch, UEFI vs BIOS boot mode, and RAM/core counts")
	if state.UseNonguix {
		fmt.Println("  2. Set up opt-in nonguix channel (USE_NONGUIX=yes) for hardware firmware")
	} else {
		fmt.Println("  2. Use 100% pure Free Software GNU Guix (linux-libre kernel)")
	}
	fmt.Println("  3. Configure guix-daemon build acceleration (multi-mirror substitutes, --cores=0, --max-jobs)")
	fmt.Println("  4. Write /mnt/etc/config.scm using filesystem labels (GUIX_ROOT, EFI)")
	fmt.Println()
	return nil
}

func (s *Step03Config) RunClean(state *State) error {
	if state.Root == "" || state.EFI == "" || state.Device == "" {
		return fmt.Errorf("required variables not set (ROOT, EFI, DEVICE)")
	}

	configPath := "/mnt/etc/config.scm"
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("[OK] Configuration file %s already exists (idempotent rerun)\n", configPath)
		return nil
	}

	if state.Hardware == nil {
		state.Hardware = lib.DetectHardwareProfile()
	}
	if state.BootMode == "" {
		state.BootMode = state.Hardware.BootMode
	}

	if state.UseNonguix {
		if err := lib.SetupNonguixChannel("generic"); err != nil {
			return err
		}
	}

	if state.KeyboardLayout == "" {
		layout, err := lib.PromptKeyboardLayout()
		if err != nil {
			fmt.Printf("[WARN] Keyboard layout prompt skipped (%v), defaulting to 'us'\n", err)
			layout = "us"
		}
		state.KeyboardLayout = layout
	}

	if state.UserName == "" {
		state.UserName = "guix"
	}
	if state.FullName == "" {
		state.FullName = "Guix User"
	}
	if state.Timezone == "" {
		state.Timezone = "UTC"
	}
	if state.HostName == "" {
		state.HostName = "guix-host"
	}

	config := s.GenerateConfig(state)

	if err := os.MkdirAll("/mnt/etc", 0755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	if err := lib.RecordChannelCommits(); err != nil {
		fmt.Printf("[WARN] Could not record channel commits: %v\n", err)
	}

	fmt.Println()
	fmt.Println("=== Generated /mnt/etc/config.scm ===")
	fmt.Println(config)
	fmt.Println("[OK] Configuration written to /mnt/etc/config.scm")
	return nil
}

// GenerateConfig produces a complete, valid Guile Scheme operating-system declaration
// tailored to the detected hardware profile and build-speed settings.
func (s *Step03Config) GenerateConfig(state *State) string {
	hw := state.Hardware
	if hw == nil {
		hw = lib.DetectHardwareProfile()
	}

	bootMode := state.BootMode
	if bootMode == "" {
		bootMode = "uefi"
	}

	bootloader := "grub-efi-bootloader"
	targets := `'("/boot/efi")`
	if bootMode == "bios" {
		bootloader = "grub-bootloader"
		targets = fmt.Sprintf(`'("%s")`, state.Device)
	} else if strings.EqualFold(os.Getenv("EFI_REMOVABLE"), "yes") || strings.EqualFold(os.Getenv("EFI_REMOVABLE"), "1") {
		bootloader = "grub-efi-removable-bootloader"
	}

	kbDeclaration := ""
	kbBootloader := ""
	if state.KeyboardLayout != "" {
		parts := strings.Split(state.KeyboardLayout, ":")
		layout := parts[0]
		if len(parts) > 1 {
			opts := strings.Join(parts[1:], ":")
			kbDeclaration = fmt.Sprintf("\n (keyboard-layout (keyboard-layout %q #:options '(%q)))", layout, opts)
		} else {
			kbDeclaration = fmt.Sprintf("\n (keyboard-layout (keyboard-layout %q))", layout)
		}
		kbBootloader = "\n    (keyboard-layout keyboard-layout)"
	}

	maxJobs := hw.BuildMaxJobs
	if maxJobs < 1 {
		maxJobs = 4
	}

	// Build-acceleration service extension for guix-daemon:
	// - Official Bordeaux + Berlin substitute farms with fast zstd/lzip compression
	// - Parallel derivation builds (--max-jobs) and all CPU cores (--cores=0)
	// - Keep derivations/outputs for instant offline rebuilds
	serviceMod := fmt.Sprintf(`(modify-services %%base-services
            (guix-service-type config =>
              (guix-configuration
               (inherit config)
               (substitute-urls
                (append (list "https://bordeaux.guix.gnu.org"
                              "https://ci.guix.gnu.org")
                        %%default-substitute-urls))
               (extra-options
                (list "--cores=0"
                      "--max-jobs=%d"
                      "--gc-keep-derivations=yes"
                      "--gc-keep-outputs=yes")))))`, maxJobs)

	if !state.UseNonguix {
		// Pure Libre GNU Guix configuration (Default)
		return fmt.Sprintf(`;; GNU Guix Operating System Configuration (Universal Hardware - Pure Free Software)
;; Generated by guix-platform-install (GUIX_PLATFORM=generic)
;; Architecture: %s | CPU: %s | Boot Mode: %s

(use-modules (gnu)
             (gnu packages certs)
             (gnu packages linux)
             (gnu packages vim)
             (gnu services base)
             (gnu system nss))
(use-service-modules networking ssh)
(use-package-modules admin curl emacs certs ssh tls vim)

(operating-system
 (host-name %q)
 (timezone %q)
 (locale "en_US.utf8")%s

 ;; Pure Free Software Kernel (FSF/GNU Linux-libre)
 (kernel linux-libre)
 (kernel-arguments (cons* "quiet" %%default-kernel-arguments))

 (bootloader
  (bootloader-configuration
   (bootloader %s)
   (targets %s)%s
   (timeout 5)))

 (swap-devices
  (list (swap-space
         (target "/swapfile"))))

 (file-systems
  (cons* (file-system
          (mount-point "/")
          (device (file-system-label "GUIX_ROOT"))
          (type "ext4"))
         (file-system
          (mount-point "/boot/efi")
          (device (file-system-label "EFI"))
          (type "vfat"))
         %%base-file-systems))

 (users
  (cons (user-account
         (name %q)
         (comment %q)
         (group "users")
         (supplementary-groups '("wheel" "netdev" "audio" "video")))
        %%base-user-accounts))

 (packages
  (append (list nss-certs curl vim)
          %%base-packages))

 (services %s)

 (name-service-switch %%mdns-host-lookup-nss))
`,
			hw.Arch, hw.CPUModel, bootMode,
			state.HostName, state.Timezone, kbDeclaration,
			bootloader, targets, kbBootloader,
			state.UserName, state.FullName,
			serviceMod)
	}

	// Opt-in Non-Free Hardware Overlay (USE_NONGUIX=yes)
	microcodeModule := ""
	microcodeInitrd := ""
	if hw.CPUVendor == "GenuineIntel" {
		microcodeModule = "\n             (nongnu packages linux)"
		microcodeInitrd = "\n (initrd microcode-initrd)"
	} else if hw.CPUVendor == "AuthenticAMD" {
		microcodeModule = "\n             (nongnu packages linux)"
		microcodeInitrd = "\n (initrd microcode-initrd)"
	} else {
		microcodeModule = "\n             (nongnu packages linux)"
	}

	return fmt.Sprintf(`;; GNU Guix Operating System Configuration (Universal Hardware + Opt-in Firmware Overlay)
;; Generated by guix-platform-install (GUIX_PLATFORM=generic, USE_NONGUIX=yes)
;; Architecture: %s | CPU: %s (%s) | Boot Mode: %s

(use-modules (gnu)
             (gnu packages certs)
             (gnu packages linux)
             (gnu packages vim)
             (gnu services base)
             (gnu system nss)%s
             (nongnu system linux-initrd))
(use-service-modules networking ssh)
(use-package-modules admin curl emacs certs ssh tls vim)

(operating-system
 (host-name %q)
 (timezone %q)
 (locale "en_US.utf8")%s

 ;; Hardware enablement kernel and firmware (opt-in USE_NONGUIX=yes)
 (kernel linux)%s
 (firmware (list linux-firmware))
 (kernel-arguments (cons* "quiet" %%default-kernel-arguments))

 (bootloader
  (bootloader-configuration
   (bootloader %s)
   (targets %s)%s
   (timeout 5)))

 (swap-devices
  (list (swap-space
         (target "/swapfile"))))

 (file-systems
  (cons* (file-system
          (mount-point "/")
          (device (file-system-label "GUIX_ROOT"))
          (type "ext4"))
         (file-system
          (mount-point "/boot/efi")
          (device (file-system-label "EFI"))
          (type "vfat"))
         %%base-file-systems))

 (users
  (cons (user-account
         (name %q)
         (comment %q)
         (group "users")
         (supplementary-groups '("wheel" "netdev" "audio" "video")))
        %%base-user-accounts))

 (packages
  (append (list nss-certs curl vim)
          %%base-packages))

 (services %s)

 (name-service-switch %%mdns-host-lookup-nss))
`,
		hw.Arch, hw.CPUModel, hw.CPUVendor, bootMode,
		microcodeModule,
		state.HostName, state.Timezone, kbDeclaration,
		microcodeInitrd,
		bootloader, targets, kbBootloader,
		state.UserName, state.FullName,
		serviceMod)
}
