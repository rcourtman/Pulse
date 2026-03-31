package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func mockTrueNASConnectionResponses() []trueNASConnectionResponse {
	fixture := mock.DefaultTrueNASConnectionFixture()

	instance := config.TrueNASInstance{
		ID:               fixture.ID,
		Name:             fixture.Name,
		Host:             fixture.Host,
		Port:             fixture.Port,
		APIKey:           fixture.APIKey,
		UseHTTPS:         fixture.UseHTTPS,
		Enabled:          fixture.Enabled,
		PollIntervalSecs: fixture.PollIntervalSeconds,
	}
	instance.ApplyDefaults()

	observed := &monitoring.TrueNASConnectionObservedSummary{
		Host:              fixture.Host,
		ResourceID:        fixture.ResourceID,
		Systems:           fixture.Systems,
		StoragePools:      fixture.StoragePools,
		Datasets:          fixture.Datasets,
		Apps:              fixture.Apps,
		Disks:             fixture.Disks,
		RecoveryArtifacts: fixture.RecoveryArtifacts,
		CollectedAt:       mockPlatformTimePointer(fixture.CollectedAt),
	}

	return []trueNASConnectionResponse{{
		TrueNASInstance: instance.Redacted(),
		Poll: &monitoring.TrueNASConnectionPollStatus{
			IntervalSeconds: instance.EffectivePollIntervalSecs(),
			LastAttemptAt:   mockPlatformTimePointer(fixture.CollectedAt),
			LastSuccessAt:   mockPlatformTimePointer(fixture.CollectedAt),
		},
		Observed: observed,
	}}
}

func mockVMwareConnectionResponses() []vmwareConnectionResponse {
	fixture := mock.DefaultVMwareConnectionFixture()

	instance := config.VMwareVCenterInstance{
		ID:       fixture.ID,
		Name:     fixture.Name,
		Host:     fixture.Host,
		Port:     fixture.Port,
		Username: fixture.Username,
		Password: fixture.Password,
		Enabled:  fixture.Enabled,
	}
	instance.ApplyDefaults()

	return []vmwareConnectionResponse{{
		VMwareVCenterInstance: instance.Redacted(),
		Poll: &monitoring.VMwareConnectionPollStatus{
			IntervalSeconds: fixture.PollIntervalSeconds,
			LastAttemptAt:   mockPlatformTimePointer(fixture.CollectedAt),
			LastSuccessAt:   mockPlatformTimePointer(fixture.CollectedAt),
		},
		Observed: &monitoring.VMwareConnectionObservedSummary{
			CollectedAt: mockPlatformTimePointer(fixture.CollectedAt),
			Hosts:       fixture.Hosts,
			VMs:         fixture.VMs,
			Datastores:  fixture.Datastores,
			VIRelease:   fixture.VIRelease,
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
