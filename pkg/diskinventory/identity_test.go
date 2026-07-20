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
