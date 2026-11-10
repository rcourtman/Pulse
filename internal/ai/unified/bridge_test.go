package unified

import (
	"testing"
	"time"
)

type stubAlertProvider struct {
	alerts     []AlertAdapter
	alertCb    func(AlertAdapter)
	resolvedCb func(string)
}

func (s *stubAlertProvider) GetActiveAlerts() []AlertAdapter {
	return s.alerts
}

func (s *stubAlertProvider) GetAlert(alertID string) AlertAdapter {
	for _, alert := range s.alerts {
		if alert.GetAlertIdentifier() == alertID {
			return alert
		}
	}
	return nil
}

func (s *stubAlertProvider) SetAlertCallback(cb func(AlertAdapter)) {
	s.alertCb = cb
}

func (s *stubAlertProvider) SetResolvedCallback(cb func(alertID string)) {
	s.resolvedCb = cb
}

func TestAlertBridge_StartStopAndSync(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-1",
		ResourceName:    "web",
		Value:           90,
		Threshold:       80,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	provider := &stubAlertProvider{alerts: []AlertAdapter{alert}}
	bridge.SetAlertProvider(provider)

	bridge.Start()
	stats := bridge.Stats()
	if !stats.Running {
		t.Fatalf("expected bridge running")
	}
	if store.GetByAlertIdentifier("alert-1") == nil {
		t.Fatalf("expected alert to be synced into store")
	}

	bridge.Stop()
	stats = bridge.Stats()
	if stats.Running {
		t.Fatalf("expected bridge stopped")
	}
}

func TestAlertBridge_HandleNewAlertAndEnhance(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, BridgeConfig{
		AutoEnhance:        true,
		EnhanceDelay:       10 * time.Millisecond,
		TriggerPatrolOnNew: true,
	})
	bridge.running = true

	patrolCh := make(chan string, 1)
	bridge.SetPatrolTrigger(func(event PatrolTriggerEvent) {
		patrolCh <- event.Reason
	})

	enhanceCh := make(chan string, 1)
	bridge.SetAIEnhancement(func(findingID string) error {
		enhanceCh <- findingID
		return nil
	})

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-1",
		ResourceName:    "web",
		Value:           90,
		Threshold:       80,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}

	bridge.handleNewAlert(alert)

	select {
	case reason := <-patrolCh:
		if reason != "alert_fired" {
			t.Fatalf("expected patrol reason alert_fired, got %s", reason)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected patrol trigger")
	}

	select {
	case <-enhanceCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected enhancement callback")
	}
}

func TestAlertBridge_HandleAlertResolved(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, BridgeConfig{
		TriggerPatrolOnClear: true,
	})
	bridge.running = true

	patrolCh := make(chan string, 1)
	bridge.SetPatrolTrigger(func(event PatrolTriggerEvent) {
		patrolCh <- event.Reason
	})

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-1",
		ResourceName:    "web",
		Value:           90,
		Threshold:       80,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	bridge.pendingEnhancements[finding.ID] = time.AfterFunc(time.Second, func() {})

	bridge.handleAlertResolved(alert.AlertIdentifier)

	select {
	case reason := <-patrolCh:
		if reason != "alert_cleared" {
			t.Fatalf("expected patrol reason alert_cleared, got %s", reason)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected patrol trigger")
	}

	if _, ok := bridge.pendingEnhancements[finding.ID]; ok {
		t.Fatalf("expected enhancement to be canceled")
	}
}

func TestAlertBridge_ScheduleEnhancementInactiveFinding(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-1",
		ResourceName:    "web",
		Value:           90,
		Threshold:       80,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	store.Resolve(finding.ID)

	enhanceCh := make(chan string, 1)
	bridge.scheduleEnhancement(finding.ID, 10*time.Millisecond, func(id string) error {
		enhanceCh <- id
		return nil
	})

	select {
	case <-enhanceCh:
		t.Fatalf("did not expect enhancement on inactive finding")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAlertBridge_StatsAndStore(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	bridge := NewAlertBridge(store, DefaultBridgeConfig())
	bridge.running = true
	bridge.pendingEnhancements["f1"] = time.AfterFunc(time.Second, func() {})

	stats := bridge.Stats()
	if !stats.Running {
		t.Fatalf("expected running stats")
	}
	if stats.PendingEnhancements != 1 {
		t.Fatalf("expected pending enhancements to be 1")
	}
	if bridge.GetUnifiedStore() != store {
		t.Fatalf("expected store to be returned")
	}
}

// triggerRecorder collects alert bridge trigger events. The bridge calls the
// trigger on its own goroutine, so tests wait for each one.
type triggerRecorder struct {
	events chan PatrolTriggerEvent
}

func newTriggerRecorder() *triggerRecorder {
	return &triggerRecorder{events: make(chan PatrolTriggerEvent, 1024)}
}

func (r *triggerRecorder) record(event PatrolTriggerEvent) {
	r.events <- event
}

func (r *triggerRecorder) expect(t *testing.T, reason string) PatrolTriggerEvent {
	t.Helper()
	select {
	case event := <-r.events:
		if event.Reason != reason {
			t.Fatalf("expected %s trigger, got %s", reason, event.Reason)
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatalf("expected %s trigger, got none", reason)
	}
	return PatrolTriggerEvent{}
}

func (r *triggerRecorder) expectNone(t *testing.T) {
	t.Helper()
	select {
	case event := <-r.events:
		t.Fatalf("expected no trigger, got %s for %s", event.Reason, event.AlertIdentifier)
	case <-time.After(100 * time.Millisecond):
	}
}

func newEpisodeTestBridge(t *testing.T) (*AlertBridge, *triggerRecorder, *time.Time) {
	t.Helper()
	bridge := NewAlertBridge(NewUnifiedStore(DefaultAlertToFindingConfig()), BridgeConfig{
		TriggerPatrolOnNew:   true,
		TriggerPatrolOnClear: true,
	})
	bridge.running = true
	clock := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	bridge.now = func() time.Time { return clock }
	recorder := newTriggerRecorder()
	bridge.SetPatrolTrigger(recorder.record)
	return bridge, recorder, &clock
}

func episodeTestAlert(level string, startedAt time.Time) *SimpleAlertAdapter {
	return &SimpleAlertAdapter{
		AlertIdentifier: "docker-container-health-host1/app",
		AlertType:       "docker-container-health",
		AlertLevel:      level,
		ResourceID:      "host1/app",
		ResourceName:    "app",
		StartTime:       startedAt,
		LastSeen:        startedAt,
	}
}

func TestAlertBridge_EveryFiringEpisodeTriggersPatrol(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)

	// First episode.
	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expect(t, "alert_fired")

	// The same episode notified again must not wake Patrol twice.
	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expectNone(t)

	bridge.handleAlertResolved("docker-container-health-host1/app")
	recorder.expect(t, "alert_cleared")

	// The alert identifier is the same, so the store reopens its finding.
	// That reopening is a new episode and must wake Patrol again.
	*clock = clock.Add(10 * time.Minute)
	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	event := recorder.expect(t, "alert_fired")
	if event.AlertIdentifier != "docker-container-health-host1/app" || event.AlertLevel != "critical" {
		t.Fatalf("unexpected repeat trigger payload: %+v", event)
	}
}

func TestAlertBridge_FlappingAlertTriggersAreCapped(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	start := *clock

	flap := func() {
		t.Helper()
		bridge.handleNewAlert(episodeTestAlert("critical", *clock))
		bridge.handleAlertResolved("docker-container-health-host1/app")
	}
	countFired := func() int {
		t.Helper()
		fired := 0
		deadline := time.After(300 * time.Millisecond)
		for {
			select {
			case event := <-recorder.events:
				if event.Reason == "alert_fired" {
					fired++
				}
			case <-deadline:
				return fired
			}
		}
	}

	// Twenty flaps in twenty minutes: only the hourly allowance gets through.
	for i := 0; i < 20; i++ {
		flap()
		*clock = clock.Add(time.Minute)
	}
	if fired := countFired(); fired != maxAlertFiredTriggersPerHour {
		t.Fatalf("first hour: expected %d alert_fired triggers, got %d", maxAlertFiredTriggersPerHour, fired)
	}

	// A flap every ten minutes for the rest of the day: the daily cap holds.
	for clock.Before(start.Add(23 * time.Hour)) {
		flap()
		*clock = clock.Add(10 * time.Minute)
	}
	if fired := countFired(); fired != maxAlertFiredTriggersPerDay-maxAlertFiredTriggersPerHour {
		t.Fatalf("rest of day: expected %d more alert_fired triggers, got %d",
			maxAlertFiredTriggersPerDay-maxAlertFiredTriggersPerHour, fired)
	}

	// Once the window has passed the alert can wake Patrol again.
	*clock = start.Add(49 * time.Hour)
	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expect(t, "alert_fired")
}

func TestAlertBridge_SeverityRiseTriggersOnceThePolicyAcceptsIt(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	// The default operator policy: only critical alerts wake Patrol.
	bridge.SetPatrolTriggerPolicy(func(alertType, alertLevel string) bool {
		return alertLevel == "critical"
	})

	bridge.handleNewAlert(episodeTestAlert("warning", *clock))
	recorder.expectNone(t)

	// Same episode, now critical: this is the first dispatch the policy
	// accepts, so it wakes Patrol.
	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expect(t, "alert_fired")

	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expectNone(t)

	// A refused warning must not use up the cap.
	bridge.mu.RLock()
	spent := len(bridge.episodes["docker-container-health-host1/app"].triggers)
	bridge.mu.RUnlock()
	if spent != 1 {
		t.Fatalf("expected one spent trigger, got %d", spent)
	}
}

func TestAlertBridge_AlertActiveAtStartDoesNotTriggerUntilItChanges(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.running = false
	existing := episodeTestAlert("warning", clock.Add(-time.Hour))
	bridge.SetAlertProvider(&stubAlertProvider{alerts: []AlertAdapter{existing}})
	bridge.Start()
	t.Cleanup(bridge.Stop)

	// A repeat notification for the alert that was already active.
	bridge.handleNewAlert(existing)
	recorder.expectNone(t)

	// It rises to critical: that is new and wakes Patrol.
	bridge.handleNewAlert(episodeTestAlert("critical", clock.Add(-time.Hour)))
	recorder.expect(t, "alert_fired")
}

func TestAlertBridge_NewOccurrenceTriggersWhenTheResolveWasMissed(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)

	bridge.handleNewAlert(episodeTestAlert("critical", *clock))
	recorder.expect(t, "alert_fired")

	// No resolve arrives, but the alert manager reports a later start time:
	// the earlier occurrence ended and a new one began.
	bridge.handleNewAlert(episodeTestAlert("critical", clock.Add(30*time.Minute)))
	recorder.expect(t, "alert_fired")
}

func sustainedTestAlert(identifier, level string, startedAt time.Time) *SimpleAlertAdapter {
	return &SimpleAlertAdapter{
		AlertIdentifier: identifier,
		AlertType:       "disk",
		AlertLevel:      level,
		ResourceID:      "resource-" + identifier,
		ResourceName:    identifier,
		Value:           92,
		Threshold:       90,
		StartTime:       startedAt,
		LastSeen:        startedAt,
	}
}

// criticalOnly is the default operator policy for alerts the moment they fire.
func criticalOnly(_, alertLevel string) bool { return alertLevel == "critical" }

func alwaysRunSustained(PatrolTriggerEvent, time.Time) SustainedTriggerDecision {
	return SustainedTriggerRun
}

func TestAlertBridge_SustainedWarningWakesPatrolOnceAfterTheDelay(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.SetPatrolTriggerPolicy(criticalOnly)
	bridge.SetSustainedTriggerPolicy(alwaysRunSustained)
	start := *clock

	bridge.handleNewAlert(sustainedTestAlert("disk-data", "warning", start))
	recorder.expectNone(t)

	*clock = start.Add(sustainedAlertDelay - time.Minute)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)

	*clock = start.Add(sustainedAlertDelay)
	bridge.checkSustainedAlerts()
	event := recorder.expect(t, "alert_fired")
	if !event.Sustained || !event.ActiveSince.Equal(start) || event.AlertLevel != "warning" || event.AlertIdentifier != "disk-data" {
		t.Fatalf("unexpected sustained trigger: %+v", event)
	}

	// Once per episode, however long the alert stays.
	*clock = start.Add(6 * time.Hour)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)

	// The next episode gets its own look.
	bridge.handleAlertResolved("disk-data")
	recorder.expect(t, "alert_cleared")
	second := start.Add(7 * time.Hour)
	*clock = second
	bridge.handleNewAlert(sustainedTestAlert("disk-data", "warning", second))
	*clock = second.Add(sustainedAlertDelay)
	bridge.checkSustainedAlerts()
	recorder.expect(t, "alert_fired")
}

func TestAlertBridge_SustainedTriggerLeavesShortAndAlreadyHandledAlertsAlone(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.SetPatrolTriggerPolicy(criticalOnly)
	bridge.SetSustainedTriggerPolicy(alwaysRunSustained)
	start := *clock

	// A warning that clears inside the delay never wakes Patrol.
	bridge.handleNewAlert(sustainedTestAlert("blip", "warning", start))
	*clock = start.Add(5 * time.Minute)
	bridge.handleAlertResolved("blip")
	recorder.expect(t, "alert_cleared")

	// A critical alert woke Patrol when it fired. It is not looked at twice.
	bridge.handleNewAlert(sustainedTestAlert("down", "critical", *clock))
	recorder.expect(t, "alert_fired")

	*clock = start.Add(time.Hour)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
}

func TestAlertBridge_SustainedTriggerIsOffWithoutAPolicy(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.SetPatrolTriggerPolicy(criticalOnly)

	bridge.handleNewAlert(sustainedTestAlert("disk-data", "warning", *clock))
	*clock = clock.Add(time.Hour)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
}

// Seven warnings active at once, as on the maintainer's own Pulse: the
// sustained trigger spreads them at four an hour across all alerts.
func TestAlertBridge_SustainedTriggersAreCappedAcrossAlerts(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.SetPatrolTriggerPolicy(criticalOnly)
	bridge.SetSustainedTriggerPolicy(alwaysRunSustained)
	start := *clock

	names := []string{"smart-sdf", "pbs-datastore", "unraid-parity", "exited-test", "disk-data", "update-qbittorrent", "update-gluetun"}
	for i, name := range names {
		bridge.handleNewAlert(sustainedTestAlert(name, "warning", start.Add(time.Duration(i)*time.Second)))
	}
	recorder.expectNone(t)

	countSustained := func() []string {
		t.Helper()
		var fired []string
		deadline := time.After(300 * time.Millisecond)
		for {
			select {
			case event := <-recorder.events:
				if event.Reason == "alert_fired" && event.Sustained {
					fired = append(fired, event.AlertIdentifier)
				}
			case <-deadline:
				return fired
			}
		}
	}

	*clock = start.Add(sustainedAlertDelay + time.Minute)
	bridge.checkSustainedAlerts()
	bridge.checkSustainedAlerts()
	if fired := countSustained(); len(fired) != maxSustainedTriggersPerHour {
		t.Fatalf("first hour: expected %d sustained triggers, got %v", maxSustainedTriggersPerHour, fired)
	}

	*clock = start.Add(sustainedAlertDelay + 30*time.Minute)
	bridge.checkSustainedAlerts()
	if fired := countSustained(); len(fired) != 0 {
		t.Fatalf("expected the cap to hold inside the hour, got %v", fired)
	}

	// The rest follow when the hour has passed, and nothing fires after that.
	*clock = start.Add(sustainedAlertDelay + 62*time.Minute)
	bridge.checkSustainedAlerts()
	if fired := countSustained(); len(fired) != len(names)-maxSustainedTriggersPerHour {
		t.Fatalf("second hour: expected %d sustained triggers, got %v", len(names)-maxSustainedTriggersPerHour, fired)
	}
	*clock = start.Add(24 * time.Hour)
	bridge.checkSustainedAlerts()
	if fired := countSustained(); len(fired) != 0 {
		t.Fatalf("every alert was looked at once already, got %v", fired)
	}
}

func TestAlertBridge_SustainedPolicyCanWaitOrSkip(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.SetPatrolTriggerPolicy(criticalOnly)
	decision := SustainedTriggerWait
	asked := 0
	bridge.SetSustainedTriggerPolicy(func(PatrolTriggerEvent, time.Time) SustainedTriggerDecision {
		asked++
		return decision
	})
	start := *clock
	bridge.handleNewAlert(sustainedTestAlert("disk-data", "warning", start))
	bridge.handleNewAlert(sustainedTestAlert("disk-media", "warning", start))

	// Wait: Patrol is in Watch only, say. Asked again on every check.
	*clock = start.Add(sustainedAlertDelay)
	bridge.checkSustainedAlerts()
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
	if asked != 4 {
		t.Fatalf("expected both alerts to be asked about on both checks, got %d questions", asked)
	}

	// Skip: a run already looked at this episode. Never asked again.
	decision = SustainedTriggerSkip
	bridge.checkSustainedAlerts()
	decision = SustainedTriggerRun
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
	if asked != 6 {
		t.Fatalf("expected skipped alerts to be left alone, got %d questions", asked)
	}
}

// An alert that was already active when Pulse started has not woken Patrol in
// this process. The sustained trigger looks at it once, after the same delay
// counted from the start so the inventory has loaded.
func TestAlertBridge_SustainedTriggerLooksAtAlertsActiveAtStart(t *testing.T) {
	bridge, recorder, clock := newEpisodeTestBridge(t)
	bridge.running = false
	bridge.SetPatrolTriggerPolicy(criticalOnly)
	bridge.SetSustainedTriggerPolicy(alwaysRunSustained)
	start := *clock
	old := sustainedTestAlert("local-zfs", "critical", start.Add(-17*24*time.Hour))
	bridge.SetAlertProvider(&stubAlertProvider{alerts: []AlertAdapter{old}})
	bridge.Start()
	t.Cleanup(bridge.Stop)

	*clock = start.Add(time.Minute)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)

	*clock = start.Add(sustainedAlertDelay)
	bridge.checkSustainedAlerts()
	event := recorder.expect(t, "alert_fired")
	if !event.Sustained || event.AlertIdentifier != "local-zfs" || event.AlertLevel != "critical" {
		t.Fatalf("unexpected sustained trigger: %+v", event)
	}
	*clock = start.Add(time.Hour)
	bridge.checkSustainedAlerts()
	recorder.expectNone(t)
}
