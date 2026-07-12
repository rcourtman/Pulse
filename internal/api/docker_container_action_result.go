package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func dockerContainerExecutionResult(resourceID, agentID string, req agentexec.DockerContainerLifecyclePayload, facts agentexec.DockerContainerLifecycleResultPayload, independent *dockerContainerPostconditionObservation, receivedAt time.Time) (*unified.ExecutionResult, error) {
	summary := dockerContainerFactSummary(facts)
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: "preflight_refused", Summary: summary}
	if facts.MutationCompleted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: summary}
	} else if facts.MutationStarted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: summary}
	}

	verification := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "agent_readback_unavailable", Summary: "The typed operation did not return a usable container readback."}
	if facts.ReadbackRan {
		if !freshDockerLifecycleObservation(facts.Before.ObservedAt, facts.After.ObservedAt, receivedAt) {
			verification.ReasonCode = "stale_agent_readback"
			verification.Summary = "The agent readback was stale, skewed, or had invalid mutation chronology."
		} else {
			evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
				Version: unified.ActionEvidenceVersion, ID: req.Operation + "-agent-readback", ObserverID: agentID,
				ObserverKind: "unified_agent", ObserverTrustDomain: "agent:" + agentID, ExecutorTrustDomain: "agent:" + agentID,
				Method: "typed_container_read_after_write", SubjectID: resourceID, ObservedAt: facts.After.ObservedAt.UTC(), ReceivedAt: receivedAt.UTC(), Summary: summary,
			})
			if err != nil {
				return nil, fmt.Errorf("normalize docker lifecycle evidence: %w", err)
			}
			status := unified.ActionVerificationContradicted
			reason := "postcondition_contradicted"
			if dockerLifecycleFactsMatch(req.Operation, facts.Before, facts.After) {
				status = unified.ActionVerificationConfirmed
				reason = ""
			}
			verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, ReasonCode: reason, Summary: summary, Evidence: []unified.ActionEvidence{evidence}}
		}
	}
	if independent != nil && freshDockerLifecycleObservation(facts.Before.ObservedAt, independent.Snapshot.ObservedAt, independent.ReceivedAt) && strings.TrimSpace(independent.TrustDomain) != "" && strings.TrimSpace(independent.TrustDomain) != "agent:"+agentID && strings.EqualFold(independent.Snapshot.ContainerID, facts.ContainerID) {
		observationDigest, err := operationreceipt.DigestCanonicalJSON(struct {
			ActionID  string                                     `json:"action_id"`
			SubjectID string                                     `json:"subject_id"`
			Before    agentexec.DockerContainerLifecycleSnapshot `json:"before"`
			After     agentexec.DockerContainerLifecycleSnapshot `json:"after"`
		}{req.ActionID, resourceID, facts.Before, independent.Snapshot})
		if err != nil {
			return nil, err
		}
		independentSummary := fmt.Sprintf("Direct daemon observation: before=%s; after=%s; running=%t", facts.Before.State, independent.Snapshot.State, independent.Snapshot.Running)
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version: unified.ActionEvidenceVersion, ID: req.ActionID + "-direct-daemon-observation", ObserverID: independent.ObserverID,
			ObserverKind: "docker_daemon_observer", ObserverTrustDomain: independent.TrustDomain, ExecutorTrustDomain: "agent:" + agentID,
			Method: independent.Method, SubjectID: resourceID, ObservedAt: independent.Snapshot.ObservedAt, ReceivedAt: independent.ReceivedAt,
			Summary: independentSummary, Refs: []unified.ActionEvidenceRef{{ID: req.ActionID, Kind: "docker_before_after", Digest: observationDigest}},
		})
		if err != nil {
			return nil, fmt.Errorf("normalize independent docker observation: %w", err)
		}
		status := unified.ActionVerificationContradicted
		reason := "postcondition_contradicted"
		if dockerLifecycleFactsMatch(req.Operation, facts.Before, independent.Snapshot) {
			status = unified.ActionVerificationConfirmed
			reason = ""
		}
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceIndependent, ReasonCode: reason, Summary: independentSummary, Evidence: []unified.ActionEvidence{evidence}}
	}

	canonical := unified.ActionResultV2{
		Version: unified.ActionResultV2Version, Execution: execution, Verification: verification,
		Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable, Summary: "Container restart is non-rollbackable. Lab fixture cleanup is compensation for the disposable test environment, not product rollback."},
	}
	legacy := &unified.ExecutionResult{Output: summary}
	projected, _, err := unified.ApplyActionResultV2(legacy, canonical)
	if err != nil {
		return nil, fmt.Errorf("apply canonical docker lifecycle action truth: %w", err)
	}
	projected.Verification = unified.LegacyActionVerificationFromV2(*projected.ActionResultV2)
	return projected, nil
}

func dockerContainerFactSummary(facts agentexec.DockerContainerLifecycleResultPayload) string {
	return fmt.Sprintf("Container lifecycle: phase=%s; mutation started=%t; mutation completed=%t; readback ran=%t; before=%s; after=%s", strings.TrimSpace(facts.ExecutionPhase), facts.MutationStarted, facts.MutationCompleted, facts.ReadbackRan, strings.TrimSpace(facts.Before.State), strings.TrimSpace(facts.After.State))
}

func freshDockerLifecycleObservation(before, after, receivedAt time.Time) bool {
	if before.IsZero() || after.IsZero() || receivedAt.IsZero() || after.Before(before) {
		return false
	}
	return !after.Before(receivedAt.UTC().Add(-15*time.Minute)) && !after.After(receivedAt.UTC().Add(5*time.Minute))
}

func dockerLifecycleFactsMatch(operation string, before, after agentexec.DockerContainerLifecycleSnapshot) bool {
	if !strings.EqualFold(before.ContainerID, after.ContainerID) || after.ContainerID == "" {
		return false
	}
	switch operation {
	case agentexec.DockerContainerOperationStart:
		return after.Running && after.State == "running"
	case agentexec.DockerContainerOperationStop:
		return !after.Running && after.State == "exited"
	case agentexec.DockerContainerOperationRestart:
		return after.Running && after.State == "running" && !after.StartedAt.IsZero() && after.StartedAt.After(before.StartedAt)
	default:
		return false
	}
}
