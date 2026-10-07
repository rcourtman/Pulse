package diskinventory

import "testing"

func TestPreferredIDPreservesDirectDeviceFallbacks(t *testing.T) {
	for _, test := range []struct {
		name       string
		device     string
		controller string
		target     string
		want       string
	}{
		{name: "sata", device: "/dev/sda", want: "host:sda"},
		{name: "nvme", device: "nvme0n1", want: "host:nvme0n1"},
		{name: "direct sas hctl", device: "sdb", controller: "0000:03:00.0", target: "6:0:0:0", want: "host:sdb"},
		{name: "controller member", device: "sdc [megaraid,7]", controller: "sdc", target: "megaraid,7", want: "host:sdc@sdc/megaraid,7"},
		{name: "areca member", device: "sdc [areca,1/1]", controller: "arcmsr0", target: "areca,1/1", want: "host:sdc@arcmsr0/areca,1/1"},
		{name: "sssraid member", device: "sg2 [sssraid,0,1]", controller: "sssraid0", target: "sssraid,0,1", want: "host:sg2@sssraid0/sssraid,0,1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := PreferredID("", "", "host", test.device, test.controller, test.target)
			if got != test.want {
				t.Fatalf("PreferredID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPreferredIDKeepsExistingHardwareIdentityPriority(t *testing.T) {
	if got := PreferredID(" SERIAL ", "WWN", "host", "sda", "controller", "megaraid,7"); got != "SERIAL" {
		t.Fatalf("serial identity = %q, want SERIAL", got)
	}
	if got := PreferredID("", " WWN ", "host", "sda", "controller", "megaraid,7"); got != "WWN" {
		t.Fatalf("WWN identity = %q, want WWN", got)
	}
}

func TestPreferredIDRejectsPlaceholderHardwareIdentity(t *testing.T) {
	for _, placeholder := range []string{
		"UNKNOWN",
		"N/A",
		"DEFAULT-SERIAL",
		"0000-0000-0000",
		"FFFF:FFFF",
		"0x0000000000000000",
		"naa.0000000000000000",
		"wwn-0x0000000000000000",
		"0xffffffffffffffff",
		"0x",
	} {
		if IsUsableHardwareID(placeholder) {
			t.Fatalf("placeholder %q was treated as usable hardware identity", placeholder)
		}
		if got := PreferredID(placeholder, "", "host-a", "/dev/sda", "", ""); got != "host-a:sda" {
			t.Fatalf("placeholder %q produced ID %q, want scoped fallback", placeholder, got)
		}
	}
	if !IsUsableHardwareID("ZR5DLAYJ") || !IsUsableHardwareID("naa.5000c500abcdef01") ||
		!IsUsableHardwareID("0x5000c500abcdef01") {
		t.Fatal("real disk serial/WWN was rejected")
	}
}

// QEMU gives a virtual disk without a configured serial one built from its
// drive ID or a per-VM counter. Observed: a Proxmox VM's virtio-scsi disk
// reports udev ID_SERIAL_SHORT=drive-scsi0 and a libvirt VM's
// drive-scsi0-0-0-0, which Proxmox's disks/list passes through as the serial.
func TestIsUsableHardwareIDRejectsQEMUDefaultDiskSerials(t *testing.T) {
	for _, serial := range []string{
		"drive-scsi0",
		"drive-scsi12",
		"DRIVE-SCSI0",
		"drive-scsi0-0-0-0",
		"drive-scsi1-0-2",
		"scsi0-hd0",
		"scsi1-hd3",
		"none0",
		"none12",
		"QM00001",
		"qm00005",
	} {
		if IsUsableHardwareID(serial) {
			t.Errorf("QEMU default serial %q was treated as usable hardware identity", serial)
		}
		if got := PreferredID(serial, "", "pve1", "/dev/sda", "", ""); got != "pve1:sda" {
			t.Errorf("QEMU default serial %q produced ID %q, want scoped fallback pve1:sda", serial, got)
		}
		if HardwareIdentityMatch(serial, "", serial, "") {
			t.Errorf("QEMU default serial %q matched itself as hardware identity", serial)
		}
	}

	// Anything outside those exact shapes stays usable, including real
	// serials that share a prefix with them. ATA disks never surface their
	// drive ID, so only SCSI drive IDs are rejected.
	for _, serial := range []string{
		"DRIVE-ASSET123",
		"drive-sata0",
		"drive-scsi0a",
		"drive-scsi0-0-0-0-0",
		"drive-scsi",
		"scsi0-cd0",
		"ide0-hd0",
		"none0a",
		"nonesuch1",
		"QM0001",
		"QM000001",
		"QM00001A",
		"QMX0001",
	} {
		if !IsUsableHardwareID(serial) {
			t.Errorf("serial %q outside QEMU's default grammar was rejected", serial)
		}
	}
}

func TestHardwareIdentityMatch(t *testing.T) {
	cases := []struct {
		name          string
		aSerial, aWWN string
		bSerial, bWWN string
		want          bool
	}{
		{
			name:    "pve bare-hex serial matches smartctl naa wwn",
			aSerial: "61866da053481f002f58a43b22f964a7",
			aWWN:    "0x61866da053481f00",
			bWWN:    "naa.61866da053481f002f58a43b22f964a7",
			want:    true,
		},
		{
			name:    "same serial different case",
			aSerial: "zr5dlayj",
			bSerial: "ZR5DLAYJ",
			want:    true,
		},
		{
			name: "udev wwn-0x token matches naa wwn",
			aWWN: "wwn-0x5000c500abcdef01",
			bWWN: "naa.5000c500abcdef01",
			want: true,
		},
		{
			name: "eui prefix matches bare nvme id",
			aWWN: "eui.0025385b91501234",
			bWWN: "0025385b91501234",
			want: true,
		},
		{
			// internal/hostagent formatWWN spells smartctl's NAA 5 triple
			// with unpadded fields; PVE reports udev's ID_WWN.
			name: "agent naa-oui-id wwn matches pve 0x wwn",
			aWWN: "5-c50-a1b2c3d4",
			bWWN: "0x5000c500a1b2c3d4",
			want: true,
		},
		{
			name: "agent naa-oui-id wwn of another disk stays distinct",
			aWWN: "5-c50-a1b2c3d5",
			bWWN: "0x5000c500a1b2c3d4",
			want: false,
		},
		{
			name:    "serial shaped like the field form is not re-padded",
			aSerial: "5-c50-a1b2c3d4",
			bWWN:    "0x5000c500a1b2c3d4",
			want:    false,
		},
		{
			name: "field form wider than NAA 5 is not re-padded",
			aWWN: "5-c50-123456789abc",
			bWWN: "0x5000c50123456789abc",
			want: false,
		},
		{
			name:    "truncated udev wwn never matches full sibling identifier",
			aWWN:    "0x61866da053481f00",
			bSerial: "61866da053481f0030543ecb1d3b4cca",
			bWWN:    "naa.61866da053481f0030543ecb1d3b4cca",
			want:    false,
		},
		{
			name:    "placeholder serials do not match each other",
			aSerial: "UNKNOWN",
			bSerial: "UNKNOWN",
			want:    false,
		},
		{
			name:    "distinct disks stay distinct",
			aSerial: "9410A0FWFVL9",
			bSerial: "35C0A39YFVL9",
			want:    false,
		},
		{
			name: "empty observations never match",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HardwareIdentityMatch(tc.aSerial, tc.aWWN, tc.bSerial, tc.bWWN)
			if got != tc.want {
				t.Fatalf("HardwareIdentityMatch(%q,%q,%q,%q) = %v, want %v",
					tc.aSerial, tc.aWWN, tc.bSerial, tc.bWWN, got, tc.want)
			}
			mirrored := HardwareIdentityMatch(tc.bSerial, tc.bWWN, tc.aSerial, tc.aWWN)
			if mirrored != tc.want {
				t.Fatalf("match is not symmetric: mirrored = %v, want %v", mirrored, tc.want)
			}
		})
	}
}
