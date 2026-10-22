package monitoring

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// Only a node missing/offline in the current provider inventory can need the
// existing offline grace policy. An actually online node with no guests is an
// authoritative empty result, not a gap to fill with deleted workloads.
func (m *Monitor) preserveGuestsForGracePeriod(
	instanceName string,
	resources []proxmox.ClusterResource,
	previous previousGuestContext,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
	allVMs []models.VM,
	allContainers []models.Container,
) ([]models.VM, []models.Container) {
	onlineNodes := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		onlineNodes[node.Node] = strings.EqualFold(strings.TrimSpace(node.Status), "online")
	}
	nodesWithResources := make(map[string]bool)
	observedGuests := make(map[int]bool, len(resources))
	for _, res := range resources {
		nodesWithResources[res.Node] = true
		// VMIDs are cluster-wide. A migration or conversion to a template is
		// positive inventory evidence even when the old node is in grace.
		observedGuests[res.VMID] = true
	}

	retainedNodes := make(map[string]struct{})
	for node, effectiveStatus := range nodeEffectiveStatus {
		if effectiveStatus == "online" && !onlineNodes[node] && !nodesWithResources[node] {
			retainedNodes[node] = struct{}{}
		}
	}
	// Preserve the existing unknown-node-data fallback, without treating an
	// explicit empty inventory from known online nodes as an unknown outage.
	if len(nodes) == 0 && len(nodeEffectiveStatus) == 0 && len(allVMs) == 0 && len(allContainers) == 0 {
		for _, vm := range previous.vms {
			retainedNodes[vm.Node] = struct{}{}
		}
		for _, ct := range previous.containers {
			retainedNodes[ct.Node] = struct{}{}
		}
	}
	if len(retainedNodes) == 0 {
		return allVMs, allContainers
	}

	// Read-state projections omit some counters and may contain linked-agent
	// readings. Retain the complete source-owned tuples and original times,
	// just as the failed per-node enumeration path does.
	vms, containers := m.retainedGuestsForFailedNodes(instanceName, retainedNodes, previous)
	preservedVMs, preservedContainers := 0, 0
	for _, vm := range vms {
		if !observedGuests[vm.VMID] {
			allVMs = append(allVMs, vm)
			preservedVMs++
		}
	}
	for _, ct := range containers {
		if !observedGuests[ct.VMID] {
			allContainers = append(allContainers, ct)
			preservedContainers++
		}
	}
	if preservedVMs > 0 || preservedContainers > 0 {
		log.Info().Str("instance", instanceName).
			Int("preservedVMs", preservedVMs).
			Int("preservedContainers", preservedContainers).
			Msg("Retained guest observations for nodes without current enumeration")
	}
	return allVMs, allContainers
}

// guestObservedInCycle reports whether a guest's telemetry came from this poll
// rather than being carried forward while its node sits in the grace period.
//
// Preserved guests keep the LastSeen of the cycle that actually observed them,
// so anything older than this cycle's start was not observed. Recording one as
// a sample fabricates a reading: the carried-forward projection has no counters,
// so it writes CPU, disk and network zeroes for a guest Pulse cannot currently
// see, and the history shows a collapse to zero instead of a gap.
//
// A zero LastSeen means we have no evidence either way, so this fails open and
// records. Dropping a legitimate sample is the worse error of the two, and
// harder to notice.
func guestObservedInCycle(lastSeen, cycleStart time.Time) bool {
	if lastSeen.IsZero() {
		return true
	}
	return !lastSeen.Before(cycleStart)
}

// guestCPUUsage preserves the provider ratio, including a measured zero, but
// never sanitizes unavailable or non-finite readings into an idle observation.
func guestCPUUsage(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return -1
	}
	return value
}

// recordGuestMetrics records metrics for all running VMs and containers that
// this cycle actually observed.
func (m *Monitor) recordGuestMetrics(allVMs []models.VM, allContainers []models.Container, cycleStart time.Time) {
	now := time.Now()
	for _, vm := range allVMs {
		if !guestObservedInCycle(vm.LastSeen, cycleStart) {
			continue
		}
		if vm.Status == "running" {
			diskRead, diskWrite, networkIn, networkOut := guestHistoryRates(vm.DiskRead, vm.DiskWrite, vm.NetworkIn, vm.NetworkOut, vm.IORateValidity)
			memoryUsage, memoryUsed, diskUsage := historyMemoryUsage(vm.Memory), historyMemoryUsed(vm.Memory), historyVMDiskUsage(vm)
			// Only legacy unannotated memory needs the diagnostic heuristic.
			// An explicit observation belongs to the selected source: deferred
			// filesystem/QGA work cannot disqualify independent current memory.
			if vm.GuestAgentStatus == "deferred" && vm.Memory.Observation.State == "" {
				snapshot := m.previousGuestSnapshot(vm.Instance, "qemu", vm.Node, vm.VMID)
				if snapshot == nil || snapshot.MemorySource == "previous-snapshot" || strings.HasPrefix(CanonicalMemorySource(snapshot.MemorySource), "guest-agent-meminfo") {
					memoryUsage, memoryUsed = -1, -1
				}
			}
			m.recordGuestMetric("vm", vm.ID, unifiedresources.ProxmoxGuestCPUPercent(vm.CPU), memoryUsage, memoryUsed, diskUsage, diskRead, diskWrite, networkIn, networkOut, now)
		}
	}
	for _, ct := range allContainers {
		if !guestObservedInCycle(ct.LastSeen, cycleStart) {
			continue
		}
		if ct.Status == "running" {
			diskRead, diskWrite, networkIn, networkOut := guestHistoryRates(ct.DiskRead, ct.DiskWrite, ct.NetworkIn, ct.NetworkOut, ct.IORateValidity)
			m.recordGuestMetric("container", ct.ID, unifiedresources.ProxmoxGuestCPUPercent(ct.CPU), historyMemoryUsage(ct.Memory), historyMemoryUsed(ct.Memory), ct.Disk.Usage, diskRead, diskWrite, networkIn, networkOut, now)
		}
	}
}

// A failed filesystem read can leave a positive cluster allocation or retained
// usage for display. Neither is a fresh History sample, whichever poll route
// collected it. A complete measured zero remains an ordinary observation.
func historyVMDiskUsage(vm models.VM) float64 {
	if vm.DiskStatusReason != "" {
		return -1
	}
	return vm.Disk.Usage
}

func guestHistoryRates(diskRead, diskWrite, networkIn, networkOut int64, validity models.IORateValidity) (int64, int64, int64, int64) {
	effective := validity.EffectiveForRates(diskRead, diskWrite, networkIn, networkOut)
	if !effective.DiskRead {
		diskRead = -1
	}
	if !effective.DiskWrite {
		diskWrite = -1
	}
	if !effective.NetworkIn {
		networkIn = -1
	}
	if !effective.NetworkOut {
		networkOut = -1
	}
	return diskRead, diskWrite, networkIn, networkOut
}

// Retained numbers remain useful for display, but a newly refreshed guest row
// cannot turn them into a new percentage or byte observation. The producer owns
// freshness; ordinary current TTL-cache hits and legacy readings remain valid.
func historyMemoryUsage(memory models.Memory) float64 {
	if !memory.HasKnownUsage() || (memory.Observation.State != "" && memory.Observation.State != "current") {
		return -1
	}
	return memory.Usage
}

func historyMemoryUsed(memory models.Memory) float64 {
	if !memory.HasKnownUsage() || (memory.Observation.State != "" && memory.Observation.State != "current") {
		return -1
	}
	return float64(memory.Used)
}

// recordGuestMetric records metrics for a single guest (VM or container) to both
// the in-memory metrics history and the persistent metrics store.
func (m *Monitor) recordGuestMetric(
	resourceType, resourceID string,
	cpu, memory, memoryUsed, diskUsage float64,
	diskRead, diskWrite, networkIn, networkOut int64,
	now time.Time,
) {
	if m == nil || shouldSkipNativeMockStateMetricWrites() {
		return
	}

	if m.metricsHistory != nil {
		if cpu >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "cpu", cpu, now)
		}
		if memory >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "memory", memory, now)
		}
		if memoryUsed >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "memoryused", memoryUsed, now)
		}
		if diskUsage >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "disk", diskUsage, now)
		}
		if diskRead >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "diskread", float64(diskRead), now)
		}
		if diskWrite >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "diskwrite", float64(diskWrite), now)
		}
		if networkIn >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "netin", float64(networkIn), now)
		}
		if networkOut >= 0 {
			m.metricsHistory.AddGuestMetric(resourceID, "netout", float64(networkOut), now)
		}
	}

	if m.metricsStore != nil {
		if cpu >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "cpu", cpu, now)
		}
		if memory >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "memory", memory, now)
		}
		if memoryUsed >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "memoryused", memoryUsed, now)
		}
		if diskUsage >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "disk", diskUsage, now)
		}
		if diskRead >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "diskread", float64(diskRead), now)
		}
		if diskWrite >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "diskwrite", float64(diskWrite), now)
		}
		if networkIn >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "netin", float64(networkIn), now)
		}
		if networkOut >= 0 {
			m.metricsStore.Write(resourceType, resourceID, "netout", float64(networkOut), now)
		}
	}
}
