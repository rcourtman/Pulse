package hostagent

// Regression tests for #1483: on a Proxmox VE node the agent reported NVMe
// disks keyed by controller with size 0 and dropped a SATA SSD entirely.
// Fixture strings are captured from real PVE 9.1.9 hosts (smartctl 7.4) and
// from the issue report itself.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

// scanOpenPVENVMeOnly reproduces the #1483 discovery failure: smartctl
// --scan-open lists the two NVMe controllers but omits the SATA disk it could
// not open at scan time (open failures are only emitted as # comments).
// The NVMe lines are verbatim `smartctl --scan-open` output from a PVE 9.1.9
// host (delly, 2026-06-10).
const scanOpenPVENVMeOnly = `/dev/nvme0 -d nvme # /dev/nvme0, NVMe device
/dev/nvme1 -d nvme # /dev/nvme1, NVMe device
`

// scanOpenPVESATA is verbatim `smartctl --scan-open` output from a PVE 9.1.9
// host with a single SATA SSD (minipc, 2026-06-10).
const scanOpenPVESATA = `/dev/sda -d sat # /dev/sda [SAT], ATA device
`

// smartctlSATAProbeJSON carries the key fields of a real
// `smartctl -d sat -n standby,3 -i -A -H --json=o /dev/sda` run on a PVE
// 9.1.9 host (minipc, 2026-06-10).
const smartctlSATAProbeJSON = `{
	"device": {"name": "/dev/sda", "info_name": "/dev/sda [SAT]", "type": "sat", "protocol": "ATA"},
	"model_name": "N900-512",
	"serial_number": "AA000000000000000588",
	"user_capacity": {"blocks": 1000215216, "bytes": 512110190592},
	"smart_status": {"passed": true},
	"temperature": {"current": 40}
}`

// smartctlNVMeProbeJSON mirrors the reporter's KINGSTON SNV3S2000G output
// (issue #1483); smartctl reports NVMe controllers by their char device.
const smartctlNVMeProbeJSON = `{
	"device": {"name": "/dev/nvme0", "type": "nvme", "protocol": "NVMe"},
	"model_name": "KINGSTON SNV3S2000G",
	"serial_number": "50026B7282A0FB69",
	"nvme_total_capacity": 2000398934016,
	"smart_status": {"passed": true},
	"temperature": {"current": 37}
}`

// smartctlNoDataJSON is a syntactically valid smartctl JSON envelope with no
// usable SMART payload, as produced when a probe addresses a device with the
// wrong -d type.
const smartctlNoDataJSON = `{
	"device": {"name": "/dev/sda", "type": "sat", "protocol": "ATA"}
}`

func stubLinuxSysfs(t *testing.T, entries []string, files map[string]string) {
	t.Helper()

	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	origNow := timeNow
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
		timeNow = origNow
	})

	runtimeGOOS = "linux"
	timeNow = func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) }
	readDir = func(string) ([]os.DirEntry, error) {
		dirEntries := make([]os.DirEntry, 0, len(entries))
		for _, name := range entries {
			dirEntries = append(dirEntries, fakeDirEntry{name: name})
		}
		return dirEntries, nil
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return []byte(content), nil
		}
		return nil, fs.ErrNotExist
	}
	smartctlEvalSymlinks = func(string) (string, error) { return "", fs.ErrNotExist }
}

func TestParseSmartctlScanOpenTargetsRealPVEOutput(t *testing.T) {
	targets := parseSmartctlScanOpenTargets([]byte(scanOpenPVENVMeOnly+scanOpenPVESATA), nil)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %#v", targets)
	}
	if targets[0].Path != "/dev/nvme0" || targets[0].DeviceType != "nvme" {
		t.Errorf("unexpected first target: %#v", targets[0])
	}
	if targets[2].Path != "/dev/sda" || targets[2].DeviceType != "sat" {
		t.Errorf("unexpected SATA target: %#v", targets[2])
	}
}

func TestUnionSMARTTargetsAddsDevicesMissingFromScanOpen(t *testing.T) {
	stubLinuxSysfs(t, []string{"nvme0n1", "nvme1n1", "sda"}, nil)

	scanTargets := []smartctlTarget{
		{Path: "/dev/nvme0", DeviceType: "nvme"},
		{Path: "/dev/nvme1", DeviceType: "nvme"},
	}
	devices := []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/sda"}

	targets := unionSMARTTargets(scanTargets, devices)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %#v", targets)
	}
	if targets[2].Path != "/dev/sda" || targets[2].DeviceType != "" {
		t.Errorf("expected untyped /dev/sda appended, got %#v", targets[2])
	}

	// A device already covered by a scan target must not be duplicated.
	scanTargets = append(scanTargets, smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
	targets = unionSMARTTargets(scanTargets, devices)
	if len(targets) != 3 {
		t.Fatalf("expected no duplicate for covered devices, got %#v", targets)
	}
}

// TestCollectSMARTLocalIncludesSATADiskOmittedByScanOpen reproduces the #1483
// host: two NVMe controllers in the scan-open output, the SATA SSD missing
// from it, and all three disks present in /sys/block. The SATA disk must
// still be discovered, probed untyped, and reported.
func TestCollectSMARTLocalIncludesSATADiskOmittedByScanOpen(t *testing.T) {
	stubLinuxSysfs(t,
		[]string{"loop0", "nvme0n1", "nvme1n1", "sda", "zd0"},
		map[string]string{
			"/sys/block/nvme0n1/size": "3907029168\n", // 2000398934016 bytes
			"/sys/block/nvme1n1/size": "3907029168\n",
			"/sys/block/sda/size":     "468862128\n", // 240057409536 bytes
		},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var probed []string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte(scanOpenPVENVMeOnly), nil
		}
		device := args[len(args)-1]
		probed = append(probed, strings.Join(args, " "))
		switch device {
		case "/dev/nvme0", "/dev/nvme1":
			return []byte(smartctlNVMeProbeJSON), nil
		case "/dev/sda":
			if args[0] == "-d" {
				t.Errorf("expected untyped probe for scan-omitted /dev/sda, got args %v", args)
			}
			return []byte(smartctlSATAProbeJSON), nil
		default:
			return nil, errors.New("unexpected device " + device)
		}
	}

	results, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 disks (2 NVMe + omitted SATA), got %#v", results)
	}

	byDevice := make(map[string]DiskSMART, len(results))
	for _, disk := range results {
		byDevice[disk.Device] = disk
	}
	for _, namespace := range []string{"nvme0n1", "nvme1n1"} {
		disk, ok := byDevice[namespace]
		if !ok {
			t.Fatalf("missing namespace-keyed NVMe disk %q in %#v", namespace, results)
		}
		if disk.SizeBytes != 2000398934016 {
			t.Errorf("%s SizeBytes = %d, want 2000398934016", namespace, disk.SizeBytes)
		}
	}
	sata, ok := byDevice["sda"]
	if !ok {
		t.Fatalf("SATA disk missing from results: %#v", results)
	}
	if sata.Model != "N900-512" || sata.Serial != "AA000000000000000588" {
		t.Errorf("unexpected SATA identity: %#v", sata)
	}
	if sata.SizeBytes != 240057409536 {
		t.Errorf("SATA SizeBytes = %d, want 240057409536 (from /sys/block)", sata.SizeBytes)
	}
	if sata.Health != "PASSED" {
		t.Errorf("SATA health = %q, want PASSED", sata.Health)
	}
}

func TestCollectSMARTTargetRetriesUntypedAfterTypedProbeFailure(t *testing.T) {
	cases := map[string]func(args []string) ([]byte, error){
		"typed probe errors": func(args []string) ([]byte, error) {
			return nil, errors.New("smartctl: unsupported field in scsi command")
		},
		"typed probe returns no usable data": func(args []string) ([]byte, error) {
			return []byte(smartctlNoDataJSON), nil
		},
	}

	for name, typedResponse := range cases {
		t.Run(name, func(t *testing.T) {
			stubLinuxSysfs(t, []string{"sda"}, nil)

			origRun := smartRunCommandOutput
			origLook := execLookPath
			t.Cleanup(func() {
				smartRunCommandOutput = origRun
				execLookPath = origLook
			})
			execLookPath = func(string) (string, error) { return "smartctl", nil }

			var attempts [][]string
			smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				attempts = append(attempts, append([]string(nil), args...))
				if args[0] == "-d" {
					return typedResponse(args)
				}
				return []byte(smartctlSATAProbeJSON), nil
			}

			result, err := collectSMARTTarget(context.Background(), smartctlTarget{Path: "/dev/sda", DeviceType: "sat"})
			if err != nil {
				t.Fatalf("collectSMARTTarget error: %v", err)
			}
			if len(attempts) != 2 {
				t.Fatalf("expected typed attempt then untyped retry, got %v", attempts)
			}
			if attempts[0][0] != "-d" || attempts[0][1] != "sat" {
				t.Errorf("first attempt should be typed, got %v", attempts[0])
			}
			if attempts[1][0] == "-d" {
				t.Errorf("retry should be untyped, got %v", attempts[1])
			}
			if result.Model != "N900-512" || result.SizeBytes != 512110190592 {
				t.Errorf("unexpected retry result: %#v", result)
			}
		})
	}
}

func TestCollectSMARTTargetKeepsSingleTypedAttemptForMultiplexed(t *testing.T) {
	stubLinuxSysfs(t, []string{"sda"}, nil)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var attempts int
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		return []byte(smartctlNoDataJSON), nil
	}

	if _, err := collectSMARTTarget(context.Background(), smartctlTarget{Path: "/dev/sda", DeviceType: "megaraid,7"}); !errors.Is(err, errSMARTDataUnavailable) {
		t.Fatalf("expected errSMARTDataUnavailable, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("multiplexed member must not retry untyped (would probe the array), got %d attempts", attempts)
	}
}

// TestCollectSMARTLocalEmitsIdentityOnlyEntryWhenSMARTUnavailable covers the
// last line of defence: every probe failed, but the disk demonstrably exists
// in /sys/block, so it is reported with its provable identity instead of
// silently disappearing from the UI.
func TestCollectSMARTLocalEmitsIdentityOnlyEntryWhenSMARTUnavailable(t *testing.T) {
	stubLinuxSysfs(t,
		[]string{"sda"},
		map[string]string{
			"/sys/block/sda/size":         "1000215216\n", // 512110190592 bytes
			"/sys/block/sda/device/model": "N900-512        \n",
		},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte(scanOpenPVESATA), nil
		}
		return []byte(smartctlNoDataJSON), nil
	}

	results, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected identity-only entry, got %#v", results)
	}
	disk := results[0]
	if disk.Device != "sda" || disk.Model != "N900-512" || disk.Type != "sata" {
		t.Errorf("unexpected identity: %#v", disk)
	}
	if disk.SizeBytes != 512110190592 {
		t.Errorf("SizeBytes = %d, want 512110190592", disk.SizeBytes)
	}
	if disk.Health != "UNKNOWN" {
		t.Errorf("Health = %q, want UNKNOWN", disk.Health)
	}
	if disk.Temperature != 0 || disk.Attributes != nil {
		t.Errorf("identity-only entry must not fabricate SMART data: %#v", disk)
	}
}

func TestCollectSMARTLocalSkipsIdentityOnlyEntries(t *testing.T) {
	t.Run("multiplexed controller path", func(t *testing.T) {
		stubLinuxSysfs(t,
			[]string{"sda"},
			map[string]string{"/sys/block/sda/size": "1000215216\n"},
		)

		origRun := smartRunCommandOutput
		origLook := execLookPath
		t.Cleanup(func() {
			smartRunCommandOutput = origRun
			execLookPath = origLook
		})
		execLookPath = func(string) (string, error) { return "smartctl", nil }

		smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) == 1 && args[0] == "--scan-open" {
				return []byte("/dev/sda -d megaraid,0 # slot 0\n/dev/sda -d megaraid,1 # slot 1\n"), nil
			}
			return []byte(smartctlNoDataJSON), nil
		}

		results, err := CollectSMARTLocal(context.Background(), nil)
		if err != nil {
			t.Fatalf("CollectSMARTLocal error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("array path behind a multiplexing controller must not gain an identity row, got %#v", results)
		}
	})

	t.Run("device without capacity", func(t *testing.T) {
		stubLinuxSysfs(t, []string{"sda"}, map[string]string{"/sys/block/sda/size": "0\n"})

		origRun := smartRunCommandOutput
		origLook := execLookPath
		t.Cleanup(func() {
			smartRunCommandOutput = origRun
			execLookPath = origLook
		})
		execLookPath = func(string) (string, error) { return "smartctl", nil }

		smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) == 1 && args[0] == "--scan-open" {
				return []byte(scanOpenPVESATA), nil
			}
			return []byte(smartctlNoDataJSON), nil
		}

		results, err := CollectSMARTLocal(context.Background(), nil)
		if err != nil {
			t.Fatalf("CollectSMARTLocal error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("zero-capacity device must not gain an identity row, got %#v", results)
		}
	})
}

// TestCollectSMARTLocalAppliesExcludeToCanonicalNamespaceName guards the
// post-refine exclusion check: the scan target is the controller (nvme0), but
// the user excludes the canonical namespace name they see in the UI.
func TestCollectSMARTLocalAppliesExcludeToCanonicalNamespaceName(t *testing.T) {
	stubLinuxSysfs(t,
		[]string{"nvme0n1"},
		map[string]string{"/sys/block/nvme0n1/size": "3907029168\n"},
	)

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte("/dev/nvme0 -d nvme # /dev/nvme0, NVMe device\n"), nil
		}
		return []byte(smartctlNVMeProbeJSON), nil
	}

	results, err := CollectSMARTLocal(context.Background(), []string{"nvme0n1"})
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("excluded canonical namespace must not be reported, got %#v", results)
	}
}
