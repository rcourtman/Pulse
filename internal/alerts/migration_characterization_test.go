package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func characterizationBaseConfig() AlertConfig {
	return AlertConfig{
		Enabled:         true,
		ActivationState: ActivationActive,
		GuestDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		AgentDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		StorageDefault: HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides:      map[string]ThresholdConfig{},
		TimeThresholds: map[string]int{},
	}
}

func newCharacterizationManager(t *testing.T, cfg AlertConfig) *Manager {
	t.Helper()

	m := newTestManager(t)
	m.UpdateConfig(cfg)

	// Force deterministic immediate evaluation for characterization tests.
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.MetricTimeThresholds = nil
	m.mu.Unlock()

	m.ClearActiveAlerts()
	return m
}

func activeAlert(t *testing.T, m *Manager, alertID string) *Alert {
	t.Helper()

	m.mu.RLock()
	alert := testRequireActiveAlert(t, m, alertID)
	m.mu.RUnlock()
	if alert != nil {
		return alert.Clone()
	}

	t.Fatalf("expected active alert %q, got active alerts %v", alertID, alertKeys(m))
	return nil
}

func testVM(resourceID string, vmID int, name, node, instance, status string, cpu float64, tags ...string) models.VM {
	return models.VM{
		ID:       resourceID,
		VMID:     vmID,
		Name:     name,
		Node:     node,
		Instance: instance,
		Status:   status,
		CPU:      cpu,
		Tags:     tags,
	}
}

func TestAlertCharacterizationCanonicalGuestIdentityAcrossTypedAndUnifiedChecks(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"

	m := newCharacterizationManager(t, characterizationBaseConfig())
	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.85), "pve1")
	typedAlert := activeAlert(t, m, alertID)

	m.ClearActiveAlerts()
	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 85},
	})
	unifiedAlert := activeAlert(t, m, alertID)

	if typedAlert.ResourceID != resourceID {
		t.Fatalf("typed ResourceID = %q, want %q", typedAlert.ResourceID, resourceID)
	}
	if unifiedAlert.ResourceID != resourceID {
		t.Fatalf("unified ResourceID = %q, want %q", unifiedAlert.ResourceID, resourceID)
	}
	if typedAlert.ID != alertID || unifiedAlert.ID != alertID {
		t.Fatalf("expected stable alert ID %q, got typed=%q unified=%q", alertID, typedAlert.ID, unifiedAlert.ID)
	}
	if typedAlert.CanonicalSpecID != alertID || unifiedAlert.CanonicalSpecID != alertID {
		t.Fatalf("expected canonical spec id %q, got typed=%q unified=%q", alertID, typedAlert.CanonicalSpecID, unifiedAlert.CanonicalSpecID)
	}
	if typedAlert.CanonicalState != resourceID+"::"+alertID || unifiedAlert.CanonicalState != resourceID+"::"+alertID {
		t.Fatalf("expected canonical state %q, got typed=%q unified=%q", resourceID+"::"+alertID, typedAlert.CanonicalState, unifiedAlert.CanonicalState)
	}
}

func TestAlertCharacterizationAcknowledgmentSurvivesAlertIDChangeForSameCanonicalState(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	oldAlertID := "legacy-" + resourceID + "-cpu"
	newAlertID := resourceID + "-cpu"

	m := newCharacterizationManager(t, characterizationBaseConfig())

	m.mu.Lock()
	oldAlert := &Alert{
		ID:         oldAlertID,
		Type:       "cpu",
		Level:      AlertLevelWarning,
		ResourceID: resourceID,
		Message:    "legacy alert",
		StartTime:  time.Now().Add(-5 * time.Minute),
		LastSeen:   time.Now().Add(-1 * time.Minute),
	}
	applyCanonicalIdentity(oldAlert, newAlertID, "metric-threshold")
	m.activeAlerts[oldAlertID] = oldAlert
	m.mu.Unlock()

	if err := m.AcknowledgeAlert(oldAlertID, "alice"); err != nil {
		t.Fatalf("AcknowledgeAlert() error = %v", err)
	}

	m.mu.Lock()
	m.removeActiveAlertNoLock(oldAlertID)
	replacement := &Alert{
		ID:         newAlertID,
		Type:       "cpu",
		Level:      AlertLevelWarning,
		ResourceID: resourceID,
		Message:    "canonical alert",
		StartTime:  time.Now(),
		LastSeen:   time.Now(),
	}
	applyCanonicalIdentity(replacement, newAlertID, "metric-threshold")
	m.preserveAlertState(newAlertID, replacement)
	m.activeAlerts[newAlertID] = replacement
	m.mu.Unlock()

	alert := activeAlert(t, m, newAlertID)
	if !alert.Acknowledged {
		t.Fatal("expected acknowledgment to survive alert ID change")
	}
	if alert.AckUser != "alice" {
		t.Fatalf("AckUser = %q, want alice", alert.AckUser)
	}
	if alert.CanonicalState != resourceID+"::"+newAlertID {
		t.Fatalf("CanonicalState = %q, want %q", alert.CanonicalState, resourceID+"::"+newAlertID)
	}
}

func TestAlertCharacterizationHistoryUpdateUsesCanonicalStateAcrossAlertIDChange(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	oldAlertID := "legacy-" + resourceID + "-cpu"
	newAlertID := resourceID + "-cpu"
	lastSeen := time.Now()

	m := newCharacterizationManager(t, characterizationBaseConfig())

	oldHistoryAlert := Alert{
		ID:         oldAlertID,
		Type:       "cpu",
		ResourceID: resourceID,
		StartTime:  time.Now().Add(-10 * time.Minute),
		LastSeen:   time.Now().Add(-5 * time.Minute),
	}
	applyCanonicalIdentity(&oldHistoryAlert, newAlertID, "metric-threshold")
	m.historyManager.AddAlert(oldHistoryAlert)

	current := &Alert{
		ID:         newAlertID,
		Type:       "cpu",
		ResourceID: resourceID,
		StartTime:  time.Now().Add(-2 * time.Minute),
		LastSeen:   lastSeen,
	}
	applyCanonicalIdentity(current, newAlertID, "metric-threshold")

	m.historyManager.UpdateAlertLastSeenForAlert(current, lastSeen)

	history := m.GetAlertHistory(10)
	if len(history) == 0 {
		t.Fatalf("expected history entry")
	}
	if !history[0].LastSeen.Equal(lastSeen) {
		t.Fatalf("LastSeen = %v, want %v", history[0].LastSeen, lastSeen)
	}
	if history[0].ID != oldAlertID {
		t.Fatalf("history alert ID = %q, want legacy ID %q preserved", history[0].ID, oldAlertID)
	}
}

func TestAlertCharacterizationRecentSuppressionSurvivesAlertIDChangeForSameCanonicalState(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	oldAlertID := "legacy-" + resourceID + "-cpu"
	newAlertID := resourceID + "-cpu"
	clear := 75.0
	critical := 90.0

	m := newCharacterizationManager(t, characterizationBaseConfig())
	m.mu.Lock()
	m.config.MinimumDelta = 5
	m.config.SuppressionWindow = 10
	recent := &Alert{
		ID:         oldAlertID,
		Type:       "cpu",
		Level:      AlertLevelWarning,
		ResourceID: resourceID,
		Value:      90,
		StartTime:  time.Now().Add(-1 * time.Minute),
		LastSeen:   time.Now().Add(-30 * time.Second),
	}
	applyCanonicalIdentity(recent, newAlertID, "metric-threshold")
	m.recentAlerts[recent.CanonicalState] = recent
	m.mu.Unlock()

	spec := alertspecs.ResourceAlertSpec{
		ID:           newAlertID,
		ResourceID:   resourceID,
		ResourceType: unifiedresources.ResourceTypeVM,
		Kind:         alertspecs.AlertSpecKindMetricThreshold,
		Severity:     alertspecs.AlertSeverityWarning,
		MetricThreshold: &alertspecs.MetricThresholdSpec{
			Metric:    "cpu",
			Direction: alertspecs.ThresholdDirectionAbove,
			Trigger:   80,
			Recovery:  &clear,
			Critical:  &critical,
		},
	}

	m.evaluateCanonicalMetricAlert(spec, "app01", "node1", "pve1", "cpu", 91, &HysteresisThreshold{Trigger: 80, Clear: 75}, nil)

	assertAlertMissing(t, m, newAlertID)

	m.mu.RLock()
	suppressedUntil, ok := m.suppressedUntil[resourceID+"::"+newAlertID]
	m.mu.RUnlock()
	if !ok {
		t.Fatal("expected canonical suppression window to be recorded")
	}
	if time.Until(suppressedUntil) <= 0 {
		t.Fatal("expected canonical suppression window to be in the future")
	}
}

func TestAlertCharacterizationAcknowledgeByCanonicalStateAlias(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"
	canonicalState := buildCanonicalStateID(resourceID, alertID)

	m := newCharacterizationManager(t, characterizationBaseConfig())
	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.95), "pve1")

	if err := m.AcknowledgeAlert(canonicalState, "alice"); err != nil {
		t.Fatalf("AcknowledgeAlert(%q) error = %v", canonicalState, err)
	}

	alert := activeAlert(t, m, alertID)
	if !alert.Acknowledged {
		t.Fatal("expected alert to be acknowledged through canonical state alias")
	}
	if alert.AckUser != "alice" {
		t.Fatalf("AckUser = %q, want alice", alert.AckUser)
	}

	m.mu.RLock()
	_, legacyAck := m.ackState[alertID]
	record, canonicalAck := m.ackStateByCanonical[canonicalState]
	m.mu.RUnlock()
	if legacyAck {
		t.Fatalf("expected canonical alert acknowledgment to be keyed by canonical state, not legacy alert ID")
	}
	if !canonicalAck || !record.acknowledged || record.user != "alice" {
		t.Fatalf("expected canonical ack record for %q, got %+v exists=%t", canonicalState, record, canonicalAck)
	}
}

func TestAlertCharacterizationGuestThresholdPrecedence(t *testing.T) {
	overrideGuestID := BuildGuestKey("pve1", "node1", 101)
	ruleGuestID := BuildGuestKey("pve1", "node1", 102)

	cfg := characterizationBaseConfig()
	cfg.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 90, Clear: 85}
	cfg.CustomRules = []CustomAlertRule{
		{
			Name:     "named-apps",
			Enabled:  true,
			Priority: 10,
			FilterConditions: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{Type: "text", Field: "name", Value: "app"},
				},
			},
			Thresholds: ThresholdConfig{
				CPU: &HysteresisThreshold{Trigger: 70, Clear: 65},
			},
		},
	}
	cfg.Overrides[overrideGuestID] = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 95, Clear: 90},
	}

	m := newCharacterizationManager(t, cfg)
	m.CheckGuest(testVM(ruleGuestID, 102, "app-rule", "node1", "pve1", "running", 0.80), "pve1")
	m.CheckGuest(testVM(overrideGuestID, 101, "app-override", "node1", "pve1", "running", 0.80), "pve1")

	assertAlertPresent(t, m, ruleGuestID+"-cpu")
	assertAlertMissing(t, m, overrideGuestID+"-cpu")
}

func TestAlertCharacterizationDisableConnectivitySuppressesPoweredOffButNotMetrics(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	cfg := characterizationBaseConfig()
	cfg.Overrides[resourceID] = ThresholdConfig{DisableConnectivity: true}

	m := newCharacterizationManager(t, cfg)
	stopped := testVM(resourceID, 101, "app01", "node1", "pve1", "stopped", 0)

	m.CheckGuest(stopped, "pve1")
	m.CheckGuest(stopped, "pve1")

	assertAlertMissing(t, m, "guest-powered-off-"+resourceID)

	m.mu.RLock()
	_, hasConfirmations := m.offlineConfirmations[resourceID]
	m.mu.RUnlock()
	if hasConfirmations {
		t.Fatalf("expected powered-off tracking to stay clear when connectivity is disabled")
	}

	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.95), "pve1")
	assertAlertPresent(t, m, resourceID+"-cpu")
}

func TestAlertCharacterizationReevaluatesAlertsWhenConfigChanges(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"
	cfg := characterizationBaseConfig()

	m := newCharacterizationManager(t, cfg)
	resolved := make(chan string, 1)
	m.SetResolvedCallback(func(id string) {
		select {
		case resolved <- id:
		default:
		}
	})

	m.CheckUnifiedResource(&UnifiedResourceInput{
		ID:       resourceID,
		Type:     "vm",
		Name:     "app01",
		Node:     "node1",
		Instance: "pve1",
		CPU:      &UnifiedResourceMetric{Percent: 87},
	})
	assertAlertPresent(t, m, alertID)

	updated := cfg
	updated.GuestDefaults.CPU = &HysteresisThreshold{Trigger: 90, Clear: 85}
	m.UpdateConfig(updated)

	select {
	case got := <-resolved:
		if got != alertID {
			t.Fatalf("resolved callback = %q, want %q", got, alertID)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("expected alert %q to resolve after config change", alertID)
	}

	assertAlertMissing(t, m, alertID)

	m.resolvedMutex.RLock()
	_, wasResolved := testLookupResolvedAlert(t, m, alertID)
	m.resolvedMutex.RUnlock()
	if !wasResolved {
		t.Fatalf("expected %q in recently resolved after config change", alertID)
	}
}

func TestAlertCharacterizationPoweredOffLifecycle(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := "guest-powered-off-" + resourceID
	cfg := characterizationBaseConfig()
	m := newCharacterizationManager(t, cfg)

	running := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.95)
	stopped := testVM(resourceID, 101, "app01", "node1", "pve1", "stopped", 0)
	paused := testVM(resourceID, 101, "app01", "node1", "pve1", "paused", 0)

	m.CheckGuest(running, "pve1")
	assertAlertPresent(t, m, resourceID+"-cpu")

	m.CheckGuest(stopped, "pve1")
	m.CheckGuest(stopped, "pve1")

	assertAlertMissing(t, m, resourceID+"-cpu")
	assertAlertPresent(t, m, alertID)

	m.CheckGuest(paused, "pve1")
	assertAlertMissing(t, m, alertID)

	m.CheckGuest(stopped, "pve1")
	m.CheckGuest(stopped, "pve1")
	assertAlertPresent(t, m, alertID)

	m.CheckGuest(running, "pve1")
	assertAlertMissing(t, m, alertID)
}

func TestAlertCharacterizationAcknowledgmentSurvivesPoweredOffReappearance(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := "guest-powered-off-" + resourceID
	m := newCharacterizationManager(t, characterizationBaseConfig())

	stopped := testVM(resourceID, 101, "app01", "node1", "pve1", "stopped", 0)
	running := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.10)

	m.CheckGuest(stopped, "pve1")
	m.CheckGuest(stopped, "pve1")
	if err := m.AcknowledgeAlert(alertID, "alice"); err != nil {
		t.Fatalf("AcknowledgeAlert(%q) failed: %v", alertID, err)
	}

	acknowledged := activeAlert(t, m, alertID)
	if acknowledged.AckTime == nil {
		t.Fatalf("expected acknowledged alert to record ack time")
	}

	m.CheckGuest(running, "pve1")
	assertAlertMissing(t, m, alertID)

	m.CheckGuest(stopped, "pve1")
	m.CheckGuest(stopped, "pve1")

	reappeared := activeAlert(t, m, alertID)
	if !reappeared.Acknowledged {
		t.Fatalf("expected acknowledgment to be restored when the same powered-off identity reappears")
	}
	if reappeared.AckUser != "alice" {
		t.Fatalf("AckUser = %q, want %q", reappeared.AckUser, "alice")
	}
	if reappeared.AckTime == nil || !reappeared.AckTime.Equal(*acknowledged.AckTime) {
		t.Fatalf("expected AckTime to be preserved across clear/recreate, got %v want %v", reappeared.AckTime, acknowledged.AckTime)
	}
}

func TestAlertCharacterizationSuppressTagClearsMetricSuppressionIdentity(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"
	trackingKey := buildCanonicalStateID(resourceID, alertID)
	cfg := characterizationBaseConfig()
	cfg.SuppressionWindow = 30
	cfg.MinimumDelta = 5

	m := newCharacterizationManager(t, cfg)
	running := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.90)
	cleared := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.70)
	similarSpike := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.91)
	suppressedByTag := testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.91, "pulse-no-alerts")

	m.CheckGuest(running, "pve1")
	assertAlertPresent(t, m, alertID)

	m.CheckGuest(cleared, "pve1")
	assertAlertMissing(t, m, alertID)

	m.CheckGuest(similarSpike, "pve1")
	assertAlertMissing(t, m, alertID)

	m.mu.RLock()
	_, isSuppressed := m.suppressedUntil[trackingKey]
	m.mu.RUnlock()
	if !isSuppressed {
		t.Fatalf("expected similar retrigger to be suppressed for canonical tracking key %q", trackingKey)
	}

	m.CheckGuest(suppressedByTag, "pve1")

	m.mu.RLock()
	_, isSuppressed = m.suppressedUntil[trackingKey]
	m.mu.RUnlock()
	if isSuppressed {
		t.Fatalf("expected pulse-no-alerts suppression to clear stale suppression state for %q", trackingKey)
	}

	m.CheckGuest(similarSpike, "pve1")
	assertAlertPresent(t, m, alertID)
}

func TestAlertCharacterizationResolvedLookupByCanonicalStateAlias(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"
	canonicalState := buildCanonicalStateID(resourceID, alertID)

	m := newCharacterizationManager(t, characterizationBaseConfig())
	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.95), "pve1")
	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.70), "pve1")

	resolved := m.GetResolvedAlert(canonicalState)
	if resolved == nil || resolved.Alert == nil {
		t.Fatalf("expected resolved alert lookup by canonical state %q", canonicalState)
	}
	if resolved.Alert.ID != alertID {
		t.Fatalf("resolved alert ID = %q, want %q", resolved.Alert.ID, alertID)
	}
}

func TestAlertCharacterizationManualClearRemovesCanonicalTrackingState(t *testing.T) {
	resourceID := BuildGuestKey("pve1", "node1", 101)
	alertID := resourceID + "-cpu"
	trackingKey := buildCanonicalStateID(resourceID, alertID)

	m := newCharacterizationManager(t, characterizationBaseConfig())
	m.CheckGuest(testVM(resourceID, 101, "app01", "node1", "pve1", "running", 0.85), "pve1")
	assertAlertPresent(t, m, alertID)

	m.mu.Lock()
	m.recentAlerts[trackingKey] = &Alert{ID: alertID, ResourceID: resourceID, CanonicalState: trackingKey, StartTime: time.Now(), LastSeen: time.Now()}
	m.suppressedUntil[trackingKey] = time.Now().Add(time.Hour)
	m.alertRateLimit[trackingKey] = []time.Time{time.Now()}
	m.mu.Unlock()

	if !m.ClearAlert(alertID) {
		t.Fatalf("expected ClearAlert(%q) to succeed", alertID)
	}

	m.mu.RLock()
	_, recentExists := m.recentAlerts[trackingKey]
	_, suppressedExists := m.suppressedUntil[trackingKey]
	_, rateExists := m.alertRateLimit[trackingKey]
	m.mu.RUnlock()
	if recentExists || suppressedExists || rateExists {
		t.Fatalf("expected manual clear to remove canonical tracking entries, got recent=%t suppressed=%t rate=%t", recentExists, suppressedExists, rateExists)
	}
}
