package hostagent

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

// The -n standby guard exists to avoid spinning up sleeping rotational disks.
// SSDs have nothing to spin up, and some SATA SSDs answer CHECK POWER MODE
// with a bogus standby state that permanently hides their SMART data (#1516),
// so confirmed non-rotational devices must be probed without the guard.
func TestSmartctlArgsStandbyGuard(t *testing.T) {
	origGOOS := runtimeGOOS
	origReadFile := smartctlReadFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		smartctlReadFile = origReadFile
	})

	rotationalByDevice := map[string]string{}
	smartctlReadFile = func(path string) ([]byte, error) {
		for device, value := range rotationalByDevice {
			if path == fmt.Sprintf("/sys/block/%s/queue/rotational", device) {
				return []byte(value + "\n"), nil
			}
		}
		return nil, fs.ErrNotExist
	}

	hasStandbyGuard := func(args []string) bool {
		for i, arg := range args {
			if arg == "-n" && i+1 < len(args) && strings.HasPrefix(args[i+1], "standby,") {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name       string
		goos       string
		device     string
		deviceType string
		rotational map[string]string
		wantGuard  bool
	}{
		{
			name:       "linux ssd drops guard",
			goos:       "linux",
			device:     "/dev/sda",
			deviceType: "sat",
			rotational: map[string]string{"sda": "0"},
			wantGuard:  false,
		},
		{
			name:       "linux hdd keeps guard",
			goos:       "linux",
			device:     "/dev/sdb",
			deviceType: "sat",
			rotational: map[string]string{"sdb": "1"},
			wantGuard:  true,
		},
		{
			name:       "unreadable sysfs keeps guard",
			goos:       "linux",
			device:     "/dev/sdc",
			deviceType: "",
			rotational: map[string]string{},
			wantGuard:  true,
		},
		{
			name:       "multiplexed member keeps guard even when array is non-rotational",
			goos:       "linux",
			device:     "/dev/sda",
			deviceType: "megaraid,7",
			rotational: map[string]string{"sda": "0"},
			wantGuard:  true,
		},
		{
			name:       "non-linux keeps guard",
			goos:       "freebsd",
			device:     "/dev/ada0",
			deviceType: "sat",
			rotational: map[string]string{"ada0": "0"},
			wantGuard:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtimeGOOS = tc.goos
			rotationalByDevice = tc.rotational

			args := smartctlArgs(tc.device, tc.deviceType)
			if got := hasStandbyGuard(args); got != tc.wantGuard {
				t.Fatalf("smartctlArgs(%q, %q) standby guard = %v, want %v (args %v)",
					tc.device, tc.deviceType, got, tc.wantGuard, args)
			}
			// The probe flags and device must survive in both shapes.
			joined := strings.Join(args, " ")
			for _, required := range []string{"-i", "-A", "-H", "--json=o", tc.device} {
				if !strings.Contains(joined, required) {
					t.Fatalf("smartctlArgs(%q, %q) missing %q in %v", tc.device, tc.deviceType, required, args)
				}
			}
		})
	}
}
