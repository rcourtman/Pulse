package proxmox

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDiskUnmarshalWearout(t *testing.T) {
	tests := []struct {
		name     string
		wearout  string
		expected int
	}{
		{
			name:     "numeric",
			wearout:  "81",
			expected: 81,
		},
		{
			name:     "numeric string",
			wearout:  "\"81\"",
			expected: 81,
		},
		{
			name:     "not applicable string",
			wearout:  "\"N/A\"",
			expected: 0,
		},
		{
			name:     "empty string",
			wearout:  "\"\"",
			expected: 0,
		},
		{
			name:     "null value",
			wearout:  "null",
			expected: 0,
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
