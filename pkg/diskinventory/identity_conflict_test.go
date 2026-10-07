package diskinventory

import "testing"

func TestHardwareIdentityConflict(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		leftSerial, leftWWN   string
		rightSerial, rightWWN string
		want                  bool
	}{
		{name: "different serials", leftSerial: "WD-NEW0001", rightSerial: "WD-OLD0001", want: true},
		{name: "different WWNs", leftWWN: "0x5000c500b0000001", rightWWN: "naa.5000c500a0000001", want: true},
		{name: "WWN disagrees while serial is absent on one side", leftSerial: "PVE-ONLY", leftWWN: "0x5000c500b0000001", rightWWN: "5-c50-a0000001", want: true},
		{name: "same serial", leftSerial: "WD-WX1234", rightSerial: "wd-wx1234"},
		{name: "udev underscores for whitespace", leftSerial: "WD-WX_1234", rightSerial: "WD-WX 1234"},
		{name: "T10 designator ending with the drive serial", leftSerial: "ATA_ST4000NM000A-2HZ_ZC1ABCDE", rightSerial: "ZC1ABCDE"},
		{name: "serial that only begins with the other", leftSerial: "SN1234", rightSerial: "SN123", want: true},
		{name: "bare serial that ends with the other", leftSerial: "001234567890", rightSerial: "1234567890", want: true},
		{name: "agent NAA fields against PVE hex", leftSerial: "5000c500a0000001", leftWWN: "0x5000c500a0000001", rightSerial: "ZR5TESTA0001", rightWWN: "5-c50-a0000001"},
		{name: "udev 64-bit prefix of an NAA 6 WWN", leftWWN: "0x600508b1001c4d5e", rightWWN: "naa.600508b1001c4d5e7f8a9b0c1d2e3f40"},
		{name: "RAID volume NAA as serial against WWN", leftSerial: "600508b1001c4d5e7f8a9b0c1d2e3f40", leftWWN: "0x600508b1001c4d5e", rightSerial: "OTHER", rightWWN: "naa.600508b1001c4d5e7f8a9b0c1d2e3f40"},
		{name: "serial against WWN only is no evidence", leftSerial: "WD-NEW0001", rightWWN: "naa.5000c500a0000001"},
		{name: "placeholders are no evidence", leftSerial: "unknown", leftWWN: "0x0000000000000000", rightSerial: "WD-OLD0001", rightWWN: "naa.5000c500a0000001"},
		{name: "one side without identity", rightSerial: "WD-OLD0001", rightWWN: "naa.5000c500a0000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HardwareIdentityConflict(tc.leftSerial, tc.leftWWN, tc.rightSerial, tc.rightWWN); got != tc.want {
				t.Fatalf("HardwareIdentityConflict = %v, want %v", got, tc.want)
			}
			if got := HardwareIdentityConflict(tc.rightSerial, tc.rightWWN, tc.leftSerial, tc.leftWWN); got != tc.want {
				t.Fatalf("HardwareIdentityConflict (swapped) = %v, want %v", got, tc.want)
			}
		})
	}
}
