package proxmox

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDiskUnmarshalWearout(t *testing.T) {
	tests := []struct {
		name         string
		wearout      json.RawMessage
		expectedLife int
		expectedUsed int
	}{
		{
			name:         "numeric",
			wearout:      json.RawMessage(`81`),
			expectedLife: 19,
			expectedUsed: 81,
		},
		{
			name:         "numeric string",
			wearout:      json.RawMessage(`"81"`),
			expectedLife: 19,
			expectedUsed: 81,
		},
		{
			name:         "escaped numeric string",
			wearout:      json.RawMessage(`"\"81\""`),
			expectedLife: 19,
			expectedUsed: 81,
		},
		{
			name:         "percentage string",
			wearout:      json.RawMessage(`"81%"`),
			expectedLife: 19,
			expectedUsed: 81,
		},
		{
			name:         "percentage with spaces",
			wearout:      json.RawMessage(`"  82 % "`),
			expectedLife: 18,
			expectedUsed: 82,
		},
		{
			name:         "not applicable string",
			wearout:      json.RawMessage(`"N/A"`),
			expectedLife: 100,
			expectedUsed: 0,
		},
		{
			name:         "empty string",
			wearout:      json.RawMessage(`""`),
			expectedLife: 100,
			expectedUsed: 0,
		},
		{
			name:         "null value",
			wearout:      json.RawMessage(`null`),
			expectedLife: 100,
			expectedUsed: 0,
		},
		{
			name:         "unknown string",
			wearout:      json.RawMessage(`"Unknown"`),
			expectedLife: 100,
			expectedUsed: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"devpath":"/dev/sda","model":"Example","serial":"123","type":"hdd","health":"OK","wearout":%s,"size":1000,"rpm":7200,"used":"LVM","vendor":"Example","wwn":"example"}`, tc.wearout)
			var disk Disk
			if err := json.Unmarshal([]byte(payload), &disk); err != nil {
				t.Fatalf("unexpected error unmarshalling disk: %v", err)
			}
			if disk.Wearout != tc.expectedLife {
				t.Fatalf("life remaining: got %d, want %d", disk.Wearout, tc.expectedLife)
			}
			if disk.WearoutUsed != tc.expectedUsed {
				t.Fatalf("wear consumed: got %d, want %d", disk.WearoutUsed, tc.expectedUsed)
			}
		})
	}
}
