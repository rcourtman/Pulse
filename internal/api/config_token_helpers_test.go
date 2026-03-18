package api

import "testing"

func TestBuildPulseMonitorTokenName(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		want       string
	}{
		{
			name:       "uses url host when available",
			candidates: []string{"https://192.168.0.98:7655"},
			want:       "pulse-192-168-0-98",
		},
		{
			name:       "uses bare host with port",
			candidates: []string{"pulse.example.com:7655"},
			want:       "pulse-pulse-example-com",
		},
		{
			name:       "falls back to second candidate",
			candidates: []string{"", "tower.local"},
			want:       "pulse-tower-local",
		},
		{
			name:       "prefers canonical pulse url over request local fallback",
			candidates: []string{"https://public.example.com/base", "127.0.0.1:7655"},
			want:       "pulse-public-example-com",
		},
		{
			name:       "falls back to server when no valid candidates",
			candidates: []string{"", "   "},
			want:       "pulse-server",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildPulseMonitorTokenName(tc.candidates...)
			if got != tc.want {
				t.Fatalf("buildPulseMonitorTokenName(%v) = %q, want %q", tc.candidates, got, tc.want)
			}
		})
	}
}

func TestPulseTokenSlugTruncatesToMaxLen(t *testing.T) {
	// 64 chars to ensure truncation path is exercised.
	raw := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	got := pulseTokenSlug(raw)
	if len(got) > 48 {
		t.Fatalf("pulseTokenSlug length = %d, want <= 48", len(got))
	}
}

func TestIsPulseAgentToken(t *testing.T) {
	tests := []struct {
		tokenID string
		want    bool
	}{
		{tokenID: "pulse-monitor@pve!pulse-192-168-0-98", want: true},
		{tokenID: "pulse-monitor@pbs!pulse-server", want: true},
		{tokenID: "root@pam!pulse-foo", want: false},
		{tokenID: "pulse-monitor@pve!other-token", want: false},
	}

	for _, tc := range tests {
		if got := isPulseAgentToken(tc.tokenID); got != tc.want {
			t.Fatalf("isPulseAgentToken(%q) = %v, want %v", tc.tokenID, got, tc.want)
		}
	}
}
