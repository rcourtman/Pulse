package monitoring

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// pollVMsAndContainersEfficient uses the cluster/resources endpoint to get all VMs and containers in one call
// This works on both clustered and standalone nodes for efficient polling
// When the instance is part of a cluster, the cluster name is used for guest IDs to prevent duplicates
// when multiple cluster nodes are configured as separate PVE instances.
func (m *Monitor) pollVMsAndContainersEfficient(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodeEffectiveStatus map[string]string) bool {
	log.Debug().
		Str("instance", instanceName).
		Str("clusterName", clusterName).
		Bool("isCluster", isCluster).
		Msg("Polling VMs and containers using efficient cluster/resources endpoint")

	// Get all resources in a single API call
	resources, err := client.GetClusterResources(ctx, "vm")
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("cluster/resources not available, falling back to traditional polling")
		return false
	}

	// Seed OCI classification from previous state so we never "downgrade" to LXC
	// if container config fetching intermittently fails (permissions or transient API errors).
	prevState := m.GetState()
	prevContainerIsOCI := seedPrevContainerOCI(instanceName, prevState)
	// Build a lookup map from VM guest ID -> linked host agent.
	// When a Pulse agent runs inside a VM, it reads /proc/meminfo directly
	// and gets accurate MemAvailable (excluding page cache). We use this as
	// a memory fallback before the inflated status.Mem value. Refs: #1270
	vmIDToHostAgent := make(map[string]models.Host)
	for _, h := range prevState.Hosts {
		if h.LinkedVMID != "" && h.Status == "online" && h.Memory.Total > 0 {
			vmIDToHostAgent[h.LinkedVMID] = h
		}
	}

	allVMs, allContainers := m.collectGuestsFromClusterResources(ctx, instanceName, resources, client, prevContainerIsOCI, vmIDToHostAgent)

	allVMs, allContainers = m.preserveGuestsForGracePeriod(instanceName, resources, prevState, nodeEffectiveStatus, allVMs, allContainers)

	// Always update state when using efficient polling path
	// Even if arrays are empty, we need to update to clear out VMs from genuinely offline nodes
	m.state.UpdateVMsForInstance(instanceName, allVMs)

	// Check Docker presence for containers that need it (new, restarted, started)
	allContainers = m.CheckContainersForDocker(ctx, allContainers)

	m.state.UpdateContainersForInstance(instanceName, allContainers)

	m.recordGuestMetrics(allVMs, allContainers)

	m.pollReplicationStatus(ctx, instanceName, client, allVMs)

	log.Debug().
		Str("instance", instanceName).
		Int("vms", len(allVMs)).
		Int("containers", len(allContainers)).
		Msg("VMs and containers polled efficiently with cluster/resources")

	return true
}

func (m *Monitor) collectGuestsFromClusterResources(
	ctx context.Context,
	instanceName string,
	resources []proxmox.ClusterResource,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
	vmIDToHostAgent map[string]models.Host,
) ([]models.VM, []models.Container) {
	allVMs := make([]models.VM, 0, len(resources))
	allContainers := make([]models.Container, 0, len(resources))

	for _, res := range resources {
		// Generate canonical guest ID: instance:node:vmid
		guestID := makeGuestID(instanceName, res.Node, res.VMID)

		// Debug log the resource type
		log.Debug().
			Str("instance", instanceName).
			Str("name", res.Name).
			Int("vmid", res.VMID).
			Str("type", res.Type).
			Msg("Processing cluster resource")

		switch res.Type {
		case "qemu":
			vm, ok := m.handleClusterVMResource(ctx, instanceName, res, guestID, client, vmIDToHostAgent)
			if !ok {
				continue
			}
			allVMs = append(allVMs, vm)
		case "lxc":
			container, ok := m.handleClusterContainerResource(ctx, instanceName, res, guestID, client, prevContainerIsOCI)
			if !ok {
				continue
			}
			allContainers = append(allContainers, container)
		}
	}

	return allVMs, allContainers
}

func (m *Monitor) handleClusterVMResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	guestID string,
	client PVEClientInterface,
	vmIDToHostAgent map[string]models.Host,
) (models.VM, bool) {
	vm, guestRaw, memorySource, sampleTime, ok := m.buildVMFromClusterResource(ctx, instanceName, res, client, guestID, vmIDToHostAgent)
	if !ok {
		return models.VM{}, false
	}

	// Trigger guest metadata migration if old format exists
	if m.guestMetadataStore != nil {
		m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, res.Node, res.VMID)
	}

	m.recordGuestSnapshot(instanceName, vm.Type, res.Node, res.VMID, GuestMemorySnapshot{
		Name:         vm.Name,
		Status:       vm.Status,
		RetrievedAt:  sampleTime,
		MemorySource: memorySource,
		Memory:       vm.Memory,
		Raw:          guestRaw,
	})

	m.checkGuestAlertsForVM(instanceName, vm)

	return vm, true
}

func (m *Monitor) handleClusterContainerResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	guestID string,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
) (models.Container, bool) {
	container, guestRaw, memorySource, sampleTime, ok := m.buildContainerFromClusterResource(ctx, instanceName, res, client, prevContainerIsOCI)
	if !ok {
		return models.Container{}, false
	}

	// Trigger guest metadata migration if old format exists
	if m.guestMetadataStore != nil {
		m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, res.Node, res.VMID)
	}

	m.recordGuestSnapshot(instanceName, container.Type, res.Node, res.VMID, GuestMemorySnapshot{
		Name:         container.Name,
		Status:       container.Status,
		RetrievedAt:  sampleTime,
		MemorySource: memorySource,
		Memory:       container.Memory,
		Raw:          guestRaw,
	})

	m.alertManager.CheckGuest(container, instanceName)

	return container, true
}

func (m *Monitor) checkGuestAlertsForVM(instanceName string, vm models.VM) {
	// For non-running VMs, zero out resource usage metrics to prevent false alerts.
	// Proxmox may report stale or residual metrics for stopped VMs.
	alertVM := vm
	if alertVM.Status != "running" {
		log.Debug().
			Str("vm", alertVM.Name).
			Str("status", alertVM.Status).
			Float64("originalCpu", alertVM.CPU).
			Float64("originalMemUsage", alertVM.Memory.Usage).
			Msg("Non-running VM detected - zeroing metrics")

		// Zero out all usage metrics for stopped/paused/suspended VMs
		alertVM.CPU = 0
		alertVM.Memory.Usage = 0
		alertVM.Disk.Usage = 0
		alertVM.NetworkIn = 0
		alertVM.NetworkOut = 0
		alertVM.DiskRead = 0
		alertVM.DiskWrite = 0
	}

	m.alertManager.CheckGuest(alertVM, instanceName)
}
