package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func trueNASMemoryAlertResource(id, supersededID, hostname string, memory float64) unifiedresources.Resource {
	resource := unifiedresources.Resource{
		ID:      id,
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    hostname,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		TrueNAS: &unifiedresources.TrueNASData{Hostname: hostname},
		Metrics: &unifiedresources.ResourceMetrics{
			Memory: &unifiedresources.MetricValue{
				Value:   memory,
				Percent: memory,
				Unit:    "percent",
				Source:  unifiedresources.SourceTrueNAS,
			},
		},
	}
	if supersededID != "" {
		resource.SupersededCanonicalIDs = []string{supersededID}
	}
	return resource
}

func activeMemoryAlertForResource(manager *alerts.Manager, resourceID string) (alerts.Alert, bool) {
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == resourceID && alert.Type == "memory" {
			return alert, true
		}
	}
	return alerts.Alert{}, false
}

func waitForAlertNotification(t *testing.T, notifications <-chan alerts.Alert, resourceID string, level alerts.AlertLevel) alerts.Alert {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		select {
		case notification := <-notifications:
			if notification.ResourceID == resourceID && notification.Level == level {
				return notification
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for %s notification for %s", level, resourceID)
		}
	}
}

func waitForAlertResolution(t *testing.T, resolutions <-chan string, resourceID string) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		select {
		case alertID := <-resolutions:
			if strings.Contains(alertID, resourceID) {
				return
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for resolution callback for %s", resourceID)
		}
	}
}

func TestSyncUnifiedResourceAlertsPersistsAndEvaluatesTrueNASOverrideSuccession(t *testing.T) {
	const (
		oldID       = "agent-535886018cb53055"
		newID       = "agent-b9ed6d0e20e94eaf"
		secondID    = "agent-4266ee45469c27f1"
		sharedName  = "strawberrynas"
		overridePct = 95.0
	)

	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)
	manager := alerts.NewManagerWithDataDir(dataDir)
	t.Cleanup(manager.Stop)

	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.ActivationState = alerts.ActivationActive
	alertConfig.TrueNASDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
	alertConfig.TimeThresholds["truenas-system"] = 0
	alertConfig.Overrides = map[string]alerts.ThresholdConfig{
		oldID: {
			Memory: &alerts.HysteresisThreshold{Trigger: overridePct, Clear: 90},
		},
	}
	manager.UpdateConfig(alertConfig)
	if err := persistence.SaveAlertConfig(manager.GetConfig()); err != nil {
		t.Fatalf("seed legacy alert config: %v", err)
	}

	monitor := &Monitor{
		alertManager:  manager,
		configPersist: persistence,
		state:         models.NewState(),
	}
	notifications := make(chan alerts.Alert, 8)
	resolutions := make(chan string, 4)
	manager.SetAlertCallback(func(alert *alerts.Alert) {
		notifications <- *alert
	})
	manager.SetResolvedCallback(func(alertID string) {
		resolutions <- alertID
	})

	// Two configured TrueNAS systems can legitimately report the same
	// hostname. The first resource owns the migrated 95% threshold; the
	// second remains on the 85% default. Reordered resource snapshots must
	// never transfer the override between their connection-scoped IDs.
	resources := []unifiedresources.Resource{
		trueNASMemoryAlertResource(secondID, "", sharedName, 90),
		trueNASMemoryAlertResource(newID, oldID, sharedName, 90),
	}
	monitor.syncUnifiedResourceAlertsToState(resources)

	inMemory := manager.GetConfig()
	if _, exists := inMemory.Overrides[oldID]; exists {
		t.Fatalf("in-memory override remained under superseded identity %s", oldID)
	}
	if override := inMemory.Overrides[newID]; override.Memory == nil || override.Memory.Trigger != 95 {
		t.Fatalf("in-memory override missing under canonical identity %s: %+v", newID, override)
	}
	if _, exists := activeMemoryAlertForResource(manager, newID); exists {
		t.Fatalf("custom TrueNAS threshold fired at the default threshold for %s", newID)
	}
	defaultAlert, exists := activeMemoryAlertForResource(manager, secondID)
	if !exists || defaultAlert.Threshold != 85 {
		t.Fatalf("second TrueNAS system did not retain its independent default threshold: %+v", defaultAlert)
	}
	waitForAlertNotification(t, notifications, secondID, alerts.AlertLevelWarning)

	reloaded, err := config.NewConfigPersistence(dataDir).LoadAlertConfig()
	if err != nil {
		t.Fatalf("reload migrated alert config: %v", err)
	}
	if _, exists := reloaded.Overrides[oldID]; exists {
		t.Fatalf("persisted override remained under superseded identity %s", oldID)
	}
	if override := reloaded.Overrides[newID]; override.Memory == nil || override.Memory.Trigger != 95 {
		t.Fatalf("reloaded override missing under canonical identity %s: %+v", newID, override)
	}

	resources = []unifiedresources.Resource{
		trueNASMemoryAlertResource(newID, oldID, sharedName, 96),
		trueNASMemoryAlertResource(secondID, "", sharedName, 70),
	}
	monitor.syncUnifiedResourceAlertsToState(resources)
	warning := waitForAlertNotification(t, notifications, newID, alerts.AlertLevelWarning)
	waitForAlertResolution(t, resolutions, secondID)
	if warning.Threshold != overridePct {
		t.Fatalf("warning notification threshold = %.1f, want %.1f", warning.Threshold, overridePct)
	}

	resources[0] = trueNASMemoryAlertResource(newID, oldID, sharedName, 99)
	monitor.syncUnifiedResourceAlertsToState(resources)
	critical := waitForAlertNotification(t, notifications, newID, alerts.AlertLevelCritical)
	if critical.Threshold != overridePct {
		t.Fatalf("critical notification threshold = %.1f, want %.1f", critical.Threshold, overridePct)
	}
	if active, exists := activeMemoryAlertForResource(manager, newID); !exists || active.Level != alerts.AlertLevelCritical {
		t.Fatalf("expected critical TrueNAS alert at derived 99%% escalation, got %+v", active)
	}

	resources[0] = trueNASMemoryAlertResource(newID, oldID, sharedName, 89)
	monitor.syncUnifiedResourceAlertsToState(resources)
	if _, exists := activeMemoryAlertForResource(manager, newID); exists {
		t.Fatalf("TrueNAS alert did not clear below the persisted 90%% recovery threshold")
	}
	waitForAlertResolution(t, resolutions, newID)

	// Model a process restart from alerts.json. No provider-declared legacy ID
	// is required after migration, and a 90% sample must remain below the
	// persisted 95% override instead of reverting to the 85% default.
	restartedManager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(restartedManager.Stop)
	restartedManager.UpdateConfig(*reloaded)
	restartedMonitor := &Monitor{
		alertManager:  restartedManager,
		configPersist: persistence,
		state:         models.NewState(),
	}
	restartedMonitor.syncUnifiedResourceAlertsToState([]unifiedresources.Resource{
		trueNASMemoryAlertResource(newID, "", sharedName, 90),
		trueNASMemoryAlertResource(secondID, "", sharedName, 70),
	})
	if _, exists := activeMemoryAlertForResource(restartedManager, newID); exists {
		t.Fatalf("restarted manager evaluated %s at the default threshold", newID)
	}
	restartedConfig := restartedManager.GetConfig()
	if override := restartedConfig.Overrides[newID]; override.Memory == nil || override.Memory.Trigger != overridePct || override.Memory.Clear != 90 {
		t.Fatalf("restarted manager lost persisted TrueNAS hysteresis: %+v", override.Memory)
	}
}
