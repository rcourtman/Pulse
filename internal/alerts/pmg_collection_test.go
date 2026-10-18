package alerts

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"testing"
	"time"
)

func TestPMGCollectionMissingReadingsPreserveAlerts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nodes []models.PMGNodeStatus
	}{
		{"disabled", []models.PMGNodeStatus{{Name: "one"}, {Name: "two"}}},
		{"missing-cluster", nil},
		{"partial-node", []models.PMGNodeStatus{{Name: "one", QueueStatus: &models.PMGQueueStatus{}}, {Name: "two"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			ids := []string{"gateway-queue-total", "gateway-queue-deferred", "gateway-queue-hold", "gateway-oldest-message", "gateway-quarantine-spam", "gateway-quarantine-virus"}
			m.mu.Lock()
			for _, id := range ids {
				key := buildCanonicalStateID("gateway", id)
				m.activeAlerts[key] = &Alert{ID: key, ResourceID: "gateway", Type: id, StartTime: time.Now()}
			}
			m.mu.Unlock()
			v := models.PMGInstance{ID: "gateway", Status: "online", ConnectionHealth: "healthy", Nodes: tc.nodes}
			thresholds := PMGThresholdConfig{QueueTotalWarning: 1, DeferredQueueWarn: 1, HoldQueueWarn: 1, OldestMessageWarnMins: 1}
			m.checkPMGQueueDepths(v, thresholds)
			m.checkPMGOldestMessage(v, thresholds)
			m.checkPMGQuarantineBacklog(v, thresholds)
			m.mu.RLock()
			defer m.mu.RUnlock()
			for _, id := range ids {
				if _, exists := testLookupActiveAlert(t, m, buildCanonicalStateID("gateway", id)); !exists {
					t.Errorf("missing data resolved %s", id)
				}
			}
			if len(m.pmgQuarantineHistory["gateway"]) != 0 {
				t.Fatal("missing quarantine added zero growth sample")
			}
		})
	}
}
func TestPMGCollectionObservedZeroRecoversOldest(t *testing.T) {
	m := newTestManager(t)
	key := buildCanonicalStateID("gateway", "gateway-oldest-message")
	m.mu.Lock()
	m.activeAlerts[key] = &Alert{ID: key, ResourceID: "gateway", Type: "message-age"}
	m.mu.Unlock()
	v := models.PMGInstance{ID: "gateway", Nodes: []models.PMGNodeStatus{{Name: "one", QueueStatus: &models.PMGQueueStatus{}}, {Name: "two", QueueStatus: &models.PMGQueueStatus{}}}}
	m.checkPMGOldestMessage(v, PMGThresholdConfig{OldestMessageWarnMins: 1})
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, exists := testLookupActiveAlert(t, m, key); exists {
		t.Fatal("fresh complete zero did not resolve oldest alert")
	}
}

func TestPMGCollectionPartialQueueLowerBound(t *testing.T) {
	m := newTestManager(t)
	v := models.PMGInstance{ID: "gateway", Name: "gateway", Nodes: []models.PMGNodeStatus{{Name: "one", QueueStatus: &models.PMGQueueStatus{Total: 200}}, {Name: "two"}}}
	m.checkPMGQueueDepths(v, PMGThresholdConfig{QueueTotalWarning: 10, QueueTotalCritical: 50})
	key := buildCanonicalStateID("gateway", "gateway-queue-total")
	m.mu.RLock()
	alert := testRequireActiveAlert(t, m, key)
	m.mu.RUnlock()
	if alert.Level != AlertLevelCritical || alert.Value != 200 {
		t.Fatalf("known-high partial observation=%+v", alert)
	}
	// The next partial lower bound must not resolve or downgrade critical state.
	v.Nodes[0].QueueStatus.Total = 20
	m.checkPMGQueueDepths(v, PMGThresholdConfig{QueueTotalWarning: 10, QueueTotalCritical: 50})
	m.mu.RLock()
	defer m.mu.RUnlock()
	after := testRequireActiveAlert(t, m, key)
	if after.Level != AlertLevelCritical || after.Value != 200 {
		t.Fatalf("partial warning downgraded critical state=%+v", after)
	}
}
