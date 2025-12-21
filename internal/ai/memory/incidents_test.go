package memory

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestIncidentStore_RecordTimeline(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		DataDir:              t.TempDir(),
		MaxIncidents:         10,
		MaxEventsPerIncident: 10,
		MaxAgeDays:           30,
	})

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-1",
		ResourceName: "vm-1",
		StartTime:    time.Now().Add(-5 * time.Minute),
		Value:        92,
		Threshold:    85,
	}

	store.RecordAlertFired(alert)
	store.RecordAlertAcknowledged(alert, "admin")
	store.RecordAnalysis(alert.ID, "analysis complete", map[string]interface{}{
		"findings": 1,
	})
	store.RecordCommand(alert.ID, "systemctl restart nginx", true, "ok", nil)
	store.RecordAlertResolved(alert, time.Now())

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatalf("expected timeline, got nil")
	}
	if timeline.Status != IncidentStatusResolved {
		t.Fatalf("expected status %q, got %q", IncidentStatusResolved, timeline.Status)
	}
	if timeline.AckUser != "admin" {
		t.Fatalf("expected ack user admin, got %q", timeline.AckUser)
	}
	if len(timeline.Events) < 4 {
		t.Fatalf("expected events recorded, got %d", len(timeline.Events))
	}

	if ok := store.RecordNote(alert.ID, "", "note text", ""); !ok {
		t.Fatalf("expected note to be saved")
	}
}

func TestIncidentStore_GetTimelineByAlertAt(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		DataDir:              t.TempDir(),
		MaxIncidents:         10,
		MaxEventsPerIncident: 10,
		MaxAgeDays:           30,
	})

	base := time.Now().UTC()

	first := &alerts.Alert{
		ID:           "alert-2",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-2",
		ResourceName: "vm-2",
		StartTime:    base.Add(-2 * time.Hour),
	}

	second := &alerts.Alert{
		ID:           "alert-2",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-2",
		ResourceName: "vm-2",
		StartTime:    base.Add(-10 * time.Minute),
	}

	store.RecordAlertFired(first)
	store.RecordAlertResolved(first, base.Add(-90*time.Minute))
	store.RecordAlertFired(second)

	timeline := store.GetTimelineByAlertAt(first.ID, first.StartTime)
	if timeline == nil {
		t.Fatalf("expected timeline for first incident, got nil")
	}
	if !timeline.OpenedAt.Equal(first.StartTime) {
		t.Fatalf("expected openedAt %s, got %s", first.StartTime, timeline.OpenedAt)
	}

	timeline = store.GetTimelineByAlertAt(second.ID, second.StartTime)
	if timeline == nil {
		t.Fatalf("expected timeline for second incident, got nil")
	}
	if !timeline.OpenedAt.Equal(second.StartTime) {
		t.Fatalf("expected openedAt %s, got %s", second.StartTime, timeline.OpenedAt)
	}

	timeline = store.GetTimelineByAlertAt(second.ID, base.Add(-45*time.Minute))
	if timeline != nil {
		t.Fatalf("expected no timeline for mismatched start time")
	}
}
