package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// HostLedgerEntry represents a single host that counts against the license limit.
type HostLedgerEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // "proxmox-pve", "proxmox-pbs", "proxmox-pmg", "host-agent", "docker", "kubernetes", "truenas"
	Status    string `json:"status"`     // "online", "offline", "unknown"
	LastSeen  string `json:"last_seen"`  // RFC3339 or empty
	Source    string `json:"source"`     // how discovered — e.g. "proxmox", "agent", "docker", "kubernetes", "truenas", or "proxmox + agent" when deduped
	FirstSeen string `json:"first_seen"` // RFC3339 or empty
}

// HostLedgerResponse is the response for GET /api/license/host-ledger.
type HostLedgerResponse struct {
	Hosts []HostLedgerEntry `json:"hosts"`
	Total int               `json:"total"`
	Limit int               `json:"limit"` // 0 = unlimited
}

func (r *Router) handleHostLedger(w http.ResponseWriter, req *http.Request) {
	orgID := GetOrgID(req.Context())

	persistence, err := r.multiTenant.GetPersistence(orgID)
	if err != nil {
		log.Error().Err(err).Str("org", orgID).Msg("host-ledger: failed to get persistence")
		writeErrorResponse(w, http.StatusInternalServerError, "config_error", "Failed to load configuration", nil)
		return
	}
	nodesConfig, err := persistence.LoadNodesConfig()
	if err != nil {
		log.Error().Err(err).Str("org", orgID).Msg("host-ledger: failed to load nodes config")
		writeErrorResponse(w, http.StatusInternalServerError, "config_error", "Failed to load node configuration", nil)
		return
	}

	// Get runtime state for status enrichment and agent-based host enumeration.
	var state models.StateSnapshot
	var monitorResolved bool
	if r.mtMonitor != nil {
		monitor, monErr := r.mtMonitor.GetMonitor(orgID)
		if monErr != nil {
			log.Warn().Err(monErr).Str("org", orgID).Msg("host-ledger: failed to resolve tenant monitor")
		}
		if monitor != nil {
			state = monitor.GetState()
			monitorResolved = true
		}
	}
	// Fallback to the default monitor only for the default org to avoid cross-tenant data leaks.
	if !monitorResolved && orgID == "default" && r.monitor != nil {
		state = r.monitor.GetState()
	}

	// Build config entries for the dedup layer.
	var configPVE, configPBS, configPMG, configTrueNAS []unifiedresources.ConfigEntry
	if nodesConfig != nil {
		for _, pve := range nodesConfig.PVEInstances {
			configPVE = append(configPVE, unifiedresources.ConfigEntry{
				ID: pve.Host, Name: pve.Name, Host: pve.Host,
			})
		}
		for _, pbs := range nodesConfig.PBSInstances {
			configPBS = append(configPBS, unifiedresources.ConfigEntry{
				ID: pbs.Host, Name: pbs.Name, Host: pbs.Host,
			})
		}
		for _, pmg := range nodesConfig.PMGInstances {
			configPMG = append(configPMG, unifiedresources.ConfigEntry{
				ID: pmg.Host, Name: pmg.Name, Host: pmg.Host,
			})
		}
	}

	trueNASInstances, trueNASErr := persistence.LoadTrueNASConfig()
	if trueNASErr != nil {
		log.Warn().Err(trueNASErr).Str("org", orgID).Msg("host-ledger: failed to load TrueNAS config")
	}
	for _, nas := range trueNASInstances {
		configTrueNAS = append(configTrueNAS, unifiedresources.ConfigEntry{
			ID: nas.ID, Name: nas.Name, Host: nas.Host,
		})
	}

	// Resolve deduplicated hosts.
	candidates := unifiedresources.CollectHostCandidates(state, configPVE, configPBS, configPMG, configTrueNAS)
	resolved := unifiedresources.ResolveHosts(candidates)

	// Convert resolved hosts to ledger entries.
	entries := make([]HostLedgerEntry, 0, len(resolved.Hosts))
	for _, h := range resolved.Hosts {
		entries = append(entries, HostLedgerEntry{
			Name:      h.Name,
			Type:      h.PrimaryType,
			Status:    h.Status,
			LastSeen:  h.LastSeen,
			Source:    strings.Join(h.SourceLabels, " + "),
			FirstSeen: h.FirstSeen,
		})
	}

	// Determine node limit.
	limit := maxNodesLimitForContext(req.Context())

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
// Display-name helpers (still used by other files in the api package)
// ---------------------------------------------------------------------------

func pveNodeDisplayName(display, name, id string) string {
	if display != "" {
		return display
	}
	if name != "" {
		return name
	}
	return id
}

func pbsPmgDisplayName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}

func hostDisplayName(display, hostname, id string) string {
	if display != "" {
		return display
	}
	if hostname != "" {
		return hostname
	}
	return id
}

func dockerDisplayName(display, custom, hostname, id string) string {
	if custom != "" {
		return custom
	}
	if display != "" {
		return display
	}
	if hostname != "" {
		return hostname
	}
	return id
}

func trueNASDisplayName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}

func k8sDisplayName(display, custom, name, id string) string {
	if custom != "" {
		return custom
	}
	if display != "" {
		return display
	}
	if name != "" {
		return name
	}
	return id
}

// ---------------------------------------------------------------------------
// Status enrichment helpers (still used by other files)
// ---------------------------------------------------------------------------

func pveStatusFromState(host string, state models.StateSnapshot) string {
	for _, n := range state.Nodes {
		if n.Host == host {
			return normalizeStatus(n.Status)
		}
	}
	return "unknown"
}

func enrichPBSStatus(entry *HostLedgerEntry, host string, state models.StateSnapshot) {
	for _, p := range state.PBSInstances {
		if p.Host == host {
			entry.Status = normalizeStatus(p.Status)
			entry.LastSeen = formatLastSeen(p.LastSeen)
			return
		}
	}
	entry.Status = "unknown"
}

func enrichPMGStatus(entry *HostLedgerEntry, host string, state models.StateSnapshot) {
	for _, p := range state.PMGInstances {
		if p.Host == host {
			entry.Status = normalizeStatus(p.Status)
			entry.LastSeen = formatLastSeen(p.LastSeen)
			return
		}
	}
	entry.Status = "unknown"
}

func k8sNodeStatus(ready bool) string {
	if ready {
		return "online"
	}
	return "offline"
}

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
