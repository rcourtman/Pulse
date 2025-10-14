package api

import "testing"

func TestValidateAlertID(t *testing.T) {
	testCases := []struct {
		name  string
		id    string
		valid bool
	}{
		{name: "basic", id: "guest-powered-off-pve-101", valid: true},
		{name: "with spaces", id: "cluster one-node-101-cpu", valid: true},
		{name: "with slash and colon", id: "pve1:qemu/101-cpu", valid: true},
		{name: "empty", id: "", valid: false},
		{name: "too long", id: string(make([]byte, 501)), valid: false},
		{name: "control char", id: "bad\nvalue", valid: false},
		{name: "path traversal", id: "../etc/passwd", valid: false},
		{name: "path traversal middle", id: "pve/../secret", valid: false},
	}

	// Populate the oversized id string once to avoid zero bytes being mistaken for a valid character set.
	for i := range testCases {
		if testCases[i].name == "too long" {
			value := make([]byte, 501)
			for j := range value {
				value[j] = 'a'
			}
			testCases[i].id = string(value)
		}
	}

	for _, tc := range testCases {
		if got := validateAlertID(tc.id); got != tc.valid {
			t.Errorf("validateAlertID(%s) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}
