package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Regression: the confirmation maps hold only counts, so the reconstructed
// pending state dated the run at the current observation and a
// confirmation-based lifecycle alert stamped StartTime at the final
// confirming poll — understating outage start by the whole confirmation
// window. lifecycleFirstMatched preserves the first matched observation
// (mirroring unifiedIncidentFirstSeen) so activation keeps the true start.
func TestLifecycleAlertStartTimeIsFirstMatchedObservation(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.mu.Unlock()

	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	observe := func(observed string, at time.Time) {
		spec, err := buildCanonicalDiscreteStateSpec(
			"node-9", "node-9", unifiedresources.ResourceTypeAgent,
			AlertLevelCritical, 3, false, "connectivity", []string{"offline"},
		)
		if err != nil {
			t.Fatalf("build spec: %v", err)
		}
		if _, ok := manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt:    at,
				DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "connectivity", Observed: observed},
			},
			Tracking:     manager.offlineConfirmations,
			TrackingKey:  "conn:node-9",
			AlertID:      canonicalDiscreteStateStateID("node-9", "connectivity"),
			AlertType:    "connectivity",
			ResourceID:   "node-9",
			ResourceName: "node-9",
			Message:      "node offline",
		}); !ok {
			t.Fatal("evaluation rejected")
		}
	}

	observe("offline", epoch)
	observe("offline", epoch.Add(30*time.Second))
	observe("offline", epoch.Add(60*time.Second))

	manager.mu.Lock()
	alert, exists := manager.getActiveAlertNoLock(canonicalDiscreteStateStateID("node-9", "connectivity"))
	manager.mu.Unlock()
	if !exists || alert == nil {
		t.Fatal("expected alert to fire after three confirmations")
	}
	if !alert.StartTime.Equal(epoch) {
		t.Fatalf("StartTime = %v, want first offline observation %v", alert.StartTime, epoch)
	}

	// Recovery clears the preserved first-match so a later run re-dates.
	observe("online", epoch.Add(90*time.Second))
	manager.mu.Lock()
	_, tracked := manager.lifecycleFirstMatched["conn:node-9"]
	manager.mu.Unlock()
	if tracked {
		t.Fatal("first-matched entry should clear with the confirmation run")
	}
}
