package api

import "testing"

func TestNormalizeNodeHost(t *testing.T) {
	tests := []struct {
		name     string
		rawHost  string
	nodeType string
	want     string
}{
	{
		name:     "adds default port to explicit https without port",
		rawHost:  "https://example.com",
		nodeType: "pve",
		want:     "https://example.com:8006",
	},
	{
		name:     "adds default port for bare pve host",
		rawHost:  "pve.lan",
		nodeType: "pve",
			want:     "https://pve.lan:8006",
		},
		{
			name:     "adds default port for bare pbs host",
			rawHost:  "pbs.lan",
			nodeType: "pbs",
			want:     "https://pbs.lan:8007",
		},
		{
			name:     "preserves custom port",
			rawHost:  "https://example.com:8443",
			nodeType: "pve",
			want:     "https://example.com:8443",
		},
		{
			name:     "supports ipv6 without scheme",
			rawHost:  "2001:db8::1",
			nodeType: "pmg",
			want:     "https://[2001:db8::1]:8006",
		},
	{
		name:     "drops path segments",
		rawHost:  "https://example.com/api",
		nodeType: "pve",
		want:     "https://example.com:8006",
	},
	{
		name:     "adds default port to explicit http scheme",
		rawHost:  "http://example.com",
		nodeType: "pve",
		want:     "http://example.com:8006",
	},
}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNodeHost(tt.rawHost, tt.nodeType)
			if err != nil {
				t.Fatalf("normalizeNodeHost returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeNodeHost(%q, %q) = %q, want %q", tt.rawHost, tt.nodeType, got, tt.want)
			}
		})
	}
}
