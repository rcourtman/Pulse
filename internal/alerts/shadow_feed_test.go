package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newShadowFeedManager(t *testing.T) *Manager {
	t.Helper()
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	manager.SetEventLog(store)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.config.FlappingEnabled = false
	manager.mu.Unlock()
	manager.EnableShadowFeed()
	return manager
}

func shadowObserveConnectivity(t *testing.T, manager *Manager, resourceID string, connected bool, at time.Time) {
	t.Helper()
	spec, err := buildCanonicalConnectivitySpec(
		resourceID, resourceID, unifiedresources.ResourceTypePBS,
		AlertLevelCritical, 3, false,
	)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	if _, ok := manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt:   at,
			Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: connected},
		},
		Tracking:     manager.offlineConfirmations,
		TrackingKey:  resourceID,
		AlertID:      canonicalConnectivityStateID(resourceID),
		AlertType:    "offline",
		ResourceID:   resourceID,
		ResourceName: resourceID,
		Message:      "shadow feed test offline",
	}); !ok {
		t.Fatal("evaluateCanonicalLifecycleAlert rejected the spec")
	}
}

func shadowDivergenceEvents(t *testing.T, manager *Manager) []eventlog.Event {
	t.Helper()
	events, err := manager.AlertEvents(eventlog.Filter{Types: []string{eventlog.TypeShadowDivergence}})
	if err != nil {
		t.Fatalf("AlertEvents: %v", err)
	}
	return events
}

// The full production cycle — confirmation-based activation, ack, unack,
// recovery-gated resolve, and re-fire — must produce zero divergences: the
// shadow reducer tracks the live manager exactly through the hooked paths.
func TestShadowFeedNoDivergenceThroughFullCycle(t *testing.T) {
	manager := newShadowFeedManager(t)
	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	resourceID := "shadow-pbs-1"

	// Fire after three offline confirmations.
	shadowObserveConnectivity(t, manager, resourceID, false, epoch)
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(30*time.Second))
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(time.Minute))

	// Ack, then unack.
	alertID := canonicalConnectivityStateID(resourceID)
	if err := manager.AcknowledgeAlert(alertID, "richard"); err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}
	if err := manager.UnacknowledgeAlert(alertID); err != nil {
		t.Fatalf("UnacknowledgeAlert: %v", err)
	}

	// Recovery-gated resolve: three healthy polls.
	for i := 0; i < 3; i++ {
		manager.clearResourceOfflineAlert(resourceID, resourceID, "host", "PBS", offlineRecoveryConfirmationsDefault)
	}

	// Re-fire.
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(2*time.Minute))
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(150*time.Second))
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(3*time.Minute))

	if divergences := manager.ShadowDivergences(); divergences != 0 {
		t.Fatalf("ShadowDivergences = %d, want 0; events: %+v", divergences, shadowDivergenceEvents(t, manager))
	}
	if events := shadowDivergenceEvents(t, manager); len(events) != 0 {
		t.Fatalf("divergence events = %+v, want none", events)
	}
}

// A reducer/manager disagreement — simulated by seeding a bogus shadow
// incident the manager knows nothing about — is counted, recorded once,
// and resynced so the next observation is clean.
func TestShadowFeedRecordsAndResyncsDivergence(t *testing.T) {
	manager := newShadowFeedManager(t)
	resourceID := "shadow-bogus-1"

	manager.mu.Lock()
	manager.shadow.state.SeedFiringIncident(
		resourceID, canonicalConnectivitySpecID(resourceID),
		reducer.SeverityCritical, time.Now().Add(-time.Minute), false,
	)
	manager.mu.Unlock()

	// A healthy poll with a 3-poll recovery gate: the reducer holds firing,
	// the manager has no alert — divergence, then resync (Forget).
	manager.clearResourceOfflineAlert(resourceID, resourceID, "host", "PBS", offlineRecoveryConfirmationsDefault)

	if divergences := manager.ShadowDivergences(); divergences != 1 {
		t.Fatalf("ShadowDivergences = %d, want 1", divergences)
	}
	events := shadowDivergenceEvents(t, manager)
	if len(events) != 1 {
		t.Fatalf("divergence events = %d, want 1", len(events))
	}
	if events[0].Details["managerFiring"] != "false" || events[0].Details["reducerFiring"] != "true" {
		t.Fatalf("event details = %+v, want manager=false reducer=true", events[0].Details)
	}

	// After resync the next healthy poll agrees.
	manager.clearResourceOfflineAlert(resourceID, resourceID, "host", "PBS", offlineRecoveryConfirmationsDefault)
	if divergences := manager.ShadowDivergences(); divergences != 1 {
		t.Fatalf("ShadowDivergences after resync = %d, want still 1", divergences)
	}
}

// Enabling the feed on a manager with existing alerts seeds the reducer so
// restored state does not read as mass divergence.
func TestShadowFeedSeedsFromActiveAlerts(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)
	manager.mu.Lock()
	manager.config.Enabled = true
	manager.mu.Unlock()

	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	resourceID := "shadow-seeded-1"

	// Fire before the feed exists (simulating restored state).
	shadowObserveConnectivity(t, manager, resourceID, false, epoch)
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(30*time.Second))
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(time.Minute))

	manager.EnableShadowFeed()

	// Continued offline observations and a full recovery agree throughout.
	shadowObserveConnectivity(t, manager, resourceID, false, epoch.Add(90*time.Second))
	for i := 0; i < 3; i++ {
		manager.clearResourceOfflineAlert(resourceID, resourceID, "host", "PBS", offlineRecoveryConfirmationsDefault)
	}
	if divergences := manager.ShadowDivergences(); divergences != 0 {
		t.Fatalf("ShadowDivergences = %d, want 0", divergences)
	}
}
