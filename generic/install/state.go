package install

import (
	"fmt"
	"os"

	"github.com/durantschoon/guix-platform-install/lib"
)

// State holds all installation variables shared between steps for the universal generic installer
type State struct {
	// Disk/partition info
	Device   string
	EFI      string
	Root     string
	SwapSize string

	// User & system configuration
	UserName       string
	FullName       string
	Timezone       string
	HostName       string
	BootMode       string
	KeyboardLayout string // Layout and options, e.g., "us:ctrl:swapcaps"

	// Hardware & channel mode
	UseNonguix   bool
	Hardware     *lib.HardwareProfile
	GuixPlatform string
}

// NewState creates a new State from environment variables and hardware auto-detection
func NewState() *State {
	platform := getEnv("GUIX_PLATFORM", "generic")
	if platform != "generic" && platform != "" {
		fmt.Printf("[WARN] GUIX_PLATFORM=%s but running generic installer. Using 'generic' instead.\n", platform)
		platform = "generic"
	}

	hw := lib.DetectHardwareProfile()
	swapSize := os.Getenv("SWAP_SIZE")
	if swapSize == "" {
		swapSize = hw.RecommendedSwap
	}

	bootMode := os.Getenv("BOOT_MODE")
	if bootMode == "" {
		bootMode = hw.BootMode
	}

	return &State{
		Device:         os.Getenv("DEVICE"),
		EFI:            os.Getenv("EFI"),
		Root:           os.Getenv("ROOT"),
		UserName:       os.Getenv("USER_NAME"),
		FullName:       os.Getenv("FULL_NAME"),
		Timezone:       os.Getenv("TIMEZONE"),
		HostName:       os.Getenv("HOST_NAME"),
		BootMode:       bootMode,
		SwapSize:       swapSize,
		KeyboardLayout: os.Getenv("KEYBOARD_LAYOUT"),
		UseNonguix:     hw.UseNonguix,
		Hardware:       hw,
		GuixPlatform:   platform,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
