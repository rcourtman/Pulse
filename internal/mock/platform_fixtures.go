package mock

import (
	"math"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

const DefaultPlatformPollIntervalSeconds = 60

type PlatformFixtures struct {
	TrueNAS truenas.FixtureSnapshot
	VMware  vmware.InventorySnapshot
}

type TrueNASConnectionFixture struct {
	ID                  string
	Name                string
	Host                string
	Port                int
	APIKey              string
	UseHTTPS            bool
	Enabled             bool
	PollIntervalSeconds int
	CollectedAt         time.Time
	ResourceID          string
	Systems             int
	StoragePools        int
	Datasets            int
	Apps                int
	Disks               int
	RecoveryArtifacts   int
}

type VMwareConnectionFixture struct {
	ID                  string
	Name                string
	Host                string
	Port                int
	Username            string
	Password            string
	Enabled             bool
	PollIntervalSeconds int
	CollectedAt         time.Time
	Hosts               int
	VMs                 int
	Datastores          int
	VIRelease           string
}

func defaultPlatformFixtures() PlatformFixtures {
	return PlatformFixtures{
		TrueNAS: truenas.DefaultFixtures(),
		VMware:  vmware.DefaultFixtures(),
	}
}

func rebasePlatformFixtures(fixtures PlatformFixtures, now time.Time) PlatformFixtures {
	target := now.UTC()
	return PlatformFixtures{
		TrueNAS: rebaseTrueNASPlatformFixture(refreshTrueNASPlatformFixture(fixtures.TrueNAS, target), target),
		VMware:  rebaseVMwarePlatformFixture(refreshVMwarePlatformFixture(fixtures.VMware, target), target),
	}
}

func DefaultTrueNASConnectionFixture() TrueNASConnectionFixture {
	return defaultTrueNASConnectionFixture(currentOrDefaultPlatformFixtures())
}

func DefaultVMwareConnectionFixture() VMwareConnectionFixture {
	fixtures := currentOrDefaultPlatformFixtures().VMware

	return VMwareConnectionFixture{
		ID:                  strings.TrimSpace(fixtures.ConnectionID),
		Name:                strings.TrimSpace(fixtures.ConnectionName),
		Host:                strings.TrimSpace(fixtures.VCenterHost),
		Port:                443,
		Username:            "administrator@vsphere.local",
		Password:            "mock-vcenter-password",
		Enabled:             true,
		PollIntervalSeconds: DefaultPlatformPollIntervalSeconds,
		CollectedAt:         fixtures.CollectedAt,
		Hosts:               len(fixtures.Hosts),
		VMs:                 len(fixtures.VMs),
		Datastores:          len(fixtures.Datastores),
		VIRelease:           strings.TrimSpace(fixtures.VIRelease),
	}
}

func defaultTrueNASConnectionFixture(fixtures PlatformFixtures) TrueNASConnectionFixture {
	snapshot := fixtures.TrueNAS
	collectedAt := trueNASCollectedAt(snapshot)
	host := strings.TrimSpace(snapshot.System.Hostname)

	return TrueNASConnectionFixture{
		ID:                  "truenas-mock-1",
		Name:                "Archive NAS",
		Host:                host,
		Port:                443,
		APIKey:              "mock-truenas-api-key",
		UseHTTPS:            true,
		Enabled:             true,
		PollIntervalSeconds: DefaultPlatformPollIntervalSeconds,
		CollectedAt:         collectedAt,
		ResourceID:          host,
		Systems:             1,
		StoragePools:        len(snapshot.Pools),
		Datasets:            len(snapshot.Datasets),
		Apps:                len(snapshot.Apps),
		Disks:               len(snapshot.Disks),
		RecoveryArtifacts:   len(snapshot.ZFSSnapshots) + len(snapshot.ReplicationTasks),
	}
}

func SupplementalRecords(source unifiedresources.DataSource) []unifiedresources.IngestRecord {
	fixtures := currentOrDefaultPlatformFixtures()
	switch normalizePlatformSource(source) {
	case unifiedresources.SourceTrueNAS:
		return truenas.FixtureRecords(fixtures.TrueNAS)
	case unifiedresources.SourceVMware:
		return vmware.FixtureRecords(fixtures.VMware)
	default:
		return nil
	}
}

func PlatformOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{
		unifiedresources.SourceTrueNAS,
		unifiedresources.SourceVMware,
	}
}

func UnifiedResourceSnapshot() ([]unifiedresources.Resource, time.Time) {
	if !IsMockEnabled() {
		return nil, time.Time{}
	}

	return CurrentFixtureGraph().UnifiedResourceSnapshot()
}

func (g FixtureGraph) UnifiedResourceSnapshot() ([]unifiedresources.Resource, time.Time) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(unifiedresources.SnapshotWithoutSources(g.State, PlatformOwnedSources()))

	for _, source := range PlatformOwnedSources() {
		records := g.SupplementalRecords(source)
		if len(records) == 0 {
			continue
		}
		registry.IngestRecords(source, records)
	}

	freshness := g.State.LastUpdate
	for _, candidate := range []time.Time{
		trueNASCollectedAt(g.PlatformFixtures.TrueNAS),
		g.PlatformFixtures.VMware.CollectedAt,
	} {
		if candidate.IsZero() {
			continue
		}
		if freshness.IsZero() || candidate.After(freshness) {
			freshness = candidate
		}
	}

	resources := registry.List()
	for i := range resources {
		if target := registry.MetricsTarget(resources[i].ID); target != nil {
			resources[i].MetricsTarget = target
		}
	}

	return resources, freshness
}

func (g FixtureGraph) SupplementalRecords(source unifiedresources.DataSource) []unifiedresources.IngestRecord {
	switch normalizePlatformSource(source) {
	case unifiedresources.SourceTrueNAS:
		return truenas.FixtureRecords(g.PlatformFixtures.TrueNAS)
	case unifiedresources.SourceVMware:
		return vmware.FixtureRecords(g.PlatformFixtures.VMware)
	default:
		return nil
	}
}

func trueNASCollectedAt(fixtures truenas.FixtureSnapshot) time.Time {
	if !fixtures.CollectedAt.IsZero() {
		return fixtures.CollectedAt
	}
	return fixtures.System.CollectedAt
}

func rebaseTrueNASPlatformFixture(snapshot truenas.FixtureSnapshot, target time.Time) truenas.FixtureSnapshot {
	out := cloneTrueNASFixtureSnapshot(snapshot)
	anchor := trueNASCollectedAt(snapshot)
	if anchor.IsZero() {
		anchor = target
	}
	shift := target.Sub(anchor)

	out.CollectedAt = target
	out.System.CollectedAt = shiftTime(snapshot.System.CollectedAt, shift, target)

	for i := range out.Alerts {
		out.Alerts[i].Datetime = shiftTime(snapshot.Alerts[i].Datetime, shift, target)
	}
	for i := range out.Apps {
		if out.Apps[i].Stats != nil {
			out.Apps[i].Stats.CollectedAt = shiftTime(snapshot.Apps[i].Stats.CollectedAt, shift, target)
		}
	}
	for i := range out.ZFSSnapshots {
		if snapshot.ZFSSnapshots[i].CreatedAt != nil {
			rebased := shiftTime(*snapshot.ZFSSnapshots[i].CreatedAt, shift, target)
			out.ZFSSnapshots[i].CreatedAt = &rebased
		}
	}
	for i := range out.ReplicationTasks {
		if snapshot.ReplicationTasks[i].LastRun != nil {
			rebased := shiftTime(*snapshot.ReplicationTasks[i].LastRun, shift, target)
			out.ReplicationTasks[i].LastRun = &rebased
		}
	}

	return out
}

func rebaseVMwarePlatformFixture(snapshot vmware.InventorySnapshot, target time.Time) vmware.InventorySnapshot {
	out := cloneVMwareInventorySnapshot(snapshot)
	anchor := snapshot.CollectedAt
	if anchor.IsZero() {
		anchor = target
	}
	shift := target.Sub(anchor)

	out.CollectedAt = target
	for i := range out.Hosts {
		rebaseVMwareAlarms(out.Hosts[i].TriggeredAlarms, snapshot.Hosts[i].TriggeredAlarms, shift, target)
		rebaseVMwareTasks(out.Hosts[i].RecentTasks, snapshot.Hosts[i].RecentTasks, shift, target)
		rebaseVMwareEvents(out.Hosts[i].RecentEvents, snapshot.Hosts[i].RecentEvents, shift, target)
	}
	for i := range out.VMs {
		rebaseVMwareAlarms(out.VMs[i].TriggeredAlarms, snapshot.VMs[i].TriggeredAlarms, shift, target)
		rebaseVMwareTasks(out.VMs[i].RecentTasks, snapshot.VMs[i].RecentTasks, shift, target)
		rebaseVMwareEvents(out.VMs[i].RecentEvents, snapshot.VMs[i].RecentEvents, shift, target)
	}
	for i := range out.Datastores {
		rebaseVMwareAlarms(out.Datastores[i].TriggeredAlarms, snapshot.Datastores[i].TriggeredAlarms, shift, target)
		rebaseVMwareTasks(out.Datastores[i].RecentTasks, snapshot.Datastores[i].RecentTasks, shift, target)
		rebaseVMwareEvents(out.Datastores[i].RecentEvents, snapshot.Datastores[i].RecentEvents, shift, target)
	}

	return out
}

func refreshTrueNASPlatformFixture(snapshot truenas.FixtureSnapshot, at time.Time) truenas.FixtureSnapshot {
	out := cloneTrueNASFixtureSnapshot(snapshot)
	hostname := strings.TrimSpace(out.System.Hostname)
	if hostname != "" {
		out.System.CPUPercent = SampleMetric("agent", hostname, "cpu", at)
		out.System.DiskReadRate = SampleMetric("agent", hostname, "diskread", at)
		out.System.DiskWriteRate = SampleMetric("agent", hostname, "diskwrite", at)
		out.System.NetInRate = SampleMetric("agent", hostname, "netin", at)
		out.System.NetOutRate = SampleMetric("agent", hostname, "netout", at)
		if out.System.MemoryTotalBytes > 0 {
			memoryPercent := SampleMetric("agent", hostname, "memory", at)
			usedBytes := bytesFromPercent(out.System.MemoryTotalBytes, memoryPercent)
			out.System.MemoryAvailableBytes = out.System.MemoryTotalBytes - usedBytes
			if out.System.MemoryAvailableBytes < 0 {
				out.System.MemoryAvailableBytes = 0
			}
		}
	}

	for i := range out.Pools {
		usage := SampleMetric("storage", "pool:"+strings.TrimSpace(out.Pools[i].Name), "usage", at)
		applyTrueNASCapacityUsage(&out.Pools[i].UsedBytes, &out.Pools[i].FreeBytes, out.Pools[i].TotalBytes, usage)
	}

	for i := range out.Datasets {
		totalBytes := out.Datasets[i].UsedBytes + out.Datasets[i].AvailBytes
		usage := SampleMetric("storage", "dataset:"+strings.TrimSpace(out.Datasets[i].Name), "usage", at)
		usedBytes := bytesFromPercent(totalBytes, usage)
		out.Datasets[i].UsedBytes = usedBytes
		out.Datasets[i].AvailBytes = totalBytes - usedBytes
		if out.Datasets[i].AvailBytes < 0 {
			out.Datasets[i].AvailBytes = 0
		}
	}

	for i := range out.Disks {
		resourceID := trueNASDiskMetricID(out.Disks[i])
		if resourceID == "" {
			continue
		}
		out.Disks[i].Temperature = int(math.Round(SampleMetric("disk", resourceID, "smart_temp", at)))
		refreshTrueNASDiskAggregate(&out.Disks[i].TemperatureAggregate, out.Disks[i].Temperature)
	}

	for i := range out.Apps {
		stats := out.Apps[i].Stats
		if stats == nil {
			continue
		}

		appID := strings.TrimSpace(out.Apps[i].ID)
		if appID == "" {
			appID = strings.TrimSpace(out.Apps[i].Name)
		}
		if appID == "" {
			continue
		}

		stats.CollectedAt = at
		stopped := strings.EqualFold(strings.TrimSpace(out.Apps[i].State), "stopped")
		if stopped {
			stats.CPUPercent = 0
			stats.NetInRate = 0
			stats.NetOutRate = 0
			stats.DiskReadRate = 0
			stats.DiskWriteRate = 0
			stats.BlockReadBytes = 0
			stats.BlockWriteBytes = 0
			if out.System.MemoryTotalBytes > 0 {
				idlePercent := clampFloat(SampleMetric("dockerContainer", appID, "memory", at)*0.12, 0.5, 8)
				stats.MemoryBytes = bytesFromPercent(out.System.MemoryTotalBytes, idlePercent)
			}
		} else {
			stats.CPUPercent = SampleMetric("dockerContainer", appID, "cpu", at)
			stats.NetInRate = SampleMetric("dockerContainer", appID, "netin", at)
			stats.NetOutRate = SampleMetric("dockerContainer", appID, "netout", at)
			stats.DiskReadRate = SampleMetric("dockerContainer", appID, "diskread", at)
			stats.DiskWriteRate = SampleMetric("dockerContainer", appID, "diskwrite", at)
			stats.BlockReadBytes = int64(math.Round(stats.DiskReadRate * math.Max(1, float64(stats.IntervalSeconds))))
			stats.BlockWriteBytes = int64(math.Round(stats.DiskWriteRate * math.Max(1, float64(stats.IntervalSeconds))))
			if out.System.MemoryTotalBytes > 0 {
				memoryPercent := SampleMetric("dockerContainer", appID, "memory", at)
				stats.MemoryBytes = bytesFromPercent(out.System.MemoryTotalBytes, memoryPercent)
			}
		}
		refreshTrueNASAppInterfaces(stats)
	}

	return out
}

func refreshVMwarePlatformFixture(snapshot vmware.InventorySnapshot, at time.Time) vmware.InventorySnapshot {
	out := cloneVMwareInventorySnapshot(snapshot)
	for i := range out.Hosts {
		refreshVMwareInventoryMetrics(out.Hosts[i].Metrics, "agent", vmware.SourceID(out.ConnectionID, "host", out.Hosts[i].Host), at)
	}
	for i := range out.VMs {
		resourceID := vmware.SourceID(out.ConnectionID, "vm", out.VMs[i].VM)
		refreshVMwareInventoryMetrics(out.VMs[i].Metrics, "vm", resourceID, at)
		if strings.EqualFold(strings.TrimSpace(out.VMs[i].PowerState), "powered_off") && out.VMs[i].Metrics != nil {
			*out.VMs[i].Metrics.CPUPercent = 0
			*out.VMs[i].Metrics.NetInBytesPerSecond = 0
			*out.VMs[i].Metrics.NetOutBytesPerSecond = 0
			*out.VMs[i].Metrics.DiskReadBytesPerSecond = 0
			*out.VMs[i].Metrics.DiskWriteBytesPerSecond = 0
		}
	}
	for i := range out.Datastores {
		total := out.Datastores[i].Capacity
		if total <= 0 {
			continue
		}
		usage := SampleMetric("storage", vmware.SourceID(out.ConnectionID, "datastore", out.Datastores[i].Datastore), "usage", at)
		used := bytesFromPercent(total, usage)
		out.Datastores[i].FreeSpace = total - used
		if out.Datastores[i].FreeSpace < 0 {
			out.Datastores[i].FreeSpace = 0
		}
	}
	return out
}

func applyTrueNASCapacityUsage(usedBytes *int64, freeBytes *int64, totalBytes int64, usagePercent float64) {
	if usedBytes == nil || freeBytes == nil || totalBytes <= 0 {
		return
	}
	used := bytesFromPercent(totalBytes, usagePercent)
	*usedBytes = used
	*freeBytes = totalBytes - used
	if *freeBytes < 0 {
		*freeBytes = 0
	}
}

func bytesFromPercent(totalBytes int64, usagePercent float64) int64 {
	if totalBytes <= 0 {
		return 0
	}
	usage := clampFloat(usagePercent, 0, 100)
	used := int64(math.Round((float64(totalBytes) * usage) / 100.0))
	if used < 0 {
		return 0
	}
	if used > totalBytes {
		return totalBytes
	}
	return used
}

func refreshTrueNASDiskAggregate(aggregate *truenas.DiskTemperatureAggregate, current int) {
	if aggregate == nil {
		return
	}
	currentFloat := float64(current)
	if aggregate.WindowDays <= 0 {
		aggregate.WindowDays = 7
	}
	if aggregate.MinCelsius <= 0 || aggregate.MinCelsius > currentFloat {
		aggregate.MinCelsius = math.Max(25, currentFloat-4)
	}
	if aggregate.MaxCelsius <= 0 || aggregate.MaxCelsius < currentFloat {
		aggregate.MaxCelsius = math.Min(95, currentFloat+5)
	}
	aggregate.AvgCelsius = clampFloat(currentFloat-0.8, aggregate.MinCelsius, aggregate.MaxCelsius)
}

func trueNASDiskMetricID(disk truenas.Disk) string {
	resourceID := strings.TrimSpace(disk.Serial)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.ID)
	}
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.Name)
	}
	return resourceID
}

func refreshTrueNASAppInterfaces(stats *truenas.AppStats) {
	if stats == nil || len(stats.Interfaces) == 0 {
		return
	}

	var totalRx float64
	var totalTx float64
	for _, iface := range stats.Interfaces {
		totalRx += iface.RxBytesPS
		totalTx += iface.TxBytesPS
	}

	for i := range stats.Interfaces {
		rxShare := 1 / float64(len(stats.Interfaces))
		txShare := 1 / float64(len(stats.Interfaces))
		if totalRx > 0 {
			rxShare = stats.Interfaces[i].RxBytesPS / totalRx
		}
		if totalTx > 0 {
			txShare = stats.Interfaces[i].TxBytesPS / totalTx
		}
		stats.Interfaces[i].RxBytesPS = stats.NetInRate * rxShare
		stats.Interfaces[i].TxBytesPS = stats.NetOutRate * txShare
	}
}

func refreshVMwareInventoryMetrics(metrics *vmware.InventoryMetrics, resourceClass, resourceID string, at time.Time) {
	if metrics == nil || strings.TrimSpace(resourceID) == "" {
		return
	}

	memoryTotal := int64(0)
	if metrics.MemoryTotalBytes != nil {
		memoryTotal = *metrics.MemoryTotalBytes
	}

	*ensureFloat64Ptr(&metrics.CPUPercent) = SampleMetric(resourceClass, resourceID, "cpu", at)
	*ensureFloat64Ptr(&metrics.MemoryPercent) = SampleMetric(resourceClass, resourceID, "memory", at)
	*ensureFloat64Ptr(&metrics.NetInBytesPerSecond) = SampleMetric(resourceClass, resourceID, "netin", at)
	*ensureFloat64Ptr(&metrics.NetOutBytesPerSecond) = SampleMetric(resourceClass, resourceID, "netout", at)
	*ensureFloat64Ptr(&metrics.DiskReadBytesPerSecond) = SampleMetric(resourceClass, resourceID, "diskread", at)
	*ensureFloat64Ptr(&metrics.DiskWriteBytesPerSecond) = SampleMetric(resourceClass, resourceID, "diskwrite", at)
	if memoryTotal > 0 {
		*ensureInt64Ptr(&metrics.MemoryUsedBytes) = bytesFromPercent(memoryTotal, *metrics.MemoryPercent)
	}
}

func ensureFloat64Ptr(target **float64) *float64 {
	if *target == nil {
		*target = new(float64)
	}
	return *target
}

func ensureInt64Ptr(target **int64) *int64 {
	if *target == nil {
		*target = new(int64)
	}
	return *target
}

func rebaseVMwareAlarms(out []vmware.InventoryAlarm, in []vmware.InventoryAlarm, shift time.Duration, target time.Time) {
	for i := range out {
		out[i].TriggeredAt = shiftTime(in[i].TriggeredAt, shift, target)
	}
}

func rebaseVMwareTasks(out []vmware.InventoryTask, in []vmware.InventoryTask, shift time.Duration, target time.Time) {
	for i := range out {
		out[i].StartedAt = shiftTime(in[i].StartedAt, shift, target)
		out[i].CompletedAt = shiftTime(in[i].CompletedAt, shift, target)
	}
}

func rebaseVMwareEvents(out []vmware.InventoryEvent, in []vmware.InventoryEvent, shift time.Duration, target time.Time) {
	for i := range out {
		out[i].CreatedAt = shiftTime(in[i].CreatedAt, shift, target)
	}
}

func shiftTime(value time.Time, shift time.Duration, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.Add(shift)
}

func normalizePlatformSource(source unifiedresources.DataSource) unifiedresources.DataSource {
	switch strings.ToLower(strings.TrimSpace(string(source))) {
	case "truenas":
		return unifiedresources.SourceTrueNAS
	case "vmware", "vmware-vsphere":
		return unifiedresources.SourceVMware
	default:
		return ""
	}
}
