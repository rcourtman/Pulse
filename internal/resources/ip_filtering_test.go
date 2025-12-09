package resources

import (
	"testing"
	"time"
)

// TestIsNonUniqueIP tests the IP filtering logic for deduplication.
// This was added to fix the issue where Docker bridge IPs (172.17.0.1) were
// causing false positive deduplication between hosts.
func TestIsNonUniqueIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Localhost addresses - should be filtered
		{"localhost IPv4", "127.0.0.1", true},
		{"localhost IPv4 alternate", "127.0.0.2", true},
		{"localhost IPv4 high", "127.255.255.255", true},
		{"localhost IPv6", "::1", true},

		// Docker bridge network addresses - should be filtered (the key fix)
		{"docker bridge default", "172.17.0.1", true},
		{"docker bridge container", "172.17.0.2", true},
		{"docker bridge high", "172.17.255.255", true},
		{"docker bridge 172.18.x", "172.18.0.1", true},
		{"docker bridge 172.19.x", "172.19.0.1", true},
		{"docker bridge 172.20.x", "172.20.0.1", true},
		{"docker bridge 172.21.x", "172.21.0.1", true},
		{"docker bridge 172.22.x", "172.22.0.1", true},

		// Link-local IPv6 - should be filtered
		{"link-local IPv6", "fe80::1", true},
		{"link-local IPv6 full", "fe80:0000:0000:0000:0000:0000:0000:0001", true},
		{"link-local uppercase", "FE80::1:2:3", true},

		// Valid private IPs - should NOT be filtered
		{"private 192.168.x", "192.168.1.100", false},
		{"private 192.168.0.x", "192.168.0.1", false},
		{"private 10.x", "10.0.0.1", false},
		{"private 10.x alternate", "10.10.10.10", false},
		{"private 172.16.x", "172.16.0.1", false},
		{"private 172.23.x", "172.23.0.1", false}, // Outside Docker bridge range
		{"private 172.24.x", "172.24.0.1", false},
		{"private 172.31.x", "172.31.255.255", false},

		// Public IPs - should NOT be filtered
		{"public IP 1", "8.8.8.8", false},
		{"public IP 2", "1.1.1.1", false},
		{"public IP 3", "203.0.113.42", false},

		// Public IPv6 - should NOT be filtered
		{"public IPv6", "2001:db8::1", false},
		{"public IPv6 documentation", "2001:0db8:85a3::8a2e:0370:7334", false},

		// Edge cases
		{"empty string", "", false},
		{"random string", "not-an-ip", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isNonUniqueIP(tt.ip)
			if result != tt.expected {
				t.Errorf("isNonUniqueIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestNoDeduplicationForDockerBridgeIP verifies that hosts with Docker bridge IPs
// are not incorrectly deduplicated. This was the root cause of the "delly disappearing" bug.
func TestNoDeduplicationForDockerBridgeIP(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// Host 1 with Docker bridge IP
	host1 := Resource{
		ID:           "host-1",
		Type:         ResourceTypeHost,
		Name:         "server1",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server1",
			IPs:      []string{"192.168.1.10", "172.17.0.1"}, // 172.17.0.1 is Docker bridge
		},
	}
	store.Upsert(host1)

	// Host 2 with same Docker bridge IP (every Docker host has this!)
	host2 := Resource{
		ID:           "host-2",
		Type:         ResourceTypeHost,
		Name:         "server2",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server2",
			IPs:      []string{"192.168.1.20", "172.17.0.1"}, // Same Docker bridge IP
		},
	}
	store.Upsert(host2)

	// Both should exist (172.17.0.1 shouldn't trigger dedup)
	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 hosts (Docker bridge IP shouldn't cause dedup), got %d", len(all))
	}

	// Verify both are retrievable
	_, ok1 := store.Get("host-1")
	_, ok2 := store.Get("host-2")
	if !ok1 || !ok2 {
		t.Error("Both hosts should be retrievable independently")
	}
}

// TestDeduplicationStillWorksForUniqueIPs verifies that deduplication still works
// for actual unique IPs while filtering out non-unique ones.
func TestDeduplicationStillWorksForUniqueIPs(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// Add first host
	host1 := Resource{
		ID:           "host-old",
		Type:         ResourceTypeHost,
		Name:         "server",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "server",
			IPs:      []string{"192.168.1.100"}, // Unique IP
		},
	}
	store.Upsert(host1)

	// Add second host with same unique IP but different ID (same physical machine)
	host2 := Resource{
		ID:           "host-new",
		Type:         ResourceTypeHost,
		Name:         "server-renamed",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now.Add(time.Second), // Newer
		Identity: &ResourceIdentity{
			Hostname: "server-renamed",
			IPs:      []string{"192.168.1.100"}, // Same unique IP
		},
	}
	store.Upsert(host2)

	// Should have only 1 host (deduplicated by unique IP)
	all := store.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 host (deduplicated by unique IP), got %d", len(all))
	}
}

// TestMixedIPsDeduplication verifies that a host with both unique and non-unique IPs
// is properly handled (dedup only on unique IPs).
func TestMixedIPsDeduplication(t *testing.T) {
	store := NewStore()

	now := time.Now()

	// Host with unique IP and Docker bridge
	host1 := Resource{
		ID:           "host-1",
		Type:         ResourceTypeHost,
		Name:         "docker-server-1",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "docker-server-1",
			IPs:      []string{"192.168.1.50", "172.17.0.1", "127.0.0.1"},
		},
	}
	store.Upsert(host1)

	// Different host sharing only the non-unique IPs
	host2 := Resource{
		ID:           "host-2",
		Type:         ResourceTypeHost,
		Name:         "docker-server-2",
		PlatformType: PlatformHostAgent,
		SourceType:   SourceAgent,
		LastSeen:     now,
		Identity: &ResourceIdentity{
			Hostname: "docker-server-2",
			IPs:      []string{"192.168.1.60", "172.17.0.1", "127.0.0.1"}, // Different unique IP
		},
	}
	store.Upsert(host2)

	// Both should exist (only 127.0.0.1 and 172.17.0.1 are shared, which are non-unique)
	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 hosts (shared non-unique IPs only), got %d", len(all))
	}
}
