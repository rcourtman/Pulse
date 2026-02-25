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
	Name      string `json:"name"`
	Type      string `json:"type"`       // "proxmox-pve", "proxmox-pbs", "proxmox-pmg", "host-agent", "docker", "kubernetes", "truenas"
	Status    string `json:"status"`     // "online", "offline", "unknown"
	LastSeen  string `json:"last_seen"`  // RFC3339 or empty
	Source    string `json:"source"`     // how discovered — e.g. "proxmox", "agent", "docker", "kubernetes", "truenas"
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

	// PVE: list individual nodes discovered at runtime (one PVE connection may
	// represent a multi-node cluster). Each node counts as one host slot.
	// If the monitor has no state yet, fall back to listing configured connections.
	if len(state.Nodes) > 0 {
		for _, n := range state.Nodes {
			entries = append(entries, HostLedgerEntry{
				Name:     pveNodeDisplayName(n.DisplayName, n.Name, n.ID),
				Type:     "proxmox-pve",
				Status:   normalizeStatus(n.Status),
				LastSeen: formatLastSeen(n.LastSeen),
				Source:   "proxmox",
			})
		}
	} else if nodesConfig != nil {
		for _, pve := range nodesConfig.PVEInstances {
			entries = append(entries, HostLedgerEntry{
				Name:   pbsPmgDisplayName(pve.Name, pve.Host),
				Type:   "proxmox-pve",
				Status: "unknown",
				Source: "proxmox",
			})
		}
	}

	if nodesConfig != nil {
		for _, pbs := range nodesConfig.PBSInstances {
			entry := HostLedgerEntry{
				Name:   pbsPmgDisplayName(pbs.Name, pbs.Host),
				Type:   "proxmox-pbs",
				Source: "proxmox",
			}
			enrichPBSStatus(&entry, pbs.Host, state)
			entries = append(entries, entry)
		}
		for _, pmg := range nodesConfig.PMGInstances {
			entry := HostLedgerEntry{
				Name:   pbsPmgDisplayName(pmg.Name, pmg.Host),
				Type:   "proxmox-pmg",
				Source: "proxmox",
			}
			enrichPMGStatus(&entry, pmg.Host, state)
			entries = append(entries, entry)
		}
	}

	// Collect TrueNAS instances from config.
	trueNASInstances, trueNASErr := persistence.LoadTrueNASConfig()
	if trueNASErr != nil {
		log.Warn().Err(trueNASErr).Str("org", orgID).Msg("host-ledger: failed to load TrueNAS config")
	}
	for _, nas := range trueNASInstances {
		entries = append(entries, HostLedgerEntry{
			Name:   trueNASDisplayName(nas.Name, nas.Host),
			Type:   "truenas",
			Status: "unknown",
			Source: "truenas",
		})
	}

	// Collect runtime-only hosts (agents).
	for _, h := range state.Hosts {
		entries = append(entries, HostLedgerEntry{
			Name:     hostDisplayName(h.DisplayName, h.Hostname, h.ID),
			Type:     "host-agent",
			Status:   normalizeStatus(h.Status),
			LastSeen: formatLastSeen(h.LastSeen),
			Source:   "agent",
		})
	}
	for _, d := range state.DockerHosts {
		entries = append(entries, HostLedgerEntry{
			Name:     dockerDisplayName(d.DisplayName, d.CustomDisplayName, d.Hostname, d.ID),
			Type:     "docker",
			Status:   normalizeStatus(d.Status),
			LastSeen: formatLastSeen(d.LastSeen),
			Source:   "docker",
		})
	}
	for _, k := range state.KubernetesClusters {
		clusterName := k8sDisplayName(k.DisplayName, k.CustomDisplayName, k.Name, k.ID)
		if len(k.Nodes) > 0 {
			// List individual K8s nodes — enforcement counts nodes, not clusters.
			for _, kn := range k.Nodes {
				nodeName := kn.Name
				if nodeName == "" {
					nodeName = kn.UID
				}
				entries = append(entries, HostLedgerEntry{
					Name:     clusterName + "/" + nodeName,
					Type:     "kubernetes",
					Status:   k8sNodeStatus(kn.Ready),
					LastSeen: formatLastSeen(k.LastSeen),
					Source:   "kubernetes",
				})
			}
		} else {
			// No node list yet — count cluster as 1 slot (matches enforcement fallback).
			entries = append(entries, HostLedgerEntry{
				Name:     clusterName,
				Type:     "kubernetes",
				Status:   normalizeStatus(k.Status),
				LastSeen: formatLastSeen(k.LastSeen),
				Source:   "kubernetes",
			})
		}
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
