//go:build !windows

package hostagent

import (
	"io/fs"
	"os"
	"testing"
)

func TestParseSMARTOutputParsesNVMeCapacity(t *testing.T) {
	output := []byte(`{
		"device": {"name": "/dev/nvme0", "type": "nvme", "protocol": "NVMe"},
		"model_name": "KINGSTON SNV3S2000G",
		"serial_number": "50026B7282A0FB69",
		"nvme_total_capacity": 2000398934016,
		"smart_status": {"passed": true},
		"temperature": {"current": 37}
	}`)

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/nvme0", DeviceType: "nvme"})
	if err != nil {
		t.Fatalf("parseSMARTOutput returned error: %v", err)
	}
	if result.SizeBytes != 2000398934016 {
		t.Errorf("SizeBytes = %d, want 2000398934016", result.SizeBytes)
	}
}

func TestParseSMARTOutputParsesUserCapacity(t *testing.T) {
	output := []byte(`{
		"device": {"name": "/dev/sda", "type": "sat", "protocol": "ATA"},
		"model_name": "INTEL SSDSC2BW240A4",
		"serial_number": "CVDA000000000000",
		"user_capacity": {"bytes": 240057409536},
		"smart_status": {"passed": true},
		"temperature": {"current": 30}
	}`)

	result, err := parseSMARTOutput(output, smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if err != nil {
		t.Fatalf("parseSMARTOutput returned error: %v", err)
	}
	if result.SizeBytes != 240057409536 {
		t.Errorf("SizeBytes = %d, want 240057409536", result.SizeBytes)
	}
}

func TestIsNVMeControllerName(t *testing.T) {
	cases := map[string]bool{
		"nvme0":     true,
		"nvme11":    true,
		"nvme0n1":   false, // namespace, not controller
		"nvme0n1p1": false, // partition
		"sda":       false,
		"nvme":      false,
		"":          false,
	}
	for name, want := range cases {
		if got := isNVMeControllerName(name); got != want {
			t.Errorf("isNVMeControllerName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCanonicalBlockDeviceForScanPath(t *testing.T) {
	origReadDir := readDir
	t.Cleanup(func() { readDir = origReadDir })

	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{name: "nvme0n1"},
			fakeDirEntry{name: "nvme0n1p1"}, // partition must be ignored
			fakeDirEntry{name: "nvme1n1"},
			fakeDirEntry{name: "sda"},
		}, nil
	}

	cases := map[string]string{
		"/dev/nvme0":   "nvme0n1", // controller resolves to its namespace
		"/dev/nvme1":   "nvme1n1",
		"/dev/nvme0n1": "nvme0n1", // already a namespace
		"/dev/sda":     "sda",
	}
	for scanPath, want := range cases {
		if got := canonicalBlockDeviceForScanPath(scanPath); got != want {
			t.Errorf("canonicalBlockDeviceForScanPath(%q) = %q, want %q", scanPath, got, want)
		}
	}
}

func TestCanonicalBlockDeviceControllerWithoutNamespaceKeepsName(t *testing.T) {
	origReadDir := readDir
	t.Cleanup(func() { readDir = origReadDir })

	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{name: "nvme0n1p1"}}, nil // only a partition exists
	}

	if got := canonicalBlockDeviceForScanPath("/dev/nvme0"); got != "nvme0" {
		t.Errorf("canonicalBlockDeviceForScanPath(/dev/nvme0) = %q, want fallback nvme0", got)
	}
}

func TestBlockDeviceSizeBytes(t *testing.T) {
	origReadFile := smartctlReadFile
	t.Cleanup(func() { smartctlReadFile = origReadFile })

	smartctlReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/sys/block/nvme0n1/size":
			return []byte("3907029168\n"), nil
		case "/sys/block/sda/size":
			return []byte("468862128\n"), nil
		default:
			return nil, fs.ErrNotExist
		}
	}

	if got := blockDeviceSizeBytes("nvme0n1"); got != 2000398934016 {
		t.Errorf("blockDeviceSizeBytes(nvme0n1) = %d, want 2000398934016", got)
	}
	if got := blockDeviceSizeBytes("sda"); got != 240057409536 {
		t.Errorf("blockDeviceSizeBytes(sda) = %d, want 240057409536", got)
	}
	if got := blockDeviceSizeBytes("missing"); got != 0 {
		t.Errorf("blockDeviceSizeBytes(missing) = %d, want 0", got)
	}
}

func TestRefineLinuxBlockDeviceIdentity(t *testing.T) {
	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
	})

	runtimeGOOS = "linux"
	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{fakeDirEntry{name: "nvme0n1"}, fakeDirEntry{name: "sda"}}, nil
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/sys/block/nvme0n1/size":
			return []byte("3907029168"), nil
		default:
			return nil, fs.ErrNotExist
		}
	}

	// NVMe controller scan target rewrites to the namespace and gains the size.
	smart := &DiskSMART{Device: "nvme0 [nvme]"}
	refineLinuxBlockDeviceIdentity(smart, smartctlTarget{Path: "/dev/nvme0", DeviceType: "nvme"})
	if smart.Device != "nvme0n1" {
		t.Errorf("Device = %q, want nvme0n1", smart.Device)
	}
	if smart.SizeBytes != 2000398934016 {
		t.Errorf("SizeBytes = %d, want 2000398934016", smart.SizeBytes)
	}

	// A smartctl-reported capacity is preserved when /sys/block has no size.
	smart = &DiskSMART{Device: "sda [sat]", SizeBytes: 240057409536}
	refineLinuxBlockDeviceIdentity(smart, smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	if smart.Device != "sda" {
		t.Errorf("Device = %q, want sda", smart.Device)
	}
	if smart.SizeBytes != 240057409536 {
		t.Errorf("SizeBytes = %d, want fallback 240057409536", smart.SizeBytes)
	}

	// Disks behind a multiplexing controller keep their disambiguating label.
	smart = &DiskSMART{Device: "sda [megaraid,7]", SizeBytes: 480103981056}
	refineLinuxBlockDeviceIdentity(smart, smartctlTarget{Path: "/dev/sda", DeviceType: "megaraid,7"})
	if smart.Device != "sda [megaraid,7]" {
		t.Errorf("Device = %q, want unchanged megaraid label", smart.Device)
	}
	if smart.SizeBytes != 480103981056 {
		t.Errorf("SizeBytes = %d, want preserved smartctl capacity", smart.SizeBytes)
	}
}

func TestIsMultiplexedDeviceType(t *testing.T) {
	cases := map[string]bool{
		"megaraid,7": true,
		"cciss,1":    true,
		"areca,1/1":  true,
		"aacraid,0":  true,
		"sat":        false,
		"nvme":       false,
		"":           false,
		"sat,auto":   false,
	}
	for dt, want := range cases {
		if got := isMultiplexedDeviceType(dt); got != want {
			t.Errorf("isMultiplexedDeviceType(%q) = %v, want %v", dt, got, want)
		}
	}
}

func TestRefineLinuxBlockDeviceIdentityNonLinuxNoOp(t *testing.T) {
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = origGOOS })
	runtimeGOOS = "darwin"

	smart := &DiskSMART{Device: "nvme0 [nvme]"}
	refineLinuxBlockDeviceIdentity(smart, smartctlTarget{Path: "/dev/nvme0", DeviceType: "nvme"})
	if smart.Device != "nvme0 [nvme]" {
		t.Errorf("Device = %q, want unchanged on non-linux", smart.Device)
	}
}
