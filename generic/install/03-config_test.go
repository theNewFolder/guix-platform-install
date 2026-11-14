package install

import (
	"strings"
	"testing"

	"github.com/durantschoon/guix-platform-install/lib"
)

func TestGenerateConfigPureLibreDefault(t *testing.T) {
	step := &Step03Config{}
	state := &State{
		Device:         "/dev/nvme0n1",
		EFI:            "/dev/nvme0n1p1",
		Root:           "/dev/nvme0n1p2",
		UserName:       "om",
		FullName:       "Omran Altunaiji",
		Timezone:       "Asia/Dubai",
		HostName:       "guix-workstation",
		BootMode:       "uefi",
		KeyboardLayout: "us:ctrl:swapcaps",
		UseNonguix:     false,
		Hardware: &lib.HardwareProfile{
			Arch:         "x86_64",
			CPUVendor:    "GenuineIntel",
			CPUModel:     "Intel Core i9-14900K",
			CPUCores:     32,
			TotalRAMMiB:  65536,
			BuildMaxJobs: 8,
		},
	}

	cfg := step.GenerateConfig(state)

	if !strings.Contains(cfg, "(kernel linux-libre)") {
		t.Errorf("expected pure libre kernel (kernel linux-libre), got:\n%s", cfg)
	}
	if strings.Contains(cfg, "nongnu") {
		t.Errorf("pure libre config must not import nongnu modules:\n%s", cfg)
	}
	if !strings.Contains(cfg, "grub-efi-bootloader") {
		t.Errorf("expected grub-efi-bootloader for UEFI mode")
	}
	if !strings.Contains(cfg, "--cores=0") || !strings.Contains(cfg, "--max-jobs=8") {
		t.Errorf("expected build acceleration flags (--cores=0 --max-jobs=8) in guix-configuration")
	}
}

func TestGenerateConfigBIOSAndOptInNonguix(t *testing.T) {
	step := &Step03Config{}
	state := &State{
		Device:     "/dev/sda",
		EFI:        "/dev/sda2",
		Root:       "/dev/sda3",
		UserName:   "om",
		FullName:   "Omran",
		Timezone:   "Asia/Dubai",
		HostName:   "guix-bios",
		BootMode:   "bios",
		UseNonguix: true,
		Hardware: &lib.HardwareProfile{
			Arch:         "x86_64",
			CPUVendor:    "AuthenticAMD",
			CPUModel:     "AMD Ryzen 9",
			CPUCores:     16,
			TotalRAMMiB:  32768,
			BuildMaxJobs: 8,
		},
	}

	cfg := step.GenerateConfig(state)

	if !strings.Contains(cfg, "(bootloader grub-bootloader)") {
		t.Errorf("expected grub-bootloader for BIOS mode")
	}
	if !strings.Contains(cfg, `'("/dev/sda")`) {
		t.Errorf("expected BIOS target '/dev/sda'")
	}
	if !strings.Contains(cfg, "(kernel linux)") || !strings.Contains(cfg, "(initrd microcode-initrd)") {
		t.Errorf("expected opt-in linux kernel and microcode-initrd when UseNonguix=true")
	}
}
