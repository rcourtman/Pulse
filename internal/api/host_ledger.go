package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// HostLedgerEntry represents a single host that counts against the license limit.
type HostLedgerEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"`      // "proxmox-pve", "proxmox-pbs", "proxmox-pmg", "host-agent", "docker", "kubernetes"
	Status   string `json:"status"`    // "online", "offline", "unknown"
	LastSeen string `json:"last_seen"` // RFC3339 or empty
}

// HostLedgerResponse is the response for GET /api/license/host-ledger.
type HostLedgerResponse struct {
	Hosts []HostLedgerEntry `json:"hosts"`
	Total int               `json:"total"`
	Limit int               `json:"limit"` // 0 = unlimited
}

func (r *Router) handleHostLedger(w http.ResponseWriter, req *http.Request) {
	orgID := GetOrgID(req.Context())

	// Collect config-based hosts (PVE/PBS/PMG).
	var entries []HostLedgerEntry

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

	if nodesConfig != nil {
		for _, pve := range nodesConfig.PVEInstances {
			entries = append(entries, HostLedgerEntry{
				Name:   pveDisplayName(pve.Name, pve.Host),
				Type:   "proxmox-pve",
				Status: pveStatusFromState(pve.Host, state),
			})
		}
		for _, pbs := range nodesConfig.PBSInstances {
			entry := HostLedgerEntry{
				Name: pbsPmgDisplayName(pbs.Name, pbs.Host),
				Type: "proxmox-pbs",
			}
			enrichPBSStatus(&entry, pbs.Host, state)
			entries = append(entries, entry)
		}
		for _, pmg := range nodesConfig.PMGInstances {
			entry := HostLedgerEntry{
				Name: pbsPmgDisplayName(pmg.Name, pmg.Host),
				Type: "proxmox-pmg",
			}
			enrichPMGStatus(&entry, pmg.Host, state)
			entries = append(entries, entry)
		}
	}

	// Collect runtime-only hosts (agents).
	for _, h := range state.Hosts {
		entries = append(entries, HostLedgerEntry{
			Name:     hostDisplayName(h.DisplayName, h.Hostname, h.ID),
			Type:     "host-agent",
			Status:   normalizeStatus(h.Status),
			LastSeen: formatLastSeen(h.LastSeen),
		})
	}
	for _, d := range state.DockerHosts {
		entries = append(entries, HostLedgerEntry{
			Name:     dockerDisplayName(d.DisplayName, d.CustomDisplayName, d.Hostname, d.ID),
			Type:     "docker",
			Status:   normalizeStatus(d.Status),
			LastSeen: formatLastSeen(d.LastSeen),
		})
	}
	for _, k := range state.KubernetesClusters {
		entries = append(entries, HostLedgerEntry{
			Name:     k8sDisplayName(k.DisplayName, k.CustomDisplayName, k.Name, k.ID),
			Type:     "kubernetes",
			Status:   normalizeStatus(k.Status),
			LastSeen: formatLastSeen(k.LastSeen),
		})
	}

	// Sort by type then name for stable output.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return entries[i].Name < entries[j].Name
	})

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
// Display-name helpers
// ---------------------------------------------------------------------------

func pveDisplayName(name, host string) string {
	if name != "" {
		return name
	}
	return host
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
// Status enrichment helpers
// ---------------------------------------------------------------------------

// pveStatusFromState finds the status for a PVE config entry by matching its
// Host URL against runtime nodes. Returns the status of the first matching node.
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
