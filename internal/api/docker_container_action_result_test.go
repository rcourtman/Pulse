package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestDockerContainerExecutionResultDerivesTruthOnlyFromFacts(t *testing.T) {
	now := time.Now().UTC()
	req := agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}
	tests := []struct {
		name         string
		facts        agentexec.DockerContainerLifecycleResultPayload
		execution    unified.ActionExecutionStatus
		verification unified.ActionVerificationStatus
		class        unified.ActionEvidenceClass
	}{
		{name: "preflight no effect", facts: dockerResultFacts(now, false, false, false, true), execution: unified.ActionExecutionNotRun, verification: unified.ActionVerificationInconclusive, class: unified.ActionEvidenceNone},
		{name: "completed confirmed", facts: dockerResultFacts(now, true, true, true, true), execution: unified.ActionExecutionSucceeded, verification: unified.ActionVerificationConfirmed, class: unified.ActionEvidenceAgentAttested},
		{name: "completed contradicted", facts: dockerResultFacts(now, true, true, true, false), execution: unified.ActionExecutionSucceeded, verification: unified.ActionVerificationContradicted, class: unified.ActionEvidenceAgentAttested},
		{name: "partial unknown", facts: dockerResultFacts(now, true, false, false, false), execution: unified.ActionExecutionInconclusive, verification: unified.ActionVerificationInconclusive, class: unified.ActionEvidenceNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", req, tc.facts, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if result.ActionResultV2.Execution.Status != tc.execution || result.ActionResultV2.Verification.Status != tc.verification || result.ActionResultV2.Verification.EvidenceClass != tc.class {
				t.Fatalf("truth = %#v", result.ActionResultV2)
			}
			if result.ActionResultV2.Compensation.Support != unified.ActionCompensationUnavailable {
				t.Fatalf("restart acquired rollback: %#v", result.ActionResultV2.Compensation)
			}
		})
	}
}

func TestDockerContainerExecutionResultStaleReadbackIsInconclusive(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now.Add(-time.Hour), true, true, true, true)
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, facts, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive || result.ActionResultV2.Verification.ReasonCode != "stale_agent_readback" {
		t.Fatalf("verification = %#v", result.ActionResultV2.Verification)
	}
}

func TestDockerContainerExecutionResultClassifiesDistinctDaemonObserverAsIndependent(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, true, true, true, true)
	facts.ContainerID = dockerLifecycleTestID
	req := agentexec.DockerContainerLifecyclePayload{ActionID: "action-1", Operation: agentexec.DockerContainerOperationRestart}
	observation := &dockerContainerPostconditionObservation{
		ObserverID: "colima-direct-cli", TrustDomain: "daemon:colima-direct", Method: "docker_api_inspect",
		Snapshot: facts.After, ReceivedAt: now,
	}
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", req, facts, observation, now)
	if err != nil {
		t.Fatal(err)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationConfirmed || truth.EvidenceClass != unified.ActionEvidenceIndependent || len(truth.Evidence) != 1 || truth.Evidence[0].Digest == "" || len(truth.Evidence[0].Refs) != 1 {
		t.Fatalf("independent verification = %#v", truth)
	}
}

func dockerResultFacts(now time.Time, started, completed, readback, matches bool) agentexec.DockerContainerLifecycleResultPayload {
	beforeStart := now.Add(-time.Minute)
	afterStart := now
	state, running := "running", true
	if !matches {
		afterStart = beforeStart
		state, running = "exited", false
	}
	return agentexec.DockerContainerLifecycleResultPayload{
		ExecutionPhase: agentexec.DockerContainerPhaseComplete, MutationStarted: started, MutationCompleted: completed, ReadbackRan: readback,
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: "running", Running: true, StartedAt: beforeStart, ObservedAt: now.Add(-time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: state, Running: running, StartedAt: afterStart, ObservedAt: now},
	}
}

const dockerLifecycleTestID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
