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

func TestDedupeUint32(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint32
		expected []uint32
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []uint32{},
			expected: nil,
		},
		{
			name:     "no duplicates",
			input:    []uint32{1, 2, 3},
			expected: []uint32{1, 2, 3},
		},
		{
			name:     "all duplicates",
			input:    []uint32{5, 5, 5, 5},
			expected: []uint32{5},
		},
		{
			name:     "mixed duplicates",
			input:    []uint32{1, 2, 1, 3, 2, 4},
			expected: []uint32{1, 2, 3, 4},
		},
		{
			name:     "single element",
			input:    []uint32{42},
			expected: []uint32{42},
		},
		{
			name:     "preserves order",
			input:    []uint32{3, 1, 4, 1, 5, 9, 2, 6, 5, 3},
			expected: []uint32{3, 1, 4, 5, 9, 2, 6},
		},
		{
			name:     "zero values",
			input:    []uint32{0, 1, 0, 2},
			expected: []uint32{0, 1, 2},
		},
		{
			name:     "max uint32",
			input:    []uint32{4294967295, 0, 4294967295},
			expected: []uint32{4294967295, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := dedupeUint32(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("dedupeUint32(%v) = %v (len %d), want %v (len %d)",
					tc.input, result, len(result), tc.expected, len(tc.expected))
				return
			}

			for i := range result {
				if result[i] != tc.expected[i] {
					t.Errorf("dedupeUint32(%v) = %v, want %v", tc.input, result, tc.expected)
					return
				}
			}
		})
	}
}

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all duplicates",
			input:    []string{"foo", "foo", "foo"},
			expected: []string{"foo"},
		},
		{
			name:     "mixed duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "single element",
			input:    []string{"single"},
			expected: []string{"single"},
		},
		{
			name:     "preserves order",
			input:    []string{"c", "a", "b", "a", "c"},
			expected: []string{"c", "a", "b"},
		},
		{
			name:     "trims whitespace",
			input:    []string{"  foo  ", "bar", "foo"},
			expected: []string{"foo", "bar"},
		},
		{
			name:     "filters empty after trim",
			input:    []string{"a", "   ", "", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "whitespace only",
			input:    []string{"  ", "\t", "\n"},
			expected: nil,
		},
		{
			name:     "case sensitive",
			input:    []string{"Foo", "foo", "FOO"},
			expected: []string{"Foo", "foo", "FOO"},
		},
		{
			name:     "leading and trailing whitespace dedup",
			input:    []string{"  test", "test  ", "test"},
			expected: []string{"test"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := dedupeStrings(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("dedupeStrings(%v) = %v (len %d), want %v (len %d)",
					tc.input, result, len(result), tc.expected, len(tc.expected))
				return
			}

			for i := range result {
				if result[i] != tc.expected[i] {
					t.Errorf("dedupeStrings(%v) = %v, want %v", tc.input, result, tc.expected)
					return
				}
			}
		})
	}
}
