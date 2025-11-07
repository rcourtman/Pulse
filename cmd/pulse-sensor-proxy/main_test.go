package main

import (
	"testing"
)

// TestPrivilegedMethodsCompleteness ensures all host-side RPC methods are in privilegedMethods
func TestPrivilegedMethodsCompleteness(t *testing.T) {
	// Define RPC methods that expose host-side effects
	hostSideEffects := map[string]string{
		RPCEnsureClusterKeys: "SSH key distribution to cluster nodes",
		RPCRegisterNodes:     "Node discovery and registration",
		RPCRequestCleanup:    "Cleanup operations on host",
	}

	// Verify each host-side effect RPC is in privilegedMethods
	for method, description := range hostSideEffects {
		if !privilegedMethods[method] {
			t.Errorf("SECURITY: %s (%s) is not in privilegedMethods - containers can call it!", method, description)
		}
	}

	// Verify read-only methods are NOT in privilegedMethods
	readOnlyMethods := map[string]string{
		RPCGetStatus:      "proxy status query",
		RPCGetTemperature: "temperature data query",
	}

	for method, description := range readOnlyMethods {
		if privilegedMethods[method] {
			t.Errorf("Read-only method %s (%s) should not be in privilegedMethods", method, description)
		}
	}
}

// TestPrivilegedMethodsBlocked ensures containers cannot call privileged methods
func TestPrivilegedMethodsBlocked(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		allowedPeerUIDs:   map[uint32]struct{}{0: {}},
		allowedPeerGIDs:   map[uint32]struct{}{0: {}},
		peerCapabilities:  map[uint32]Capability{0: capabilityLegacyAll},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	// Container credentials (ID-mapped root)
	containerCreds := &peerCredentials{
		uid: 101000, // Inside ID-mapped range
		gid: 101000,
		pid: 12345,
	}

	// Host credentials (real root)
	hostCreds := &peerCredentials{
		uid: 0,
		gid: 0,
		pid: 1,
	}

	// Test that containers ARE blocked from privileged methods
	t.Run("ContainerBlockedFromPrivilegedMethods", func(t *testing.T) {
		// Container should pass authentication
		caps, err := p.authorizePeer(containerCreds)
		if err != nil {
			t.Fatalf("Container should pass authentication, got: %v", err)
		}

		if caps.Has(CapabilityAdmin) {
			t.Fatal("Container should not have admin capability")
		}
	})

	// Test that host CAN call privileged methods
	t.Run("HostAllowedPrivilegedMethods", func(t *testing.T) {
		// Host should pass authentication
		caps, err := p.authorizePeer(hostCreds)
		if err != nil {
			t.Fatalf("Host should pass authentication, got: %v", err)
		}

		if !caps.Has(CapabilityAdmin) {
			t.Fatal("Host should have admin capability")
		}
	})
}

// TestIDMappedRootDetection tests container detection via ID mapping
func TestIDMappedRootDetection(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	tests := []struct {
		name       string
		cred       *peerCredentials
		isIDMapped bool
	}{
		{
			name:       "Container root (ID-mapped)",
			cred:       &peerCredentials{uid: 100000, gid: 100000},
			isIDMapped: true,
		},
		{
			name:       "Container user inside range",
			cred:       &peerCredentials{uid: 110000, gid: 110000},
			isIDMapped: true,
		},
		{
			name:       "Container at range boundary",
			cred:       &peerCredentials{uid: 165535, gid: 165535},
			isIDMapped: true,
		},
		{
			name:       "Host root",
			cred:       &peerCredentials{uid: 0, gid: 0},
			isIDMapped: false,
		},
		{
			name:       "Host user (low UID)",
			cred:       &peerCredentials{uid: 1000, gid: 1000},
			isIDMapped: false,
		},
		{
			name:       "Outside range (high)",
			cred:       &peerCredentials{uid: 200000, gid: 200000},
			isIDMapped: false,
		},
		{
			name:       "UID in range but GID not (should fail)",
			cred:       &peerCredentials{uid: 110000, gid: 50},
			isIDMapped: false,
		},
		{
			name:       "GID in range but UID not (should fail)",
			cred:       &peerCredentials{uid: 50, gid: 110000},
			isIDMapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.isIDMappedRoot(tt.cred)
			if got != tt.isIDMapped {
				t.Errorf("isIDMappedRoot() = %v, want %v for uid=%d gid=%d",
					got, tt.isIDMapped, tt.cred.uid, tt.cred.gid)
			}
		})
	}
}

// TestIDMappedRootWithoutRanges tests behavior when no ID ranges configured
func TestIDMappedRootWithoutRanges(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{}, // Empty
		idMappedGIDRanges: []idRange{}, // Empty
	}

	// Should return false when no ranges are configured
	cred := &peerCredentials{uid: 110000, gid: 110000}
	if p.isIDMappedRoot(cred) {
		t.Error("isIDMappedRoot should return false when no ranges configured")
	}
}

// TestIDMappedRootDisabled tests when AllowIDMappedRoot is disabled
func TestIDMappedRootDisabled(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: false},
		allowedPeerUIDs:   map[uint32]struct{}{0: {}},
		peerCapabilities:  map[uint32]Capability{0: capabilityLegacyAll},
		idMappedUIDRanges: []idRange{{start: 100000, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 100000, length: 65536}},
	}

	// Container credentials
	cred := &peerCredentials{uid: 110000, gid: 110000}

	// Should fail authorization when AllowIDMappedRoot is false
	if _, err := p.authorizePeer(cred); err == nil {
		t.Error("authorizePeer should fail for ID-mapped root when AllowIDMappedRoot is false")
	}
}

// TestMultipleIDRanges tests handling of multiple ID mapping ranges
func TestMultipleIDRanges(t *testing.T) {
	p := &Proxy{
		config: &Config{AllowIDMappedRoot: true},
		idMappedUIDRanges: []idRange{
			{start: 100000, length: 65536},
			{start: 200000, length: 65536},
		},
		idMappedGIDRanges: []idRange{
			{start: 100000, length: 65536},
			{start: 200000, length: 65536},
		},
	}

	tests := []struct {
		name       string
		uid        uint32
		gid        uint32
		isIDMapped bool
	}{
		{"First range", 110000, 110000, true},
		{"Second range", 210000, 210000, true},
		{"Between ranges", 180000, 180000, false},
		{"Below ranges", 50000, 50000, false},
		{"Above ranges", 300000, 300000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &peerCredentials{uid: tt.uid, gid: tt.gid}
			got := p.isIDMappedRoot(cred)
			if got != tt.isIDMapped {
				t.Errorf("isIDMappedRoot() = %v, want %v for uid=%d gid=%d",
					got, tt.isIDMapped, tt.uid, tt.gid)
			}
		})
	}
}
