package proxmox

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDiskUnmarshalWearout(t *testing.T) {
	tests := []struct {
		name     string
		wearout  json.RawMessage
		expected int
	}{
		{
			name:     "numeric",
			wearout:  json.RawMessage(`81`),
			expected: 81,
		},
		{
			name:     "numeric string",
			wearout:  json.RawMessage(`"81"`),
			expected: 81,
		},
		{
			name:     "escaped numeric string",
			wearout:  json.RawMessage(`"\"81\""`),
			expected: 81,
		},
		{
			name:     "percentage string",
			wearout:  json.RawMessage(`"81%"`),
			expected: 81,
		},
		{
			name:     "percentage with spaces",
			wearout:  json.RawMessage(`"  82 % "`),
			expected: 82,
		},
		{
			name:     "not applicable string",
			wearout:  json.RawMessage(`"N/A"`),
			expected: wearoutUnknown,
		},
		{
			name:     "empty string",
			wearout:  json.RawMessage(`""`),
			expected: wearoutUnknown,
		},
		{
			name:     "null value",
			wearout:  json.RawMessage(`null`),
			expected: wearoutUnknown,
		},
		{
			name:     "unknown string",
			wearout:  json.RawMessage(`"Unknown"`),
			expected: wearoutUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"devpath":"/dev/sda","model":"Example","serial":"123","type":"hdd","health":"OK","wearout":%s,"size":1000,"rpm":7200,"used":"LVM","vendor":"Example","wwn":"example"}`, tc.wearout)
			var disk Disk
			if err := json.Unmarshal([]byte(payload), &disk); err != nil {
				t.Fatalf("unexpected error unmarshalling disk: %v", err)
			}
			if disk.Wearout != tc.expected {
				t.Fatalf("wearout: got %d, want %d", disk.Wearout, tc.expected)
			}
		})
	}
}

func TestMemoryStatusEffectiveAvailable(t *testing.T) {
	t.Run("nil receiver returns zero", func(t *testing.T) {
		var status *MemoryStatus
		if status.EffectiveAvailable() != 0 {
			t.Fatalf("expected nil receiver to return 0")
		}
	})

	tests := []struct {
		name   string
		status MemoryStatus
		want   uint64
	}{
		{
			name:   "uses available field when set",
			status: MemoryStatus{Total: 16 * 1024, Available: 6 * 1024},
			want:   6 * 1024,
		},
		{
			name:   "uses avail field fallback",
			status: MemoryStatus{Total: 16 * 1024, Avail: 5 * 1024},
			want:   5 * 1024,
		},
		{
			name:   "derives from free buffers cached",
			status: MemoryStatus{Total: 32 * 1024, Free: 4 * 1024, Buffers: 2 * 1024, Cached: 6 * 1024},
			want:   12 * 1024,
		},
		{
			name:   "caps derived value at total",
			status: MemoryStatus{Total: 8 * 1024, Free: 4 * 1024, Buffers: 4 * 1024, Cached: 4 * 1024},
			want:   8 * 1024,
		},
		{
			name:   "returns zero when no data",
			status: MemoryStatus{Total: 24 * 1024},
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.EffectiveAvailable(); got != tc.want {
				t.Fatalf("EffectiveAvailable: got %d, want %d", got, tc.want)
			}
		})
	}
}

// TestMemoryStatusEffectiveAvailable_RegressionIssue435 tests the specific scenarios
// reported in GitHub issue #435 where memory calculations incorrectly included cache/buffers
func TestMemoryStatusEffectiveAvailable_RegressionIssue435(t *testing.T) {
	tests := []struct {
		name          string
		status        MemoryStatus
		wantAvailable uint64
		wantUsedPct   float64 // Expected usage percentage when using EffectiveAvailable
		description   string
	}{
		{
			name: "real proxmox 8.x node with available field",
			status: MemoryStatus{
				Total:     16466186240, // 16GB
				Used:      13981696000, // ~14GB (includes cache)
				Free:      1558323200,  // ~1.5GB
				Available: 7422545920,  // ~7GB (cache-aware - this is what we should use!)
				Buffers:   23592960,    // ~23MB
				Cached:    5478891520,  // ~5.4GB
			},
			wantAvailable: 7422545920,
			wantUsedPct:   54.92, // Should be ~55%, not ~85%!
			description:   "Proxmox 8.x returns 'available' field - most accurate",
		},
		{
			name: "older proxmox with avail field",
			status: MemoryStatus{
				Total: 8589934592, // 8GB
				Used:  6871947674, // ~6.4GB (includes cache)
				Free:  805306368,  // ~768MB
				Avail: 3221225472, // ~3GB (cache-aware)
			},
			wantAvailable: 3221225472,
			wantUsedPct:   62.5, // Should be ~62.5%, not ~80%
			description:   "Older Proxmox uses 'avail' field as fallback",
		},
		{
			name: "proxmox without available/avail - derive from components",
			status: MemoryStatus{
				Total:   16777216000, // ~16GB
				Used:    13421772800, // ~12.5GB (includes cache)
				Free:    2147483648,  // 2GB
				Buffers: 536870912,   // 512MB
				Cached:  4294967296,  // 4GB
			},
			wantAvailable: 6979321856, // Free + Buffers + Cached = ~6.5GB
			wantUsedPct:   58.4,        // Should be ~58%, not ~80%
			description:   "When available/avail missing, derive from free+buffers+cached",
		},
		{
			name: "issue #435 specific case - 86% vs 42% real usage",
			status: MemoryStatus{
				Total:     33554432000, // ~32GB
				Used:      28857589760, // ~27GB (includes cache - WRONG!)
				Free:      1073741824,  // 1GB
				Available: 19327352832, // ~18GB (cache-aware - CORRECT!)
			},
			wantAvailable: 19327352832,
			wantUsedPct:   42.4, // Should be ~42%, not 86%!
			description:   "Real user report: 86% shown when actual usage is 42%",
		},
		{
			name: "missing all cache-aware fields - fallback to zero",
			status: MemoryStatus{
				Total: 8589934592,
				Used:  6871947674,
				Free:  0, // All fields missing
			},
			wantAvailable: 0,
			wantUsedPct:   80.0, // Falls back to cache-inclusive calculation
			description:   "When all cache fields missing, EffectiveAvailable returns 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.status.EffectiveAvailable()
			if got != tc.wantAvailable {
				t.Errorf("EffectiveAvailable() = %d, want %d\nScenario: %s",
					got, tc.wantAvailable, tc.description)
			}

			// Calculate what the usage percentage would be with cache-aware calculation
			if tc.wantAvailable > 0 && tc.wantAvailable <= tc.status.Total {
				actualUsed := tc.status.Total - tc.wantAvailable
				usagePct := (float64(actualUsed) / float64(tc.status.Total)) * 100

				// Allow 0.5% tolerance for floating point differences
				if usagePct < tc.wantUsedPct-0.5 || usagePct > tc.wantUsedPct+0.5 {
					t.Errorf("Calculated usage = %.2f%%, want ~%.2f%%\nScenario: %s",
						usagePct, tc.wantUsedPct, tc.description)
				}
			}
		})
	}
}
