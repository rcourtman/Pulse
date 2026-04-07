package unifiedresources

import "strings"

// MonitoredSystemProjection describes how a prospective change affects the
// canonical monitored-system count after shared top-level deduplication.
type MonitoredSystemProjection struct {
	CurrentCount    int
	ProjectedCount  int
	AdditionalCount int
}

// MonitoredSystemReplacement describes one existing monitored-system surface to
// replace while preserving any other source ownership already attached to the
// same top-level resource.
type MonitoredSystemReplacement struct {
	Source  DataSource
	Matches func(Resource) bool
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

func monitoredSystemRootsWithReplacement(
	currentRoots []Resource,
	replacement *MonitoredSystemReplacement,
) []Resource {
	if replacement == nil || replacement.Matches == nil {
		return append([]Resource(nil), currentRoots...)
	}

	projected := make([]Resource, 0, len(currentRoots))
	for _, root := range currentRoots {
		if !replacement.Matches(root) {
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

func monitoredSystemCandidateResource(candidate MonitoredSystemCandidate) *Resource {
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
