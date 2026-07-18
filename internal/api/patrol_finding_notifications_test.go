package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestPatrolFindingNotificationAlert(t *testing.T) {
	t.Run("nil finding yields nil alert", func(t *testing.T) {
		if patrolFindingNotificationAlert(nil) != nil {
			t.Fatal("nil finding must map to nil alert")
		}
	})

	t.Run("critical finding maps to critical alert", func(t *testing.T) {
		detected := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
		lastSeen := detected.Add(30 * time.Minute)
		finding := &ai.Finding{
			ID:             "f-123",
			Severity:       ai.FindingSeverityCritical,
			Category:       ai.FindingCategoryReliability,
			ResourceID:     "docker:abc123",
			ResourceName:   "app-container",
			Node:           "edge-host",
			Title:          "Container health check is failing",
			Description:    "The running container is reported unhealthy.",
			Recommendation: "Inspect the container health-check output.",
			DetectedAt:     detected,
			LastSeenAt:     lastSeen,
		}

		alert := patrolFindingNotificationAlert(finding)
		if alert == nil {
			t.Fatal("expected an alert")
		}
		if alert.ID != "patrol-finding-f-123" {
			t.Fatalf("ID = %q", alert.ID)
		}
		if alert.Type != "patrol_finding" {
			t.Fatalf("Type = %q", alert.Type)
		}
		if alert.Level != alerts.AlertLevelCritical {
			t.Fatalf("Level = %q", alert.Level)
		}
		if alert.ResourceID != finding.ResourceID || alert.ResourceName != finding.ResourceName || alert.Node != finding.Node {
			t.Fatalf("resource identity mismatch: %+v", alert)
		}
		if alert.Message != finding.Title {
			t.Fatalf("Message = %q", alert.Message)
		}
		if !alert.StartTime.Equal(detected) || !alert.LastSeen.Equal(lastSeen) {
			t.Fatalf("timestamps mismatch: start=%v lastSeen=%v", alert.StartTime, alert.LastSeen)
		}
		if alert.Metadata["findingId"] != "f-123" {
			t.Fatalf("metadata findingId = %v", alert.Metadata["findingId"])
		}
		if alert.Metadata["category"] != string(ai.FindingCategoryReliability) {
			t.Fatalf("metadata category = %v", alert.Metadata["category"])
		}
		if alert.Metadata["description"] != finding.Description {
			t.Fatalf("metadata description = %v", alert.Metadata["description"])
		}
		if alert.Metadata["recommendation"] != finding.Recommendation {
			t.Fatalf("metadata recommendation = %v", alert.Metadata["recommendation"])
		}
	})

	t.Run("warning finding maps to warning alert with zero-time fallbacks", func(t *testing.T) {
		finding := &ai.Finding{
			ID:       "f-456",
			Severity: ai.FindingSeverityWarning,
			Category: ai.FindingCategoryGeneral,
			Title:    "Array running without parity protection",
		}

		alert := patrolFindingNotificationAlert(finding)
		if alert == nil {
			t.Fatal("expected an alert")
		}
		if alert.Level != alerts.AlertLevelWarning {
			t.Fatalf("Level = %q", alert.Level)
		}
		if alert.StartTime.IsZero() || alert.LastSeen.IsZero() {
			t.Fatal("zero finding timestamps must fall back to a real time")
		}
		if _, present := alert.Metadata["description"]; present {
			t.Fatal("empty description must not be present in metadata")
		}
		if _, present := alert.Metadata["recommendation"]; present {
			t.Fatal("empty recommendation must not be present in metadata")
		}
	})
}
