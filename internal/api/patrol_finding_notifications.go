package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// patrolFindingAlertIDPrefix namespaces synthetic notification alerts so
// their IDs can never collide with threshold alert IDs in the notification
// manager's cooldown map.
const patrolFindingAlertIDPrefix = "patrol-finding-"

// patrolFindingNotificationAlert projects a Patrol finding into the alert
// shape the notification manager delivers, so findings reach the operator's
// existing email, webhook, and Apprise destinations. The synthetic alert is
// never stored in the active-alert store; it exists only for delivery.
func patrolFindingNotificationAlert(f *ai.Finding) *alerts.Alert {
	if f == nil {
		return nil
	}

	level := alerts.AlertLevelWarning
	if f.Severity == ai.FindingSeverityCritical {
		level = alerts.AlertLevelCritical
	}

	startTime := f.DetectedAt
	if startTime.IsZero() {
		startTime = time.Now()
	}
	lastSeen := f.LastSeenAt
	if lastSeen.IsZero() {
		lastSeen = startTime
	}

	metadata := map[string]interface{}{
		"findingId": f.ID,
		"category":  string(f.Category),
	}
	if f.Description != "" {
		metadata["description"] = f.Description
	}
	if f.Recommendation != "" {
		metadata["recommendation"] = f.Recommendation
	}

	return &alerts.Alert{
		ID:           patrolFindingAlertIDPrefix + f.ID,
		Type:         "patrol_finding",
		Level:        level,
		ResourceID:   f.ResourceID,
		ResourceName: f.ResourceName,
		Node:         f.Node,
		Message:      f.Title,
		StartTime:    startTime,
		LastSeen:     lastSeen,
		Metadata:     metadata,
	}
}
