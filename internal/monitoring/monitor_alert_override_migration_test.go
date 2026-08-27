package monitoring

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
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

func TestHandleAlertEscalatedPreservesLegacyAppriseAndExactWebhookRouting(t *testing.T) {
	t.Run("legacy Apprise", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		requests := make(chan struct{}, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests <- struct{}{}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifMgr := notifications.NewNotificationManager("https://pulse.local")
		defer notifMgr.Stop()
		notifMgr.SetGroupingWindow(0)
		if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
			t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
		}
		notifMgr.SetAppriseConfig(notifications.AppriseConfig{
			Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: server.URL,
		})

		manager := alerts.NewManager()
		cfg := manager.GetConfig()
		cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{After: 1, Notify: "apprise"}}
		cfg.Schedule.QuietHours.Enabled = false
		manager.UpdateConfig(cfg)

		(&Monitor{notificationMgr: notifMgr, alertManager: manager}).handleAlertEscalated(nil, &alerts.Alert{
			ID: "apprise-escalation", Type: "connectivity", Level: alerts.AlertLevelCritical,
			ResourceID: "node/pve-1", ResourceName: "pve-1", StartTime: time.Now(),
		}, 1)

		select {
		case <-requests:
		case <-time.After(2 * time.Second):
			t.Fatal("expected legacy Apprise escalation delivery")
		}
	})

	t.Run("exact webhook", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		selectedRequests := make(chan struct{}, 1)
		selectedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			selectedRequests <- struct{}{}
			w.WriteHeader(http.StatusOK)
		}))
		defer selectedServer.Close()
		unselectedRequests := make(chan struct{}, 1)
		unselectedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			unselectedRequests <- struct{}{}
			w.WriteHeader(http.StatusOK)
		}))
		defer unselectedServer.Close()

		notifMgr := notifications.NewNotificationManager("https://pulse.local")
		defer notifMgr.Stop()
		notifMgr.SetGroupingWindow(0)
		if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
			t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
		}
		notifMgr.AddWebhook(notifications.WebhookConfig{ID: "ops", Name: "ops", URL: unselectedServer.URL, Enabled: true})
		notifMgr.AddWebhook(notifications.WebhookConfig{ID: "pager", Name: "pager", URL: selectedServer.URL, Enabled: true})

		manager := alerts.NewManager()
		cfg := manager.GetConfig()
		cfg.Schedule.Escalation.Levels = []alerts.EscalationLevel{{
			After: 1, Notify: "webhook", DestinationIDs: []string{"webhook:pager"},
		}}
		cfg.Schedule.QuietHours.Enabled = false
		manager.UpdateConfig(cfg)

		(&Monitor{notificationMgr: notifMgr, alertManager: manager}).handleAlertEscalated(nil, &alerts.Alert{
			ID: "exact-webhook-escalation", Type: "connectivity", Level: alerts.AlertLevelCritical,
			ResourceID: "node/pve-1", ResourceName: "pve-1", StartTime: time.Now(),
		}, 1)

		select {
		case <-selectedRequests:
		case <-time.After(2 * time.Second):
			t.Fatal("expected selected webhook escalation delivery")
		}
		select {
		case <-unselectedRequests:
			t.Fatal("exact escalation selection widened to an unselected webhook")
		case <-time.After(300 * time.Millisecond):
		}
	})
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
	if got := reloaded.IdentitySchemaVersion; got != alerts.CurrentAlertIdentitySchemaVersion {
		t.Fatalf("persisted alert identity schema version = %d, want %d", got, alerts.CurrentAlertIdentitySchemaVersion)
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

func TestMigrateAvailabilityLinkedResources(t *testing.T) {
	t.Parallel()

	guestID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "delly", 100)
	retiredID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeVM, unifiedresources.SourceProxmox, "delly:pve1:100")
	resources := []unifiedresources.Resource{
		{
			ID:                     guestID,
			Type:                   unifiedresources.ResourceTypeVM,
			SupersededCanonicalIDs: []string{retiredID},
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "delly",
				VMID:     100,
				NodeName: "pve2",
			},
		},
		{
			ID:   "agent-1234567890abcdef",
			Type: unifiedresources.ResourceTypeAgent,
		},
	}

	targets := []config.AvailabilityTarget{
		{ID: "t-live", LinkedResourceID: guestID},
		{ID: "t-retired", LinkedResourceID: retiredID},
		{ID: "t-old-node-source", LinkedResourceID: "delly:pve1:100"},
		{ID: "t-unrelated", LinkedResourceID: "agent-1234567890abcdef"},
		{ID: "t-unknown", LinkedResourceID: "delly:pve9:999"},
		{ID: "t-unlinked"},
	}

	migrated, changed := migrateAvailabilityLinkedResources(targets, resources)
	if !changed {
		t.Fatal("expected migration to report changes")
	}

	byID := make(map[string]config.AvailabilityTarget, len(migrated))
	for _, target := range migrated {
		byID[target.ID] = target
	}
	if got := byID["t-live"].LinkedResourceID; got != guestID {
		t.Fatalf("live link rewritten: %q", got)
	}
	if got := byID["t-retired"].LinkedResourceID; got != guestID {
		t.Fatalf("retired canonical link = %q, want %q", got, guestID)
	}
	if got := byID["t-old-node-source"].LinkedResourceID; got != guestID {
		t.Fatalf("old-node source link = %q, want %q", got, guestID)
	}
	if got := byID["t-unrelated"].LinkedResourceID; got != "agent-1234567890abcdef" {
		t.Fatalf("unrelated link rewritten: %q", got)
	}
	if got := byID["t-unknown"].LinkedResourceID; got != "delly:pve9:999" {
		t.Fatalf("unknown guest link rewritten: %q", got)
	}

	// Input slice must stay untouched: the migration returns a copy.
	if targets[1].LinkedResourceID != retiredID {
		t.Fatalf("input slice mutated: %q", targets[1].LinkedResourceID)
	}

	if _, changedAgain := migrateAvailabilityLinkedResources(migrated, resources); changedAgain {
		t.Fatal("second pass must be a no-op")
	}
}

// Two guests claiming the same retired ID (or the same instance+VMID) is
// ambiguous; the link must fail closed rather than re-home to either.
func TestMigrateAvailabilityLinkedResourcesFailsClosedOnAmbiguity(t *testing.T) {
	t.Parallel()

	retiredID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeVM, unifiedresources.SourceProxmox, "delly:pve1:100")
	resources := []unifiedresources.Resource{
		{
			ID:                     "vm-aaaaaaaaaaaaaaaa",
			Type:                   unifiedresources.ResourceTypeVM,
			SupersededCanonicalIDs: []string{retiredID},
			Proxmox:                &unifiedresources.ProxmoxData{Instance: "delly", VMID: 100},
		},
		{
			ID:                     "vm-bbbbbbbbbbbbbbbb",
			Type:                   unifiedresources.ResourceTypeVM,
			SupersededCanonicalIDs: []string{retiredID},
			Proxmox:                &unifiedresources.ProxmoxData{Instance: "delly", VMID: 100},
		},
	}
	targets := []config.AvailabilityTarget{
		{ID: "t-ambiguous", LinkedResourceID: retiredID},
		{ID: "t-ambiguous-source", LinkedResourceID: "delly:pve1:100"},
	}

	migrated, changed := migrateAvailabilityLinkedResources(targets, resources)
	if changed {
		t.Fatalf("ambiguous claims must not migrate, got %+v", migrated)
	}
}

func TestSyncUnifiedResourceAlertsMigratesDockerContainerOverrideKeys(t *testing.T) {
	dockerApp := func(id, hostID, containerID, name string) unifiedresources.Resource {
		return unifiedresources.Resource{
			ID:   id,
			Type: unifiedresources.ResourceTypeAppContainer,
			Name: name,
			Docker: &unifiedresources.DockerData{
				HostSourceID: hostID,
				ContainerID:  containerID,
			},
		}
	}

	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)
	manager := alerts.NewManagerWithDataDir(dataDir)
	t.Cleanup(manager.Stop)

	alertConfig := manager.GetConfig()
	alertConfig.Enabled = true
	alertConfig.ActivationState = alerts.ActivationActive
	alertConfig.Overrides = map[string]alerts.ThresholdConfig{
		// Container IDs change on every recreate; the sync loop must re-home
		// this live legacy key onto docker:{host}/{name} (#1601).
		"docker:host-1/aaaaaaaaaaaa": {Disabled: true},
		// The v6 UI keyed overrides by the unified hash id; re-homed too.
		"docker:host-1/app-container-0011223344556677": {Disabled: true},
		// Orphan from a past recreate: pruned.
		"docker:host-1/bbbbbbbbbbbb": {Disabled: true},
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
	monitor.syncUnifiedResourceAlertsToState([]unifiedresources.Resource{
		dockerApp("app-container-1111111111111111", "host-1", "aaaaaaaaaaaa", "media-server"),
		dockerApp("app-container-0011223344556677", "host-1", "cccccccccccc", "proxy"),
	})

	inMemory := manager.GetConfig()
	for _, gone := range []string{
		"docker:host-1/aaaaaaaaaaaa",
		"docker:host-1/app-container-0011223344556677",
		"docker:host-1/bbbbbbbbbbbb",
	} {
		if _, exists := inMemory.Overrides[gone]; exists {
			t.Fatalf("in-memory override remained under retired docker key %s", gone)
		}
	}
	if override := inMemory.Overrides["docker:host-1/media-server"]; !override.Disabled {
		t.Fatalf("legacy container-ID override did not re-home onto the name key: %+v", inMemory.Overrides)
	}
	if override := inMemory.Overrides["docker:host-1/proxy"]; !override.Disabled {
		t.Fatalf("unified-hash override did not re-home onto the name key: %+v", inMemory.Overrides)
	}

	reloaded, err := config.NewConfigPersistence(dataDir).LoadAlertConfig()
	if err != nil {
		t.Fatalf("reload migrated alert config: %v", err)
	}
	if _, exists := reloaded.Overrides["docker:host-1/aaaaaaaaaaaa"]; exists {
		t.Fatalf("persisted override remained under the retired container-ID key")
	}
	if override := reloaded.Overrides["docker:host-1/media-server"]; !override.Disabled {
		t.Fatalf("persisted config missing the re-homed name-keyed override: %+v", reloaded.Overrides)
	}
}
