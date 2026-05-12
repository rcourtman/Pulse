package api

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// TestEmitFlappingPostmortemFinding_DedupsByTrackingKey verifies the key
// invariant stated in the lane brief: two flapping transitions for the same
// trackingKey within the cooldown window must produce exactly one finding.
// The stable finding ID is derived from trackingKey, so re-emission folds
// into the existing record via FindingsStore.Add's same-ID branch.
func TestEmitFlappingPostmortemFinding_DedupsByTrackingKey(t *testing.T) {
	patrol := ai.NewPatrolService(nil, nil)
	alertManager := alerts.NewManager()
	t.Cleanup(func() { alertManager.Stop() })

	store := patrol.GetFindings()
	if store == nil {
		t.Fatal("patrol service has nil findings store; cannot run dedup test")
	}

	alert := &alerts.Alert{
		ID:            "vm-100-cpu-1",
		Type:          "cpu",
		ResourceID:    "vm-100",
		ResourceName:  "testvm",
		CanonicalKind: "vm",
		Level:         alerts.AlertLevelWarning,
	}
	trackingKey := "vm-100/cpu"

	emitFlappingPostmortemFinding(patrol, alertManager, alert, trackingKey)
	emitFlappingPostmortemFinding(patrol, alertManager, alert, trackingKey)

	got := store.GetByResource("vm-100")
	flappingCount := 0
	var found *ai.Finding
	for _, f := range got {
		if f == nil {
			continue
		}
		if strings.HasPrefix(f.ID, "alert-flapping:") {
			flappingCount++
			found = f
		}
	}
	if flappingCount != 1 {
		t.Fatalf("expected exactly 1 flapping finding after 2 emits with same trackingKey, got %d", flappingCount)
	}
	if found == nil {
		t.Fatal("flapping finding not present after dedup test")
	}
	if found.ID != "alert-flapping:"+trackingKey {
		t.Errorf("expected stable ID derived from trackingKey, got %q", found.ID)
	}
	if found.Category != ai.FindingCategoryReliability {
		t.Errorf("expected reliability category, got %q", found.Category)
	}
	if !strings.Contains(found.Title, "flapping") {
		t.Errorf("expected title to mention flapping, got %q", found.Title)
	}
	if !strings.Contains(found.Description, "Pulse detected") {
		t.Errorf("expected description to explain detection, got %q", found.Description)
	}
	// TimesRaised must increment on re-detection (heartbeat behaviour from
	// FindingsStore.Add's same-ID branch).
	if found.TimesRaised < 2 {
		t.Errorf("expected TimesRaised >= 2 after two emits, got %d", found.TimesRaised)
	}
}

// TestEmitFlappingPostmortemFinding_DistinctTrackingKeys verifies that two
// different trackingKeys produce two distinct findings -- the dedup is
// per-trackingKey, not global.
func TestEmitFlappingPostmortemFinding_DistinctTrackingKeys(t *testing.T) {
	patrol := ai.NewPatrolService(nil, nil)
	alertManager := alerts.NewManager()
	t.Cleanup(func() { alertManager.Stop() })

	a1 := &alerts.Alert{
		ID: "vm-100-cpu", Type: "cpu", ResourceID: "vm-100",
		ResourceName: "testvm", CanonicalKind: "vm", Level: alerts.AlertLevelWarning,
	}
	a2 := &alerts.Alert{
		ID: "vm-100-memory", Type: "memory", ResourceID: "vm-100",
		ResourceName: "testvm", CanonicalKind: "vm", Level: alerts.AlertLevelWarning,
	}

	emitFlappingPostmortemFinding(patrol, alertManager, a1, "vm-100/cpu")
	emitFlappingPostmortemFinding(patrol, alertManager, a2, "vm-100/memory")

	count := 0
	for _, f := range patrol.GetFindings().GetByResource("vm-100") {
		if f != nil && strings.HasPrefix(f.ID, "alert-flapping:") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 distinct flapping findings for distinct tracking keys, got %d", count)
	}
}

// TestEmitFlappingPostmortemFinding_NilGuards confirms the helper is safe
// against the nil cases the router lambda might hand it.
func TestEmitFlappingPostmortemFinding_NilGuards(t *testing.T) {
	patrol := ai.NewPatrolService(nil, nil)
	alertManager := alerts.NewManager()
	t.Cleanup(func() { alertManager.Stop() })

	emitFlappingPostmortemFinding(nil, alertManager, &alerts.Alert{ID: "a"}, "k")
	emitFlappingPostmortemFinding(patrol, alertManager, nil, "k")
	emitFlappingPostmortemFinding(patrol, alertManager, &alerts.Alert{ID: "a"}, "")
	// No panic == pass.
}
