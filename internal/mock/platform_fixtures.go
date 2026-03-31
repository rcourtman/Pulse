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

	return registry.List(), freshness
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
