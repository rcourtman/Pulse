// patrol_intelligence.go provides pre-patrol guest intelligence gathering:
// service identity from the discovery store and network reachability via host agents.
package ai

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	GuestType   string // "vm" or "system-container"
	ServiceName string // e.g. "PostgreSQL 15", "Nginx" (from discovery)
	ServiceType string // e.g. "postgres", "nginx" (from discovery)
	Reachable   *bool  // nil = not checked (no agent/no IP), true/false = checked
}

// discoveryKey indexes service discovery records by (resourceType, targetID, resourceID).
type discoveryKey struct {
	resourceType servicediscovery.ResourceType
	targetID     string
	resourceID   string
}

// guestIPInfo pairs a guest ID with one of its IP addresses for reachability probing.
type guestIPInfo struct {
	guestID string
	ip      string
}

// gatherGuestIntelligence builds a map of guest intelligence keyed by guest ID
// (e.g. "qemu/100" or "lxc/101"). It loads discovery data for service identity
// and probes reachability via host agents. The entire operation is best-effort
// and capped at 5 seconds — errors are logged but never block patrol.
//
// Requires ReadState to be wired — returns an empty map if it is not.
func (p *PatrolService) gatherGuestIntelligence(ctx context.Context) map[string]*GuestIntelligence {
	intel := make(map[string]*GuestIntelligence)

	// ReadState is required — return empty if not wired.
	p.mu.RLock()
	rs := p.readState
	p.mu.RUnlock()

	if rs == nil {
		log.Warn().Msg("AI Patrol: ReadState not wired, skipping guest intelligence gathering")
		return intel
	}

	// Phase 1: Load discovery data for service identity
	p.mu.RLock()
	ds := p.discoveryStore
	p.mu.RUnlock()

	discoveryIndex := make(map[discoveryKey]*servicediscovery.ResourceDiscovery)

	if ds != nil {
		discoveries, err := ds.List()
		if err != nil {
			log.Warn().Err(err).Msg("AI Patrol: Failed to list discoveries for guest intelligence")
		} else {
			for _, d := range discoveries {
				targetID := d.TargetID
				if targetID == "" {
					targetID = d.HostID
				}
				key := discoveryKey{
					resourceType: d.ResourceType,
					targetID:     targetID,
					resourceID:   d.ResourceID,
				}
				discoveryIndex[key] = d
			}
		}
	}

	// Phase 2: Build GuestIntelligence entry for every guest
	// Also collect running guests with IPs grouped by node for reachability probing.
	nodeGuests := make(map[string][]guestIPInfo) // node name -> guests with IPs

	gatherGuestsFromReadState(rs, discoveryIndex, intel, nodeGuests)

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
						// A guest with multiple IPs is reachable if ANY IP responds.
						// Never overwrite reachable=true with false from a different IP.
						if gi.Reachable != nil && *gi.Reachable {
							continue
						}
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

// gatherGuestsFromReadState iterates VMs and Containers via ReadState typed views.
// Keys are SourceID() (the legacy guest ID like "qemu/100") for compatibility with
// downstream consumers (scope matching, triage, finding dedup). Status checks use
// StatusOnline because the registry normalizes "running" → "online".
func gatherGuestsFromReadState(
	rs unifiedresources.ReadState,
	discoveryIndex map[discoveryKey]*servicediscovery.ResourceDiscovery,
	intel map[string]*GuestIntelligence,
	nodeGuests map[string][]guestIPInfo,
) {
	for _, vm := range rs.VMs() {
		if vm.Template() {
			continue
		}
		gi := &GuestIntelligence{
			Name:      vm.Name(),
			GuestType: "vm",
		}

		vmidStr := strconv.Itoa(vm.VMID())
		if d, ok := discoveryIndex[discoveryKey{
			resourceType: servicediscovery.ResourceTypeVM,
			targetID:     vm.Node(),
			resourceID:   vmidStr,
		}]; ok {
			gi.ServiceName = d.ServiceName
			gi.ServiceType = d.ServiceType
		}
		if gi.ServiceName == "" && vm.Instance() != "" && vm.Instance() != vm.Node() {
			if d, ok := discoveryIndex[discoveryKey{
				resourceType: servicediscovery.ResourceTypeVM,
				targetID:     vm.Instance(),
				resourceID:   vmidStr,
			}]; ok {
				gi.ServiceName = d.ServiceName
				gi.ServiceType = d.ServiceType
			}
		}

		// Use SourceID (legacy guest ID like "qemu/100") as the map key
		// to stay compatible with downstream consumers. Fall back to
		// unified ID() if SourceID is empty (defensive).
		sourceID := vm.SourceID()
		if sourceID == "" {
			sourceID = vm.ID()
		}
		intel[sourceID] = gi

		if vm.Status() == unifiedresources.StatusOnline && len(vm.IPAddresses()) > 0 {
			for _, ip := range vm.IPAddresses() {
				nodeGuests[vm.Node()] = append(nodeGuests[vm.Node()], guestIPInfo{
					guestID: sourceID,
					ip:      ip,
				})
			}
		}
	}

	for _, ct := range rs.Containers() {
		if ct.Template() {
			continue
		}
		gi := &GuestIntelligence{
			Name:      ct.Name(),
			GuestType: "system-container",
		}

		vmidStr := strconv.Itoa(ct.VMID())
		if d, ok := discoveryIndex[discoveryKey{
			resourceType: servicediscovery.ResourceTypeSystemContainer,
			targetID:     ct.Node(),
			resourceID:   vmidStr,
		}]; ok {
			gi.ServiceName = d.ServiceName
			gi.ServiceType = d.ServiceType
		}
		if gi.ServiceName == "" && ct.Instance() != "" && ct.Instance() != ct.Node() {
			if d, ok := discoveryIndex[discoveryKey{
				resourceType: servicediscovery.ResourceTypeSystemContainer,
				targetID:     ct.Instance(),
				resourceID:   vmidStr,
			}]; ok {
				gi.ServiceName = d.ServiceName
				gi.ServiceType = d.ServiceType
			}
		}

		sourceID := ct.SourceID()
		if sourceID == "" {
			sourceID = ct.ID()
		}
		intel[sourceID] = gi

		if ct.Status() == unifiedresources.StatusOnline && len(ct.IPAddresses()) > 0 {
			for _, ip := range ct.IPAddresses() {
				nodeGuests[ct.Node()] = append(nodeGuests[ct.Node()], guestIPInfo{
					guestID: sourceID,
					ip:      ip,
				})
			}
		}
	}
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
