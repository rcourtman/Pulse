package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

const defaultMockPlatformPollIntervalSeconds = 60

func mockTrueNASConnectionResponses() []trueNASConnectionResponse {
	fixtures := truenas.DefaultFixtures()
	collectedAt := fixtures.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = fixtures.System.CollectedAt
	}

	instance := config.TrueNASInstance{
		ID:               "truenas-mock-1",
		Name:             "Archive NAS",
		Host:             strings.TrimSpace(fixtures.System.Hostname),
		Port:             443,
		APIKey:           "mock-truenas-api-key",
		UseHTTPS:         true,
		Enabled:          true,
		PollIntervalSecs: defaultMockPlatformPollIntervalSeconds,
	}
	instance.ApplyDefaults()

	observed := &monitoring.TrueNASConnectionObservedSummary{
		Host:              strings.TrimSpace(fixtures.System.Hostname),
		ResourceID:        strings.TrimSpace(fixtures.System.Hostname),
		Systems:           1,
		StoragePools:      len(fixtures.Pools),
		Datasets:          len(fixtures.Datasets),
		Apps:              len(fixtures.Apps),
		Disks:             len(fixtures.Disks),
		RecoveryArtifacts: len(fixtures.ZFSSnapshots) + len(fixtures.ReplicationTasks),
		CollectedAt:       mockPlatformTimePointer(collectedAt),
	}

	return []trueNASConnectionResponse{{
		TrueNASInstance: instance.Redacted(),
		Poll: &monitoring.TrueNASConnectionPollStatus{
			IntervalSeconds: instance.EffectivePollIntervalSecs(),
			LastAttemptAt:   mockPlatformTimePointer(collectedAt),
			LastSuccessAt:   mockPlatformTimePointer(collectedAt),
		},
		Observed: observed,
	}}
}

func mockVMwareConnectionResponses() []vmwareConnectionResponse {
	snapshot := vmware.DefaultFixtures()
	collectedAt := snapshot.CollectedAt

	instance := config.VMwareVCenterInstance{
		ID:       strings.TrimSpace(snapshot.ConnectionID),
		Name:     strings.TrimSpace(snapshot.ConnectionName),
		Host:     strings.TrimSpace(snapshot.VCenterHost),
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "mock-vcenter-password",
		Enabled:  true,
	}
	instance.ApplyDefaults()

	return []vmwareConnectionResponse{{
		VMwareVCenterInstance: instance.Redacted(),
		Poll: &monitoring.VMwareConnectionPollStatus{
			IntervalSeconds: defaultMockPlatformPollIntervalSeconds,
			LastAttemptAt:   mockPlatformTimePointer(collectedAt),
			LastSuccessAt:   mockPlatformTimePointer(collectedAt),
		},
		Observed: &monitoring.VMwareConnectionObservedSummary{
			CollectedAt: mockPlatformTimePointer(collectedAt),
			Hosts:       len(snapshot.Hosts),
			VMs:         len(snapshot.VMs),
			Datastores:  len(snapshot.Datastores),
			VIRelease:   strings.TrimSpace(snapshot.VIRelease),
		},
	}}
}

func mockPlatformTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
