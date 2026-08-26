package alerts

import (
	"sync"
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newPBSOfflinePolicyFixture(t *testing.T) (models.PBSInstance, *unifiedresources.MonitorAdapter, string) {
	t.Helper()

	pbs := models.PBSInstance{
		ID:               "pbs-monitor-id",
		Name:             "pbs-main",
		Host:             "https://pbs.example.invalid:8007",
		Status:           "offline",
		ConnectionHealth: "error",
		LastSeen:         time.Now(),
	}
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	adapter.PopulateFromSnapshot(models.StateSnapshot{PBSInstances: []models.PBSInstance{pbs}})
	canonicalID, ok := adapter.ResolveCanonicalResourceID(pbs.ID)
	if !ok {
		t.Fatalf("registry did not resolve PBS monitor ID %q", pbs.ID)
	}
	if canonicalID == "" || canonicalID == pbs.ID {
		t.Fatalf("canonical PBS ID %q did not differ from monitor ID %q", canonicalID, pbs.ID)
	}
	registryResourceFound := false
	for _, resource := range adapter.GetAll() {
		if resource.Type == unifiedresources.ResourceTypePBS && resource.ID == canonicalID {
			registryResourceFound = true
			break
		}
	}
	if !registryResourceFound {
		t.Fatalf("canonical PBS resource %q was not present in the registry", canonicalID)
	}
	return pbs, adapter, canonicalID
}

func configurePBSOfflinePolicyTestManager(t *testing.T, canonicalID string, disabled bool) (*Manager, chan *Alert) {
	t.Helper()

	m := newTestManager(t)
	cfg := m.GetConfig()
	cfg.ActivationState = ActivationActive
	cfg.TimeThresholds["pbs"] = 0
	cfg.Overrides = map[string]ThresholdConfig{}
	if disabled {
		cfg.Overrides[canonicalID] = ThresholdConfig{DisableConnectivity: true}
	}
	m.UpdateConfig(cfg)

	dispatched := make(chan *Alert, 4)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched <- alert
	})
	return m, dispatched
}

func checkPBSRepeatedly(m *Manager, pbs models.PBSInstance, count int) {
	for range count {
		m.CheckPBS(pbs)
	}
}

func requirePBSDispatch(t *testing.T, dispatched <-chan *Alert) *Alert {
	t.Helper()
	select {
	case alert := <-dispatched:
		return alert
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PBS alert callback")
		return nil
	}
}

func requireNoPBSDispatch(t *testing.T, dispatched <-chan *Alert) {
	t.Helper()
	select {
	case alert := <-dispatched:
		t.Fatalf("unexpected alert callback for %s", alert.ID)
	case <-time.After(100 * time.Millisecond):
	}
}

func requireSinglePBSAlert(t *testing.T, m *Manager, alertType string, level AlertLevel) Alert {
	t.Helper()
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("active alerts = %d, want 1: %+v", len(active), active)
	}
	if active[0].Type != alertType || active[0].Level != level {
		t.Fatalf("active alert = type %q level %q, want type %q level %q", active[0].Type, active[0].Level, alertType, level)
	}
	return active[0]
}

func TestCheckPBSOfflineCanonicalOverrideBlocksAlertAndNotification(t *testing.T) {
	pbs, adapter, canonicalID := newPBSOfflinePolicyFixture(t)

	t.Run("legacy ID lookup reproduces disabled-policy notification leak", func(t *testing.T) {
		m, dispatched := configurePBSOfflinePolicyTestManager(t, canonicalID, true)

		// This control deliberately omits the registry resolver. It models the
		// pre-fix lookup, which checked only pbs.ID even though the UI persisted
		// the override under the canonical registry ID.
		checkPBSRepeatedly(m, pbs, 3)

		alert := requireSinglePBSAlert(t, m, "offline", AlertLevelCritical)
		if alert.ResourceID != pbs.ID {
			t.Fatalf("offline alert resource ID = %q, want monitor ID %q", alert.ResourceID, pbs.ID)
		}
		dispatch := requirePBSDispatch(t, dispatched)
		if dispatch.Type != "offline" || dispatch.Level != AlertLevelCritical {
			t.Fatalf("dispatch = type %q level %q, want critical offline", dispatch.Type, dispatch.Level)
		}
	})

	t.Run("canonical override blocks offline flaps but preserves PBS metrics", func(t *testing.T) {
		m, dispatched := configurePBSOfflinePolicyTestManager(t, canonicalID, true)
		m.SetResourceIntentIdentityResolver(adapter.ResolveCanonicalResourceID)

		checkPBSRepeatedly(m, pbs, 3)
		if active := m.GetActiveAlerts(); len(active) != 0 {
			t.Fatalf("disabled PBS offline policy created active alerts: %+v", active)
		}
		m.mu.RLock()
		tracked := testCoreHasIncident(m, pbs.ID, canonicalConnectivitySpecID(pbs.ID))
		m.mu.RUnlock()
		if tracked {
			t.Fatalf("disabled PBS offline policy retained confirmation tracking for %q", pbs.ID)
		}
		requireNoPBSDispatch(t, dispatched)

		// Exercise an offline -> healthy -> offline flap. The healthy sample
		// also proves DisableConnectivity does not suppress unrelated metrics.
		healthy := pbs
		healthy.Status = "online"
		healthy.ConnectionHealth = "healthy"
		healthy.CPU = 99
		m.CheckPBS(healthy)
		metricAlert := requireSinglePBSAlert(t, m, "cpu", AlertLevelCritical)
		metricDispatch := requirePBSDispatch(t, dispatched)
		if metricDispatch.ID != metricAlert.ID || metricDispatch.Type != "cpu" || metricDispatch.Level != AlertLevelCritical {
			t.Fatalf("metric dispatch = %+v, want critical CPU alert %q", metricDispatch, metricAlert.ID)
		}

		healthy.CPU = 0
		m.CheckPBS(healthy)
		if active := m.GetActiveAlerts(); len(active) != 0 {
			t.Fatalf("healthy CPU recovery left active alerts: %+v", active)
		}
		checkPBSRepeatedly(m, pbs, 3)
		if active := m.GetActiveAlerts(); len(active) != 0 {
			t.Fatalf("disabled PBS policy created an alert after a reconnect flap: %+v", active)
		}
		requireNoPBSDispatch(t, dispatched)
	})

	t.Run("enabled offline policy still alerts and recovers", func(t *testing.T) {
		m, dispatched := configurePBSOfflinePolicyTestManager(t, canonicalID, false)
		m.SetResourceIntentIdentityResolver(adapter.ResolveCanonicalResourceID)

		checkPBSRepeatedly(m, pbs, 3)
		alert := requireSinglePBSAlert(t, m, "offline", AlertLevelCritical)
		dispatch := requirePBSDispatch(t, dispatched)
		if dispatch.ID != alert.ID || dispatch.Type != "offline" || dispatch.Level != AlertLevelCritical {
			t.Fatalf("enabled dispatch = %+v, want critical offline alert %q", dispatch, alert.ID)
		}

		healthy := pbs
		healthy.Status = "online"
		healthy.ConnectionHealth = "healthy"
		checkPBSRepeatedly(m, healthy, offlineRecoveryConfirmationsDefault)
		if active := m.GetActiveAlerts(); len(active) != 0 {
			t.Fatalf("recovered PBS left active alerts: %+v", active)
		}
	})

	t.Run("concurrent policy updates keep the pre-dispatch gate race-free", func(t *testing.T) {
		m, _ := configurePBSOfflinePolicyTestManager(t, canonicalID, false)
		m.SetResourceIntentIdentityResolver(adapter.ResolveCanonicalResourceID)
		m.SetAlertCallback(nil)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				cfg := m.GetConfig()
				cfg.Overrides = map[string]ThresholdConfig{}
				if i%2 == 0 {
					cfg.Overrides[canonicalID] = ThresholdConfig{DisableConnectivity: true}
				}
				m.UpdateConfig(cfg)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				m.CheckPBS(pbs)
			}
		}()
		wg.Wait()

		cfg := m.GetConfig()
		cfg.Overrides = map[string]ThresholdConfig{
			canonicalID: {DisableConnectivity: true},
		}
		m.UpdateConfig(cfg)
		checkPBSRepeatedly(m, pbs, 3)
		if active := m.GetActiveAlerts(); len(active) != 0 {
			t.Fatalf("final disabled policy left active alerts after concurrent updates: %+v", active)
		}
	})
}

func TestCheckPBSOfflineDoesNotRenotifyExistingAlert(t *testing.T) {
	m := newTestManager(t)
	m.config.ActivationState = ActivationActive
	dispatched := make(chan *Alert, 1)
	m.SetAlertCallback(func(alert *Alert) {
		dispatched <- alert
	})

	oldSeen := time.Now().Add(-time.Hour)
	notifiedAt := time.Now().Add(-10 * time.Minute)
	state, existing := testNewCanonicalAlert("pbs1", canonicalConnectivitySpecID("pbs1"), string(alertspecs.AlertSpecKindConnectivity), "offline")
	existing.Level = AlertLevelCritical
	existing.LastSeen = oldSeen
	existing.LastNotified = &notifiedAt
	existing.Metadata = map[string]interface{}{
		"resourceType": "pbs",
	}

	m.mu.Lock()
	m.setActiveAlertNoLock(state, existing)
	m.mu.Unlock()

	m.checkPBSOffline(models.PBSInstance{ID: "pbs1", Name: "PBS 1", Host: "pbs.local"})

	select {
	case alert := <-dispatched:
		t.Fatalf("expected repeated PBS offline poll to stay quiet, got callback for %s", alert.ID)
	case <-time.After(50 * time.Millisecond):
	}

	m.mu.RLock()
	active := m.activeAlerts[state]
	m.mu.RUnlock()

	if active == nil {
		t.Fatal("expected existing PBS offline alert to remain active")
	}
	if !active.LastSeen.After(oldSeen) {
		t.Fatalf("expected repeated PBS offline poll to refresh LastSeen after %s, got %s", oldSeen, active.LastSeen)
	}
	if active.LastNotified == nil || !active.LastNotified.Equal(notifiedAt) {
		t.Fatalf("expected LastNotified to remain %s, got %v", notifiedAt, active.LastNotified)
	}
}
