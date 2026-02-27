package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// HostLedgerEntry represents a single installed Pulse Unified Agent
// that counts against the agent limit.
type HostLedgerEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // always "agent"
	Status   string `json:"status"`    // "online", "offline", "unknown"
	LastSeen string `json:"last_seen"` // RFC3339 or empty
	Source   string `json:"source"`    // always "agent"
}

// HostLedgerResponse is the response for GET /api/license/host-ledger.
type HostLedgerResponse struct {
	Hosts []HostLedgerEntry `json:"hosts"`
	Total int               `json:"total"`
	Limit int               `json:"limit"` // 0 = unlimited
}

func (r *Router) handleHostLedger(w http.ResponseWriter, req *http.Request) {
	orgID := GetOrgID(req.Context())

	// Get host agents from the unified ReadState surface.
	var hosts []*unifiedresources.HostView
	var monitorResolved bool
	if r.mtMonitor != nil {
		monitor, monErr := r.mtMonitor.GetMonitor(orgID)
		if monErr != nil {
			log.Warn().Err(monErr).Str("org", orgID).Msg("host-ledger: failed to resolve tenant monitor")
		}
		if monitor != nil {
			if rs := monitor.GetUnifiedReadState(); rs != nil {
				hosts = rs.Hosts()
			}
			monitorResolved = true
		}
	}
	// Fallback to the default monitor only for the default org to avoid cross-tenant data leaks.
	if !monitorResolved && orgID == "default" && r.monitor != nil {
		if rs := r.monitor.GetUnifiedReadState(); rs != nil {
			hosts = rs.Hosts()
		}
	}

	// Build ledger entries from installed agents only.
	entries := make([]HostLedgerEntry, 0, len(hosts))
	for _, h := range hosts {
		entries = append(entries, HostLedgerEntry{
			Name:     hostDisplayName(h.Name(), h.Hostname(), h.ID()),
			Type:     "agent",
			Status:   normalizeStatus(string(h.Status())),
			LastSeen: formatLastSeen(h.LastSeen()),
			Source:   "agent",
		})
	}

	limit := maxAgentsLimitForContext(req.Context())

	resp := HostLedgerResponse{
		Hosts: entries,
		Total: len(entries),
		Limit: limit,
	}
	if resp.Hosts == nil {
		resp.Hosts = []HostLedgerEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// Display-name helper
// ---------------------------------------------------------------------------

func hostDisplayName(display, hostname, id string) string {
	if display != "" {
		return display
	}
	if hostname != "" {
		return hostname
	}
	return id
}

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

func normalizeStatus(s string) string {
	switch s {
	case "online", "offline":
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
