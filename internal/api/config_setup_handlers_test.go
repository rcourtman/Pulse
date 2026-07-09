package api

import "testing"

func TestShouldPreserveExistingAutoRegisterHost(t *testing.T) {
	tests := []struct {
		name           string
		existingHost   string
		candidateHosts []string
		want           bool
	}{
		{
			name:           "existing host outranked in candidate list is replaced",
			existingHost:   "https://pve02:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           false,
		},
		{
			name:           "existing host matches candidate case-insensitively",
			existingHost:   "https://PVE02:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           false,
		},
		{
			name:           "admin-managed host absent from candidates is preserved",
			existingHost:   "https://pve02.internal.example.com:8006",
			candidateHosts: []string{"https://192.168.1.12:8006", "https://pve02:8006"},
			want:           true,
		},
		{
			name:           "empty candidate list preserves existing host",
			existingHost:   "https://pve02:8006",
			candidateHosts: nil,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPreserveExistingAutoRegisterHost(tt.existingHost, tt.candidateHosts); got != tt.want {
				t.Fatalf("shouldPreserveExistingAutoRegisterHost(%q, %v) = %v, want %v",
					tt.existingHost, tt.candidateHosts, got, tt.want)
			}
		})
	}
}
