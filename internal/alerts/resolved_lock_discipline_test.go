package alerts

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog"
)

// Regression for issue #1590. The canonical alert evaluation paths used to
// read and mutate recentlyResolved/resolvedAlias while holding only m.mu,
// while the broadcast and recovery paths guarded them with resolvedMutex, so
// the two lock domains did not exclude each other and Go's runtime aborted
// the process with a concurrent map access fault on flapping resources.
//
// Run with -race. Unlike the original reproducer, this proof drives both
// production canonical evaluators through fire -> recovery -> cooldown refire
// while public broadcast/lookup, alias repair, cleanup, and shutdown paths run
// concurrently.
func TestRecentlyResolvedProductionPathsAreRaceAndDeadlockFree(t *testing.T) {
	originalLogLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(originalLogLevel)
	})

	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)

	lifecycleResourceID := "agent:issue-1590-lifecycle"
	lifecycleSpec, err := buildCanonicalConnectivitySpec(
		lifecycleResourceID,
		"Issue 1590 lifecycle",
		unifiedresources.ResourceTypeAgent,
		AlertLevelWarning,
		1,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleParams := canonicalLifecycleAlertParams{
		Spec:         lifecycleSpec,
		AlertID:      "host-offline-issue-1590-lifecycle",
		AlertType:    "connectivity",
		ResourceID:   lifecycleResourceID,
		ResourceName: "Issue 1590 lifecycle",
		Instance:     "issue-1590",
		Message:      "lifecycle test alert",
		Metadata:     map[string]interface{}{"resourceType": string(unifiedresources.ResourceTypeAgent)},
		AddToHistory: true,
	}
	lifecycleStateID := canonicalTrackingKeyForSpec(lifecycleSpec, lifecycleParams.AlertID)

	statefulResourceID := "storage:issue-1590-stateful"
	statefulSpec, err := buildCanonicalHealthAssessmentSpec(
		"issue-1590-health",
		statefulResourceID,
		"Issue 1590 stateful",
		unifiedresources.ResourceTypeStorage,
		"pool-health",
		[]string{"degraded"},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	statefulParams := canonicalStatefulAlertParams{
		Spec:         statefulSpec,
		AlertID:      "issue-1590-stateful-health",
		AlertType:    "storage-health",
		ResourceID:   statefulResourceID,
		ResourceName: "Issue 1590 stateful",
		Instance:     "issue-1590",
		Message:      "stateful test alert",
		Metadata:     map[string]interface{}{"resourceType": string(unifiedresources.ResourceTypeStorage)},
		AddToHistory: true,
	}
	statefulStateID := canonicalTrackingKeyForSpec(statefulSpec, statefulParams.AlertID)

	fireLifecycle := func(observedAt time.Time) alertspecs.AlertState {
		lifecycleParams.Evidence = alertspecs.AlertEvidence{
			ObservedAt: observedAt,
			Connectivity: &alertspecs.ConnectivityEvidence{
				Signal:    "status",
				Connected: false,
			},
		}
		result, _ := m.evaluateCanonicalLifecycleAlert(lifecycleParams)
		return result.State.State
	}
	recoverLifecycle := func(observedAt time.Time) alertspecs.AlertState {
		lifecycleParams.Evidence = alertspecs.AlertEvidence{
			ObservedAt: observedAt,
			Connectivity: &alertspecs.ConnectivityEvidence{
				Signal:    "status",
				Connected: true,
			},
		}
		result, _ := m.evaluateCanonicalLifecycleAlert(lifecycleParams)
		return result.State.State
	}
	fireStateful := func(observedAt time.Time) alertspecs.AlertState {
		statefulParams.Evidence = alertspecs.AlertEvidence{
			ObservedAt: observedAt,
			HealthAssessment: &alertspecs.HealthAssessmentEvidence{
				Signal:   "pool-health",
				Severity: alertspecs.AlertSeverityWarning,
				Codes:    []string{"degraded"},
			},
		}
		result, _ := m.evaluateCanonicalStatefulAlert(statefulParams)
		return result.State.State
	}
	recoverStateful := func(observedAt time.Time) alertspecs.AlertState {
		statefulParams.Evidence = alertspecs.AlertEvidence{
			ObservedAt:       observedAt,
			HealthAssessment: &alertspecs.HealthAssessmentEvidence{Signal: "pool-health"},
		}
		result, _ := m.evaluateCanonicalStatefulAlert(statefulParams)
		return result.State.State
	}

	base := time.Now().Add(-time.Minute)
	if got := fireLifecycle(base); got != alertspecs.AlertStateFiring {
		t.Fatalf("initial lifecycle state = %q, want firing", got)
	}
	lifecycleStart := testRequireActiveAlert(t, m, lifecycleStateID).StartTime
	if got := fireStateful(base); got != alertspecs.AlertStateFiring {
		t.Fatalf("initial stateful state = %q, want firing", got)
	}
	statefulStart := testRequireActiveAlert(t, m, statefulStateID).StartTime
	if got := recoverLifecycle(base.Add(time.Millisecond)); got != alertspecs.AlertStateClear {
		t.Fatalf("initial lifecycle recovery state = %q, want clear", got)
	}
	if got := recoverStateful(base.Add(time.Millisecond)); got != alertspecs.AlertStateClear {
		t.Fatalf("initial stateful recovery state = %q, want clear", got)
	}

	// Force the canonical-alias fallback path. GetResolvedAlert repairs this
	// alias under the resolved write lock while broadcasts iterate the map.
	aliasStateID := buildCanonicalStateID("agent:issue-1590-alias", "issue-1590-alias")
	m.resolvedMutex.Lock()
	m.recentlyResolved["issue-1590-legacy-storage-key"] = &ResolvedAlert{
		Alert: &Alert{
			ID:              "issue-1590-legacy-alert",
			ResourceID:      "agent:issue-1590-alias",
			CanonicalSpecID: "issue-1590-alias",
			CanonicalState:  aliasStateID,
			StartTime:       base,
		},
		ResolvedTime: time.Now(),
	}
	m.resolvedMutex.Unlock()

	start := make(chan struct{})
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	wg.Add(4)

	// Canonical lifecycle recovery and cooldown refire path.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100; i++ {
			observedAt := base.Add(time.Duration(2*i+2) * time.Millisecond)
			if got := fireLifecycle(observedAt); got != alertspecs.AlertStateFiring {
				errs <- fmt.Errorf("lifecycle iteration %d fire state = %q", i, got)
				return
			}
			if got := recoverLifecycle(observedAt.Add(time.Microsecond)); got != alertspecs.AlertStateClear {
				errs <- fmt.Errorf("lifecycle iteration %d recovery state = %q", i, got)
				return
			}
			runtime.Gosched()
		}
	}()

	// Canonical stateful recovery and cooldown refire path.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100; i++ {
			observedAt := base.Add(time.Duration(2*i+2) * time.Millisecond)
			if got := fireStateful(observedAt); got != alertspecs.AlertStateFiring {
				errs <- fmt.Errorf("stateful iteration %d fire state = %q", i, got)
				return
			}
			if got := recoverStateful(observedAt.Add(time.Microsecond)); got != alertspecs.AlertStateClear {
				errs <- fmt.Errorf("stateful iteration %d recovery state = %q", i, got)
				return
			}
			runtime.Gosched()
		}
	}()

	// Production broadcast and point-lookup paths.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 800; i++ {
			m.GetRecentlyResolved()
			m.GetResolvedAlert(aliasStateID)
			m.GetResolvedAlert(lifecycleStateID)
			m.GetResolvedAlert(statefulStateID)
			runtime.Gosched()
		}
	}()

	// Cleanup is the other production path that holds m.mu before acquiring
	// resolvedMutex.
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100; i++ {
			m.Cleanup(time.Hour)
			runtime.Gosched()
		}
	}()

	close(start)
	waitForResolvedConcurrencyGroup(t, &wg)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		return
	}

	finalObservedAt := base.Add(2 * time.Second)
	if got := fireLifecycle(finalObservedAt); got != alertspecs.AlertStateFiring {
		t.Fatalf("final lifecycle state = %q, want firing", got)
	}
	if got := fireStateful(finalObservedAt); got != alertspecs.AlertStateFiring {
		t.Fatalf("final stateful state = %q, want firing", got)
	}
	if got := testRequireActiveAlert(t, m, lifecycleStateID).StartTime; !got.Equal(lifecycleStart) {
		t.Fatalf("lifecycle refire start = %v, want original %v", got, lifecycleStart)
	}
	if got := testRequireActiveAlert(t, m, statefulStateID).StartTime; !got.Equal(statefulStart) {
		t.Fatalf("stateful refire start = %v, want original %v", got, statefulStart)
	}
	if got := len(m.historyManager.GetAllHistory(1000)); got != 2 {
		t.Fatalf("history entries after repeated cooldown refires = %d, want 2", got)
	}
	if got := m.GetResolvedAlert(lifecycleStateID); got != nil {
		t.Fatal("lifecycle resolved entry survived cooldown refire")
	}
	if got := m.GetResolvedAlert(statefulStateID); got != nil {
		t.Fatal("stateful resolved entry survived cooldown refire")
	}
	if got := m.GetResolvedAlert(aliasStateID); got == nil || got.Alert == nil {
		t.Fatal("canonical alias lookup did not survive concurrent repair and broadcasts")
	}

	// Shutdown must not introduce a reverse-order wait against concurrent
	// resolved reads or cleanup.
	var shutdownWG sync.WaitGroup
	shutdownWG.Add(6)
	for i := 0; i < 2; i++ {
		go func() {
			defer shutdownWG.Done()
			for j := 0; j < 100; j++ {
				m.GetRecentlyResolved()
				m.GetResolvedAlert(aliasStateID)
			}
		}()
	}
	for i := 0; i < 2; i++ {
		go func() {
			defer shutdownWG.Done()
			for j := 0; j < 30; j++ {
				m.Cleanup(time.Hour)
			}
		}()
	}
	for i := 0; i < 2; i++ {
		go func() {
			defer shutdownWG.Done()
			m.Stop()
		}()
	}
	waitForResolvedConcurrencyGroup(t, &shutdownWG)
}

func waitForResolvedConcurrencyGroup(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("resolved alert concurrency paths deadlocked")
	}
}

// GetConfig snapshots are encoded, persisted and edited outside m.mu, and
// UpdateConfig normalizes the maps it is handed in place. While a snapshot
// shared the live config's maps, re-applying one rewrote the override map
// that the GET /api/alerts/config handler was encoding from another snapshot,
// and Go aborted the process with a concurrent map write. Run with -race.
func TestConfigSnapshotsShareNothingWithTheLiveConfig(t *testing.T) {
	originalLogLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(originalLogLevel)
	})

	m := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(m.Stop)

	seed := m.GetConfig()
	seed.Enabled = true
	seed.ActivationState = ActivationActive
	seed.Overrides = make(map[string]ThresholdConfig)
	for i := 0; i < 64; i++ {
		seed.Overrides[fmt.Sprintf("agent:snapshot-%d", i)] = ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
		}
	}
	m.UpdateConfig(seed)

	snapshot := m.GetConfig()
	snapshot.Overrides["agent:snapshot-0"].CPU.Trigger = 1
	delete(snapshot.Overrides, "agent:snapshot-1")
	snapshot.TimeThresholds["guest"] = 99
	snapshot.AgentDefaults.CPU.Trigger = 1

	live := m.GetConfig()
	if got := live.Overrides["agent:snapshot-0"].CPU.Trigger; got != 95 {
		t.Fatalf("editing a snapshot changed the live override trigger to %v", got)
	}
	if _, ok := live.Overrides["agent:snapshot-1"]; !ok {
		t.Fatal("deleting from a snapshot removed the live override")
	}
	if live.TimeThresholds["guest"] == 99 {
		t.Fatal("editing a snapshot changed the live guest delay")
	}
	if live.AgentDefaults.CPU.Trigger == 1 {
		t.Fatal("editing a snapshot changed the live agent CPU trigger")
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			m.UpdateConfig(m.GetConfig())
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := json.Marshal(m.GetConfig()); err != nil {
				t.Errorf("encode snapshot: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			id := fmt.Sprintf("snapshot-%d", i%64)
			m.CheckHost(models.Host{ID: id, Hostname: id, Status: "online", CPUUsage: 50})
		}
	}()
	wg.Wait()

	if got := len(m.GetConfig().Overrides); got != 64 {
		t.Fatalf("live overrides = %d after re-applying snapshots, want 64", got)
	}
}
