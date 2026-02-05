package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const autoMergeThreshold = 0.85

var defaultStaleThresholds = map[DataSource]time.Duration{
	SourceProxmox: 60 * time.Second,
	SourceAgent:   60 * time.Second,
	SourceDocker:  120 * time.Second,
	SourcePBS:     120 * time.Second,
	SourcePMG:     120 * time.Second,
	SourceK8s:     120 * time.Second,
}

// ResourceRegistry merges resources from multiple sources.
type ResourceRegistry struct {
	mu         sync.RWMutex
	resources  map[string]*Resource
	bySource   map[DataSource]map[string]string
	matcher    *IdentityMatcher
	store      ResourceStore
	links      []ResourceLink
	exclusions map[string]struct{}
}

// NewRegistry creates a new registry using the provided store for overrides.
func NewRegistry(store ResourceStore) *ResourceRegistry {
	rr := &ResourceRegistry{
		resources:  make(map[string]*Resource),
		bySource:   make(map[DataSource]map[string]string),
		matcher:    NewIdentityMatcher(),
		store:      store,
		exclusions: make(map[string]struct{}),
	}

	rr.bySource[SourceProxmox] = make(map[string]string)
	rr.bySource[SourceAgent] = make(map[string]string)
	rr.bySource[SourceDocker] = make(map[string]string)
	rr.bySource[SourcePBS] = make(map[string]string)
	rr.bySource[SourcePMG] = make(map[string]string)
	rr.bySource[SourceK8s] = make(map[string]string)

	rr.loadOverrides()
	return rr
}

func (rr *ResourceRegistry) loadOverrides() {
	if rr.store == nil {
		return
	}
	links, err := rr.store.GetLinks()
	if err == nil {
		rr.links = links
	}
	exclusions, err := rr.store.GetExclusions()
	if err == nil {
		for _, exclusion := range exclusions {
			key := exclusionKey(exclusion.ResourceA, exclusion.ResourceB)
			rr.exclusions[key] = struct{}{}
		}
	}
}

// IngestSnapshot ingests all resources from the current state snapshot.
func (rr *ResourceRegistry) IngestSnapshot(snapshot models.StateSnapshot) {
	for _, node := range snapshot.Nodes {
		rr.ingestProxmoxNode(node)
	}
	for _, host := range snapshot.Hosts {
		rr.ingestHost(host)
	}
	for _, dh := range snapshot.DockerHosts {
		rr.ingestDockerHost(dh)
	}
	for _, vm := range snapshot.VMs {
		rr.ingestVM(vm)
	}
	for _, ct := range snapshot.Containers {
		rr.ingestContainer(ct)
	}
	for _, dh := range snapshot.DockerHosts {
		for _, dc := range dh.Containers {
			rr.ingestDockerContainer(dc, dh)
		}
	}

	rr.applyManualLinks()
	rr.buildChildCounts()
	rr.MarkStale(time.Now().UTC(), nil)
}

// List returns all resources.
func (rr *ResourceRegistry) List() []Resource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	out := make([]Resource, 0, len(rr.resources))
	for _, r := range rr.resources {
		out = append(out, *r)
	}
	return out
}

// Get returns a resource by ID.
func (rr *ResourceRegistry) Get(id string) (*Resource, bool) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	r, ok := rr.resources[id]
	return r, ok
}

// SourceTargets returns the source-specific IDs that map to the provided resource ID.
func (rr *ResourceRegistry) SourceTargets(resourceID string) []SourceTarget {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	resource := rr.resources[resourceID]
	if resource == nil {
		return nil
	}

	out := make([]SourceTarget, 0)
	for source, mapping := range rr.bySource {
		for sourceID, mappedID := range mapping {
			if mappedID != resourceID {
				continue
			}
			out = append(out, SourceTarget{
				Source:      source,
				SourceID:    sourceID,
				CandidateID: rr.sourceSpecificID(resource.Type, source, sourceID),
			})
		}
	}
	return out
}

// GetChildren returns child resources for a parent.
func (rr *ResourceRegistry) GetChildren(parentID string) []Resource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	var out []Resource
	for _, r := range rr.resources {
		if r.ParentID != nil && *r.ParentID == parentID {
			out = append(out, *r)
		}
	}
	return out
}

// Stats returns aggregated stats.
func (rr *ResourceRegistry) Stats() ResourceStats {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	stats := ResourceStats{
		Total:    len(rr.resources),
		ByType:   make(map[ResourceType]int),
		ByStatus: make(map[ResourceStatus]int),
		BySource: make(map[DataSource]int),
	}
	for _, r := range rr.resources {
		stats.ByType[r.Type]++
		stats.ByStatus[r.Status]++
		for _, source := range r.Sources {
			stats.BySource[source]++
		}
	}
	return stats
}

// MarkStale marks sources as stale based on last seen timestamps.
// If thresholds is nil, default thresholds are used.
func (rr *ResourceRegistry) MarkStale(now time.Time, thresholds map[DataSource]time.Duration) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if thresholds == nil {
		thresholds = defaultStaleThresholds
	}

	for _, resource := range rr.resources {
		staleFound := false
		for source, status := range resource.SourceStatus {
			threshold, ok := thresholds[source]
			if !ok {
				threshold = 120 * time.Second
			}
			if status.LastSeen.IsZero() {
				continue
			}
			if now.Sub(status.LastSeen) > threshold {
				status.Status = "stale"
				resource.SourceStatus[source] = status
				staleFound = true
			}
		}
		if staleFound && resource.Status == StatusOnline {
			resource.Status = StatusWarning
		}
	}
}

func (rr *ResourceRegistry) ingestProxmoxNode(node models.Node) {
	resource, identity := resourceFromProxmoxNode(node)
	rr.ingest(SourceProxmox, node.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestHost(host models.Host) {
	resource, identity := resourceFromHost(host)
	rr.ingest(SourceAgent, host.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerHost(host models.DockerHost) {
	resource, identity := resourceFromDockerHost(host)
	rr.ingest(SourceDocker, host.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestVM(vm models.VM) {
	resource, identity := resourceFromVM(vm)
	parentSourceID := proxmoxNodeSourceID(vm.Instance, vm.Node)
	if parentID, ok := rr.bySource[SourceProxmox][parentSourceID]; ok {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceProxmox, vm.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestContainer(ct models.Container) {
	resource, identity := resourceFromContainer(ct)
	parentSourceID := proxmoxNodeSourceID(ct.Instance, ct.Node)
	if parentID, ok := rr.bySource[SourceProxmox][parentSourceID]; ok {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceProxmox, ct.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerContainer(ct models.DockerContainer, host models.DockerHost) {
	resource, identity := resourceFromDockerContainer(ct)
	if parentID, ok := rr.bySource[SourceDocker][host.ID]; ok {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceDocker, ct.ID, resource, identity)
}

func (rr *ResourceRegistry) ingest(source DataSource, sourceID string, resource Resource, identity ResourceIdentity) string {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	resource.Identity = identity
	resource.Sources = []DataSource{source}
	resource.SourceStatus = map[DataSource]SourceStatus{
		source: {Status: "online", LastSeen: resource.LastSeen},
	}

	if resource.LastSeen.IsZero() {
		resource.LastSeen = time.Now().UTC()
	}

	// Link hints override identity matching
	if linked := rr.resolveLinkedResource(source, resource); linked != "" {
		existing := rr.resources[linked]
		if existing != nil {
			rr.mergeInto(existing, resource, source)
			rr.bySource[source][sourceID] = existing.ID
			return existing.ID
		}
	}

	candidateID := rr.sourceSpecificID(resource.Type, source, sourceID)

	if resource.Type == ResourceTypeHost {
		if match, excluded := rr.findMatch(identity, resource.Type, candidateID); match != nil {
			existing := rr.resources[match.ResourceB]
			if existing != nil {
				rr.mergeInto(existing, resource, source)
				rr.bySource[source][sourceID] = existing.ID
				return existing.ID
			}
		} else if excluded {
			resource.ID = candidateID
			rr.resources[resource.ID] = &resource
			rr.bySource[source][sourceID] = resource.ID
			rr.matcher.Add(resource.ID, identity)
			return resource.ID
		}
	}

	resource.ID = rr.chooseNewID(resource.Type, identity, source, sourceID)
	rr.resources[resource.ID] = &resource
	rr.bySource[source][sourceID] = resource.ID
	rr.matcher.Add(resource.ID, identity)

	return resource.ID
}

func (rr *ResourceRegistry) findMatch(identity ResourceIdentity, resourceType ResourceType, candidateID string) (*MatchResult, bool) {
	excludedMatch := false
	candidates := rr.matcher.FindCandidates(identity)
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		if candidate.Confidence < autoMergeThreshold {
			continue
		}
		existing := rr.resources[candidate.ID]
		if existing == nil || existing.Type != resourceType {
			continue
		}
		if rr.isExcluded(candidate.ID, candidateID) {
			excludedMatch = true
			continue
		}
		return &MatchResult{
			ResourceA:      candidateID,
			ResourceB:      candidate.ID,
			Confidence:     candidate.Confidence,
			MatchReason:    candidate.Reason,
			RequiresReview: candidate.RequiresReview,
		}, false
	}
	return nil, excludedMatch
}

func (rr *ResourceRegistry) resolveLinkedResource(source DataSource, resource Resource) string {
	switch source {
	case SourceProxmox:
		if resource.Proxmox != nil && resource.Proxmox.LinkedHostAgentID != "" {
			if id, ok := rr.bySource[SourceAgent][resource.Proxmox.LinkedHostAgentID]; ok {
				return id
			}
		}
	case SourceAgent:
		if resource.Agent != nil && resource.Agent.LinkedNodeID != "" {
			if id, ok := rr.bySource[SourceProxmox][resource.Agent.LinkedNodeID]; ok {
				return id
			}
		}
	}
	return ""
}

func (rr *ResourceRegistry) mergeInto(existing *Resource, incoming Resource, source DataSource) {
	if existing == nil {
		return
	}

	// Merge identity
	existing.Identity = mergeIdentity(existing.Identity, incoming.Identity)

	// Merge tags
	existing.Tags = uniqueStrings(append(existing.Tags, incoming.Tags...))

	// Update source payload
	switch source {
	case SourceProxmox:
		existing.Proxmox = incoming.Proxmox
	case SourceAgent:
		existing.Agent = incoming.Agent
	case SourceDocker:
		existing.Docker = incoming.Docker
	case SourcePBS:
		existing.PBS = incoming.PBS
	case SourceK8s:
		existing.Kubernetes = incoming.Kubernetes
	}

	existing.Sources = addSource(existing.Sources, source)
	if existing.SourceStatus == nil {
		existing.SourceStatus = make(map[DataSource]SourceStatus)
	}
	existing.SourceStatus[source] = SourceStatus{Status: "online", LastSeen: incoming.LastSeen}

	if incoming.LastSeen.After(existing.LastSeen) {
		existing.LastSeen = incoming.LastSeen
	}
	existing.UpdatedAt = time.Now().UTC()

	existing.Status = chooseStatus(existing.Status, incoming.Status, source)
	existing.Metrics = mergeMetrics(existing.Metrics, incoming.Metrics, source)

	// Prefer agent naming when available
	if incoming.Name != "" {
		if existing.Name == "" || sourcePriority(source) >= sourcePriority(SourceAgent) {
			existing.Name = incoming.Name
		}
	}
}

func (rr *ResourceRegistry) applyManualLinks() {
	if len(rr.links) == 0 {
		return
	}
	for _, link := range rr.links {
		primaryID := link.PrimaryID
		if primaryID == "" {
			primaryID = link.ResourceA
		}
		primary := rr.resources[primaryID]
		if primary == nil {
			primary = rr.resources[link.ResourceA]
			primaryID = link.ResourceA
		}
		if primary == nil {
			primary = rr.resources[link.ResourceB]
			primaryID = link.ResourceB
		}
		if primary == nil {
			continue
		}

		otherID := link.ResourceB
		if otherID == primaryID {
			otherID = link.ResourceA
		}
		other := rr.resources[otherID]
		if other == nil || otherID == primaryID {
			continue
		}

		rr.mergeResourceData(primary, other)
		delete(rr.resources, otherID)
		rr.updateSourceMappings(otherID, primaryID)
	}
}

func (rr *ResourceRegistry) mergeResourceData(primary *Resource, other *Resource) {
	if other == nil || primary == nil {
		return
	}
	primary.Identity = mergeIdentity(primary.Identity, other.Identity)
	primary.Tags = uniqueStrings(append(primary.Tags, other.Tags...))
	primary.Sources = addSources(primary.Sources, other.Sources)
	if primary.SourceStatus == nil {
		primary.SourceStatus = make(map[DataSource]SourceStatus)
	}
	for source, status := range other.SourceStatus {
		primary.SourceStatus[source] = status
	}
	if other.LastSeen.After(primary.LastSeen) {
		primary.LastSeen = other.LastSeen
	}
	primary.UpdatedAt = time.Now().UTC()

	if primary.Proxmox == nil {
		primary.Proxmox = other.Proxmox
	}
	if primary.Agent == nil {
		primary.Agent = other.Agent
	}
	if primary.Docker == nil {
		primary.Docker = other.Docker
	}
	if primary.PBS == nil {
		primary.PBS = other.PBS
	}
	if primary.Kubernetes == nil {
		primary.Kubernetes = other.Kubernetes
	}

	primary.Metrics = mergeMetrics(primary.Metrics, other.Metrics, SourceAgent)
	primary.Status = aggregateStatus(primary)
}

func (rr *ResourceRegistry) updateSourceMappings(fromID, toID string) {
	for source, mapping := range rr.bySource {
		for key, value := range mapping {
			if value == fromID {
				mapping[key] = toID
			}
		}
		rr.bySource[source] = mapping
	}
}

func (rr *ResourceRegistry) buildChildCounts() {
	childCounts := make(map[string]int)
	for _, r := range rr.resources {
		if r.ParentID != nil {
			childCounts[*r.ParentID]++
		}
	}
	for id, count := range childCounts {
		if r, ok := rr.resources[id]; ok {
			r.ChildCount = count
		}
	}
	// Resolve parent names for child resources.
	for _, r := range rr.resources {
		if r.ParentID != nil {
			if parent, ok := rr.resources[*r.ParentID]; ok {
				r.ParentName = parent.Name
			}
		}
	}
}

func (rr *ResourceRegistry) chooseNewID(resourceType ResourceType, identity ResourceIdentity, source DataSource, sourceID string) string {
	if resourceType != ResourceTypeHost {
		return rr.sourceSpecificID(resourceType, source, sourceID)
	}
	if identity.MachineID != "" || identity.DMIUUID != "" || identity.ClusterName != "" {
		return rr.canonicalIDFromIdentity(resourceType, identity)
	}
	return rr.sourceSpecificID(resourceType, source, sourceID)
}

func (rr *ResourceRegistry) canonicalIDFromIdentity(resourceType ResourceType, identity ResourceIdentity) string {
	var stable string
	switch {
	case identity.MachineID != "":
		stable = "machine:" + strings.TrimSpace(identity.MachineID)
	case identity.DMIUUID != "":
		stable = "dmi:" + strings.TrimSpace(identity.DMIUUID)
	case identity.ClusterName != "" && len(identity.Hostnames) > 0:
		stable = fmt.Sprintf("cluster:%s:%s", identity.ClusterName, NormalizeHostname(identity.Hostnames[0]))
	case len(identity.Hostnames) > 0:
		stable = "hostname:" + NormalizeHostname(identity.Hostnames[0])
	case len(identity.IPAddresses) > 0:
		stable = "ip:" + NormalizeIP(identity.IPAddresses[0])
	default:
		stable = fmt.Sprintf("unknown:%d", time.Now().UnixNano())
	}
	return buildHashID(resourceType, stable)
}

func (rr *ResourceRegistry) sourceSpecificID(resourceType ResourceType, source DataSource, sourceID string) string {
	stable := fmt.Sprintf("%s:%s", source, sourceID)
	return buildHashID(resourceType, stable)
}

func buildHashID(resourceType ResourceType, stable string) string {
	hash := sha256.Sum256([]byte(stable))
	return fmt.Sprintf("%s-%s", resourceType, hex.EncodeToString(hash[:8]))
}

func proxmoxNodeSourceID(instance, nodeName string) string {
	if instance == "" {
		return nodeName
	}
	return fmt.Sprintf("%s-%s", instance, nodeName)
}

func mergeIdentity(existing ResourceIdentity, incoming ResourceIdentity) ResourceIdentity {
	if existing.MachineID == "" {
		existing.MachineID = incoming.MachineID
	}
	if existing.DMIUUID == "" {
		existing.DMIUUID = incoming.DMIUUID
	}
	if existing.ClusterName == "" {
		existing.ClusterName = incoming.ClusterName
	}
	existing.Hostnames = uniqueStrings(append(existing.Hostnames, incoming.Hostnames...))
	existing.IPAddresses = uniqueStrings(append(existing.IPAddresses, incoming.IPAddresses...))
	existing.MACAddresses = uniqueStrings(append(existing.MACAddresses, incoming.MACAddresses...))
	return existing
}

func addSource(sources []DataSource, source DataSource) []DataSource {
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func addSources(sources []DataSource, more []DataSource) []DataSource {
	out := sources
	for _, source := range more {
		out = addSource(out, source)
	}
	return out
}

func mergeMetrics(existing *ResourceMetrics, incoming *ResourceMetrics, source DataSource) *ResourceMetrics {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}
	merged := *existing
	merged.CPU = mergeMetric(existing.CPU, incoming.CPU, source)
	merged.Memory = mergeMetric(existing.Memory, incoming.Memory, source)
	merged.Disk = mergeMetric(existing.Disk, incoming.Disk, source)
	merged.NetIn = mergeMetric(existing.NetIn, incoming.NetIn, source)
	merged.NetOut = mergeMetric(existing.NetOut, incoming.NetOut, source)
	merged.DiskRead = mergeMetric(existing.DiskRead, incoming.DiskRead, source)
	merged.DiskWrite = mergeMetric(existing.DiskWrite, incoming.DiskWrite, source)
	return &merged
}

func mergeMetric(existing *MetricValue, incoming *MetricValue, source DataSource) *MetricValue {
	if incoming == nil {
		return existing
	}
	incomingCopy := *incoming
	incomingCopy.Source = source
	if existing == nil {
		return &incomingCopy
	}
	if sourcePriority(source) >= sourcePriority(existing.Source) {
		return &incomingCopy
	}
	return existing
}

func sourcePriority(source DataSource) int {
	switch source {
	case SourceAgent:
		return 3
	case SourceProxmox:
		return 2
	case SourceDocker:
		return 1
	default:
		return 0
	}
}

func chooseStatus(existing ResourceStatus, incoming ResourceStatus, source DataSource) ResourceStatus {
	if existing == "" || existing == StatusUnknown {
		return incoming
	}
	if sourcePriority(source) >= sourcePriority(SourceAgent) {
		return incoming
	}
	return existing
}

func aggregateStatus(resource *Resource) ResourceStatus {
	statusPriority := map[string]int{
		"online":  3,
		"stale":   2,
		"offline": 1,
	}
	best := StatusUnknown
	bestScore := 0
	for _, status := range resource.SourceStatus {
		score := statusPriority[strings.ToLower(status.Status)]
		if score > bestScore {
			bestScore = score
			if status.Status == "online" {
				best = StatusOnline
			} else if status.Status == "stale" {
				best = StatusWarning
			} else if status.Status == "offline" {
				best = StatusOffline
			}
		}
	}
	if best == "" {
		return StatusUnknown
	}
	return best
}

func (rr *ResourceRegistry) isExcluded(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	_, ok := rr.exclusions[exclusionKey(a, b)]
	return ok
}

func exclusionKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// Stable ordering helper for deterministic output.
func sortResourcesByName(resources []Resource) {
	sort.Slice(resources, func(i, j int) bool {
		return strings.ToLower(resources[i].Name) < strings.ToLower(resources[j].Name)
	})
}
