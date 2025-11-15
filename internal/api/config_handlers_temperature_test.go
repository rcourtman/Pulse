package api

import "testing"

func TestDetermineTemperatureTransport(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                string
		enabled             bool
		proxyURL            string
		proxyToken          string
		socketAvailable     bool
		containerSSHBlocked bool
		expectedTransport   string
	}{
		{
			name:              "disabled",
			enabled:           false,
			expectedTransport: temperatureTransportDisabled,
		},
		{
			name:              "https proxy preferred when configured",
			enabled:           true,
			proxyURL:          " https://pve.example ",
			proxyToken:        "token",
			expectedTransport: temperatureTransportHTTPSProxy,
		},
		{
			name:              "socket proxy when available",
			enabled:           true,
			socketAvailable:   true,
			expectedTransport: temperatureTransportSocketProxy,
		},
		{
			name:                "ssh blocked in container without override",
			enabled:             true,
			socketAvailable:     false,
			containerSSHBlocked: true,
			expectedTransport:   temperatureTransportSSHBlocked,
		},
		{
			name:              "ssh fallback when nothing else available",
			enabled:           true,
			expectedTransport: temperatureTransportSSHFallback,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := determineTemperatureTransport(tc.enabled, tc.proxyURL, tc.proxyToken, tc.socketAvailable, tc.containerSSHBlocked)
			if got != tc.expectedTransport {
				t.Fatalf("expected %q, got %q", tc.expectedTransport, got)
			}
		})
	}
}
