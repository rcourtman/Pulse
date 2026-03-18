package unifiedresources

import (
	"net"
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
	return len(monitoredSystemGroups(rs))
}

// MonitoredSystems returns the canonical counted monitored systems after
// deduping overlapping top-level roots across collection paths.
func MonitoredSystems(rs ReadState) []MonitoredSystemRecord {
	groups := monitoredSystemGroups(rs)
	records := make([]MonitoredSystemRecord, 0, len(groups))
	for _, group := range groups {
		records = append(records, monitoredSystemRecord(group))
	}
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
	candidateKeys := monitoredSystemCandidateKeys(candidate)
	if len(candidateKeys) == 0 {
		return false
	}

	for _, group := range monitoredSystemGroups(rs) {
		for key := range candidateKeys {
			if _, ok := group.keys[key]; ok {
				return true
			}
		}
	}

	return false
}

type monitoredSystemGroup struct {
	keys      map[string]struct{}
	resources []*Resource
}

func monitoredSystemGroups(rs ReadState) []monitoredSystemGroup {
	roots := monitoredSystemRoots(rs)
	groups := make([]monitoredSystemGroup, 0, len(roots))

	for _, resource := range roots {
		keys := monitoredSystemResourceKeys(resource)
		matched := -1
		for i := range groups {
			if monitoredSystemKeySetsOverlap(groups[i].keys, keys) {
				matched = i
				break
			}
		}

		if matched == -1 {
			groups = append(groups, monitoredSystemGroup{
				keys:      cloneStringSet(keys),
				resources: []*Resource{resource},
			})
			continue
		}

		for key := range keys {
			groups[matched].keys[key] = struct{}{}
		}
		groups[matched].resources = append(groups[matched].resources, resource)

		// Collapse any later groups that now overlap transitively.
		for i := matched + 1; i < len(groups); {
			if !monitoredSystemKeySetsOverlap(groups[matched].keys, groups[i].keys) {
				i++
				continue
			}
			for key := range groups[i].keys {
				groups[matched].keys[key] = struct{}{}
			}
			groups[matched].resources = append(groups[matched].resources, groups[i].resources...)
			groups = append(groups[:i], groups[i+1:]...)
		}
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
	if resource.Canonical != nil && strings.TrimSpace(resource.Canonical.DisplayName) != "" {
		return strings.TrimSpace(resource.Canonical.DisplayName)
	}
	if strings.TrimSpace(resource.Name) != "" {
		return strings.TrimSpace(resource.Name)
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

func monitoredSystemResourceKeys(resource *Resource) map[string]struct{} {
	keys := make(map[string]struct{})
	if resource == nil {
		return keys
	}

	addMonitoredSystemCanonicalKeys(keys, resource.Canonical)

	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeAgent:
		addMonitoredSystemTokens(keys, "host", resource.Name)
		addMonitoredSystemTokens(keys, "host", resource.Identity.Hostnames...)
		addMonitoredSystemTokens(keys, "ip", resource.Identity.IPAddresses...)
		addMonitoredSystemTokens(keys, "machine", resource.Identity.MachineID)
		addMonitoredSystemTokens(keys, "resource", resource.ID)
		if resource.Agent != nil {
			addMonitoredSystemTokens(keys, "host", resource.Agent.Hostname)
			addMonitoredSystemTokens(keys, "machine", resource.Agent.MachineID)
			addMonitoredSystemTokens(keys, "agent", resource.Agent.AgentID)
		}
		if resource.Proxmox != nil {
			addMonitoredSystemTokens(keys, "host", resource.Proxmox.NodeName)
			addMonitoredSystemTokens(keys, "proxmox", resource.Proxmox.SourceID)
			addMonitoredSystemHostURL(keys, resource.Proxmox.HostURL)
		}
		if resource.Docker != nil {
			addMonitoredSystemTokens(keys, "host", resource.Docker.Hostname)
			addMonitoredSystemTokens(keys, "docker", resource.Docker.HostSourceID)
		}
		if resource.TrueNAS != nil {
			addMonitoredSystemTokens(keys, "host", resource.TrueNAS.Hostname)
		}
	case ResourceTypePBS:
		if resource.PBS != nil {
			addMonitoredSystemTokens(keys, "pbs", resource.PBS.InstanceID)
			addMonitoredSystemTokens(keys, "host", resource.PBS.Hostname)
			addMonitoredSystemHostURL(keys, resource.PBS.HostURL)
		}
	case ResourceTypePMG:
		if resource.PMG != nil {
			addMonitoredSystemTokens(keys, "pmg", resource.PMG.InstanceID)
			addMonitoredSystemTokens(keys, "host", resource.PMG.Hostname)
		}
	case ResourceTypeK8sCluster:
		if resource.Kubernetes != nil {
			addMonitoredSystemTokens(keys, "k8s", resource.Kubernetes.ClusterID)
			addMonitoredSystemTokens(keys, "agent", resource.Kubernetes.AgentID)
			addMonitoredSystemHostURL(keys, resource.Kubernetes.Server)
		}
	}

	if len(keys) == 0 {
		addMonitoredSystemTokens(keys, "fallback", string(resource.Type)+":"+resource.ID)
	}

	return keys
}

func monitoredSystemCandidateKeys(candidate MonitoredSystemCandidate) map[string]struct{} {
	keys := make(map[string]struct{})
	addMonitoredSystemTokens(keys, "resource", candidate.ResourceID)
	addMonitoredSystemTokens(keys, "host", candidate.Name, candidate.Hostname)
	addMonitoredSystemTokens(keys, "machine", candidate.MachineID)
	addMonitoredSystemTokens(keys, "agent", candidate.AgentID)
	addMonitoredSystemHostURL(keys, candidate.HostURL)
	if len(keys) == 0 {
		addMonitoredSystemTokens(keys, "fallback", string(candidate.Type))
	}
	return keys
}

func addMonitoredSystemCanonicalKeys(keys map[string]struct{}, canonical *CanonicalIdentity) {
	if canonical == nil {
		return
	}
	addMonitoredSystemTokens(keys, "canonical-primary", canonical.PrimaryID)
	addMonitoredSystemTokens(keys, "host", canonical.Hostname)
	addMonitoredSystemTokens(keys, "canonical-platform", canonical.PlatformID)
	for _, alias := range canonical.Aliases {
		addMonitoredSystemTokens(keys, "canonical-alias", alias)
	}
}

func addMonitoredSystemHostURL(keys map[string]struct{}, raw string) {
	host := extractHostname(raw)
	if ip := normalizeIPToken(host); ip != "" {
		keys["ip:"+ip] = struct{}{}
		return
	}
	addMonitoredSystemTokens(keys, "host", host)
}

func addMonitoredSystemTokens(keys map[string]struct{}, namespace string, values ...string) {
	for _, value := range values {
		if normalized := normalizeMonitoredSystemToken(value); normalized != "" {
			keys[namespace+":"+normalized] = struct{}{}
		}
	}
}

func normalizeMonitoredSystemToken(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if ip := normalizeIPToken(trimmed); ip != "" {
		return ip
	}
	return trimmed
}

func normalizeIPToken(value string) string {
	if parsed := net.ParseIP(strings.TrimSpace(value)); parsed != nil {
		return parsed.String()
	}
	return ""
}

func monitoredSystemKeySetsOverlap(left, right map[string]struct{}) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	if len(left) > len(right) {
		left, right = right, left
	}
	for key := range left {
		if _, ok := right[key]; ok {
			return true
		}
	}
	return false
}

func cloneStringSet(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
