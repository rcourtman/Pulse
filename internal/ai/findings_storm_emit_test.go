package ai

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func newClusterMember(idSuffix, resourceID, title string) *Finding {
	return &Finding{
		ID:           "member-" + idSuffix,
		ResourceID:   resourceID,
		ResourceName: resourceID,
		ResourceType: "vm",
		Title:        title,
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		Source:       "test",
	}
}

// TestStormEmit_BelowThresholdDoesNotEmit verifies that two distinct
// findings on the same resource within the window do NOT produce a storm
// finding through FindingsStore.Add.
func TestStormEmit_BelowThresholdDoesNotEmit(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	if !store.Add(newClusterMember("a", "vm/810", "cpu pegged")) {
		t.Fatal("first add: expected new-finding return true")
	}
	if !store.Add(newClusterMember("b", "vm/810", "swap pressure")) {
		t.Fatal("second add: expected new-finding return true")
	}
	if got := store.Get(stormFindingIDPrefix + "vm/810"); got != nil {
		t.Errorf("storm finding leaked below threshold: %+v", got)
	}
}

// TestStormEmit_ThirdEmissionEmitsStormFinding crosses threshold and
// asserts the storm finding lands in the store with the expected shape.
func TestStormEmit_ThirdEmissionEmitsStormFinding(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	store.Add(newClusterMember("a", "vm/811", "cpu pegged"))
	store.Add(newClusterMember("b", "vm/811", "swap pressure"))
	store.Add(newClusterMember("c", "vm/811", "disk fill"))

	storm := store.Get(stormFindingIDPrefix + "vm/811")
	if storm == nil {
		t.Fatal("storm finding not emitted after third in-window emission")
	}
	if storm.Source != stormFindingSource {
		t.Errorf("storm Source: want %q, got %q", stormFindingSource, storm.Source)
	}
	if storm.Category != FindingCategoryReliability {
		t.Errorf("storm Category: want %q, got %q", FindingCategoryReliability, storm.Category)
	}
	if storm.Severity != FindingSeverityWarning {
		t.Errorf("storm Severity: want %q, got %q", FindingSeverityWarning, storm.Severity)
	}
	if !storm.IsActive() {
		t.Errorf("storm finding: want IsActive() true, got false (%+v)", storm)
	}
	if storm.TimesRaised < 1 {
		t.Errorf("storm TimesRaised: want >= 1, got %d", storm.TimesRaised)
	}
	if !strings.Contains(storm.Description, "disk fill") {
		t.Errorf("storm Description should mention freshest contributor; got %q", storm.Description)
	}
}

// TestStormEmit_FourthEmissionDoesNotDuplicate confirms the storm
// finding's stable ID lands in Add's existing-finding branch so a fourth
// emission inside the same window bumps TimesRaised rather than spawning
// a duplicate storm finding.
func TestStormEmit_FourthEmissionDoesNotDuplicate(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	store.Add(newClusterMember("a", "vm/812", "cpu pegged"))
	store.Add(newClusterMember("b", "vm/812", "swap pressure"))
	store.Add(newClusterMember("c", "vm/812", "disk fill"))
	stormAfterThird := store.Get(stormFindingIDPrefix + "vm/812")
	if stormAfterThird == nil {
		t.Fatal("storm finding missing after third emission")
	}
	raisedAfterThird := stormAfterThird.TimesRaised

	store.Add(newClusterMember("d", "vm/812", "net packet loss"))

	// Exactly one storm finding for this cluster.
	stormFindings := stormFindingsFor(store, "vm/812")
	if len(stormFindings) != 1 {
		t.Fatalf("expected exactly one storm finding for vm/812, got %d (%v)", len(stormFindings), stormFindings)
	}
	if stormFindings[0].TimesRaised <= raisedAfterThird {
		t.Errorf("storm TimesRaised should advance on re-emission: was %d, now %d", raisedAfterThird, stormFindings[0].TimesRaised)
	}
}

// TestStormEmit_CycleGuardThroughAdd directly emits findings whose Source
// matches the storm-finding source through Add and verifies they never
// trip a storm. This proves the cycle guard is reachable through Add and
// not only through the unit-test direct call.
func TestStormEmit_CycleGuardThroughAdd(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	for i := 0; i < 6; i++ {
		f := newClusterMember(fmt.Sprintf("s%d", i), "vm/813", fmt.Sprintf("symptom %d", i))
		f.Source = stormFindingSource
		store.Add(f)
	}
	if got := store.Get(stormFindingIDPrefix + "vm/813"); got != nil {
		t.Errorf("cycle guard breached: storm finding emitted from storm-source emissions: %+v", got)
	}
}

// TestStormEmit_AIServiceFilterThroughAdd verifies findings against the
// synthetic patrol-runtime resource never trip a storm.
func TestStormEmit_AIServiceFilterThroughAdd(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	for i := 0; i < 6; i++ {
		store.Add(newClusterMember(fmt.Sprintf("p%d", i), patrolRuntimeResourceID, fmt.Sprintf("patrol err %d", i)))
	}
	if got := store.Get(stormFindingIDPrefix + patrolRuntimeResourceID); got != nil {
		t.Errorf("ai-service filter breached: %+v", got)
	}
}

// TestStormEmit_ResolveSentinelAutoResolves drives Add to emit a storm
// finding, then exercises the resolve path by calling the throttler
// directly with a synthetic future time (the brief's lazy auto-resolve
// would otherwise require waiting 2 * stormWindow of wall clock time).
// The sentinel is dispatched through ResolveWithReason to prove that the
// store resolves the storm finding under the throttler's contract.
func TestStormEmit_ResolveSentinelAutoResolves(t *testing.T) {
	store := NewFindingsStore()
	tr := newFindingStormThrottler()
	store.SetStormThrottler(tr)

	store.Add(newClusterMember("a", "vm/814", "cpu pegged"))
	store.Add(newClusterMember("b", "vm/814", "swap pressure"))
	store.Add(newClusterMember("c", "vm/814", "disk fill"))

	storm := store.Get(stormFindingIDPrefix + "vm/814")
	if storm == nil || !storm.IsActive() {
		t.Fatal("storm finding missing or already resolved before quiet-window simulation")
	}

	// Simulate the next observe trip happening well after the cluster
	// has aged out. observeLocked here stands in for what the Add hook
	// would emit on the next real Add against this cluster: a sentinel
	// whose ResolvedAt is non-nil and whose ID names the emitted storm
	// finding.
	future := time.Now().Add(3 * stormWindow)
	sentinel := tr.observeLocked(newClusterMember("d", "vm/814", "much later"), future)
	if sentinel == nil {
		t.Fatal("expected resolve sentinel from throttler after quiet window")
	}
	if sentinel.ResolvedAt == nil {
		t.Fatal("resolve sentinel: ResolvedAt should be non-nil")
	}
	if !store.ResolveWithReason(sentinel.ID, stormResolveReason) {
		t.Fatal("ResolveWithReason on storm finding ID returned false")
	}

	resolved := store.Get(sentinel.ID)
	if resolved == nil {
		t.Fatal("resolved storm finding missing from store")
	}
	if resolved.IsActive() {
		t.Errorf("storm finding still active after ResolveWithReason: %+v", resolved)
	}
	if !resolved.AutoResolved {
		t.Errorf("storm finding AutoResolved: want true, got false")
	}
	if resolved.ResolveReason != stormResolveReason {
		t.Errorf("storm finding ResolveReason: want %q, got %q", stormResolveReason, resolved.ResolveReason)
	}
}

func stormFindingsFor(store *FindingsStore, resourceID string) []*Finding {
	out := []*Finding{}
	for _, f := range store.GetAll(nil) {
		if f == nil {
			continue
		}
		if f.Source == stormFindingSource && f.ResourceID == resourceID {
			out = append(out, f)
		}
	}
	return out
}
