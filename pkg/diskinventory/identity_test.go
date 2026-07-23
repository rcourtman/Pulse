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
	} {
		if IsUsableHardwareID(placeholder) {
			t.Fatalf("placeholder %q was treated as usable hardware identity", placeholder)
		}
		if got := PreferredID(placeholder, "", "host-a", "/dev/sda", "", ""); got != "host-a:sda" {
			t.Fatalf("placeholder %q produced ID %q, want scoped fallback", placeholder, got)
		}
	}
	if !IsUsableHardwareID("ZR5DLAYJ") || !IsUsableHardwareID("naa.5000c500abcdef01") {
		t.Fatal("real disk serial/WWN was rejected")
	}
}
