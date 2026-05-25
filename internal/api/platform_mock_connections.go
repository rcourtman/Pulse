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
			Networks:    fixture.Networks,
			VIRelease:   fixture.VIRelease,
		},
	}}
}

func mockAvailabilityConnectionInputs() ([]config.AvailabilityTarget, map[string]monitoring.AvailabilityProbeStatus) {
	fixtures := mock.AvailabilityFixtures()
	targets := make([]config.AvailabilityTarget, 0, len(fixtures))
	statuses := make(map[string]monitoring.AvailabilityProbeStatus, len(fixtures))
	for _, fixture := range fixtures {
		target := mockAvailabilityConfigTarget(fixture.Target)
		if target.ID == "" {
			continue
		}
		targets = append(targets, target)
		statuses[target.ID] = mockAvailabilityProbeStatus(fixture, target)
	}
	return targets, statuses
}

func mergeAvailabilityTargets(existing, additions []config.AvailabilityTarget) []config.AvailabilityTarget {
	if len(additions) == 0 {
		return existing
	}
	out := make([]config.AvailabilityTarget, 0, len(existing)+len(additions))
	seen := make(map[string]struct{}, len(existing)+len(additions))
	for _, target := range existing {
		target = config.NormalizeAvailabilityTarget(target)
		if target.ID == "" {
			continue
		}
		seen[target.ID] = struct{}{}
		out = append(out, target)
	}
	for _, target := range additions {
		target = config.NormalizeAvailabilityTarget(target)
		if target.ID == "" {
			continue
		}
		if _, ok := seen[target.ID]; ok {
			continue
		}
		seen[target.ID] = struct{}{}
		out = append(out, target)
	}
	return out
}

func mergeAvailabilityStatuses(existing, additions map[string]monitoring.AvailabilityProbeStatus) map[string]monitoring.AvailabilityProbeStatus {
	if len(existing) == 0 && len(additions) == 0 {
		return map[string]monitoring.AvailabilityProbeStatus{}
	}
	out := make(map[string]monitoring.AvailabilityProbeStatus, len(existing)+len(additions))
	for id, status := range existing {
		if id != "" {
			out[id] = status
		}
	}
	for id, status := range additions {
		if id != "" {
			out[id] = status
		}
	}
	return out
}

func mockAvailabilityTargetResponses() []availabilityTargetResponse {
	fixtures := mock.AvailabilityFixtures()
	responses := make([]availabilityTargetResponse, 0, len(fixtures))
	for _, fixture := range fixtures {
		target := mockAvailabilityConfigTarget(fixture.Target)
		if target.ID == "" {
			continue
		}
		status := mockAvailabilityProbeStatus(fixture, target)
		responses = append(responses, availabilityTargetResponse{
			AvailabilityTarget: target,
			Status:             &status,
		})
	}
	return responses
}

func mockAvailabilityTestResponse(targetID string) (availabilityTestResponse, bool) {
	for _, fixture := range mock.AvailabilityFixtures() {
		target := mockAvailabilityConfigTarget(fixture.Target)
		if target.ID != targetID {
			continue
		}
		response := availabilityTestResponse{
			Success:       fixture.Available,
			LatencyMillis: fixture.LatencyMillis,
			Error:         fixture.LastError,
		}
		return response, true
	}
	return availabilityTestResponse{}, false
}

func mockAvailabilityConfigTarget(target mock.AvailabilityTargetFixture) config.AvailabilityTarget {
	return config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:               target.ID,
		Name:             target.Name,
		TargetKind:       config.AvailabilityTargetKind(target.TargetKind),
		Address:          target.Address,
		Protocol:         config.AvailabilityProbeProtocol(target.Protocol),
		Port:             target.Port,
		Path:             target.Path,
		Enabled:          target.Enabled,
		PollIntervalSecs: target.PollIntervalSecs,
		TimeoutMillis:    target.TimeoutMillis,
		FailureThreshold: target.FailureThreshold,
	})
}

func mockAvailabilityProbeStatus(fixture mock.AvailabilityFixture, target config.AvailabilityTarget) monitoring.AvailabilityProbeStatus {
	return monitoring.AvailabilityProbeStatus{
		TargetID:            target.ID,
		Name:                target.DisplayName(),
		TargetKind:          string(target.TargetKind),
		Address:             target.Address,
		Protocol:            string(target.Protocol),
		Enabled:             target.Enabled,
		Available:           fixture.Available,
		LastChecked:         fixture.LastChecked,
		LastSuccess:         fixture.LastSuccess,
		LatencyMillis:       fixture.LatencyMillis,
		ConsecutiveFailures: fixture.ConsecutiveFailures,
		LastError:           fixture.LastError,
		FailureThreshold:    target.EffectiveFailureThreshold(),
	}
}

func mockPlatformTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
