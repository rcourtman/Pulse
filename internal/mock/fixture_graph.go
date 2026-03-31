package mock

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

// FixtureGraph is the canonical mock runtime owner for snapshot-backed and
// provider-backed platform fixtures. All mock projections should derive from
// this graph rather than mixing independent snapshot and provider helpers.
type FixtureGraph struct {
	State            models.StateSnapshot
	AlertHistory     []models.Alert
	PlatformFixtures PlatformFixtures
}

func emptyFixtureGraph() FixtureGraph {
	return FixtureGraph{
		State: models.EmptyStateSnapshot(),
	}
}

func buildFixtureGraph(cfg MockConfig, now time.Time) FixtureGraph {
	state := GenerateMockData(cfg)
	state.LastUpdate = now

	return FixtureGraph{
		State:            state,
		AlertHistory:     GenerateAlertHistory(state.Nodes, state.VMs, state.Containers),
		PlatformFixtures: DefaultPlatformFixtures(),
	}
}

func cloneFixtureGraph(in FixtureGraph) FixtureGraph {
	return FixtureGraph{
		State:            cloneState(in.State),
		AlertHistory:     append([]models.Alert(nil), in.AlertHistory...),
		PlatformFixtures: clonePlatformFixtures(in.PlatformFixtures),
	}
}

func (g *FixtureGraph) UpdateMetrics(cfg MockConfig, now time.Time) {
	if g == nil {
		return
	}
	UpdateMetrics(&g.State, cfg)
	g.State.LastUpdate = now
}

func (g *FixtureGraph) UpdateAlertSnapshots(active []alerts.Alert, resolved []models.ResolvedAlert) {
	if g == nil {
		return
	}

	converted := make([]models.Alert, 0, len(active))
	for _, alert := range active {
		converted = append(converted, models.Alert{
			ID:           alert.ID,
			Type:         alert.Type,
			Level:        string(alert.Level),
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Node:         alert.Node,
			Instance:     alert.Instance,
			Message:      alert.Message,
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Acknowledged: alert.Acknowledged,
		})
	}

	g.State.ActiveAlerts = converted
	g.State.RecentlyResolved = append([]models.ResolvedAlert(nil), resolved...)
}

func CurrentFixtureGraph() FixtureGraph {
	if !IsMockEnabled() {
		return emptyFixtureGraph()
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	return cloneFixtureGraph(mockGraph)
}

func GetPlatformFixtures() PlatformFixtures {
	if !IsMockEnabled() {
		return DefaultPlatformFixtures()
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	return clonePlatformFixtures(mockGraph.PlatformFixtures)
}

func clonePlatformFixtures(in PlatformFixtures) PlatformFixtures {
	return PlatformFixtures{
		TrueNAS: cloneTrueNASFixtureSnapshot(in.TrueNAS),
		VMware:  cloneVMwareInventorySnapshot(in.VMware),
	}
}

func cloneTrueNASFixtureSnapshot(in truenas.FixtureSnapshot) truenas.FixtureSnapshot {
	out := in
	out.System = cloneTrueNASSystemInfo(in.System)
	out.Pools = append([]truenas.Pool(nil), in.Pools...)
	out.Datasets = append([]truenas.Dataset(nil), in.Datasets...)
	out.Disks = append([]truenas.Disk(nil), in.Disks...)
	out.Alerts = append([]truenas.Alert(nil), in.Alerts...)
	out.Apps = cloneTrueNASApps(in.Apps)
	out.ZFSSnapshots = cloneTrueNASZFSSnapshots(in.ZFSSnapshots)
	out.ReplicationTasks = cloneTrueNASReplicationTasks(in.ReplicationTasks)
	return out
}

func cloneTrueNASSystemInfo(in truenas.SystemInfo) truenas.SystemInfo {
	out := in
	if len(in.TemperatureCelsius) > 0 {
		out.TemperatureCelsius = make(map[string]float64, len(in.TemperatureCelsius))
		for key, value := range in.TemperatureCelsius {
			out.TemperatureCelsius[key] = value
		}
	}
	return out
}

func cloneTrueNASApps(in []truenas.App) []truenas.App {
	if in == nil {
		return nil
	}

	out := make([]truenas.App, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].UsedHostIPs = append([]string(nil), in[i].UsedHostIPs...)
		out[i].UsedPorts = cloneTrueNASAppPorts(in[i].UsedPorts)
		out[i].Containers = cloneTrueNASAppContainers(in[i].Containers)
		out[i].Volumes = append([]truenas.AppVolume(nil), in[i].Volumes...)
		out[i].Images = append([]string(nil), in[i].Images...)
		out[i].Networks = cloneTrueNASAppNetworks(in[i].Networks)
		out[i].Stats = cloneTrueNASAppStats(in[i].Stats)
	}

	return out
}

func cloneTrueNASAppPorts(in []truenas.AppPort) []truenas.AppPort {
	if in == nil {
		return nil
	}

	out := make([]truenas.AppPort, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].HostPorts = append([]truenas.AppHostPort(nil), in[i].HostPorts...)
	}
	return out
}

func cloneTrueNASAppContainers(in []truenas.AppContainer) []truenas.AppContainer {
	if in == nil {
		return nil
	}

	out := make([]truenas.AppContainer, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].PortConfig = cloneTrueNASAppPorts(in[i].PortConfig)
		out[i].VolumeMounts = append([]truenas.AppVolume(nil), in[i].VolumeMounts...)
	}
	return out
}

func cloneTrueNASAppNetworks(in []truenas.AppNetwork) []truenas.AppNetwork {
	if in == nil {
		return nil
	}

	out := make([]truenas.AppNetwork, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Labels = cloneStringMap(in[i].Labels)
	}
	return out
}

func cloneTrueNASAppStats(in *truenas.AppStats) *truenas.AppStats {
	if in == nil {
		return nil
	}

	out := *in
	out.Interfaces = append([]truenas.AppInterfaceStats(nil), in.Interfaces...)
	return &out
}

func cloneTrueNASZFSSnapshots(in []truenas.ZFSSnapshot) []truenas.ZFSSnapshot {
	if in == nil {
		return nil
	}

	out := make([]truenas.ZFSSnapshot, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].CreatedAt = cloneTimePtr(in[i].CreatedAt)
		out[i].UsedBytes = cloneInt64Ptr(in[i].UsedBytes)
		out[i].Referenced = cloneInt64Ptr(in[i].Referenced)
	}
	return out
}

func cloneTrueNASReplicationTasks(in []truenas.ReplicationTask) []truenas.ReplicationTask {
	if in == nil {
		return nil
	}

	out := make([]truenas.ReplicationTask, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].SourceDatasets = append([]string(nil), in[i].SourceDatasets...)
		out[i].LastRun = cloneTimePtr(in[i].LastRun)
	}
	return out
}

func cloneVMwareInventorySnapshot(in vmware.InventorySnapshot) vmware.InventorySnapshot {
	out := in
	out.Hosts = cloneVMwareInventoryHosts(in.Hosts)
	out.VMs = cloneVMwareInventoryVMs(in.VMs)
	out.Datastores = cloneVMwareInventoryDatastores(in.Datastores)
	out.EnrichmentIssues = append([]vmware.InventoryEnrichmentIssue(nil), in.EnrichmentIssues...)
	return out
}

func cloneVMwareInventoryHosts(in []vmware.InventoryHost) []vmware.InventoryHost {
	if in == nil {
		return nil
	}

	out := make([]vmware.InventoryHost, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].DatastoreIDs = append([]string(nil), in[i].DatastoreIDs...)
		out[i].DatastoreNames = append([]string(nil), in[i].DatastoreNames...)
		out[i].TriggeredAlarms = append([]vmware.InventoryAlarm(nil), in[i].TriggeredAlarms...)
		out[i].RecentTasks = append([]vmware.InventoryTask(nil), in[i].RecentTasks...)
		out[i].RecentEvents = append([]vmware.InventoryEvent(nil), in[i].RecentEvents...)
		out[i].Metrics = cloneVMwareInventoryMetrics(in[i].Metrics)
	}

	return out
}

func cloneVMwareInventoryVMs(in []vmware.InventoryVM) []vmware.InventoryVM {
	if in == nil {
		return nil
	}

	out := make([]vmware.InventoryVM, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].DatastoreIDs = append([]string(nil), in[i].DatastoreIDs...)
		out[i].DatastoreNames = append([]string(nil), in[i].DatastoreNames...)
		out[i].GuestIPAddresses = append([]string(nil), in[i].GuestIPAddresses...)
		out[i].TriggeredAlarms = append([]vmware.InventoryAlarm(nil), in[i].TriggeredAlarms...)
		out[i].RecentTasks = append([]vmware.InventoryTask(nil), in[i].RecentTasks...)
		out[i].RecentEvents = append([]vmware.InventoryEvent(nil), in[i].RecentEvents...)
		out[i].Metrics = cloneVMwareInventoryMetrics(in[i].Metrics)
	}

	return out
}

func cloneVMwareInventoryDatastores(in []vmware.InventoryDatastore) []vmware.InventoryDatastore {
	if in == nil {
		return nil
	}

	out := make([]vmware.InventoryDatastore, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].HostIDs = append([]string(nil), in[i].HostIDs...)
		out[i].HostNames = append([]string(nil), in[i].HostNames...)
		out[i].VMIDs = append([]string(nil), in[i].VMIDs...)
		out[i].VMNames = append([]string(nil), in[i].VMNames...)
		out[i].Accessible = cloneBoolPtr(in[i].Accessible)
		out[i].MultipleHostAccess = cloneBoolPtr(in[i].MultipleHostAccess)
		out[i].TriggeredAlarms = append([]vmware.InventoryAlarm(nil), in[i].TriggeredAlarms...)
		out[i].RecentTasks = append([]vmware.InventoryTask(nil), in[i].RecentTasks...)
		out[i].RecentEvents = append([]vmware.InventoryEvent(nil), in[i].RecentEvents...)
	}

	return out
}

func cloneVMwareInventoryMetrics(in *vmware.InventoryMetrics) *vmware.InventoryMetrics {
	if in == nil {
		return nil
	}

	out := *in
	out.CPUPercent = cloneFloat64Ptr(in.CPUPercent)
	out.MemoryPercent = cloneFloat64Ptr(in.MemoryPercent)
	out.MemoryUsedBytes = cloneInt64Ptr(in.MemoryUsedBytes)
	out.MemoryTotalBytes = cloneInt64Ptr(in.MemoryTotalBytes)
	out.NetInBytesPerSecond = cloneFloat64Ptr(in.NetInBytesPerSecond)
	out.NetOutBytesPerSecond = cloneFloat64Ptr(in.NetOutBytesPerSecond)
	out.DiskReadBytesPerSecond = cloneFloat64Ptr(in.DiskReadBytesPerSecond)
	out.DiskWriteBytesPerSecond = cloneFloat64Ptr(in.DiskWriteBytesPerSecond)
	return &out
}

func cloneFloat64Ptr(n *float64) *float64 {
	if n == nil {
		return nil
	}
	value := *n
	return &value
}
