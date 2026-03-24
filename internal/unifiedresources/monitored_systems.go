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

// MonitoredSystemGroupingExplanation explains why Pulse counted one or more
// top-level collection paths as a single monitored system.
type MonitoredSystemGroupingExplanation struct {
	Summary  string
	Reasons  []MonitoredSystemGroupingReason
	Surfaces []MonitoredSystemGroupingSurface
}

// MonitoredSystemGroupingReason captures one canonical signal that contributed
// to a top-level monitored-system grouping decision.
type MonitoredSystemGroupingReason struct {
	Kind    string
	Signal  string
	Summary string
}

// MonitoredSystemGroupingSurface describes one counted top-level collection
// path included in a monitored-system group.
type MonitoredSystemGroupingSurface struct {
	Name   string
	Type   string
	Source string
}

// MonitoredSystemStatusExplanation explains why Pulse chose the canonical
// monitored-system runtime status.
type MonitoredSystemStatusExplanation struct {
	Summary string
	Reasons []MonitoredSystemStatusReason
}

// MonitoredSystemStatusReason captures one canonical degraded-status signal
// that contributed to the monitored-system runtime status.
type MonitoredSystemStatusReason struct {
	Kind       string
	Name       string
	Type       string
	Source     string
	Status     string
	ReportedAt time.Time
	Summary    string
}

// MonitoredSystemLatestSignal captures the freshest included grouped signal
// that contributed to the monitored-system runtime view.
type MonitoredSystemLatestSignal struct {
	Name   string
	Type   string
	Source string
	At     time.Time
}

// MonitoredSystemRecord describes a counted top-level monitored system after
// canonical cross-view deduplication.
type MonitoredSystemRecord struct {
	Name                 string
	Type                 string
	Status               ResourceStatus
	StatusExplanation    MonitoredSystemStatusExplanation
	LastSeen             time.Time
	LatestIncludedSignal MonitoredSystemLatestSignal
	Source               string
	Explanation          MonitoredSystemGroupingExplanation
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
	keys        map[string]struct{}
	resources   []*Resource
	explanation MonitoredSystemGroupingExplanation
}

func monitoredSystemGroups(rs ReadState) []monitoredSystemGroup {
	resolver := resolveMonitoredSystemTopLevelSystems(rs)
	groups := make([]monitoredSystemGroup, 0, len(resolver.groups))
	for _, group := range resolver.groups {
		groups = append(groups, monitoredSystemGroup{
			keys:        cloneStringSet(group.strongIDs),
			resources:   group.resources,
			explanation: group.explanation,
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
	status := monitoredSystemStatus(group.resources)
	latestSignal := monitoredSystemLatestObservation(group.resources)
	record := MonitoredSystemRecord{
		Name:                 monitoredSystemDisplayName(group.resources, resource),
		Type:                 monitoredSystemType(resource),
		Status:               status,
		StatusExplanation:    monitoredSystemStatusExplanation(group.resources, status),
		LastSeen:             latestSignal.LastSeen,
		LatestIncludedSignal: monitoredSystemLatestSignal(latestSignal),
		Source:               monitoredSystemSource(group.resources),
		Explanation:          normalizeMonitoredSystemGroupingExplanation(group.explanation),
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
	record.StatusExplanation = normalizeMonitoredSystemStatusExplanation(record.StatusExplanation)
	if record.StatusExplanation.Summary == "" {
		record.StatusExplanation.Summary = monitoredSystemStatusSummary(group.resources, record.Status, record.StatusExplanation.Reasons)
	}
	if record.Source == "" {
		record.Source = "unknown"
	}
	record.LatestIncludedSignal = normalizeMonitoredSystemLatestSignal(record.LatestIncludedSignal)
	if record.Explanation.Summary == "" {
		record.Explanation = monitoredSystemStandaloneExplanation(group.resources)
	}
	return record
}

func normalizeMonitoredSystemGroupingExplanation(
	explanation MonitoredSystemGroupingExplanation,
) MonitoredSystemGroupingExplanation {
	if explanation.Reasons == nil {
		explanation.Reasons = []MonitoredSystemGroupingReason{}
	}
	if explanation.Surfaces == nil {
		explanation.Surfaces = []MonitoredSystemGroupingSurface{}
	}
	return explanation
}

func normalizeMonitoredSystemStatusExplanation(
	explanation MonitoredSystemStatusExplanation,
) MonitoredSystemStatusExplanation {
	if explanation.Reasons == nil {
		explanation.Reasons = []MonitoredSystemStatusReason{}
	}
	return explanation
}

func normalizeMonitoredSystemLatestSignal(signal MonitoredSystemLatestSignal) MonitoredSystemLatestSignal {
	if strings.TrimSpace(signal.Name) == "" {
		signal.Name = "Unnamed source"
	}
	if strings.TrimSpace(signal.Type) == "" {
		signal.Type = "system"
	}
	if strings.TrimSpace(signal.Source) == "" {
		signal.Source = "unknown"
	}
	return signal
}

func monitoredSystemStandaloneExplanation(resources []*Resource) MonitoredSystemGroupingExplanation {
	surfaces := monitoredSystemGroupingSurfaces(resources)
	resource := preferredMonitoredSystemResource(resources)
	source := monitoredSystemPrimarySource(resource)
	if source == "" {
		source = "unknown"
	}

	return normalizeMonitoredSystemGroupingExplanation(MonitoredSystemGroupingExplanation{
		Summary: "Counts as one monitored system because Pulse sees one top-level " +
			monitoredSystemGroupingTypeLabel(monitoredSystemType(resource)) +
			" view from " + monitoredSystemGroupingSourceLabel(source) + ".",
		Reasons: []MonitoredSystemGroupingReason{
			{
				Kind:    "standalone",
				Signal:  "single-top-level-view",
				Summary: "No overlapping top-level source matched this system.",
			},
		},
		Surfaces: surfaces,
	})
}

func monitoredSystemGroupingSurfaces(resources []*Resource) []MonitoredSystemGroupingSurface {
	surfaces := make([]MonitoredSystemGroupingSurface, 0, len(resources))
	for _, resource := range resources {
		if resource == nil {
			continue
		}

		name := monitoredSystemResourceDisplayName(resource)
		if name == "" {
			name = strings.TrimSpace(resource.ID)
		}
		if name == "" {
			name = "Unnamed source"
		}

		resourceType := monitoredSystemType(resource)
		if resourceType == "" {
			resourceType = "system"
		}

		source := monitoredSystemPrimarySource(resource)
		if source == "" {
			source = "unknown"
		}

		surfaces = append(surfaces, MonitoredSystemGroupingSurface{
			Name:   name,
			Type:   resourceType,
			Source: source,
		})
	}

	sort.Slice(surfaces, func(i, j int) bool {
		if surfaces[i].Name != surfaces[j].Name {
			return surfaces[i].Name < surfaces[j].Name
		}
		if surfaces[i].Type != surfaces[j].Type {
			return surfaces[i].Type < surfaces[j].Type
		}
		return surfaces[i].Source < surfaces[j].Source
	})

	if surfaces == nil {
		return []MonitoredSystemGroupingSurface{}
	}
	return surfaces
}

func monitoredSystemGroupingTypeLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "docker-host":
		return "Docker host"
	case "host":
		return "host"
	case "kubernetes-cluster":
		return "Kubernetes cluster"
	case "pbs-server":
		return "PBS server"
	case "pmg-server":
		return "PMG server"
	case "proxmox-node":
		return "Proxmox node"
	case "truenas-system":
		return "TrueNAS system"
	}
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return strings.ReplaceAll(trimmed, "-", " ")
	}
	return "system"
}

func monitoredSystemGroupingSourceLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "":
		return "an unknown source"
	case "multiple":
		return "multiple sources"
	default:
		return strings.TrimSpace(value)
	}
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
	var (
		best     ResourceStatus
		foundAny bool
	)
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		if !foundAny {
			best = resource.Status
			foundAny = true
			continue
		}
		priority := monitoredSystemStatusPriority(resource.Status)
		bestPriority := monitoredSystemStatusPriority(best)
		if priority < bestPriority {
			best = resource.Status
		}
	}
	if !foundAny {
		return StatusUnknown
	}
	return best
}

func monitoredSystemStatusExplanation(
	resources []*Resource,
	status ResourceStatus,
) MonitoredSystemStatusExplanation {
	reasons := monitoredSystemStatusReasons(resources)
	return normalizeMonitoredSystemStatusExplanation(MonitoredSystemStatusExplanation{
		Summary: monitoredSystemStatusSummary(resources, status, reasons),
		Reasons: reasons,
	})
}

func monitoredSystemStatusReasons(resources []*Resource) []MonitoredSystemStatusReason {
	reasons := make([]MonitoredSystemStatusReason, 0)
	for _, resource := range resources {
		reasons = append(reasons, monitoredSystemResourceStatusReasons(resource)...)
	}
	sort.Slice(reasons, func(i, j int) bool {
		if monitoredSystemStatusReasonPriority(reasons[i]) != monitoredSystemStatusReasonPriority(reasons[j]) {
			return monitoredSystemStatusReasonPriority(reasons[i]) < monitoredSystemStatusReasonPriority(reasons[j])
		}
		if reasons[i].Name != reasons[j].Name {
			return reasons[i].Name < reasons[j].Name
		}
		if reasons[i].Type != reasons[j].Type {
			return reasons[i].Type < reasons[j].Type
		}
		if reasons[i].Source != reasons[j].Source {
			return reasons[i].Source < reasons[j].Source
		}
		if !reasons[i].ReportedAt.Equal(reasons[j].ReportedAt) {
			return reasons[i].ReportedAt.Before(reasons[j].ReportedAt)
		}
		return reasons[i].Summary < reasons[j].Summary
	})
	if reasons == nil {
		return []MonitoredSystemStatusReason{}
	}
	return reasons
}

func monitoredSystemResourceStatusReasons(resource *Resource) []MonitoredSystemStatusReason {
	if resource == nil {
		return nil
	}

	name := monitoredSystemResourceDisplayName(resource)
	if name == "" {
		name = "Unnamed source"
	}

	resourceType := monitoredSystemType(resource)
	if resourceType == "" {
		resourceType = "system"
	}

	reasons := make([]MonitoredSystemStatusReason, 0)
	if len(resource.SourceStatus) > 0 {
		sourceKeys := make([]DataSource, 0, len(resource.SourceStatus))
		for source := range resource.SourceStatus {
			sourceKeys = append(sourceKeys, source)
		}
		sort.Slice(sourceKeys, func(i, j int) bool {
			return sourceKeys[i] < sourceKeys[j]
		})

		for _, source := range sourceKeys {
			sourceStatus := resource.SourceStatus[source]
			normalizedStatus := normalizeMonitoredSystemSourceStatus(sourceStatus.Status)
			if normalizedStatus == "online" {
				continue
			}
			reasons = append(reasons, MonitoredSystemStatusReason{
				Kind:       "source-" + normalizedStatus,
				Name:       name,
				Type:       resourceType,
				Source:     string(source),
				Status:     normalizedStatus,
				ReportedAt: sourceStatus.LastSeen,
				Summary:    monitoredSystemSourceStatusReasonSummary(name, source, normalizedStatus, sourceStatus.LastSeen),
			})
		}
	}

	if len(reasons) > 0 {
		return reasons
	}

	normalizedStatus := normalizeMonitoredSystemSourceStatus(string(resource.Status))
	if normalizedStatus == "online" {
		return nil
	}

	source := monitoredSystemPrimarySource(resource)
	if source == "" {
		source = "unknown"
	}
	return []MonitoredSystemStatusReason{
		{
			Kind:       "surface-" + normalizedStatus,
			Name:       name,
			Type:       resourceType,
			Source:     source,
			Status:     normalizedStatus,
			ReportedAt: resource.LastSeen,
			Summary:    monitoredSystemSurfaceStatusReasonSummary(name, resourceType, source, normalizedStatus, resource.LastSeen),
		},
	}
}

func normalizeMonitoredSystemSourceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return "online"
	case "stale", "warning":
		return "stale"
	case "offline":
		return "offline"
	default:
		return "unknown"
	}
}

func monitoredSystemStatusSummary(
	resources []*Resource,
	status ResourceStatus,
	reasons []MonitoredSystemStatusReason,
) string {
	if summary := monitoredSystemMixedStateStatusSummary(resources, status, reasons); summary != "" {
		return summary
	}

	switch status {
	case StatusOnline:
		return "All included top-level collection paths currently report online status."
	case StatusWarning:
		switch {
		case monitoredSystemHasReasonStatus(reasons, "stale"):
			return "At least one included source is stale, so Pulse marks this monitored system as warning."
		case monitoredSystemHasReasonStatus(reasons, "offline"):
			return "At least one included source is offline or disconnected, but the canonical grouped status currently resolves to warning."
		default:
			return "At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning."
		}
	case StatusOffline:
		return "At least one included source is offline or disconnected, so Pulse marks this monitored system as offline."
	default:
		return "Pulse cannot determine a canonical runtime status for this monitored system yet."
	}
}

type monitoredSystemObservation struct {
	Name     string
	Type     string
	Source   string
	LastSeen time.Time
}

func monitoredSystemMixedStateStatusSummary(
	resources []*Resource,
	status ResourceStatus,
	reasons []MonitoredSystemStatusReason,
) string {
	if len(reasons) == 0 || (status != StatusWarning && status != StatusOffline) {
		return ""
	}

	degraded := reasons[0]
	if degraded.ReportedAt.IsZero() {
		return ""
	}

	latest := monitoredSystemLatestOnlineObservation(resources)
	if latest.LastSeen.IsZero() || !latest.LastSeen.After(degraded.ReportedAt) {
		return ""
	}

	return "Pulse most recently heard from " +
		monitoredSystemStatusSourceLabel(latest.Source) + " for " + latest.Name +
		" at " + latest.LastSeen.UTC().Format(time.RFC3339) +
		", but " + monitoredSystemStatusReasonClause(degraded) +
		", so this monitored system is " + string(status) + "."
}

func monitoredSystemHasReasonStatus(reasons []MonitoredSystemStatusReason, status string) bool {
	for _, reason := range reasons {
		if reason.Status == status {
			return true
		}
	}
	return false
}

func monitoredSystemLatestOnlineObservation(resources []*Resource) monitoredSystemObservation {
	return monitoredSystemLatestObservationMatching(resources, func(status string) bool {
		return status == "online"
	})
}

func monitoredSystemLatestObservation(resources []*Resource) monitoredSystemObservation {
	return monitoredSystemLatestObservationMatching(resources, func(status string) bool {
		return status != ""
	})
}

func monitoredSystemLatestObservationMatching(
	resources []*Resource,
	include func(status string) bool,
) monitoredSystemObservation {
	var latest monitoredSystemObservation
	for _, resource := range resources {
		if resource == nil {
			continue
		}

		name := monitoredSystemResourceDisplayName(resource)
		if name == "" {
			name = "Unnamed source"
		}
		resourceType := monitoredSystemType(resource)
		if resourceType == "" {
			resourceType = "system"
		}

		if len(resource.SourceStatus) > 0 {
			sourceKeys := make([]DataSource, 0, len(resource.SourceStatus))
			for source := range resource.SourceStatus {
				sourceKeys = append(sourceKeys, source)
			}
			sort.Slice(sourceKeys, func(i, j int) bool {
				return sourceKeys[i] < sourceKeys[j]
			})
			for _, source := range sourceKeys {
				sourceStatus := resource.SourceStatus[source]
				normalizedStatus := normalizeMonitoredSystemSourceStatus(sourceStatus.Status)
				if !include(normalizedStatus) {
					continue
				}
				if monitoredSystemObservationIsLater(latest.LastSeen, latest.Source, sourceStatus.LastSeen, string(source)) {
					latest = monitoredSystemObservation{
						Name:     name,
						Type:     resourceType,
						Source:   string(source),
						LastSeen: sourceStatus.LastSeen,
					}
				}
			}
		}

		normalizedStatus := normalizeMonitoredSystemSourceStatus(string(resource.Status))
		primarySource := monitoredSystemPrimarySource(resource)
		if include(normalizedStatus) &&
			monitoredSystemObservationIsLater(latest.LastSeen, latest.Source, resource.LastSeen, primarySource) {
			latest = monitoredSystemObservation{
				Name:     name,
				Type:     resourceType,
				Source:   primarySource,
				LastSeen: resource.LastSeen,
			}
		}
	}
	return latest
}

func monitoredSystemLatestSignal(observation monitoredSystemObservation) MonitoredSystemLatestSignal {
	return MonitoredSystemLatestSignal{
		Name:   observation.Name,
		Type:   observation.Type,
		Source: observation.Source,
		At:     observation.LastSeen,
	}
}

func monitoredSystemObservationIsLater(
	currentAt time.Time,
	currentSource string,
	candidateAt time.Time,
	candidateSource string,
) bool {
	if candidateAt.IsZero() {
		return false
	}
	if currentAt.IsZero() {
		return true
	}
	if candidateAt.After(currentAt) {
		return true
	}
	if candidateAt.Equal(currentAt) && strings.TrimSpace(candidateSource) < strings.TrimSpace(currentSource) {
		return true
	}
	return false
}

func monitoredSystemStatusReasonPriority(reason MonitoredSystemStatusReason) int {
	switch reason.Status {
	case "offline":
		return 0
	case "stale":
		return 1
	case "unknown":
		return 2
	default:
		return 3
	}
}

func monitoredSystemStatusReasonClause(reason MonitoredSystemStatusReason) string {
	subject := reason.Name
	if strings.TrimSpace(subject) == "" {
		subject = "this monitored system"
	}

	sourceLabel := monitoredSystemStatusSourceLabel(reason.Source)
	switch reason.Kind {
	case "source-stale":
		return sourceLabel + " data for " + subject + " is stale (last reported " + reason.ReportedAt.UTC().Format(time.RFC3339) + ")"
	case "source-offline":
		return sourceLabel + " data for " + subject + " is offline or disconnected" + monitoredSystemStatusReportedAtSuffix(reason.ReportedAt)
	case "source-unknown":
		return sourceLabel + " data for " + subject + " does not report a canonical status yet" + monitoredSystemStatusReportedAtSuffix(reason.ReportedAt)
	case "surface-stale":
		return monitoredSystemGroupingTypeLabel(reason.Type) + " view for " + subject + " currently reports warning status from " + sourceLabel + monitoredSystemStatusReportedAtSuffix(reason.ReportedAt)
	case "surface-offline":
		return monitoredSystemGroupingTypeLabel(reason.Type) + " view for " + subject + " currently reports offline status from " + sourceLabel + monitoredSystemStatusReportedAtSuffix(reason.ReportedAt)
	case "surface-unknown":
		return monitoredSystemGroupingTypeLabel(reason.Type) + " view for " + subject + " currently reports unknown status from " + sourceLabel + monitoredSystemStatusReportedAtSuffix(reason.ReportedAt)
	default:
		clause := strings.TrimSpace(reason.Summary)
		clause = strings.TrimSuffix(clause, ".")
		return clause
	}
}

func monitoredSystemStatusReportedAtSuffix(reportedAt time.Time) string {
	if reportedAt.IsZero() {
		return ""
	}
	return " (last reported " + reportedAt.UTC().Format(time.RFC3339) + ")"
}

func monitoredSystemStatusPriority(status ResourceStatus) int {
	switch status {
	case StatusOffline:
		return 0
	case StatusWarning:
		return 1
	case StatusUnknown:
		return 2
	case StatusOnline:
		return 3
	default:
		return 4
	}
}

func monitoredSystemSourceStatusReasonSummary(
	name string,
	source DataSource,
	status string,
	lastSeen time.Time,
) string {
	subject := name
	if strings.TrimSpace(subject) == "" {
		subject = "this monitored system"
	}

	summary := monitoredSystemStatusSourceLabel(string(source)) + " data for " + subject
	switch status {
	case "stale":
		summary += " is stale"
	case "offline":
		summary += " is offline or disconnected"
	default:
		summary += " does not report a canonical status yet"
	}

	if !lastSeen.IsZero() {
		summary += " (last reported " + lastSeen.UTC().Format(time.RFC3339) + ")."
		return summary
	}
	return summary + "."
}

func monitoredSystemSurfaceStatusReasonSummary(
	name string,
	resourceType string,
	source string,
	status string,
	lastSeen time.Time,
) string {
	subject := name
	if strings.TrimSpace(subject) == "" {
		subject = "This monitored system"
	}

	summary := monitoredSystemGroupingTypeLabel(resourceType) + " view for " + subject + " currently reports "
	switch status {
	case "stale":
		summary += "warning"
	case "offline":
		summary += "offline"
	default:
		summary += "unknown"
	}
	summary += " status from " + monitoredSystemStatusSourceLabel(source)
	if !lastSeen.IsZero() {
		summary += " (last reported " + lastSeen.UTC().Format(time.RFC3339) + ")."
		return summary
	}
	return summary + "."
}

func monitoredSystemLastSeen(resources []*Resource) time.Time {
	return monitoredSystemLatestObservation(resources).LastSeen
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

func monitoredSystemStatusSourceLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "agent":
		return "Agent"
	case "docker":
		return "Docker"
	case "kubernetes":
		return "Kubernetes"
	case "pbs":
		return "PBS"
	case "pmg":
		return "PMG"
	case "proxmox":
		return "Proxmox"
	case "truenas":
		return "TrueNAS"
	case "", "unknown":
		return "Unknown source"
	default:
		return strings.TrimSpace(value)
	}
}

func cloneStringSet(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
