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

func dockerContainerUpdateExecutionResult(resourceID, agentID string, facts agentexec.DockerContainerUpdateResultPayload, independent *dockerContainerPostconditionObservation, receivedAt time.Time) (*unified.ExecutionResult, error) {
	summary := dockerContainerUpdateFactSummary(facts)
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: "preflight_refused", Summary: summary}
	if facts.MutationCompleted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: summary}
	} else if facts.MutationStarted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: summary}
	}

	verification := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "agent_readback_unavailable", Summary: "The typed operation did not return a usable replacement-container readback."}
	if facts.ReadbackRan {
		if !freshDockerUpdateObservation(facts.After.ObservedAt, receivedAt) {
			verification.ReasonCode = "stale_agent_readback"
			verification.Summary = "The agent readback was stale or skewed."
		} else {
			evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
				Version: unified.ActionEvidenceVersion, ID: facts.Operation + "-agent-readback", ObserverID: agentID,
				ObserverKind: "unified_agent", ObserverTrustDomain: "agent:" + agentID, ExecutorTrustDomain: "agent:" + agentID,
				Method: "typed_container_read_after_write", SubjectID: resourceID, ObservedAt: facts.After.ObservedAt.UTC(), ReceivedAt: receivedAt.UTC(), Summary: summary,
			})
			if err != nil {
				return nil, fmt.Errorf("normalize docker update evidence: %w", err)
			}
			status := unified.ActionVerificationContradicted
			reason := "postcondition_contradicted"
			if dockerUpdateFactsMatch(facts, facts.After) {
				status = unified.ActionVerificationConfirmed
				reason = ""
			}
			verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, ReasonCode: reason, Summary: summary, Evidence: []unified.ActionEvidence{evidence}}
		}
	}
	if independent != nil && facts.NewContainerID != "" && freshDockerUpdateObservation(independent.Snapshot.ObservedAt, independent.ReceivedAt) && strings.TrimSpace(independent.TrustDomain) != "" && strings.TrimSpace(independent.TrustDomain) != "agent:"+agentID {
		observationDigest, err := operationreceipt.DigestCanonicalJSON(struct {
			ActionID       string                                     `json:"action_id"`
			SubjectID      string                                     `json:"subject_id"`
			NewContainerID string                                     `json:"new_container_id"`
			After          agentexec.DockerContainerLifecycleSnapshot `json:"after"`
		}{facts.ActionID, resourceID, facts.NewContainerID, independent.Snapshot})
		if err != nil {
			return nil, err
		}
		independentSummary := fmt.Sprintf("Direct daemon observation of replacement container: state=%s; running=%t", independent.Snapshot.State, independent.Snapshot.Running)
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version: unified.ActionEvidenceVersion, ID: facts.ActionID + "-direct-daemon-observation", ObserverID: independent.ObserverID,
			ObserverKind: "docker_daemon_observer", ObserverTrustDomain: independent.TrustDomain, ExecutorTrustDomain: "agent:" + agentID,
			Method: independent.Method, SubjectID: resourceID, ObservedAt: independent.Snapshot.ObservedAt, ReceivedAt: independent.ReceivedAt,
			Summary: independentSummary, Refs: []unified.ActionEvidenceRef{{ID: facts.ActionID, Kind: "docker_update_replacement", Digest: observationDigest}},
		})
		if err != nil {
			return nil, fmt.Errorf("normalize independent docker update observation: %w", err)
		}
		status := unified.ActionVerificationContradicted
		reason := "postcondition_contradicted"
		if dockerUpdateFactsMatch(facts, independent.Snapshot) {
			status = unified.ActionVerificationConfirmed
			reason = ""
		}
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceIndependent, ReasonCode: reason, Summary: independentSummary, Evidence: []unified.ActionEvidence{evidence}}
	}

	canonical := unified.ActionResultV2{
		Version: unified.ActionResultV2Version, Execution: execution, Verification: verification,
		Compensation: dockerUpdateCompensationTruth(facts),
	}
	legacy := &unified.ExecutionResult{Output: summary}
	projected, _, err := unified.ApplyActionResultV2(legacy, canonical)
	if err != nil {
		return nil, fmt.Errorf("apply canonical docker update action truth: %w", err)
	}
	projected.Verification = unified.LegacyActionVerificationFromV2(*projected.ActionResultV2)
	return projected, nil
}

// dockerUpdateCompensationTruth reports real compensation state: the agent
// creates a backup rename before recreating and restores it when the
// replacement fails, so unlike restarts this operation declares support.
func dockerUpdateCompensationTruth(facts agentexec.DockerContainerUpdateResultPayload) unified.ActionCompensationTruth {
	truth := unified.ActionCompensationTruth{
		Support:  unified.ActionCompensationDeclared,
		Strategy: "backup_rename_restore",
		Trigger:  "replacement_create_start_or_stability_failure",
	}
	switch {
	case facts.MutationCompleted:
		truth.Status = unified.ActionCompensationNotNeeded
		truth.Summary = "Update succeeded; the renamed backup container is removed automatically after a stability window."
	case !facts.MutationStarted:
		truth.Status = unified.ActionCompensationNotNeeded
		truth.Summary = "The update failed before mutating the container, so there was nothing to roll back."
	case facts.RolledBack:
		truth.Status = unified.ActionCompensationSucceeded
		truth.Summary = "The update failed and the original container was restored from its backup."
	case facts.RollbackAttempted:
		truth.Status = unified.ActionCompensationFailed
		truth.Summary = "The update failed and the automatic restore did not complete; the original container may still carry its backup name (" + strings.TrimSpace(facts.BackupContainer) + ")."
	default:
		truth.Status = unified.ActionCompensationNotAttempted
		truth.Summary = "The update failed after mutation started and no automatic restore ran."
	}
	return truth
}

func dockerContainerUpdateFactSummary(facts agentexec.DockerContainerUpdateResultPayload) string {
	summary := fmt.Sprintf(
		"Container update: phase=%s; mutation started=%t; mutation completed=%t; readback ran=%t; container=%s; replacement=%s; old image=%s; new image=%s; backup=%s; rollback attempted=%t; rolled back=%t",
		strings.TrimSpace(facts.ExecutionPhase), facts.MutationStarted, facts.MutationCompleted, facts.ReadbackRan,
		strings.TrimSpace(facts.ContainerName), strings.TrimSpace(facts.NewContainerID),
		shortDockerDigest(facts.OldImageDigest), shortDockerDigest(facts.NewImageDigest),
		strings.TrimSpace(facts.BackupContainer), facts.RollbackAttempted, facts.RolledBack,
	)
	if errText := strings.TrimSpace(facts.Error); errText != "" {
		summary += "; error=" + errText
	}
	return summary
}

func shortDockerDigest(digest string) string {
	digest = strings.TrimSpace(digest)
	const previewLen = 19 // "sha256:" + 12 hex characters
	if len(digest) <= previewLen {
		return digest
	}
	return digest[:previewLen]
}

func freshDockerUpdateObservation(observedAt, receivedAt time.Time) bool {
	if observedAt.IsZero() || receivedAt.IsZero() {
		return false
	}
	return !observedAt.Before(receivedAt.UTC().Add(-30*time.Minute)) && !observedAt.After(receivedAt.UTC().Add(5*time.Minute))
}

// dockerUpdateFactsMatch confirms the observed container is the replacement
// the agent claims to have created. A stopped original is recreated without
// being started, so run-state is not part of the postcondition; identity is.
func dockerUpdateFactsMatch(facts agentexec.DockerContainerUpdateResultPayload, observed agentexec.DockerContainerLifecycleSnapshot) bool {
	if facts.NewContainerID == "" || observed.ContainerID == "" {
		return false
	}
	return strings.EqualFold(facts.NewContainerID, observed.ContainerID)
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
