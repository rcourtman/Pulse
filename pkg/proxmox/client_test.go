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
