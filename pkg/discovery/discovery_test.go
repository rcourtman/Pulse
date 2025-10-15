package discovery

import "testing"

func TestInferTypeFromMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "detects PMG from auth header",
			parts: []string{`PMGAuth realm="Proxmox Mail Gateway"`, "pmgproxy/4.0"},
			want:  "pmg",
		},
		{
			name:  "detects PVE from realm string",
			parts: []string{`PVEAuth realm="Proxmox Virtual Environment"`, "pve-api-daemon/3.0"},
			want:  "pve",
		},
		{
			name:  "detects PBS from cookie",
			parts: []string{"PBS", "PBSCookie=abc123", `PBSAuth realm="Proxmox Backup Server"`},
			want:  "pbs",
		},
		{
			name:  "returns empty when no markers",
			parts: []string{"Custom Certificate", "Example Corp"},
			want:  "",
		},
		{
			name:  "tolerates compact strings",
			parts: []string{"ProxmoxMailGateway"},
			want:  "pmg",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := inferTypeFromMetadata(tc.parts...); got != tc.want {
				t.Fatalf("inferTypeFromMetadata(%v) = %q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}
