package agentexec

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestCapabilityPostconditionsCoversRequiredCapabilities pins the closed set
// of capabilities the verifier substrate must understand. Drift either way
// (missing entry, drift in field/comparator) breaks the verifier contract.
func TestCapabilityPostconditionsCoversRequiredCapabilities(t *testing.T) {
	required := []string{"qm.start", "pct.start", "docker.restart", "systemctl.restart", "kubectl.rollout"}
	sort.Strings(required)
	got := CapabilityPostconditionNames()
	if !reflect.DeepEqual(got, required) {
		t.Fatalf("capability postcondition names = %v, want %v", got, required)
	}
}

// TestCapabilityPostconditionLookupReturnsCopy ensures callers cannot mutate
// the shared substrate by editing the returned slice.
func TestCapabilityPostconditionLookupReturnsCopy(t *testing.T) {
	first, ok := LookupCapabilityPostcondition("qm.start")
	if !ok {
		t.Fatalf("qm.start not registered")
	}
	first.Checks[0].Expected = "definitely-not-running"

	second, _ := LookupCapabilityPostcondition("qm.start")
	if second.Checks[0].Expected != "running" {
		t.Fatalf("mutation leaked into substrate: %q", second.Checks[0].Expected)
	}
}

// TestCapabilityPostconditionEntriesParse exercises every entry's checks so
// missing or unrecognized fields/comparators are caught.
func TestCapabilityPostconditionEntriesParse(t *testing.T) {
	cases := []struct {
		capability       string
		minChecks        int
		expectFields     []PostconditionField
		expectCompare    []PostconditionComparator
		expectWindow     time.Duration
		mustReferenceCmd string
	}{
		{
			capability:       "qm.start",
			minChecks:        1,
			expectFields:     []PostconditionField{FieldVMStatus},
			expectCompare:    []PostconditionComparator{CompareEquals},
			expectWindow:     2 * time.Minute,
			mustReferenceCmd: "qm status",
		},
		{
			capability:       "pct.start",
			minChecks:        1,
			expectFields:     []PostconditionField{FieldContainerStatus},
			expectCompare:    []PostconditionComparator{CompareEquals},
			expectWindow:     2 * time.Minute,
			mustReferenceCmd: "pct status",
		},
		{
			capability:       "docker.restart",
			minChecks:        2,
			expectFields:     []PostconditionField{FieldDockerStatus, FieldDockerLastStarted},
			expectCompare:    []PostconditionComparator{CompareEquals, CompareAfterOrEqualActionStart},
			expectWindow:     2 * time.Minute,
			mustReferenceCmd: "docker inspect",
		},
		{
			capability:       "systemctl.restart",
			minChecks:        3,
			expectFields:     []PostconditionField{FieldUnitActiveState, FieldUnitSubState, FieldUnitActiveEnterTimestamp},
			expectCompare:    []PostconditionComparator{CompareEquals, CompareEquals, CompareAfterOrEqualActionStart},
			expectWindow:     2 * time.Minute,
			mustReferenceCmd: "systemctl show",
		},
		{
			capability:       "kubectl.rollout",
			minChecks:        1,
			expectFields:     []PostconditionField{FieldDeploymentReadyReplicas},
			expectCompare:    []PostconditionComparator{CompareEqualsField},
			expectWindow:     2 * time.Minute,
			mustReferenceCmd: "kubectl get deployment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.capability, func(t *testing.T) {
			entry, ok := LookupCapabilityPostcondition(tc.capability)
			if !ok {
				t.Fatalf("missing postcondition for %q", tc.capability)
			}
			if entry.Capability != tc.capability {
				t.Fatalf("entry.Capability = %q, want %q", entry.Capability, tc.capability)
			}
			if entry.Window != tc.expectWindow {
				t.Fatalf("entry.Window = %v, want %v", entry.Window, tc.expectWindow)
			}
			if !strings.Contains(entry.VerifyRead, tc.mustReferenceCmd) {
				t.Fatalf("VerifyRead = %q, must reference %q", entry.VerifyRead, tc.mustReferenceCmd)
			}
			if entry.Description == "" {
				t.Fatalf("entry.Description is empty")
			}
			if len(entry.Checks) < tc.minChecks {
				t.Fatalf("checks for %q = %d, want at least %d", tc.capability, len(entry.Checks), tc.minChecks)
			}
			for i, want := range tc.expectFields {
				if entry.Checks[i].Field != want {
					t.Fatalf("%s.Checks[%d].Field = %q, want %q", tc.capability, i, entry.Checks[i].Field, want)
				}
			}
			for i, want := range tc.expectCompare {
				if entry.Checks[i].Comparator != want {
					t.Fatalf("%s.Checks[%d].Comparator = %q, want %q", tc.capability, i, entry.Checks[i].Comparator, want)
				}
			}
		})
	}
}

// TestLookupCapabilityPostconditionUnknown verifies the false return path so
// callers can branch into VerificationUnknown rather than seeing a zero-value
// entry that looks valid.
func TestLookupCapabilityPostconditionUnknown(t *testing.T) {
	if _, ok := LookupCapabilityPostcondition("does.not.exist"); ok {
		t.Fatalf("expected unknown capability to return ok=false")
	}
	if _, ok := LookupCapabilityPostcondition("   "); ok {
		t.Fatalf("expected blank capability to return ok=false")
	}
}
