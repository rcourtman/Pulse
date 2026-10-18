package installtests

import "testing"

func TestNASControlInventoryRequiresEveryControlExactlyOnce(t *testing.T) {
	want := []string{"test_watchdog", "test_boot"}
	for _, tc := range []struct {
		name   string
		output string
		valid  bool
	}{
		{"complete", `["test_boot", "test_watchdog"]`, true},
		{"missing", `["test_boot"]`, false},
		{"duplicate", `["test_boot", "test_boot", "test_watchdog"]`, false},
		{"substituted", `["test_boot", "test_other"]`, false},
		{"unsorted", `["test_watchdog", "test_boot"]`, false},
		{"empty", `[]`, false},
		{"null", `null`, false},
		{"wrong type", `{"test_boot": true}`, false},
		{"malformed", `["test_boot"`, false},
		{"extra output", "[\"test_boot\", \"test_watchdog\"]\nunobserved", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateNASControlInventory(want, []byte(tc.output)); (err == nil) != tc.valid {
				t.Fatalf("valid=%t, got %v", tc.valid, err)
			}
		})
	}
	if want[0] != "test_watchdog" {
		t.Fatal("inventory validation changed execution order")
	}
}
