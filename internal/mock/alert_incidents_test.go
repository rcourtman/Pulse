package mock

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestBuildAlertIncidentFixturesCreatesCompleteLifecycleData(t *testing.T) {
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	history := []models.Alert{
		{
			ID:           "hist-complete-lifecycle",
			Type:         "threshold",
			Level:        "critical",
			ResourceID:   "vm-101",
			ResourceName: "database",
			StartTime:    now.Add(-30 * time.Minute),
			Value:        94,
			Threshold:    80,
			Metadata:     map[string]interface{}{"resourceType": "vm"},
		},
		{
			ID:           "active-complete-lifecycle",
			Type:         "connectivity",
			Level:        "critical",
			ResourceID:   "node-1",
			ResourceName: "pve-1",
			StartTime:    now.Add(-90 * time.Second),
			Metadata:     map[string]interface{}{"resourceType": "node"},
		},
	}

	enriched, incidents := buildAlertIncidentFixtures(history, now)
	if len(enriched) != 2 || len(incidents) != 2 {
		t.Fatalf("got %d enriched alerts and %d incidents, want 2 of each", len(enriched), len(incidents))
	}

	resolved := incidents[0]
	if resolved.Status != memory.IncidentStatusResolved || resolved.ClosedAt == nil {
		t.Fatalf("historical incident status = %q, closedAt = %v; want resolved lifecycle", resolved.Status, resolved.ClosedAt)
	}
	if len(resolved.Events) < 3 {
		t.Fatalf("historical incident has %d events, want fired, analysis, and resolved", len(resolved.Events))
	}
	if resolved.Events[0].Type != memory.IncidentEventAlertFired || resolved.Events[len(resolved.Events)-1].Type != memory.IncidentEventAlertResolved {
		t.Fatalf("historical lifecycle endpoints = %q ... %q", resolved.Events[0].Type, resolved.Events[len(resolved.Events)-1].Type)
	}
	if resolved.ResourceType != "vm" {
		t.Fatalf("resource type = %q, want vm", resolved.ResourceType)
	}
	if enriched[0].LastSeen == nil || resolved.ClosedAt == nil || !enriched[0].LastSeen.Equal(*resolved.ClosedAt) {
		t.Fatalf("historical row lastSeen = %v, closedAt = %v; want one resolution time", enriched[0].LastSeen, resolved.ClosedAt)
	}

	active := incidents[1]
	if active.Status != memory.IncidentStatusOpen || active.ClosedAt != nil {
		t.Fatalf("active incident status = %q, closedAt = %v; want open lifecycle", active.Status, active.ClosedAt)
	}
	if enriched[1].LastSeen == nil || !enriched[1].LastSeen.Equal(now) {
		t.Fatalf("active row lastSeen = %v, want fixture observation time %v", enriched[1].LastSeen, now)
	}
	for _, event := range active.Events {
		if event.Timestamp.After(now) {
			t.Fatalf("active incident event %q is in the future: %s", event.Type, event.Timestamp)
		}
	}

	for index, incident := range incidents {
		if enriched[index].Acknowledged != incident.Acknowledged || enriched[index].AckUser != incident.AckUser {
			t.Fatalf("alert and incident acknowledgement state diverged for %s", incident.AlertIdentifier)
		}
	}
}

func TestMockAlertIncidentQueriesAndNotesShareCanonicalFixture(t *testing.T) {
	resetMockIntegrationState(t)
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	history := []models.Alert{
		{ID: "hist-resource-new", Type: "threshold", Level: "warning", ResourceID: "vm-101", StartTime: now.Add(-10 * time.Minute)},
		{ID: "hist-resource-old", Type: "backup", Level: "critical", ResourceID: "vm-101", StartTime: now.Add(-2 * time.Hour)},
	}
	enriched, incidents := buildAlertIncidentFixtures(history, now)

	dataMu.Lock()
	mockGraph.AlertHistory = enriched
	mockGraph.AlertIncidents = incidents
	dataMu.Unlock()
	enabled.Store(true)

	timeline := GetMockAlertIncidentTimeline(enriched[0].ID, enriched[0].StartTime)
	if timeline == nil || timeline.AlertIdentifier != enriched[0].ID {
		t.Fatalf("exact occurrence lookup returned %#v", timeline)
	}
	timeline.Events[0].Summary = "mutated by caller"
	canonical := GetMockAlertIncidentTimeline(enriched[0].ID, enriched[0].StartTime)
	if canonical.Events[0].Summary == "mutated by caller" {
		t.Fatal("timeline lookup exposed the canonical fixture to caller mutation")
	}

	resourceIncidents := GetMockAlertIncidentsForResource("vm-101", 1)
	if len(resourceIncidents) != 1 || resourceIncidents[0].AlertIdentifier != "hist-resource-new" {
		t.Fatalf("resource timeline = %#v, want newest matching incident", resourceIncidents)
	}

	beforeVersion := fixtureDataVersion.Load()
	if !AddMockAlertIncidentNote(enriched[0].ID, "", "Investigating storage latency", "operator") {
		t.Fatal("AddMockAlertIncidentNote returned false for canonical mock incident")
	}
	updated := GetMockAlertIncidentTimeline(enriched[0].ID, enriched[0].StartTime)
	last := updated.Events[len(updated.Events)-1]
	if last.Type != memory.IncidentEventNote || last.Details["note"] != "Investigating storage latency" {
		t.Fatalf("last event = %#v, want persisted note", last)
	}
	if fixtureDataVersion.Load() != beforeVersion+1 {
		t.Fatalf("fixture data version did not advance after note mutation")
	}
}
