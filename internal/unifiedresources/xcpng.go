package unifiedresources

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func xcpngVMSourceID(poolUUID, vmUUID string) string {
	return "xcpng:" + strings.TrimSpace(poolUUID) + ":vm:" + strings.TrimSpace(vmUUID)
}

// ingestHostXCPNGVMs coalesces identical pool-wide xe inventories reported by
// several real host agents. The newest sighting wins and each VM is parented
// to the agent whose local XCP-ng host UUID matches resident-on.
func (rr *ResourceRegistry) ingestHostXCPNGVMs(hosts []models.Host) {
	parentByHostUUID := make(map[string]string)
	for _, host := range hosts {
		if host.XCPNG == nil || strings.TrimSpace(host.XCPNG.LocalHostUUID) == "" {
			continue
		}
		if parentID := rr.sourceResourceID(SourceAgent, host.ID); parentID != "" {
			parentByHostUUID[strings.TrimSpace(host.XCPNG.LocalHostUUID)] = parentID
		}
	}

	type candidate struct {
		host models.Host
		vm   models.HostXCPNGVM
	}
	bySourceID := make(map[string]candidate)
	for _, host := range hosts {
		if host.XCPNG == nil {
			continue
		}
		for _, vm := range host.XCPNG.VMs {
			sourceID := xcpngVMSourceID(host.XCPNG.PoolUUID, vm.UUID)
			if strings.TrimSpace(host.XCPNG.PoolUUID) == "" || strings.TrimSpace(vm.UUID) == "" {
				continue
			}
			existing, ok := bySourceID[sourceID]
			if !ok || host.XCPNG.CollectedAt.After(existing.host.XCPNG.CollectedAt) {
				bySourceID[sourceID] = candidate{host: host, vm: vm}
			}
		}
	}
	sourceIDs := make([]string, 0, len(bySourceID))
	for sourceID := range bySourceID {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Strings(sourceIDs)
	for _, sourceID := range sourceIDs {
		item := bySourceID[sourceID]
		resource, identity := resourceFromHostXCPNGVM(item.host, item.vm)
		if parentID := parentByHostUUID[strings.TrimSpace(item.vm.ResidentHostUUID)]; parentID != "" {
			resource.ParentID = &parentID
		}
		rr.ingest(SourceAgent, sourceID, resource, identity)
	}
}

func resourceFromHostXCPNGVM(host models.Host, vm models.HostXCPNGVM) (Resource, ResourceIdentity) {
	lastSeen := host.LastSeen
	if host.XCPNG != nil && !host.XCPNG.CollectedAt.IsZero() {
		lastSeen = host.XCPNG.CollectedAt
	}
	var metrics *ResourceMetrics
	if vm.MemoryStaticMax > 0 {
		used := max(int64(0), min(vm.MemoryActual, vm.MemoryStaticMax))
		total := vm.MemoryStaticMax
		metrics = &ResourceMetrics{Memory: &MetricValue{
			Used:    &used,
			Total:   &total,
			Percent: math.Max(0, math.Min(100, float64(used)/float64(total)*100)),
			Unit:    "bytes",
			Source:  SourceAgent,
		}}
	}
	return Resource{
		Type:       ResourceTypeVM,
		Technology: "xcp-ng",
		Name:       strings.TrimSpace(vm.Name),
		Status:     xcpngResourceStatus(vm.PowerState),
		LastSeen:   lastSeen,
		UpdatedAt:  time.Now().UTC(),
		Metrics:    metrics,
		Tags:       []string{"xcp-ng", "xen"},
		VirtualMachine: &VirtualMachineData{
			RuntimeState: strings.TrimSpace(vm.PowerState),
			Hypervisor:   "xcp-ng",
			VCPUs:        vm.VCPUs,
		},
	}, ResourceIdentity{Hostnames: uniqueStrings([]string{vm.Name})}
}

func xcpngResourceStatus(state string) ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return StatusOnline
	case "halted":
		return StatusOffline
	case "paused", "suspended":
		return StatusWarning
	default:
		return StatusUnknown
	}
}
