package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func hostAPTExecutionResult(resourceID, agentID, operation, output string, success, mutationStarted bool, verification string, beforeStateBound, packageManagerHealthRequired, healthChecked, packageManagerHealthy, recoveryRequired bool, beforeObservedAt, afterObservedAt, observationBoundaryAt, receivedAt time.Time) (*unified.ExecutionResult, error) {
	output = strings.TrimSpace(output)
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: output}
	if !success {
		if mutationStarted {
			execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: output}
		} else {
			execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: "preflight_refused", Summary: output}
		}
	}

	verificationSummary := "The agent readback was inconclusive and did not establish the requested postcondition."
	if output != "" {
		verificationSummary = output
	}
	verificationTruth := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "agent_readback_inconclusive", Summary: verificationSummary}
	if verification == agentexec.HostUpdateVerificationVerified || verification == agentexec.HostStorageCleanupVerificationVerified || verification == agentexec.HostUpdateVerificationFailed || verification == agentexec.HostStorageCleanupVerificationFailed {
		if !beforeStateBound {
			verificationTruth.ReasonCode = "before_state_mismatch"
			verificationTruth.Summary = "The agent readback did not match the request-bound before state."
		} else if packageManagerHealthRequired && verification == agentexec.HostUpdateVerificationVerified && !healthChecked {
			verificationTruth.ReasonCode = "package_manager_health_unknown"
			verificationTruth.Summary = "The terminal receipt predates or lacks a completed bounded package-manager health check."
		} else if packageManagerHealthRequired && verification == agentexec.HostUpdateVerificationVerified && !packageManagerHealthy {
			verificationTruth.ReasonCode = "package_manager_unhealthy"
			verificationTruth.Summary = "The package-manager health check contradicted the claimed healthy postcondition."
		} else if !freshHostAPTResultObservation(beforeObservedAt, afterObservedAt, observationBoundaryAt) {
			verificationTruth.ReasonCode = "stale_agent_readback"
			verificationTruth.Summary = "The agent readback was stale, skewed, or had invalid mutation chronology."
		} else {
			evidenceObservedAt := afterObservedAt.UTC()
			evidenceReceivedAt := receivedAt.UTC()
			// Validation permits the agent clock to be slightly ahead of the
			// server. Canonical evidence cannot claim an observation after its
			// receipt, so conservatively bind bounded positive skew to the
			// server receipt boundary.
			if evidenceObservedAt.After(evidenceReceivedAt) {
				evidenceObservedAt = evidenceReceivedAt
			}
			evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
				Version: unified.ActionEvidenceVersion, ID: operation + "-agent-readback", ObserverID: agentID,
				ObserverKind: "unified_agent", ObserverTrustDomain: "agent:" + agentID, ExecutorTrustDomain: "agent:" + agentID,
				Method: "typed_read_after_write", SubjectID: resourceID, ObservedAt: evidenceObservedAt, ReceivedAt: evidenceReceivedAt,
				Summary: output,
			})
			if err == nil {
				status := unified.ActionVerificationConfirmed
				if verification == agentexec.HostUpdateVerificationFailed || verification == agentexec.HostStorageCleanupVerificationFailed {
					status = unified.ActionVerificationContradicted
				}
				verificationTruth = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, Evidence: []unified.ActionEvidence{evidence}, Summary: evidence.Summary}
			}
		}
	}

	compensationSummary := "Rollback is unavailable and no recovery action is required."
	if operation == agentexec.HostStorageCleanupOperationPackageCache {
		compensationSummary = "Package-cache cleanup is irreversible and non-rollbackable; a rescan reports the actual effect and never restoration."
	} else if recoveryRequired {
		compensationSummary = "No automatic rollback is available; separately governed recovery is required before another update attempt."
	}
	canonical := unified.ActionResultV2{
		Version:      unified.ActionResultV2Version,
		Execution:    execution,
		Verification: verificationTruth,
		Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable, Summary: compensationSummary},
	}
	legacy := &unified.ExecutionResult{Output: strings.TrimSpace(output)}
	projected, _, err := unified.ApplyActionResultV2(legacy, canonical)
	if err != nil {
		return nil, fmt.Errorf("apply canonical host APT action truth: %w", err)
	}
	projected.Verification = unified.LegacyActionVerificationFromV2(*projected.ActionResultV2)
	return projected, nil
}

func freshHostAPTResultObservation(before, after, receivedAt time.Time) bool {
	if before.IsZero() || after.IsZero() || receivedAt.IsZero() || after.Before(before) {
		return false
	}
	receivedAt = receivedAt.UTC()
	after = after.UTC()
	return !after.Before(receivedAt.Add(-15*time.Minute)) && !after.After(receivedAt.Add(unified.HostAPTTelemetryMaxClockSkew))
}
