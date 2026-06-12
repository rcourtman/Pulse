package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

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
	m.offlineConfirmations["pbs1"] = 3
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
