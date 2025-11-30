package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestExtractHostPart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty/whitespace inputs
		{name: "empty string", input: "", expected: ""},
		{name: "whitespace only", input: "   ", expected: ""},
		{name: "tabs and newlines", input: "\t\n", expected: ""},

		// Simple hostnames
		{name: "simple hostname", input: "pve1.local", expected: "pve1.local"},
		{name: "hostname with whitespace", input: "  pve1.local  ", expected: "pve1.local"},
		{name: "ip address", input: "192.168.1.100", expected: "192.168.1.100"},

		// Hostnames with ports
		{name: "hostname with port", input: "pve1.local:8006", expected: "pve1.local"},
		{name: "ip with port", input: "192.168.1.100:8006", expected: "192.168.1.100"},
		{name: "ipv4 with default port", input: "10.0.0.1:443", expected: "10.0.0.1"},

		// URLs with scheme
		{name: "https url", input: "https://pve1.local", expected: "pve1.local"},
		{name: "https url with port", input: "https://pve1.local:8006", expected: "pve1.local"},
		{name: "http url", input: "http://pve1.local", expected: "pve1.local"},
		{name: "http url with port", input: "http://pve1.local:8080", expected: "pve1.local"},
		{name: "https url with ip", input: "https://192.168.1.100:8006", expected: "192.168.1.100"},

		// URLs with paths
		{name: "url with path", input: "https://pve1.local/api2", expected: "pve1.local"},
		{name: "url with path and port", input: "https://pve1.local:8006/api2/json", expected: "pve1.local"},

		// Hostnames with paths (no scheme)
		{name: "hostname with path", input: "pve1.local/api", expected: "pve1.local"},
		{name: "hostname with port and path", input: "pve1.local:8006/api", expected: "pve1.local"},

		// Edge cases
		{name: "just a port colon", input: ":8006", expected: ""},
		{name: "just a slash", input: "/api", expected: ""},
		{name: "double colon", input: "host::8006", expected: "host"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractHostPart(tc.input)
			if result != tc.expected {
				t.Errorf("extractHostPart(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestBuildAuthorizedNodeList(t *testing.T) {
	t.Run("empty instances", func(t *testing.T) {
		result := buildAuthorizedNodeList(nil)
		if len(result) != 0 {
			t.Errorf("expected empty list for nil input, got %d items", len(result))
		}

		result = buildAuthorizedNodeList([]config.PVEInstance{})
		if len(result) != 0 {
			t.Errorf("expected empty list for empty slice, got %d items", len(result))
		}
	})

	t.Run("single instance with host", func(t *testing.T) {
		instances := []config.PVEInstance{
			{Name: "pve1", Host: "https://pve1.local:8006"},
		}
		result := buildAuthorizedNodeList(instances)

		// Function calls add() for both Host and GuestURL (even if empty)
		// With empty GuestURL, we get one entry from Host only (name=pve1, IP=pve1.local)
		// But it also adds an entry for the empty GuestURL case: (name=pve1, IP="")
		// So we expect 2 entries
		if len(result) != 2 {
			t.Fatalf("expected 2 nodes (Host + empty GuestURL), got %d: %+v", len(result), result)
		}

		// Verify the Host-based entry exists
		foundHostEntry := false
		for _, node := range result {
			if node.Name == "pve1" && node.IP == "pve1.local" {
				foundHostEntry = true
				break
			}
		}
		if !foundHostEntry {
			t.Errorf("expected entry for Host (name=pve1, IP=pve1.local) in %+v", result)
		}
	})

	t.Run("instance with GuestURL", func(t *testing.T) {
		instances := []config.PVEInstance{
			{
				Name:     "pve1",
				Host:     "https://pve1.local:8006",
				GuestURL: "https://guest.local:8006",
			},
		}
		result := buildAuthorizedNodeList(instances)

		// Should have 2 entries: one from Host and one from GuestURL
		if len(result) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(result))
		}

		// Check that both entries exist
		foundHost := false
		foundGuest := false
		for _, node := range result {
			if node.IP == "pve1.local" {
				foundHost = true
			}
			if node.IP == "guest.local" {
				foundGuest = true
			}
		}
		if !foundHost {
			t.Error("missing entry for Host URL")
		}
		if !foundGuest {
			t.Error("missing entry for GuestURL")
		}
	})

	t.Run("cluster endpoints", func(t *testing.T) {
		instances := []config.PVEInstance{
			{
				Name:      "cluster1",
				Host:      "https://pve1.local:8006",
				IsCluster: true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve2", Host: "https://pve2.local:8006", IP: ""},
					{NodeName: "pve3", Host: "", IP: "192.168.1.103"},
				},
			},
		}
		result := buildAuthorizedNodeList(instances)

		// Should have 4 entries:
		// - (cluster1, pve1.local) from Host
		// - (cluster1, "") from empty GuestURL
		// - (pve2, pve2.local) from ClusterEndpoint[0]
		// - (pve3, 192.168.1.103) from ClusterEndpoint[1]
		if len(result) != 4 {
			t.Fatalf("expected 4 nodes, got %d: %+v", len(result), result)
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		instances := []config.PVEInstance{
			{Name: "pve1", Host: "https://pve1.local:8006"},
			{Name: "pve1", Host: "https://pve1.local:8006"}, // duplicate
		}
		result := buildAuthorizedNodeList(instances)

		// Each instance adds 2 entries (Host and empty GuestURL)
		// But deduplication should result in 2 unique entries:
		// - (pve1, pve1.local) - from Host
		// - (pve1, "") - from empty GuestURL
		if len(result) != 2 {
			t.Errorf("expected 2 nodes after dedup (host + empty guesturl), got %d: %+v", len(result), result)
		}
	})

	t.Run("empty name and ip filtered", func(t *testing.T) {
		instances := []config.PVEInstance{
			{Name: "", Host: ""},           // should be skipped
			{Name: "pve1", Host: ""},       // should be added with empty IP
			{Name: "", Host: "pve2.local"}, // should be added with empty name
		}
		result := buildAuthorizedNodeList(instances)

		// First entry has empty name and IP - should be skipped
		// Second has name but empty IP
		// Third has empty name but IP
		if len(result) != 2 {
			t.Errorf("expected 2 nodes, got %d: %+v", len(result), result)
		}
	})

	t.Run("cluster endpoint with IP takes precedence", func(t *testing.T) {
		instances := []config.PVEInstance{
			{
				Name:      "cluster1",
				Host:      "https://pve1.local:8006",
				IsCluster: true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve2", Host: "https://pve2.local:8006", IP: "10.0.0.2"},
				},
			},
		}
		result := buildAuthorizedNodeList(instances)

		// Should have 2 entries: pve1.local and 10.0.0.2 (IP from endpoint, not host)
		foundIP := false
		for _, node := range result {
			if node.IP == "10.0.0.2" {
				foundIP = true
			}
			if node.IP == "pve2.local" {
				t.Error("expected IP to take precedence over Host extraction")
			}
		}
		if !foundIP {
			t.Error("expected to find IP 10.0.0.2 from cluster endpoint")
		}
	})

	t.Run("cluster endpoint name fallback to host", func(t *testing.T) {
		instances := []config.PVEInstance{
			{
				Name:      "cluster1",
				Host:      "https://pve1.local:8006",
				IsCluster: true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "", Host: "pve2.local", IP: "10.0.0.2"},
				},
			},
		}
		result := buildAuthorizedNodeList(instances)

		// Should find an entry where name is the host value
		foundNameFallback := false
		for _, node := range result {
			if node.Name == "pve2.local" && node.IP == "10.0.0.2" {
				foundNameFallback = true
			}
		}
		if !foundNameFallback {
			t.Errorf("expected name to fallback to host, got: %+v", result)
		}
	})
}

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generates non-empty token", func(t *testing.T) {
		token, err := generateSecureToken(32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("token length corresponds to byte length", func(t *testing.T) {
		// 32 bytes = 256 bits, base64 encoded = 44 chars (with padding)
		token, err := generateSecureToken(32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Base64 URL encoding: 32 bytes = ceil(32 * 4 / 3) = 43 chars + padding = 44
		if len(token) != 44 {
			t.Errorf("expected token length 44, got %d", len(token))
		}
	})

	t.Run("tokens are unique", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := generateSecureToken(32)
			if err != nil {
				t.Fatalf("unexpected error on iteration %d: %v", i, err)
			}
			if tokens[token] {
				t.Errorf("duplicate token generated on iteration %d", i)
			}
			tokens[token] = true
		}
	})

	t.Run("different byte lengths", func(t *testing.T) {
		testCases := []struct {
			bytes          int
			expectedLength int
		}{
			{16, 24}, // 16 bytes = 128 bits, base64 = 22 chars + 2 padding
			{24, 32}, // 24 bytes = 192 bits, base64 = 32 chars
			{48, 64}, // 48 bytes = 384 bits, base64 = 64 chars
		}

		for _, tc := range testCases {
			token, err := generateSecureToken(tc.bytes)
			if err != nil {
				t.Errorf("unexpected error for %d bytes: %v", tc.bytes, err)
				continue
			}
			if len(token) != tc.expectedLength {
				t.Errorf("generateSecureToken(%d) length = %d, want %d", tc.bytes, len(token), tc.expectedLength)
			}
		}
	})

	t.Run("zero bytes returns empty string", func(t *testing.T) {
		token, err := generateSecureToken(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "" {
			t.Errorf("expected empty token for 0 bytes, got %q", token)
		}
	})
}

func TestProxySyncState(t *testing.T) {
	t.Run("record and snapshot sync", func(t *testing.T) {
		handlers := NewTemperatureProxyHandlers(nil, nil, nil)

		// Initially empty
		snap := handlers.SnapshotSyncStatus()
		if snap != nil {
			t.Errorf("expected nil snapshot initially, got %+v", snap)
		}

		// Record a sync
		handlers.recordSync("pve1", 60)

		snap = handlers.SnapshotSyncStatus()
		if snap == nil {
			t.Fatal("expected non-nil snapshot after recording sync")
		}
		if len(snap) != 1 {
			t.Errorf("expected 1 entry, got %d", len(snap))
		}

		// Key should be lowercase
		state, ok := snap["pve1"]
		if !ok {
			t.Error("expected entry for 'pve1'")
		}
		if state.Instance != "pve1" {
			t.Errorf("expected Instance 'pve1', got %q", state.Instance)
		}
		if state.RefreshInterval != 60 {
			t.Errorf("expected RefreshInterval 60, got %d", state.RefreshInterval)
		}
		if state.LastPull.IsZero() {
			t.Error("expected non-zero LastPull time")
		}
	})

	t.Run("record with empty instance ignored", func(t *testing.T) {
		handlers := NewTemperatureProxyHandlers(nil, nil, nil)

		handlers.recordSync("", 60)
		handlers.recordSync("   ", 60)

		snap := handlers.SnapshotSyncStatus()
		if snap != nil {
			t.Errorf("expected nil snapshot for empty instances, got %+v", snap)
		}
	})

	t.Run("default refresh interval", func(t *testing.T) {
		handlers := NewTemperatureProxyHandlers(nil, nil, nil)

		handlers.recordSync("pve1", 0)  // 0 should use default
		handlers.recordSync("pve2", -1) // negative should use default

		snap := handlers.SnapshotSyncStatus()
		if snap["pve1"].RefreshInterval != defaultProxyAllowlistRefreshSeconds {
			t.Errorf("expected default refresh for 0, got %d", snap["pve1"].RefreshInterval)
		}
		if snap["pve2"].RefreshInterval != defaultProxyAllowlistRefreshSeconds {
			t.Errorf("expected default refresh for -1, got %d", snap["pve2"].RefreshInterval)
		}
	})

	t.Run("snapshot returns copy", func(t *testing.T) {
		handlers := NewTemperatureProxyHandlers(nil, nil, nil)
		handlers.recordSync("pve1", 60)

		snap1 := handlers.SnapshotSyncStatus()
		snap2 := handlers.SnapshotSyncStatus()

		// Modify snap1
		snap1["modified"] = proxySyncState{Instance: "modified"}

		// snap2 should not be affected
		if _, ok := snap2["modified"]; ok {
			t.Error("snapshot should return a copy, not a reference")
		}
	})

	t.Run("nil handlers safe", func(t *testing.T) {
		var handlers *TemperatureProxyHandlers

		// These should not panic
		handlers.recordSync("pve1", 60)
		snap := handlers.SnapshotSyncStatus()
		if snap != nil {
			t.Error("expected nil from nil handlers")
		}
	})

	t.Run("case insensitive key storage", func(t *testing.T) {
		handlers := NewTemperatureProxyHandlers(nil, nil, nil)

		handlers.recordSync("PVE1", 60)
		handlers.recordSync("pve1", 120) // should overwrite

		snap := handlers.SnapshotSyncStatus()
		if len(snap) != 1 {
			t.Errorf("expected 1 entry after case-insensitive merge, got %d", len(snap))
		}
		if snap["pve1"].RefreshInterval != 120 {
			t.Errorf("expected overwritten refresh 120, got %d", snap["pve1"].RefreshInterval)
		}
	})
}
