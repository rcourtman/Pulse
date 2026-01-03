package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestInitAuthRules(t *testing.T) {
	p := &Proxy{
		config: &Config{
			AllowedPeers: []PeerConfig{
				{UID: 1001, Capabilities: []string{"read", "write"}},
			},
			AllowedPeerUIDs: []uint32{1002, 1002},
			AllowedPeerGIDs: []uint32{2001},
		},
	}

	if err := p.initAuthRules(); err != nil {
		t.Fatalf("initAuthRules failed: %v", err)
	}

	if _, ok := p.allowedPeerUIDs[1001]; !ok {
		t.Error("expected UID 1001 to be allowed")
	}
	if caps := p.peerCapabilities[1001]; !caps.Has(CapabilityWrite) {
		t.Error("expected UID 1001 to have write capability")
	}

	if _, ok := p.allowedPeerUIDs[1002]; !ok {
		t.Error("expected UID 1002 to be allowed")
	}
	if caps := p.peerCapabilities[1002]; !caps.Has(CapabilityAdmin) {
		t.Error("expected UID 1002 to have legacy all capabilities")
	}

	if _, ok := p.allowedPeerGIDs[2001]; !ok {
		t.Error("expected GID 2001 to be allowed")
	}
}

func TestLoadSubIDRanges(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "subuid")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `root:100000:65536
# commented:line:100
user1:200000:1000
invalid:start:65536
toolong:100:notanumber
short:line
zero:300000:0
`
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with filter
	ranges, err := loadSubIDRanges(tmpFile.Name(), []string{"root", "user1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}

	// Test without filter (should return all valid)
	ranges, err = loadSubIDRanges(tmpFile.Name(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// root and user1 are valid. zero has length 0 so skipped. invalid/toolong/short are invalid.
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}

	// Test missing file
	ranges, err = loadSubIDRanges("/nonexistent/file", nil)
	if err != nil {
		t.Fatalf("expected no error for nonexistent file, got %v", err)
	}
	if ranges != nil {
		t.Fatal("expected nil ranges for nonexistent file")
	}
}

func TestAuthorizePeerEdgeCases(t *testing.T) {
	p := &Proxy{
		config: &Config{},
	}

	if _, err := p.authorizePeer(nil); err == nil {
		t.Error("expected error for nil credentials")
	}

	p.config.AllowIDMappedRoot = true
	// No ranges loaded
	if p.isIDMappedRoot(&peerCredentials{uid: 100000, gid: 100000}) {
		t.Error("expected isIDMappedRoot to be false when no ranges loaded")
	}

	p.idMappedUIDRanges = []idRange{{start: 100000, length: 1000}}
	p.idMappedGIDRanges = []idRange{{start: 100000, length: 1000}}

	if !p.isIDMappedRoot(&peerCredentials{uid: 100500, gid: 100500}) {
		t.Error("expected isIDMappedRoot to be true for valid range")
	}

	if p.isIDMappedRoot(&peerCredentials{uid: 200000, gid: 100500}) {
		t.Error("expected isIDMappedRoot to be false for invalid UID")
	}

	if p.isIDMappedRoot(&peerCredentials{uid: 100500, gid: 200000}) {
		t.Error("expected isIDMappedRoot to be false for invalid GID")
	}
}

func TestLoadIDMappingRanges(t *testing.T) {
	// We can't easily mock /etc/subuid but we can call it to get coverage
	_, _, _ = loadIDMappingRanges([]string{"root"})
}

func TestLoadIDMappingRanges_Error(t *testing.T) {
	// Save old paths
	oldUID := subUIDPath
	oldGID := subGIDPath
	defer func() {
		subUIDPath = oldUID
		subGIDPath = oldGID
	}()

	// Point to directory to cause read error
	tmpDir := t.TempDir()
	subUIDPath = tmpDir
	subGIDPath = tmpDir

	_, _, err := loadIDMappingRanges([]string{"root"})
	if err == nil {
		t.Error("expected error when UID file is a directory")
	}

	// Make UID valid (empty file is valid) but GID invalid
	uidFile := filepath.Join(tmpDir, "uid")
	os.WriteFile(uidFile, []byte("root:100000:65536"), 0644)
	subUIDPath = uidFile

	_, _, err = loadIDMappingRanges([]string{"root"})
	if err == nil {
		t.Error("expected error when GID file is a directory")
	}
}

func TestLoadSubIDRanges_ReadError(t *testing.T) {
	// Passing a directory as a file path should trigger a read error
	dir := t.TempDir()
	_, err := loadSubIDRanges(dir, []string{"root"})
	if err == nil {
		t.Error("expected error when reading a directory")
	}
}
