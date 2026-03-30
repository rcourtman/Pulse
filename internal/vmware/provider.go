package vmware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// InventoryHost is the canonical phase-1 host summary returned by the vCenter
// Automation API list endpoint.
type InventoryHost struct {
	Host            string `json:"host"`
	Name            string `json:"name"`
	ConnectionState string `json:"connection_state"`
	PowerState      string `json:"power_state,omitempty"`
	HostUUID        string `json:"host_uuid,omitempty"`
}

// InventoryVM is the canonical phase-1 VM summary returned by the vCenter
// Automation API list endpoint.
type InventoryVM struct {
	VM            string `json:"vm"`
	Name          string `json:"name"`
	PowerState    string `json:"power_state"`
	CPUCount      int    `json:"cpu_count,omitempty"`
	MemorySizeMiB int64  `json:"memory_size_mib,omitempty"`
}

// InventoryDatastore is the canonical phase-1 datastore summary returned by
// the vCenter Automation API list endpoint.
type InventoryDatastore struct {
	Datastore string `json:"datastore"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	FreeSpace int64  `json:"free_space,omitempty"`
	Capacity  int64  `json:"capacity,omitempty"`
}

// InventorySnapshot captures the projected inventory floor for one vCenter
// connection at one point in time.
type InventorySnapshot struct {
	ConnectionID   string
	ConnectionName string
	VCenterHost    string
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

	for _, host := range snapshot.Hosts {
		name := firstNonEmptyTrimmed(host.Name, host.Host)
		if name == "" {
			continue
		}
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeAgent,
			Technology: "vmware",
			Name:       name,
			Status:     hostStatus(host),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			VMware: &unifiedresources.VMwareData{
				ConnectionID:    strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:  connectionName,
				VCenterHost:     vcenterHost,
				ManagedObjectID: strings.TrimSpace(host.Host),
				EntityType:      "host",
				HostUUID:        strings.TrimSpace(host.HostUUID),
				ConnectionState: strings.TrimSpace(host.ConnectionState),
				PowerState:      strings.TrimSpace(host.PowerState),
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
			DMIUUID:   strings.TrimSpace(host.HostUUID),
			Hostnames: filterNonEmptyStrings(name),
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
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeVM,
			Technology: "vmware",
			Name:       name,
			Status:     vmStatus(vm),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			VMware: &unifiedresources.VMwareData{
				ConnectionID:    strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:  connectionName,
				VCenterHost:     vcenterHost,
				ManagedObjectID: strings.TrimSpace(vm.VM),
				EntityType:      "vm",
				PowerState:      strings.TrimSpace(vm.PowerState),
				CPUCount:        vm.CPUCount,
				MemorySizeMiB:   vm.MemorySizeMiB,
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
		records = append(records, unifiedresources.IngestRecord{
			SourceID: vmwareSourceID(snapshot.ConnectionID, "vm", vm.VM),
			Resource: resource,
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
		resource := unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeStorage,
			Technology: "vmware",
			Name:       name,
			Status:     datastoreStatus(datastore),
			LastSeen:   collectedAt,
			UpdatedAt:  collectedAt,
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: diskMetric(datastore.Capacity, used),
			},
			Storage: &unifiedresources.StorageMeta{
				Type:     normalizeDatastoreType(datastore.Type),
				Platform: "vmware-vsphere",
				Topology: "datastore",
				Enabled:  true,
			},
			VMware: &unifiedresources.VMwareData{
				ConnectionID:    strings.TrimSpace(snapshot.ConnectionID),
				ConnectionName:  connectionName,
				VCenterHost:     vcenterHost,
				ManagedObjectID: strings.TrimSpace(datastore.Datastore),
				EntityType:      "datastore",
				DatastoreType:   strings.TrimSpace(datastore.Type),
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
	out.Hosts = append([]InventoryHost(nil), in.Hosts...)
	out.VMs = append([]InventoryVM(nil), in.VMs...)
	out.Datastores = append([]InventoryDatastore(nil), in.Datastores...)
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
	return unifiedresources.StatusOnline
}

func normalizeDatastoreType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
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
