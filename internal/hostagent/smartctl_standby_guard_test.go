package hostagent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestLinuxNonRotationalBlockDeviceUsesLinuxSysfsPath(t *testing.T) {
	origGOOS := runtimeGOOS
	origReadFile := smartctlReadFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		smartctlReadFile = origReadFile
	})

	runtimeGOOS = "linux"
	smartctlReadFile = func(got string) ([]byte, error) {
		const want = "/sys/block/sda/queue/rotational"
		if got != want {
			t.Fatalf("sysfs path = %q, want %q", got, want)
		}
		return []byte("0\n"), nil
	}

	if !linuxNonRotationalBlockDevice("/dev/sda") {
		t.Fatal("expected Linux /dev/sda to resolve as a confirmed non-rotational device")
	}
}

func TestIssue1653OldSmartctlRetriesWithoutJSON(t *testing.T) {
	originalRun := smartRunCommandOutput
	originalLookPath := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = originalRun
		execLookPath = originalLookPath
	})

	execLookPath = func(string) (string, error) { return "/usr/sbin/smartctl", nil }
	var calls [][]string
	smartRunCommandOutput = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls = append(calls, slices.Clone(args))
		if slices.Contains(args, "--json=o") {
			return nil, errors.New("smartctl: unrecognized option '--json=o'")
		}
		return []byte(strings.Join([]string{
			"Device Model: Synology SATA Disk",
			"Serial Number: DSM1653",
			"SMART overall-health self-assessment test result: PASSED",
			"194 Temperature_Celsius 0x0022 100 100 000 Old_age Always - 31",
		}, "\n")), nil
	}

	result, err := collectSMARTTarget(context.Background(), smartctlTarget{Path: "/dev/sata1"})
	if err != nil {
		t.Fatalf("collectSMARTTarget() error = %v", err)
	}
	if result.Serial != "DSM1653" || result.Temperature != 31 || result.Health != "PASSED" {
		t.Fatalf("legacy text result = %+v", result)
	}
	if len(calls) != 2 || !slices.Contains(calls[0], "--json=o") || slices.Contains(calls[1], "--json=o") {
		t.Fatalf("smartctl calls = %v, want JSON attempt followed by text-compatible retry", calls)
	}
}

func TestIssue1653DSMDeviceGetsSATRetry(t *testing.T) {
	originalGOOS := runtimeGOOS
	t.Cleanup(func() { runtimeGOOS = originalGOOS })
	runtimeGOOS = "linux"

	attempts := smartctlProbeAttempts(smartctlTarget{Path: "/dev/sata7"})
	if len(attempts) < 2 {
		t.Fatalf("smartctlProbeAttempts(/dev/sata7) = %v, want untyped and SAT probes", attempts)
	}
	foundSAT := false
	for _, args := range attempts {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-d" && args[i+1] == "sat" {
				foundSAT = true
			}
		}
	}
	if !foundSAT {
		t.Fatalf("smartctlProbeAttempts(/dev/sata7) = %v, want -d sat retry", attempts)
	}
}

func TestIssue1653SmartctlPathOverride(t *testing.T) {
	configuredPath := filepath.Join(t.TempDir(), "smartctl")
	t.Setenv("PULSE_SMARTCTL_PATH", configuredPath)
	originalLookPath := execLookPath
	t.Cleanup(func() { execLookPath = originalLookPath })
	execLookPath = func(string) (string, error) {
		return "", errors.New("PATH lookup must not run")
	}

	path, err := resolveSmartctlPath()
	if err != nil {
		t.Fatalf("resolveSmartctlPath() error = %v", err)
	}
	if path != configuredPath {
		t.Fatalf("resolveSmartctlPath() = %q", path)
	}
}

func TestIssue1653StandbyOSIsRecognized(t *testing.T) {
	fallback := parseSMARTTextFallback("Device is in STANDBY (OS) mode")
	if !fallback.Standby {
		t.Fatal("STANDBY (OS) text was not recognized")
	}
}

func TestIssue1653CommandErrorIncludesStderr(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is unavailable")
	}
	_, err := runCommandOutputLimited(
		context.Background(),
		1024,
		"sh",
		"-c",
		"printf 'smartctl: unrecognized option --json=o' >&2; exit 2",
	)
	if err == nil || !strings.Contains(err.Error(), "unrecognized option") {
		t.Fatalf("runCommandOutputLimited() error = %v, want captured stderr", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("wrapped error %T does not preserve exec.ExitError", err)
	}
}
