package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func hostAPTExecutionResult(resourceID, agentID, operation, output string, success, mutationStarted bool, verification string, beforeStateBound bool, beforeObservedAt, afterObservedAt, receivedAt time.Time) (*unified.ExecutionResult, error) {
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: output}
	if !success {
		if mutationStarted {
			execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: "The agent could not establish the final mutation outcome."}
		} else {
			execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: "preflight_refused", Summary: "The typed host operation did not begin mutation."}
		}
	}

	verificationTruth := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "agent_readback_inconclusive", Summary: "The agent readback was inconclusive and did not establish the requested postcondition."}
	if verification == agentexec.HostUpdateVerificationVerified || verification == agentexec.HostStorageCleanupVerificationVerified || verification == agentexec.HostUpdateVerificationFailed || verification == agentexec.HostStorageCleanupVerificationFailed {
		if !beforeStateBound {
			verificationTruth.ReasonCode = "before_state_mismatch"
			verificationTruth.Summary = "The agent readback did not match the request-bound before state."
		} else if !freshHostAPTResultObservation(beforeObservedAt, afterObservedAt, receivedAt) {
			verificationTruth.ReasonCode = "stale_agent_readback"
			verificationTruth.Summary = "The agent readback was stale, skewed, or had invalid mutation chronology."
		} else {
			evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
				Version: unified.ActionEvidenceVersion, ID: operation + "-agent-readback", ObserverID: agentID,
				ObserverKind: "unified_agent", ObserverTrustDomain: "agent:" + agentID, ExecutorTrustDomain: "agent:" + agentID,
				Method: "typed_read_after_write", SubjectID: resourceID, ObservedAt: afterObservedAt.UTC(), ReceivedAt: receivedAt.UTC(),
				Summary: "Bounded typed readback received from the mutating agent.",
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

	canonical := unified.ActionResultV2{
		Version:      unified.ActionResultV2Version,
		Execution:    execution,
		Verification: verificationTruth,
		Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable},
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
