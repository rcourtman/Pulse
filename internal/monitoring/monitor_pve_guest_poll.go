package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

const (
	// Deep guest detail is deliberately bounded below the whole PVE poll
	// deadline. The authoritative cluster/resources generation can contain
	// hundreds of guests; reserving the tail keeps connection-health
	// publication from inheriting an exhausted context.
	pveGuestEnrichmentMaxDuration = 60 * time.Second
	pvePollTailReserve            = 15 * time.Second
)

func pveGuestEnrichmentContext(ctx context.Context) (context.Context, context.CancelFunc) {
	budget := pveGuestEnrichmentMaxDuration
	if deadline, ok := ctx.Deadline(); ok {
		available := time.Until(deadline) - pvePollTailReserve
		if available <= 0 {
			boundedCtx, cancel := context.WithCancel(ctx)
			cancel()
			return boundedCtx, func() {}
		}
		if available < budget {
			budget = available
		}
	}
	return context.WithTimeout(ctx, budget)
}

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
	cycleStart := time.Now()
	resources, err := client.GetClusterResources(ctx, "vm")
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("cluster/resources not available, falling back to traditional polling")
		return false
	}
	stagePVEGuestEffect(ctx, func() { m.updatePVEBackupTemplateSubjectsFromClusterResources(instanceName, resources) })
	enrichmentCtx, cancelEnrichment := pveGuestEnrichmentContext(ctx)
	defer cancelEnrichment()

	// Capture previous guest state once per poll cycle so fallback and grace-period
	// behavior is based on a consistent pre-poll snapshot.
	prevGuests := m.previousGuestContextForInstance(instanceName)

	allVMs, allContainers := m.collectGuestsFromClusterResources(
		enrichmentCtx,
		instanceName,
		resources,
		client,
		prevGuests.containerOCIByVMID,
		prevGuests.containersByID,
		prevGuests.vmsByID,
		prevGuests.hostAgentsByVMID,
	)

	allVMs, allContainers = m.preserveGuestsForGracePeriod(instanceName, resources, prevGuests.vms, prevGuests.containers, nodeEffectiveStatus, allVMs, allContainers)

	// Check Docker presence for containers that need it (new, restarted, started)
	allContainers = m.CheckContainersForDocker(enrichmentCtx, allContainers)
	m.CollectProxmoxGuestDockerInventory(enrichmentCtx, allContainers)

	// Publish the complete guest generation only after both VM and container
	// collection/enrichment has finished. Empty authoritative results still
	// remove genuinely deleted guests.
	stagePVEGuestEffect(ctx, func() { m.recordGuestMetrics(allVMs, allContainers, cycleStart) })
	m.publishPVEGuests(ctx, instanceName, allVMs, allContainers)

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
	prevContainerByID map[string]models.Container,
	prevVMByID map[string]models.VM,
	vmIDToHostAgent map[string]models.Host,
) ([]models.VM, []models.Container) {
	// Partition the read-only inventory by index. The workers join before
	// returning, so they can use the original rows instead of copying them
	// into another inventory-sized allocation.
	vmCount, containerCount := 0, 0
	for i := range resources {
		switch resources[i].Type {
		case "qemu":
			vmCount++
		case "lxc":
			containerCount++
		}
	}
	vmResources := clusterGuestInventory{rows: resources, indices: make([]int, 0, vmCount)}
	containerResources := clusterGuestInventory{rows: resources, indices: make([]int, 0, containerCount)}

	for i := range resources {
		res := &resources[i]

		// Debug log the resource type
		log.Debug().
			Str("instance", instanceName).
			Str("name", res.Name).
			Int("vmid", res.VMID).
			Str("type", res.Type).
			Msg("Processing cluster resource")

		switch res.Type {
		case "qemu":
			vmResources.indices = append(vmResources.indices, i)
		case "lxc":
			containerResources.indices = append(containerResources.indices, i)
		}
	}

	// Let VM and LXC detail compete fairly for the shared enrichment slots.
	// Running one type to completion first can consume the entire bounded budget
	// on a large mixed cluster and permanently starve the other type.
	allVMs, allContainers := []models.VM{}, []models.Container{}
	var collectionWG sync.WaitGroup
	if containerResources.size() > 0 {
		collectionWG.Add(1)
		go func() {
			defer collectionWG.Done()
			allContainers = m.collectClusterContainerResources(ctx, instanceName, containerResources, client, prevContainerIsOCI, prevContainerByID, vmIDToHostAgent)
		}()
	}
	if vmResources.size() > 0 {
		collectionWG.Add(1)
		go func() {
			defer collectionWG.Done()
			allVMs = m.collectClusterVMResources(ctx, instanceName, vmResources, client, prevVMByID, vmIDToHostAgent)
		}()
	}
	collectionWG.Wait()

	return allVMs, allContainers
}

// A partition's positions follow provider order. A nil index selects the
// entire contiguous inventory, used by the single-kind node fallback.
type clusterGuestInventory struct {
	rows    []proxmox.ClusterResource
	indices []int
}

func (inv clusterGuestInventory) size() int {
	if inv.indices != nil {
		return len(inv.indices)
	}
	return len(inv.rows)
}

func (inv clusterGuestInventory) row(position int) proxmox.ClusterResource {
	if inv.indices != nil {
		position = inv.indices[position]
	}
	return inv.rows[position]
}

func rotatingResourceOffset(total int) int {
	if total <= 1 {
		return 0
	}
	return int(uint64(time.Now().UnixNano()) % uint64(total))
}

func preserveVMOptionalEnrichment(current models.VM, previous *models.VM) models.VM {
	if previous == nil {
		return current
	}
	if len(current.IPAddresses) == 0 {
		current.IPAddresses = cloneStringSlice(previous.IPAddresses)
	}
	// A row built without guest detail can still carry devices known from
	// configuration; those alone do not replace what the guest last reported.
	if len(guestReportedInterfaces(current.NetworkInterfaces)) == 0 && len(previous.NetworkInterfaces) > 0 {
		current.NetworkInterfaces = cloneGuestNetworkInterfaces(previous.NetworkInterfaces)
	}
	if current.OSName == "" {
		current.OSName = previous.OSName
	}
	if current.OSVersion == "" {
		current.OSVersion = previous.OSVersion
	}
	if current.AgentVersion == "" {
		current.AgentVersion = previous.AgentVersion
	}
	if len(current.Disks) == 0 {
		current.Disks = cloneGuestDisks(previous.Disks)
	}
	if current.OnBoot == nil && previous.OnBoot != nil {
		onBoot := *previous.OnBoot
		current.OnBoot = &onBoot
	}
	if current.Lock == "" {
		current.Lock = previous.Lock
	}
	return current
}

func preserveContainerOptionalEnrichment(current models.Container, previous *models.Container) models.Container {
	if previous == nil {
		return current
	}
	if len(current.IPAddresses) == 0 {
		current.IPAddresses = cloneStringSlice(previous.IPAddresses)
	}
	if len(current.NetworkInterfaces) == 0 {
		current.NetworkInterfaces = cloneGuestNetworkInterfaces(previous.NetworkInterfaces)
	}
	if current.OSName == "" {
		current.OSName = previous.OSName
	}
	if current.OSTemplate == "" {
		current.OSTemplate = previous.OSTemplate
	}
	if len(current.Disks) == 0 {
		current.Disks = cloneGuestDisks(previous.Disks)
	}
	if current.OnBoot == nil && previous.OnBoot != nil {
		onBoot := *previous.OnBoot
		current.OnBoot = &onBoot
	}
	if current.DockerCheckedAt.IsZero() {
		current.HasDocker = previous.HasDocker
		current.DockerCheckedAt = previous.DockerCheckedAt
	}
	if current.Lock == "" {
		current.Lock = previous.Lock
	}
	return current
}

func (m *Monitor) collectClusterContainerResources(
	ctx context.Context,
	instanceName string,
	resources clusterGuestInventory,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
	prevContainerByID map[string]models.Container,
	vmIDToHostAgent map[string]models.Host,
) []models.Container {
	count := resources.size()
	orderedContainers := make([]models.Container, count)
	orderedOK := make([]bool, count)
	offset := rotatingResourceOffset(count)

	for i := range count {
		index := (offset + i) % count
		res := resources.row(index)
		guestID := makeGuestID(instanceName, res.Node, res.VMID)
		var container models.Container
		var ok bool
		ran := m.runGuestAgentVMWork(ctx, func(workCtx context.Context) {
			container, ok = m.handleClusterContainerResource(workCtx, instanceName, res, guestID, client, prevContainerIsOCI, vmIDToHostAgent)
		})
		if !ran {
			// The enrichment budget is exhausted, but cluster/resources is still
			// authoritative inventory. A canceled context makes remote detail
			// calls fail immediately while the builder retains the base row.
			container, ok = m.handleClusterContainerResource(ctx, instanceName, res, guestID, client, prevContainerIsOCI, vmIDToHostAgent)
		}
		if ctx.Err() != nil {
			previous := prevContainerByID[guestID]
			container = preserveContainerOptionalEnrichment(container, &previous)
		}
		if ok {
			orderedContainers[index] = container
			orderedOK[index] = true
		}
	}

	return compactClusterGuestRows(orderedContainers, orderedOK)
}

func (m *Monitor) collectClusterVMResources(
	ctx context.Context,
	instanceName string,
	resources clusterGuestInventory,
	client PVEClientInterface,
	prevVMByID map[string]models.VM,
	vmIDToHostAgent map[string]models.Host,
) []models.VM {
	count := resources.size()
	workerCount := m.efficientQEMUWorkerCount(count)
	if workerCount == 0 {
		return []models.VM{}
	}
	// Queue only indexes, bounded by the worker count. Each index is sent
	// exactly once; its worker owns that output slot until the wait completes.
	jobCh := make(chan int, workerCount)
	orderedVMs := make([]models.VM, count)
	orderedOK := make([]bool, count)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			for index := range jobCh {
				res := resources.row(index)
				guestID := makeGuestID(instanceName, res.Node, res.VMID)
				var previous *models.VM
				if prev, ok := prevVMByID[guestID]; ok {
					previous = &prev
				}
				var vm models.VM
				var ok bool
				build := func(workCtx context.Context) {
					vm, ok = m.handleClusterVMResource(workCtx, instanceName, res, guestID, client, previous, vmIDToHostAgent)
				}
				if !m.runGuestAgentVMWork(ctx, build) {
					// Do not turn an exhausted detail budget into missing inventory.
					// Re-run only the builder with the canceled context so it emits the
					// current cluster/resources row without remote enrichment.
					build(ctx)
				}
				if !ok {
					continue
				}
				if ctx.Err() != nil {
					vm = preserveVMOptionalEnrichment(vm, previous)
				}
				orderedVMs[index] = vm
				orderedOK[index] = true
			}
		})
	}

	offset := rotatingResourceOffset(count)
	// Drain the entire inventory even after the enrichment deadline, so the
	// canceled-context builders still get to publish every accepted base row.
	for i := range count {
		jobCh <- (offset + i) % count
	}
	close(jobCh)
	wg.Wait()
	return compactClusterGuestRows(orderedVMs, orderedOK)
}

// Drop rejected rows (such as templates) without a second inventory-sized
// allocation. Clear any moved rows still referenced by the unused tail.
func compactClusterGuestRows[T any](rows []T, accepted []bool) []T {
	next := 0
	for i, ok := range accepted {
		if !ok {
			continue
		}
		if i != next {
			rows[next] = rows[i]
		}
		next++
	}
	clear(rows[next:])
	return rows[:next]
}

func (m *Monitor) handleClusterVMResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	guestID string,
	client PVEClientInterface,
	prevVM *models.VM,
	vmIDToHostAgent map[string]models.Host,
) (models.VM, bool) {
	vm, guestRaw, memorySource, snapshotNotes, sampleTime, ok := m.buildVMFromClusterResource(ctx, instanceName, res, client, guestID, vmIDToHostAgent, prevVM)
	if !ok {
		return models.VM{}, false
	}

	// Staged effects only need these coordinates from the source row.
	node, vmid := res.Node, res.VMID
	stagePVEGuestEffect(ctx, func() {
		m.recordGuestSnapshot(instanceName, vm.Type, node, vmid, GuestMemorySnapshot{
			Name:           vm.Name,
			Status:         vm.Status,
			RetrievedAt:    sampleTime,
			MemorySource:   memorySource,
			FallbackReason: guestMemoryFallbackReason(memorySource),
			Memory:         vm.Memory,
			Raw:            guestRaw,
			Notes:          snapshotNotes,
		})

		m.checkGuestAlertsForVM(instanceName, vm)
	})

	return vm, true
}

func (m *Monitor) handleClusterContainerResource(
	ctx context.Context,
	instanceName string,
	res proxmox.ClusterResource,
	guestID string,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
	vmIDToHostAgent map[string]models.Host,
) (models.Container, bool) {
	container, guestRaw, memorySource, sampleTime, ok := m.buildContainerFromClusterResource(ctx, instanceName, res, client, prevContainerIsOCI, vmIDToHostAgent)
	if !ok {
		return models.Container{}, false
	}

	node, vmid := res.Node, res.VMID
	stagePVEGuestEffect(ctx, func() {
		m.recordGuestSnapshot(instanceName, container.Type, node, vmid, GuestMemorySnapshot{
			Name:           container.Name,
			Status:         container.Status,
			RetrievedAt:    sampleTime,
			MemorySource:   memorySource,
			FallbackReason: guestMemoryFallbackReason(memorySource),
			Memory:         container.Memory,
			Raw:            guestRaw,
		})

		m.alertManager.CheckGuest(container, instanceName)
	})

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
