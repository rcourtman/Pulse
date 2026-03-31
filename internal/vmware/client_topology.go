package vmware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type vcenterVMIdentity struct {
	BIOSUUID     string `json:"bios_uuid"`
	InstanceUUID string `json:"instance_uuid"`
}

type vcenterVMInfo struct {
	Identity *vcenterVMIdentity `json:"identity"`
}

type vcenterVMGuestIdentity struct {
	Family   string `json:"family"`
	HostName string `json:"host_name"`
	IPAddr   string `json:"ip_address"`
}

type viJSONVirtualMachineRuntimeInfo struct {
	Host *viJSONReference `json:"host"`
}

type viJSONDatastoreSummary struct {
	Accessible         *bool  `json:"accessible"`
	MultipleHostAccess *bool  `json:"multipleHostAccess"`
	MaintenanceMode    string `json:"maintenanceMode"`
	URL                string `json:"url"`
}

type vmwarePlacement struct {
	DatacenterID        string
	DatacenterName      string
	ComputeResourceID   string
	ComputeResourceName string
	ClusterID           string
	ClusterName         string
	FolderID            string
	FolderName          string
}

type vmwareTopologyCache struct {
	mu sync.Mutex

	nameLoaded map[string]bool
	names      map[string]string

	parentLoaded map[string]bool
	parents      map[string]*viJSONReference

	ownerLoaded map[string]bool
	owners      map[string]*viJSONReference

	placementLoaded map[string]bool
	placements      map[string]vmwarePlacement
}

func newVMwareTopologyCache() *vmwareTopologyCache {
	return &vmwareTopologyCache{
		nameLoaded:      make(map[string]bool),
		names:           make(map[string]string),
		parentLoaded:    make(map[string]bool),
		parents:         make(map[string]*viJSONReference),
		ownerLoaded:     make(map[string]bool),
		owners:          make(map[string]*viJSONReference),
		placementLoaded: make(map[string]bool),
		placements:      make(map[string]vmwarePlacement),
	}
}

func (c *Client) enrichInventoryTopology(
	ctx context.Context,
	automationSessionID string,
	release string,
	sessionID string,
	snapshot *InventorySnapshot,
) ([]InventoryEnrichmentIssue, error) {
	if snapshot == nil {
		return nil, nil
	}

	hostNamesByID := make(map[string]string, len(snapshot.Hosts))
	for _, host := range snapshot.Hosts {
		hostNamesByID[strings.TrimSpace(host.Host)] = firstNonEmptyTrimmed(host.Name, host.Host)
	}
	vmNamesByID := make(map[string]string, len(snapshot.VMs))
	for _, vm := range snapshot.VMs {
		vmNamesByID[strings.TrimSpace(vm.VM)] = firstNonEmptyTrimmed(vm.Name, vm.VM)
	}
	datastoreNamesByID := make(map[string]string, len(snapshot.Datastores))
	for _, datastore := range snapshot.Datastores {
		datastoreNamesByID[strings.TrimSpace(datastore.Datastore)] = firstNonEmptyTrimmed(datastore.Name, datastore.Datastore)
	}

	cache := newVMwareTopologyCache()
	sem := make(chan struct{}, vmwareSignalEnrichmentConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex
	var issues []InventoryEnrichmentIssue
	var issuesMu sync.Mutex

	recordIssues := func(values []InventoryEnrichmentIssue) {
		if len(values) == 0 {
			return
		}
		issuesMu.Lock()
		issues = append(issues, values...)
		issuesMu.Unlock()
	}

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := fn(); err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				firstErrMu.Unlock()
			}
		}()
	}

	for i := range snapshot.Hosts {
		i := i
		run(func() error {
			host, hostIssues, err := c.enrichHostTopology(ctx, release, sessionID, snapshot.Hosts[i], cache, datastoreNamesByID)
			if err != nil {
				return err
			}
			recordIssues(hostIssues)
			snapshot.Hosts[i] = host
			return nil
		})
	}

	for i := range snapshot.VMs {
		i := i
		run(func() error {
			vm, vmIssues, err := c.enrichVMTopology(
				ctx,
				automationSessionID,
				release,
				sessionID,
				snapshot.VMs[i],
				cache,
				hostNamesByID,
				datastoreNamesByID,
			)
			if err != nil {
				return err
			}
			recordIssues(vmIssues)
			snapshot.VMs[i] = vm
			return nil
		})
	}

	for i := range snapshot.Datastores {
		i := i
		run(func() error {
			datastore, datastoreIssues, err := c.enrichDatastoreTopology(ctx, release, sessionID, snapshot.Datastores[i], cache, hostNamesByID, vmNamesByID)
			if err != nil {
				return err
			}
			recordIssues(datastoreIssues)
			snapshot.Datastores[i] = datastore
			return nil
		})
	}

	wg.Wait()

	firstErrMu.Lock()
	err := firstErr
	firstErrMu.Unlock()
	if err != nil {
		return nil, err
	}
	sort.Slice(issues, func(i, j int) bool {
		return inventoryEnrichmentIssueSortKey(issues[i]) < inventoryEnrichmentIssueSortKey(issues[j])
	})
	return issues, nil
}

func (c *Client) enrichHostTopology(
	ctx context.Context,
	release string,
	sessionID string,
	host InventoryHost,
	cache *vmwareTopologyCache,
	datastoreNamesByID map[string]string,
) (InventoryHost, []InventoryEnrichmentIssue, error) {
	var issues []InventoryEnrichmentIssue
	recordIssue := func(issue *InventoryEnrichmentIssue) {
		if issue != nil {
			issues = append(issues, *issue)
		}
	}

	ref := viJSONReference{Type: "HostSystem", Value: strings.TrimSpace(host.Host)}
	placement, err := cache.resolvePlacement(ctx, c, release, sessionID, ref)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "host", host.Host, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return host, nil, err
	}
	applyPlacementToHost(&host, placement)

	datastoreRefs, err := c.collectEntityReferenceList(ctx, release, sessionID, "HostSystem", host.Host, "datastore", "host datastore attachments")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "host", host.Host, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return host, nil, err
	}
	host.DatastoreIDs = idsForReferences(datastoreRefs)
	host.DatastoreNames = namesForReferences(datastoreRefs, datastoreNamesByID)

	return host, issues, nil
}

func (c *Client) enrichVMTopology(
	ctx context.Context,
	automationSessionID string,
	release string,
	sessionID string,
	vm InventoryVM,
	cache *vmwareTopologyCache,
	hostNamesByID map[string]string,
	datastoreNamesByID map[string]string,
) (InventoryVM, []InventoryEnrichmentIssue, error) {
	var issues []InventoryEnrichmentIssue
	recordIssue := func(issue *InventoryEnrichmentIssue) {
		if issue != nil {
			issues = append(issues, *issue)
		}
	}

	vmInfo, err := c.collectVMAutomationInfo(ctx, automationSessionID, vm.VM)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isAutomationNotFound(err) {
		return vm, nil, err
	}
	if vmInfo != nil && vmInfo.Identity != nil {
		vm.InstanceUUID = strings.TrimSpace(vmInfo.Identity.InstanceUUID)
		vm.BIOSUUID = strings.TrimSpace(vmInfo.Identity.BIOSUUID)
	}

	guestIdentity, err := c.collectVMGuestIdentity(ctx, automationSessionID, vm.VM)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isAutomationNotFound(err) && !isAutomationUnavailable(err) {
		return vm, nil, err
	}
	if guestIdentity != nil {
		vm.GuestOSFamily = strings.TrimSpace(guestIdentity.Family)
		vm.GuestHostname = strings.TrimSpace(guestIdentity.HostName)
		if ip := strings.TrimSpace(guestIdentity.IPAddr); ip != "" {
			vm.GuestIPAddresses = []string{ip}
		}
	}

	var placement vmwarePlacement

	parentRef, err := c.collectEntityReference(ctx, release, sessionID, "VirtualMachine", vm.VM, "parent", "vm parent placement")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return vm, nil, err
	}
	if parentRef != nil {
		parentPlacement, err := cache.resolvePlacement(ctx, c, release, sessionID, *parentRef)
		if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
			recordIssue(issue)
		} else if err != nil && !isVIJSONNotFound(err) {
			return vm, nil, err
		}
		mergePlacement(&placement, parentPlacement)
	}

	runtimeHostRef, err := c.collectVMRuntimeHostReference(ctx, release, sessionID, vm.VM)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return vm, nil, err
	}
	if runtimeHostRef != nil {
		vm.RuntimeHostID = strings.TrimSpace(runtimeHostRef.Value)
		vm.RuntimeHostName = firstNonEmptyTrimmed(hostNamesByID[vm.RuntimeHostID], vm.RuntimeHostID)
		hostPlacement, err := cache.resolvePlacement(ctx, c, release, sessionID, *runtimeHostRef)
		if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
			recordIssue(issue)
		} else if err != nil && !isVIJSONNotFound(err) {
			return vm, nil, err
		}
		mergePlacement(&placement, hostPlacement)
	}

	resourcePoolRef, err := c.collectEntityReference(ctx, release, sessionID, "VirtualMachine", vm.VM, "resourcePool", "vm resource pool")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return vm, nil, err
	}
	if resourcePoolRef != nil {
		vm.ResourcePoolID = strings.TrimSpace(resourcePoolRef.Value)
		resourcePoolName, err := cache.resolveName(ctx, c, release, sessionID, *resourcePoolRef)
		if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
			recordIssue(issue)
		} else if err != nil && !isVIJSONNotFound(err) {
			return vm, nil, err
		}
		vm.ResourcePoolName = firstNonEmptyTrimmed(resourcePoolName, vm.ResourcePoolID)

		ownerRef, err := cache.resolveResourcePoolOwner(ctx, c, release, sessionID, *resourcePoolRef)
		if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
			recordIssue(issue)
		} else if err != nil && !isVIJSONNotFound(err) {
			return vm, nil, err
		}
		if ownerRef != nil {
			ownerPlacement, err := cache.resolvePlacement(ctx, c, release, sessionID, *ownerRef)
			if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
				recordIssue(issue)
			} else if err != nil && !isVIJSONNotFound(err) {
				return vm, nil, err
			}
			mergePlacement(&placement, ownerPlacement)
		}
	}

	datastoreRefs, err := c.collectEntityReferenceList(ctx, release, sessionID, "VirtualMachine", vm.VM, "datastore", "vm datastore attachments")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "vm", vm.VM, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return vm, nil, err
	}
	vm.DatastoreIDs = idsForReferences(datastoreRefs)
	vm.DatastoreNames = namesForReferences(datastoreRefs, datastoreNamesByID)

	applyPlacementToVM(&vm, placement)
	return vm, issues, nil
}

func (c *Client) enrichDatastoreTopology(
	ctx context.Context,
	release string,
	sessionID string,
	datastore InventoryDatastore,
	cache *vmwareTopologyCache,
	hostNamesByID map[string]string,
	vmNamesByID map[string]string,
) (InventoryDatastore, []InventoryEnrichmentIssue, error) {
	var issues []InventoryEnrichmentIssue
	recordIssue := func(issue *InventoryEnrichmentIssue) {
		if issue != nil {
			issues = append(issues, *issue)
		}
	}

	ref := viJSONReference{Type: "Datastore", Value: strings.TrimSpace(datastore.Datastore)}
	placement, err := cache.resolvePlacement(ctx, c, release, sessionID, ref)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "storage", datastore.Datastore, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return datastore, nil, err
	}
	applyPlacementToDatastore(&datastore, placement)

	summary, err := c.collectDatastoreSummary(ctx, release, sessionID, datastore.Datastore)
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "storage", datastore.Datastore, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return datastore, nil, err
	}
	if summary != nil {
		datastore.Accessible = summary.Accessible
		datastore.MultipleHostAccess = summary.MultipleHostAccess
		datastore.MaintenanceMode = strings.TrimSpace(summary.MaintenanceMode)
		datastore.URL = strings.TrimSpace(summary.URL)
	}

	hostRefs, err := c.collectEntityReferenceList(ctx, release, sessionID, "Datastore", datastore.Datastore, "host", "datastore host attachments")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "storage", datastore.Datastore, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return datastore, nil, err
	}
	datastore.HostIDs = idsForReferences(hostRefs)
	datastore.HostNames = namesForReferences(hostRefs, hostNamesByID)

	vmRefs, err := c.collectEntityReferenceList(ctx, release, sessionID, "Datastore", datastore.Datastore, "vm", "datastore vm attachments")
	if issue, ok := classifyInventoryEnrichmentIssue("topology", "storage", datastore.Datastore, err); ok {
		recordIssue(issue)
	} else if err != nil && !isVIJSONNotFound(err) {
		return datastore, nil, err
	}
	datastore.VMIDs = idsForReferences(vmRefs)
	datastore.VMNames = namesForReferences(vmRefs, vmNamesByID)

	return datastore, issues, nil
}

func (c *Client) collectVMAutomationInfo(
	ctx context.Context,
	automationSessionID string,
	vmID string,
) (*vcenterVMInfo, error) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" || strings.TrimSpace(automationSessionID) == "" {
		return nil, nil
	}
	var payload vcenterVMInfo
	path := fmt.Sprintf("/api/vcenter/vm/%s", vmID)
	if err := c.getAutomationJSON(ctx, automationSessionID, path, "vm detail", &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) collectVMGuestIdentity(
	ctx context.Context,
	automationSessionID string,
	vmID string,
) (*vcenterVMGuestIdentity, error) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" || strings.TrimSpace(automationSessionID) == "" {
		return nil, nil
	}
	var payload vcenterVMGuestIdentity
	path := fmt.Sprintf("/api/vcenter/vm/%s/guest/identity", vmID)
	if err := c.getAutomationJSON(ctx, automationSessionID, path, "vm guest identity", &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) collectVMRuntimeHostReference(
	ctx context.Context,
	release string,
	sessionID string,
	vmID string,
) (*viJSONReference, error) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return nil, nil
	}
	var runtime viJSONVirtualMachineRuntimeInfo
	path := fmt.Sprintf("/sdk/vim25/%s/VirtualMachine/%s/runtime", release, vmID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "vm runtime placement", &runtime); err != nil {
		return nil, err
	}
	return cloneVIJSONReference(runtime.Host), nil
}

func (c *Client) collectDatastoreSummary(
	ctx context.Context,
	release string,
	sessionID string,
	datastoreID string,
) (*viJSONDatastoreSummary, error) {
	datastoreID = strings.TrimSpace(datastoreID)
	if datastoreID == "" {
		return nil, nil
	}
	var summary viJSONDatastoreSummary
	path := fmt.Sprintf("/sdk/vim25/%s/Datastore/%s/summary", release, datastoreID)
	if err := c.getVIJSONJSON(ctx, sessionID, path, "datastore summary", &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (c *Client) collectEntityReference(
	ctx context.Context,
	release string,
	sessionID string,
	managedType string,
	managedObjectID string,
	property string,
	label string,
) (*viJSONReference, error) {
	managedObjectID = strings.TrimSpace(managedObjectID)
	if managedObjectID == "" {
		return nil, nil
	}
	var ref *viJSONReference
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/%s", release, managedType, managedObjectID, property)
	if err := c.getVIJSONJSON(ctx, sessionID, path, label, &ref); err != nil {
		return nil, err
	}
	return cloneVIJSONReference(ref), nil
}

func (c *Client) collectEntityReferenceList(
	ctx context.Context,
	release string,
	sessionID string,
	managedType string,
	managedObjectID string,
	property string,
	label string,
) ([]viJSONReference, error) {
	managedObjectID = strings.TrimSpace(managedObjectID)
	if managedObjectID == "" {
		return nil, nil
	}
	var refs []viJSONReference
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/%s", release, managedType, managedObjectID, property)
	if err := c.getVIJSONJSON(ctx, sessionID, path, label, &refs); err != nil {
		return nil, err
	}
	return cloneVIJSONReferences(refs), nil
}

func (cache *vmwareTopologyCache) resolvePlacement(
	ctx context.Context,
	client *Client,
	release string,
	sessionID string,
	ref viJSONReference,
) (vmwarePlacement, error) {
	key := managedObjectKey(ref.Type, ref.Value)
	if key == "" {
		return vmwarePlacement{}, nil
	}

	cache.mu.Lock()
	if cache.placementLoaded[key] {
		placement := cache.placements[key]
		cache.mu.Unlock()
		return placement, nil
	}
	cache.mu.Unlock()

	placement := vmwarePlacement{}
	name, err := cache.resolveName(ctx, client, release, sessionID, ref)
	if err != nil && !isVIJSONNotFound(err) {
		return vmwarePlacement{}, err
	}
	name = firstNonEmptyTrimmed(name, ref.Value)

	switch strings.TrimSpace(ref.Type) {
	case "Datacenter":
		placement.DatacenterID = strings.TrimSpace(ref.Value)
		placement.DatacenterName = name
	case "Folder":
		placement.FolderID = strings.TrimSpace(ref.Value)
		placement.FolderName = name
	case "ClusterComputeResource":
		placement.ClusterID = strings.TrimSpace(ref.Value)
		placement.ClusterName = name
		placement.ComputeResourceID = strings.TrimSpace(ref.Value)
		placement.ComputeResourceName = name
	case "ComputeResource":
		placement.ComputeResourceID = strings.TrimSpace(ref.Value)
		placement.ComputeResourceName = name
	}

	parentRef, err := cache.resolveParent(ctx, client, release, sessionID, ref)
	if err != nil && !isVIJSONNotFound(err) {
		return vmwarePlacement{}, err
	}
	if parentRef != nil {
		parentPlacement, err := cache.resolvePlacement(ctx, client, release, sessionID, *parentRef)
		if err != nil && !isVIJSONNotFound(err) {
			return vmwarePlacement{}, err
		}
		mergePlacement(&placement, parentPlacement)
	}

	cache.mu.Lock()
	cache.placementLoaded[key] = true
	cache.placements[key] = placement
	cache.mu.Unlock()
	return placement, nil
}

func (cache *vmwareTopologyCache) resolveName(
	ctx context.Context,
	client *Client,
	release string,
	sessionID string,
	ref viJSONReference,
) (string, error) {
	key := managedObjectKey(ref.Type, ref.Value)
	if key == "" {
		return "", nil
	}

	cache.mu.Lock()
	if cache.nameLoaded[key] {
		name := cache.names[key]
		cache.mu.Unlock()
		return name, nil
	}
	cache.mu.Unlock()

	var name string
	path := fmt.Sprintf("/sdk/vim25/%s/%s/%s/name", release, ref.Type, ref.Value)
	if err := client.getVIJSONJSON(ctx, sessionID, path, strings.ToLower(strings.TrimSpace(ref.Type))+" name", &name); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)

	cache.mu.Lock()
	cache.nameLoaded[key] = true
	cache.names[key] = name
	cache.mu.Unlock()
	return name, nil
}

func (cache *vmwareTopologyCache) resolveParent(
	ctx context.Context,
	client *Client,
	release string,
	sessionID string,
	ref viJSONReference,
) (*viJSONReference, error) {
	key := managedObjectKey(ref.Type, ref.Value)
	if key == "" {
		return nil, nil
	}

	cache.mu.Lock()
	if cache.parentLoaded[key] {
		parent := cloneVIJSONReference(cache.parents[key])
		cache.mu.Unlock()
		return parent, nil
	}
	cache.mu.Unlock()

	parent, err := client.collectEntityReference(
		ctx,
		release,
		sessionID,
		strings.TrimSpace(ref.Type),
		strings.TrimSpace(ref.Value),
		"parent",
		strings.ToLower(strings.TrimSpace(ref.Type))+" parent",
	)
	if err != nil {
		return nil, err
	}

	cache.mu.Lock()
	cache.parentLoaded[key] = true
	cache.parents[key] = cloneVIJSONReference(parent)
	cache.mu.Unlock()
	return parent, nil
}

func (cache *vmwareTopologyCache) resolveResourcePoolOwner(
	ctx context.Context,
	client *Client,
	release string,
	sessionID string,
	ref viJSONReference,
) (*viJSONReference, error) {
	key := managedObjectKey(ref.Type, ref.Value)
	if key == "" {
		return nil, nil
	}

	cache.mu.Lock()
	if cache.ownerLoaded[key] {
		owner := cloneVIJSONReference(cache.owners[key])
		cache.mu.Unlock()
		return owner, nil
	}
	cache.mu.Unlock()

	owner, err := client.collectEntityReference(
		ctx,
		release,
		sessionID,
		"ResourcePool",
		strings.TrimSpace(ref.Value),
		"owner",
		"resource pool owner",
	)
	if err != nil {
		return nil, err
	}

	cache.mu.Lock()
	cache.ownerLoaded[key] = true
	cache.owners[key] = cloneVIJSONReference(owner)
	cache.mu.Unlock()
	return owner, nil
}

func applyPlacementToHost(host *InventoryHost, placement vmwarePlacement) {
	if host == nil {
		return
	}
	host.DatacenterID = firstNonEmptyTrimmed(host.DatacenterID, placement.DatacenterID)
	host.DatacenterName = firstNonEmptyTrimmed(host.DatacenterName, placement.DatacenterName)
	host.ComputeResourceID = firstNonEmptyTrimmed(host.ComputeResourceID, placement.ComputeResourceID)
	host.ComputeResourceName = firstNonEmptyTrimmed(host.ComputeResourceName, placement.ComputeResourceName)
	host.ClusterID = firstNonEmptyTrimmed(host.ClusterID, placement.ClusterID)
	host.ClusterName = firstNonEmptyTrimmed(host.ClusterName, placement.ClusterName)
	host.FolderID = firstNonEmptyTrimmed(host.FolderID, placement.FolderID)
	host.FolderName = firstNonEmptyTrimmed(host.FolderName, placement.FolderName)
}

func applyPlacementToVM(vm *InventoryVM, placement vmwarePlacement) {
	if vm == nil {
		return
	}
	vm.DatacenterID = firstNonEmptyTrimmed(vm.DatacenterID, placement.DatacenterID)
	vm.DatacenterName = firstNonEmptyTrimmed(vm.DatacenterName, placement.DatacenterName)
	vm.ComputeResourceID = firstNonEmptyTrimmed(vm.ComputeResourceID, placement.ComputeResourceID)
	vm.ComputeResourceName = firstNonEmptyTrimmed(vm.ComputeResourceName, placement.ComputeResourceName)
	vm.ClusterID = firstNonEmptyTrimmed(vm.ClusterID, placement.ClusterID)
	vm.ClusterName = firstNonEmptyTrimmed(vm.ClusterName, placement.ClusterName)
	vm.FolderID = firstNonEmptyTrimmed(vm.FolderID, placement.FolderID)
	vm.FolderName = firstNonEmptyTrimmed(vm.FolderName, placement.FolderName)
}

func applyPlacementToDatastore(datastore *InventoryDatastore, placement vmwarePlacement) {
	if datastore == nil {
		return
	}
	datastore.DatacenterID = firstNonEmptyTrimmed(datastore.DatacenterID, placement.DatacenterID)
	datastore.DatacenterName = firstNonEmptyTrimmed(datastore.DatacenterName, placement.DatacenterName)
	datastore.FolderID = firstNonEmptyTrimmed(datastore.FolderID, placement.FolderID)
	datastore.FolderName = firstNonEmptyTrimmed(datastore.FolderName, placement.FolderName)
}

func mergePlacement(dst *vmwarePlacement, src vmwarePlacement) {
	if dst == nil {
		return
	}
	if dst.DatacenterID == "" {
		dst.DatacenterID = strings.TrimSpace(src.DatacenterID)
	}
	if dst.DatacenterName == "" {
		dst.DatacenterName = strings.TrimSpace(src.DatacenterName)
	}
	if dst.ComputeResourceID == "" {
		dst.ComputeResourceID = strings.TrimSpace(src.ComputeResourceID)
	}
	if dst.ComputeResourceName == "" {
		dst.ComputeResourceName = strings.TrimSpace(src.ComputeResourceName)
	}
	if dst.ClusterID == "" {
		dst.ClusterID = strings.TrimSpace(src.ClusterID)
	}
	if dst.ClusterName == "" {
		dst.ClusterName = strings.TrimSpace(src.ClusterName)
	}
	if dst.FolderID == "" {
		dst.FolderID = strings.TrimSpace(src.FolderID)
	}
	if dst.FolderName == "" {
		dst.FolderName = strings.TrimSpace(src.FolderName)
	}
}

func idsForReferences(refs []viJSONReference) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if id := strings.TrimSpace(ref.Value); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueSortedTrimmedStrings(ids)
}

func namesForReferences(refs []viJSONReference, namesByID map[string]string) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		id := strings.TrimSpace(ref.Value)
		if id == "" {
			continue
		}
		name := firstNonEmptyTrimmed(namesByID[id], id)
		if name != "" {
			names = append(names, name)
		}
	}
	return uniqueSortedTrimmedStrings(names)
}

func cloneVIJSONReference(in *viJSONReference) *viJSONReference {
	if in == nil {
		return nil
	}
	out := *in
	out.Type = strings.TrimSpace(out.Type)
	out.Value = strings.TrimSpace(out.Value)
	return &out
}

func cloneVIJSONReferences(in []viJSONReference) []viJSONReference {
	if in == nil {
		return nil
	}
	out := make([]viJSONReference, 0, len(in))
	for _, ref := range in {
		trimmed := viJSONReference{
			Type:  strings.TrimSpace(ref.Type),
			Value: strings.TrimSpace(ref.Value),
		}
		if trimmed.Value == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func managedObjectKey(managedType, managedObjectID string) string {
	managedType = strings.TrimSpace(managedType)
	managedObjectID = strings.TrimSpace(managedObjectID)
	if managedType == "" || managedObjectID == "" {
		return ""
	}
	return managedType + "/" + managedObjectID
}

func uniqueSortedTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func isAutomationNotFound(err error) bool {
	connectionErr, ok := err.(*ConnectionError)
	return ok && connectionErr.Category == "not_found"
}

func isAutomationUnavailable(err error) bool {
	connectionErr, ok := err.(*ConnectionError)
	return ok && connectionErr.Category == "unavailable"
}
