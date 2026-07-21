package monitoring

import "testing"

// PVE's disks/list endpoint reports ATA drives as PASSED/FAILED! and SCSI/SAS
// drives as OK or a failure sentence. The raw OK previously reached the UI
// untouched and rendered as Unknown (#1595).
func TestNormalizeProxmoxDiskHealth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"scsi ok", "OK", "PASSED"},
		{"scsi ok lowercase", "ok", "PASSED"},
		{"ata passed", "PASSED", "PASSED"},
		{"ata failed bang", "FAILED!", "FAILED"},
		{"scsi failure sentence", "FAILURE PREDICTION THRESHOLD EXCEEDED", "FAILED"},
		{"unknown passthrough", "UNKNOWN", "UNKNOWN"},
		{"empty passthrough", "", ""},
		{"whitespace trimmed", "  OK  ", "PASSED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeProxmoxDiskHealth(tc.in); got != tc.want {
				t.Fatalf("normalizeProxmoxDiskHealth(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
