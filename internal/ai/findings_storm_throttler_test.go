package ai

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func newStormFinding(id, resourceID, title string) *Finding {
	return &Finding{
		ID:           id,
		ResourceID:   resourceID,
		ResourceName: resourceID,
		ResourceType: "vm",
		Title:        title,
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
	}
}

func TestStormThrottler_BelowThresholdReturnsNil(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()

	if got := tr.observeLocked(newStormFinding("a", "vm/100", "high cpu"), base); got != nil {
		t.Fatalf("first emission below threshold: want nil, got %+v", got)
	}
	if got := tr.observeLocked(newStormFinding("b", "vm/100", "memory swap"), base.Add(5*time.Second)); got != nil {
		t.Fatalf("second emission below threshold: want nil, got %+v", got)
	}
}

func TestStormThrottler_CrossesThresholdEmitsStormFinding(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()

	tr.observeLocked(newStormFinding("a", "vm/100", "high cpu"), base)
	tr.observeLocked(newStormFinding("b", "vm/100", "memory swap"), base.Add(5*time.Second))
	got := tr.observeLocked(newStormFinding("c", "vm/100", "disk fill"), base.Add(10*time.Second))
	if got == nil {
		t.Fatal("third emission within window: expected storm finding, got nil")
	}
	if got.ID != stormFindingIDPrefix+"vm/100" {
		t.Errorf("storm finding ID: want %q, got %q", stormFindingIDPrefix+"vm/100", got.ID)
	}
	if got.Key != got.ID {
		t.Errorf("storm finding Key: want %q, got %q", got.ID, got.Key)
	}
	if got.Source != stormFindingSource {
		t.Errorf("storm finding Source: want %q, got %q", stormFindingSource, got.Source)
	}
	if got.Category != FindingCategoryReliability {
		t.Errorf("storm finding Category: want %q, got %q", FindingCategoryReliability, got.Category)
	}
	if got.Severity != FindingSeverityWarning {
		t.Errorf("storm finding Severity: want %q, got %q", FindingSeverityWarning, got.Severity)
	}
	if got.ResolvedAt != nil {
		t.Errorf("storm finding ResolvedAt: want nil (emit path), got %v", got.ResolvedAt)
	}
	if !strings.Contains(got.Evidence, "emissions=3") {
		t.Errorf("storm finding Evidence: want emissions=3, got %q", got.Evidence)
	}
	// Title and Description should reference the resource name freshest contributor.
	if !strings.Contains(got.Title, "vm/100") {
		t.Errorf("storm finding Title: want resource name, got %q", got.Title)
	}
	if !strings.Contains(got.Description, "high cpu") || !strings.Contains(got.Description, "disk fill") {
		t.Errorf("storm finding Description: want contributor titles, got %q", got.Description)
	}
}

func TestStormThrottler_FiltersStormFindingSource(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	// Many emissions whose Source is the storm-finding source should NOT
	// count toward threshold — this is the cycle guard.
	for i := 0; i < 10; i++ {
		f := newStormFinding(fmt.Sprintf("s%d", i), "vm/200", "some title")
		f.Source = stormFindingSource
		if got := tr.observeLocked(f, base.Add(time.Duration(i)*time.Second)); got != nil {
			t.Fatalf("storm-source emission %d should be filtered, got %+v", i, got)
		}
	}
}

func TestStormThrottler_FiltersPatrolRuntimeResource(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	for i := 0; i < 10; i++ {
		f := newStormFinding(fmt.Sprintf("p%d", i), patrolRuntimeResourceID, "patrol error")
		if got := tr.observeLocked(f, base.Add(time.Duration(i)*time.Second)); got != nil {
			t.Fatalf("ai-service emission %d should be filtered, got %+v", i, got)
		}
	}
}

func TestStormThrottler_FiltersEmptyResourceID(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	for i := 0; i < 10; i++ {
		f := newStormFinding(fmt.Sprintf("e%d", i), "   ", "anon")
		if got := tr.observeLocked(f, base.Add(time.Duration(i)*time.Second)); got != nil {
			t.Fatalf("empty-resource emission %d should be filtered, got %+v", i, got)
		}
	}
}

func TestStormThrottler_WindowEvictionPreventsFalsePositive(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	// Two emissions then a long pause; third emission outside window
	// should NOT trip threshold because the first two have aged out.
	tr.observeLocked(newStormFinding("a", "vm/300", "x"), base)
	tr.observeLocked(newStormFinding("b", "vm/300", "y"), base.Add(2*time.Second))
	got := tr.observeLocked(newStormFinding("c", "vm/300", "z"), base.Add(stormWindow+30*time.Second))
	if got != nil {
		t.Fatalf("emission outside window should not trigger storm: %+v", got)
	}
}

func TestStormThrottler_LRUEvictsOldestClusterAtCap(t *testing.T) {
	tr := newFindingStormThrottler()
	tr.cap = 3
	base := time.Now()
	// Fill cap with three clusters, each touched at increasing times.
	tr.observeLocked(newStormFinding("a", "vm/A", "a"), base)
	tr.observeLocked(newStormFinding("b", "vm/B", "b"), base.Add(1*time.Second))
	tr.observeLocked(newStormFinding("c", "vm/C", "c"), base.Add(2*time.Second))
	// Fourth cluster forces LRU eviction; vm/A is oldest.
	tr.observeLocked(newStormFinding("d", "vm/D", "d"), base.Add(3*time.Second))
	if _, ok := tr.clusters["vm/A"]; ok {
		t.Errorf("LRU eviction: expected oldest cluster vm/A to be evicted; clusters: %v", clusterKeys(tr))
	}
	if _, ok := tr.clusters["vm/D"]; !ok {
		t.Errorf("LRU eviction: expected current cluster vm/D to be present; clusters: %v", clusterKeys(tr))
	}
	if len(tr.clusters) > tr.cap {
		t.Errorf("LRU eviction: cluster count %d exceeds cap %d", len(tr.clusters), tr.cap)
	}
}

func TestStormThrottler_StormFindingPointsAtFreshestContributor(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	a := newStormFinding("a", "vm/400", "cpu pegged")
	a.ResourceName = "vm-name-A"
	a.Node = "pve-01"
	tr.observeLocked(a, base)
	b := newStormFinding("b", "vm/400", "swap pressure")
	b.ResourceName = "vm-name-B"
	b.Node = "pve-01"
	tr.observeLocked(b, base.Add(5*time.Second))
	c := newStormFinding("c", "vm/400", "disk fill")
	c.ResourceName = "vm-name-C"
	c.Node = "pve-02"
	got := tr.observeLocked(c, base.Add(10*time.Second))
	if got == nil {
		t.Fatal("expected storm finding emission")
	}
	if got.ResourceName != "vm-name-C" {
		t.Errorf("freshest resource name: want vm-name-C, got %q", got.ResourceName)
	}
	if got.Node != "pve-02" {
		t.Errorf("freshest node: want pve-02, got %q", got.Node)
	}
}

func TestStormThrottler_ResolveSentinelAfterQuietWindow(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	// Cross threshold so the cluster has an emitted storm finding.
	tr.observeLocked(newStormFinding("a", "vm/500", "x"), base)
	tr.observeLocked(newStormFinding("b", "vm/500", "y"), base.Add(1*time.Second))
	emit := tr.observeLocked(newStormFinding("c", "vm/500", "z"), base.Add(2*time.Second))
	if emit == nil {
		t.Fatal("expected storm finding emission to seed lastEmittedAt")
	}
	// Long pause — well past 2 * stormWindow with no further activity.
	// Next emission within the same cluster should return a resolve
	// sentinel (ResolvedAt non-nil, ID set to the emitted storm finding).
	resolve := tr.observeLocked(newStormFinding("d", "vm/500", "much later"), base.Add(2*stormWindow+30*time.Second))
	if resolve == nil {
		t.Fatal("expected resolve sentinel after quiet window, got nil")
	}
	if resolve.ResolvedAt == nil {
		t.Errorf("resolve sentinel: want ResolvedAt non-nil, got nil")
	}
	if resolve.ID != stormFindingIDPrefix+"vm/500" {
		t.Errorf("resolve sentinel ID: want %q, got %q", stormFindingIDPrefix+"vm/500", resolve.ID)
	}
}

func TestStormThrottler_RefreshesStormFindingWhileAboveThreshold(t *testing.T) {
	tr := newFindingStormThrottler()
	base := time.Now()
	tr.observeLocked(newStormFinding("a", "vm/600", "x"), base)
	tr.observeLocked(newStormFinding("b", "vm/600", "y"), base.Add(1*time.Second))
	first := tr.observeLocked(newStormFinding("c", "vm/600", "z"), base.Add(2*time.Second))
	if first == nil {
		t.Fatal("expected first storm finding emission")
	}
	// 4th distinct emission while still inside the window should also
	// return the storm finding (with incremented count) so the caller
	// can re-enter it through Add — Add dedups on the stable ID.
	second := tr.observeLocked(newStormFinding("d", "vm/600", "w"), base.Add(3*time.Second))
	if second == nil {
		t.Fatal("expected refresh storm finding on 4th emission, got nil")
	}
	if second.ID != first.ID {
		t.Errorf("refresh storm finding: want same ID %q, got %q", first.ID, second.ID)
	}
	if !strings.Contains(second.Evidence, "emissions=4") {
		t.Errorf("refresh storm finding evidence: want emissions=4, got %q", second.Evidence)
	}
}

func TestStormThrottler_NilReceiverNoPanic(t *testing.T) {
	var tr *findingStormThrottler
	if got := tr.observeLocked(newStormFinding("a", "vm/700", "x"), time.Now()); got != nil {
		t.Errorf("nil throttler: want nil result, got %+v", got)
	}
}

func clusterKeys(tr *findingStormThrottler) []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	keys := make([]string, 0, len(tr.clusters))
	for k := range tr.clusters {
		keys = append(keys, k)
	}
	return keys
}
