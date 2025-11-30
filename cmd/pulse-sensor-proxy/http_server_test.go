package main

import (
	"testing"
)

func TestHashIPToUID(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantMin  uint32
		wantMax  uint32
		wantSame bool // if true, verify determinism by checking same IP gives same result
	}{
		{
			name:     "IPv4 localhost",
			ip:       "127.0.0.1",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "IPv4 standard",
			ip:       "192.168.1.100",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "IPv4 another address",
			ip:       "10.0.0.1",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "IPv6 localhost",
			ip:       "::1",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "IPv6 full address",
			ip:       "2001:db8::1",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "empty string",
			ip:       "",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "single character",
			ip:       "a",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
		{
			name:     "long string",
			ip:       "this-is-a-very-long-hostname-that-might-be-used.example.com",
			wantMin:  100000,
			wantMax:  999999,
			wantSame: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hashIPToUID(tc.ip)

			// Check range
			if result < tc.wantMin || result > tc.wantMax {
				t.Errorf("hashIPToUID(%q) = %d, want in range [%d, %d]",
					tc.ip, result, tc.wantMin, tc.wantMax)
			}

			// Check determinism
			if tc.wantSame {
				result2 := hashIPToUID(tc.ip)
				if result != result2 {
					t.Errorf("hashIPToUID(%q) not deterministic: got %d then %d",
						tc.ip, result, result2)
				}
			}
		})
	}
}

func TestHashIPToUID_DifferentInputsProduceDifferentHashes(t *testing.T) {
	ips := []string{
		"127.0.0.1",
		"192.168.1.1",
		"192.168.1.2",
		"10.0.0.1",
		"::1",
		"2001:db8::1",
	}

	hashes := make(map[uint32]string)
	collisions := 0

	for _, ip := range ips {
		hash := hashIPToUID(ip)
		if existing, found := hashes[hash]; found {
			// Collision found - not necessarily an error but worth noting
			collisions++
			t.Logf("Hash collision: %q and %q both produce %d", ip, existing, hash)
		}
		hashes[hash] = ip
	}

	// With only 6 inputs and 900000 possible outputs, collisions should be rare
	if collisions > 1 {
		t.Errorf("Too many collisions (%d) for %d inputs", collisions, len(ips))
	}
}

func TestHashIPToUID_BoundaryValues(t *testing.T) {
	// Test that the function correctly produces values in the expected range
	// even for edge cases

	tests := []string{
		"",             // empty
		"\x00",         // null byte
		"\xff\xff\xff", // high bytes
		"0.0.0.0",
		"255.255.255.255",
	}

	for _, ip := range tests {
		result := hashIPToUID(ip)
		if result < 100000 || result > 999999 {
			t.Errorf("hashIPToUID(%q) = %d, out of expected range [100000, 999999]",
				ip, result)
		}
	}
}
