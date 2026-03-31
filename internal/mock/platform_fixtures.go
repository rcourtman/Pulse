package mock

import (
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
	target := now.UTC().Truncate(time.Minute)
	return PlatformFixtures{
		TrueNAS: rebaseTrueNASPlatformFixture(fixtures.TrueNAS, target),
		VMware:  rebaseVMwarePlatformFixture(fixtures.VMware, target),
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
