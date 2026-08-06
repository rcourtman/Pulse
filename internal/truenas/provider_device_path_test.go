package truenas

import "testing"

// vdev.Device arrives from the appliance API and is published verbatim on
// ZFSDevice.Path, so devicePath normalises it instead of trusting it.
func TestDevicePathNormalisesUntrustedApplianceValues(t *testing.T) {
	cases := []struct {
		name   string
		device string
		want   string
	}{
		{name: "bare device node", device: "sda", want: "/dev/sda"},
		{name: "partition", device: "nvme0n1p2", want: "/dev/nvme0n1p2"},
		{name: "absolute by-id path preserved", device: "/dev/disk/by-id/ata-SAMSUNG", want: "/dev/disk/by-id/ata-SAMSUNG"},
		{name: "freebsd gptid", device: "gptid/abcd-1234", want: "/dev/gptid/abcd-1234"},
		{name: "whitespace trimmed", device: "  sdb  ", want: "/dev/sdb"},
		{name: "empty stays empty", device: "", want: ""},

		{name: "leading double slash collapsed", device: "//evil.example.com/share", want: "/evil.example.com/share"},
		{name: "leading triple slash collapsed", device: "///evil", want: "/evil"},
		{name: "backslash rejected", device: `/\evil.example.com`, want: ""},
		{name: "windows style path rejected", device: `C:\Windows`, want: ""},
		{name: "relative traversal rejected", device: "../../etc/passwd", want: ""},
		{name: "absolute traversal rejected", device: "/dev/../etc/passwd", want: ""},
		{name: "embedded traversal rejected", device: "disk/../../root", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := devicePath(tc.device); got != tc.want {
				t.Fatalf("devicePath(%q) = %q, want %q", tc.device, got, tc.want)
			}
		})
	}
}
