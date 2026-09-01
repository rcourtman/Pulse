package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ResourceHealthVerdict is the canonical fleet-posture vocabulary shared by
// resource payloads and the lightweight state summary. It deliberately keeps
// powered-off workloads separate from failures and stale data separate from a
// verified all-clear.
type ResourceHealthVerdict string

const (
	HealthOK        ResourceHealthVerdict = "ok"
	HealthAttention ResourceHealthVerdict = "attention"
	HealthCritical  ResourceHealthVerdict = "critical"
	HealthStale     ResourceHealthVerdict = "stale"
	HealthOff       ResourceHealthVerdict = "off"
	HealthUnknown   ResourceHealthVerdict = "unknown"
)

const resourceBackupStaleAfter = 7 * 24 * time.Hour

type ResourceHealthReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

type ResourceHealth struct {
	Verdict ResourceHealthVerdict  `json:"verdict"`
	Reasons []ResourceHealthReason `json:"reasons"`
}

// ResourceHealthAlert is the minimal alert contract needed to compute fleet
// posture without coupling the unified-resource package to alert internals.
type ResourceHealthAlert struct {
	ResourceID string
	Level      string
	Type       string
}

// EvaluateResourceHealth returns one deterministic verdict. Precedence is
// significant: live alert evidence wins over stale telemetry, while a stopped
// workload is neutral only when no warning or critical evidence exists.
func EvaluateResourceHealth(resource Resource, alerts []ResourceHealthAlert, now time.Time) ResourceHealth {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	matchingAlerts := matchingResourceHealthAlerts(resource, alerts)
	for _, alert := range matchingAlerts {
		if strings.EqualFold(strings.TrimSpace(alert.Level), "critical") {
			return healthWithReason(HealthCritical, "critical_alert", strings.TrimSpace(alert.Type))
		}
	}

	if availabilityFailureConfirmed(resource) {
		return healthWithReason(HealthCritical, "availability_failed", availabilityFailureDetail(resource))
	}
	if strings.EqualFold(strings.TrimSpace(string(resource.IncidentSeverity)), "critical") {
		return healthWithReason(HealthCritical, firstNonEmptyHealthCode(resource.IncidentCode, "critical_incident"), "")
	}
	if isInfrastructureHealthResource(resource) && resourceIsOffline(resource) {
		return healthWithReason(HealthCritical, "offline", "")
	}

	if len(matchingAlerts) > 0 {
		return healthWithReason(HealthAttention, "warning_alert", strings.TrimSpace(matchingAlerts[0].Type))
	}
	if resource.IncidentSeverity != "" && !strings.EqualFold(strings.TrimSpace(string(resource.IncidentSeverity)), "none") {
		return healthWithReason(HealthAttention, firstNonEmptyHealthCode(resource.IncidentCode, "resource_risk"), "")
	}
	if backupAge, stale := staleResourceBackup(resource, now); stale {
		return healthWithReason(HealthAttention, "backup_stale", fmt.Sprintf("%dd", int(backupAge.Hours()/24)))
	}

	if resourceHasStaleSource(resource) {
		return healthWithReason(HealthStale, "telemetry_stale", formatHealthAge(now.Sub(resource.LastSeen)))
	}
	if resourceHasUnknownSource(resource) && resource.LastSeen.IsZero() {
		return healthWithReason(HealthUnknown, "telemetry_missing", "")
	}
	if isWorkloadHealthResource(resource) && resourceIsOff(resource) {
		return healthWithReason(HealthOff, "powered_off", "")
	}

	switch strings.ToLower(strings.TrimSpace(string(resource.Status))) {
	case "online", "running", "ready", "healthy", "available", "active", "up":
		// A positive provider status without an observation timestamp is not
		// enough to prove current health. Some compatibility and synthetic
		// producers do not populate SourceStatus, so keep this guard at the
		// canonical verdict boundary rather than relying on that map alone.
		if resource.LastSeen.IsZero() {
			return healthWithReason(HealthUnknown, "telemetry_missing", "")
		}
		return ResourceHealth{Verdict: HealthOK, Reasons: []ResourceHealthReason{}}
	case "warning", "degraded", "unhealthy", "pending":
		return healthWithReason(HealthAttention, "degraded", "")
	case "offline", "down", "failed", "error":
		return healthWithReason(HealthCritical, "offline", "")
	default:
		return healthWithReason(HealthUnknown, "status_unknown", "")
	}
}

// AttachResourceHealth returns a shallow copy with backend-owned health
// envelopes. Callers can safely pass registry-backed slices without mutating
// the canonical store.
func AttachResourceHealth(resources []Resource, alerts []ResourceHealthAlert, now time.Time) []Resource {
	if len(resources) == 0 {
		return resources
	}
	out := make([]Resource, len(resources))
	copy(out, resources)
	for i := range out {
		health := EvaluateResourceHealth(out[i], alerts, now)
		out[i].Health = &health
	}
	return out
}

func healthWithReason(verdict ResourceHealthVerdict, code, detail string) ResourceHealth {
	return ResourceHealth{
		Verdict: verdict,
		Reasons: []ResourceHealthReason{{Code: strings.TrimSpace(code), Detail: strings.TrimSpace(detail)}},
	}
}

func matchingResourceHealthAlerts(resource Resource, alerts []ResourceHealthAlert) []ResourceHealthAlert {
	ids := map[string]struct{}{}
	add := func(value string) {
		if normalized := strings.TrimSpace(value); normalized != "" {
			ids[normalized] = struct{}{}
		}
	}
	add(resource.ID)
	for _, id := range resource.SupersededCanonicalIDs {
		add(id)
	}
	if resource.Canonical != nil {
		add(resource.Canonical.PrimaryID)
		for _, id := range resource.Canonical.SupersededIDs {
			add(id)
		}
	}
	if resource.Proxmox != nil {
		add(resource.Proxmox.SourceID)
	}

	matching := make([]ResourceHealthAlert, 0, 1)
	for _, alert := range alerts {
		if _, ok := ids[strings.TrimSpace(alert.ResourceID)]; ok && healthAlertSeverity(alert.Level) > 0 {
			matching = append(matching, alert)
		}
	}
	sort.SliceStable(matching, func(i, j int) bool {
		return healthAlertSeverity(matching[i].Level) > healthAlertSeverity(matching[j].Level)
	})
	return matching
}

func healthAlertSeverity(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return 2
	case "warning", "warn":
		return 1
	default:
		return 0
	}
}

func availabilityFailureConfirmed(resource Resource) bool {
	if resource.Availability == nil || !resource.Availability.Enabled || resource.Availability.Available {
		return false
	}
	threshold := resource.Availability.FailureThreshold
	if threshold <= 0 {
		threshold = 1
	}
	return resource.Availability.ConsecutiveFailures >= threshold
}

func availabilityFailureDetail(resource Resource) string {
	if resource.Availability == nil {
		return ""
	}
	// Keep operator/configured error text out of fleet summaries. The stable
	// application failure code is enough for presentation and cannot contain a
	// probed response body, credential, or customer hostname.
	return strings.TrimSpace(resource.Availability.ApplicationFailureCode)
}

func firstNonEmptyHealthCode(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isWorkloadHealthResource(resource Resource) bool {
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeVM, ResourceTypeSystemContainer, ResourceTypeAppContainer, ResourceTypePod:
		return true
	default:
		return false
	}
}

func isInfrastructureHealthResource(resource Resource) bool {
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent, ResourceTypePBS, ResourceTypePMG, ResourceTypeK8sCluster, ResourceTypeK8sNode:
		return true
	default:
		return false
	}
}

func resourceIsOffline(resource Resource) bool {
	switch strings.ToLower(strings.TrimSpace(string(resource.Status))) {
	case "offline", "down", "failed", "error":
		return true
	default:
		return false
	}
}

func resourceIsOff(resource Resource) bool {
	if resource.Proxmox != nil {
		switch strings.ToLower(strings.TrimSpace(resource.Proxmox.RuntimeStatus)) {
		case "stopped", "offline", "off", "halted", "paused", "suspended":
			return true
		}
	}
	switch strings.ToLower(strings.TrimSpace(string(resource.Status))) {
	case "stopped", "offline", "off", "halted", "paused", "suspended":
		return true
	default:
		return false
	}
}

func staleResourceBackup(resource Resource, now time.Time) (time.Duration, bool) {
	if resource.Proxmox == nil || resource.Proxmox.LastBackup.IsZero() {
		return 0, false
	}
	resourceType := CanonicalResourceType(resource.Type)
	if resourceType != ResourceTypeVM && resourceType != ResourceTypeSystemContainer {
		return 0, false
	}
	age := now.Sub(resource.Proxmox.LastBackup)
	return age, age > resourceBackupStaleAfter
}

func resourceHasStaleSource(resource Resource) bool {
	for _, source := range resource.SourceStatus {
		if strings.EqualFold(strings.TrimSpace(source.Status), "stale") {
			return true
		}
	}
	return false
}

func resourceHasUnknownSource(resource Resource) bool {
	if len(resource.SourceStatus) == 0 {
		return false
	}
	for _, source := range resource.SourceStatus {
		if strings.EqualFold(strings.TrimSpace(source.Status), "unknown") {
			return true
		}
	}
	return false
}

func formatHealthAge(age time.Duration) string {
	if age <= 0 {
		return ""
	}
	if age >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(age.Hours()/24))
	}
	if age >= time.Hour {
		return fmt.Sprintf("%dh", int(age.Hours()))
	}
	return fmt.Sprintf("%dm", int(age.Minutes()))
}
