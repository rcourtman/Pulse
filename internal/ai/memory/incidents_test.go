package memory

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestIncidentStore_RecordTimeline(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
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

func TestIncidentStore_RecordAlertUnacknowledged(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents:         10,
		MaxEventsPerIncident: 10,
		MaxAgeDays:           30,
	})

	alert := &alerts.Alert{
		ID:           "alert-unack-1",
		Type:         "memory",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-3",
		ResourceName: "vm-3",
		StartTime:    time.Now().Add(-10 * time.Minute),
	}

	// Fire alert and acknowledge it
	store.RecordAlertFired(alert)
	store.RecordAlertAcknowledged(alert, "admin")

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatalf("expected timeline after ack")
	}
	if !timeline.Acknowledged {
		t.Fatal("expected acknowledged=true after acknowledgement")
	}
	if timeline.AckUser != "admin" {
		t.Errorf("expected ack user admin, got %q", timeline.AckUser)
	}

	// Now unacknowledge
	store.RecordAlertUnacknowledged(alert, "operator")

	timeline = store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatalf("expected timeline after unack")
	}
	if timeline.Acknowledged {
		t.Fatal("expected acknowledged=false after unacknowledgement")
	}
	if timeline.AckUser != "" {
		t.Errorf("expected empty ack user after unack, got %q", timeline.AckUser)
	}
	if timeline.AckTime != nil {
		t.Error("expected nil ack time after unack")
	}

	// Check for the unacknowledge event
	foundUnackEvent := false
	for _, evt := range timeline.Events {
		if evt.Type == IncidentEventAlertUnacknowledged {
			foundUnackEvent = true
			if user, ok := evt.Details["user"].(string); !ok || user != "operator" {
				t.Errorf("expected user 'operator' in event details, got %v", evt.Details["user"])
			}
		}
	}
	if !foundUnackEvent {
		t.Error("expected to find unacknowledge event in timeline")
	}
}

func TestIncidentStore_RecordAlertUnacknowledged_NilAlert(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	// Should not panic with nil alert
	store.RecordAlertUnacknowledged(nil, "admin")
}

func TestIncidentStore_RecordRunbook(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents:         10,
		MaxEventsPerIncident: 10,
		MaxAgeDays:           30,
	})

	alert := &alerts.Alert{
		ID:           "alert-runbook-1",
		Type:         "disk",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "res-4",
		ResourceName: "storage-1",
		StartTime:    time.Now().Add(-5 * time.Minute),
	}

	store.RecordAlertFired(alert)

	// Record a runbook execution
	store.RecordRunbook(alert.ID, "runbook-cleanup", "Disk Cleanup", "success", true, "Freed 10GB")

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatal("expected timeline after runbook")
	}

	// Find the runbook event
	foundRunbookEvent := false
	for _, evt := range timeline.Events {
		if evt.Type == IncidentEventRunbook {
			foundRunbookEvent = true
			if !strings.Contains(evt.Summary, "Disk Cleanup") {
				t.Errorf("expected summary to contain 'Disk Cleanup', got %q", evt.Summary)
			}
			if !strings.Contains(evt.Summary, "success") {
				t.Errorf("expected summary to contain 'success', got %q", evt.Summary)
			}
			if runbookID, ok := evt.Details["runbook_id"].(string); !ok || runbookID != "runbook-cleanup" {
				t.Errorf("expected runbook_id 'runbook-cleanup', got %v", evt.Details["runbook_id"])
			}
			if automatic, ok := evt.Details["automatic"].(bool); !ok || !automatic {
				t.Errorf("expected automatic=true, got %v", evt.Details["automatic"])
			}
			if message, ok := evt.Details["message"].(string); !ok || message != "Freed 10GB" {
				t.Errorf("expected message 'Freed 10GB', got %v", evt.Details["message"])
			}
		}
	}
	if !foundRunbookEvent {
		t.Error("expected to find runbook event in timeline")
	}
}

func TestIncidentStore_RecordRunbook_EmptyParams(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	// Should not create incident with empty alertID
	store.RecordRunbook("", "runbook-1", "Test", "success", false, "")

	// Should not create incident with empty runbookID
	store.RecordRunbook("alert-1", "", "Test", "success", false, "")
}

func TestIncidentStore_RecordRunbook_CreatesIncident(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	// RecordRunbook should create incident if none exists
	store.RecordRunbook("new-alert", "runbook-1", "Test Runbook", "completed", false, "")

	timeline := store.GetTimelineByAlertID("new-alert")
	if timeline == nil {
		t.Fatal("expected timeline to be created by RecordRunbook")
	}
	if timeline.Status != IncidentStatusOpen {
		t.Errorf("expected status 'open', got %q", timeline.Status)
	}
}

func TestIncidentStore_ListIncidentsByResource(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 20,
		MaxAgeDays:   30,
	})

	// Create multiple incidents for the same resource
	for i := 0; i < 5; i++ {
		alert := &alerts.Alert{
			ID:           "alert-list-" + string(rune('A'+i)),
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "res-list-1",
			ResourceName: "vm-list-1",
			StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
		}
		store.RecordAlertFired(alert)
	}

	// Create incident for different resource
	otherAlert := &alerts.Alert{
		ID:           "alert-other",
		Type:         "memory",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "res-other",
		ResourceName: "vm-other",
		StartTime:    time.Now(),
	}
	store.RecordAlertFired(otherAlert)

	// List all incidents for res-list-1
	incidents := store.ListIncidentsByResource("res-list-1", 0)
	if len(incidents) != 5 {
		t.Errorf("expected 5 incidents for res-list-1, got %d", len(incidents))
	}

	// List with limit
	incidents = store.ListIncidentsByResource("res-list-1", 3)
	if len(incidents) != 3 {
		t.Errorf("expected 3 incidents with limit, got %d", len(incidents))
	}

	// List for non-existent resource
	incidents = store.ListIncidentsByResource("res-nonexistent", 0)
	if len(incidents) != 0 {
		t.Errorf("expected 0 incidents for non-existent resource, got %d", len(incidents))
	}

	// Empty resource ID
	incidents = store.ListIncidentsByResource("", 0)
	if incidents != nil {
		t.Error("expected nil for empty resource ID")
	}
}

func TestIncidentStore_FormatForAlert(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents:         10,
		MaxEventsPerIncident: 50,
		MaxAgeDays:           30,
	})

	alert := &alerts.Alert{
		ID:           "alert-format-1",
		Type:         "cpu_high",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-format-1",
		ResourceName: "vm-format-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
	}

	store.RecordAlertFired(alert)
	store.RecordAlertAcknowledged(alert, "admin")
	store.RecordAnalysis(alert.ID, "High CPU due to process X", nil)

	result := store.FormatForAlert(alert.ID, 10)

	if result == "" {
		t.Fatal("expected non-empty format result")
	}

	// Check for expected content
	if !strings.Contains(result, "## Incident Memory") {
		t.Error("expected '## Incident Memory' header")
	}
	if !strings.Contains(result, "vm-format-1") {
		t.Error("expected resource name in output")
	}
	if !strings.Contains(result, "cpu_high") {
		t.Error("expected alert type in output")
	}
	if !strings.Contains(result, "Status: open") {
		t.Error("expected status in output")
	}
}

func TestIncidentStore_FormatForAlert_MaxEvents(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents:         10,
		MaxEventsPerIncident: 100,
	})

	alert := &alerts.Alert{
		ID:           "alert-max-events",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-1",
		ResourceName: "vm-1",
	}

	store.RecordAlertFired(alert)
	for i := 0; i < 10; i++ {
		store.RecordAnalysis(alert.ID, "Analysis "+string(rune('A'+i)), nil)
	}

	// Request only 3 events
	result := store.FormatForAlert(alert.ID, 3)
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Should only have last 3 events in output (count occurrences of timestamps)
	lines := strings.Split(result, "\n")
	eventLines := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- 20") { // timestamp starts with year
			eventLines++
		}
	}
	if eventLines > 3 {
		t.Errorf("expected max 3 event lines, got %d", eventLines)
	}
}

func TestIncidentStore_FormatForAlert_NoIncident(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	result := store.FormatForAlert("nonexistent-alert", 10)
	if result != "" {
		t.Errorf("expected empty string for non-existent alert, got %q", result)
	}
}

func TestIncidentStore_FormatForResource(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 20,
		MaxAgeDays:   30,
	})

	// Create multiple incidents for the same resource
	for i := 0; i < 3; i++ {
		alert := &alerts.Alert{
			ID:           "alert-res-" + string(rune('A'+i)),
			Type:         "disk",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "res-format-res",
			ResourceName: "storage-format",
			StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
		}
		store.RecordAlertFired(alert)
		if i == 0 {
			store.RecordAlertAcknowledged(alert, "admin")
		}
	}

	result := store.FormatForResource("res-format-res", 5)

	if result == "" {
		t.Fatal("expected non-empty format result")
	}

	if !strings.Contains(result, "## Incident Memory") {
		t.Error("expected '## Incident Memory' header")
	}
	if !strings.Contains(result, "Recent incidents for this resource") {
		t.Error("expected resource incidents header")
	}
	if !strings.Contains(result, "disk") {
		t.Error("expected alert type in output")
	}
	// First incident should show as acknowledged
	if !strings.Contains(result, "acknowledged") {
		t.Error("expected 'acknowledged' status for first incident")
	}
}

func TestIncidentStore_FormatForResource_NoIncidents(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	result := store.FormatForResource("nonexistent-resource", 5)
	if result != "" {
		t.Errorf("expected empty string for resource with no incidents, got %q", result)
	}
}

func TestIncidentStore_FormatForPatrol(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 20,
		MaxAgeDays:   30,
	})

	// Create incidents for multiple resources
	resources := []string{"vm-1", "vm-2", "storage-1"}
	for i, resName := range resources {
		alert := &alerts.Alert{
			ID:           "alert-patrol-" + string(rune('A'+i)),
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "res-patrol-" + string(rune('1'+i)),
			ResourceName: resName,
			StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
			Message:      "High usage detected",
		}
		store.RecordAlertFired(alert)
		store.RecordAnalysis(alert.ID, "Analysis for "+resName, nil)
	}

	result := store.FormatForPatrol(10)

	if result == "" {
		t.Fatal("expected non-empty format result")
	}

	if !strings.Contains(result, "## Incident Memory") {
		t.Error("expected '## Incident Memory' header")
	}
	if !strings.Contains(result, "Recent incidents across infrastructure") {
		t.Error("expected infrastructure-wide header")
	}
	// Should contain resource names
	for _, resName := range resources {
		if !strings.Contains(result, resName) {
			t.Errorf("expected resource name %q in output", resName)
		}
	}
}

func TestIncidentStore_FormatForPatrol_WithLimit(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 20,
	})

	// Create 10 incidents
	for i := 0; i < 10; i++ {
		alert := &alerts.Alert{
			ID:           "alert-limit-" + string(rune('A'+i)),
			Type:         "memory",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "res-" + string(rune('A'+i)),
			ResourceName: "vm-" + string(rune('A'+i)),
		}
		store.RecordAlertFired(alert)
	}

	// Request only 3
	result := store.FormatForPatrol(3)

	// Count incident lines
	lines := strings.Split(result, "\n")
	incidentLines := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- 20") {
			incidentLines++
		}
	}
	if incidentLines > 3 {
		t.Errorf("expected max 3 incident lines, got %d", incidentLines)
	}
}

func TestIncidentStore_FormatForPatrol_Empty(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	result := store.FormatForPatrol(10)
	if result != "" {
		t.Errorf("expected empty string for empty store, got %q", result)
	}
}

func TestIncidentStore_FormatForPatrol_DefaultLimit(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 20,
	})

	// Create 15 incidents
	for i := 0; i < 15; i++ {
		alert := &alerts.Alert{
			ID:           "alert-default-" + string(rune('A'+i)),
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "res-" + string(rune('A'+i)),
			ResourceName: "vm-" + string(rune('A'+i)),
		}
		store.RecordAlertFired(alert)
	}

	// Pass 0 limit - should use default of 8
	result := store.FormatForPatrol(0)

	// Count incident lines - should be at most 8
	lines := strings.Split(result, "\n")
	incidentLines := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- 20") {
			incidentLines++
		}
	}
	if incidentLines > 8 {
		t.Errorf("expected max 8 incident lines (default), got %d", incidentLines)
	}
}

func TestIncidentStore_RecordNote_ByIncidentID(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	alert := &alerts.Alert{
		ID:           "alert-note-id",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "res-note",
		ResourceName: "vm-note",
	}
	store.RecordAlertFired(alert)

	timeline := store.GetTimelineByAlertID(alert.ID)
	if timeline == nil {
		t.Fatal("expected timeline")
	}

	// Add note by incident ID
	ok := store.RecordNote("", timeline.ID, "Test note by incident ID", "operator")
	if !ok {
		t.Fatal("expected note to be saved by incident ID")
	}

	timeline = store.GetTimelineByAlertID(alert.ID)
	foundNoteEvent := false
	for _, evt := range timeline.Events {
		if evt.Type == IncidentEventNote {
			foundNoteEvent = true
			if note, ok := evt.Details["note"].(string); !ok || note != "Test note by incident ID" {
				t.Errorf("expected note text, got %v", evt.Details["note"])
			}
		}
	}
	if !foundNoteEvent {
		t.Error("expected to find note event")
	}
}

func TestIncidentStore_RecordNote_EmptyNote(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	alert := &alerts.Alert{
		ID:         "alert-empty-note",
		ResourceID: "res-1",
	}
	store.RecordAlertFired(alert)

	// Empty note should return false
	ok := store.RecordNote(alert.ID, "", "", "admin")
	if ok {
		t.Error("expected false for empty note")
	}

	// Whitespace-only note should return false
	ok = store.RecordNote(alert.ID, "", "   ", "admin")
	if ok {
		t.Error("expected false for whitespace-only note")
	}
}

func TestIncidentStore_RecordNote_NonexistentIncident(t *testing.T) {
	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents: 10,
	})

	ok := store.RecordNote("nonexistent-alert", "", "Test note", "admin")
	if ok {
		t.Error("expected false for non-existent alert")
	}

	ok = store.RecordNote("", "nonexistent-incident", "Test note", "admin")
	if ok {
		t.Error("expected false for non-existent incident")
	}
}
