package monitoring

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

// availabilityProbeStaleFloor is the shortest window after which a probe-assigned
// target without a fresh report reads as indeterminate.
const availabilityProbeStaleFloor = 5 * time.Minute

// availabilityProbeStaleError is the single read-time explanation shown when an
// assigned agent stops reporting.
const availabilityProbeStaleError = "no recent report from probe agent"

// ProbeAvailabilityResult is one availability observation reported by a remote
// host agent that owns the target's execution.
type ProbeAvailabilityResult struct {
	ObservationID  string
	TargetID       string
	ConfigRevision int64
	Outcome        availabilityprobe.Outcome
	LatencyMillis  int64
	CheckedAt      time.Time
	Error          string
	Certificate    *tlsutil.CertificateObservation
}

// availabilityProbeAssignmentTracker provides a grace reference for a newly
// assigned target before its first report. AgentID is retained so reassigning
// the same target to a different probe starts a fresh grace window.
type availabilityProbeAssignmentTracker struct {
	AgentID string
	Since   time.Time
}

// probeAvailabilityResultsFromReport converts the wire results carried by a
// host agent report. Outcome vocabulary is normalized here so the ingestion
// path never has to trust an agent's spelling; probeResultOutcome then decides
// what an unknown outcome means for failure accounting.
func probeAvailabilityResultsFromReport(reported []agentshost.AvailabilityProbeResult) []ProbeAvailabilityResult {
	if len(reported) == 0 {
		return nil
	}
	results := make([]ProbeAvailabilityResult, 0, len(reported))
	for _, entry := range reported {
		outcome := availabilityprobe.Outcome(strings.ToLower(strings.TrimSpace(entry.Outcome)))
		switch outcome {
		case availabilityprobe.OutcomeReachable, availabilityprobe.OutcomeUnreachable, availabilityprobe.OutcomeIndeterminate:
		default:
			outcome = availabilityprobe.OutcomeIndeterminate
		}
		results = append(results, ProbeAvailabilityResult{
			ObservationID:  strings.TrimSpace(entry.ObservationID),
			TargetID:       strings.TrimSpace(entry.TargetID),
			ConfigRevision: entry.ConfigRevision,
			Outcome:        outcome,
			LatencyMillis:  entry.LatencyMillis,
			CheckedAt:      entry.CheckedAt,
			Error:          strings.TrimSpace(entry.Error),
			Certificate:    entry.Certificate.Clone(),
		})
	}
	return results
}

// effectiveProbeAgentID returns the host agent that currently owns execution of
// the target. It collapses to local execution ("") whenever the external probe
// entitlement is absent, so a license lapse resumes local polling instead of
// stranding the check on an agent that is no longer allowed to run it.
func (m *Monitor) effectiveProbeAgentID(target config.AvailabilityTarget) string {
	assigned := strings.TrimSpace(target.ProbeAgentID)
	if assigned == "" {
		return ""
	}
	if !m.hasLicensedFeature(pkglicensing.FeatureExternalProbe) {
		return ""
	}
	return assigned
}

// ApplyProbeAvailabilityResults ingests availability results reported by a host
// agent. Results are accepted only for targets currently assigned to that agent.
func (m *Monitor) ApplyProbeAvailabilityResults(hostID string, results []ProbeAvailabilityResult) {
	m.applyProbeAvailabilityResultsAt(hostID, results, time.Now().UTC())
}

// applyProbeAvailabilityResultsAt keeps server receipt time explicit for the
// host-report path and deterministic tests. Agent-authored CheckedAt describes
// the observation; receivedAt is the authoritative signal that the probe is
// still reporting.
func (m *Monitor) applyProbeAvailabilityResultsAt(hostID string, results []ProbeAvailabilityResult, receivedAt time.Time) {
	if m == nil {
		return
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" || len(results) == 0 {
		return
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	} else {
		receivedAt = receivedAt.UTC()
	}

	applied := 0
	for _, result := range results {
		targetID := strings.TrimSpace(result.TargetID)
		if targetID == "" {
			continue
		}
		target, ok := m.availabilityTargetByID(targetID)
		if !ok {
			log.Debug().
				Str("hostID", hostID).
				Str("targetID", targetID).
				Msg("Rejecting probe availability result for unknown target")
			continue
		}
		if m.effectiveProbeAgentID(target) != hostID {
			log.Debug().
				Str("hostID", hostID).
				Str("targetID", targetID).
				Msg("Rejecting probe availability result from an agent that does not own the target")
			continue
		}
		if result.ConfigRevision > 0 && result.ConfigRevision != target.ConfigRevision {
			log.Debug().
				Str("hostID", hostID).
				Str("targetID", targetID).
				Int64("reportedRevision", result.ConfigRevision).
				Int64("currentRevision", target.ConfigRevision).
				Msg("Rejecting probe availability result for an obsolete configuration revision")
			continue
		}

		checkedAt := result.CheckedAt
		if checkedAt.IsZero() {
			checkedAt = receivedAt
		}
		latency := time.Duration(result.LatencyMillis) * time.Millisecond
		if latency < 0 {
			latency = 0
		}
		outcome, probeErr := probeResultOutcome(result)
		observationID := strings.TrimSpace(result.ObservationID)
		if observationID == "" {
			observationID = legacyProbeAvailabilityObservationID(hostID, result)
		}
		m.applyAvailabilityObservation(target, observationID, checkedAt.UTC(), latency, outcome, probeErr, result.Certificate, hostID, receivedAt)
		applied++
	}

	if applied == 0 {
		return
	}
	m.updateResourceStore(m.GetState())
}

func legacyProbeAvailabilityObservationID(hostID string, result ProbeAvailabilityResult) string {
	material := fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%d\x00%s",
		strings.TrimSpace(hostID), strings.TrimSpace(result.TargetID), result.CheckedAt.UTC().UnixNano(),
		strings.TrimSpace(string(result.Outcome)), result.LatencyMillis, strings.TrimSpace(result.Error))
	sum := sha256.Sum256([]byte(material))
	return fmt.Sprintf("legacy-agent-%x", sum[:])
}

// probeResultOutcome normalizes a reported outcome and derives the failure
// signal. An unreachable report without a message still has to fail, otherwise
// remote checks would never accumulate consecutive failures.
func probeResultOutcome(result ProbeAvailabilityResult) (AvailabilityProbeOutcome, error) {
	outcome := AvailabilityProbeOutcome(strings.ToLower(strings.TrimSpace(string(result.Outcome))))
	message := strings.TrimSpace(result.Error)
	switch outcome {
	case AvailabilityProbeReachable, AvailabilityProbeUnreachable, AvailabilityProbeIndeterminate:
	default:
		if message != "" {
			outcome = AvailabilityProbeUnreachable
		} else {
			outcome = AvailabilityProbeIndeterminate
		}
	}
	if outcome == AvailabilityProbeUnreachable {
		if message == "" {
			message = "probe agent reported the target unreachable"
		}
		return outcome, errors.New(message)
	}
	if message != "" {
		return AvailabilityProbeUnreachable, errors.New(message)
	}
	return outcome, nil
}

// deriveAvailabilityProbeStaleness reports the status a reader should see for a
// probe-assigned target. Stored state is never mutated: an agent that stops
// reporting must read as indeterminate without erasing its last observation.
func (m *Monitor) deriveAvailabilityProbeStaleness(
	target config.AvailabilityTarget,
	status AvailabilityProbeStatus,
	now time.Time,
) AvailabilityProbeStatus {
	agentID := m.effectiveProbeAgentID(target)
	if agentID == "" {
		return status
	}
	reportingAgentID := strings.TrimSpace(status.ProbeAgentID)
	if reportingAgentID != agentID {
		status = availabilityStatusFromTarget(target)
	}
	status.ProbeAgentID = agentID
	reference := status.FreshnessTime()
	if reference.IsZero() {
		reference = m.availabilityProbeAssignmentReference(target.ID, agentID, now)
	}
	if !availabilityProbeReportIsStale(target, reference, now) {
		return status
	}
	status.Outcome = string(AvailabilityProbeIndeterminate)
	status.Available = false
	status.LastError = availabilityProbeStaleError
	status.LatencyMillis = 0
	return status
}

func (m *Monitor) availabilityProbeAssignmentReference(targetID, agentID string, now time.Time) time.Time {
	if m == nil {
		return now
	}
	targetID = strings.TrimSpace(targetID)
	agentID = strings.TrimSpace(agentID)
	if targetID == "" || agentID == "" {
		return now
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.availabilityProbeTrackers == nil {
		m.availabilityProbeTrackers = make(map[string]availabilityProbeAssignmentTracker)
	}
	if tracker, ok := m.availabilityProbeTrackers[targetID]; ok && tracker.AgentID == agentID && !tracker.Since.IsZero() {
		return tracker.Since
	}
	m.availabilityProbeTrackers[targetID] = availabilityProbeAssignmentTracker{
		AgentID: agentID,
		Since:   now,
	}
	return now
}

func availabilityProbeStatusIsStale(status AvailabilityProbeStatus) bool {
	return status.Outcome == string(AvailabilityProbeIndeterminate) &&
		strings.EqualFold(strings.TrimSpace(status.LastError), availabilityProbeStaleError)
}

func availabilityProbeReportIsStale(target config.AvailabilityTarget, lastChecked time.Time, now time.Time) bool {
	if lastChecked.IsZero() {
		return true
	}
	return now.Sub(lastChecked) > availabilityProbeStaleWindow(target)
}

func availabilityProbeStaleWindow(target config.AvailabilityTarget) time.Duration {
	window := time.Duration(target.EffectivePollIntervalSecs()) * 3 * time.Second
	if window < availabilityProbeStaleFloor {
		return availabilityProbeStaleFloor
	}
	return window
}

// availabilityProbeTargetsForAgent returns the probe payload for the targets the
// given agent currently owns.
func (m *Monitor) availabilityProbeTargetsForAgent(hostID string) []map[string]interface{} {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return nil
	}
	var assigned []map[string]interface{}
	for _, target := range m.availabilityTargets() {
		if m.effectiveProbeAgentID(target) != hostID {
			continue
		}
		assigned = append(assigned, availabilityProbeAgentTargetPayload(target))
	}
	return assigned
}

// availabilityProbeAgentTargetPayload carries only what the agent needs to run
// the check. Failure accounting and resource linkage stay server-side.
func availabilityProbeAgentTargetPayload(target config.AvailabilityTarget) map[string]interface{} {
	payload := map[string]interface{}{
		"id":                  target.ID,
		"configRevision":      target.ConfigRevision,
		"name":                target.DisplayName(),
		"targetKind":          string(target.TargetKind),
		"address":             target.Address,
		"protocol":            string(target.Protocol),
		"enabled":             target.Enabled,
		"pollIntervalSeconds": target.EffectivePollIntervalSecs(),
		"timeoutMillis":       target.EffectiveTimeoutMillis(),
	}
	if target.Port > 0 {
		payload["port"] = target.Port
	}
	if path := strings.TrimSpace(target.Path); path != "" {
		payload["path"] = path
	}
	if target.UDPMode != "" {
		payload["udpMode"] = string(target.UDPMode)
	}
	if target.UDPRequest != "" {
		payload["udpRequest"] = target.UDPRequest
	}
	if target.UDPExpected != "" {
		payload["udpExpectedResponse"] = target.UDPExpected
	}
	return payload
}
