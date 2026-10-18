package api

import (
	"slices"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// reportAlertScope is the set of alert resource IDs a report attributes to
// its subject. It is built from the identities the subject's unified
// resource carries, never from host or node names: an alert belongs to the
// report of each resource whose identity it was raised under. A machine's
// report therefore leaves out the guests, containers, storage pools and
// disks it hosts, whose alerts are raised under their own identities and
// appear on their own reports.
type reportAlertScope struct {
	ids map[string]struct{}
	// childPrefixes match component alerts raised under the subject's own
	// alert identity, such as "agent:<host>/disk:/" for a filesystem the
	// agent reports.
	childPrefixes []string
	// PVE disk alerts are owned by their recorded hardware, across old and
	// current paths. Use the journal's ownership rule on this report snapshot.
	pveDiskID string
}

func (s *reportAlertScope) addID(id string) {
	if id = strings.TrimSpace(id); id != "" {
		s.ids[id] = struct{}{}
	}
}

func (s *reportAlertScope) addChildPrefix(parentID string) {
	if parentID = strings.TrimSpace(parentID); parentID != "" {
		s.childPrefixes = append(s.childPrefixes, parentID+"/")
	}
}

// matchesAlert reports whether an alert belongs to the subject.
func (s reportAlertScope) matchesAlert(alert models.Alert, resources []unifiedresources.Resource) bool {
	ref := strings.TrimSpace(alert.ResourceID)
	if s.pveDiskID != "" {
		identifiers := unifiedresources.ProxmoxPhysicalDiskAlertIdentifiers(ref)
		if len(identifiers) > 0 {
			// A request/metrics alias equal to the path cannot bypass hardware
			// ownership. Only this reference's actual PVE alerts enter the gate.
			return slices.Contains(identifiers, alert.ID) &&
				unifiedresources.ProxmoxPhysicalDiskAlertOwner(ref, alert.Metadata, resources) == s.pveDiskID
		}
	}
	return s.matches(ref)
}

func (s reportAlertScope) matches(resourceID string) bool {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return false
	}
	if _, ok := s.ids[resourceID]; ok {
		return true
	}
	for _, prefix := range s.childPrefixes {
		if strings.HasPrefix(resourceID, prefix) {
			return true
		}
	}
	return false
}

// reportAlertScopeFor returns the alert identities of a report subject:
//   - the requested unified ID and the resource's metrics-target ID, which
//     key provider incidents and the alerts evaluated on unified resources
//     (vSphere hosts, TrueNAS systems, Kubernetes, PBS, PMG);
//   - for a machine, its linked Proxmox node's source ID
//     ("<instance>-<node>"), which keys the node's own alerts, and its
//     Pulse agent's "agent:<host>" alerts with their component children
//     ("agent:<host>/disk:<mount>", disk temperature, SMART, RAID, the Unraid
//     array, custom sensors). A node alert that moved to the agent and the
//     agent's own alert for that metric therefore land in the same report,
//     where pkg/reporting counts them once;
//   - for a Docker runtime, container or Swarm service, the IDs the Docker
//     alert producer builds (alerts.DockerHostResourceID,
//     DockerContainerResourceID, DockerServiceResourceID);
//   - for a storage pool, its source ID and the pool's ZFS components
//     ("<storage>/zfs-pool:<pool>/device:<dev>"), or for an agent-reported
//     Unraid array, the agent alert raised on it;
//   - for a physical disk, PVE health/wearout alerts whose recorded hardware
//     owns the subject in the unified snapshot, even after a path move.
//
// resource may be nil when the request names no live unified resource; the
// scope then holds only the request's own IDs.
func reportAlertScopeFor(req *reporting.MetricReportRequest, resource *unifiedresources.Resource) reportAlertScope {
	scope := reportAlertScope{ids: make(map[string]struct{})}
	if req != nil {
		scope.addID(req.ResourceID)
		scope.addID(req.MetricsResourceID)
	}
	if resource == nil {
		return scope
	}
	scope.addID(resource.ID)
	switch unifiedresources.CanonicalResourceType(resource.Type) {
	case unifiedresources.ResourceTypeAgent:
		if resource.Proxmox != nil {
			scope.addID(resource.Proxmox.SourceID)
		}
		if agentID := reportHostAgentID(*resource); agentID != "" {
			// Alert evaluation names a Pulse agent "agent:<host ID>"
			// (alerts.hostResourceID), the same reference the canonical
			// identity indexes for per-resource policy (#1497).
			scope.addID("agent:" + agentID)
			scope.addChildPrefix("agent:" + agentID)
		}
		if resource.Docker != nil && strings.TrimSpace(resource.Docker.HostSourceID) != "" {
			scope.addID(alerts.DockerHostResourceID(resource.Docker.HostSourceID))
		}
	case unifiedresources.ResourceTypeAppContainer:
		if docker := resource.Docker; docker != nil && strings.TrimSpace(docker.HostSourceID) != "" {
			// A container without an ID is referenced by its name, which
			// the container adapter carries as the resource name.
			scope.addID(alerts.DockerContainerResourceID(docker.HostSourceID, docker.ContainerID, resource.Name))
		}
	case unifiedresources.ResourceTypeDockerService:
		if docker := resource.Docker; docker != nil && strings.TrimSpace(docker.HostSourceID) != "" {
			// The service adapter carries the service name as the resource
			// name; the producer falls back to it when the ID is empty.
			serviceName := docker.ServiceName
			if strings.TrimSpace(serviceName) == "" {
				serviceName = resource.Name
			}
			scope.addID(alerts.DockerServiceResourceID(docker.HostSourceID, docker.ServiceID, serviceName))
		}
	case unifiedresources.ResourceTypeStorage:
		if resource.Proxmox != nil {
			scope.addID(resource.Proxmox.SourceID)
			scope.addChildPrefix(resource.Proxmox.SourceID)
		}
		// A Pulse agent reports an Unraid array as storage keyed
		// "<host>/storage:unraid-array", which is then the metrics target, and
		// raises the array's alert under its own identity as
		// "agent:<host>/storage:unraid-array". That alert belongs to both the
		// array's report and its machine's.
		if req != nil && reportStorageReportedOnlyByAgent(*resource) && strings.TrimSpace(req.MetricsResourceID) != "" {
			scope.addID("agent:" + strings.TrimSpace(req.MetricsResourceID))
		}
	case unifiedresources.ResourceTypePhysicalDisk:
		scope.pveDiskID = resource.ID
	}
	return scope
}

// reportStorageReportedOnlyByAgent reports whether a storage resource comes
// from a Pulse agent alone, so its metrics target is the agent's source ID.
func reportStorageReportedOnlyByAgent(resource unifiedresources.Resource) bool {
	if !slices.Contains(resource.Sources, unifiedresources.SourceAgent) {
		return false
	}
	for _, source := range resource.Sources {
		if source != unifiedresources.SourceAgent {
			return false
		}
	}
	return true
}

// reportHostAgentID is the host ID of the Pulse agent reporting a machine.
// vSphere hosts and TrueNAS systems also fill AgentData, so the agent source
// must be present before the ID names a Pulse agent.
func reportHostAgentID(resource unifiedresources.Resource) string {
	if resource.Agent == nil || !slices.Contains(resource.Sources, unifiedresources.SourceAgent) {
		return ""
	}
	return strings.TrimSpace(resource.Agent.AgentID)
}

// findReportSubjectResource returns the unified resource the request names.
func findReportSubjectResource(req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot) *unifiedresources.Resource {
	if req == nil {
		return nil
	}
	for i := range snapshot.Resources {
		if snapshot.Resources[i].ID == req.ResourceID {
			return &snapshot.Resources[i]
		}
	}
	return nil
}

// attachReportAlerts adds the subject's open alerts and the alerts resolved
// inside the report window, keeping each alert's resolution.
func attachReportAlerts(req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, scope reportAlertScope, start, end time.Time) {
	for _, alert := range snapshot.ActiveAlerts {
		if scope.matchesAlert(alert, snapshot.Resources) {
			req.Alerts = append(req.Alerts, reportActiveAlertInfo(alert))
		}
	}
	for _, resolved := range snapshot.RecentlyResolved {
		if scope.matchesAlert(resolved.Alert, snapshot.Resources) &&
			resolved.ResolvedTime.After(start) && resolved.ResolvedTime.Before(end) {
			req.Alerts = append(req.Alerts, reportResolvedAlertInfo(resolved))
		}
	}
}

// enrichAgentReport fills a report on a machine: every Proxmox node and
// standalone Pulse agent is a unified `agent` resource in v6. Details come
// from the resource's Proxmox and agent payloads, storage pools and physical
// disks from the resources whose unified parent is the machine, and alerts
// from reportAlertScopeFor.
func (h *ReportingHandlers) enrichAgentReport(req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	resource := findReportSubjectResource(req, snapshot)
	if resource != nil {
		req.Resource = reportMachineResourceInfo(*resource)
		for i := range snapshot.Resources {
			child := &snapshot.Resources[i]
			if child.ParentID == nil || *child.ParentID != resource.ID {
				continue
			}
			switch unifiedresources.CanonicalResourceType(child.Type) {
			case unifiedresources.ResourceTypeStorage:
				req.Storage = append(req.Storage, reportStorageInfo(*child))
			case unifiedresources.ResourceTypePhysicalDisk:
				if child.PhysicalDisk != nil {
					req.Disks = append(req.Disks, reportDiskInfo(*child.PhysicalDisk, snapshot.alertManager))
				}
			}
		}
	}
	attachReportAlerts(req, snapshot, reportAlertScopeFor(req, resource), start, end)
}

// reportMachineResourceInfo projects a machine's unified payloads onto the
// report's resource details. The Proxmox API is authoritative for PVE
// facts; the Pulse agent adds the operating system.
func reportMachineResourceInfo(resource unifiedresources.Resource) *reporting.ResourceInfo {
	info := &reporting.ResourceInfo{
		Name:        resource.Name,
		Status:      string(resource.Status),
		Uptime:      resource.Uptime,
		IPAddresses: append([]string(nil), resource.Identity.IPAddresses...),
		Tags:        append([]string(nil), resource.Tags...),
	}
	if resource.Temperature != nil {
		temp := *resource.Temperature
		info.Temperature = &temp
	}
	if metrics := resource.Metrics; metrics != nil {
		if metrics.Memory != nil && metrics.Memory.Total != nil {
			info.MemoryTotal = *metrics.Memory.Total
		}
		if metrics.Disk != nil && metrics.Disk.Total != nil {
			info.DiskTotal = *metrics.Disk.Total
		}
	}
	if px := resource.Proxmox; px != nil {
		info.Host = px.HostURL
		info.Instance = px.Instance
		info.KernelVersion = px.KernelVersion
		info.PVEVersion = px.PVEVersion
		info.ClusterName = px.ClusterName
		info.IsCluster = px.IsClusterMember
		info.LoadAverage = append([]float64(nil), px.LoadAverage...)
		if info.Uptime == 0 {
			info.Uptime = px.Uptime
		}
		if px.CPUInfo != nil {
			info.CPUModel = px.CPUInfo.Model
			info.CPUCores = px.CPUInfo.Cores
			info.CPUSockets = px.CPUInfo.Sockets
		}
	}
	if agent := resource.Agent; agent != nil && reportHostAgentID(resource) != "" {
		info.OSName = agent.OSName
		info.OSVersion = agent.OSVersion
		if info.KernelVersion == "" {
			info.KernelVersion = agent.KernelVersion
		}
		if info.CPUCores == 0 {
			info.CPUCores = agent.CPUCount
		}
		if len(info.LoadAverage) == 0 {
			info.LoadAverage = append([]float64(nil), agent.LoadAverage...)
		}
		if info.Uptime == 0 {
			info.Uptime = agent.UptimeSeconds
		}
		if info.MemoryTotal == 0 && agent.Memory != nil {
			info.MemoryTotal = agent.Memory.Total
		}
	}
	return info
}

// reportStorageInfo is the report row for a storage pool resource.
func reportStorageInfo(r unifiedresources.Resource) reporting.StorageInfo {
	var total, used, available int64
	var usagePerc float64
	if r.Metrics != nil && r.Metrics.Disk != nil {
		if r.Metrics.Disk.Total != nil {
			total = *r.Metrics.Disk.Total
		}
		if r.Metrics.Disk.Used != nil {
			used = *r.Metrics.Disk.Used
		}
		if total > 0 {
			available = total - used
		}
		usagePerc = r.Metrics.Disk.Percent
		if usagePerc == 0 && total > 0 {
			usagePerc = (float64(used) / float64(total)) * 100
		}
	}

	var storageType, content string
	if r.Storage != nil {
		storageType = r.Storage.Type
		content = r.Storage.Content
	}

	return reporting.StorageInfo{
		Name:      r.Name,
		Type:      storageType,
		Status:    string(r.Status),
		Total:     total,
		Used:      used,
		Available: available,
		UsagePerc: usagePerc,
		Content:   content,
	}
}

// reportDiskInfo is the report row for a physical disk resource. Its
// temperature is the collected reading only (reportDiskTemperature), coloured
// by the tenant's disk temperature alert thresholds; a nil manager means the
// factory alert configuration.
func reportDiskInfo(pd unifiedresources.PhysicalDiskMeta, manager *alerts.Manager) reporting.DiskInfo {
	temperatureWarning, temperatureCritical := reportDiskTemperatureThresholds(manager, pd.DiskType)
	return reporting.DiskInfo{
		Device:              pd.DevPath,
		Model:               pd.Model,
		Serial:              pd.Serial,
		Type:                pd.DiskType,
		Size:                pd.SizeBytes,
		Health:              pd.Health,
		Temperature:         reportDiskTemperature(pd),
		WearLevel:           pd.Wearout,
		TemperatureWarning:  temperatureWarning,
		TemperatureCritical: temperatureCritical,
	}
}
