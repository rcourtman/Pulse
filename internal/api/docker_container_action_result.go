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
	// The raw fact summary stays on the evidence and legacy output; the
	// execution and verification summaries are the plain sentences people read.
	summary := dockerContainerFactSummary(facts)
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: actionPreflightReasonCode(facts.ReasonCode), Summary: dockerLifecycleExecutionSummary(req.Operation, facts)}
	if facts.MutationCompleted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: dockerLifecycleExecutionSummary(req.Operation, facts)}
	} else if facts.MutationStarted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: dockerLifecycleExecutionSummary(req.Operation, facts)}
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
			status, reason := dockerLifecycleVerification(req.Operation, facts.Before, facts.After)
			verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, ReasonCode: reason, Summary: dockerLifecycleOutcomeSummary(req.Operation, status, reason, facts.Before, facts.After), Evidence: []unified.ActionEvidence{evidence}}
		}
	}
	if independent != nil && freshDockerLifecycleObservation(facts.Before.ObservedAt, independent.Snapshot.ObservedAt, independent.ReceivedAt) && strings.TrimSpace(independent.TrustDomain) != "" && strings.TrimSpace(independent.TrustDomain) != "agent:"+agentID && strings.EqualFold(independent.Snapshot.ContainerID, facts.ContainerID) {
		observationDigest, err := operationreceipt.DigestCanonicalJSON(struct {
			ActionID  string                                       `json:"action_id"`
			SubjectID string                                       `json:"subject_id"`
			Before    agentexec.DockerContainerLifecycleSnapshot   `json:"before"`
			After     agentexec.DockerContainerObservationSnapshot `json:"after"`
		}{req.ActionID, resourceID, facts.Before, independent.Snapshot})
		if err != nil {
			return nil, err
		}
		independentSummary := fmt.Sprintf("Direct daemon observation: before=%s; after=%s; running=%t; health=%s", facts.Before.State, independent.Snapshot.State, independent.Snapshot.Running, independent.Snapshot.Health)
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version: unified.ActionEvidenceVersion, ID: req.ActionID + "-direct-daemon-observation", ObserverID: independent.ObserverID,
			ObserverKind: "docker_daemon_observer", ObserverTrustDomain: independent.TrustDomain, ExecutorTrustDomain: "agent:" + agentID,
			Method: independent.Method, SubjectID: resourceID, ObservedAt: independent.Snapshot.ObservedAt.UTC(), ReceivedAt: independent.ReceivedAt.UTC(),
			Summary: independentSummary, Refs: []unified.ActionEvidenceRef{{ID: req.ActionID, Kind: "docker_before_after", Digest: observationDigest}},
		})
		if err != nil {
			return nil, fmt.Errorf("normalize independent docker observation: %w", err)
		}
		observed := agentexec.DockerContainerLifecycleSnapshot{
			ContainerID: independent.Snapshot.ContainerID, State: independent.Snapshot.State, Running: independent.Snapshot.Running,
			Health: independent.Snapshot.Health, StartedAt: independent.Snapshot.StartedAt,
		}
		status, reason := dockerLifecycleVerification(req.Operation, facts.Before, observed)
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceIndependent, ReasonCode: reason, Summary: dockerLifecycleOutcomeSummary(req.Operation, status, reason, facts.Before, observed), Evidence: []unified.ActionEvidence{evidence}}
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
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: actionPreflightReasonCode(facts.ReasonCode), Summary: dockerUpdateExecutionSummary(facts)}
	if facts.MutationCompleted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: dockerUpdateExecutionSummary(facts)}
	} else if facts.MutationStarted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: dockerUpdateExecutionSummary(facts)}
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
			// A running replacement must also have a usable running/health
			// readback; identity alone must not confirm an unhealthy update.
			// Preserve updates of containers intentionally left stopped.
			if facts.After.Running && !agentexec.IsDockerContainerHealth(facts.After.Health) {
				status = unified.ActionVerificationInconclusive
				reason = "container_health_unknown"
			} else if dockerUpdateFactsMatch(facts, facts.After.ContainerID) && dockerUpdateHealthStarting(facts.After.State, facts.After.Running, facts.After.Health) {
				// A first probe that has not run yet is not evidence against
				// the update, matching start and restart verification.
				status = unified.ActionVerificationInconclusive
				reason = "container_health_starting"
			} else if dockerUpdateFactsMatch(facts, facts.After.ContainerID) &&
				(!facts.After.Running || (facts.After.State == "running" &&
					agentexec.DockerContainerHealthAllowsVerifiedRunningState(facts.After.Health))) {
				status = unified.ActionVerificationConfirmed
				reason = ""
			}
			verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceAgentAttested, ReasonCode: reason, Summary: dockerUpdateOutcomeSummary(status, reason, facts.After.State, facts.After.Running, facts.After.Health), Evidence: []unified.ActionEvidence{evidence}}
		}
	}
	if independent != nil && facts.NewContainerID != "" && freshDockerUpdateObservation(independent.Snapshot.ObservedAt, independent.ReceivedAt) && strings.TrimSpace(independent.TrustDomain) != "" && strings.TrimSpace(independent.TrustDomain) != "agent:"+agentID {
		observationDigest, err := operationreceipt.DigestCanonicalJSON(struct {
			ActionID       string                                       `json:"action_id"`
			SubjectID      string                                       `json:"subject_id"`
			NewContainerID string                                       `json:"new_container_id"`
			After          agentexec.DockerContainerObservationSnapshot `json:"after"`
		}{facts.ActionID, resourceID, facts.NewContainerID, independent.Snapshot})
		if err != nil {
			return nil, err
		}
		independentSummary := fmt.Sprintf("Direct daemon observation of replacement container: state=%s; running=%t", independent.Snapshot.State, independent.Snapshot.Running)
		evidence, err := unified.NormalizeActionEvidence(unified.ActionEvidence{
			Version: unified.ActionEvidenceVersion, ID: facts.ActionID + "-direct-daemon-observation", ObserverID: independent.ObserverID,
			ObserverKind: "docker_daemon_observer", ObserverTrustDomain: independent.TrustDomain, ExecutorTrustDomain: "agent:" + agentID,
			Method: independent.Method, SubjectID: resourceID, ObservedAt: independent.Snapshot.ObservedAt.UTC(), ReceivedAt: independent.ReceivedAt.UTC(),
			Summary: independentSummary, Refs: []unified.ActionEvidenceRef{{ID: facts.ActionID, Kind: "docker_update_replacement", Digest: observationDigest}},
		})
		if err != nil {
			return nil, fmt.Errorf("normalize independent docker update observation: %w", err)
		}
		status := unified.ActionVerificationContradicted
		reason := "postcondition_contradicted"
		// Identity is necessary but not sufficient: a replacement can exit or
		// become unhealthy after the agent reported success. Compare against
		// its readback rather than requiring running unconditionally, because
		// updates deliberately preserve stopped containers.
		if !facts.ReadbackRan || facts.After.State == "" {
			status = unified.ActionVerificationInconclusive
			reason = "agent_readback_unavailable"
		} else if dockerUpdateFactsMatch(facts, independent.Snapshot.ContainerID) &&
			independent.Snapshot.State == facts.After.State &&
			independent.Snapshot.Running == facts.After.Running &&
			dockerUpdateHealthStarting(independent.Snapshot.State, independent.Snapshot.Running, independent.Snapshot.Health) {
			status = unified.ActionVerificationInconclusive
			reason = "container_health_starting"
		} else if dockerUpdateFactsMatch(facts, independent.Snapshot.ContainerID) &&
			independent.Snapshot.State == facts.After.State &&
			independent.Snapshot.Running == facts.After.Running &&
			(!independent.Snapshot.Running || (independent.Snapshot.State == "running" &&
				agentexec.DockerContainerHealthAllowsVerifiedRunningState(independent.Snapshot.Health))) {
			status = unified.ActionVerificationConfirmed
			reason = ""
		}
		outcome := dockerUpdateOutcomeSummary(status, reason, independent.Snapshot.State, independent.Snapshot.Running, independent.Snapshot.Health)
		if status == unified.ActionVerificationContradicted && !dockerUpdateFactsMatch(facts, independent.Snapshot.ContainerID) {
			outcome = "Pulse found a different container than the one the update created."
		}
		verification = unified.ActionVerificationTruth{Status: status, EvidenceClass: unified.ActionEvidenceIndependent, ReasonCode: reason, Summary: outcome, Evidence: []unified.ActionEvidence{evidence}}
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

// actionRefusalUncoded is the legacy aggregate recorded when a pre-mutation
// refusal arrives with no machine reason code. Agents older than the typed
// refusal contract report every refusal this way.
const actionRefusalUncoded = "preflight_refused"

// actionPreflightReasonCode preserves typed agent refusals while retaining the
// legacy aggregate for older agents that do not yet send a reason_code.
func actionPreflightReasonCode(reasonCode string) string {
	reasonCode = strings.TrimSpace(reasonCode)
	if agentexec.IsActionRefusalReasonCode(reasonCode) {
		return reasonCode
	}
	return actionRefusalUncoded
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
// the agent claims to have created. Independent verification additionally
// compares its state with the agent readback, preserving stopped updates.
func dockerUpdateFactsMatch(facts agentexec.DockerContainerUpdateResultPayload, observedContainerID string) bool {
	if facts.NewContainerID == "" || observedContainerID == "" {
		return false
	}
	return strings.EqualFold(facts.NewContainerID, observedContainerID)
}

func dockerUpdateHealthStarting(state string, running bool, health string) bool {
	return running && state == "running" && strings.EqualFold(strings.TrimSpace(health), agentexec.DockerContainerHealthStarting)
}

func dockerContainerFactSummary(facts agentexec.DockerContainerLifecycleResultPayload) string {
	summary := fmt.Sprintf("Container lifecycle: phase=%s; mutation started=%t; mutation completed=%t; readback ran=%t; before=%s/%s; after=%s/%s", strings.TrimSpace(facts.ExecutionPhase), facts.MutationStarted, facts.MutationCompleted, facts.ReadbackRan, strings.TrimSpace(facts.Before.State), dockerLifecycleHealthSummary(facts.Before.Health), strings.TrimSpace(facts.After.State), dockerLifecycleHealthSummary(facts.After.Health))
	if errText := strings.TrimSpace(facts.Error); errText != "" {
		summary += "; error=" + errText
	}
	return summary
}

func dockerLifecycleHealthSummary(health string) string {
	if health = strings.TrimSpace(health); health != "" {
		return health
	}
	return "unknown"
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
		return after.Running && after.State == "running" && agentexec.DockerContainerHealthAllowsVerifiedRunningState(after.Health)
	case agentexec.DockerContainerOperationStop:
		return !after.Running && after.State == "exited"
	case agentexec.DockerContainerOperationRestart:
		return after.Running && after.State == "running" && !after.StartedAt.IsZero() && after.StartedAt.After(before.StartedAt) && agentexec.DockerContainerHealthAllowsVerifiedRunningState(after.Health)
	default:
		return false
	}
}

func dockerLifecycleRunningHealthUnknown(operation string, after agentexec.DockerContainerLifecycleSnapshot) bool {
	return (operation == agentexec.DockerContainerOperationStart || operation == agentexec.DockerContainerOperationRestart) && !agentexec.IsDockerContainerHealth(after.Health)
}

func dockerLifecycleVerification(operation string, before, after agentexec.DockerContainerLifecycleSnapshot) (unified.ActionVerificationStatus, string) {
	if dockerLifecycleFactsMatch(operation, before, after) {
		return unified.ActionVerificationConfirmed, ""
	}
	if dockerLifecycleRunningHealthUnknown(operation, after) {
		return unified.ActionVerificationInconclusive, "container_health_unknown"
	}
	if (operation == agentexec.DockerContainerOperationStart || operation == agentexec.DockerContainerOperationRestart) &&
		after.Running && after.State == "running" && after.Health == agentexec.DockerContainerHealthStarting {
		return unified.ActionVerificationInconclusive, "container_health_starting"
	}
	return unified.ActionVerificationContradicted, "postcondition_contradicted"
}

// dockerLifecycleVerbs returns the base and past-tense verb for a lifecycle
// operation, so summaries read "restart" and "restarted".
func dockerLifecycleVerbs(operation string) (string, string) {
	switch operation {
	case agentexec.DockerContainerOperationStart:
		return "start", "started"
	case agentexec.DockerContainerOperationStop:
		return "stop", "stopped"
	case agentexec.DockerContainerOperationRestart:
		return "restart", "restarted"
	default:
		return "change", "changed"
	}
}

func dockerLifecycleExecutionSummary(operation string, facts agentexec.DockerContainerLifecycleResultPayload) string {
	base, past := dockerLifecycleVerbs(operation)
	var summary string
	switch {
	case facts.MutationCompleted:
		return "Docker " + past + " the container."
	case facts.MutationStarted:
		summary = "Docker was asked to " + base + " the container, but did not confirm it, so the change may or may not have happened."
	default:
		summary = "The agent did not " + base + " the container."
	}
	return summary + dockerAgentErrorSentence(facts.Error)
}

// dockerAgentErrorSentence appends the agent's own error text, if any, as a
// trailing sentence.
func dockerAgentErrorSentence(errText string) string {
	errText = strings.TrimRight(strings.TrimSpace(errText), " .:")
	if errText == "" {
		return ""
	}
	return " Agent said: " + errText + "."
}

// dockerContainerStateWords describes what Docker reports for a container. It
// reads the state and the running flag together so the two cannot disagree.
func dockerContainerStateWords(state string, running bool) string {
	state = strings.ToLower(strings.TrimSpace(state))
	switch state {
	case "running":
		if running {
			return "running"
		}
		return "running but not marked as running"
	case "exited":
		return "stopped"
	case "restarting":
		return "restarting, which usually means it keeps crashing"
	case "created":
		return "created but not started"
	case "paused", "dead":
		return state
	case "removing":
		return "being removed"
	case "":
		return "in an unknown state"
	default:
		return "in the " + state + " state"
	}
}

func dockerContainerIsRunning(state string, running bool) bool {
	return running && strings.EqualFold(strings.TrimSpace(state), "running")
}

const dockerHealthStartingAdvice = " Its health check was still starting when Pulse checked. Wait a moment, then refresh its status."

// dockerLifecycleOutcomeSummary says in plain words what Pulse saw after a
// container start, stop or restart. The typed facts stay on the evidence.
func dockerLifecycleOutcomeSummary(operation string, status unified.ActionVerificationStatus, reason string, before, after agentexec.DockerContainerLifecycleSnapshot) string {
	running := dockerContainerIsRunning(after.State, after.Running)
	healthy := strings.EqualFold(strings.TrimSpace(after.Health), agentexec.DockerContainerHealthHealthy)
	unhealthy := strings.EqualFold(strings.TrimSpace(after.Health), agentexec.DockerContainerHealthUnhealthy)
	switch status {
	case unified.ActionVerificationConfirmed:
		switch operation {
		case agentexec.DockerContainerOperationStop:
			return "The container is stopped."
		case agentexec.DockerContainerOperationRestart:
			if healthy {
				return "The container restarted and is running and healthy."
			}
			return "The container restarted and is running."
		default:
			if healthy {
				return "The container is running and healthy."
			}
			return "The container is running."
		}
	case unified.ActionVerificationInconclusive:
		switch reason {
		case "container_health_starting":
			return "The container is running." + dockerHealthStartingAdvice
		case "container_health_unknown":
			return "Docker reports the container as " + dockerContainerStateWords(after.State, after.Running) + ". Pulse could not get its health information, so it could not confirm the result."
		}
		return "Pulse could not confirm the result."
	case unified.ActionVerificationContradicted:
		if operation == agentexec.DockerContainerOperationStop {
			if running {
				return "The container is still running."
			}
			return "Pulse expected a stopped container. Docker reports it as " + dockerContainerStateWords(after.State, after.Running) + "."
		}
		if !running {
			return "The container is not running. Docker reports it as " + dockerContainerStateWords(after.State, after.Running) + "."
		}
		if operation == agentexec.DockerContainerOperationRestart && (after.StartedAt.IsZero() || !after.StartedAt.After(before.StartedAt)) {
			switch {
			case after.StartedAt.IsZero():
				return "The container is running, but Pulse did not receive its start time, so it could not confirm a restart."
			case after.StartedAt.Equal(before.StartedAt):
				return "The container is running, but its start time did not change, so it did not restart."
			default:
				return "The container is running, but its start time is not newer than before, so Pulse could not confirm a restart."
			}
		}
		if unhealthy {
			if operation == agentexec.DockerContainerOperationRestart {
				return "The container restarted, but its health check is failing. Check its logs."
			}
			return "The container started, but its health check is failing. Check its logs."
		}
		return "The container is not in the state this action should have left it in."
	}
	return "Pulse could not confirm the result."
}

func dockerUpdateExecutionSummary(facts agentexec.DockerContainerUpdateResultPayload) string {
	var summary string
	switch {
	case facts.MutationCompleted:
		return "Docker replaced the container with one using the new image."
	case facts.MutationStarted:
		summary = "The update started, but Pulse could not confirm it completed."
	default:
		summary = "The update stopped before changing the container."
	}
	return summary + dockerAgentErrorSentence(facts.Error)
}

// dockerUpdateOutcomeSummary says in plain words what Pulse saw of the
// replacement container after an update.
func dockerUpdateOutcomeSummary(status unified.ActionVerificationStatus, reason, state string, running bool, health string) string {
	isRunning := dockerContainerIsRunning(state, running)
	switch status {
	case unified.ActionVerificationConfirmed:
		if !isRunning {
			return "The updated container is in place. Docker reports it as " + dockerContainerStateWords(state, running) + "."
		}
		if strings.EqualFold(strings.TrimSpace(health), agentexec.DockerContainerHealthHealthy) {
			return "The updated container is running and healthy."
		}
		return "The updated container is running."
	case unified.ActionVerificationInconclusive:
		switch reason {
		case "container_health_starting":
			return "The updated container is running." + dockerHealthStartingAdvice
		case "container_health_unknown":
			return "Docker reports the updated container as " + dockerContainerStateWords(state, running) + ". Pulse could not get its health information, so it could not confirm the update."
		case "agent_readback_unavailable":
			return "The agent did not report the updated container's state, so Pulse could not confirm the update."
		}
		return "Pulse could not confirm the update."
	case unified.ActionVerificationContradicted:
		if !isRunning {
			return "Docker reports the updated container as " + dockerContainerStateWords(state, running) + "."
		}
		if strings.EqualFold(strings.TrimSpace(health), agentexec.DockerContainerHealthUnhealthy) {
			return "The updated container is running, but its health check is failing. Check its logs."
		}
		return "The updated container is not in the expected state."
	}
	return "Pulse could not confirm the update."
}
