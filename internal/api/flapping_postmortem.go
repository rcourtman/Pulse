package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// emitFlappingPostmortemFinding writes a reliability finding to the findings
// store explaining what is flapping and why Pulse suppressed it. The finding
// ID is derived from the alert's canonical tracking key so re-detection
// inside the cooldown window updates the existing finding instead of
// creating a duplicate.
//
// This is Path B from the lane brief: emit the finding directly at callback
// time so it is durable without depending on patrol synthesis. The scoped
// patrol still runs in parallel to enrich context once it lands.
func emitFlappingPostmortemFinding(patrol *ai.PatrolService, alertManager *alerts.Manager, alert *alerts.Alert, trackingKey string) {
	if patrol == nil || alert == nil || trackingKey == "" {
		return
	}
	store := patrol.GetFindings()
	if store == nil {
		log.Debug().
			Str("trackingKey", trackingKey).
			Msg("Flapping postmortem: findings store unavailable, skipping finding emission")
		return
	}

	cfg := alertManager.GetConfig()
	now := time.Now()

	resourceName := strings.TrimSpace(alert.ResourceName)
	if resourceName == "" {
		resourceName = strings.TrimSpace(alert.ResourceID)
	}
	if resourceName == "" {
		resourceName = "resource"
	}
	alertType := strings.TrimSpace(alert.Type)
	if alertType == "" {
		alertType = "alert"
	}

	title := fmt.Sprintf("Alert %s on %s is flapping", alertType, resourceName)
	description := fmt.Sprintf(
		"Pulse detected this alert switching state %d or more times within %s and suppressed further notifications for %s. Suppression is in effect to avoid alarm-storm noise; the underlying condition has NOT been fixed.",
		cfg.FlappingThreshold,
		formatSecondsDuration(cfg.FlappingWindowSeconds),
		formatMinutesDuration(cfg.FlappingCooldownMinutes),
	)
	recommendation := "Consider widening the threshold (so transient blips do not toggle the alert), raising the flapping cooldown, or stabilising the underlying resource. If the resource really is unstable, investigating why it crosses the threshold so often is the durable fix."

	findingID := "alert-flapping:" + trackingKey
	finding := &ai.Finding{
		ID:              findingID,
		Key:             findingID,
		Severity:        ai.FindingSeverityWarning,
		Category:        ai.FindingCategoryReliability,
		ResourceID:      alert.ResourceID,
		ResourceName:    resourceName,
		ResourceType:    alert.CanonicalKind,
		Node:            alert.Node,
		Title:           title,
		Description:     description,
		Impact:          "Alarm-storm flapping is being silenced. Real problems on this resource may be missed until you tune the alert or fix the instability.",
		Recommendation:  recommendation,
		Evidence:        fmt.Sprintf("trackingKey=%s alertType=%s threshold=%d windowSeconds=%d cooldownMinutes=%d", trackingKey, alertType, cfg.FlappingThreshold, cfg.FlappingWindowSeconds, cfg.FlappingCooldownMinutes),
		Source:          "alert-flapping",
		DetectedAt:      now,
		LastSeenAt:      now,
		AlertIdentifier: alert.ID,
	}

	if store.Add(finding) {
		log.Info().
			Str("findingID", findingID).
			Str("trackingKey", trackingKey).
			Str("resourceID", alert.ResourceID).
			Str("alertType", alertType).
			Msg("Emitted flapping postmortem finding")
	} else {
		log.Debug().
			Str("findingID", findingID).
			Str("trackingKey", trackingKey).
			Msg("Flapping postmortem finding deduped against existing record")
	}
}

func formatSecondsDuration(seconds int) string {
	if seconds <= 0 {
		return "the configured window"
	}
	d := time.Duration(seconds) * time.Second
	return d.String()
}

func formatMinutesDuration(minutes int) string {
	if minutes <= 0 {
		return "the configured cooldown"
	}
	d := time.Duration(minutes) * time.Minute
	return d.String()
}
