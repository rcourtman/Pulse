package unifiedresources

import (
	"sort"
	"strings"
	"time"
)

// MonitoredSystemCandidate describes a prospective top-level monitored system
// that may be added through an agent report or API-backed registration.
type MonitoredSystemCandidate struct {
	Type       ResourceType
	Name       string
	Hostname   string
	HostURL    string
	AgentID    string
	MachineID  string
	ResourceID string
}

// MonitoredSystemRecord describes a counted top-level monitored system after
// canonical cross-view deduplication.
type MonitoredSystemRecord struct {
	Name     string
	Type     string
	Status   ResourceStatus
	LastSeen time.Time
	Source   string
}

// MonitoredSystemCount returns the number of top-level monitored systems after
// canonical cross-view deduplication. Child resources are intentionally
// excluded.
func MonitoredSystemCount(rs ReadState) int {
	return resolveMonitoredSystemTopLevelSystems(rs).Count()
}

// MonitoredSystems returns the canonical counted monitored systems after
// deduping overlapping top-level roots across collection paths.
func MonitoredSystems(rs ReadState) []MonitoredSystemRecord {
	records := resolveMonitoredSystemTopLevelSystems(rs).records()
	sort.Slice(records, func(i, j int) bool {
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		if records[i].Type != records[j].Type {
			return records[i].Type < records[j].Type
		}
		if !records[i].LastSeen.Equal(records[j].LastSeen) {
			return records[i].LastSeen.After(records[j].LastSeen)
		}
		return records[i].Source < records[j].Source
	})
	return records
}

// HasMatchingMonitoredSystem reports whether a prospective monitored system
// would dedupe onto an already-counted top-level monitored system.
func HasMatchingMonitoredSystem(rs ReadState, candidate MonitoredSystemCandidate) bool {
	return resolveMonitoredSystemTopLevelSystems(rs).HasMatchingCandidate(candidate)
}

type monitoredSystemGroup struct {
	keys      map[string]struct{}
	resources []*Resource
}

func monitoredSystemGroups(rs ReadState) []monitoredSystemGroup {
	resolver := resolveMonitoredSystemTopLevelSystems(rs)
	groups := make([]monitoredSystemGroup, 0, len(resolver.groups))
	for _, group := range resolver.groups {
		groups = append(groups, monitoredSystemGroup{
			keys:      cloneStringSet(group.strongIDs),
			resources: group.resources,
		})
	}
	return groups
}

func monitoredSystemRoots(rs ReadState) []*Resource {
	if rs == nil {
		return nil
	}

	roots := make([]*Resource, 0)
	for _, infra := range rs.Infrastructure() {
		if infra == nil || infra.r == nil {
			continue
		}
		roots = append(roots, infra.r)
	}
	for _, pbs := range rs.PBSInstances() {
		if pbs == nil || pbs.r == nil {
			continue
		}
		roots = append(roots, pbs.r)
	}
	for _, pmg := range rs.PMGInstances() {
		if pmg == nil || pmg.r == nil {
			continue
		}
		roots = append(roots, pmg.r)
	}
	for _, cluster := range rs.K8sClusters() {
		if cluster == nil || cluster.r == nil {
			continue
		}
		roots = append(roots, cluster.r)
	}

	return roots
}

func resolveMonitoredSystemTopLevelSystems(rs ReadState) TopLevelSystemResolver {
	roots := monitoredSystemRoots(rs)
	resources := make([]Resource, 0, len(roots))
	for _, resource := range roots {
		if resource == nil {
			continue
		}
		resources = append(resources, *resource)
	}
	return ResolveTopLevelSystems(resources)
}

func monitoredSystemRecord(group monitoredSystemGroup) MonitoredSystemRecord {
	resource := preferredMonitoredSystemResource(group.resources)
	record := MonitoredSystemRecord{
		Name:     monitoredSystemDisplayName(group.resources, resource),
		Type:     monitoredSystemType(resource),
		Status:   monitoredSystemStatus(group.resources),
		LastSeen: monitoredSystemLastSeen(group.resources),
		Source:   monitoredSystemSource(group.resources),
	}
	if record.Name == "" {
		record.Name = "Unnamed system"
	}
	if record.Type == "" {
		record.Type = "system"
	}
	if record.Status == "" {
		record.Status = StatusUnknown
	}
	if record.Source == "" {
		record.Source = "unknown"
	}
	return record
}

func preferredMonitoredSystemResource(resources []*Resource) *Resource {
	var preferred *Resource
	bestPriority := 1 << 30
	for _, resource := range resources {
		priority := monitoredSystemResourcePriority(resource)
		if priority < bestPriority {
			bestPriority = priority
			preferred = resource
		}
	}
	return preferred
}

func monitoredSystemResourcePriority(resource *Resource) int {
	if resource == nil {
		return 1 << 30
	}
	switch {
	case resource.Proxmox != nil:
		return 0
	case resource.TrueNAS != nil:
		return 1
	case resource.Docker != nil:
		return 2
	case resource.Agent != nil:
		return 3
	case resource.PBS != nil:
		return 10
	case resource.PMG != nil:
		return 11
	case resource.Kubernetes != nil:
		return 12
	default:
		return 100
	}
}

func monitoredSystemDisplayName(resources []*Resource, preferred *Resource) string {
	if name := monitoredSystemResourceDisplayName(preferred); name != "" {
		return name
	}
	for _, resource := range resources {
		if name := monitoredSystemResourceDisplayName(resource); name != "" {
			return name
		}
	}
	return ""
}

func monitoredSystemResourceDisplayName(resource *Resource) string {
	if resource == nil {
		return ""
	}
	if name := ResourceDisplayName(*resource); name != "" {
		return name
	}
	switch {
	case resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.NodeName) != "":
		return strings.TrimSpace(resource.Proxmox.NodeName)
	case resource.Agent != nil && strings.TrimSpace(resource.Agent.Hostname) != "":
		return strings.TrimSpace(resource.Agent.Hostname)
	case resource.Docker != nil && strings.TrimSpace(resource.Docker.Hostname) != "":
		return strings.TrimSpace(resource.Docker.Hostname)
	case resource.TrueNAS != nil && strings.TrimSpace(resource.TrueNAS.Hostname) != "":
		return strings.TrimSpace(resource.TrueNAS.Hostname)
	case resource.PBS != nil && strings.TrimSpace(resource.PBS.Hostname) != "":
		return strings.TrimSpace(resource.PBS.Hostname)
	case resource.PMG != nil && strings.TrimSpace(resource.PMG.Hostname) != "":
		return strings.TrimSpace(resource.PMG.Hostname)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.ClusterName) != "":
		return strings.TrimSpace(resource.Kubernetes.ClusterName)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.SourceName) != "":
		return strings.TrimSpace(resource.Kubernetes.SourceName)
	}
	return strings.TrimSpace(resource.ID)
}

func monitoredSystemType(resource *Resource) string {
	if resource == nil {
		return ""
	}
	switch {
	case resource.Proxmox != nil:
		return "proxmox-node"
	case resource.TrueNAS != nil:
		return "truenas-system"
	case resource.Docker != nil:
		return "docker-host"
	case resource.Agent != nil:
		return "host"
	case resource.PBS != nil:
		return "pbs-server"
	case resource.PMG != nil:
		return "pmg-server"
	case resource.Kubernetes != nil:
		return "kubernetes-cluster"
	default:
		return string(CanonicalResourceType(resource.Type))
	}
}

func monitoredSystemStatus(resources []*Resource) ResourceStatus {
	best := StatusUnknown
	bestPriority := monitoredSystemStatusPriority(best)
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		priority := monitoredSystemStatusPriority(resource.Status)
		if priority < bestPriority {
			best = resource.Status
			bestPriority = priority
		}
	}
	return best
}

func monitoredSystemStatusPriority(status ResourceStatus) int {
	switch status {
	case StatusWarning:
		return 0
	case StatusOffline:
		return 1
	case StatusUnknown:
		return 2
	case StatusOnline:
		return 3
	default:
		return 4
	}
}

func monitoredSystemLastSeen(resources []*Resource) time.Time {
	var lastSeen time.Time
	for _, resource := range resources {
		if resource == nil || resource.LastSeen.IsZero() {
			continue
		}
		if lastSeen.IsZero() || resource.LastSeen.After(lastSeen) {
			lastSeen = resource.LastSeen
		}
	}
	return lastSeen
}

func monitoredSystemSource(resources []*Resource) string {
	sources := make(map[string]struct{})
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		if source := monitoredSystemPrimarySource(resource); source != "" {
			sources[source] = struct{}{}
		}
	}
	if len(sources) == 0 {
		return ""
	}
	if len(sources) > 1 {
		return "multiple"
	}
	for source := range sources {
		return source
	}
	return ""
}

func monitoredSystemPrimarySource(resource *Resource) string {
	if resource == nil {
		return ""
	}
	switch {
	case resource.Proxmox != nil:
		return string(SourceProxmox)
	case resource.TrueNAS != nil:
		return string(SourceTrueNAS)
	case resource.Docker != nil:
		return string(SourceDocker)
	case resource.Agent != nil:
		return string(SourceAgent)
	case resource.PBS != nil:
		return string(SourcePBS)
	case resource.PMG != nil:
		return string(SourcePMG)
	case resource.Kubernetes != nil:
		return string(SourceK8s)
	}
	if len(resource.Sources) > 0 {
		return string(resource.Sources[0])
	}
	return ""
}

func cloneStringSet(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
