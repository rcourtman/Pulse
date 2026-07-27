package monitoring

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AvailabilityProbeStatus captures the last observed state of an agentless
// endpoint probe.
type AvailabilityProbeStatus struct {
	TargetID            string    `json:"targetId"`
	Name                string    `json:"name"`
	TargetKind          string    `json:"targetKind,omitempty"`
	Address             string    `json:"address"`
	Protocol            string    `json:"protocol"`
	Outcome             string    `json:"outcome,omitempty"`
	Enabled             bool      `json:"enabled"`
	Available           bool      `json:"available"`
	LastChecked         time.Time `json:"lastChecked,omitempty"`
	LastSuccess         time.Time `json:"lastSuccess,omitempty"`
	LatencyMillis       int64     `json:"latencyMillis,omitempty"`
	ConsecutiveFailures int       `json:"consecutiveFailures,omitempty"`
	LastError           string    `json:"lastError,omitempty"`
	FailureThreshold    int       `json:"failureThreshold,omitempty"`
	ProbeAgentID        string    `json:"probeAgentId,omitempty"`
}

// AvailabilityProbeOutcome and its values are aliases for the shared probe
// package so existing monitoring and API callers keep their spelling.
type AvailabilityProbeOutcome = availabilityprobe.Outcome

const (
	AvailabilityProbeReachable     = availabilityprobe.OutcomeReachable
	AvailabilityProbeUnreachable   = availabilityprobe.OutcomeUnreachable
	AvailabilityProbeIndeterminate = availabilityprobe.OutcomeIndeterminate
)

type availabilityPollProvider struct{}

func newAvailabilityPollProvider() PollProvider {
	return availabilityPollProvider{}
}

func (availabilityPollProvider) Type() InstanceType {
	return InstanceTypeAvailability
}

func (availabilityPollProvider) ListInstances(m *Monitor) []string {
	targets := m.availabilityTargets()
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		// A licensed probe assignment moves execution to the agent. When the
		// entitlement lapses the effective assignment collapses to local, and
		// the next polling cycle re-plans this instance without a restart.
		if m.effectiveProbeAgentID(target) != "" {
			continue
		}
		names = append(names, target.ID)
	}
	sort.Strings(names)
	return names
}

func (availabilityPollProvider) BaseInterval(m *Monitor) time.Duration {
	targets := m.availabilityTargets()
	minInterval := time.Duration(config.DefaultAvailabilityPollIntervalSecs) * time.Second
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		interval := time.Duration(target.EffectivePollIntervalSecs()) * time.Second
		if interval > 0 && interval < minInterval {
			minInterval = interval
		}
	}
	return clampInterval(minInterval, 10*time.Second, time.Hour)
}

func (availabilityPollProvider) FixedInstanceInterval(m *Monitor, instanceName string) time.Duration {
	target, ok := m.availabilityTargetByID(instanceName)
	if !ok || !target.Enabled {
		return 0
	}
	return clampInterval(time.Duration(target.EffectivePollIntervalSecs())*time.Second, 10*time.Second, time.Hour)
}

func (availabilityPollProvider) BuildPollTask(m *Monitor, instanceName string) (PollTask, error) {
	target, ok := m.availabilityTargetByID(instanceName)
	if !ok || !target.Enabled {
		return PollTask{}, fmt.Errorf("availability target %q is not enabled", instanceName)
	}
	// Refuse the local run even for a queue entry scheduled before the target
	// was assigned, so a probe-assigned target never executes twice.
	if agentID := m.effectiveProbeAgentID(target); agentID != "" {
		return PollTask{}, fmt.Errorf("availability target %q is assigned to probe agent %q", instanceName, agentID)
	}
	return PollTask{
		InstanceName: target.ID,
		InstanceType: string(InstanceTypeAvailability),
		Run: func(ctx context.Context) {
			m.pollAvailabilityTarget(ctx, target)
		},
	}, nil
}

func (availabilityPollProvider) DescribeInstances(m *Monitor) []PollProviderInstanceInfo {
	targets := m.availabilityTargets()
	infos := make([]PollProviderInstanceInfo, 0, len(targets))
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		infos = append(infos, PollProviderInstanceInfo{
			Name:        target.ID,
			DisplayName: target.DisplayName(),
			Connection:  availabilityConnectionKey(target.ID),
			Metadata: map[string]string{
				"address":  target.Address,
				"protocol": string(target.Protocol),
			},
		})
	}
	return infos
}

func (availabilityPollProvider) ConnectionStatuses(m *Monitor) map[string]bool {
	statuses := m.availabilityStatusSnapshotForTargets(m.availabilityTargets(), time.Now())
	out := make(map[string]bool, len(statuses))
	for targetID, status := range statuses {
		out[availabilityConnectionKey(targetID)] = status.Enabled && status.Available
	}
	return out
}

func (availabilityPollProvider) ConnectionHealthKey(_ *Monitor, instanceName string) string {
	return availabilityConnectionKey(instanceName)
}

func (availabilityPollProvider) SupplementalSource() unifiedresources.DataSource {
	return unifiedresources.SourceAvailability
}

func (availabilityPollProvider) SupplementalRecords(m *Monitor, orgID string) []unifiedresources.IngestRecord {
	targets := m.availabilityTargets()
	now := time.Now().UTC()
	statuses := m.availabilityStatusSnapshotForTargets(targets, now)
	records := make([]unifiedresources.IngestRecord, 0, len(targets))
	for _, target := range targets {
		status := statuses[target.ID]
		if status.TargetID == "" {
			status = m.deriveAvailabilityProbeStaleness(target, availabilityStatusFromTarget(target), now)
		}
		resource, identity := availabilityResourceFromTarget(target, status, orgID, now)
		records = append(records, unifiedresources.IngestRecord{
			SourceID: target.ID,
			Resource: resource,
			Identity: identity,
		})
	}
	return records
}

func (m *Monitor) availabilityTargets() []config.AvailabilityTarget {
	if m == nil || m.configPersist == nil {
		return nil
	}
	targets, err := m.configPersist.LoadAvailabilityTargets()
	if err != nil {
		return nil
	}
	out := make([]config.AvailabilityTarget, 0, len(targets))
	for _, target := range targets {
		target = config.NormalizeAvailabilityTarget(target)
		if strings.TrimSpace(target.ID) == "" {
			continue
		}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(out[i].DisplayName())
		right := strings.ToLower(out[j].DisplayName())
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

func (m *Monitor) availabilityTargetByID(id string) (config.AvailabilityTarget, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return config.AvailabilityTarget{}, false
	}
	for _, target := range m.availabilityTargets() {
		if target.ID == id {
			return target, true
		}
	}
	return config.AvailabilityTarget{}, false
}

// AvailabilityStatusSnapshot is the single read path every availability
// consumer flows through, so probe staleness is derived here once instead of
// being re-implemented per reader.
func (m *Monitor) AvailabilityStatusSnapshot() map[string]AvailabilityProbeStatus {
	if m == nil {
		return nil
	}
	return m.availabilityStatusSnapshotForTargets(m.availabilityTargets(), time.Now())
}

func (m *Monitor) availabilityStatusSnapshotForTargets(targets []config.AvailabilityTarget, now time.Time) map[string]AvailabilityProbeStatus {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	out := make(map[string]AvailabilityProbeStatus, len(m.availabilityStatuses))
	for id, status := range m.availabilityStatuses {
		out[id] = status
	}
	m.mu.RUnlock()

	for _, target := range targets {
		status, ok := out[target.ID]
		if !ok {
			continue
		}
		out[target.ID] = m.deriveAvailabilityProbeStaleness(target, status, now)
	}
	return out
}

func (m *Monitor) RefreshAvailabilityTargets() {
	if m == nil {
		return
	}
	targets := m.availabilityTargets()
	activeIDs := make(map[string]struct{}, len(targets))
	now := time.Now()
	for _, target := range targets {
		activeIDs[target.ID] = struct{}{}
		if m.taskQueue == nil {
			continue
		}
		task := ScheduledTask{
			InstanceName: target.ID,
			InstanceType: InstanceTypeAvailability,
			NextRun:      now,
			Interval:     clampInterval(time.Duration(target.EffectivePollIntervalSecs())*time.Second, 10*time.Second, time.Hour),
		}
		if target.Enabled && m.effectiveProbeAgentID(target) == "" {
			m.taskQueue.Upsert(task)
		} else {
			m.taskQueue.Remove(InstanceTypeAvailability, target.ID)
			m.removeProviderConnectionHealth(InstanceTypeAvailability, target.ID)
		}
	}

	removedIDs := make([]string, 0)
	m.mu.Lock()
	for id := range m.availabilityStatuses {
		if _, ok := activeIDs[id]; !ok {
			delete(m.availabilityStatuses, id)
			removedIDs = append(removedIDs, id)
		}
	}
	m.mu.Unlock()
	for _, id := range removedIDs {
		m.removeProviderConnectionHealth(InstanceTypeAvailability, id)
	}

	m.refreshInstanceInfoCacheFromProviders()
	m.updateResourceStore(m.GetState())
}

func (m *Monitor) pollAvailabilityTarget(ctx context.Context, target config.AvailabilityTarget) {
	target = config.NormalizeAvailabilityTarget(target)
	start := time.Now()
	outcome, err := ProbeAvailabilityTargetResult(ctx, target)
	latency := time.Since(start)
	checkedAt := time.Now().UTC()
	m.applyAvailabilityObservation(target, checkedAt, latency, outcome, err, "")
	m.updateResourceStore(m.GetState())
}

// applyAvailabilityObservation records one availability observation regardless of
// where it executed, so remote probe results and local polls share failure
// accounting, connection health, and task bookkeeping.
func (m *Monitor) applyAvailabilityObservation(
	target config.AvailabilityTarget,
	checkedAt time.Time,
	latency time.Duration,
	outcome AvailabilityProbeOutcome,
	probeErr error,
	probeAgentID string,
) {
	m.setAvailabilityStatus(target, checkedAt, latency, outcome, probeErr, probeAgentID)

	if probeErr == nil {
		if m.stalenessTracker != nil {
			m.stalenessTracker.UpdateSuccess(InstanceTypeAvailability, target.ID, nil)
		}
		m.setProviderConnectionHealth(InstanceTypeAvailability, target.ID, true)
	} else {
		if m.stalenessTracker != nil {
			m.stalenessTracker.UpdateSuccess(InstanceTypeAvailability, target.ID, []byte(probeErr.Error()))
		}
		m.setProviderConnectionHealth(InstanceTypeAvailability, target.ID, false)
	}
	m.recordTaskResult(InstanceTypeAvailability, target.ID, nil)
}

func (m *Monitor) setAvailabilityStatus(target config.AvailabilityTarget, checkedAt time.Time, latency time.Duration, outcome AvailabilityProbeOutcome, probeErr error, probeAgentID string) {
	if m == nil {
		return
	}
	status := availabilityStatusFromTarget(target)
	status.Outcome = string(outcome)
	status.LastChecked = checkedAt
	status.ProbeAgentID = strings.TrimSpace(probeAgentID)
	latencyMs := latency.Milliseconds()
	if probeErr == nil && latencyMs == 0 {
		latencyMs = 1
	}
	status.LatencyMillis = latencyMs
	if probeErr == nil {
		// An open-or-filtered UDP timeout is healthy probe execution but does
		// not prove endpoint reachability. Keep it non-failing without claiming
		// the endpoint is available.
		status.Available = outcome == AvailabilityProbeReachable
		if outcome == AvailabilityProbeReachable {
			status.LastSuccess = checkedAt
		}
	} else {
		status.Available = false
		status.LastError = probeErr.Error()
	}

	m.mu.Lock()
	if m.availabilityStatuses == nil {
		m.availabilityStatuses = make(map[string]AvailabilityProbeStatus)
	}
	if previous, ok := m.availabilityStatuses[target.ID]; ok {
		status.LastSuccess = previous.LastSuccess
		if probeErr == nil {
			status.ConsecutiveFailures = 0
			status.LastError = ""
			if outcome == AvailabilityProbeReachable {
				status.LastSuccess = checkedAt
			}
		} else {
			status.ConsecutiveFailures = previous.ConsecutiveFailures + 1
		}
	} else if probeErr != nil {
		status.ConsecutiveFailures = 1
	}
	m.availabilityStatuses[target.ID] = status
	m.mu.Unlock()
}

// ProbeAvailabilityTarget executes one agentless availability check. The probe
// execution core lives in internal/availabilityprobe so the host agent can run
// the same checks without importing the monitoring package; this wrapper keeps
// the historical monitoring entry point for existing callers.
func ProbeAvailabilityTarget(ctx context.Context, target config.AvailabilityTarget) error {
	return availabilityprobe.Run(ctx, target)
}

// ProbeAvailabilityTargetResult preserves UDP's open-or-filtered state rather
// than incorrectly claiming that a silent UDP endpoint was proven reachable.
func ProbeAvailabilityTargetResult(ctx context.Context, target config.AvailabilityTarget) (AvailabilityProbeOutcome, error) {
	return availabilityprobe.Result(ctx, target)
}

func availabilityStatusFromTarget(target config.AvailabilityTarget) AvailabilityProbeStatus {
	return AvailabilityProbeStatus{
		TargetID:         target.ID,
		Name:             target.DisplayName(),
		TargetKind:       string(target.TargetKind),
		Address:          target.Address,
		Protocol:         string(target.Protocol),
		Enabled:          target.Enabled,
		FailureThreshold: target.EffectiveFailureThreshold(),
	}
}

func availabilityResourceFromTarget(target config.AvailabilityTarget, status AvailabilityProbeStatus, _ string, now time.Time) (unifiedresources.Resource, unifiedresources.ResourceIdentity) {
	lastSeen := status.LastChecked
	if lastSeen.IsZero() {
		lastSeen = now
	}
	resourceStatus := availabilityResourceStatus(target, status)
	data := &unifiedresources.AvailabilityData{
		TargetID:            target.ID,
		LinkedResourceID:    strings.TrimSpace(target.LinkedResourceID),
		Name:                target.DisplayName(),
		TargetKind:          string(target.TargetKind),
		Address:             target.Address,
		Protocol:            string(target.Protocol),
		ProbeOutcome:        status.Outcome,
		UDPMode:             string(target.UDPMode),
		Port:                target.Port,
		Path:                target.Path,
		Enabled:             target.Enabled,
		Available:           status.Available,
		LastChecked:         timePointerIfSet(status.LastChecked),
		LastSuccess:         timePointerIfSet(status.LastSuccess),
		LatencyMillis:       status.LatencyMillis,
		ConsecutiveFailures: status.ConsecutiveFailures,
		LastError:           status.LastError,
		FailureThreshold:    target.EffectiveFailureThreshold(),
		PollIntervalSeconds: target.EffectivePollIntervalSecs(),
		TimeoutMillis:       target.EffectiveTimeoutMillis(),
	}
	data.Evidence = availabilityEvidenceEnvelope(target, status, lastSeen, now)
	resource := unifiedresources.Resource{
		Type:         unifiedresources.ResourceTypeNetworkEndpoint,
		Technology:   string(target.Protocol),
		Name:         target.DisplayName(),
		Status:       resourceStatus,
		LastSeen:     lastSeen,
		UpdatedAt:    now,
		Sources:      []unifiedresources.DataSource{unifiedresources.SourceAvailability},
		Tags:         availabilityResourceTags(target),
		Availability: data,
	}
	if incident := availabilityIncident(target, status, lastSeen); incident != nil {
		resource.Incidents = []unifiedresources.ResourceIncident{*incident}
	}

	identity := unifiedresources.ResourceIdentity{}
	if ip := net.ParseIP(target.ProbeAddress()); ip != nil {
		identity.IPAddresses = []string{ip.String()}
	} else if host := target.ProbeAddress(); host != "" {
		identity.Hostnames = []string{host}
	}
	return resource, identity
}

func availabilityEvidenceEnvelope(
	target config.AvailabilityTarget,
	status AvailabilityProbeStatus,
	observedAt time.Time,
	ingestedAt time.Time,
) *operationaltrust.EvidenceEnvelope {
	if observedAt.IsZero() {
		return nil
	}
	if ingestedAt.IsZero() {
		ingestedAt = observedAt
	}

	source := operationaltrust.EvidenceSource{
		Provider:  string(unifiedresources.SourceAvailability),
		Collector: "availability-poller",
	}
	subject := operationaltrust.EvidenceSubject{
		ProviderRef:   target.ID,
		ProviderScope: "availability-target",
	}
	evidenceID, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		observedAt,
		target.ID,
	)
	if err != nil {
		return nil
	}

	validUntil := observedAt.Add(
		time.Duration(target.EffectivePollIntervalSecs()*2) * time.Second,
	)
	completeness := operationaltrust.EvidenceComplete
	confidence := operationaltrust.EvidenceConfirmed
	var reason *operationaltrust.EvidenceReason
	if status.LastChecked.IsZero() {
		completeness = operationaltrust.EvidencePartial
		confidence = operationaltrust.EvidenceUnknown
		reason = &operationaltrust.EvidenceReason{
			Code:    "availability_not_observed",
			Message: "The availability target has not completed its first probe.",
		}
	}

	envelope := operationaltrust.EvidenceEnvelope{
		ID:           evidenceID,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   ingestedAt,
		ValidUntil:   &validUntil,
		Completeness: completeness,
		Confidence:   confidence,
		Reason:       reason,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
		PayloadRef: &operationaltrust.EvidencePayloadRef{
			Kind: "availability-target",
			ID:   target.ID,
		},
	}
	if err := envelope.Validate(); err != nil {
		return nil
	}
	return &envelope
}

func availabilityResourceTags(target config.AvailabilityTarget) []string {
	tags := []string{"agentless"}
	if target.TargetKind != "" {
		tags = append(tags, string(target.TargetKind))
	}
	return tags
}

func availabilityResourceStatus(target config.AvailabilityTarget, status AvailabilityProbeStatus) unifiedresources.ResourceStatus {
	if !target.Enabled {
		return unifiedresources.StatusUnknown
	}
	if status.LastChecked.IsZero() {
		return unifiedresources.StatusUnknown
	}
	if status.Available {
		return unifiedresources.StatusOnline
	}
	if status.ConsecutiveFailures >= target.EffectiveFailureThreshold() {
		return unifiedresources.StatusOffline
	}
	return unifiedresources.StatusWarning
}

func availabilityIncident(target config.AvailabilityTarget, status AvailabilityProbeStatus, startedAt time.Time) *unifiedresources.ResourceIncident {
	if !target.Enabled || status.Available || status.LastChecked.IsZero() {
		return nil
	}
	if status.ConsecutiveFailures < target.EffectiveFailureThreshold() {
		return nil
	}
	summary := fmt.Sprintf("%s is unreachable by %s probe", target.DisplayName(), strings.ToUpper(string(target.Protocol)))
	if status.LastError != "" {
		summary = summary + ": " + status.LastError
	}
	return &unifiedresources.ResourceIncident{
		Provider:  string(unifiedresources.SourceAvailability),
		NativeID:  target.ID,
		Code:      "availability_unreachable",
		Severity:  storagehealth.RiskCritical,
		Source:    string(unifiedresources.SourceAvailability),
		Summary:   summary,
		StartedAt: startedAt,
	}
}

func availabilityConnectionKey(targetID string) string {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return ""
	}
	return "availability-" + targetID
}
