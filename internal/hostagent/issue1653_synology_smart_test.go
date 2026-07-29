package hostagent

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

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
	t.Setenv("PULSE_SMARTCTL_PATH", "/opt/syno/bin/smartctl")
	originalLookPath := execLookPath
	t.Cleanup(func() { execLookPath = originalLookPath })
	execLookPath = func(string) (string, error) {
		return "", errors.New("PATH lookup must not run")
	}

	path, err := resolveSmartctlPath()
	if err != nil {
		t.Fatalf("resolveSmartctlPath() error = %v", err)
	}
	if path != "/opt/syno/bin/smartctl" {
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
