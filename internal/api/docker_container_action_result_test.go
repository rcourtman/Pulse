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

func TestDockerContainerExecutionResultPreservesTypedPreflightRefusal(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, false, false, false, true)
	facts.ReasonCode = agentexec.ActionRefusalTargetStateChanged
	result, err := dockerContainerExecutionResult(
		"app-container:fixture",
		"agent-1",
		agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart},
		facts,
		nil,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.ActionResultV2.Execution.ReasonCode; got != agentexec.ActionRefusalTargetStateChanged {
		t.Fatalf("execution reason = %q, want %q", got, agentexec.ActionRefusalTargetStateChanged)
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

func TestDockerContainerExecutionResultClampsBoundedPositiveAgentClockSkew(t *testing.T) {
	receivedAt := time.Now().UTC()
	facts := dockerResultFacts(receivedAt.Add(2*time.Second), true, true, true, true)
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, facts, nil, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	verification := result.ActionResultV2.Verification
	if verification.Status != unified.ActionVerificationConfirmed || len(verification.Evidence) != 1 {
		t.Fatalf("verification = %#v", verification)
	}
	if !verification.Evidence[0].ObservedAt.Equal(receivedAt) || !verification.Evidence[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("evidence timestamps = observed %s received %s, want receipt boundary %s", verification.Evidence[0].ObservedAt, verification.Evidence[0].ReceivedAt, receivedAt)
	}
}

func TestDockerContainerExecutionResultRejectsExcessivePositiveAgentClockSkew(t *testing.T) {
	receivedAt := time.Now().UTC()
	facts := dockerResultFacts(receivedAt.Add(6*time.Minute), true, true, true, true)
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, facts, nil, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	verification := result.ActionResultV2.Verification
	if verification.Status != unified.ActionVerificationInconclusive || verification.ReasonCode != "stale_agent_readback" || len(verification.Evidence) != 0 {
		t.Fatalf("verification = %#v", verification)
	}
}

func TestDockerContainerUpdateExecutionResultClampsBoundedPositiveAgentClockSkew(t *testing.T) {
	receivedAt := time.Now().UTC()
	observedAt := receivedAt.Add(2 * time.Second)
	facts := agentexec.DockerContainerUpdateResultPayload{
		Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhaseComplete,
		MutationStarted: true, MutationCompleted: true, ReadbackRan: true, NewContainerID: dockerLifecycleTestID,
		After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: "running", Running: true, ObservedAt: observedAt},
	}
	result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, nil, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	verification := result.ActionResultV2.Verification
	if verification.Status != unified.ActionVerificationConfirmed || len(verification.Evidence) != 1 {
		t.Fatalf("verification = %#v", verification)
	}
	if !verification.Evidence[0].ObservedAt.Equal(receivedAt) || !verification.Evidence[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("evidence timestamps = observed %s received %s, want receipt boundary %s", verification.Evidence[0].ObservedAt, verification.Evidence[0].ReceivedAt, receivedAt)
	}
}

func TestDockerContainerExecutionResultClassifiesDistinctDaemonObserverAsIndependent(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, true, true, true, true)
	facts.ContainerID = dockerLifecycleTestID
	req := agentexec.DockerContainerLifecyclePayload{ActionID: "action-1", Operation: agentexec.DockerContainerOperationRestart}
	observation := &dockerContainerPostconditionObservation{
		ObserverID: "colima-direct-cli", TrustDomain: "daemon:colima-direct", Method: "docker_api_inspect",
		Snapshot: dockerObservationSnapshotFromLifecycle(facts.After), ReceivedAt: now,
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

func TestDockerContainerExecutionResultDoesNotVerifyUnrecoveredHealth(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, true, true, true, true)
	facts.ContainerID = dockerLifecycleTestID
	req := agentexec.DockerContainerLifecyclePayload{ActionID: "action-1", Operation: agentexec.DockerContainerOperationRestart}
	for _, health := range []string{"", agentexec.DockerContainerHealthStarting, agentexec.DockerContainerHealthUnhealthy} {
		observation := &dockerContainerPostconditionObservation{
			ObserverID: "docker-observer", TrustDomain: "docker-daemon:agent-1", Method: "docker_api_inspect",
			Snapshot: dockerObservationSnapshotFromLifecycle(facts.After), ReceivedAt: now,
		}
		observation.Snapshot.Health = health
		result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", req, facts, observation, now)
		if err != nil {
			t.Fatal(err)
		}
		if result.ActionResultV2.Verification.Status == unified.ActionVerificationConfirmed {
			t.Fatalf("health %q produced confirmed verification: %#v", health, result.ActionResultV2.Verification)
		}
	}
}

func dockerObservationSnapshotFromLifecycle(snapshot agentexec.DockerContainerLifecycleSnapshot) agentexec.DockerContainerObservationSnapshot {
	return agentexec.DockerContainerObservationSnapshot{
		ContainerID: snapshot.ContainerID, State: snapshot.State, Running: snapshot.Running, Health: snapshot.Health,
		StartedAt: snapshot.StartedAt, RestartCount: snapshot.RestartCount, ObservedAt: snapshot.ObservedAt,
	}
}

func TestDockerContainerExecutionResultTreatsLegacyMissingHealthAsInconclusive(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, true, true, true, true)
	facts.After.Health = ""
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, facts, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	verification := result.ActionResultV2.Verification
	if verification.Status != unified.ActionVerificationInconclusive || verification.ReasonCode != "container_health_unknown" {
		t.Fatalf("legacy health verification = %#v", verification)
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
		Before: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: "running", Running: true, Health: agentexec.DockerContainerHealthHealthy, StartedAt: beforeStart, ObservedAt: now.Add(-time.Second)},
		After:  agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: state, Running: running, Health: agentexec.DockerContainerHealthHealthy, StartedAt: afterStart, ObservedAt: now},
	}
}

const dockerLifecycleTestID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// A replacement identity alone is not proof that its reported running state
// survived until the independent daemon readback.
func TestDockerContainerUpdateIndependentObservationMustMatchState(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, baseline, state, health string
		running                       bool
		want                          unified.ActionVerificationStatus
	}{
		{"running", "running", "running", "healthy", true, unified.ActionVerificationConfirmed},
		{"no healthcheck", "running", "running", "none", true, unified.ActionVerificationConfirmed},
		{"stopped original", "created", "created", "none", false, unified.ActionVerificationConfirmed},
		{"stopped after update", "running", "exited", "", false, unified.ActionVerificationContradicted},
		{"restarting", "running", "restarting", "", true, unified.ActionVerificationContradicted},
		{"unhealthy", "running", "running", "unhealthy", true, unified.ActionVerificationContradicted},
		{"starting healthcheck", "running", "running", "starting", true, unified.ActionVerificationContradicted},
		{"unknown health", "running", "running", "", true, unified.ActionVerificationContradicted},
		{"missing agent readback", "", "running", "healthy", true, unified.ActionVerificationInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := agentexec.DockerContainerUpdateResultPayload{
				Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhaseComplete,
				MutationStarted: true, MutationCompleted: true, ReadbackRan: true, NewContainerID: dockerLifecycleTestID,
				After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: "running", Running: true, ObservedAt: now},
			}
			facts.After.State = tc.baseline
			facts.After.Running = tc.baseline == "running"
			facts.ReadbackRan = tc.baseline != ""
			observation := &dockerContainerPostconditionObservation{
				ObserverID: "daemon-1", TrustDomain: "daemon:1", Method: "daemon_inspect", ReceivedAt: now,
				Snapshot: agentexec.DockerContainerObservationSnapshot{ContainerID: dockerLifecycleTestID, State: tc.state, Running: tc.running, Health: tc.health, ObservedAt: now},
			}
			result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, observation, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.ActionResultV2.Verification; got.Status != tc.want || got.EvidenceClass != unified.ActionEvidenceIndependent {
				t.Fatalf("verification = %+v, want %s / independent", got, tc.want)
			}
			if result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded {
				t.Fatal("readback changed execution history")
			}
		})
	}
}
