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

	if err := p.authorizePeer(&peerCredentials{uid: 0, gid: 0}); err != nil {
		t.Fatalf("expected root to be authorized, got %v", err)
	}

	if err := p.authorizePeer(&peerCredentials{uid: 170000, gid: 170000}); err != nil {
		t.Fatalf("expected idmapped root to be authorized, got %v", err)
	}

	if err := p.authorizePeer(&peerCredentials{uid: 900, gid: 900}); err == nil {
		t.Fatalf("expected non-allowed user to be rejected")
	}
}
