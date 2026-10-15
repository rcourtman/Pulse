package notifications

import (
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// The Pulse routes notifications link to. The frontend owns the same
// addresses (ALERT_HISTORY_PATH and buildAlertHistoryAlertPath in
// frontend-modern/src/routing/resourceLinks.ts): the `alert` parameter opens
// Alert History narrowed to that alert with its latest timeline open.
const (
	alertHistoryPath     = "/alerts/history"
	alertHistoryAlertKey = "alert"
	alertsOverviewPath   = "/alerts/overview"
	// Patrol findings reach notifications as synthetic alerts that Alert
	// History never stores, so they link to the Patrol page instead.
	patrolPath = "/patrol"
)

func trimmedPublicURL(publicURL string) string {
	return strings.TrimRight(strings.TrimSpace(publicURL), "/")
}

// alertHistoryURL is the Pulse address that opens one alert in Alert
// History. It is empty when no public URL is configured or the alert has no
// identifier, so callers keep their existing fallback.
func alertHistoryURL(publicURL, alertID string) string {
	base := trimmedPublicURL(publicURL)
	id := strings.TrimSpace(alertID)
	if base == "" || id == "" {
		return ""
	}
	return base + alertHistoryPath + "?" + url.Values{alertHistoryAlertKey: {id}}.Encode()
}

// alertLinkURL is the Pulse address for one notified alert: its Alert
// History entry, or the Patrol page for a Patrol finding.
func alertLinkURL(publicURL string, alert *alerts.Alert) string {
	if alert == nil {
		return ""
	}
	if isPatrolFindingAlert(alert) {
		if base := trimmedPublicURL(publicURL); base != "" {
			return base + patrolPath
		}
		return ""
	}
	return alertHistoryURL(publicURL, alert.ID)
}

// alertsPageURL is where a notification about these alerts links: one alert
// opens in Alert History; several open the page that lists them, the
// overview while they fire and History once they have resolved.
func alertsPageURL(publicURL string, alertList []*alerts.Alert, resolved bool) string {
	base := trimmedPublicURL(publicURL)
	if base == "" {
		return ""
	}
	var only *alerts.Alert
	count, findings := 0, 0
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		count++
		if isPatrolFindingAlert(alert) {
			findings++
		}
		only = alert
	}
	if count == 1 {
		if link := alertLinkURL(base, only); link != "" {
			return link
		}
	}
	// A batch of Patrol findings alone lists nowhere under Alerts.
	if count > 0 && findings == count {
		return base + patrolPath
	}
	if resolved {
		return base + alertHistoryPath
	}
	return base + alertsOverviewPath
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
