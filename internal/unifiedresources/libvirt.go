package unifiedresources

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func hostLibvirtDomainSourceID(host models.Host, domain models.HostLibvirtDomain) string {
	return strings.TrimSpace(host.ID) + ":libvirt:" + strings.TrimSpace(domain.ID)
}

func (rr *ResourceRegistry) ingestHostLibvirtDomains(host models.Host) {
	if host.Libvirt == nil {
		return
	}
	parentID := rr.sourceResourceID(SourceAgent, host.ID)
	for _, domain := range host.Libvirt.Domains {
		sourceID := hostLibvirtDomainSourceID(host, domain)
		if strings.TrimSpace(host.ID) == "" || strings.TrimSpace(domain.ID) == "" {
			continue
		}
		resource, identity := resourceFromHostLibvirtDomain(host, domain)
		if parentID != "" {
			resource.ParentID = &parentID
		}
		rr.ingest(SourceAgent, sourceID, resource, identity)
	}
}

func resourceFromHostLibvirtDomain(host models.Host, domain models.HostLibvirtDomain) (Resource, ResourceIdentity) {
	metrics := &ResourceMetrics{}
	if domain.CPUUsageValid {
		percent := math.Max(0, math.Min(100, domain.CPUUsagePercent))
		metrics.CPU = &MetricValue{
			Value:   percent,
			Percent: percent,
			Unit:    "percent",
			Source:  SourceAgent,
		}
	}
	if domain.MemoryMaximumBytes > 0 {
		used := max(int64(0), min(domain.MemoryCurrentBytes, domain.MemoryMaximumBytes))
		total := domain.MemoryMaximumBytes
		metrics.Memory = &MetricValue{
			Used:    &used,
			Total:   &total,
			Percent: math.Max(0, math.Min(100, (float64(used)/float64(total))*100)),
			Unit:    "bytes",
			Source:  SourceAgent,
		}
	}
	if domain.IORatesValid {
		metrics.NetIn = &MetricValue{Value: max(0, domain.NetworkInRate), Unit: "bytes/s", Source: SourceAgent}
		metrics.NetOut = &MetricValue{Value: max(0, domain.NetworkOutRate), Unit: "bytes/s", Source: SourceAgent}
		metrics.DiskRead = &MetricValue{Value: max(0, domain.DiskReadRate), Unit: "bytes/s", Source: SourceAgent}
		metrics.DiskWrite = &MetricValue{Value: max(0, domain.DiskWriteRate), Unit: "bytes/s", Source: SourceAgent}
	}

	lastSeen := host.LastSeen
	if host.Libvirt != nil && !host.Libvirt.CollectedAt.IsZero() {
		lastSeen = host.Libvirt.CollectedAt
	}
	resource := Resource{
		Type:       ResourceTypeVM,
		Technology: "libvirt",
		Name:       strings.TrimSpace(domain.Name),
		Status:     libvirtResourceStatus(domain.State),
		LastSeen:   lastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Tags:       []string{"libvirt", "kvm"},
		VirtualMachine: &VirtualMachineData{
			RuntimeState: strings.TrimSpace(domain.State),
			Hypervisor:   "libvirt",
			VCPUs:        domain.VCPUs,
		},
	}
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{domain.Name}),
	}
	return resource, identity
}

func libvirtResourceStatus(state string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running", "blocked":
		return StatusOnline
	case "shutoff":
		return StatusOffline
	case "paused", "shutting-down", "crashed", "suspended":
		return StatusWarning
	default:
		return StatusUnknown
	}
}
