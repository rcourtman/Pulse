package api

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/maintenancesentinel"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// buildMaintenanceVerificationSentinel constructs the sentinel with
// adapters that pull alerts, findings, action audits, and recent
// metric samples out of the live runtime. Closures capture `r` so the
// providers always read the current monitor / aiSettings / store
// state (multi-tenant rebuilds replace those in place; latching to a
// snapshot would go stale).
//
// Returns nil when the resource handlers are not initialized — the
// caller treats nil as "sentinel disabled" and the API rerun endpoint
// returns 503.
func (r *Router) buildMaintenanceVerificationSentinel() *maintenancesentinel.Sentinel {
	if r == nil || r.resourceHandlers == nil {
		return nil
	}
	providers := maintenancesentinel.Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) {
			return r.resourceHandlers.getStore(orgID)
		},
		ActiveAlerts:            r.maintenanceVerificationActiveAlerts,
		ActiveFindings:          r.maintenanceVerificationActiveFindings,
		RecentActions:           r.maintenanceVerificationRecentActions,
		PostWindowMetricSamples: r.maintenanceVerificationMetricSamples,
		Now:                     func() time.Time { return time.Now().UTC() },
	}
	sentinel, err := maintenancesentinel.New(maintenancesentinel.Config{
		OrgID: "default",
	}, providers)
	if err != nil {
		return nil
	}
	return sentinel
}

func (r *Router) maintenanceVerificationActiveAlerts(orgID, canonicalID string) []maintenancesentinel.AlertSummary {
	mgr := r.resolveAlertManagerForOrg(orgID)
	if mgr == nil {
		return nil
	}
	canonicalID = unified.CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return nil
	}
	out := []maintenancesentinel.AlertSummary{}
	for _, a := range mgr.GetActiveAlerts() {
		if !alertMatchesResource(a, canonicalID) {
			continue
		}
		out = append(out, maintenancesentinel.AlertSummary{
			ID:           a.ID,
			Severity:     mapAlertLevel(a.Level),
			Type:         a.Type,
			Acknowledged: a.Acknowledged,
		})
	}
	return out
}

func (r *Router) maintenanceVerificationActiveFindings(orgID, canonicalID string) []maintenancesentinel.FindingSummary {
	patrol := r.resolvePatrolServiceForOrg(orgID)
	if patrol == nil {
		return nil
	}
	canonicalID = unified.CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return nil
	}
	findings := patrol.GetFindingsForResource(canonicalID)
	out := make([]maintenancesentinel.FindingSummary, 0, len(findings))
	for _, f := range findings {
		if f == nil {
			continue
		}
		out = append(out, maintenancesentinel.FindingSummary{
			ID:           f.ID,
			Severity:     mapFindingSeverity(f.Severity),
			Category:     string(f.Category),
			Resolved:     f.ResolvedAt != nil,
			Acknowledged: f.AcknowledgedAt != nil,
		})
	}
	return out
}

func (r *Router) maintenanceVerificationRecentActions(orgID, canonicalID string, since time.Time) []maintenancesentinel.ActionSummary {
	if r.resourceHandlers == nil {
		return nil
	}
	store, err := r.resourceHandlers.getStore(orgID)
	if err != nil || store == nil {
		return nil
	}
	audits, err := store.GetActionAudits(canonicalID, since, 50)
	if err != nil {
		return nil
	}
	out := make([]maintenancesentinel.ActionSummary, 0, len(audits))
	for _, a := range audits {
		out = append(out, maintenancesentinel.ActionSummary{
			ID:        a.ID,
			State:     string(a.State),
			UpdatedAt: a.UpdatedAt,
		})
	}
	return out
}

func (r *Router) maintenanceVerificationMetricSamples(orgID, canonicalID string, windowEnd, now time.Time) ([]maintenancesentinel.MetricSample, bool) {
	history := r.resolveMetricsHistoryForOrg(orgID)
	if history == nil {
		return nil, false
	}
	canonicalID = unified.CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return nil, false
	}
	// The metrics history is keyed by source IDs (e.g. "qemu/101",
	// "node/pve") not canonical resource IDs. Strip the canonical
	// `kind:` prefix so the lookup works for the common case (vm,
	// container, node). Resources whose source-id form does not
	// match this convention (storage, agents, docker hosts) won't
	// have metric history available — the sentinel reports
	// MetricSourceAvailable=false in that case, which is honest.
	sourceID := canonicalToSourceID(canonicalID)
	if sourceID == "" {
		return nil, false
	}
	since := now.Sub(windowEnd)
	if since <= 0 {
		since = time.Hour
	}
	// Inspect cpu + memory only for the MVP. These are the metrics
	// every guest/node reports; storage / disk / network are not
	// universally available across resource kinds.
	samples := []maintenancesentinel.MetricSample{}
	anyData := false
	for _, metric := range []string{"cpu", "memory"} {
		// Try both guest and node lookups; whichever has data wins.
		points := history.GetGuestMetrics(sourceID, metric, since)
		if len(points) == 0 {
			points = history.GetNodeMetrics(sourceID, metric, since)
		}
		if len(points) > 0 {
			anyData = true
			for _, p := range points {
				if !p.Timestamp.After(windowEnd) {
					continue
				}
				samples = append(samples, maintenancesentinel.MetricSample{
					Metric:    metric,
					Value:     p.Value,
					Timestamp: p.Timestamp,
				})
			}
		}
	}
	// available=true even when samples is empty if we know there's
	// a history bucket for the resource — that's still useful
	// evidence ("the resource exists in history but has not reported
	// since the window closed").
	return samples, anyData
}

// resolveAlertManagerForOrg returns the AlertManager for the supplied
// org. MVP runs the default org only; multi-tenant resolution is
// supported as a future change.
func (r *Router) resolveAlertManagerForOrg(orgID string) *alerts.Manager {
	monitor := r.resolveMonitorForOrg(orgID)
	if monitor == nil {
		return nil
	}
	return monitor.GetAlertManager()
}

func (r *Router) resolvePatrolServiceForOrg(orgID string) *ai.PatrolService {
	if r.aiSettingsHandler == nil {
		return nil
	}
	aiService := r.aiSettingsHandler.GetAIService(maintenanceVerificationOrgContext(orgID))
	if aiService == nil {
		return nil
	}
	return aiService.GetPatrolService()
}

func (r *Router) resolveMetricsHistoryForOrg(orgID string) *monitoring.MetricsHistory {
	monitor := r.resolveMonitorForOrg(orgID)
	if monitor == nil {
		return nil
	}
	return monitor.GetMetricsHistory()
}

func (r *Router) resolveMonitorForOrg(orgID string) *monitoring.Monitor {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" || orgID == "default" {
		return r.monitor
	}
	if r.mtMonitor == nil {
		return nil
	}
	monitor, err := r.mtMonitor.GetMonitor(orgID)
	if err != nil {
		return nil
	}
	return monitor
}

func maintenanceVerificationOrgContext(orgID string) context.Context {
	if orgID == "" {
		orgID = "default"
	}
	return context.WithValue(context.Background(), OrgIDContextKey, orgID)
}

// alertMatchesResource reports whether the alert targets the supplied
// canonical resource id. Alert IDs are not canonical — the alert
// model carries `ResourceID` (legacy form) and `CanonicalSpecID` /
// `CanonicalState` from the canonical engine. We try the canonical
// form first, then fall back to canonicalizing the legacy form so we
// catch both providers.
func alertMatchesResource(a alerts.Alert, canonicalID string) bool {
	if unified.CanonicalResourceID(a.CanonicalState) == canonicalID {
		return true
	}
	if unified.CanonicalResourceID(a.CanonicalSpecID) == canonicalID {
		return true
	}
	return unified.CanonicalResourceID(a.ResourceID) == canonicalID
}

func mapAlertLevel(level alerts.AlertLevel) maintenancesentinel.Severity {
	switch level {
	case alerts.AlertLevelCritical:
		return maintenancesentinel.SeverityCritical
	case alerts.AlertLevelWarning:
		return maintenancesentinel.SeverityWarning
	default:
		return ""
	}
}

func mapFindingSeverity(s ai.FindingSeverity) maintenancesentinel.Severity {
	switch s {
	case ai.FindingSeverityCritical:
		return maintenancesentinel.SeverityCritical
	case ai.FindingSeverityWarning:
		return maintenancesentinel.SeverityWarning
	default:
		return ""
	}
}

// canonicalToSourceID best-effort maps a canonical resource ID into
// the legacy source-id form used by `monitoring.MetricsHistory`.
// Canonical ids look like `vm:101`, `ct:200`, `node:pve`,
// `docker-container:abc`, etc. The metrics history keys are
// `qemu/101`, `lxc/200`, `node/pve`. Resources not covered here will
// not surface metric history evidence on their report — the sentinel
// honestly reports MetricSourceAvailable=false.
func canonicalToSourceID(canonicalID string) string {
	parts := strings.SplitN(canonicalID, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	kind, id := parts[0], parts[1]
	switch kind {
	case "vm":
		return "qemu/" + id
	case "ct":
		return "lxc/" + id
	case "node":
		return "node/" + id
	}
	return ""
}
