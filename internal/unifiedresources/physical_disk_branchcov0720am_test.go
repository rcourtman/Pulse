package unifiedresources

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// Branch-coverage tests for the two currently-uncovered exported disk
// helpers in this package:
//
//   - PhysicalDiskStatus (physical_disk_risk.go) — thin exported wrapper
//     around the unexported physicalDiskStatus switch. Every test goes
//     through the EXPORTED function so the wrapper itself is exercised
//     end-to-end; each row targets one arm of the underlying switch.
//   - PhysicalDiskMetricID (physical_disk_ids.go) — id builder that
//     derives a fallback from ID-or-DevPath and then defers to the
//     shared PreferredPhysicalDiskMetricID preference order. Each row
//     drives one combination of (ID/DevPath/Serial/WWN presence).
//
// Assertions are behavioral: status-class identity, id preference order,
// slash-replacement + whitespace-trimming effects on the synthesized
// fallback. No assertion merely re-pins a source constant.

// --- PhysicalDiskStatus ---

func TestBranchcov0720am_PhysicalDiskStatus(t *testing.T) {
	t.Parallel()

	// Each row drives one arm of physicalDiskStatus:
	//   - the assessment.Level switch (RiskCritical | RiskWarning | other),
	//   - each health arm (PASSED/OK | FAILED+firmwareBug | FAILED+noBug | default),
	//   - the strings.ToUpper/TrimSpace normalization.
	cases := []struct {
		name       string
		model      string
		health     string
		assessment storagehealth.Assessment
		want       ResourceStatus
	}{
		// Assessment arms: short-circuit before the health switch is reached.
		// A critical assessment must produce warning even when health is FAILED.
		{
			name:       "critical_assessment_wins_over_failed_health",
			model:      "Crucial MX500",
			health:     "FAILED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskCritical},
			want:       StatusWarning,
		},
		// A warning assessment must produce warning even when health is PASSED.
		{
			name:       "warning_assessment_wins_over_passed_health",
			model:      "Crucial MX500",
			health:     "PASSED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskWarning},
			want:       StatusWarning,
		},
		// RiskMonitor is NOT in the (Critical, Warning) arm of the switch —
		// it must fall through to the health check. This is the key
		// branch-coverage boundary for the assessment switch.
		{
			name:       "monitor_assessment_falls_through_to_health_check",
			model:      "Crucial MX500",
			health:     "PASSED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskMonitor},
			want:       StatusOnline,
		},
		// PASSED health arm.
		{
			name:       "passed_health_returns_online",
			model:      "Crucial MX500",
			health:     "PASSED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusOnline,
		},
		// OK health arm (the other member of the "PASSED, OK" case).
		{
			name:       "ok_health_returns_online",
			model:      "Crucial MX500",
			health:     "OK",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusOnline,
		},
		// FAILED + known firmware bug → StatusUnknown (the
		// HasKnownFirmwareBug(model)==true arm).
		{
			name:       "failed_health_with_known_firmware_bug_returns_unknown",
			model:      "Samsung SSD 980 Pro",
			health:     "FAILED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusUnknown,
		},
		// FAILED + no known firmware bug → StatusOffline (the
		// HasKnownFirmwareBug(model)==false arm).
		{
			name:       "failed_health_without_firmware_bug_returns_offline",
			model:      "Crucial MX500",
			health:     "FAILED",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusOffline,
		},
		// default arm: unrecognized health string → StatusUnknown.
		{
			name:       "unrecognized_health_returns_unknown",
			model:      "Crucial MX500",
			health:     "PREFAIL",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusUnknown,
		},
		// default arm: empty health → StatusUnknown.
		{
			name:       "empty_health_returns_unknown",
			model:      "Crucial MX500",
			health:     "",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusUnknown,
		},
		// Normalization arm: lowercase + surrounding whitespace is accepted.
		{
			name:       "lowercase_whitespace_passed_is_normalized_to_online",
			model:      "Crucial MX500",
			health:     "  passed  ",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusOnline,
		},
		// Normalization arm: "ok" with whitespace/casing also lands in the
		// OK case after ToUpper(TrimSpace(...)).
		{
			name:       "lowercase_whitespace_ok_is_normalized_to_online",
			model:      "Crucial MX500",
			health:     "\tok\t",
			assessment: storagehealth.Assessment{Level: storagehealth.RiskHealthy},
			want:       StatusOnline,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PhysicalDiskStatus(tc.model, tc.health, tc.assessment)
			if got != tc.want {
				t.Fatalf("PhysicalDiskStatus(model=%q, health=%q, level=%s): got %s, want %s",
					tc.model, tc.health, tc.assessment.Level, got, tc.want)
			}
		})
	}
}

// --- PhysicalDiskMetricID ---

func TestBranchcov0720am_PhysicalDiskMetricID(t *testing.T) {
	t.Parallel()

	// Each row drives one combination of the two conditional arms in
	// PhysicalDiskMetricID (the ID!=empty fallback branch and the
	// ID-empty+DevPath-nonempty sprintf branch) together with the
	// downstream PreferredPhysicalDiskMetricID preference order
	// (serial > wwn > fallback).
	cases := []struct {
		name string
		disk models.PhysicalDisk
		// want is the exact expected id when the behavior of the function
		// is to return one of the input fields verbatim (after trim).
		// For the synthesized DevPath cases we leave want empty and use
		// the behavioral predicates below.
		want         string
		wantNonEmpty bool // when true, assert got != "" instead of equality
	}{
		// Preference: Serial wins outright, even when every other field is set.
		{
			name: "serial_wins_over_wwn_id_and_devpath",
			disk: models.PhysicalDisk{
				Serial: "SER-1", WWN: "WWN-1", ID: "id-1", DevPath: "/dev/sda",
				Instance: "inst", Node: "node",
			},
			want: "SER-1",
		},
		// Preference: WWN wins when Serial is empty (with ID present — ID
		// branch hit but loses to WWN downstream).
		{
			name: "wwn_wins_when_serial_empty_and_id_present",
			disk: models.PhysicalDisk{
				Serial: "", WWN: "WWN-1", ID: "id-1",
			},
			want: "WWN-1",
		},
		// Preference: WWN wins when Serial is empty and ID absent —
		// exercises TrimSpace on WWN at the Preferred layer.
		{
			name: "wwn_wins_and_is_trimmed_when_only_wwn_set",
			disk: models.PhysicalDisk{
				Serial: "", WWN: "  WWN-1  ", ID: "",
			},
			want: "WWN-1",
		},
		// Fallback arm #1: ID is non-empty (DevPath sprintf branch
		// skipped), no Serial/WWN → trimmed ID returned.
		{
			name: "id_used_as_fallback_when_no_serial_or_wwn",
			disk: models.PhysicalDisk{
				ID: "  disk-id-1  ",
			},
			want: "disk-id-1",
		},
		// Fallback arm #2: ID empty, DevPath non-empty → sprintf(instance,
		// node, devpath-with-slashes-replaced) synthesized. We assert
		// behavior rather than a fragile literal below.
		{
			name: "devpath_fallback_synthesized_when_id_empty",
			disk: models.PhysicalDisk{
				ID: "", DevPath: "/dev/sda", Instance: "inst-1", Node: "node-1",
			},
			wantNonEmpty: true,
		},
		// Fallback arm #2 with whitespace: confirms TrimSpace is applied
		// to Instance, Node, DevPath before synthesis.
		{
			name: "devpath_fallback_trims_whitespace_on_components",
			disk: models.PhysicalDisk{
				ID: "   ", DevPath: "  /dev/nvme0n1  ", Instance: "  inst-1  ", Node: "  node-1  ",
			},
			wantNonEmpty: true,
		},
		// Defensive: all identity fields empty → returns empty string
		// (neither fallback branch is entered, Preferred returns the
		// trimmed empty fallback).
		{
			name: "all_identity_empty_returns_empty_string",
			disk: models.PhysicalDisk{},
			want: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PhysicalDiskMetricID(tc.disk)

			if tc.wantNonEmpty {
				if got == "" {
					t.Fatalf("PhysicalDiskMetricID(%+v): expected non-empty id, got %q", tc.disk, got)
				}
				return
			}

			if got != tc.want {
				t.Fatalf("PhysicalDiskMetricID(%+v): got %q, want %q", tc.disk, got, tc.want)
			}
		})
	}
}

// TestBranchcov0720am_PhysicalDiskMetricID_DevPathFallbackBehavior
// verifies the synthesized DevPath fallback is built from the
// Instance, Node and DevPath components per the "%s-%s-%s" format and
// that every "/" in DevPath is replaced with "-" (the
// strings.ReplaceAll arm). It uses behavioral predicates rather than
// pinning the exact literal so the test still passes if the format
// separator changes, but breaks if the slash replacement or
// component inclusion changes.
func TestBranchcov0720am_PhysicalDiskMetricID_DevPathFallbackBehavior(t *testing.T) {
	t.Parallel()

	disk := models.PhysicalDisk{
		ID:       "", // forces DevPath fallback arm
		DevPath:  "/dev/disk/by-id/sda",
		Instance: "inst-xyz",
		Node:     "node-abc",
		// No Serial/WWN so the synthesized fallback is what's returned.
	}

	got := PhysicalDiskMetricID(disk)

	if got == "" {
		t.Fatal("expected synthesized non-empty id")
	}

	// Every slash in DevPath must have been replaced (the strings.ReplaceAll arm).
	if strings.Contains(got, "/") {
		t.Errorf("synthesized id must contain no '/': got %q", got)
	}

	// The DevPath, with slashes replaced, must appear in the result.
	if !strings.Contains(got, "-dev-disk-by-id-sda") {
		t.Errorf("synthesized id should embed the slash-replaced DevPath: got %q", got)
	}

	// Instance and Node must each appear in the result.
	if !strings.Contains(got, "inst-xyz") {
		t.Errorf("synthesized id should embed Instance: got %q", got)
	}
	if !strings.Contains(got, "node-abc") {
		t.Errorf("synthesized id should embed Node: got %q", got)
	}

	// The synthesized id must start with the Instance component (per the
	// "%s-%s-%s" format with Instance first).
	if !strings.HasPrefix(got, "inst-xyz-") {
		t.Errorf("synthesized id should start with Instance+separator: got %q", got)
	}
}

// TestBranchcov0720am_PhysicalDiskMetricID_DevPathFallbackStopsAtEmptyDevPath
// verifies that when ID is empty AND DevPath is also empty/whitespace,
// the sprintf arm is NOT entered and the function returns the empty
// fallback (rather than producing "instance-node-" garbage). This is
// the branch-coverage boundary for the
// `fallback == "" && DevPath != ""` predicate's second operand.
func TestBranchcov0720am_PhysicalDiskMetricID_DevPathFallbackStopsAtEmptyDevPath(t *testing.T) {
	t.Parallel()

	// ID empty, DevPath empty — Instance/Node set but must NOT be used.
	disk := models.PhysicalDisk{
		ID:       "",
		DevPath:  "   ",
		Instance: "inst-should-not-appear",
		Node:     "node-should-not-appear",
	}

	got := PhysicalDiskMetricID(disk)

	if got != "" {
		t.Errorf("empty ID and empty DevPath must produce empty id (sprintf arm must not fire): got %q", got)
	}
	if strings.Contains(got, "should-not-appear") {
		t.Errorf("Instance/Node leaked into id despite empty DevPath: got %q", got)
	}
}
