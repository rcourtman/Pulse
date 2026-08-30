package api

import (
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

// mockKeepsRealPolling reports whether the operator opted real PVE/PBS/PMG
// polling back in while mock mode is enabled. The monitor reads the same
// variable to decide whether to build real clients at all (see
// keepRealPollingInMockMode in internal/monitoring/monitor.go); the ledger
// needs the same answer to tell a source that mock mode suspended apart from
// one that is genuinely still waiting on its first poll.
func mockKeepsRealPolling() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("PULSE_MOCK_KEEP_REAL_POLLING"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// applyMockLedgerInputs shapes the connections aggregator inputs for mock mode.
//
// Mock mode is a clean room: real PVE/PBS/PMG clients are never initialised
// while it is on and the platform pollers do not run, so every configured real
// source can only sit at "awaiting first poll" forever. Leaving those rows in
// the ledger publishes real connection names and addresses through a payload
// that is otherwise entirely authored fixtures, and reports a collection state
// that mock mode itself suspended. /api/config/nodes already substitutes mock
// entries wholesale and rejects node mutations outright, so the ledger is held
// to the same rule. The one exception is the real-polling opt-in, where the
// configured sources genuinely do collect and belong in the ledger.
func applyMockLedgerInputs(inputs aggregatorInputs) aggregatorInputs {
	if !mockKeepsRealPolling() {
		inputs.pveInstances = nil
		inputs.pbsInstances = nil
		inputs.pmgInstances = nil
		inputs.vmwareInstances = nil
		inputs.vmwareSummaries = nil
		inputs.truenasInstances = nil
		inputs.truenasSummaries = nil
		inputs.availabilityTargets = nil
	}

	mockTargets, mockStatuses := mockAvailabilityConnectionInputs()
	inputs.availabilityTargets = mergeAvailabilityTargets(inputs.availabilityTargets, mockTargets)
	inputs.availabilityStatuses = mergeAvailabilityStatuses(inputs.availabilityStatuses, mockStatuses)

	// Mock vSphere/TrueNAS pollers feed the fabric but never persistence, so
	// without these the mock ledger has no platform source rows for the
	// machines those integrations monitor.
	if len(inputs.vmwareInstances) == 0 {
		inputs.vmwareInstances, inputs.vmwareSummaries = mockVMwareLedgerInputs()
	}
	if len(inputs.truenasInstances) == 0 {
		inputs.truenasInstances, inputs.truenasSummaries = mockTrueNASLedgerInputs()
	}
	return inputs
}

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
		Transport: &truenas.TransportStatus{
			Mode:             truenas.TransportJSONRPC,
			Endpoint:         "wss://truenas-main/api/current",
			TLS:              true,
			Connected:        true,
			AuthMechanism:    "api-key-plain",
			ApplianceVersion: "TrueNAS-SCALE-25.10.4",
			LastConnectedAt:  mockPlatformTimePointer(fixture.CollectedAt),
		},
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

// mockVMwareLedgerInputs adapts the mock vCenter fixture into the unified
// connections aggregator's instance+summary inputs so mock mode presents the
// same vSphere source row (with member hosts) a real deployment would.
func mockVMwareLedgerInputs() ([]config.VMwareVCenterInstance, map[string]monitoring.VMwareConnectionSummary) {
	responses := mockVMwareConnectionResponses()
	instances := make([]config.VMwareVCenterInstance, 0, len(responses))
	summaries := make(map[string]monitoring.VMwareConnectionSummary, len(responses))
	for _, response := range responses {
		instances = append(instances, response.VMwareVCenterInstance)
		summaries[response.VMwareVCenterInstance.ID] = monitoring.VMwareConnectionSummary{
			Poll:     response.Poll,
			Observed: response.Observed,
		}
	}
	return instances, summaries
}

// mockTrueNASLedgerInputs is the TrueNAS counterpart of
// mockVMwareLedgerInputs: one source row per mock NAS, no member composition
// because a TrueNAS connection monitors exactly one machine.
func mockTrueNASLedgerInputs() ([]config.TrueNASInstance, map[string]monitoring.TrueNASConnectionSummary) {
	responses := mockTrueNASConnectionResponses()
	instances := make([]config.TrueNASInstance, 0, len(responses))
	summaries := make(map[string]monitoring.TrueNASConnectionSummary, len(responses))
	for _, response := range responses {
		instances = append(instances, response.TrueNASInstance)
		summaries[response.TrueNASInstance.ID] = monitoring.TrueNASConnectionSummary{
			Poll:      response.Poll,
			Observed:  response.Observed,
			Transport: response.Transport,
		}
	}
	return instances, summaries
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
	configTarget := config.AvailabilityTarget{
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
	}
	if target.ID == "mock-availability-docker-frontend-service" {
		configTarget.ObservationLocationIDs = []string{
			config.AvailabilityObservationLocationLocal,
			config.AvailabilityAgentObservationLocationID("edge-london"),
		}
	}
	return config.NormalizeAvailabilityTarget(configTarget)
}

func mockAvailabilityProbeStatus(fixture mock.AvailabilityFixture, target config.AvailabilityTarget) monitoring.AvailabilityProbeStatus {
	status := monitoring.AvailabilityProbeStatus{
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
	if target.ID == "mock-availability-docker-frontend-service" {
		status.AggregateState = monitoring.AvailabilityAggregateDegraded
		status.Disagreement = true
		status.ExpectedLocations = 2
		status.ReportingLocations = 2
		status.Locations = []monitoring.AvailabilityObservationLocationStatus{
			{
				LocationID:    config.AvailabilityObservationLocationLocal,
				Kind:          "pulse",
				Outcome:       string(monitoring.AvailabilityProbeReachable),
				Available:     true,
				LastChecked:   fixture.LastChecked,
				LastSuccess:   fixture.LastSuccess,
				FreshnessAt:   fixture.LastChecked,
				LatencyMillis: fixture.LatencyMillis,
			},
			{
				LocationID:          config.AvailabilityAgentObservationLocationID("edge-london"),
				Kind:                "agent",
				ProbeAgentID:        "edge-london",
				Outcome:             string(monitoring.AvailabilityProbeUnreachable),
				Available:           false,
				LastChecked:         fixture.LastChecked,
				FreshnessAt:         fixture.LastChecked,
				ConsecutiveFailures: 2,
				LastError:           "connection timed out",
			},
		}
	}
	return status
}

func mockPlatformTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
