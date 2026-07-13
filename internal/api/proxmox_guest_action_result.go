package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func proxmoxGuestExecutionResult(actionID, resourceID, agentID string, kind proxmoxGuestKind, operation string, exitCode int, output, commandError string, agentVerification *unified.ActionVerificationResult, independentBefore, independentAfter *proxmoxGuestPostconditionObservation, independentEvaluation agentexec.PostconditionEvaluation, actionStartedAt time.Time) (*unified.ExecutionResult, error) {
	executionSummary := fmt.Sprintf("Proxmox guest %s command exited with status %d.", strings.TrimSpace(operation), exitCode)
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: executionSummary}
	verification := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "agent_readback_inconclusive", Summary: "The executing agent did not return a conclusive postcondition read."}
	legacy := &unified.ExecutionResult{Output: strings.TrimSpace(output)}
	if exitCode != 0 {
		execution.Status = unified.ActionExecutionFailed
		execution.ReasonCode = "proxmox_command_failed"
		execution.Summary = strings.TrimSpace(firstNonEmpty(commandError, output, executionSummary))
		verification = unified.ActionVerificationTruth{Status: unified.ActionVerificationNotAttempted, EvidenceClass: unified.ActionEvidenceNone}
		legacy.ErrorMessage = execution.Summary
	} else if agentVerification != nil && agentVerification.Ran {
		agentSummary := strings.TrimSpace(firstNonEmpty(agentVerification.Note, agentVerification.Output, "The executing agent read the Proxmox guest postcondition."))
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version:             unified.ActionEvidenceVersion,
			ID:                  actionID + "-agent-proxmox-readback",
			ObserverID:          agentID,
			ObserverKind:        "unified_agent",
			ObserverTrustDomain: "agent:" + agentID,
			ExecutorTrustDomain: "agent:" + agentID,
			Method:              "server_owned_proxmox_cli_status",
			SubjectID:           resourceID,
			ObservedAt:          agentVerification.RanAt.UTC(),
			ReceivedAt:          time.Now().UTC(),
			Summary:             agentSummary,
		})
		if err != nil {
			return nil, fmt.Errorf("normalize Proxmox agent readback evidence: %w", err)
		}
		status := unified.ActionVerificationContradicted
		reason := "postcondition_contradicted"
		if agentVerification.Success {
			status = unified.ActionVerificationConfirmed
			reason = ""
		}
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, ReasonCode: reason, Summary: agentSummary, Evidence: []unified.ActionEvidence{evidence}}
	}

	if exitCode == 0 && usableIndependentProxmoxObservation(agentID, actionStartedAt, independentAfter) {
		beforeSnapshot := (*proxmoxGuestLifecycleSnapshot)(nil)
		if independentBefore != nil && usableIndependentProxmoxBeforeObservation(actionStartedAt, independentBefore) {
			beforeCopy := independentBefore.Snapshot
			beforeSnapshot = &beforeCopy
		}
		observationDigest, err := operationreceipt.DigestCanonicalJSON(struct {
			ActionID  string                         `json:"action_id"`
			SubjectID string                         `json:"subject_id"`
			Before    *proxmoxGuestLifecycleSnapshot `json:"before,omitempty"`
			After     proxmoxGuestLifecycleSnapshot  `json:"after"`
		}{ActionID: actionID, SubjectID: resourceID, Before: beforeSnapshot, After: independentAfter.Snapshot})
		if err != nil {
			return nil, fmt.Errorf("digest Proxmox control-plane observation: %w", err)
		}
		independentSummary := fmt.Sprintf("Proxmox API observation: status=%s; uptime=%ds", independentAfter.Snapshot.Status, independentAfter.Snapshot.Uptime)
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version:             unified.ActionEvidenceVersion,
			ID:                  actionID + "-proxmox-api-observation",
			ObserverID:          independentAfter.ObserverID,
			ObserverKind:        "proxmox_control_plane",
			ObserverTrustDomain: independentAfter.TrustDomain,
			ExecutorTrustDomain: "agent:" + agentID,
			Method:              independentAfter.Method,
			SubjectID:           resourceID,
			ObservedAt:          independentAfter.Snapshot.ObservedAt.UTC(),
			ReceivedAt:          independentAfter.ReceivedAt.UTC(),
			Summary:             independentSummary,
			Refs:                []unified.ActionEvidenceRef{{ID: actionID, Kind: "proxmox_before_after", Digest: observationDigest}},
		})
		if err != nil {
			return nil, fmt.Errorf("normalize Proxmox control-plane evidence: %w", err)
		}
		status := unified.ActionVerificationInconclusive
		reason := strings.TrimSpace(firstNonEmpty(independentEvaluation.ReasonCode, "postcondition_inconclusive"))
		if independentEvaluation.Conclusive {
			status = unified.ActionVerificationContradicted
			reason = "postcondition_contradicted"
			if independentEvaluation.Matched {
				status = unified.ActionVerificationConfirmed
				reason = ""
			}
		}
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceIndependent, ReasonCode: reason, Summary: independentSummary, Evidence: []unified.ActionEvidence{evidence}}
	}

	canonical := unified.ActionResultV2{
		Version:      unified.ActionResultV2Version,
		Execution:    execution,
		Verification: verification,
		Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable, Summary: "Proxmox guest lifecycle actions do not provide automatic rollback."},
	}
	projected, _, err := unified.ApplyActionResultV2(legacy, canonical)
	if err != nil {
		return nil, fmt.Errorf("apply canonical Proxmox lifecycle action truth: %w", err)
	}
	projected.Verification = unified.LegacyActionVerificationFromV2(*projected.ActionResultV2)
	return projected, nil
}

func usableIndependentProxmoxObservation(agentID string, actionStartedAt time.Time, observation *proxmoxGuestPostconditionObservation) bool {
	if observation == nil || strings.TrimSpace(observation.ObserverID) == "" || strings.TrimSpace(observation.TrustDomain) == "" || strings.TrimSpace(observation.Method) == "" || strings.TrimSpace(observation.TrustDomain) == "agent:"+strings.TrimSpace(agentID) {
		return false
	}
	observedAt := observation.Snapshot.ObservedAt.UTC()
	receivedAt := observation.ReceivedAt.UTC()
	return !actionStartedAt.IsZero() && !observedAt.Before(actionStartedAt.UTC()) && !observedAt.After(receivedAt.Add(5*time.Minute)) && !observedAt.Before(receivedAt.Add(-15*time.Minute))
}

func usableIndependentProxmoxBeforeObservation(actionStartedAt time.Time, observation *proxmoxGuestPostconditionObservation) bool {
	if observation == nil || actionStartedAt.IsZero() || observation.Snapshot.ObservedAt.IsZero() || observation.ReceivedAt.IsZero() {
		return false
	}
	observedAt := observation.Snapshot.ObservedAt.UTC()
	receivedAt := observation.ReceivedAt.UTC()
	return !observedAt.After(actionStartedAt.UTC()) && !observedAt.After(receivedAt.Add(5*time.Minute)) && !observedAt.Before(receivedAt.Add(-15*time.Minute))
}
