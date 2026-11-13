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

func addNotificationTagValue(target map[string]struct{}, raw string) {
	if tag := strings.ToLower(strings.TrimSpace(raw)); tag != "" {
		target[tag] = struct{}{}
	}
}

// Collect directly into one call-local membership set. Arrays contain literal
// tags (including commas/semicolons); only a scalar string is delimited.
func addNotificationTagValues(target map[string]struct{}, raw interface{}) {
	switch value := raw.(type) {
	case string:
		for part := range strings.FieldsFuncSeq(value, func(r rune) bool {
			return r == ',' || r == ';'
		}) {
			addNotificationTagValue(target, part)
		}
	case []string:
		for _, tag := range value {
			addNotificationTagValue(target, tag)
		}
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok {
				addNotificationTagValue(target, text)
			}
		}
	}
}

// prepareNotificationTagFilter returns an owned, lowercase filter for one
// destination batch. Never lowercase the saved configuration's slice in place.
func prepareNotificationTagFilter(filter []string) []string {
	filter = normalizeNotificationTagFilter(filter)
	for i := range filter {
		filter[i] = strings.ToLower(filter[i])
	}
	return filter
}

func notificationAlertMatchesTags(alert *alerts.Alert, filter []string, mode string) bool {
	return notificationAlertMatchesPreparedTags(alert, prepareNotificationTagFilter(filter), normalizeNotificationTagMode(mode))
}

// filter and mode have already been prepared for this destination. Alert tags
// keep the existing parsing and ToLower identity, including non-ASCII values.
func notificationAlertMatchesPreparedTags(alert *alerts.Alert, filter []string, mode string) bool {
	if len(filter) == 0 {
		return true
	}

	if alert == nil || alert.Metadata == nil {
		return false
	}

	tagSet := make(map[string]struct{})
	for _, key := range []string{"tags", "resourceTags", "hostTags", "serviceTags"} {
		addNotificationTagValues(tagSet, alert.Metadata[key])
	}

	if mode == notificationTagModeAny {
		for _, required := range filter {
			if _, exists := tagSet[required]; exists {
				return true
			}
		}
		return false
	}

	for _, required := range filter {
		if _, exists := tagSet[required]; !exists {
			return false
		}
	}
	return true
}

func notificationAlertMeetsMinimumSeverity(alert *alerts.Alert, minimumSeverity string) bool {
	return notificationAlertMeetsPreparedMinimumSeverity(alert, normalizeNotificationMinimumSeverity(minimumSeverity))
}

func notificationAlertMeetsPreparedMinimumSeverity(alert *alerts.Alert, minimumSeverity string) bool {
	if alert == nil {
		return false
	}
	level := alerts.NormalizeAlertLevel(alert.Level)
	switch minimumSeverity {
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

	normalizedFilter := prepareNotificationTagFilter(filter)
	normalizedMinimumSeverity := normalizeNotificationMinimumSeverity(minimumSeverity)
	if len(normalizedFilter) == 0 && normalizedMinimumSeverity == notificationMinimumSeverityAll {
		return alertList
	}

	normalizedMode := normalizeNotificationTagMode(mode)
	routed := make([]*alerts.Alert, 0, len(alertList))
	for _, alert := range alertList {
		// A severity rejection needs no tag parsing or temporary tag set.
		if notificationAlertMeetsPreparedMinimumSeverity(alert, normalizedMinimumSeverity) &&
			notificationAlertMatchesPreparedTags(alert, normalizedFilter, normalizedMode) {
			routed = append(routed, alert)
		}
	}
	return routed
}
