package lib

import (
	"os"
	"strings"
	"testing"
)

func TestNeedsPartitionSeparatorP(t *testing.T) {
	cases := []struct {
		device string
		wantP  bool
		part1  string
	}{
		{"/dev/nvme0n1", true, "/dev/nvme0n1p1"},
		{"/dev/nvme1n1", true, "/dev/nvme1n1p1"},
		{"/dev/mmcblk0", true, "/dev/mmcblk0p1"},
		{"/dev/loop0", true, "/dev/loop0p1"},
		{"/dev/nbd0", true, "/dev/nbd0p1"},
		{"/dev/md0", true, "/dev/md0p1"},
		{"/dev/sda", false, "/dev/sda1"},
		{"/dev/vda", false, "/dev/vda1"},
		{"/dev/xvda", false, "/dev/xvda1"},
	}

	for _, tc := range cases {
		gotP := NeedsPartitionSeparatorP(tc.device)
		if gotP != tc.wantP {
			t.Errorf("NeedsPartitionSeparatorP(%q) = %v, want %v", tc.device, gotP, tc.wantP)
		}
		gotPart := MakePartitionPath(tc.device, "1")
		if gotPart != tc.part1 {
			t.Errorf("MakePartitionPath(%q, \"1\") = %q, want %q", tc.device, gotPart, tc.part1)
		}
	}
}

func TestParseLsblkPairsAndMatchModel(t *testing.T) {
	sample := `NAME="loop0" SIZE="1.1G" TYPE="loop" TRAN="" RM="0" MOUNTPOINT="/iso" MODEL="" SERIAL=""
NAME="nvme0n1" SIZE="931.5G" TYPE="disk" TRAN="nvme" RM="0" MOUNTPOINT="" MODEL="WD_BLACK SN850X 1000GB" SERIAL="SN1001"
NAME="nvme1n1" SIZE="1.8T" TYPE="disk" TRAN="nvme" RM="0" MOUNTPOINT="" MODEL="Samsung SSD 990 PRO 2TB" SERIAL="S7H990PRO"
NAME="sda" SIZE="931.5G" TYPE="disk" TRAN="sata" RM="0" MOUNTPOINT="" MODEL="Crucial CT1000MX500" SERIAL="CT1000"`

	devices := ParseLsblkPairs(sample)
	if len(devices) != 3 {
		t.Fatalf("expected 3 physical disks (loop0 filtered), got %d", len(devices))
	}

	matched, err := MatchBlockDeviceByModelOrSerial(devices, "Samsung SSD 990 PRO 2TB", "")
	if err != nil {
		t.Fatalf("unexpected error matching Samsung 990 PRO: %v", err)
	}
	if matched.Path != "/dev/nvme1n1" {
		t.Errorf("got path %q, want /dev/nvme1n1", matched.Path)
	}

	// Verify ambiguous match is rejected (protects secondary drives)
	_, err = MatchBlockDeviceByModelOrSerial(devices, "1000", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous disk match") {
		t.Errorf("expected ambiguous disk match error, got: %v", err)
	}

	// Verify serial match disambiguates
	matchedBySerial, err := MatchBlockDeviceByModelOrSerial(devices, "1000", "SN1001")
	if err != nil {
		t.Fatalf("unexpected error matching by serial: %v", err)
	}
	if matchedBySerial.Path != "/dev/nvme0n1" {
		t.Errorf("got path %q, want /dev/nvme0n1", matchedBySerial.Path)
	}
}

func TestDetectHardwareProfileDefaultsToLibre(t *testing.T) {
	os.Unsetenv("USE_NONGUIX")
	hp := DetectHardwareProfile()
	if hp.UseNonguix {
		t.Errorf("expected UseNonguix=false by default (pure free software mode)")
	}
	if strings.Contains(hp.SubstituteMirrors, "nonguix") {
		t.Errorf("expected pure GNU Guix substitute mirrors by default, got %q", hp.SubstituteMirrors)
	}

	os.Setenv("USE_NONGUIX", "yes")
	defer os.Unsetenv("USE_NONGUIX")
	hpNonfree := DetectHardwareProfile()
	if !hpNonfree.UseNonguix {
		t.Errorf("expected UseNonguix=true when USE_NONGUIX=yes")
	}
	if !strings.Contains(hpNonfree.SubstituteMirrors, "substitutes.nonguix.org") {
		t.Errorf("expected nonguix substitute mirror when USE_NONGUIX=yes, got %q", hpNonfree.SubstituteMirrors)
	}
}
