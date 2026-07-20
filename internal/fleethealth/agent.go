package fleethealth

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

const (
	defaultAgentReportIntervalSeconds = 30
	minimumAgentStaleThreshold        = 5 * time.Minute
)

type AgentLiveness string

const (
	AgentLivenessActive  AgentLiveness = "active"
	AgentLivenessPending AgentLiveness = "pending"
	AgentLivenessStale   AgentLiveness = "stale"
)

type AgentVersionDrift string

const (
	AgentVersionBehind  AgentVersionDrift = "behind"
	AgentVersionCurrent AgentVersionDrift = "current"
	AgentVersionUnknown AgentVersionDrift = "unknown"
)

// AgentConnectionID returns the stable ledger identity for a host-backed
// agent. Empty agent IDs do not produce a connection identity.
func AgentConnectionID(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ""
	}
	return "agent:" + agentID
}

// AgentStaleThreshold returns the canonical heartbeat cutoff shared by the
// connections ledger and agent diagnostics. Five expected reports must be
// missed, with a five-minute floor to tolerate updates and service restarts.
func AgentStaleThreshold(intervalSeconds int) time.Duration {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultAgentReportIntervalSeconds
	}
	threshold := time.Duration(intervalSeconds*5) * time.Second
	if threshold < minimumAgentStaleThreshold {
		return minimumAgentStaleThreshold
	}
	return threshold
}

// DeriveAgentLiveness derives agent heartbeat state without probing or
// mutating runtime state.
func DeriveAgentLiveness(lastSeen, now time.Time, intervalSeconds int) AgentLiveness {
	if lastSeen.IsZero() {
		return AgentLivenessPending
	}
	if now.IsZero() {
		now = time.Now()
	}
	if now.Sub(lastSeen) > AgentStaleThreshold(intervalSeconds) {
		return AgentLivenessStale
	}
	return AgentLivenessActive
}

// DeriveAgentVersionDrift compares a reported version with the canonical
// agent update target. Missing or invalid release versions remain unknown.
func DeriveAgentVersionDrift(currentVersion, targetVersion string) AgentVersionDrift {
	currentVersion = strings.TrimSpace(currentVersion)
	targetVersion = strings.TrimSpace(targetVersion)
	if currentVersion == "" || targetVersion == "" {
		return AgentVersionUnknown
	}

	current, err := updates.ParseVersion(currentVersion)
	if err != nil {
		return AgentVersionUnknown
	}
	target, err := updates.ParseVersion(targetVersion)
	if err != nil {
		return AgentVersionUnknown
	}
	if target.IsNewerThan(current) {
		return AgentVersionBehind
	}
	return AgentVersionCurrent
}
