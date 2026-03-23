package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// MonitoredSystemLedgerEntry represents a single counted top-level monitored
// system.
type MonitoredSystemLedgerEntry struct {
	Name        string                           `json:"name"`
	Type        string                           `json:"type"`
	Status      string                           `json:"status"`    // "online", "offline", "unknown"
	LastSeen    string                           `json:"last_seen"` // RFC3339 or empty
	Source      string                           `json:"source"`
	Explanation MonitoredSystemLedgerExplanation `json:"explanation"`
}

type MonitoredSystemLedgerExplanation struct {
	Summary  string                                    `json:"summary"`
	Reasons  []MonitoredSystemLedgerExplanationReason  `json:"reasons"`
	Surfaces []MonitoredSystemLedgerExplanationSurface `json:"surfaces"`
}

type MonitoredSystemLedgerExplanationReason struct {
	Kind    string `json:"kind"`
	Signal  string `json:"signal"`
	Summary string `json:"summary"`
}

type MonitoredSystemLedgerExplanationSurface struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// MonitoredSystemLedgerResponse is the response for GET /api/license/monitored-system-ledger.
type MonitoredSystemLedgerResponse struct {
	Systems []MonitoredSystemLedgerEntry `json:"systems"`
	Total   int                          `json:"total"`
	Limit   int                          `json:"limit"` // 0 = unlimited
}

func EmptyMonitoredSystemLedgerResponse() MonitoredSystemLedgerResponse {
	return MonitoredSystemLedgerResponse{}.NormalizeCollections()
}

func (r MonitoredSystemLedgerResponse) NormalizeCollections() MonitoredSystemLedgerResponse {
	if r.Systems == nil {
		r.Systems = []MonitoredSystemLedgerEntry{}
	}
	for i := range r.Systems {
		r.Systems[i] = r.Systems[i].NormalizeCollections()
	}
	return r
}

func (e MonitoredSystemLedgerEntry) NormalizeCollections() MonitoredSystemLedgerEntry {
	if e.Explanation.Reasons == nil {
		e.Explanation.Reasons = []MonitoredSystemLedgerExplanationReason{}
	}
	if e.Explanation.Surfaces == nil {
		e.Explanation.Surfaces = []MonitoredSystemLedgerExplanationSurface{}
	}
	return e
}

func (r *Router) handleMonitoredSystemLedger(w http.ResponseWriter, req *http.Request) {
	orgID := GetOrgID(req.Context())

	// Get canonical monitored systems from the unified ReadState surface.
	var systems []unifiedresources.MonitoredSystemRecord
	var monitorResolved bool
	if r.mtMonitor != nil {
		monitor, monErr := r.mtMonitor.GetMonitor(orgID)
		if monErr != nil {
			log.Warn().Err(monErr).Str("org", orgID).Msg("monitored-system-ledger: failed to resolve tenant monitor")
		}
		if monitor != nil {
			if rs := monitor.GetUnifiedReadState(); rs != nil {
				systems = unifiedresources.MonitoredSystems(rs)
			}
			monitorResolved = true
		}
	}
	// Fallback to the default monitor only for the default org to avoid cross-tenant data leaks.
	if !monitorResolved && orgID == "default" && r.monitor != nil {
		if rs := r.monitor.GetUnifiedReadState(); rs != nil {
			systems = unifiedresources.MonitoredSystems(rs)
		}
	}

	entries := make([]MonitoredSystemLedgerEntry, 0, len(systems))
	for _, system := range systems {
		entries = append(entries, MonitoredSystemLedgerEntry{
			Name:        system.Name,
			Type:        system.Type,
			Status:      normalizeStatus(string(system.Status)),
			LastSeen:    formatLastSeen(system.LastSeen),
			Source:      system.Source,
			Explanation: monitoredSystemLedgerExplanation(system.Explanation),
		})
	}

	limit := maxMonitoredSystemsLimitForContext(req.Context())

	resp := EmptyMonitoredSystemLedgerResponse()
	resp.Systems = entries
	resp.Total = len(entries)
	resp.Limit = limit

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.NormalizeCollections())
}

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

func normalizeStatus(s string) string {
	switch s {
	case "online", "warning", "offline", "unknown":
		return s
	default:
		return "unknown"
	}
}

func formatLastSeen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func monitoredSystemLedgerExplanation(
	explanation unifiedresources.MonitoredSystemGroupingExplanation,
) MonitoredSystemLedgerExplanation {
	reasons := make([]MonitoredSystemLedgerExplanationReason, 0, len(explanation.Reasons))
	for _, reason := range explanation.Reasons {
		reasons = append(reasons, MonitoredSystemLedgerExplanationReason{
			Kind:    reason.Kind,
			Signal:  reason.Signal,
			Summary: reason.Summary,
		})
	}

	surfaces := make([]MonitoredSystemLedgerExplanationSurface, 0, len(explanation.Surfaces))
	for _, surface := range explanation.Surfaces {
		surfaces = append(surfaces, MonitoredSystemLedgerExplanationSurface{
			Name:   surface.Name,
			Type:   surface.Type,
			Source: surface.Source,
		})
	}

	return MonitoredSystemLedgerExplanation{
		Summary:  explanation.Summary,
		Reasons:  reasons,
		Surfaces: surfaces,
	}
}
