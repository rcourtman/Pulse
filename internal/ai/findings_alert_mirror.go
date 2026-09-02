// findings_alert_mirror.go decides when a Patrol finding restates an active
// alert. Real-time alerts own down, threshold, and age conditions; Patrol's
// deterministic watchers (Proxmox guest lifecycle, PDM bridge, signal
// detection) and the model can still emit a finding for the same resource
// and condition. Rather than dropping those findings, Patrol stamps them
// with the alert they mirror after every run so the attention surface can
// fold them under the alert and the findings list can demote them into an
// expansion. The matcher is deliberately conservative: it requires the same
// canonical resource and a recognised condition class on both sides, or an
// explicit AlertIdentifier link.
package ai

import (
	"strings"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// alertMirrorCandidate is the minimal alert shape the matcher needs.
type alertMirrorCandidate struct {
	ID         string
	ResourceID string
	Type       string
}

// Condition classes shared by alerts and findings. An empty class never
// matches, so unknown alert types and free-form findings are left alone.
const (
	mirrorConditionDown            = "down"
	mirrorConditionRestartLoop     = "restart-loop"
	mirrorConditionContainerHealth = "container-health"
	mirrorConditionDiskCapacity    = "disk-capacity"
	mirrorConditionMemory          = "memory"
	mirrorConditionCPU             = "cpu"
	mirrorConditionBackupAge       = "backup-age"
	mirrorConditionSnapshotAge     = "snapshot-age"
	mirrorConditionTemperature     = "temperature"
)

// alertConditionClass maps an alert type onto the shared condition class.
func alertConditionClass(alertType string) string {
	normalized := strings.NewReplacer("_", "-", " ", "-").Replace(strings.ToLower(strings.TrimSpace(alertType)))
	switch normalized {
	case "offline", "connectivity", "powered-off", "docker-container-state", "docker-host-offline", "node-offline":
		return mirrorConditionDown
	case "docker-container-restart-loop":
		return mirrorConditionRestartLoop
	case "docker-container-health", "docker-service-health":
		return mirrorConditionContainerHealth
	case "disk", "usage", "storage-usage":
		return mirrorConditionDiskCapacity
	case "memory", "docker-container-memory-limit", "docker-container-oom-kill":
		return mirrorConditionMemory
	case "cpu":
		return mirrorConditionCPU
	case "backup-age":
		return mirrorConditionBackupAge
	case "snapshot-age":
		return mirrorConditionSnapshotAge
	case "temperature", "disk-temperature":
		return mirrorConditionTemperature
	}
	return ""
}

// findingConditionClass maps a finding onto the shared condition class using
// its stable key or dedup prefix first and its category plus title words
// second. Findings that do not describe an alert-owned condition return "".
func findingConditionClass(f *Finding) string {
	if f == nil {
		return ""
	}
	id := strings.ToLower(strings.TrimSpace(f.ID))
	key := strings.ToLower(strings.TrimSpace(f.Key))
	switch {
	case strings.HasPrefix(id, proxmoxGuestStoppedFindingIDPrefix), key == proxmoxGuestStoppedFindingKey:
		return mirrorConditionDown
	case strings.HasPrefix(id, strings.ToLower(PDMAlertFindingPrefix)):
		return mirrorConditionDown
	}

	text := strings.ToLower(f.Title)
	contains := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(text, word) {
				return true
			}
		}
		return false
	}
	switch f.Category {
	case FindingCategoryCapacity:
		switch {
		case contains("memory", "swap"):
			return mirrorConditionMemory
		case contains("disk", "storage", "pool", "datastore", "volume", "full", "%"):
			return mirrorConditionDiskCapacity
		}
	case FindingCategoryPerformance:
		switch {
		case contains("memory", "swap", "oom"):
			return mirrorConditionMemory
		case contains("cpu", "load"):
			return mirrorConditionCPU
		case contains("temperature", "thermal"):
			return mirrorConditionTemperature
		}
	case FindingCategoryReliability:
		switch {
		case contains("restarting repeatedly", "restart loop", "crash loop", "crashlooping"):
			return mirrorConditionRestartLoop
		case contains("unhealthy", "health check"):
			return mirrorConditionContainerHealth
		case contains("stopped", "exited", "offline", "powered off", "is down", "unreachable", "not running"):
			return mirrorConditionDown
		}
	case FindingCategoryBackup:
		if contains("snapshot") {
			return mirrorConditionSnapshotAge
		}
		return mirrorConditionBackupAge
	}
	return ""
}

// findingMirrorsAlert reports whether f restates alert: an explicit
// AlertIdentifier link, or the same canonical resource with the same
// condition class.
func findingMirrorsAlert(f *Finding, alert alertMirrorCandidate) bool {
	if f == nil {
		return false
	}
	alertID := strings.TrimSpace(alert.ID)
	if alertID != "" && strings.TrimSpace(f.AlertIdentifier) == alertID {
		return true
	}
	findingResource := unified.CanonicalResourceID(f.ResourceID)
	alertResource := unified.CanonicalResourceID(alert.ResourceID)
	if findingResource == "" || alertResource == "" || findingResource != alertResource {
		return false
	}
	class := findingConditionClass(f)
	return class != "" && class == alertConditionClass(alert.Type)
}

// matchFindingsToAlerts returns, for every unresolved finding that mirrors
// an active alert, the alert it mirrors. Storm meta-findings are never
// mirrors; they summarise several findings rather than one condition. When
// several alerts match, the first in the caller's order wins so the result
// is deterministic for a stable alert list.
func matchFindingsToAlerts(findings []*Finding, alerts []alertMirrorCandidate) map[string]findingAlertMirror {
	mirrors := make(map[string]findingAlertMirror)
	if len(findings) == 0 || len(alerts) == 0 {
		return mirrors
	}
	for _, f := range findings {
		if f == nil || f.ResolvedAt != nil || f.Source == stormFindingSource {
			continue
		}
		for _, alert := range alerts {
			if findingMirrorsAlert(f, alert) {
				mirrors[f.ID] = findingAlertMirror{
					AlertID:   strings.TrimSpace(alert.ID),
					AlertType: strings.TrimSpace(alert.Type),
				}
				break
			}
		}
	}
	return mirrors
}

// reconcileAlertMirrors re-stamps every unresolved finding against the
// current unscoped active-alert set. It runs at the end of every patrol
// cycle so a finding that mirrors an alert is folded while the alert is
// active and released once the alert resolves.
func (p *PatrolService) reconcileAlertMirrors() int {
	if p == nil || p.findings == nil || p.stateProvider == nil {
		return 0
	}
	snapshot := p.stateProvider.ReadSnapshot()
	candidates := make([]alertMirrorCandidate, 0, len(snapshot.ActiveAlerts))
	for _, alert := range snapshot.ActiveAlerts {
		candidates = append(candidates, alertMirrorCandidate{
			ID:         alert.ID,
			ResourceID: alert.ResourceID,
			Type:       alert.Type,
		})
	}
	findings := p.findings.GetAll(nil)
	return p.findings.StampAlertMirrors(matchFindingsToAlerts(findings, candidates))
}
