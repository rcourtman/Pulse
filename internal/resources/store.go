package resources

import (
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Store maintains a unified collection of all resources with deduplication.
type Store struct {
	mu        sync.RWMutex
	resources map[string]*Resource // Keyed by Resource.ID
	
	// Index by identity for deduplication
	byHostname  map[string][]string // hostname (lower) -> resource IDs
	byMachineID map[string]string   // machine-id -> resource ID
	byIP        map[string][]string // IP -> resource IDs
	
	// Track merged resources (one source is preferred over another)
	mergedFrom map[string]string // suppressed ID -> preferred ID
}

// NewStore creates a new resource store.
func NewStore() *Store {
	return &Store{
		resources:   make(map[string]*Resource),
		byHostname:  make(map[string][]string),
		byMachineID: make(map[string]string),
		byIP:        make(map[string][]string),
		mergedFrom:  make(map[string]string),
	}
}

// Upsert adds or updates a resource in the store, performing deduplication.
// Returns the ID of the resource (may differ if merged with existing).
func (s *Store) Upsert(r Resource) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check for duplicates
	if r.Identity != nil {
		if existingID := s.findDuplicate(&r); existingID != "" {
			// Found a duplicate - determine which to prefer
			existing := s.resources[existingID]
			preferred := s.preferredResource(existing, &r)
			
			if preferred == &r {
				// New resource is preferred, replace the old one
				s.removeFromIndexes(existing)
				delete(s.resources, existingID)
				s.mergedFrom[existingID] = r.ID
			} else {
				// Existing resource is preferred, mark this as merged
				s.mergedFrom[r.ID] = existingID
				return existingID
			}
		}
	}
	
	// Add/update the resource
	s.resources[r.ID] = &r
	s.addToIndexes(&r)
	
	return r.ID
}

// Get retrieves a resource by ID.
func (s *Store) Get(id string) (*Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if this ID was merged into another
	if preferredID, merged := s.mergedFrom[id]; merged {
		r, ok := s.resources[preferredID]
		return r, ok
	}
	
	r, ok := s.resources[id]
	return r, ok
}

// GetAll returns all resources (excluding suppressed duplicates).
func (s *Store) GetAll() []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]Resource, 0, len(s.resources))
	for _, r := range s.resources {
		result = append(result, *r)
	}
	return result
}

// GetByType returns all resources of a specific type.
func (s *Store) GetByType(t ResourceType) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Resource
	for _, r := range s.resources {
		if r.Type == t {
			result = append(result, *r)
		}
	}
	return result
}

// GetByPlatform returns all resources from a specific platform.
func (s *Store) GetByPlatform(p PlatformType) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Resource
	for _, r := range s.resources {
		if r.PlatformType == p {
			result = append(result, *r)
		}
	}
	return result
}

// GetInfrastructure returns all infrastructure resources (nodes, hosts).
func (s *Store) GetInfrastructure() []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Resource
	for _, r := range s.resources {
		if r.IsInfrastructure() {
			result = append(result, *r)
		}
	}
	return result
}

// GetWorkloads returns all workload resources (VMs, containers).
func (s *Store) GetWorkloads() []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Resource
	for _, r := range s.resources {
		if r.IsWorkload() {
			result = append(result, *r)
		}
	}
	return result
}

// GetChildren returns all resources with the specified parent ID.
func (s *Store) GetChildren(parentID string) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Resource
	for _, r := range s.resources {
		if r.ParentID == parentID {
			result = append(result, *r)
		}
	}
	return result
}

// FindContainerHost looks up a Docker container by name or ID and returns the
// hostname of its parent DockerHost. This is used by AI routing to automatically
// determine which host should execute commands for a container.
// Returns empty string if not found.
func (s *Store) FindContainerHost(containerNameOrID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if containerNameOrID == "" {
		return ""
	}
	
	containerNameLower := strings.ToLower(containerNameOrID)
	
	// Find the container
	var container *Resource
	for _, r := range s.resources {
		if r.Type != ResourceTypeDockerContainer {
			continue
		}
		// Match by name or ID (case-insensitive)
		if strings.EqualFold(r.Name, containerNameOrID) ||
		   strings.EqualFold(r.ID, containerNameOrID) ||
		   strings.Contains(strings.ToLower(r.Name), containerNameLower) ||
		   strings.Contains(strings.ToLower(r.ID), containerNameLower) {
			container = r
			break
		}
	}
	
	if container == nil || container.ParentID == "" {
		return ""
	}
	
	// Find the parent DockerHost
	parent := s.resources[container.ParentID]
	if parent == nil {
		return ""
	}
	
	// Return the hostname from identity, or the name
	if parent.Identity != nil && parent.Identity.Hostname != "" {
		return parent.Identity.Hostname
	}
	return parent.Name
}


// Remove removes a resource from the store.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if r, ok := s.resources[id]; ok {
		s.removeFromIndexes(r)
		delete(s.resources, id)
	}
	
	// Also clean up any merge references
	delete(s.mergedFrom, id)
	for k, v := range s.mergedFrom {
		if v == id {
			delete(s.mergedFrom, k)
		}
	}
}

// IsSuppressed returns true if the resource ID has been merged/suppressed by another.
func (s *Store) IsSuppressed(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	_, suppressed := s.mergedFrom[id]
	return suppressed
}

// GetPreferredID returns the preferred resource ID if this one is suppressed.
func (s *Store) GetPreferredID(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if preferredID, ok := s.mergedFrom[id]; ok {
		return preferredID
	}
	return id
}

// GetStats returns statistics about the store.
func (s *Store) GetStats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := StoreStats{
		TotalResources:      len(s.resources),
		SuppressedResources: len(s.mergedFrom),
		ByType:              make(map[ResourceType]int),
		ByPlatform:          make(map[PlatformType]int),
		ByStatus:            make(map[ResourceStatus]int),
		WithAlerts:          0,
		LastUpdated:         time.Now().UTC().Format(time.RFC3339),
	}
	
	for _, r := range s.resources {
		stats.ByType[r.Type]++
		stats.ByPlatform[r.PlatformType]++
		stats.ByStatus[r.Status]++
		if len(r.Alerts) > 0 {
			stats.WithAlerts++
		}
	}
	
	return stats
}

// StoreStats contains statistics about the resource store.
type StoreStats struct {
	TotalResources      int                      `json:"totalResources"`
	SuppressedResources int                      `json:"suppressedResources"`
	ByType              map[ResourceType]int     `json:"byType"`
	ByPlatform          map[PlatformType]int     `json:"byPlatform"`
	ByStatus            map[ResourceStatus]int   `json:"byStatus"`
	WithAlerts          int                      `json:"withAlerts"`
	LastUpdated         string                   `json:"lastUpdated"`
}

// GetPreferredResourceFor returns the preferred resource for a given ID.
// If the ID was merged into another resource, returns that preferred resource.
// If not merged, returns the resource itself. Returns nil if not found.
func (s *Store) GetPreferredResourceFor(resourceID string) *Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if this ID was merged
	if preferredID, merged := s.mergedFrom[resourceID]; merged {
		if r, ok := s.resources[preferredID]; ok {
			return r
		}
	}
	
	// Return the resource itself if it exists
	if r, ok := s.resources[resourceID]; ok {
		return r
	}
	return nil
}

// IsSamePhysicalMachine checks if two resource IDs represent the same physical machine.
// This is useful for alert deduplication between different source types (API vs Agent).
func (s *Store) IsSamePhysicalMachine(id1, id2 string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if they're literally the same
	if id1 == id2 {
		return true
	}
	
	// Check if both map to the same preferred resource
	preferred1 := id1
	if pid, merged := s.mergedFrom[id1]; merged {
		preferred1 = pid
	}
	
	preferred2 := id2
	if pid, merged := s.mergedFrom[id2]; merged {
		preferred2 = pid
	}
	
	return preferred1 == preferred2
}

// HasPreferredSourceForHostname checks if there's a resource with a preferred source
// (like host-agent) monitoring a machine with the given hostname.
// This helps alert managers determine if they should skip alerts from API sources.
func (s *Store) HasPreferredSourceForHostname(hostname string) bool {
	if hostname == "" {
		return false
	}
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	hostnameLower := strings.ToLower(hostname)
	resourceIDs, exists := s.byHostname[hostnameLower]
	if !exists {
		return false
	}
	
	// Check if any resource with this hostname has a preferred source type
	for _, id := range resourceIDs {
		if r, ok := s.resources[id]; ok {
			// Host agent and Docker agent sources are preferred over API
			if r.SourceType == SourceAgent || r.SourceType == SourceHybrid {
				return true
			}
		}
	}
	
	return false
}

// ============================================================================
// Polling Optimization Methods (Phase 3)
// These methods help reduce redundant API polling when agents are active
// ============================================================================

// ShouldSkipAPIPolling returns true if API polling should be skipped for the 
// given hostname because an agent is providing richer, more frequent data.
// This is useful for reducing load when both Proxmox API and host agents monitor
// the same machine.
func (s *Store) ShouldSkipAPIPolling(hostname string) bool {
	return s.HasPreferredSourceForHostname(hostname)
}

// GetAgentMonitoredHostnames returns a list of hostnames that are being monitored
// by agents (host-agent, docker-agent). This can be used by the monitoring loop
// to adjust polling behavior for these hosts.
func (s *Store) GetAgentMonitoredHostnames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var hostnames []string
	seen := make(map[string]bool)
	
	for _, r := range s.resources {
		if r.SourceType != SourceAgent && r.SourceType != SourceHybrid {
			continue
		}
		if r.Identity == nil || r.Identity.Hostname == "" {
			continue
		}
		hostnameLower := strings.ToLower(r.Identity.Hostname)
		if !seen[hostnameLower] {
			seen[hostnameLower] = true
			hostnames = append(hostnames, r.Identity.Hostname)
		}
	}
	
	return hostnames
}

// GetPollingRecommendations returns recommendations for optimizing polling.
// Returns a map of hostname -> recommended polling interval multiplier.
// - Value of 0 means skip entirely (agent provides all needed data)
// - Value > 1 means reduce frequency (poll less often)
// - Value of 1 means normal polling
func (s *Store) GetPollingRecommendations() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	recommendations := make(map[string]float64)
	
	for _, r := range s.resources {
		if r.Identity == nil || r.Identity.Hostname == "" {
			continue
		}
		hostname := strings.ToLower(r.Identity.Hostname)
		
		switch r.SourceType {
		case SourceAgent:
			// Agent provides all data - skip API polling for metrics
			// (we may still want to poll for cluster-wide info)
			recommendations[hostname] = 0
		case SourceHybrid:
			// Hybrid mode - reduce frequency but don't skip
			recommendations[hostname] = 0.5 // Poll at half frequency
		}
	}
	
	return recommendations
}

// findDuplicate looks for an existing resource that represents the same machine.
// Must be called with the lock held.
func (s *Store) findDuplicate(r *Resource) string {
	if r.Identity == nil {
		return ""
	}
	
	// 1. Machine ID match (most reliable) - but only for same type
	// A node and host agent on the same machine should coexist as different data sources
	if r.Identity.MachineID != "" && r.IsInfrastructure() {
		if existingID, ok := s.byMachineID[r.Identity.MachineID]; ok && existingID != r.ID {
			existing := s.resources[existingID]
			// Only match if same type
			if existing != nil && existing.Type == r.Type {
				return existingID
			}
		}
	}
	
	// 2. Hostname match (case-insensitive) - only for same infrastructure type
	// Workloads (VMs, containers) can have duplicate names across clusters
	if r.Identity.Hostname != "" && r.IsInfrastructure() {
		hostnameLower := strings.ToLower(r.Identity.Hostname)
		if existingIDs, ok := s.byHostname[hostnameLower]; ok {
			for _, existingID := range existingIDs {
				if existingID != r.ID {
					existing := s.resources[existingID]
					// Only match same infrastructure type (e.g., host with host, node with node)
					// Different types represent different data sources and should coexist
					if existing.Type == r.Type {
						return existingID
					}
				}
			}
		}
	}
	
	// 3. IP overlap (if same non-localhost IP, likely same machine) - only for same infrastructure type
	if r.IsInfrastructure() {
		for _, ip := range r.Identity.IPs {
			if isNonUniqueIP(ip) {
				continue
			}
			if existingIDs, ok := s.byIP[ip]; ok {
				for _, existingID := range existingIDs {
					if existingID != r.ID {
						existing := s.resources[existingID]
						// Only match same infrastructure type
						if existing.Type == r.Type {
							return existingID
						}
					}
				}
			}
		}
	}
	
	return ""
}

// preferredResource determines which of two duplicate resources should be kept.
// Agent data is preferred over API data.
func (s *Store) preferredResource(a, b *Resource) *Resource {
	// Prefer agent over API
	aScore := s.sourceScore(a.SourceType)
	bScore := s.sourceScore(b.SourceType)
	
	if aScore > bScore {
		return a
	}
	if bScore > aScore {
		return b
	}
	
	// Same source type - prefer the one with more recent data
	if a.LastSeen.After(b.LastSeen) {
		return a
	}
	return b
}

func (s *Store) sourceScore(st SourceType) int {
	switch st {
	case SourceAgent:
		return 3 // Agent data is most preferred
	case SourceHybrid:
		return 2 // Hybrid is second
	case SourceAPI:
		return 1 // API is least preferred
	default:
		return 0
	}
}

func (s *Store) addToIndexes(r *Resource) {
	if r.Identity == nil {
		return
	}
	
	if r.Identity.MachineID != "" {
		s.byMachineID[r.Identity.MachineID] = r.ID
	}
	
	if r.Identity.Hostname != "" {
		hostnameLower := strings.ToLower(r.Identity.Hostname)
		s.byHostname[hostnameLower] = append(s.byHostname[hostnameLower], r.ID)
	}
	
	for _, ip := range r.Identity.IPs {
		if !isNonUniqueIP(ip) {
			s.byIP[ip] = append(s.byIP[ip], r.ID)
		}
	}
}

func (s *Store) removeFromIndexes(r *Resource) {
	if r.Identity == nil {
		return
	}
	
	if r.Identity.MachineID != "" {
		if s.byMachineID[r.Identity.MachineID] == r.ID {
			delete(s.byMachineID, r.Identity.MachineID)
		}
	}
	
	if r.Identity.Hostname != "" {
		hostnameLower := strings.ToLower(r.Identity.Hostname)
		s.byHostname[hostnameLower] = removeFromSlice(s.byHostname[hostnameLower], r.ID)
		if len(s.byHostname[hostnameLower]) == 0 {
			delete(s.byHostname, hostnameLower)
		}
	}
	
	for _, ip := range r.Identity.IPs {
		s.byIP[ip] = removeFromSlice(s.byIP[ip], r.ID)
		if len(s.byIP[ip]) == 0 {
			delete(s.byIP, ip)
		}
	}
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// isNonUniqueIP returns true if the IP address is not useful for machine identification.
// This includes localhost addresses and Docker bridge IPs that exist on every Docker host.
func isNonUniqueIP(ip string) bool {
	// Localhost addresses
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "127.") {
		return true
	}
	
	// Docker bridge network - 172.17.0.1/16 exists on every Docker host
	// Also filter other Docker-assigned bridge networks (172.17-31.x.x)
	if strings.HasPrefix(ip, "172.17.") || strings.HasPrefix(ip, "172.18.") || 
	   strings.HasPrefix(ip, "172.19.") || strings.HasPrefix(ip, "172.20.") ||
	   strings.HasPrefix(ip, "172.21.") || strings.HasPrefix(ip, "172.22.") {
		return true
	}
	
	// Link-local addresses (fe80::)
	if strings.HasPrefix(strings.ToLower(ip), "fe80:") {
		return true
	}
	
	return false
}

// MarkStale marks resources that haven't been updated recently.
func (s *Store) MarkStale(threshold time.Duration) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	var staleIDs []string
	
	for id, r := range s.resources {
		if now.Sub(r.LastSeen) > threshold {
			// Mark as offline/degraded
			if r.Status == StatusOnline || r.Status == StatusRunning {
				r.Status = StatusDegraded
				staleIDs = append(staleIDs, id)
			}
		}
	}
	
	return staleIDs
}

// PruneStale removes resources that have been stale for too long.
func (s *Store) PruneStale(staleThreshold, removeThreshold time.Duration) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	var removedIDs []string
	
	for id, r := range s.resources {
		if now.Sub(r.LastSeen) > removeThreshold {
			s.removeFromIndexes(r)
			delete(s.resources, id)
			removedIDs = append(removedIDs, id)
		}
	}
	
	return removedIDs
}

// Query provides a fluent interface for querying resources.
func (s *Store) Query() *ResourceQuery {
	return &ResourceQuery{store: s}
}

// ResourceQuery provides a fluent query interface.
type ResourceQuery struct {
	store       *Store
	types       []ResourceType
	platforms   []PlatformType
	statuses    []ResourceStatus
	parentID    *string
	clusterID   *string
	hasAlerts   *bool
	sortBy      string
	sortDesc    bool
	limit       int
	offset      int
}

// OfType filters by resource types.
func (q *ResourceQuery) OfType(types ...ResourceType) *ResourceQuery {
	q.types = types
	return q
}

// FromPlatform filters by platform types.
func (q *ResourceQuery) FromPlatform(platforms ...PlatformType) *ResourceQuery {
	q.platforms = platforms
	return q
}

// WithStatus filters by resource status.
func (q *ResourceQuery) WithStatus(statuses ...ResourceStatus) *ResourceQuery {
	q.statuses = statuses
	return q
}

// WithParent filters by parent ID.
func (q *ResourceQuery) WithParent(parentID string) *ResourceQuery {
	q.parentID = &parentID
	return q
}

// InCluster filters by cluster ID.
func (q *ResourceQuery) InCluster(clusterID string) *ResourceQuery {
	q.clusterID = &clusterID
	return q
}

// WithAlerts filters to resources that have active alerts.
func (q *ResourceQuery) WithAlerts() *ResourceQuery {
	hasAlerts := true
	q.hasAlerts = &hasAlerts
	return q
}

// SortBy sets the sort field.
func (q *ResourceQuery) SortBy(field string, desc bool) *ResourceQuery {
	q.sortBy = field
	q.sortDesc = desc
	return q
}

// Limit sets the maximum number of results.
func (q *ResourceQuery) Limit(n int) *ResourceQuery {
	q.limit = n
	return q
}

// Offset sets the offset for pagination.
func (q *ResourceQuery) Offset(n int) *ResourceQuery {
	q.offset = n
	return q
}

// Execute runs the query and returns matching resources.
func (q *ResourceQuery) Execute() []Resource {
	q.store.mu.RLock()
	defer q.store.mu.RUnlock()
	
	var results []Resource
	
	for _, r := range q.store.resources {
		if q.matches(r) {
			results = append(results, *r)
		}
	}
	
	// TODO: Implement sorting
	
	// Apply pagination
	if q.offset > 0 {
		if q.offset >= len(results) {
			return []Resource{}
		}
		results = results[q.offset:]
	}
	
	if q.limit > 0 && q.limit < len(results) {
		results = results[:q.limit]
	}
	
	return results
}

// Count returns the number of matching resources without fetching them.
func (q *ResourceQuery) Count() int {
	q.store.mu.RLock()
	defer q.store.mu.RUnlock()
	
	count := 0
	for _, r := range q.store.resources {
		if q.matches(r) {
			count++
		}
	}
	return count
}

func (q *ResourceQuery) matches(r *Resource) bool {
	// Type filter
	if len(q.types) > 0 {
		found := false
		for _, t := range q.types {
			if r.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Platform filter
	if len(q.platforms) > 0 {
		found := false
		for _, p := range q.platforms {
			if r.PlatformType == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Status filter
	if len(q.statuses) > 0 {
		found := false
		for _, s := range q.statuses {
			if r.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Parent filter
	if q.parentID != nil && r.ParentID != *q.parentID {
		return false
	}
	
	// Cluster filter
	if q.clusterID != nil && r.ClusterID != *q.clusterID {
		return false
	}
	
	// Alerts filter
	if q.hasAlerts != nil && *q.hasAlerts && len(r.Alerts) == 0 {
		return false
	}
	
	return true
}

// ============================================================================
// Cross-Platform Analysis Methods
// These methods support AI queries like "what's using the most CPU"
// ============================================================================

// GetTopByCPU returns resources sorted by CPU usage (highest first).
// Optionally filter by resource types. Pass nil to include all types.
func (s *Store) GetTopByCPU(limit int, types []ResourceType) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var resources []Resource
	for _, r := range s.resources {
		if r.CPU == nil || r.CPU.Current == 0 {
			continue
		}
		if len(types) > 0 {
			matched := false
			for _, t := range types {
				if r.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		resources = append(resources, *r)
	}
	
	// Sort by CPU usage descending
	for i := 0; i < len(resources)-1; i++ {
		for j := i + 1; j < len(resources); j++ {
			if resources[j].CPUPercent() > resources[i].CPUPercent() {
				resources[i], resources[j] = resources[j], resources[i]
			}
		}
	}
	
	if limit > 0 && limit < len(resources) {
		return resources[:limit]
	}
	return resources
}

// GetTopByMemory returns resources sorted by memory usage (highest first).
// Optionally filter by resource types. Pass nil to include all types.
func (s *Store) GetTopByMemory(limit int, types []ResourceType) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var resources []Resource
	for _, r := range s.resources {
		if r.Memory == nil || r.Memory.Current == 0 {
			continue
		}
		if len(types) > 0 {
			matched := false
			for _, t := range types {
				if r.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		resources = append(resources, *r)
	}
	
	// Sort by memory usage descending
	for i := 0; i < len(resources)-1; i++ {
		for j := i + 1; j < len(resources); j++ {
			if resources[j].MemoryPercent() > resources[i].MemoryPercent() {
				resources[i], resources[j] = resources[j], resources[i]
			}
		}
	}
	
	if limit > 0 && limit < len(resources) {
		return resources[:limit]
	}
	return resources
}

// GetTopByDisk returns resources sorted by disk usage (highest first).
// Optionally filter by resource types. Pass nil to include all types.
func (s *Store) GetTopByDisk(limit int, types []ResourceType) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var resources []Resource
	for _, r := range s.resources {
		if r.Disk == nil || r.Disk.Current == 0 {
			continue
		}
		if len(types) > 0 {
			matched := false
			for _, t := range types {
				if r.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		resources = append(resources, *r)
	}
	
	// Sort by disk usage descending
	for i := 0; i < len(resources)-1; i++ {
		for j := i + 1; j < len(resources); j++ {
			if resources[j].DiskPercent() > resources[i].DiskPercent() {
				resources[i], resources[j] = resources[j], resources[i]
			}
		}
	}
	
	if limit > 0 && limit < len(resources) {
		return resources[:limit]
	}
	return resources
}

// GetRelated returns resources that are related to the given resource.
// This includes: parent, children, siblings (same parent), and co-located resources (same cluster).
func (s *Store) GetRelated(resourceID string) map[string][]Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string][]Resource)
	
	r, ok := s.resources[resourceID]
	if !ok {
		return result
	}
	
	// Get parent
	if r.ParentID != "" {
		if parent, ok := s.resources[r.ParentID]; ok {
			result["parent"] = []Resource{*parent}
		}
	}
	
	// Get children
	var children []Resource
	for _, other := range s.resources {
		if other.ParentID == resourceID {
			children = append(children, *other)
		}
	}
	if len(children) > 0 {
		result["children"] = children
	}
	
	// Get siblings (same parent)
	if r.ParentID != "" {
		var siblings []Resource
		for _, other := range s.resources {
			if other.ID != resourceID && other.ParentID == r.ParentID {
				siblings = append(siblings, *other)
			}
		}
		if len(siblings) > 0 {
			result["siblings"] = siblings
		}
	}
	
	// Get co-located resources (same cluster)
	if r.ClusterID != "" {
		var colocated []Resource
		for _, other := range s.resources {
			if other.ID != resourceID && other.ClusterID == r.ClusterID {
				colocated = append(colocated, *other)
			}
		}
		if len(colocated) > 0 {
			result["cluster_members"] = colocated
		}
	}
	
	return result
}

// GetResourceSummary returns a summary of resource utilization across the infrastructure.
// This is useful for AI to get a quick overview of the infrastructure state.
func (s *Store) GetResourceSummary() ResourceSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	summary := ResourceSummary{
		ByType:     make(map[ResourceType]TypeSummary),
		ByPlatform: make(map[PlatformType]PlatformSummary),
	}
	
	for _, r := range s.resources {
		summary.TotalResources++
		
		// Count by status
		switch r.Status {
		case StatusOnline, StatusRunning:
			summary.Healthy++
		case StatusDegraded:
			summary.Degraded++
		case StatusOffline, StatusStopped, StatusUnknown:
			summary.Offline++
		}
		
		// Track alerts
		if len(r.Alerts) > 0 {
			summary.WithAlerts++
		}
		
		// Aggregate by type
		ts := summary.ByType[r.Type]
		ts.Count++
		if r.CPU != nil {
			ts.TotalCPUPercent += r.CPUPercent()
		}
		if r.Memory != nil {
			ts.TotalMemoryPercent += r.MemoryPercent()
		}
		summary.ByType[r.Type] = ts
		
		// Aggregate by platform
		ps := summary.ByPlatform[r.PlatformType]
		ps.Count++
		summary.ByPlatform[r.PlatformType] = ps
	}
	
	// Calculate averages
	for t, ts := range summary.ByType {
		if ts.Count > 0 {
			ts.AvgCPUPercent = ts.TotalCPUPercent / float64(ts.Count)
			ts.AvgMemoryPercent = ts.TotalMemoryPercent / float64(ts.Count)
			summary.ByType[t] = ts
		}
	}
	
	return summary
}

// ResourceSummary provides an overview of resource utilization.
type ResourceSummary struct {
	TotalResources int
	Healthy        int
	Degraded       int
	Offline        int
	WithAlerts     int
	ByType         map[ResourceType]TypeSummary
	ByPlatform     map[PlatformType]PlatformSummary
}

// TypeSummary aggregates metrics for a resource type.
type TypeSummary struct {
	Count              int
	TotalCPUPercent    float64
	TotalMemoryPercent float64
	AvgCPUPercent      float64
	AvgMemoryPercent   float64
}

// PlatformSummary aggregates counts for a platform.
type PlatformSummary struct {
	Count int
}

// PopulateFromSnapshot converts all resources from a StateSnapshot to the unified store.
// This should be called whenever the state is updated (e.g., before WebSocket broadcasts).
func (s *Store) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	// Convert nodes
	for _, node := range snapshot.Nodes {
		r := FromNode(node)
		s.Upsert(r)
	}

	// Convert VMs
	for _, vm := range snapshot.VMs {
		r := FromVM(vm)
		s.Upsert(r)
	}

	// Convert containers
	for _, ct := range snapshot.Containers {
		r := FromContainer(ct)
		s.Upsert(r)
	}

	// Convert hosts
	for _, host := range snapshot.Hosts {
		r := FromHost(host)
		s.Upsert(r)
	}

	// Convert docker hosts and their containers
	for _, dh := range snapshot.DockerHosts {
		r := FromDockerHost(dh)
		s.Upsert(r)

		// Convert containers within the docker host
		for _, dc := range dh.Containers {
			r := FromDockerContainer(dc, dh.ID, dh.Hostname)
			s.Upsert(r)
		}
	}

	// Convert PBS instances
	for _, pbs := range snapshot.PBSInstances {
		r := FromPBSInstance(pbs)
		s.Upsert(r)
	}

	// Convert storage
	for _, storage := range snapshot.Storage {
		r := FromStorage(storage)
		s.Upsert(r)
	}
}
