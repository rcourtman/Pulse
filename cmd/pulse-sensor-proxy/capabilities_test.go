package main

import (
	"testing"
)

func TestCapability_Has(t *testing.T) {
	tests := []struct {
		name     string
		cap      Capability
		flag     Capability
		expected bool
	}{
		// Single capability checks
		{
			name:     "read has read",
			cap:      CapabilityRead,
			flag:     CapabilityRead,
			expected: true,
		},
		{
			name:     "read does not have write",
			cap:      CapabilityRead,
			flag:     CapabilityWrite,
			expected: false,
		},
		{
			name:     "read does not have admin",
			cap:      CapabilityRead,
			flag:     CapabilityAdmin,
			expected: false,
		},
		{
			name:     "write has write",
			cap:      CapabilityWrite,
			flag:     CapabilityWrite,
			expected: true,
		},
		{
			name:     "admin has admin",
			cap:      CapabilityAdmin,
			flag:     CapabilityAdmin,
			expected: true,
		},

		// Combined capability checks
		{
			name:     "read+write has read",
			cap:      CapabilityRead | CapabilityWrite,
			flag:     CapabilityRead,
			expected: true,
		},
		{
			name:     "read+write has write",
			cap:      CapabilityRead | CapabilityWrite,
			flag:     CapabilityWrite,
			expected: true,
		},
		{
			name:     "read+write does not have admin",
			cap:      CapabilityRead | CapabilityWrite,
			flag:     CapabilityAdmin,
			expected: false,
		},
		{
			name:     "all capabilities has read",
			cap:      CapabilityRead | CapabilityWrite | CapabilityAdmin,
			flag:     CapabilityRead,
			expected: true,
		},
		{
			name:     "all capabilities has write",
			cap:      CapabilityRead | CapabilityWrite | CapabilityAdmin,
			flag:     CapabilityWrite,
			expected: true,
		},
		{
			name:     "all capabilities has admin",
			cap:      CapabilityRead | CapabilityWrite | CapabilityAdmin,
			flag:     CapabilityAdmin,
			expected: true,
		},

		// Zero capability
		{
			name:     "zero capability does not have read",
			cap:      0,
			flag:     CapabilityRead,
			expected: false,
		},
		{
			name:     "zero capability does not have write",
			cap:      0,
			flag:     CapabilityWrite,
			expected: false,
		},

		// Check for combined flags
		{
			name:     "read+write has read+write combined",
			cap:      CapabilityRead | CapabilityWrite,
			flag:     CapabilityRead | CapabilityWrite,
			expected: true,
		},
		{
			name:     "read only does not have read+write combined",
			cap:      CapabilityRead,
			flag:     CapabilityRead | CapabilityWrite,
			expected: false,
		},

		// Legacy all constant
		{
			name:     "legacy all has read",
			cap:      capabilityLegacyAll,
			flag:     CapabilityRead,
			expected: true,
		},
		{
			name:     "legacy all has write",
			cap:      capabilityLegacyAll,
			flag:     CapabilityWrite,
			expected: true,
		},
		{
			name:     "legacy all has admin",
			cap:      capabilityLegacyAll,
			flag:     CapabilityAdmin,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cap.Has(tt.flag)
			if got != tt.expected {
				t.Errorf("Capability(%d).Has(%d) = %v, want %v", tt.cap, tt.flag, got, tt.expected)
			}
		})
	}
}

func TestParseCapabilityList(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected Capability
	}{
		// Empty/nil input defaults to read
		{
			name:     "nil slice defaults to read",
			input:    nil,
			expected: CapabilityRead,
		},
		{
			name:     "empty slice defaults to read",
			input:    []string{},
			expected: CapabilityRead,
		},

		// Single capabilities
		{
			name:     "parse read",
			input:    []string{"read"},
			expected: CapabilityRead,
		},
		{
			name:     "parse write",
			input:    []string{"write"},
			expected: CapabilityWrite,
		},
		{
			name:     "parse admin",
			input:    []string{"admin"},
			expected: CapabilityAdmin,
		},

		// Case insensitivity
		{
			name:     "parse READ uppercase",
			input:    []string{"READ"},
			expected: CapabilityRead,
		},
		{
			name:     "parse Write mixed case",
			input:    []string{"Write"},
			expected: CapabilityWrite,
		},
		{
			name:     "parse ADMIN uppercase",
			input:    []string{"ADMIN"},
			expected: CapabilityAdmin,
		},

		// Whitespace handling
		{
			name:     "parse with leading space",
			input:    []string{" read"},
			expected: CapabilityRead,
		},
		{
			name:     "parse with trailing space",
			input:    []string{"write "},
			expected: CapabilityWrite,
		},
		{
			name:     "parse with surrounding space",
			input:    []string{" admin "},
			expected: CapabilityAdmin,
		},

		// Multiple capabilities
		{
			name:     "parse read and write",
			input:    []string{"read", "write"},
			expected: CapabilityRead | CapabilityWrite,
		},
		{
			name:     "parse read and admin",
			input:    []string{"read", "admin"},
			expected: CapabilityRead | CapabilityAdmin,
		},
		{
			name:     "parse write and admin",
			input:    []string{"write", "admin"},
			expected: CapabilityWrite | CapabilityAdmin,
		},
		{
			name:     "parse all three",
			input:    []string{"read", "write", "admin"},
			expected: CapabilityRead | CapabilityWrite | CapabilityAdmin,
		},

		// Duplicate handling (should still work - OR is idempotent)
		{
			name:     "duplicates ignored",
			input:    []string{"read", "read", "read"},
			expected: CapabilityRead,
		},
		{
			name:     "duplicates with others",
			input:    []string{"read", "write", "read"},
			expected: CapabilityRead | CapabilityWrite,
		},

		// Unknown capabilities ignored
		{
			name:     "unknown capability ignored",
			input:    []string{"unknown"},
			expected: 0,
		},
		{
			name:     "unknown with valid",
			input:    []string{"read", "unknown", "write"},
			expected: CapabilityRead | CapabilityWrite,
		},
		{
			name:     "empty string ignored",
			input:    []string{""},
			expected: 0,
		},
		{
			name:     "whitespace only ignored",
			input:    []string{"   "},
			expected: 0,
		},

		// Order independence
		{
			name:     "order admin-write-read",
			input:    []string{"admin", "write", "read"},
			expected: CapabilityRead | CapabilityWrite | CapabilityAdmin,
		},

		// Mixed case and whitespace combined
		{
			name:     "mixed case and whitespace",
			input:    []string{" READ ", "  Write", "ADMIN  "},
			expected: CapabilityRead | CapabilityWrite | CapabilityAdmin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCapabilityList(tt.input)
			if got != tt.expected {
				t.Errorf("parseCapabilityList(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCapabilityNames(t *testing.T) {
	tests := []struct {
		name     string
		cap      Capability
		expected []string
	}{
		// Single capabilities
		{
			name:     "read only",
			cap:      CapabilityRead,
			expected: []string{"read"},
		},
		{
			name:     "write only",
			cap:      CapabilityWrite,
			expected: []string{"write"},
		},
		{
			name:     "admin only",
			cap:      CapabilityAdmin,
			expected: []string{"admin"},
		},

		// Combined capabilities (order matters: read, write, admin)
		{
			name:     "read and write",
			cap:      CapabilityRead | CapabilityWrite,
			expected: []string{"read", "write"},
		},
		{
			name:     "read and admin",
			cap:      CapabilityRead | CapabilityAdmin,
			expected: []string{"read", "admin"},
		},
		{
			name:     "write and admin",
			cap:      CapabilityWrite | CapabilityAdmin,
			expected: []string{"write", "admin"},
		},
		{
			name:     "all three",
			cap:      CapabilityRead | CapabilityWrite | CapabilityAdmin,
			expected: []string{"read", "write", "admin"},
		},

		// Zero capability
		{
			name:     "zero capability returns empty slice",
			cap:      0,
			expected: []string{},
		},

		// Legacy all constant
		{
			name:     "legacy all",
			cap:      capabilityLegacyAll,
			expected: []string{"read", "write", "admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capabilityNames(tt.cap)

			if len(got) != len(tt.expected) {
				t.Errorf("capabilityNames(%d) returned %d items, want %d: got %v, want %v",
					tt.cap, len(got), len(tt.expected), got, tt.expected)
				return
			}

			for i, name := range got {
				if name != tt.expected[i] {
					t.Errorf("capabilityNames(%d)[%d] = %q, want %q",
						tt.cap, i, name, tt.expected[i])
				}
			}
		})
	}
}

// TestCapabilityRoundTrip verifies that parsing and naming are consistent
func TestCapabilityRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "single read",
			input:  []string{"read"},
			expect: []string{"read"},
		},
		{
			name:   "all capabilities",
			input:  []string{"read", "write", "admin"},
			expect: []string{"read", "write", "admin"},
		},
		{
			name:   "reverse order normalizes",
			input:  []string{"admin", "write", "read"},
			expect: []string{"read", "write", "admin"},
		},
		{
			name:   "duplicates removed",
			input:  []string{"read", "read", "write"},
			expect: []string{"read", "write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := parseCapabilityList(tt.input)
			names := capabilityNames(cap)

			if len(names) != len(tt.expect) {
				t.Errorf("round trip %v -> %d -> %v, want %v",
					tt.input, cap, names, tt.expect)
				return
			}

			for i, name := range names {
				if name != tt.expect[i] {
					t.Errorf("round trip result[%d] = %q, want %q",
						i, name, tt.expect[i])
				}
			}
		})
	}
}

// TestCapabilityConstants verifies the bit positions are correct
func TestCapabilityConstants(t *testing.T) {
	// Verify each capability is a single bit
	if CapabilityRead != 1 {
		t.Errorf("CapabilityRead = %d, want 1", CapabilityRead)
	}
	if CapabilityWrite != 2 {
		t.Errorf("CapabilityWrite = %d, want 2", CapabilityWrite)
	}
	if CapabilityAdmin != 4 {
		t.Errorf("CapabilityAdmin = %d, want 4", CapabilityAdmin)
	}

	// Verify legacy all is the combination of all three
	if capabilityLegacyAll != CapabilityRead|CapabilityWrite|CapabilityAdmin {
		t.Errorf("capabilityLegacyAll = %d, want %d",
			capabilityLegacyAll, CapabilityRead|CapabilityWrite|CapabilityAdmin)
	}
}
