package main

import "testing"

func TestAuthorizePeer(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: true},
		allowedPeerUIDs:   map[uint32]struct{}{0: {}},
		allowedPeerGIDs:   map[uint32]struct{}{0: {}},
		idMappedUIDRanges: []idRange{{start: 165536, length: 65536}},
		idMappedGIDRanges: []idRange{{start: 165536, length: 65536}},
	}

	if _, err := p.authorizePeer(&peerCredentials{uid: 0, gid: 0}); err != nil {
		t.Fatalf("expected root to be authorized, got %v", err)
	}

	if _, err := p.authorizePeer(&peerCredentials{uid: 170000, gid: 170000}); err != nil {
		t.Fatalf("expected idmapped root to be authorized, got %v", err)
	}

	if _, err := p.authorizePeer(&peerCredentials{uid: 900, gid: 900}); err == nil {
		t.Fatalf("expected non-allowed user to be rejected")
	}
}

func TestAuthorizePeerWithGIDAllowList(t *testing.T) {
	p := &Proxy{
		config:            &Config{AllowIDMappedRoot: false},
		allowedPeerUIDs:   map[uint32]struct{}{},
		allowedPeerGIDs:   map[uint32]struct{}{2000: {}},
		idMappedUIDRanges: nil,
		idMappedGIDRanges: nil,
	}

	// Should authorize because GID is explicitly allowed
	if _, err := p.authorizePeer(&peerCredentials{uid: 1500, gid: 2000}); err != nil {
		t.Fatalf("expected peer to be authorized via GID allow-list, got %v", err)
	}

	// Should reject because UID and GID are not permitted
	if _, err := p.authorizePeer(&peerCredentials{uid: 1500, gid: 3000}); err == nil {
		t.Fatalf("expected peer to be rejected when GID not in allow-list")
	}
}

func TestAuthorizePeerCapabilities(t *testing.T) {
	p := &Proxy{
		config:           &Config{},
		allowedPeerUIDs:  map[uint32]struct{}{1000: {}},
		peerCapabilities: map[uint32]Capability{1000: CapabilityRead},
	}

	caps, err := p.authorizePeer(&peerCredentials{uid: 1000, gid: 1000})
	if err != nil {
		t.Fatalf("expected authorization to succeed: %v", err)
	}
	if !caps.Has(CapabilityRead) {
		t.Fatal("expected read capability")
	}
	if caps.Has(CapabilityAdmin) {
		t.Fatal("did not expect admin capability")
	}

	// Legacy behaviour (all capabilities) when configured accordingly
	p.allowedPeerUIDs[2000] = struct{}{}
	p.peerCapabilities[2000] = capabilityLegacyAll
	caps, err = p.authorizePeer(&peerCredentials{uid: 2000, gid: 2000})
	if err != nil {
		t.Fatalf("expected legacy authorization to succeed: %v", err)
	}
	if !caps.Has(CapabilityAdmin) {
		t.Fatal("expected admin capability for legacy peer")
	}
}
