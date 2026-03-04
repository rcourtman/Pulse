package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AgentLedgerEntry represents a single installed Pulse Unified Agent
// that counts against the agent limit.
type AgentLedgerEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // always "agent"
	Status   string `json:"status"`    // "online", "offline", "unknown"
	LastSeen string `json:"last_seen"` // RFC3339 or empty
	Source   string `json:"source"`    // always "agent"
}

// AgentLedgerResponse is the response for GET /api/license/agent-ledger.
type AgentLedgerResponse struct {
	Agents []AgentLedgerEntry `json:"agents"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"` // 0 = unlimited
}

func (r *Router) handleAgentLedger(w http.ResponseWriter, req *http.Request) {
	orgID := GetOrgID(req.Context())

	// Get host agents from the unified ReadState surface.
	var hostAgents []*unifiedresources.HostView
	var monitorResolved bool
	if r.mtMonitor != nil {
		monitor, monErr := r.mtMonitor.GetMonitor(orgID)
		if monErr != nil {
			log.Warn().Err(monErr).Str("org", orgID).Msg("agent-ledger: failed to resolve tenant monitor")
		}
		if monitor != nil {
			if rs := monitor.GetUnifiedReadState(); rs != nil {
				hostAgents = rs.Hosts()
			}
			monitorResolved = true
		}
	}
	// Fallback to the default monitor only for the default org to avoid cross-tenant data leaks.
	if !monitorResolved && orgID == "default" && r.monitor != nil {
		if rs := r.monitor.GetUnifiedReadState(); rs != nil {
			hostAgents = rs.Hosts()
		}
	}

	// Build ledger entries from installed agents only.
	entries := make([]AgentLedgerEntry, 0, len(hostAgents))
	for _, h := range hostAgents {
		entries = append(entries, AgentLedgerEntry{
			Name:     agentDisplayName(h.Name(), h.Hostname(), h.ID()),
			Type:     "agent",
			Status:   normalizeStatus(string(h.Status())),
			LastSeen: formatLastSeen(h.LastSeen()),
			Source:   "agent",
		})
	}

	limit := maxAgentsLimitForContext(req.Context())

	resp := AgentLedgerResponse{
		Agents: entries,
		Total:  len(entries),
		Limit:  limit,
	}
	if resp.Agents == nil {
		resp.Agents = []AgentLedgerEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------------------------------------------------------
// Display-name helper
// ---------------------------------------------------------------------------

func agentDisplayName(display, hostname, id string) string {
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
