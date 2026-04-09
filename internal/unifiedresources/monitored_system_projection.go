package unifiedresources

import "strings"

// MonitoredSystemProjection describes how a prospective change affects the
// canonical monitored-system count after shared top-level deduplication.
type MonitoredSystemProjection struct {
	CurrentCount    int
	ProjectedCount  int
	AdditionalCount int
}

// MonitoredSystemProjectionPreview describes how a prospective change affects
// canonical monitored-system counting and which current/projected systems are
// involved in that change.
type MonitoredSystemProjectionPreview struct {
	CurrentCount     int
	ProjectedCount   int
	AdditionalCount  int
	CurrentSystems   []MonitoredSystemRecord
	ProjectedSystems []MonitoredSystemRecord
	CurrentSystem    *MonitoredSystemRecord
	ProjectedSystem  *MonitoredSystemRecord
}

// MonitoredSystemReplacementSelector identifies one source-owned top-level
// monitored-system surface using canonical source-specific fields.
type MonitoredSystemReplacementSelector struct {
	Name       string
	Hostname   string
	HostURL    string
	AgentID    string
	MachineID  string
	ResourceID string
}

// MonitoredSystemReplacement describes one existing monitored-system surface to
// replace while preserving any other source ownership already attached to the
// same top-level resource.
type MonitoredSystemReplacement struct {
	Source   DataSource
	Selector MonitoredSystemReplacementSelector
	Matches  func(Resource) bool
}

// MatchesResource reports whether the replacement targets the provided
// source-owned top-level resource.
func (r MonitoredSystemReplacement) MatchesResource(resource Resource) bool {
	if r.Matches != nil {
		return r.Matches(resource)
	}
	return monitoredSystemReplacementSelectorMatches(r.Source, r.Selector, resource)
}

// ProjectMonitoredSystemCandidate projects one prospective monitored-system
// candidate through the canonical top-level resolver.
func ProjectMonitoredSystemCandidate(
	rs ReadState,
	candidate MonitoredSystemCandidate,
) MonitoredSystemProjection {
	resource := monitoredSystemCandidateResource(candidate)
	if resource == nil {
		return projectMonitoredSystemResources(rs, nil, nil, nil)
	}
	return projectMonitoredSystemResources(rs, []Resource{*resource}, nil, nil)
}

// PreviewMonitoredSystemCandidate projects one prospective monitored-system
// candidate through the canonical top-level resolver and returns the affected
// current/projected monitored-system records.
func PreviewMonitoredSystemCandidate(
	rs ReadState,
	candidate MonitoredSystemCandidate,
) MonitoredSystemProjectionPreview {
	resource := monitoredSystemCandidateResource(candidate)
	if resource == nil {
		return previewMonitoredSystemResources(rs, nil, nil, nil)
	}
	return previewMonitoredSystemResources(rs, []Resource{*resource}, nil, nil)
}

// ProjectMonitoredSystemCandidateReplacement projects a prospective monitored-system
// candidate while replacing one existing source-owned surface in the canonical
// top-level resolver.
func ProjectMonitoredSystemCandidateReplacement(
	rs ReadState,
	replacement MonitoredSystemReplacement,
	candidate MonitoredSystemCandidate,
) MonitoredSystemProjection {
	resource := monitoredSystemCandidateResource(candidate)
	if resource == nil {
		return projectMonitoredSystemResources(rs, nil, nil, &replacement)
	}
	return projectMonitoredSystemResources(rs, []Resource{*resource}, nil, &replacement)
}

// PreviewMonitoredSystemCandidateReplacement projects a prospective monitored-system
// candidate while replacing one existing source-owned surface and returns the
// affected current/projected monitored-system records.
func PreviewMonitoredSystemCandidateReplacement(
	rs ReadState,
	replacement MonitoredSystemReplacement,
	candidate MonitoredSystemCandidate,
) MonitoredSystemProjectionPreview {
	resource := monitoredSystemCandidateResource(candidate)
	if resource == nil {
		return previewMonitoredSystemResources(rs, nil, nil, &replacement)
	}
	return previewMonitoredSystemResources(rs, []Resource{*resource}, nil, &replacement)
}

// PreviewMonitoredSystemRecords projects source-native records through the
// canonical top-level resolver and returns the affected current/projected
// monitored-system records.
func PreviewMonitoredSystemRecords(
	rs ReadState,
	recordsBySource map[DataSource][]IngestRecord,
) MonitoredSystemProjectionPreview {
	return previewMonitoredSystemResources(rs, nil, recordsBySource, nil)
}

// PreviewMonitoredSystemRecordsReplacement projects source-native records while
// replacing one existing source-owned surface and returns the affected
// current/projected monitored-system records.
func PreviewMonitoredSystemRecordsReplacement(
	rs ReadState,
	replacement MonitoredSystemReplacement,
	recordsBySource map[DataSource][]IngestRecord,
) MonitoredSystemProjectionPreview {
	return previewMonitoredSystemResources(rs, nil, recordsBySource, &replacement)
}

// ProjectMonitoredSystemRecords projects source-native records through the
// canonical top-level resolver before they are admitted into runtime state.
func ProjectMonitoredSystemRecords(
	rs ReadState,
	recordsBySource map[DataSource][]IngestRecord,
) MonitoredSystemProjection {
	return projectMonitoredSystemResources(rs, nil, recordsBySource, nil)
}

// ProjectMonitoredSystemRecordsReplacement projects source-native records while
// replacing one existing source-owned surface in the canonical top-level
// resolver.
func ProjectMonitoredSystemRecordsReplacement(
	rs ReadState,
	replacement MonitoredSystemReplacement,
	recordsBySource map[DataSource][]IngestRecord,
) MonitoredSystemProjection {
	return projectMonitoredSystemResources(rs, nil, recordsBySource, &replacement)
}

func projectMonitoredSystemResources(
	rs ReadState,
	additionalResources []Resource,
	recordsBySource map[DataSource][]IngestRecord,
	replacement *MonitoredSystemReplacement,
) MonitoredSystemProjection {
	currentRoots := monitoredSystemRootResources(rs)
	currentCount := ResolveTopLevelSystems(currentRoots).Count()

	projectedRoots := monitoredSystemRootsWithReplacement(currentRoots, replacement)
	projectedRoots = append(projectedRoots, additionalResources...)
	projectedRoots = append(projectedRoots, monitoredSystemRootsFromRecords(recordsBySource)...)

	projectedCount := ResolveTopLevelSystems(projectedRoots).Count()
	additionalCount := projectedCount - currentCount
	if additionalCount < 0 {
		additionalCount = 0
	}

	return MonitoredSystemProjection{
		CurrentCount:    currentCount,
		ProjectedCount:  projectedCount,
		AdditionalCount: additionalCount,
	}
}

func previewMonitoredSystemResources(
	rs ReadState,
	additionalResources []Resource,
	recordsBySource map[DataSource][]IngestRecord,
	replacement *MonitoredSystemReplacement,
) MonitoredSystemProjectionPreview {
	currentRoots := monitoredSystemRootResources(rs)
	currentResolver := ResolveTopLevelSystems(currentRoots)

	recordRoots := monitoredSystemRootsFromRecords(recordsBySource)
	projectedRoots := monitoredSystemRootsWithReplacement(currentRoots, replacement)
	projectedRoots = append(projectedRoots, additionalResources...)
	projectedRoots = append(projectedRoots, recordRoots...)
	projectedResolver := ResolveTopLevelSystems(projectedRoots)
	projectedResourceIDs := monitoredSystemResourceIDs(additionalResources, recordRoots)

	currentCount := currentResolver.Count()
	projectedCount := projectedResolver.Count()
	additionalCount := projectedCount - currentCount
	if additionalCount < 0 {
		additionalCount = 0
	}

	currentSystems := previewCurrentMonitoredSystems(
		currentResolver,
		currentRoots,
		replacement,
		projectedResolver,
		projectedResourceIDs,
	)
	projectedSystems := previewProjectedMonitoredSystems(projectedResolver, projectedResourceIDs)

	return MonitoredSystemProjectionPreview{
		CurrentCount:     currentCount,
		ProjectedCount:   projectedCount,
		AdditionalCount:  additionalCount,
		CurrentSystems:   currentSystems,
		ProjectedSystems: projectedSystems,
		CurrentSystem:    firstMonitoredSystemRecord(currentSystems),
		ProjectedSystem:  firstMonitoredSystemRecord(projectedSystems),
	}
}

func monitoredSystemRootsWithReplacement(
	currentRoots []Resource,
	replacement *MonitoredSystemReplacement,
) []Resource {
	if replacement == nil {
		return append([]Resource(nil), currentRoots...)
	}

	projected := make([]Resource, 0, len(currentRoots))
	for _, root := range currentRoots {
		if !replacement.MatchesResource(root) {
			projected = append(projected, root)
			continue
		}

		stripped, ok := stripMonitoredSystemSource(root, replacement.Source)
		if ok {
			projected = append(projected, stripped)
		}
	}
	return projected
}

func previewCurrentMonitoredSystems(
	currentResolver TopLevelSystemResolver,
	currentRoots []Resource,
	replacement *MonitoredSystemReplacement,
	projectedResolver TopLevelSystemResolver,
	projectedResourceIDs []string,
) []MonitoredSystemRecord {
	if replacement != nil {
		groupIDs := make(map[string]struct{})
		for _, root := range currentRoots {
			if !replacement.MatchesResource(root) {
				continue
			}
			if groupID := currentResolver.GroupIDForResource(root); strings.TrimSpace(groupID) != "" {
				groupIDs[groupID] = struct{}{}
			}
		}
		return monitoredSystemRecordsForGroupIDs(currentResolver, groupIDs)
	}

	if len(projectedResourceIDs) == 0 {
		return []MonitoredSystemRecord{}
	}

	currentGroupIDs := make(map[string]struct{})
	excludedProjectedIDs := make(map[string]struct{}, len(projectedResourceIDs))
	for _, resourceID := range projectedResourceIDs {
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" {
			continue
		}
		if group := currentResolver.groupForResourceID(resourceID); group != nil {
			currentGroupIDs[group.id] = struct{}{}
			continue
		}
		excludedProjectedIDs[resourceID] = struct{}{}
	}

	seenProjectedGroups := make(map[string]struct{})
	for _, projectedResourceID := range projectedResourceIDs {
		projectedGroup := projectedResolver.groupForResourceID(projectedResourceID)
		if projectedGroup == nil {
			continue
		}
		if _, seen := seenProjectedGroups[projectedGroup.id]; seen {
			continue
		}
		seenProjectedGroups[projectedGroup.id] = struct{}{}

		projectedIDs := monitoredSystemGroupResourceIDsExcluding(projectedGroup.resources, excludedProjectedIDs)
		if len(projectedIDs) == 0 {
			continue
		}

		for _, group := range currentResolver.groups {
			if monitoredSystemGroupMatchesResourceIDs(group.resources, projectedIDs) {
				currentGroupIDs[group.id] = struct{}{}
				break
			}
		}
	}

	return monitoredSystemRecordsForGroupIDs(currentResolver, currentGroupIDs)
}

func previewProjectedMonitoredSystems(
	projectedResolver TopLevelSystemResolver,
	projectedResourceIDs []string,
) []MonitoredSystemRecord {
	if len(projectedResourceIDs) == 0 {
		return []MonitoredSystemRecord{}
	}

	groupIDs := make(map[string]struct{})
	for _, projectedResourceID := range projectedResourceIDs {
		if groupID := projectedResolver.GroupIDForResource(Resource{ID: projectedResourceID}); strings.TrimSpace(groupID) != "" {
			groupIDs[groupID] = struct{}{}
		}
	}
	return monitoredSystemRecordsForGroupIDs(projectedResolver, groupIDs)
}

func monitoredSystemRecordForGroupID(resolver TopLevelSystemResolver, groupID string) *MonitoredSystemRecord {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	for _, group := range resolver.groups {
		if group.id != groupID {
			continue
		}
		record := monitoredSystemRecordForResolvedGroup(group)
		return &record
	}
	return nil
}

func monitoredSystemRecordsForGroupIDs(
	resolver TopLevelSystemResolver,
	groupIDs map[string]struct{},
) []MonitoredSystemRecord {
	if len(groupIDs) == 0 {
		return []MonitoredSystemRecord{}
	}

	records := make([]MonitoredSystemRecord, 0, len(groupIDs))
	for _, group := range resolver.groups {
		if _, ok := groupIDs[group.id]; !ok {
			continue
		}
		records = append(records, monitoredSystemRecordForResolvedGroup(group))
	}
	return records
}

func firstMonitoredSystemRecord(records []MonitoredSystemRecord) *MonitoredSystemRecord {
	if len(records) != 1 {
		return nil
	}
	record := records[0]
	return &record
}

func monitoredSystemRecordForResolvedGroup(group topLevelSystemResolvedGroup) MonitoredSystemRecord {
	return monitoredSystemRecord(monitoredSystemGroup{
		keys:        cloneStringSet(group.strongIDs),
		resources:   group.resources,
		explanation: group.explanation,
	})
}

func (r TopLevelSystemResolver) groupForResourceID(resourceID string) *topLevelSystemResolvedGroup {
	groupID := r.resourceToGroup[strings.TrimSpace(resourceID)]
	if groupID == "" {
		return nil
	}
	for i := range r.groups {
		if r.groups[i].id != groupID {
			continue
		}
		return &r.groups[i]
	}
	return nil
}

func monitoredSystemGroupResourceIDs(resources []*Resource, excludeID string) map[string]struct{} {
	excluded := make(map[string]struct{})
	if excludeID = strings.TrimSpace(excludeID); excludeID != "" {
		excluded[excludeID] = struct{}{}
	}
	return monitoredSystemGroupResourceIDsExcluding(resources, excluded)
}

func monitoredSystemGroupResourceIDsExcluding(
	resources []*Resource,
	excluded map[string]struct{},
) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		resourceID := strings.TrimSpace(resource.ID)
		if resourceID == "" {
			continue
		}
		if _, skip := excluded[resourceID]; skip {
			continue
		}
		ids[resourceID] = struct{}{}
	}
	return ids
}

func monitoredSystemGroupMatchesResourceIDs(resources []*Resource, ids map[string]struct{}) bool {
	count := 0
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		resourceID := strings.TrimSpace(resource.ID)
		if resourceID == "" {
			continue
		}
		if _, ok := ids[resourceID]; !ok {
			return false
		}
		count++
	}
	return count == len(ids)
}

func monitoredSystemReplacementSelectorMatches(
	source DataSource,
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	switch source {
	case SourceAgent:
		return monitoredSystemReplacementSelectorMatchesAgent(selector, resource)
	case SourceDocker:
		return monitoredSystemReplacementSelectorMatchesDocker(selector, resource)
	case SourceProxmox:
		return monitoredSystemReplacementSelectorMatchesProxmox(selector, resource)
	case SourceTrueNAS:
		return monitoredSystemReplacementSelectorMatchesTrueNAS(selector, resource)
	case SourcePBS:
		return monitoredSystemReplacementSelectorMatchesPBS(selector, resource)
	case SourcePMG:
		return monitoredSystemReplacementSelectorMatchesPMG(selector, resource)
	case SourceK8s:
		return monitoredSystemReplacementSelectorMatchesK8s(selector, resource)
	case SourceVMware:
		return monitoredSystemReplacementSelectorMatchesVMware(selector, resource)
	default:
		return false
	}
}

func monitoredSystemReplacementSelectorMatchesAgent(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.Agent == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.AgentID, resource.Agent.AgentID) ||
		trimmedEqualFold(selector.MachineID, resource.Identity.MachineID) ||
		monitoredSystemSelectorMatchesHost(selector, resource.Agent.Hostname, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesDocker(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.Docker == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.Docker.HostSourceID) ||
		trimmedEqualFold(selector.AgentID, resource.Docker.AgentID) ||
		trimmedEqualFold(selector.MachineID, resource.Docker.MachineID) ||
		monitoredSystemSelectorMatchesHost(selector, resource.Docker.Hostname, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesProxmox(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.Proxmox == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.Proxmox.SourceID) ||
		trimmedEqualFold(selector.Name, resource.Proxmox.Instance) ||
		trimmedEqualFold(selector.Name, resource.Proxmox.NodeName) ||
		trimmedEqualFold(selector.HostURL, resource.Proxmox.HostURL) ||
		monitoredSystemSelectorMatchesHost(selector, resource.Proxmox.HostURL, resource.Proxmox.NodeName, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesTrueNAS(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.TrueNAS == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.MachineID, resource.Identity.MachineID) ||
		monitoredSystemSelectorMatchesHost(selector, resource.TrueNAS.Hostname, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesPBS(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.PBS == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.PBS.InstanceID) ||
		trimmedEqualFold(selector.HostURL, resource.PBS.HostURL) ||
		monitoredSystemSelectorMatchesHost(selector, resource.PBS.Hostname, resource.PBS.HostURL, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesPMG(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.PMG == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.PMG.InstanceID) ||
		monitoredSystemSelectorMatchesHost(selector, resource.PMG.Hostname, resource.Name)
}

func monitoredSystemReplacementSelectorMatchesK8s(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.Kubernetes == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.Kubernetes.ClusterID) ||
		trimmedEqualFold(selector.Name, resource.Kubernetes.ClusterName) ||
		trimmedEqualFold(selector.Name, resource.Kubernetes.SourceName) ||
		trimmedEqualFold(selector.HostURL, resource.Kubernetes.Server) ||
		trimmedEqualFold(selector.AgentID, resource.Kubernetes.AgentID)
}

func monitoredSystemReplacementSelectorMatchesVMware(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if resource.VMware == nil {
		return false
	}
	return monitoredSystemSelectorMatchesCanonicalResourceID(selector, resource) ||
		trimmedEqualFold(selector.ResourceID, resource.VMware.ConnectionID) ||
		trimmedEqualFold(selector.MachineID, resource.VMware.HostUUID) ||
		trimmedEqualFold(selector.MachineID, resource.Identity.DMIUUID) ||
		trimmedEqualFold(selector.Name, resource.Name) ||
		monitoredSystemSelectorMatchesHost(selector, resource.VMware.RuntimeHostName, resource.VMware.VCenterHost, resource.Name)
}

func monitoredSystemSelectorMatchesCanonicalResourceID(
	selector MonitoredSystemReplacementSelector,
	resource Resource,
) bool {
	if trimmedEqualFold(selector.ResourceID, resource.ID) {
		return true
	}
	return false
}

func monitoredSystemSelectorMatchesHost(
	selector MonitoredSystemReplacementSelector,
	candidates ...string,
) bool {
	selectorHosts := make(map[string]struct{})
	for _, value := range []string{selector.Hostname, extractHostname(selector.HostURL)} {
		if normalized := topLevelSystemNormalizeHost(value); normalized != "" {
			selectorHosts[normalized] = struct{}{}
		}
	}
	if len(selectorHosts) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if normalized := topLevelSystemNormalizeHost(candidate); normalized != "" {
			if _, ok := selectorHosts[normalized]; ok {
				return true
			}
		}
		if normalized := topLevelSystemNormalizeHost(extractHostname(candidate)); normalized != "" {
			if _, ok := selectorHosts[normalized]; ok {
				return true
			}
		}
	}
	return false
}

func trimmedEqualFold(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func stripMonitoredSystemSource(resource Resource, source DataSource) (Resource, bool) {
	stripped := cloneResource(&resource)
	stripped.Sources = removeMonitoredSystemSource(stripped.Sources, source)

	switch source {
	case SourceAgent:
		stripped.Agent = nil
	case SourceDocker:
		stripped.Docker = nil
	case SourceProxmox:
		stripped.Proxmox = nil
	case SourceTrueNAS:
		stripped.TrueNAS = nil
	case SourcePBS:
		stripped.PBS = nil
	case SourcePMG:
		stripped.PMG = nil
	case SourceK8s:
		stripped.Kubernetes = nil
	case SourceVMware:
		stripped.VMware = nil
	}

	if !isMonitoredSystemRootResource(stripped) {
		return Resource{}, false
	}
	return stripped, true
}

func isMonitoredSystemRootResource(resource Resource) bool {
	switch {
	case resource.Agent != nil:
		return true
	case resource.Docker != nil:
		return true
	case resource.Proxmox != nil:
		return true
	case resource.TrueNAS != nil:
		return true
	case resource.PBS != nil:
		return true
	case resource.PMG != nil:
		return true
	case resource.Kubernetes != nil:
		return true
	case resource.VMware != nil:
		return true
	default:
		return false
	}
}

func removeMonitoredSystemSource(sources []DataSource, target DataSource) []DataSource {
	if len(sources) == 0 {
		return nil
	}
	filtered := make([]DataSource, 0, len(sources))
	for _, source := range sources {
		if source == target {
			continue
		}
		filtered = append(filtered, source)
	}
	return filtered
}

func monitoredSystemRootResources(rs ReadState) []Resource {
	roots := monitoredSystemRoots(rs)
	resources := make([]Resource, 0, len(roots))
	for _, resource := range roots {
		if resource == nil {
			continue
		}
		resources = append(resources, cloneResource(resource))
	}
	return resources
}

func monitoredSystemRootsFromRecords(recordsBySource map[DataSource][]IngestRecord) []Resource {
	if len(recordsBySource) == 0 {
		return nil
	}

	registry := NewRegistry(nil)
	for source, records := range recordsBySource {
		if strings.TrimSpace(string(source)) == "" || len(records) == 0 {
			continue
		}
		registry.IngestRecords(source, records)
	}
	return monitoredSystemRootResources(registry)
}

func monitoredSystemResourceIDs(resourceSets ...[]Resource) []string {
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, resources := range resourceSets {
		for _, resource := range resources {
			resourceID := strings.TrimSpace(resource.ID)
			if resourceID == "" {
				continue
			}
			if _, ok := seen[resourceID]; ok {
				continue
			}
			seen[resourceID] = struct{}{}
			ids = append(ids, resourceID)
		}
	}
	return ids
}

func monitoredSystemCandidateResource(candidate MonitoredSystemCandidate) *Resource {
	if !candidate.CountsTowardMonitoredSystems() {
		return nil
	}

	source := normalizeMonitoredSystemCandidateSource(candidate)

	host := firstTrimmed(candidate.Hostname, extractHostname(candidate.HostURL))
	name := firstTrimmed(candidate.Name, host, candidate.ResourceID, candidate.AgentID)
	if name == "" {
		return nil
	}

	resourceID := monitoredSystemCandidateResourceID(candidate, source, host, name)
	if resourceID == "" {
		return nil
	}

	resource := &Resource{
		ID:     resourceID,
		Name:   name,
		Status: StatusOnline,
	}

	identity := ResourceIdentity{
		MachineID: strings.TrimSpace(candidate.MachineID),
		Hostnames: uniqueTrimmed(host, extractHostname(candidate.HostURL)),
	}

	switch source {
	case SourceDocker:
		resource.Type = ResourceTypeAgent
		resource.Docker = &DockerData{
			HostSourceID: strings.TrimSpace(candidate.ResourceID),
			AgentID:      strings.TrimSpace(candidate.AgentID),
			Hostname:     host,
			MachineID:    strings.TrimSpace(candidate.MachineID),
		}
	case SourceProxmox:
		resource.Type = ResourceTypeAgent
		resource.Proxmox = &ProxmoxData{
			SourceID: strings.TrimSpace(candidate.ResourceID),
			NodeName: name,
			HostURL:  strings.TrimSpace(candidate.HostURL),
		}
	case SourceTrueNAS:
		resource.Type = ResourceTypeAgent
		resource.TrueNAS = &TrueNASData{
			Hostname: host,
		}
	case SourcePBS:
		resource.Type = ResourceTypePBS
		resource.PBS = &PBSData{
			InstanceID: strings.TrimSpace(candidate.ResourceID),
			Hostname:   host,
			HostURL:    strings.TrimSpace(candidate.HostURL),
		}
	case SourcePMG:
		resource.Type = ResourceTypePMG
		resource.PMG = &PMGData{
			InstanceID: strings.TrimSpace(candidate.ResourceID),
			Hostname:   host,
		}
	case SourceK8s:
		clusterName := firstTrimmed(candidate.Name, host, candidate.ResourceID)
		resource.Type = ResourceTypeK8sCluster
		resource.Name = clusterName
		resource.Kubernetes = &K8sData{
			ClusterID:   firstTrimmed(candidate.ResourceID, clusterName),
			ClusterName: clusterName,
			AgentID:     strings.TrimSpace(candidate.AgentID),
			Server:      strings.TrimSpace(candidate.HostURL),
		}
		identity.Hostnames = uniqueTrimmed(clusterName, host, extractHostname(candidate.HostURL))
	default:
		resource.Type = ResourceTypeAgent
		resource.Agent = &AgentData{
			AgentID:   strings.TrimSpace(candidate.AgentID),
			Hostname:  host,
			MachineID: strings.TrimSpace(candidate.MachineID),
			ReportIP:  strings.TrimSpace(candidate.HostURL),
		}
	}

	resource.Identity = identity
	return resource
}

func normalizeMonitoredSystemCandidateSource(candidate MonitoredSystemCandidate) DataSource {
	if candidate.Source != "" {
		return candidate.Source
	}

	switch CanonicalResourceType(candidate.Type) {
	case ResourceTypePBS:
		return SourcePBS
	case ResourceTypePMG:
		return SourcePMG
	case ResourceTypeK8sCluster:
		return SourceK8s
	default:
		return SourceAgent
	}
}

func monitoredSystemCandidateResourceID(
	candidate MonitoredSystemCandidate,
	source DataSource,
	host string,
	name string,
) string {
	if sourceID := strings.TrimSpace(candidate.ResourceID); sourceID != "" {
		switch source {
		case SourcePBS:
			return SourceSpecificID(ResourceTypePBS, source, sourceID)
		case SourcePMG:
			return SourceSpecificID(ResourceTypePMG, source, sourceID)
		case SourceK8s:
			return SourceSpecificID(ResourceTypeK8sCluster, source, sourceID)
		default:
			return CanonicalResourceID(sourceID)
		}
	}

	if machineID := strings.TrimSpace(candidate.MachineID); machineID != "" {
		return CanonicalResourceID("candidate:" + string(source) + ":" + machineID)
	}
	if agentID := strings.TrimSpace(candidate.AgentID); agentID != "" {
		return CanonicalResourceID("candidate:" + string(source) + ":" + agentID)
	}
	if host != "" {
		return CanonicalResourceID("candidate:" + string(source) + ":" + strings.ToLower(host))
	}
	return CanonicalResourceID("candidate:" + string(source) + ":" + strings.ToLower(strings.TrimSpace(name)))
}
