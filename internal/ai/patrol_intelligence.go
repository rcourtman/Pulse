// patrol_intelligence.go provides pre-patrol guest intelligence gathering:
// service identity from the discovery store and network reachability via host agents.
package ai

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rs/zerolog/log"
)

// GuestProber checks reachability of guest IPs via connected host agents.
type GuestProber interface {
	// GetAgentForHost returns the agent ID for a given hostname, if connected.
	GetAgentForHost(hostname string) (agentID string, ok bool)
	// PingGuests pings a list of IPs from a specific agent and returns results.
	PingGuests(ctx context.Context, agentID string, ips []string) (map[string]PingResult, error)
}

// PingResult holds the result of a reachability check for a single IP.
type PingResult struct {
	Reachable bool
}

// GuestIntelligence enriches a single guest with discovery and reachability data.
type GuestIntelligence struct {
	Name        string // Human-readable guest name (e.g. "db-server")
	GuestType   string // "vm" or "lxc"
	ServiceName string // e.g. "PostgreSQL 15", "Nginx" (from discovery)
	ServiceType string // e.g. "postgres", "nginx" (from discovery)
	Reachable   *bool  // nil = not checked (no agent/no IP), true/false = checked
}

// gatherGuestIntelligence builds a map of guest intelligence keyed by guest ID
// (e.g. "qemu/100" or "lxc/101"). It loads discovery data for service identity
// and probes reachability via host agents. The entire operation is best-effort
// and capped at 5 seconds — errors are logged but never block patrol.
func (p *PatrolService) gatherGuestIntelligence(ctx context.Context, state models.StateSnapshot) map[string]*GuestIntelligence {
	intel := make(map[string]*GuestIntelligence)

	// Phase 1: Load discovery data for service identity
	p.mu.RLock()
	ds := p.discoveryStore
	p.mu.RUnlock()

	// Build discovery index: (resourceType, hostID, resourceID) -> discovery
	type discoveryKey struct {
		resourceType servicediscovery.ResourceType
		hostID       string
		resourceID   string
	}
	discoveryIndex := make(map[discoveryKey]*servicediscovery.ResourceDiscovery)

	if ds != nil {
		discoveries, err := ds.List()
		if err != nil {
			log.Warn().Err(err).Msg("AI Patrol: Failed to list discoveries for guest intelligence")
		} else {
			for _, d := range discoveries {
				key := discoveryKey{
					resourceType: d.ResourceType,
					hostID:       d.HostID,
					resourceID:   d.ResourceID,
				}
				discoveryIndex[key] = d
			}
		}
	}

	// Phase 2: Build GuestIntelligence entry for every guest
	// Also collect running guests with IPs grouped by node for reachability probing.
	type guestIPInfo struct {
		guestID string
		ip      string
	}
	nodeGuests := make(map[string][]guestIPInfo) // node name -> guests with IPs

	for _, vm := range state.VMs {
		if vm.Template {
			continue
		}
		gi := &GuestIntelligence{
			Name:      vm.Name,
			GuestType: "vm",
		}

		// Look up discovery
		vmidStr := strconv.Itoa(vm.VMID)
		if d, ok := discoveryIndex[discoveryKey{servicediscovery.ResourceTypeVM, vm.Node, vmidStr}]; ok {
			gi.ServiceName = d.ServiceName
			gi.ServiceType = d.ServiceType
		}
		// Also check Instance as hostID (some deployments use instance name)
		if gi.ServiceName == "" && vm.Instance != "" && vm.Instance != vm.Node {
			if d, ok := discoveryIndex[discoveryKey{servicediscovery.ResourceTypeVM, vm.Instance, vmidStr}]; ok {
				gi.ServiceName = d.ServiceName
				gi.ServiceType = d.ServiceType
			}
		}

		intel[vm.ID] = gi

		// Collect for reachability (only running guests with known IPs)
		if vm.Status == "running" && len(vm.IPAddresses) > 0 {
			nodeGuests[vm.Node] = append(nodeGuests[vm.Node], guestIPInfo{
				guestID: vm.ID,
				ip:      vm.IPAddresses[0],
			})
		}
	}

	for _, ct := range state.Containers {
		if ct.Template {
			continue
		}
		gi := &GuestIntelligence{
			Name:      ct.Name,
			GuestType: "lxc",
		}

		// Look up discovery
		vmidStr := strconv.Itoa(ct.VMID)
		if d, ok := discoveryIndex[discoveryKey{servicediscovery.ResourceTypeLXC, ct.Node, vmidStr}]; ok {
			gi.ServiceName = d.ServiceName
			gi.ServiceType = d.ServiceType
		}
		if gi.ServiceName == "" && ct.Instance != "" && ct.Instance != ct.Node {
			if d, ok := discoveryIndex[discoveryKey{servicediscovery.ResourceTypeLXC, ct.Instance, vmidStr}]; ok {
				gi.ServiceName = d.ServiceName
				gi.ServiceType = d.ServiceType
			}
		}

		intel[ct.ID] = gi

		if ct.Status == "running" && len(ct.IPAddresses) > 0 {
			nodeGuests[ct.Node] = append(nodeGuests[ct.Node], guestIPInfo{
				guestID: ct.ID,
				ip:      ct.IPAddresses[0],
			})
		}
	}

	// Phase 3: Reachability probing via host agents
	p.mu.RLock()
	prober := p.guestProber
	p.mu.RUnlock()

	if prober == nil || len(nodeGuests) == 0 {
		return intel
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // protects intel map writes

	for node, guests := range nodeGuests {
		agentID, ok := prober.GetAgentForHost(node)
		if !ok {
			continue
		}

		// Collect unique IPs for this node
		ips := make([]string, 0, len(guests))
		ipSeen := make(map[string]bool)
		for _, g := range guests {
			if !ipSeen[g.ip] {
				ips = append(ips, g.ip)
				ipSeen[g.ip] = true
			}
		}

		wg.Add(1)
		go func(node, agentID string, ips []string, guests []guestIPInfo) {
			defer wg.Done()

			results, err := prober.PingGuests(ctx, agentID, ips)
			if err != nil {
				log.Warn().Err(err).Str("node", node).Msg("AI Patrol: Guest ping probe failed")
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, g := range guests {
				if result, ok := results[g.ip]; ok {
					gi := intel[g.guestID]
					if gi != nil {
						reachable := result.Reachable
						gi.Reachable = &reachable
					}
				}
			}
		}(node, agentID, ips, guests)
	}

	// Wait for all probes to complete (bounded by caller's context timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All probes completed
	case <-ctx.Done():
		log.Warn().Msg("AI Patrol: Guest reachability probing timed out")
	}

	probed := 0
	for _, gi := range intel {
		if gi.Reachable != nil {
			probed++
		}
	}
	if probed > 0 {
		log.Debug().
			Int("probed", probed).
			Int("total_guests", len(intel)).
			Msg("AI Patrol: Guest reachability probing complete")
	}

	return intel
}

// formatReachable returns the display string for the Reachable column.
func formatReachable(r *bool) string {
	if r == nil {
		return "-"
	}
	if *r {
		return "yes"
	}
	return "NO"
}

// formatService returns the display string for the Service column.
func formatService(gi *GuestIntelligence) string {
	if gi == nil || gi.ServiceName == "" {
		return "-"
	}
	name := gi.ServiceName
	if len(name) > 25 {
		name = name[:22] + "..."
	}
	return name
}

// reachableFromIntel extracts the Reachable pointer from a GuestIntelligence entry.
// Returns nil if the entry is nil (no intelligence available).
func reachableFromIntel(gi *GuestIntelligence) *bool {
	if gi == nil {
		return nil
	}
	return gi.Reachable
}

// serviceHealthIssue represents a running guest that is unreachable.
type serviceHealthIssue struct {
	name    string
	service string
	node    string
}

// buildServiceHealthIssues returns a markdown section listing the given issues.
// Returns empty string if no issues.
func buildServiceHealthIssues(issues []serviceHealthIssue) string {
	if len(issues) == 0 {
		return ""
	}

	result := "# Service Health Issues\n"
	for _, iss := range issues {
		result += fmt.Sprintf("- %s (%s on %s): Running but UNREACHABLE\n", iss.name, iss.service, iss.node)
	}
	result += "\n"
	return result
}
