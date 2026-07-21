package hostagent

import (
	"reflect"
	"testing"
)

// TestBranchcov0721amSmartctlTargets exercises smartctlTargetsFromDevices
// across its two branches (empty -> nil, non-empty -> ordered targets) and
// confirms exact values for each returned target, including duplicate paths.
func TestBranchcov0721amSmartctlTargets(t *testing.T) {
	t.Run("nil_input_returns_nil", func(t *testing.T) {
		got := smartctlTargetsFromDevices(nil)
		if got != nil {
			t.Fatalf("expected nil slice for nil input, got %#v (len=%d)", got, len(got))
		}
	})

	t.Run("empty_non_nil_slice_returns_nil", func(t *testing.T) {
		got := smartctlTargetsFromDevices([]string{})
		if got != nil {
			t.Fatalf("expected nil slice for empty input, got %#v (len=%d)", got, len(got))
		}
	})

	t.Run("single_device_single_target_path_set", func(t *testing.T) {
		got := smartctlTargetsFromDevices([]string{"/dev/sda"})
		expected := []smartctlTarget{{Path: "/dev/sda", DeviceType: ""}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
		if len(got) != 1 {
			t.Fatalf("expected len 1, got %d", len(got))
		}
		if got[0].Path != "/dev/sda" {
			t.Fatalf("expected Path /dev/sda, got %q", got[0].Path)
		}
		if got[0].DeviceType != "" {
			t.Fatalf("expected empty DeviceType, got %q", got[0].DeviceType)
		}
	})

	t.Run("multiple_devices_preserve_order", func(t *testing.T) {
		input := []string{"/dev/sda", "/dev/nvme0n1", "/dev/sdb"}
		got := smartctlTargetsFromDevices(input)
		expected := []smartctlTarget{
			{Path: "/dev/sda", DeviceType: ""},
			{Path: "/dev/nvme0n1", DeviceType: ""},
			{Path: "/dev/sdb", DeviceType: ""},
		}
		if len(got) != len(expected) {
			t.Fatalf("expected len %d, got %d", len(expected), len(got))
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
		for i := range expected {
			if got[i].Path != expected[i].Path {
				t.Fatalf("index %d: expected Path %q, got %q", i, expected[i].Path, got[i].Path)
			}
			if got[i].DeviceType != "" {
				t.Fatalf("index %d: expected empty DeviceType, got %q", i, got[i].DeviceType)
			}
		}
	})

	t.Run("duplicate_paths_preserved_no_dedup", func(t *testing.T) {
		input := []string{"/dev/sda", "/dev/sda", "/dev/sda"}
		got := smartctlTargetsFromDevices(input)
		expected := []smartctlTarget{
			{Path: "/dev/sda", DeviceType: ""},
			{Path: "/dev/sda", DeviceType: ""},
			{Path: "/dev/sda", DeviceType: ""},
		}
		if len(got) != 3 {
			t.Fatalf("expected len 3 (no dedup), got %d", len(got))
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
		for i, target := range got {
			if target.Path != "/dev/sda" {
				t.Fatalf("index %d: expected Path /dev/sda, got %q", i, target.Path)
			}
			if target.DeviceType != "" {
				t.Fatalf("index %d: expected empty DeviceType, got %q", i, target.DeviceType)
			}
		}
	})

	t.Run("empty_string_device_still_emits_target", func(t *testing.T) {
		// len(devices) != 0, so the function constructs a target even for an
		// empty-string path; this documents that no filtering occurs.
		got := smartctlTargetsFromDevices([]string{""})
		expected := []smartctlTarget{{Path: "", DeviceType: ""}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
	})
}
