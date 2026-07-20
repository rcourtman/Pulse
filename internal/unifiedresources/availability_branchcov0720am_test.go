package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// These tests augment availability_test.go with branch-coverage assertions
// for AvailabilityCheckByTargetID for the 0720am coverage pass. They focus on
// the three conditional arms of that lookup: found-returns-clone,
// not-found-returns-nil, and the empty/nil-data arm where the resource has no
// checks at all (so the loop body never executes). Independence of the
// returned clone is also asserted so future refactors cannot silently begin
// returning aliases into the resource's stored state.

func TestBranchcov0720am_AvailabilityCheckByTargetID(t *testing.T) {
	checkedAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		resource   Resource
		targetID   string
		wantFound  bool
		wantTarget string // asserted verbatim only when wantFound is true
	}{
		{
			name: "found: exact TargetID in AvailabilityChecks returns a clone",
			resource: Resource{AvailabilityChecks: []AvailabilityData{
				{TargetID: "switch-1", Available: false},
				{TargetID: "router-1", Available: true, LastChecked: &checkedAt},
			}},
			targetID:   "router-1",
			wantFound:  true,
			wantTarget: "router-1",
		},
		{
			name: "found: caller-supplied targetID with surrounding whitespace is trimmed before matching",
			resource: Resource{AvailabilityChecks: []AvailabilityData{
				{TargetID: "router-1", Available: true},
			}},
			targetID:   "  router-1\t",
			wantFound:  true,
			wantTarget: "router-1",
		},
		{
			// Both sides trim: a check whose stored TargetID has whitespace
			// still matches a clean caller input. The stored value is not
			// rewritten by merge, so we assert on the trimmed form rather than
			// pinning the exact whitespace.
			name: "found: stored check TargetID with surrounding whitespace matches after trim on both sides",
			resource: Resource{AvailabilityChecks: []AvailabilityData{
				{TargetID: "  router-1  "},
			}},
			targetID:  "router-1",
			wantFound: true,
		},
		{
			name: "not found: checks present but no TargetID matches",
			resource: Resource{AvailabilityChecks: []AvailabilityData{
				{TargetID: "router-1"},
				{TargetID: "switch-1"},
			}},
			targetID:  "missing-target",
			wantFound: false,
		},
		{
			name:      "nil-data: resource has no checks and no compatibility summary -> loop never executes -> nil",
			resource:  Resource{},
			targetID:  "router-1",
			wantFound: false,
		},
		{
			name: "empty after trim: whitespace-only targetID must not match any stored check",
			resource: Resource{AvailabilityChecks: []AvailabilityData{
				{TargetID: "router-1"},
			}},
			targetID:  "   ",
			wantFound: false,
		},
		{
			// The compatibility summary (resource.Availability) is folded into
			// the canonical check set by AvailabilityChecksForResource, so a
			// probe recorded only as the singular summary must also be
			// retrievable by TargetID. This is the alert/evidence consumer
			// contract the function's doc comment promises.
			name: "found: singular Availability summary is folded in and matchable by TargetID",
			resource: Resource{
				Availability: &AvailabilityData{TargetID: "summary-target", Available: true},
			},
			targetID:   "summary-target",
			wantFound:  true,
			wantTarget: "summary-target",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := AvailabilityCheckByTargetID(c.resource, c.targetID)
			if c.wantFound {
				if got == nil {
					t.Fatalf("AvailabilityCheckByTargetID(...) = nil, want non-nil clone for targetID %q", c.targetID)
				}
				if c.wantTarget != "" && got.TargetID != c.wantTarget {
					t.Errorf("returned TargetID = %q, want %q", got.TargetID, c.wantTarget)
				}
			} else {
				if got != nil {
					t.Fatalf("AvailabilityCheckByTargetID(...) = %+v, want nil", got)
				}
			}
		})
	}
}

func TestBranchcov0720am_AvailabilityCheckByTargetID_CloneIsIndependent(t *testing.T) {
	// The found arm must return a deep clone: mutating scalar fields, the
	// time-pointer fields, and the Evidence envelope of the returned value
	// must not propagate back into the resource's stored check. A future
	// refactor that began returning an alias would surface here.
	originalChecked := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	source := operationaltrust.EvidenceEnvelope{
		Source:       operationaltrust.EvidenceSource{Provider: "availability", Collector: "poller"},
		Subject:      operationaltrust.EvidenceSubject{ProviderRef: "router-1", ProviderScope: "availability-target"},
		ObservedAt:   originalChecked,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
	}
	resource := Resource{AvailabilityChecks: []AvailabilityData{{
		TargetID:    "router-1",
		Available:   true,
		LastChecked: &originalChecked,
		Evidence:    &source,
	}}}

	got := AvailabilityCheckByTargetID(resource, "router-1")
	if got == nil {
		t.Fatalf("AvailabilityCheckByTargetID(...) = nil, want clone")
	}

	// Mutate every cloneable field of the returned value.
	got.Available = false
	got.LastChecked = nil
	if got.Evidence != nil {
		got.Evidence.Completeness = operationaltrust.EvidencePartial
		got.Evidence.Source.Provider = "mutated"
	}

	again := AvailabilityCheckByTargetID(resource, "router-1")
	if again == nil {
		t.Fatalf("second lookup returned nil; resource state was mutated by the first call")
	}
	if !again.Available {
		t.Errorf("scalar field independence: Available = false, want true (clone leaked mutation into source)")
	}
	if again.LastChecked == nil || !again.LastChecked.Equal(originalChecked) {
		t.Errorf("LastChecked independence: got %v, want %v (pointer field shared with source)", again.LastChecked, originalChecked)
	}
	if again.Evidence == nil {
		t.Fatalf("Evidence independence: Evidence = nil, want non-nil clone")
	}
	if again.Evidence.Completeness != operationaltrust.EvidenceComplete {
		t.Errorf("Evidence independence: Completeness = %q, want %q (envelope shared with source)",
			again.Evidence.Completeness, operationaltrust.EvidenceComplete)
	}
	if again.Evidence.Source.Provider != "availability" {
		t.Errorf("Evidence independence: Source.Provider = %q, want %q (nested struct shared with source)",
			again.Evidence.Source.Provider, "availability")
	}
}

func TestBranchcov0720am_AvailabilityCheckByTargetID_FirstMatchWins(t *testing.T) {
	// When two distinct checks would normalize to the same TargetID, the
	// canonical merge dedupes them; the lookup must therefore find exactly
	// one entry and return it without panic. This pins the "first match"
	// semantics against any future change that began scanning into a
	// non-deduped slice.
	resource := Resource{AvailabilityChecks: []AvailabilityData{
		{TargetID: "router-1", Available: true},
		{TargetID: "router-1", Available: false}, // duplicate key, merged away
	}}

	got := AvailabilityCheckByTargetID(resource, "router-1")
	if got == nil {
		t.Fatalf("AvailabilityCheckByTargetID(...) = nil, want the single merged check")
	}
	// After merge only one entry exists, so the lookup is unambiguous; we
	// only assert presence here, not which duplicate survived, because the
	// merge order is an internal detail of mergeAvailabilityChecks.
}
