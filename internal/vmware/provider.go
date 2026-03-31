package vmware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type InventoryAlarm struct {
	Alarm         string    `json:"alarm,omitempty"`
	Name          string    `json:"name,omitempty"`
	OverallStatus string    `json:"overall_status,omitempty"`
	Acknowledged  bool      `json:"acknowledged,omitempty"`
	TriggeredAt   time.Time `json:"triggered_at,omitempty"`
}

type InventoryTask struct {
	Task          string    `json:"task,omitempty"`
	Name          string    `json:"name,omitempty"`
	State         string    `json:"state,omitempty"`
	DescriptionID string    `json:"description_id,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

type InventoryEvent struct {
	Event     string    `json:"event,omitempty"`
	Type      string    `json:"type,omitempty"`
	Message   string    `json:"message,omitempty"`
	User      string    `json:"user,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// InventoryMetrics captures the current runtime metric floor projected onto
// canonical Pulse metrics for VMware-backed hosts and VMs.
type InventoryMetrics struct {
	CPUPercent              *float64 `json:"cpu_percent,omitempty"`
	MemoryPercent           *float64 `json:"memory_percent,omitempty"`
	MemoryUsedBytes         *int64   `json:"memory_used_bytes,omitempty"`
	MemoryTotalBytes        *int64   `json:"memory_total_bytes,omitempty"`
	NetInBytesPerSecond     *float64 `json:"net_in_bytes_per_second,omitempty"`
	NetOutBytesPerSecond    *float64 `json:"net_out_bytes_per_second,omitempty"`
	DiskReadBytesPerSecond  *float64 `json:"disk_read_bytes_per_second,omitempty"`
	DiskWriteBytesPerSecond *float64 `json:"disk_write_bytes_per_second,omitempty"`
}

// InventoryHost is the canonical phase-1 host summary returned by the vCenter
// Automation API list endpoint.
type InventoryHost struct {
	Host                string            `json:"host"`
	Name                string            `json:"name"`
	ConnectionState     string            `json:"connection_state"`
	PowerState          string            `json:"power_state,omitempty"`
	HostUUID            string            `json:"host_uuid,omitempty"`
	DatacenterID        string            `json:"datacenter_id,omitempty"`
	DatacenterName      string            `json:"datacenter_name,omitempty"`
	ComputeResourceID   string            `json:"compute_resource_id,omitempty"`
	ComputeResourceName string            `json:"compute_resource_name,omitempty"`
	ClusterID           string            `json:"cluster_id,omitempty"`
	ClusterName         string            `json:"cluster_name,omitempty"`
	FolderID            string            `json:"folder_id,omitempty"`
	FolderName          string            `json:"folder_name,omitempty"`
	DatastoreIDs        []string          `json:"datastore_ids,omitempty"`
	DatastoreNames      []string          `json:"datastore_names,omitempty"`
	OverallStatus       string            `json:"overall_status,omitempty"`
	TriggeredAlarms     []InventoryAlarm  `json:"triggered_alarms,omitempty"`
	RecentTasks         []InventoryTask   `json:"recent_tasks,omitempty"`
	RecentEvents        []InventoryEvent  `json:"recent_events,omitempty"`
	Metrics             *InventoryMetrics `json:"metrics,omitempty"`
}

// InventoryVM is the canonical phase-1 VM summary returned by the vCenter
// Automation API list endpoint.
type InventoryVM struct {
	VM                  string            `json:"vm"`
	Name                string            `json:"name"`
	PowerState          string            `json:"power_state"`
	CPUCount            int               `json:"cpu_count,omitempty"`
	MemorySizeMiB       int64             `json:"memory_size_mib,omitempty"`
	DatacenterID        string            `json:"datacenter_id,omitempty"`
	DatacenterName      string            `json:"datacenter_name,omitempty"`
	ComputeResourceID   string            `json:"compute_resource_id,omitempty"`
	ComputeResourceName string            `json:"compute_resource_name,omitempty"`
	ClusterID           string            `json:"cluster_id,omitempty"`
	ClusterName         string            `json:"cluster_name,omitempty"`
	FolderID            string            `json:"folder_id,omitempty"`
	FolderName          string            `json:"folder_name,omitempty"`
	ResourcePoolID      string            `json:"resource_pool_id,omitempty"`
	ResourcePoolName    string            `json:"resource_pool_name,omitempty"`
	RuntimeHostID       string            `json:"runtime_host_id,omitempty"`
	RuntimeHostName     string            `json:"runtime_host_name,omitempty"`
	DatastoreIDs        []string          `json:"datastore_ids,omitempty"`
	DatastoreNames      []string          `json:"datastore_names,omitempty"`
	InstanceUUID        string            `json:"instance_uuid,omitempty"`
	BIOSUUID            string            `json:"bios_uuid,omitempty"`
	GuestOSFamily       string            `json:"guest_os_family,omitempty"`
	GuestHostname       string            `json:"guest_hostname,omitempty"`
	GuestIPAddresses    []string          `json:"guest_ip_addresses,omitempty"`
	OverallStatus       string            `json:"overall_status,omitempty"`
	TriggeredAlarms     []InventoryAlarm  `json:"triggered_alarms,omitempty"`
	RecentTasks         []InventoryTask   `json:"recent_tasks,omitempty"`
	RecentEvents        []InventoryEvent  `json:"recent_events,omitempty"`
	SnapshotCount       int               `json:"snapshot_count,omitempty"`
	Metrics             *InventoryMetrics `json:"metrics,omitempty"`
}

// InventoryDatastore is the canonical phase-1 datastore summary returned by
// the vCenter Automation API list endpoint.
type InventoryDatastore struct {
	Datastore          string           `json:"datastore"`
	Name               string           `json:"name"`
	Type               string           `json:"type"`
	FreeSpace          int64            `json:"free_space,omitempty"`
	Capacity           int64            `json:"capacity,omitempty"`
	DatacenterID       string           `json:"datacenter_id,omitempty"`
	DatacenterName     string           `json:"datacenter_name,omitempty"`
	FolderID           string           `json:"folder_id,omitempty"`
	FolderName         string           `json:"folder_name,omitempty"`
	HostIDs            []string         `json:"host_ids,omitempty"`
	HostNames          []string         `json:"host_names,omitempty"`
	VMIDs              []string         `json:"vm_ids,omitempty"`
	VMNames            []string         `json:"vm_names,omitempty"`
	Accessible         *bool            `json:"accessible,omitempty"`
	MultipleHostAccess *bool            `json:"multiple_host_access,omitempty"`
	MaintenanceMode    string           `json:"maintenance_mode,omitempty"`
	URL                string           `json:"url,omitempty"`
	OverallStatus      string           `json:"overall_status,omitempty"`
	TriggeredAlarms    []InventoryAlarm `json:"triggered_alarms,omitempty"`
	RecentTasks        []InventoryTask  `json:"recent_tasks,omitempty"`
	RecentEvents       []InventoryEvent `json:"recent_events,omitempty"`
}

// InventorySnapshot captures the projected inventory floor for one vCenter
// connection at one point in time.
type InventorySnapshot struct {
	ConnectionID   string
	ConnectionName string
	VCenterHost    string
	VIRelease      string
	CollectedAt    time.Time
	Hosts          []InventoryHost
	VMs            []InventoryVM
	Datastores     []InventoryDatastore
}

// ProviderMetadata carries operator-owned vCenter connection labels onto the
// projected resource graph.
type ProviderMetadata struct {
	ConnectionID   string
	ConnectionName string
	VCenterHost    string
}

// Fetcher loads a VMware inventory snapshot from a concrete source.
type Fetcher interface {
	Fetch(ctx context.Context) (*InventorySnapshot, error)
}

type fetcherCloser interface {
	Close()
}

// APIFetcher loads inventory from the live VMware client.
type APIFetcher struct {
	Client   *Client
	Metadata ProviderMetadata
}

// Fetch implements Fetcher.
func (f *APIFetcher) Fetch(ctx context.Context) (*InventorySnapshot, error) {
	if f == nil || f.Client == nil {
		return nil, fmt.Errorf("vmware api fetcher client is nil")
	}
	snapshot, err := f.Client.CollectInventory(ctx)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("vmware api fetcher returned nil inventory")
	}
	snapshot.ConnectionID = strings.TrimSpace(f.Metadata.ConnectionID)
	snapshot.ConnectionName = strings.TrimSpace(f.Metadata.ConnectionName)
	snapshot.VCenterHost = strings.TrimSpace(f.Metadata.VCenterHost)
	return snapshot, nil
}

// Close releases idle resources held by the underlying VMware client.
func (f *APIFetcher) Close() {
	if f == nil || f.Client == nil {
		return
	}
	f.Client.Close()
}

// FixtureFetcher loads inventory from static fixtures for tests.
type FixtureFetcher struct {
	Snapshot InventorySnapshot
}

// Fetch implements Fetcher.
func (f *FixtureFetcher) Fetch(context.Context) (*InventorySnapshot, error) {
	if f == nil {
		return nil, nil
	}
	return cloneInventorySnapshot(&f.Snapshot), nil
}

// Provider converts VMware inventory snapshots into unified resources.
type Provider struct {
	fetcher      Fetcher
	lastSnapshot *InventorySnapshot
	mu           sync.Mutex
	now          func() time.Time
}

// NewLiveProvider returns a provider backed by a concrete fetcher.
func NewLiveProvider(fetcher Fetcher) *Provider {
	return &Provider{
		fetcher: fetcher,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// NewAPIProvider returns a provider backed by the live VMware API client.
func NewAPIProvider(metadata ProviderMetadata, client *Client) *Provider {
	return NewLiveProvider(&APIFetcher{
		Client:   client,
		Metadata: metadata,
	})
}

// NewProvider returns a fixture-backed provider.
func NewProvider(snapshot InventorySnapshot) *Provider {
	if snapshot.CollectedAt.IsZero() {
		snapshot.CollectedAt = time.Now().UTC()
	}
	provider := NewLiveProvider(&FixtureFetcher{Snapshot: snapshot})
	provider.lastSnapshot = cloneInventorySnapshot(&snapshot)
	return provider
}

// Refresh fetches and caches the latest snapshot.
func (p *Provider) Refresh(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("vmware provider is nil")
	}
	if p.fetcher == nil {
		return fmt.Errorf("vmware provider fetcher is nil")
	}

	snapshot, err := p.fetcher.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("refresh vmware inventory: %w", err)
	}
	if snapshot == nil {
		return fmt.Errorf("vmware provider fetcher returned nil inventory")
	}

	sortInventorySnapshot(snapshot)

	p.mu.Lock()
	p.lastSnapshot = cloneInventorySnapshot(snapshot)
	p.mu.Unlock()
	return nil
}

// Close releases resources held by the active fetcher, if supported.
func (p *Provider) Close() {
	if p == nil || p.fetcher == nil {
		return
	}
	if closer, ok := p.fetcher.(fetcherCloser); ok {
		closer.Close()
	}
}

// Snapshot returns a defensive copy of the cached inventory snapshot.
func (p *Provider) Snapshot() *InventorySnapshot {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	snapshot := cloneInventorySnapshot(p.lastSnapshot)
	p.mu.Unlock()
	return snapshot
}

// Records returns canonical VMware unified resources if the integration is enabled.
func (p *Provider) Records() []unifiedresources.IngestRecord {
	if p == nil || !IsFeatureEnabled() {
		return nil
	}

	snapshot := p.Snapshot()
	if snapshot == nil {
		return nil
	}

	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = p.now()
	}

	connectionName := firstNonEmptyTrimmed(snapshot.ConnectionName, snapshot.VCenterHost, snapshot.ConnectionID)
	vcenterHost := strings.TrimSpace(snapshot.VCenterHost)
	records := make([]unifiedresources.IngestRecord, 0, len(snapshot.Hosts)+len(snapshot.VMs)+len(snapshot.Datastores))
	hostSourceIDsByManagedObject := make(map[string]string, len(snapshot.Hosts))
	for _, host := range snapshot.Hosts {
		hostID := strings.TrimSpace(host.Host)
		if hostID == "" {
			continue
		}
		hostSourceIDsByManagedObject[hostID] = vmwareSourceID(snapshot.ConnectionID, "host", hostID)
	}

	for _, host := range snapshot.Hosts {
		name := firstNonEmptyTrimmed(host.Name, host.Host)
		if name == "" {
			continue
		}
		incidents := hostIncidents(host)
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeAgent,
			Technology: "vmware",
			Name:       name,
			Status:     unifiedresources.IncidentsStatus(hostStatus(host), incidents),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			Incidents:  incidents,
			Metrics:    inventoryMetricsResourceMetrics(host.Metrics),
			VMware: &unifiedresources.VMwareData{
				ConnectionID:        strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:      connectionName,
				VCenterHost:         vcenterHost,
				ManagedObjectID:     strings.TrimSpace(host.Host),
				EntityType:          "host",
				HostUUID:            strings.TrimSpace(host.HostUUID),
				DatacenterID:        strings.TrimSpace(host.DatacenterID),
				DatacenterName:      strings.TrimSpace(host.DatacenterName),
				ComputeResourceID:   strings.TrimSpace(host.ComputeResourceID),
				ComputeResourceName: strings.TrimSpace(host.ComputeResourceName),
				ClusterID:           strings.TrimSpace(host.ClusterID),
				ClusterName:         strings.TrimSpace(host.ClusterName),
				FolderID:            strings.TrimSpace(host.FolderID),
				FolderName:          strings.TrimSpace(host.FolderName),
				ConnectionState:     strings.TrimSpace(host.ConnectionState),
				PowerState:          strings.TrimSpace(host.PowerState),
				OverallStatus:       strings.TrimSpace(host.OverallStatus),
				DatastoreIDs:        cloneStringSlice(host.DatastoreIDs),
				DatastoreNames:      cloneStringSlice(host.DatastoreNames),
				ActiveAlarmCount:    len(host.TriggeredAlarms),
				ActiveAlarmSummary:  vmwareAlarmSummary(host.TriggeredAlarms),
				RecentTaskCount:     len(host.RecentTasks),
				RecentTaskSummary:   vmwareRecentTaskSummary(host.RecentTasks),
			},
			Tags: filterNonEmptyStrings(
				"vmware",
				"vsphere",
				"host",
				"source:vcenter",
				tagWithValue("connection", strings.ToLower(connectionName)),
				tagWithValue("power", strings.ToLower(strings.TrimSpace(host.PowerState))),
				tagWithValue("state", strings.ToLower(strings.TrimSpace(host.ConnectionState))),
			),
		}
		identity := unifiedresources.ResourceIdentity{
			DMIUUID:     strings.TrimSpace(host.HostUUID),
			Hostnames:   filterNonEmptyStrings(name),
			ClusterName: vmwareClusterHint(host.ClusterName, host.ComputeResourceName),
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID: vmwareSourceID(snapshot.ConnectionID, "host", host.Host),
			Resource: resource,
			Identity: identity,
		})
	}

	for _, vm := range snapshot.VMs {
		name := firstNonEmptyTrimmed(vm.Name, vm.VM)
		if name == "" {
			continue
		}
		incidents := vmIncidents(vm)
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeVM,
			Technology: "vmware",
			Name:       name,
			Status:     unifiedresources.IncidentsStatus(vmStatus(vm), incidents),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			Incidents:  incidents,
			Metrics:    inventoryMetricsResourceMetrics(vm.Metrics),
			ParentName: strings.TrimSpace(vm.RuntimeHostName),
			VMware: &unifiedresources.VMwareData{
				ConnectionID:        strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:      connectionName,
				VCenterHost:         vcenterHost,
				ManagedObjectID:     strings.TrimSpace(vm.VM),
				EntityType:          "vm",
				DatacenterID:        strings.TrimSpace(vm.DatacenterID),
				DatacenterName:      strings.TrimSpace(vm.DatacenterName),
				ComputeResourceID:   strings.TrimSpace(vm.ComputeResourceID),
				ComputeResourceName: strings.TrimSpace(vm.ComputeResourceName),
				ClusterID:           strings.TrimSpace(vm.ClusterID),
				ClusterName:         strings.TrimSpace(vm.ClusterName),
				FolderID:            strings.TrimSpace(vm.FolderID),
				FolderName:          strings.TrimSpace(vm.FolderName),
				ResourcePoolID:      strings.TrimSpace(vm.ResourcePoolID),
				ResourcePoolName:    strings.TrimSpace(vm.ResourcePoolName),
				RuntimeHostID:       strings.TrimSpace(vm.RuntimeHostID),
				RuntimeHostName:     strings.TrimSpace(vm.RuntimeHostName),
				PowerState:          strings.TrimSpace(vm.PowerState),
				CPUCount:            vm.CPUCount,
				MemorySizeMiB:       vm.MemorySizeMiB,
				DatastoreIDs:        cloneStringSlice(vm.DatastoreIDs),
				DatastoreNames:      cloneStringSlice(vm.DatastoreNames),
				InstanceUUID:        strings.TrimSpace(vm.InstanceUUID),
				BIOSUUID:            strings.TrimSpace(vm.BIOSUUID),
				GuestOSFamily:       strings.TrimSpace(vm.GuestOSFamily),
				GuestHostname:       strings.TrimSpace(vm.GuestHostname),
				GuestIPAddresses:    cloneStringSlice(vm.GuestIPAddresses),
				OverallStatus:       strings.TrimSpace(vm.OverallStatus),
				ActiveAlarmCount:    len(vm.TriggeredAlarms),
				ActiveAlarmSummary:  vmwareAlarmSummary(vm.TriggeredAlarms),
				RecentTaskCount:     len(vm.RecentTasks),
				RecentTaskSummary:   vmwareRecentTaskSummary(vm.RecentTasks),
				SnapshotCount:       vm.SnapshotCount,
			},
			Tags: filterNonEmptyStrings(
				"vmware",
				"vsphere",
				"vm",
				"source:vcenter",
				tagWithValue("connection", strings.ToLower(connectionName)),
				tagWithValue("power", strings.ToLower(strings.TrimSpace(vm.PowerState))),
			),
		}
		identity := unifiedresources.ResourceIdentity{
			MachineID:   firstNonEmptyTrimmed(vm.InstanceUUID, vm.BIOSUUID),
			Hostnames:   uniqueSortedTrimmedStrings([]string{name, vm.GuestHostname}),
			IPAddresses: uniqueSortedTrimmedStrings(vm.GuestIPAddresses),
			ClusterName: vmwareClusterHint(vm.ClusterName, vm.ComputeResourceName),
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID:       vmwareSourceID(snapshot.ConnectionID, "vm", vm.VM),
			ParentSourceID: hostSourceIDsByManagedObject[strings.TrimSpace(vm.RuntimeHostID)],
			Resource:       resource,
			Identity:       identity,
		})
	}

	for _, datastore := range snapshot.Datastores {
		name := firstNonEmptyTrimmed(datastore.Name, datastore.Datastore)
		if name == "" {
			continue
		}
		used := datastore.Capacity - datastore.FreeSpace
		if used < 0 {
			used = 0
		}
		incidents := datastoreIncidents(datastore)
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeStorage,
			Technology: "vmware",
			Name:       name,
			Status:     unifiedresources.IncidentsStatus(datastoreStatus(datastore), incidents),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			Incidents:  incidents,
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: diskMetric(datastore.Capacity, used),
			},
			Storage: &unifiedresources.StorageMeta{
				Type:          normalizeDatastoreType(datastore.Type),
				Platform:      "vmware-vsphere",
				Topology:      "datastore",
				Enabled:       vmwareDatastoreEnabled(datastore),
				Active:        vmwareDatastoreActive(datastore),
				Shared:        vmwareDatastoreShared(datastore),
				Nodes:         cloneStringSlice(datastore.HostNames),
				ConsumerCount: len(datastore.VMNames),
				ConsumerTypes: vmwareDatastoreConsumerTypes(datastore),
				TopConsumers:  vmwareDatastoreTopConsumers(datastore),
			},
			VMware: &unifiedresources.VMwareData{
				ConnectionID:        strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:      connectionName,
				VCenterHost:         vcenterHost,
				ManagedObjectID:     strings.TrimSpace(datastore.Datastore),
				EntityType:          "datastore",
				DatacenterID:        strings.TrimSpace(datastore.DatacenterID),
				DatacenterName:      strings.TrimSpace(datastore.DatacenterName),
				FolderID:            strings.TrimSpace(datastore.FolderID),
				FolderName:          strings.TrimSpace(datastore.FolderName),
				DatastoreType:       strings.TrimSpace(datastore.Type),
				DatastoreURL:        strings.TrimSpace(datastore.URL),
				DatastoreAccessible: cloneBoolPointer(datastore.Accessible),
				MultipleHostAccess:  cloneBoolPointer(datastore.MultipleHostAccess),
				MaintenanceMode:     strings.TrimSpace(datastore.MaintenanceMode),
				OverallStatus:       strings.TrimSpace(datastore.OverallStatus),
				ActiveAlarmCount:    len(datastore.TriggeredAlarms),
				ActiveAlarmSummary:  vmwareAlarmSummary(datastore.TriggeredAlarms),
				RecentTaskCount:     len(datastore.RecentTasks),
				RecentTaskSummary:   vmwareRecentTaskSummary(datastore.RecentTasks),
			},
			Tags: filterNonEmptyStrings(
				"vmware",
				"vsphere",
				"datastore",
				"source:vcenter",
				tagWithValue("connection", strings.ToLower(connectionName)),
				tagWithValue("type", strings.ToLower(strings.TrimSpace(datastore.Type))),
			),
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID: vmwareSourceID(snapshot.ConnectionID, "datastore", datastore.Datastore),
			Resource: resource,
		})
	}

	return records
}

func sortInventorySnapshot(snapshot *InventorySnapshot) {
	if snapshot == nil {
		return
	}
	sort.Slice(snapshot.Hosts, func(i, j int) bool {
		return vmwareSortKey(snapshot.Hosts[i].Host, snapshot.Hosts[i].Name) < vmwareSortKey(snapshot.Hosts[j].Host, snapshot.Hosts[j].Name)
	})
	sort.Slice(snapshot.VMs, func(i, j int) bool {
		return vmwareSortKey(snapshot.VMs[i].VM, snapshot.VMs[i].Name) < vmwareSortKey(snapshot.VMs[j].VM, snapshot.VMs[j].Name)
	})
	sort.Slice(snapshot.Datastores, func(i, j int) bool {
		return vmwareSortKey(snapshot.Datastores[i].Datastore, snapshot.Datastores[i].Name) < vmwareSortKey(snapshot.Datastores[j].Datastore, snapshot.Datastores[j].Name)
	})
}

func cloneInventorySnapshot(in *InventorySnapshot) *InventorySnapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.Hosts = cloneInventoryHosts(in.Hosts)
	out.VMs = cloneInventoryVMs(in.VMs)
	out.Datastores = cloneInventoryDatastores(in.Datastores)
	return &out
}

func cloneInventoryHosts(in []InventoryHost) []InventoryHost {
	if in == nil {
		return nil
	}
	out := make([]InventoryHost, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].DatastoreIDs = cloneStringSlice(in[i].DatastoreIDs)
		out[i].DatastoreNames = cloneStringSlice(in[i].DatastoreNames)
		out[i].TriggeredAlarms = cloneInventoryAlarms(in[i].TriggeredAlarms)
		out[i].RecentTasks = cloneInventoryTasks(in[i].RecentTasks)
		out[i].RecentEvents = cloneInventoryEvents(in[i].RecentEvents)
		out[i].Metrics = cloneInventoryMetrics(in[i].Metrics)
	}
	return out
}

func cloneInventoryVMs(in []InventoryVM) []InventoryVM {
	if in == nil {
		return nil
	}
	out := make([]InventoryVM, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].DatastoreIDs = cloneStringSlice(in[i].DatastoreIDs)
		out[i].DatastoreNames = cloneStringSlice(in[i].DatastoreNames)
		out[i].GuestIPAddresses = cloneStringSlice(in[i].GuestIPAddresses)
		out[i].TriggeredAlarms = cloneInventoryAlarms(in[i].TriggeredAlarms)
		out[i].RecentTasks = cloneInventoryTasks(in[i].RecentTasks)
		out[i].RecentEvents = cloneInventoryEvents(in[i].RecentEvents)
		out[i].Metrics = cloneInventoryMetrics(in[i].Metrics)
	}
	return out
}

func cloneInventoryDatastores(in []InventoryDatastore) []InventoryDatastore {
	if in == nil {
		return nil
	}
	out := make([]InventoryDatastore, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].HostIDs = cloneStringSlice(in[i].HostIDs)
		out[i].HostNames = cloneStringSlice(in[i].HostNames)
		out[i].VMIDs = cloneStringSlice(in[i].VMIDs)
		out[i].VMNames = cloneStringSlice(in[i].VMNames)
		out[i].Accessible = cloneBoolPointer(in[i].Accessible)
		out[i].MultipleHostAccess = cloneBoolPointer(in[i].MultipleHostAccess)
		out[i].TriggeredAlarms = cloneInventoryAlarms(in[i].TriggeredAlarms)
		out[i].RecentTasks = cloneInventoryTasks(in[i].RecentTasks)
		out[i].RecentEvents = cloneInventoryEvents(in[i].RecentEvents)
	}
	return out
}

func cloneInventoryAlarms(in []InventoryAlarm) []InventoryAlarm {
	if in == nil {
		return nil
	}
	out := make([]InventoryAlarm, len(in))
	copy(out, in)
	return out
}

func cloneInventoryTasks(in []InventoryTask) []InventoryTask {
	if in == nil {
		return nil
	}
	out := make([]InventoryTask, len(in))
	copy(out, in)
	return out
}

func cloneInventoryEvents(in []InventoryEvent) []InventoryEvent {
	if in == nil {
		return nil
	}
	out := make([]InventoryEvent, len(in))
	copy(out, in)
	return out
}

func cloneInventoryMetrics(in *InventoryMetrics) *InventoryMetrics {
	if in == nil {
		return nil
	}
	out := *in
	out.CPUPercent = cloneFloat64Pointer(in.CPUPercent)
	out.MemoryPercent = cloneFloat64Pointer(in.MemoryPercent)
	out.MemoryUsedBytes = cloneInt64Pointer(in.MemoryUsedBytes)
	out.MemoryTotalBytes = cloneInt64Pointer(in.MemoryTotalBytes)
	out.NetInBytesPerSecond = cloneFloat64Pointer(in.NetInBytesPerSecond)
	out.NetOutBytesPerSecond = cloneFloat64Pointer(in.NetOutBytesPerSecond)
	out.DiskReadBytesPerSecond = cloneFloat64Pointer(in.DiskReadBytesPerSecond)
	out.DiskWriteBytesPerSecond = cloneFloat64Pointer(in.DiskWriteBytesPerSecond)
	return &out
}

func vmwareSourceID(connectionID, entityType, managedObjectID string) string {
	parts := filterNonEmptyStrings(strings.TrimSpace(connectionID), strings.TrimSpace(entityType), strings.TrimSpace(managedObjectID))
	return strings.Join(parts, ":")
}

func hostStatus(host InventoryHost) unifiedresources.ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(host.ConnectionState)) {
	case "CONNECTED":
		switch strings.ToUpper(strings.TrimSpace(host.PowerState)) {
		case "", "POWERED_ON":
			return unifiedresources.StatusOnline
		case "POWERED_OFF":
			return unifiedresources.StatusOffline
		default:
			return unifiedresources.StatusWarning
		}
	case "DISCONNECTED", "NOT_RESPONDING":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func vmStatus(vm InventoryVM) unifiedresources.ResourceStatus {
	switch strings.ToUpper(strings.TrimSpace(vm.PowerState)) {
	case "POWERED_ON":
		return unifiedresources.StatusOnline
	case "POWERED_OFF", "SUSPENDED":
		return unifiedresources.StatusOffline
	default:
		return unifiedresources.StatusUnknown
	}
}

func datastoreStatus(datastore InventoryDatastore) unifiedresources.ResourceStatus {
	if strings.TrimSpace(datastore.Datastore) == "" && strings.TrimSpace(datastore.Name) == "" {
		return unifiedresources.StatusUnknown
	}
	if datastore.Accessible != nil && !*datastore.Accessible {
		return unifiedresources.StatusOffline
	}
	if mode := strings.ToLower(strings.TrimSpace(datastore.MaintenanceMode)); mode != "" && mode != "normal" {
		return unifiedresources.StatusWarning
	}
	return unifiedresources.StatusOnline
}

func hostIncidents(host InventoryHost) []unifiedresources.ResourceIncident {
	return appendVMwareAlarmsAndHealthIncidents("host", host.Host, strings.TrimSpace(host.OverallStatus), host.TriggeredAlarms)
}

func vmIncidents(vm InventoryVM) []unifiedresources.ResourceIncident {
	return appendVMwareAlarmsAndHealthIncidents("vm", vm.VM, strings.TrimSpace(vm.OverallStatus), vm.TriggeredAlarms)
}

func datastoreIncidents(datastore InventoryDatastore) []unifiedresources.ResourceIncident {
	return appendVMwareAlarmsAndHealthIncidents("datastore", datastore.Datastore, strings.TrimSpace(datastore.OverallStatus), datastore.TriggeredAlarms)
}

func appendVMwareAlarmsAndHealthIncidents(entityType, managedObjectID, overallStatus string, alarms []InventoryAlarm) []unifiedresources.ResourceIncident {
	incidents := make([]unifiedresources.ResourceIncident, 0, len(alarms)+1)
	for _, alarm := range alarms {
		severity, ok := vmwareRiskLevel(alarm.OverallStatus)
		if !ok {
			continue
		}
		nativeID := firstNonEmptyTrimmed(alarm.Alarm, alarm.Name, managedObjectID)
		summary := vmwareAlarmIncidentSummary(entityType, managedObjectID, alarm)
		startedAt := alarm.TriggeredAt
		incidents = append(incidents, unifiedresources.ResourceIncident{
			Provider:  "vmware",
			NativeID:  nativeID,
			Code:      "vmware_alarm_state",
			Severity:  severity,
			Source:    string(unifiedresources.SourceVMware),
			Summary:   summary,
			StartedAt: startedAt,
		})
	}
	if len(incidents) == 0 {
		if severity, ok := vmwareRiskLevel(overallStatus); ok {
			incidents = append(incidents, unifiedresources.ResourceIncident{
				Provider: "vmware",
				NativeID: firstNonEmptyTrimmed(managedObjectID, entityType),
				Code:     "vmware_health_state",
				Severity: severity,
				Source:   string(unifiedresources.SourceVMware),
				Summary:  vmwareOverallStatusSummary(entityType, overallStatus),
			})
		}
	}
	return incidents
}

func vmwareRiskLevel(status string) (storagehealth.RiskLevel, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "red":
		return storagehealth.RiskCritical, true
	case "yellow":
		return storagehealth.RiskWarning, true
	default:
		return "", false
	}
}

func vmwareAlarmIncidentSummary(entityType, managedObjectID string, alarm InventoryAlarm) string {
	entityLabel := vmwareEntityLabel(entityType)
	alarmName := firstNonEmptyTrimmed(alarm.Name, alarm.Alarm)
	status := strings.ToLower(strings.TrimSpace(alarm.OverallStatus))
	if alarmName == "" {
		alarmName = "VMware alarm"
	}
	if status == "" {
		status = "active"
	}
	if ref := strings.TrimSpace(managedObjectID); ref != "" {
		return fmt.Sprintf("%s %s has VMware alarm %s (%s)", entityLabel, ref, alarmName, status)
	}
	return fmt.Sprintf("%s has VMware alarm %s (%s)", entityLabel, alarmName, status)
}

func vmwareOverallStatusSummary(entityType, overallStatus string) string {
	entityLabel := vmwareEntityLabel(entityType)
	status := strings.ToLower(strings.TrimSpace(overallStatus))
	if status == "" {
		status = "degraded"
	}
	return fmt.Sprintf("%s has VMware overall status %s", entityLabel, status)
}

func vmwareEntityLabel(entityType string) string {
	switch strings.ToLower(strings.TrimSpace(entityType)) {
	case "host":
		return "Host"
	case "vm":
		return "VM"
	case "datastore":
		return "Datastore"
	default:
		return "Resource"
	}
}

func vmwareAlarmSummary(alarms []InventoryAlarm) string {
	if len(alarms) == 0 {
		return ""
	}
	parts := make([]string, 0, len(alarms))
	for _, alarm := range alarms {
		name := firstNonEmptyTrimmed(alarm.Name, alarm.Alarm)
		if name == "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(alarm.OverallStatus))
		if status == "" {
			parts = append(parts, name)
			continue
		}
		parts = append(parts, name+" ("+status+")")
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(alarms) > len(parts) {
		return strings.Join(parts, ", ") + fmt.Sprintf(", and %d more", len(alarms)-len(parts))
	}
	return strings.Join(parts, ", ")
}

func vmwareRecentTaskSummary(tasks []InventoryTask) string {
	if len(tasks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tasks))
	for _, task := range tasks {
		name := firstNonEmptyTrimmed(task.Name, task.DescriptionID, task.Task)
		if name == "" {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(task.State))
		if state == "" {
			parts = append(parts, name)
		} else {
			parts = append(parts, name+" ("+state+")")
		}
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(tasks) > len(parts) {
		return strings.Join(parts, ", ") + fmt.Sprintf(", and %d more", len(tasks)-len(parts))
	}
	return strings.Join(parts, ", ")
}

func normalizeDatastoreType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func vmwareClusterHint(clusterName, computeResourceName string) string {
	return firstNonEmptyTrimmed(clusterName, computeResourceName)
}

func vmwareDatastoreEnabled(datastore InventoryDatastore) bool {
	if datastore.Accessible == nil {
		return true
	}
	return *datastore.Accessible
}

func vmwareDatastoreActive(datastore InventoryDatastore) bool {
	if datastore.Accessible != nil && !*datastore.Accessible {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(datastore.MaintenanceMode))
	return mode == "" || mode == "normal"
}

func vmwareDatastoreShared(datastore InventoryDatastore) bool {
	if datastore.MultipleHostAccess != nil {
		return *datastore.MultipleHostAccess
	}
	return len(datastore.HostNames) > 1
}

func vmwareDatastoreConsumerTypes(datastore InventoryDatastore) []string {
	if len(datastore.VMNames) == 0 {
		return nil
	}
	return []string{string(unifiedresources.ResourceTypeVM)}
}

func vmwareDatastoreTopConsumers(datastore InventoryDatastore) []unifiedresources.StorageConsumerMeta {
	if len(datastore.VMNames) == 0 {
		return nil
	}
	consumers := make([]unifiedresources.StorageConsumerMeta, 0, len(datastore.VMNames))
	for _, name := range datastore.VMNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		consumer := unifiedresources.StorageConsumerMeta{
			ResourceType: unifiedresources.ResourceTypeVM,
			Name:         strings.TrimSpace(name),
		}
		consumers = append(consumers, consumer)
		if len(consumers) == 5 {
			break
		}
	}
	if len(consumers) == 0 {
		return nil
	}
	return consumers
}

func diskMetric(total, used int64) *unifiedresources.MetricValue {
	if total <= 0 {
		return nil
	}
	totalCopy := total
	usedCopy := used
	percent := (float64(used) / float64(total)) * 100
	return &unifiedresources.MetricValue{
		Total:   &totalCopy,
		Used:    &usedCopy,
		Value:   percent,
		Percent: percent,
		Unit:    "bytes",
	}
}

func inventoryMetricsResourceMetrics(in *InventoryMetrics) *unifiedresources.ResourceMetrics {
	if in == nil {
		return nil
	}

	metrics := &unifiedresources.ResourceMetrics{}
	if in.CPUPercent != nil {
		metrics.CPU = &unifiedresources.MetricValue{
			Value:   *in.CPUPercent,
			Percent: *in.CPUPercent,
			Unit:    "percent",
			Source:  unifiedresources.SourceVMware,
		}
	}
	if in.MemoryPercent != nil {
		metrics.Memory = &unifiedresources.MetricValue{
			Percent: *in.MemoryPercent,
			Unit:    "bytes",
			Source:  unifiedresources.SourceVMware,
		}
		if in.MemoryUsedBytes != nil {
			used := *in.MemoryUsedBytes
			metrics.Memory.Used = &used
		}
		if in.MemoryTotalBytes != nil {
			total := *in.MemoryTotalBytes
			metrics.Memory.Total = &total
		}
	}
	if in.NetInBytesPerSecond != nil {
		metrics.NetIn = &unifiedresources.MetricValue{
			Value:  *in.NetInBytesPerSecond,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceVMware,
		}
	}
	if in.NetOutBytesPerSecond != nil {
		metrics.NetOut = &unifiedresources.MetricValue{
			Value:  *in.NetOutBytesPerSecond,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceVMware,
		}
	}
	if in.DiskReadBytesPerSecond != nil {
		metrics.DiskRead = &unifiedresources.MetricValue{
			Value:  *in.DiskReadBytesPerSecond,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceVMware,
		}
	}
	if in.DiskWriteBytesPerSecond != nil {
		metrics.DiskWrite = &unifiedresources.MetricValue{
			Value:  *in.DiskWriteBytesPerSecond,
			Unit:   "bytes/s",
			Source: unifiedresources.SourceVMware,
		}
	}

	if metrics.CPU == nil &&
		metrics.Memory == nil &&
		metrics.NetIn == nil &&
		metrics.NetOut == nil &&
		metrics.DiskRead == nil &&
		metrics.DiskWrite == nil {
		return nil
	}
	return metrics
}

func cloneFloat64Pointer(in *float64) *float64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func cloneInt64Pointer(in *int64) *int64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func cloneBoolPointer(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func cloneStringSlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func filterNonEmptyStrings(values ...string) []string {
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
	return out
}

func tagWithValue(prefix, value string) string {
	prefix = strings.TrimSpace(prefix)
	value = strings.TrimSpace(value)
	if prefix == "" || value == "" {
		return ""
	}
	return prefix + ":" + value
}

func vmwareSortKey(id, name string) string {
	return firstNonEmptyTrimmed(strings.ToLower(strings.TrimSpace(id)), strings.ToLower(strings.TrimSpace(name)))
}
