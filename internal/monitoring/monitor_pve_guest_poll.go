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
	m.updatePVEBackupTemplateSubjectsFromClusterResources(instanceName, resources)
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
	m.state.UpdateGuestsForInstance(instanceName, allVMs, allContainers)

	m.recordGuestMetrics(allVMs, allContainers, cycleStart)

	m.pollReplicationStatusAsync(instanceName, client, allVMs)

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
	allVMs := make([]models.VM, 0, len(resources))
	allContainers := make([]models.Container, 0, len(resources))
	vmResources := make([]indexedClusterResource, 0, len(resources))
	containerResources := make([]indexedClusterResource, 0, len(resources))

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
			vmResources = append(vmResources, indexedClusterResource{
				order:    len(vmResources),
				resource: res,
				guestID:  guestID,
			})
		case "lxc":
			containerResources = append(containerResources, indexedClusterResource{
				order:    len(containerResources),
				resource: res,
				guestID:  guestID,
			})
		}
	}

	// Let VM and LXC detail compete fairly for the shared enrichment slots.
	// Running one type to completion first can consume the entire bounded budget
	// on a large mixed cluster and permanently starve the other type.
	var collectionWG sync.WaitGroup
	if len(containerResources) > 0 {
		collectionWG.Add(1)
		go func() {
			defer collectionWG.Done()
			allContainers = m.collectClusterContainerResources(ctx, instanceName, containerResources, client, prevContainerIsOCI, prevContainerByID)
		}()
	}
	if len(vmResources) > 0 {
		collectionWG.Add(1)
		go func() {
			defer collectionWG.Done()
			allVMs = m.collectClusterVMResources(ctx, instanceName, vmResources, client, prevVMByID, vmIDToHostAgent)
		}()
	}
	collectionWG.Wait()

	return allVMs, allContainers
}

type indexedClusterResource struct {
	order    int
	resource proxmox.ClusterResource
	guestID  string
}

type clusterVMResourceResult struct {
	order int
	vm    models.VM
	ok    bool
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
	if len(current.NetworkInterfaces) == 0 {
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
	resources []indexedClusterResource,
	client PVEClientInterface,
	prevContainerIsOCI map[int]bool,
	prevContainerByID map[string]models.Container,
) []models.Container {
	orderedContainers := make([]models.Container, len(resources))
	orderedOK := make([]bool, len(resources))
	offset := rotatingResourceOffset(len(resources))

	for i := range resources {
		entry := resources[(offset+i)%len(resources)]
		var container models.Container
		var ok bool
		ran := m.runGuestAgentVMWork(ctx, func(workCtx context.Context) {
			container, ok = m.handleClusterContainerResource(workCtx, instanceName, entry.resource, entry.guestID, client, prevContainerIsOCI)
		})
		if !ran {
			// The enrichment budget is exhausted, but cluster/resources is still
			// authoritative inventory. A canceled context makes remote detail
			// calls fail immediately while the builder retains the base row.
			container, ok = m.handleClusterContainerResource(ctx, instanceName, entry.resource, entry.guestID, client, prevContainerIsOCI)
		}
		if ctx.Err() != nil {
			previous := prevContainerByID[entry.guestID]
			container = preserveContainerOptionalEnrichment(container, &previous)
		}
		if ok {
			orderedContainers[entry.order] = container
			orderedOK[entry.order] = true
		}
	}

	containers := make([]models.Container, 0, len(resources))
	for i, ok := range orderedOK {
		if ok {
			containers = append(containers, orderedContainers[i])
		}
	}
	return containers
}

func (m *Monitor) collectClusterVMResources(
	ctx context.Context,
	instanceName string,
	resources []indexedClusterResource,
	client PVEClientInterface,
	prevVMByID map[string]models.VM,
	vmIDToHostAgent map[string]models.Host,
) []models.VM {
	resultCh := make(chan clusterVMResourceResult, len(resources))
	jobCh := make(chan indexedClusterResource, len(resources))
	var wg sync.WaitGroup

	workerCount := m.efficientQEMUWorkerCount(len(resources))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobCh {
				var result clusterVMResourceResult
				ran := m.runGuestAgentVMWork(ctx, func(workCtx context.Context) {
					var prevVM *models.VM
					if prev, ok := prevVMByID[entry.guestID]; ok {
						prevVM = &prev
					}
					vm, ok := m.handleClusterVMResource(workCtx, instanceName, entry.resource, entry.guestID, client, prevVM, vmIDToHostAgent)
					result = clusterVMResourceResult{
						order: entry.order,
						vm:    vm,
						ok:    ok,
					}
				})
				if !ran {
					// Do not turn an exhausted detail budget into missing inventory.
					// Re-run only the builder with the canceled context so it emits the
					// current cluster/resources row without remote enrichment.
					var prevVM *models.VM
					if prev, ok := prevVMByID[entry.guestID]; ok {
						prevVM = &prev
					}
					vm, ok := m.handleClusterVMResource(ctx, instanceName, entry.resource, entry.guestID, client, prevVM, vmIDToHostAgent)
					result = clusterVMResourceResult{order: entry.order, vm: vm, ok: ok}
				}
				if ctx.Err() != nil && result.ok {
					previous, exists := prevVMByID[entry.guestID]
					if exists {
						result.vm = preserveVMOptionalEnrichment(result.vm, &previous)
					}
				}
				resultCh <- result
			}
		}()
	}

	offset := rotatingResourceOffset(len(resources))
	for i := range resources {
		jobCh <- resources[(offset+i)%len(resources)]
	}
	close(jobCh)

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	orderedVMs := make([]models.VM, len(resources))
	orderedOK := make([]bool, len(resources))
	for result := range resultCh {
		if result.order < 0 || result.order >= len(resources) || !result.ok {
			continue
		}
		orderedVMs[result.order] = result.vm
		orderedOK[result.order] = true
	}

	vms := make([]models.VM, 0, len(resources))
	for i, ok := range orderedOK {
		if ok {
			vms = append(vms, orderedVMs[i])
		}
	}
	return vms
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

	// Trigger guest metadata migration if old format exists
	if m.guestMetadataStore != nil {
		m.guestMetadataStore.GetWithLegacyMigration(guestID, instanceName, res.Node, res.VMID)
	}

	m.recordGuestSnapshot(instanceName, vm.Type, res.Node, res.VMID, GuestMemorySnapshot{
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
		Name:           container.Name,
		Status:         container.Status,
		RetrievedAt:    sampleTime,
		MemorySource:   memorySource,
		FallbackReason: guestMemoryFallbackReason(memorySource),
		Memory:         container.Memory,
		Raw:            guestRaw,
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
