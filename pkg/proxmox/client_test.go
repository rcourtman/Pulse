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
