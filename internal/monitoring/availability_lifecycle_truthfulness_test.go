package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkgmetrics "github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestPollAvailabilityTargetDropsCancelledProbeWithoutFabricatingFailure(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:               "cancel-check",
		Name:             "Cancel Check",
		Address:          server.URL,
		Protocol:         config.AvailabilityProbeHTTP,
		Enabled:          true,
		PollIntervalSecs: 60,
		TimeoutMillis:    5000,
	})
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.stalenessTracker = NewStalenessTracker(nil)
	store, err := pkgmetrics.NewStore(pkgmetrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	monitor.metricsStore = store

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.pollAvailabilityTarget(ctx, target)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("probe did not reach test HTTP server")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pollAvailabilityTarget did not return after context cancellation")
	}

	if _, exists := monitor.availabilityStatuses[target.ID]; exists {
		t.Fatalf("cancelled probe mutated availabilityStatuses: %+v", monitor.availabilityStatuses[target.ID])
	}
	if _, exists := monitor.pollStatusMap[schedulerKey(InstanceTypeAvailability, target.ID)]; exists {
		t.Fatalf("cancelled probe mutated pollStatusMap: %+v", monitor.pollStatusMap[schedulerKey(InstanceTypeAvailability, target.ID)])
	}
	if _, exists := monitor.state.GetSnapshot().ConnectionHealth[availabilityConnectionKey(target.ID)]; exists {
		t.Fatal("cancelled probe mutated ConnectionHealth")
	}
	rows, _, _, err := store.AvailabilityHistoryRowCounts(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("cancelled probe recorded %d history rows, want 0", rows)
	}
}

func TestPollAvailabilityTargetDropsInFlightResultAfterLifecycleTransition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, monitor *Monitor, target config.AvailabilityTarget)
	}{
		{
			name: "deleted",
			mutate: func(t *testing.T, monitor *Monitor, _ config.AvailabilityTarget) {
				if err := monitor.configPersist.SaveAvailabilityTargets(nil); err != nil {
					t.Fatal(err)
				}
				monitor.RefreshAvailabilityTargets()
			},
		},
		{
			name: "paused",
			mutate: func(t *testing.T, monitor *Monitor, target config.AvailabilityTarget) {
				target.Enabled = false
				if err := monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
					t.Fatal(err)
				}
				monitor.RefreshAvailabilityTargets()
			},
		},
		{
			name: "edited_revision",
			mutate: func(t *testing.T, monitor *Monitor, target config.AvailabilityTarget) {
				target.ConfigRevision++
				if err := monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
					t.Fatal(err)
				}
				monitor.RefreshAvailabilityTargets()
			},
		},
		{
			name: "reassigned_to_remote_agent",
			mutate: func(t *testing.T, monitor *Monitor, target config.AvailabilityTarget) {
				monitor.SetLicenseChecker(licenseWithExternalProbe(true))
				target.ProbeAgentID = "edge-1"
				target.ObservationLocationIDs = []string{config.AvailabilityAgentObservationLocationID("edge-1")}
				if err := monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
					t.Fatal(err)
				}
				monitor.RefreshAvailabilityTargets()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-started:
				default:
					close(started)
				}
				select {
				case <-release:
					w.WriteHeader(http.StatusNoContent)
				case <-r.Context().Done():
				}
			}))
			defer server.Close()

			target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
				ID:               "in-flight-check",
				Name:             "In-flight Check",
				Address:          server.URL,
				Protocol:         config.AvailabilityProbeHTTP,
				Enabled:          true,
				ConfigRevision:   1,
				PollIntervalSecs: 60,
				TimeoutMillis:    5000,
			})
			monitor := newProbeAgentTestMonitor(t, target)
			monitor.stalenessTracker = NewStalenessTracker(nil)
			store, err := pkgmetrics.NewStore(pkgmetrics.DefaultConfig(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			monitor.metricsStore = store
			monitor.RefreshAvailabilityTargets()

			done := make(chan struct{})
			go func() {
				defer close(done)
				monitor.pollAvailabilityTarget(context.Background(), target)
			}()

			select {
			case <-started:
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("probe did not reach test HTTP server")
			}

			tc.mutate(t, monitor, target)
			close(release)

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("pollAvailabilityTarget did not finish after release")
			}

			monitor.mu.RLock()
			_, hasStatus := monitor.availabilityStatuses[target.ID]
			_, hasLocalLocation := monitor.availabilityByLocation[target.ID][config.AvailabilityObservationLocationLocal]
			_, hasPollStatus := monitor.pollStatusMap[schedulerKey(InstanceTypeAvailability, target.ID)]
			monitor.mu.RUnlock()

			if hasStatus {
				t.Fatalf("stale in-flight local probe resurrected availabilityStatuses after %s", tc.name)
			}
			if hasLocalLocation {
				t.Fatalf("stale in-flight local probe wrote local observation after %s", tc.name)
			}
			if hasPollStatus {
				t.Fatalf("stale in-flight local probe recorded pollStatusMap entry after %s", tc.name)
			}
			if _, exists := monitor.state.GetSnapshot().ConnectionHealth[availabilityConnectionKey(target.ID)]; exists {
				t.Fatalf("stale in-flight local probe set ConnectionHealth after %s", tc.name)
			}
			rows, _, _, err := store.AvailabilityHistoryRowCounts(target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if rows != 0 {
				t.Fatalf("stale in-flight local probe wrote %d history rows after %s, want 0", rows, tc.name)
			}
		})
	}
}

func TestApplyProbeAvailabilityResultsRejectsDisabledTargets(t *testing.T) {
	disabled := probeAgentTarget("remote-paused", "agent-1")
	disabled.Enabled = false
	monitor := newProbeAgentTestMonitor(t, disabled)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	monitor.stalenessTracker = NewStalenessTracker(nil)
	store, err := pkgmetrics.NewStore(pkgmetrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	monitor.metricsStore = store

	now := time.Now().UTC()
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: "remote-paused", Outcome: AvailabilityProbeReachable, LatencyMillis: 12, CheckedAt: now},
		{TargetID: "remote-paused", Outcome: AvailabilityProbeUnreachable, Error: "timeout", CheckedAt: now.Add(time.Second)},
	}, now)

	monitor.mu.RLock()
	_, hasStatus := monitor.availabilityStatuses["remote-paused"]
	_, hasLocation := monitor.availabilityByLocation["remote-paused"]
	_, hasPollStatus := monitor.pollStatusMap[schedulerKey(InstanceTypeAvailability, "remote-paused")]
	monitor.mu.RUnlock()

	if hasStatus || hasLocation || hasPollStatus {
		t.Fatalf("disabled target accepted remote probe results: status=%v location=%v pollStatus=%v", hasStatus, hasLocation, hasPollStatus)
	}
	if _, exists := monitor.state.GetSnapshot().ConnectionHealth[availabilityConnectionKey("remote-paused")]; exists {
		t.Fatal("disabled target updated ConnectionHealth from remote probe")
	}
	rows, _, _, err := store.AvailabilityHistoryRowCounts("remote-paused")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("disabled target wrote %d history rows, want 0", rows)
	}
}

func TestPausedProbeTargetDoesNotBecomeStaleAndGetsFreshGraceOnResume(t *testing.T) {
	target := config.NormalizeAvailabilityTarget(probeAgentTarget("remote-pause-resume", "agent-1"))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	alertManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(alertManager.Stop)
	monitor.alertManager = alertManager

	t0 := time.Now().UTC().Add(-time.Hour)
	monitor.applyProbeAvailabilityResultsAt("agent-1", []ProbeAvailabilityResult{
		{TargetID: target.ID, Outcome: AvailabilityProbeReachable, LatencyMillis: 10, CheckedAt: t0},
	}, t0)

	// Pause the check.
	target.Enabled = false
	if err := monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatal(err)
	}
	monitor.RefreshAvailabilityTargets()

	monitor.mu.RLock()
	trackerCount := len(monitor.availabilityProbeTrackers)
	monitor.mu.RUnlock()
	if trackerCount != 0 {
		t.Fatalf("paused target retained %d probe grace trackers, want 0", trackerCount)
	}

	pausedSnapshot := monitor.availabilityStatusSnapshotForTargets([]config.AvailabilityTarget{target}, t0.Add(2*time.Hour))[target.ID]
	if availabilityProbeStatusIsStale(pausedSnapshot) {
		t.Fatalf("paused probe target degraded to stale: %+v", pausedSnapshot)
	}
	monitor.mu.RLock()
	trackerCountAfterRead := len(monitor.availabilityProbeTrackers)
	monitor.mu.RUnlock()
	if trackerCountAfterRead != 0 {
		t.Fatalf("reading paused snapshot created %d probe grace trackers, want 0", trackerCountAfterRead)
	}
	_ = (availabilityPollProvider{}).SupplementalRecords(monitor, "")
	if active := alertManager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("paused target raised alerts: %+v", active)
	}

	// Resume the check: must receive a fresh grace window rather than immediately
	// turning stale or claiming the hour-old pre-pause result is still current.
	target.Enabled = true
	if err := monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatal(err)
	}
	monitor.RefreshAvailabilityTargets()

	resumed := monitor.AvailabilityStatusSnapshot()[target.ID]
	if availabilityProbeStatusIsStale(resumed) {
		t.Fatalf("resumed target immediately degraded to stale without grace window: %+v", resumed)
	}
	if !resumed.LastChecked.IsZero() || resumed.Available {
		t.Fatalf("resumed target carried over pre-pause availability: %+v", resumed)
	}
	_ = (availabilityPollProvider{}).SupplementalRecords(monitor, "")
	if active := alertManager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("resumed target raised immediate stale alert during fresh grace window: %+v", active)
	}

	// Once the fresh grace window elapses with no report, staleness triggers normally.
	afterGrace := monitor.availabilityStatusSnapshotForTargets(
		[]config.AvailabilityTarget{target},
		time.Now().UTC().Add(availabilityProbeStaleFloor+time.Second),
	)[target.ID]
	if !availabilityProbeStatusIsStale(afterGrace) {
		t.Fatalf("resumed target did not become stale after grace window elapsed: %+v", afterGrace)
	}
}

func TestRefreshAvailabilityTargetsPrunesDeletedSchedulerAndStalenessState(t *testing.T) {
	const sharedID = "same-name-target"
	target := config.NormalizeAvailabilityTarget(probeAgentTarget(sharedID, ""))
	monitor := newProbeAgentTestMonitor(t, target)
	monitor.stalenessTracker = NewStalenessTracker(nil)
	monitor.deadLetterQueue = NewTaskQueue()
	monitor.dlqInsightMap = make(map[string]*dlqInsight)
	monitor.instanceInfoCache = make(map[string]*instanceInfo)

	availKey := schedulerKey(InstanceTypeAvailability, sharedID)
	pveKey := schedulerKey(InstanceTypePVE, sharedID)

	// Populate both availability and unrelated PVE peer state under the same name.
	now := time.Now().UTC()
	monitor.applyAvailabilityObservation(target, "obs-1", now, 5*time.Millisecond, AvailabilityProbeReachable, nil, nil, "", time.Time{})
	monitor.mu.Lock()
	monitor.failureCounts[availKey] = 2
	monitor.lastOutcome[availKey] = taskOutcome{success: true, recordedAt: now}
	monitor.circuitBreakers[availKey] = newCircuitBreaker(3, time.Second, time.Minute, time.Second)
	monitor.instanceInfoCache[availKey] = &instanceInfo{}
	monitor.dlqInsightMap[availKey] = &dlqInsight{Reason: "availability-failure", LastAttempt: now}

	monitor.pollStatusMap[pveKey] = &pollStatus{LastSuccess: now}
	monitor.failureCounts[pveKey] = 1
	monitor.lastOutcome[pveKey] = taskOutcome{success: true, recordedAt: now}
	monitor.circuitBreakers[pveKey] = newCircuitBreaker(3, time.Second, time.Minute, time.Second)
	monitor.instanceInfoCache[pveKey] = &instanceInfo{}
	monitor.dlqInsightMap[pveKey] = &dlqInsight{Reason: "pve-failure", LastAttempt: now}
	monitor.mu.Unlock()

	monitor.deadLetterQueue.Upsert(ScheduledTask{InstanceType: InstanceTypeAvailability, InstanceName: sharedID, NextRun: now})
	monitor.deadLetterQueue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: sharedID, NextRun: now})
	monitor.stalenessTracker.UpdateSuccess(InstanceTypePVE, sharedID, nil)
	monitor.setProviderConnectionHealth(InstanceTypePVE, sharedID, true)

	// Delete the availability target and refresh.
	if err := monitor.configPersist.SaveAvailabilityTargets(nil); err != nil {
		t.Fatal(err)
	}
	monitor.RefreshAvailabilityTargets()

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	if _, ok := monitor.availabilityStatuses[sharedID]; ok {
		t.Error("deleted target remained in availabilityStatuses")
	}
	if _, ok := monitor.availabilityByLocation[sharedID]; ok {
		t.Error("deleted target remained in availabilityByLocation")
	}
	if _, ok := monitor.pollStatusMap[availKey]; ok {
		t.Error("deleted target remained in pollStatusMap")
	}
	if _, ok := monitor.failureCounts[availKey]; ok {
		t.Error("deleted target remained in failureCounts")
	}
	if _, ok := monitor.lastOutcome[availKey]; ok {
		t.Error("deleted target remained in lastOutcome")
	}
	if _, ok := monitor.circuitBreakers[availKey]; ok {
		t.Error("deleted target remained in circuitBreakers")
	}
	if _, ok := monitor.instanceInfoCache[availKey]; ok {
		t.Error("deleted target remained in instanceInfoCache")
	}
	if _, ok := monitor.dlqInsightMap[availKey]; ok {
		t.Error("deleted target remained in dlqInsightMap")
	}
	if _, ok := monitor.deadLetterQueue.Get(InstanceTypeAvailability, sharedID); ok {
		t.Error("deleted target remained in deadLetterQueue")
	}
	if _, ok := monitor.stalenessTracker.snapshot(InstanceTypeAvailability, sharedID); ok {
		t.Error("deleted target remained in stalenessTracker")
	}
	if _, ok := monitor.state.GetSnapshot().ConnectionHealth[availabilityConnectionKey(sharedID)]; ok {
		t.Error("deleted target remained in ConnectionHealth")
	}

	// Unrelated PVE peer with the exact same ID must remain intact.
	if _, ok := monitor.pollStatusMap[pveKey]; !ok {
		t.Error("deleting availability target removed unrelated PVE pollStatusMap entry")
	}
	if _, ok := monitor.failureCounts[pveKey]; !ok {
		t.Error("deleting availability target removed unrelated PVE failureCounts entry")
	}
	if _, ok := monitor.lastOutcome[pveKey]; !ok {
		t.Error("deleting availability target removed unrelated PVE lastOutcome entry")
	}
	if _, ok := monitor.circuitBreakers[pveKey]; !ok {
		t.Error("deleting availability target removed unrelated PVE circuitBreakers entry")
	}
	if _, ok := monitor.dlqInsightMap[pveKey]; !ok {
		t.Error("deleting availability target removed unrelated PVE dlqInsightMap entry")
	}
	if _, ok := monitor.deadLetterQueue.Get(InstanceTypePVE, sharedID); !ok {
		t.Error("deleting availability target removed unrelated PVE deadLetterQueue entry")
	}
	if _, ok := monitor.stalenessTracker.snapshot(InstanceTypePVE, sharedID); !ok {
		t.Error("deleting availability target removed unrelated PVE stalenessTracker entry")
	}
}

func TestAvailabilityConcurrentPollAndRefreshIsRaceFree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	localTarget := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:               "race-local",
		Name:             "Race Local",
		Address:          server.URL,
		Protocol:         config.AvailabilityProbeHTTP,
		Enabled:          true,
		ConfigRevision:   1,
		PollIntervalSecs: 60,
		TimeoutMillis:    1000,
	})
	remoteTarget := config.NormalizeAvailabilityTarget(probeAgentTarget("race-remote", "agent-1"))
	monitor := newProbeAgentTestMonitor(t, localTarget, remoteTarget)
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))
	monitor.stalenessTracker = NewStalenessTracker(nil)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			monitor.pollAvailabilityTarget(context.Background(), localTarget)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			monitor.ApplyProbeAvailabilityResults("agent-1", []ProbeAvailabilityResult{
				{TargetID: remoteTarget.ID, Outcome: AvailabilityProbeReachable, LatencyMillis: 5, CheckedAt: time.Now().UTC()},
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			l := localTarget
			r := remoteTarget
			l.Enabled = i%2 == 0
			r.Enabled = i%2 == 0
			_ = monitor.configPersist.SaveAvailabilityTargets([]config.AvailabilityTarget{l, r})
			monitor.RefreshAvailabilityTargets()
		}
	}()
	wg.Wait()
}
