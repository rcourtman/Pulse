package notifications

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

const (
	notificationTagModeAll = "all"
	notificationTagModeAny = "any"
)

const (
	notificationMinimumSeverityAll      = "all"
	notificationMinimumSeverityWarning  = "warning"
	notificationMinimumSeverityCritical = "critical"
)

func normalizeNotificationMinimumSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case notificationMinimumSeverityWarning:
		return notificationMinimumSeverityWarning
	case notificationMinimumSeverityCritical:
		return notificationMinimumSeverityCritical
	default:
		return notificationMinimumSeverityAll
	}
}

func normalizeNotificationTagMode(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), notificationTagModeAny) {
		return notificationTagModeAny
	}
	return notificationTagModeAll
}

func normalizeNotificationTagFilter(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func notificationAlertTags(alert *alerts.Alert) []string {
	if alert == nil || alert.Metadata == nil {
		return nil
	}

	values := make([]string, 0)
	for _, key := range []string{"tags", "resourceTags", "hostTags", "serviceTags"} {
		values = appendNotificationTagValues(values, alert.Metadata[key])
	}
	return normalizeNotificationTagFilter(values)
}

func appendNotificationTagValues(target []string, raw interface{}) []string {
	switch value := raw.(type) {
	case string:
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';'
		}) {
			target = append(target, part)
		}
	case []string:
		target = append(target, value...)
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok {
				target = append(target, text)
			}
		}
	}
	return target
}

func notificationAlertMatchesTags(alert *alerts.Alert, filter []string, mode string) bool {
	filter = normalizeNotificationTagFilter(filter)
	if len(filter) == 0 {
		return true
	}

	alertTags := notificationAlertTags(alert)
	if len(alertTags) == 0 {
		return false
	}

	tagSet := make(map[string]struct{}, len(alertTags))
	for _, tag := range alertTags {
		tagSet[strings.ToLower(tag)] = struct{}{}
	}

	if normalizeNotificationTagMode(mode) == notificationTagModeAny {
		for _, required := range filter {
			if _, exists := tagSet[strings.ToLower(required)]; exists {
				return true
			}
		}
		return false
	}

	for _, required := range filter {
		if _, exists := tagSet[strings.ToLower(required)]; !exists {
			return false
		}
	}
	return true
}

func notificationAlertMeetsMinimumSeverity(alert *alerts.Alert, minimumSeverity string) bool {
	if alert == nil {
		return false
	}
	level := alerts.NormalizeAlertLevel(alert.Level)
	switch normalizeNotificationMinimumSeverity(minimumSeverity) {
	case notificationMinimumSeverityAll:
		return true
	case notificationMinimumSeverityWarning:
		return level == alerts.AlertLevelWarning || level == alerts.AlertLevelCritical
	case notificationMinimumSeverityCritical:
		return level == alerts.AlertLevelCritical
	default:
		return false
	}
}

func routeNotificationAlerts(
	alertList []*alerts.Alert,
	filter []string,
	mode string,
	minimumSeverity string,
	event notificationEvent,
) []*alerts.Alert {
	// Resolutions are routed by delivery receipts later in the pipeline.
	// Reapplying mutable tag or severity policy here could suppress the clear
	// after destination routing changed between firing and recovery.
	if event == eventResolved {
		return alertList
	}

	normalizedFilter := normalizeNotificationTagFilter(filter)
	normalizedMinimumSeverity := normalizeNotificationMinimumSeverity(minimumSeverity)
	if len(normalizedFilter) == 0 && normalizedMinimumSeverity == notificationMinimumSeverityAll {
		return alertList
	}

	routed := make([]*alerts.Alert, 0, len(alertList))
	for _, alert := range alertList {
		if notificationAlertMatchesTags(alert, normalizedFilter, mode) &&
			notificationAlertMeetsMinimumSeverity(alert, normalizedMinimumSeverity) {
			routed = append(routed, alert)
		}
	}
	return routed
}
