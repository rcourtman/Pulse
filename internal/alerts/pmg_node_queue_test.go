package alerts

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Choose thresholds so these readings breach the node thresholds but not the
// instance aggregate. A second healthy metric must not veto a node alert.
func newPMGNodeQueueManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.mu.Lock()
	m.config.PMGDefaults = PMGThresholdConfig{
		QueueTotalWarning:     100,
		QueueTotalCritical:    200,
		DeferredQueueWarn:     25,
		DeferredQueueCritical: 30,
		HoldQueueWarn:         10,
		HoldQueueCritical:     20,
		OldestMessageWarnMins: 30,
		OldestMessageCritMins: 60,
	}
	m.mu.Unlock()
	return m
}

func pmgNodeQueueSample(id string, queue *models.PMGQueueStatus) models.PMGInstance {
	return models.PMGInstance{
		ID: id, Name: id, Status: "online", ConnectionHealth: "healthy",
		Nodes: []models.PMGNodeStatus{{Name: "relay", QueueStatus: queue}},
	}
}

func TestPMGNodeQueueChecksAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		queue models.PMGQueueStatus
		want  map[string]AlertLevel
	}{
		{
			name:  "healthy total permits deferred hold and age",
			queue: models.PMGQueueStatus{Total: 33, Active: 1, Deferred: 24, Hold: 8, OldestAge: 20 * 60},
			want: map[string]AlertLevel{
				"queue-deferred": AlertLevelCritical, "queue-hold": AlertLevelWarning, "message-age": AlertLevelWarning,
			},
		},
		{
			name:  "healthy deferred permits hold and age",
			queue: models.PMGQueueStatus{Total: 65, Active: 52, Deferred: 5, Hold: 8, OldestAge: 20 * 60},
			want: map[string]AlertLevel{
				"queue-total": AlertLevelWarning, "queue-hold": AlertLevelWarning, "message-age": AlertLevelWarning,
			},
		},
		{
			name:  "healthy hold permits age",
			queue: models.PMGQueueStatus{Total: 65, Active: 40, Deferred: 24, Hold: 1, OldestAge: 20 * 60},
			want: map[string]AlertLevel{
				"queue-total": AlertLevelWarning, "queue-deferred": AlertLevelCritical, "message-age": AlertLevelWarning,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newPMGNodeQueueManager(t)
			m.CheckPMG(pmgNodeQueueSample("gateway-a", &tc.queue))
			got := make(map[string]AlertLevel)
			for _, alert := range m.GetActiveAlerts() {
				if alert.ResourceID != "gateway-a" || alert.Node != "relay" {
					t.Fatalf("unexpected aggregate or cross-node alert: %+v", alert)
				}
				if _, seen := got[alert.Type]; seen {
					t.Fatalf("duplicate alert for %q", alert.Type)
				}
				got[alert.Type] = alert.Level
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("node alerts = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestPMGNodeQueueRecoveryRequiresAnObservation(t *testing.T) {
	m := newPMGNodeQueueManager(t)
	high := &models.PMGQueueStatus{Total: 75, Active: 43, Deferred: 24, Hold: 8, OldestAge: 20 * 60}
	m.CheckPMG(pmgNodeQueueSample("gateway-a", high))
	m.CheckPMG(pmgNodeQueueSample("gateway-b", high))
	before := m.GetActiveAlerts()
	if len(before) != 8 {
		t.Fatalf("seeded %d alerts; want four per gateway", len(before))
	}

	// An unread same-named node keeps its previous evidence and resolves nothing.
	m.CheckPMG(pmgNodeQueueSample("gateway-a", nil))
	if got := m.GetActiveAlerts(); !reflect.DeepEqual(got, before) || len(m.GetRecentlyResolved()) != 0 {
		t.Fatal("missing queue observation changed active or resolved evidence")
	}

	// Catch the separate zero-age bug while the earlier three checks still
	// breach their own thresholds. Young mail can keep the count alerting.
	young := *high
	young.OldestAge = 0
	m.CheckPMG(pmgNodeQueueSample("gateway-a", &young))
	intermediate := m.GetActiveAlerts()
	if len(intermediate) != 7 {
		t.Errorf("after the observed zero age, %d alerts remain; want 7", len(intermediate))
	}
	for _, alert := range intermediate {
		if alert.ResourceID == "gateway-a" && alert.Type == "message-age" {
			t.Error("observed zero did not recover the oldest-message alert")
		}
	}

	// All four observations have now recovered, including metrics after total.
	m.CheckPMG(pmgNodeQueueSample("gateway-a", &models.PMGQueueStatus{}))
	var want []Alert
	for _, alert := range before {
		if alert.ResourceID == "gateway-b" {
			want = append(want, alert)
		}
	}
	if got := m.GetActiveAlerts(); !reflect.DeepEqual(got, want) {
		t.Errorf("complete recovery left %d active alerts; want only the four unchanged peer alerts", len(got))
	}
	if got := len(m.GetRecentlyResolved()); got != 4 {
		t.Errorf("complete recovery resolved %d alerts; want 4", got)
	}
}
