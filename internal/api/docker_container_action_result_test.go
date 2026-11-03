package api

import (
	"encoding/json"
	"strings"
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

func TestDockerContainerExecutionResultPreservesBoundedPositiveAgentClockSkew(t *testing.T) {
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
	if !verification.Evidence[0].ObservedAt.Equal(facts.After.ObservedAt) || !verification.Evidence[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("evidence timestamps = observed %s received %s, want original observation and receipt boundary %s", verification.Evidence[0].ObservedAt, verification.Evidence[0].ReceivedAt, receivedAt)
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

func TestDockerContainerUpdateExecutionResultPreservesBoundedPositiveAgentClockSkew(t *testing.T) {
	receivedAt := time.Now().UTC()
	observedAt := receivedAt.Add(2 * time.Second)
	facts := agentexec.DockerContainerUpdateResultPayload{
		Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhaseComplete,
		MutationStarted: true, MutationCompleted: true, ReadbackRan: true, NewContainerID: dockerLifecycleTestID,
		After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: "running", Running: true, Health: agentexec.DockerContainerHealthHealthy, ObservedAt: observedAt},
	}
	result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, nil, receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	verification := result.ActionResultV2.Verification
	if verification.Status != unified.ActionVerificationConfirmed || len(verification.Evidence) != 1 {
		t.Fatalf("verification = %#v", verification)
	}
	if !verification.Evidence[0].ObservedAt.Equal(observedAt) || !verification.Evidence[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("evidence timestamps = observed %s received %s, want original observation and receipt boundary %s", verification.Evidence[0].ObservedAt, verification.Evidence[0].ReceivedAt, receivedAt)
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

func TestDockerContainerExecutionResultStartingHealthRemainsInconclusive(t *testing.T) {
	now := time.Now().UTC()
	facts := dockerResultFacts(now, true, false, false, true)
	facts.ContainerID = dockerLifecycleTestID
	observation := &dockerContainerPostconditionObservation{
		ObserverID: "daemon", TrustDomain: "docker-daemon:agent-1", Method: "docker_api_inspect",
		Snapshot: dockerObservationSnapshotFromLifecycle(facts.After), ReceivedAt: now,
	}
	observation.Snapshot.Health = agentexec.DockerContainerHealthStarting
	result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{ActionID: "action", Operation: agentexec.DockerContainerOperationRestart}, facts, observation, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActionResultV2.Execution.Status != unified.ActionExecutionInconclusive || result.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive || result.ActionResultV2.Verification.ReasonCode != "container_health_starting" {
		t.Fatalf("transitional health was treated as a final contradiction: %#v", result.ActionResultV2)
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
		{"starting healthcheck", "running", "running", "starting", true, unified.ActionVerificationInconclusive},
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
			if tc.health == "starting" && result.ActionResultV2.Verification.ReasonCode != "container_health_starting" {
				t.Fatalf("reason = %q, want container_health_starting", result.ActionResultV2.Verification.ReasonCode)
			}
			if result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded {
				t.Fatal("readback changed execution history")
			}
		})
	}
}

func TestDockerContainerUpdateAgentReadbackMustSupportRunningClaim(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, state, health string
		running             bool
		want                unified.ActionVerificationStatus
	}{
		{"healthy", "running", "healthy", true, unified.ActionVerificationConfirmed},
		{"no healthcheck", "running", "none", true, unified.ActionVerificationConfirmed},
		{"stopped preserved", "created", "none", false, unified.ActionVerificationConfirmed},
		{"unhealthy", "running", "unhealthy", true, unified.ActionVerificationContradicted},
		{"starting", "running", "starting", true, unified.ActionVerificationInconclusive},
		{"restarting", "restarting", "healthy", true, unified.ActionVerificationContradicted},
		{"unknown health", "running", "", true, unified.ActionVerificationInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := agentexec.DockerContainerUpdateResultPayload{
				Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhaseComplete,
				MutationStarted: true, MutationCompleted: true, ReadbackRan: true, NewContainerID: dockerLifecycleTestID,
				After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: tc.state, Running: tc.running, Health: tc.health, ObservedAt: now},
			}
			result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.ActionResultV2.Verification; got.Status != tc.want || got.EvidenceClass != unified.ActionEvidenceAgentAttested {
				t.Fatalf("verification = %+v, want %s / agent_attested", got, tc.want)
			}
			if tc.health == "starting" && result.ActionResultV2.Verification.ReasonCode != "container_health_starting" {
				t.Fatalf("reason = %q, want container_health_starting", result.ActionResultV2.Verification.ReasonCode)
			}
			if result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded {
				t.Fatal("readback changed execution history")
			}
		})
	}
}

// The execution and verification summaries are the sentences people read in
// the action result, so they must say what Pulse saw in plain words while the
// typed facts stay on the evidence.
func TestDockerContainerExecutionResultSummariesSayWhatPulseSaw(t *testing.T) {
	now := time.Now().UTC()
	beforeStart := now.Add(-time.Minute)
	for _, tc := range []struct {
		name, operation, state, health string
		running                        bool
		startedAt                      time.Time
		status                         unified.ActionVerificationStatus
		want                           string
	}{
		{name: "restart recovered", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthHealthy, startedAt: now, status: unified.ActionVerificationConfirmed, want: "The container restarted and is running and healthy."},
		{name: "restart without health check", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthNone, startedAt: now, status: unified.ActionVerificationConfirmed, want: "The container restarted and is running."},
		{name: "restart health failing", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthUnhealthy, startedAt: now, status: unified.ActionVerificationContradicted, want: "The container restarted, but its health check is failing. Check its logs."},
		{name: "restart crash loop", operation: agentexec.DockerContainerOperationRestart, state: "restarting", running: true, health: agentexec.DockerContainerHealthNone, startedAt: now, status: unified.ActionVerificationContradicted, want: "The container is not running. Docker reports it as restarting, which usually means it keeps crashing."},
		{name: "restart same start time", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart, status: unified.ActionVerificationContradicted, want: "The container is running, but its start time did not change, so it did not restart."},
		{name: "restart earlier start time", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart.Add(-time.Hour), status: unified.ActionVerificationContradicted, want: "The container is running, but its start time is not newer than before, so Pulse could not confirm a restart."},
		{name: "restart missing start time", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthNone, status: unified.ActionVerificationContradicted, want: "The container is running, but Pulse did not receive its start time, so it could not confirm a restart."},
		{name: "restart health pending", operation: agentexec.DockerContainerOperationRestart, state: "running", running: true, health: agentexec.DockerContainerHealthStarting, startedAt: now, status: unified.ActionVerificationInconclusive, want: "The container is running. Its health check was still starting when Pulse checked. Wait a moment, then refresh its status."},
		{name: "running state without running flag", operation: agentexec.DockerContainerOperationStart, state: "running", running: false, health: agentexec.DockerContainerHealthNone, startedAt: now, status: unified.ActionVerificationContradicted, want: "The container is not running. Docker reports it as running but not marked as running."},
		{name: "start exited again", operation: agentexec.DockerContainerOperationStart, state: "exited", running: false, health: agentexec.DockerContainerHealthNone, startedAt: now, status: unified.ActionVerificationContradicted, want: "The container is not running. Docker reports it as stopped."},
		{name: "start health failing", operation: agentexec.DockerContainerOperationStart, state: "running", running: true, health: agentexec.DockerContainerHealthUnhealthy, startedAt: now, status: unified.ActionVerificationContradicted, want: "The container started, but its health check is failing. Check its logs."},
		{name: "stop still running", operation: agentexec.DockerContainerOperationStop, state: "running", running: true, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart, status: unified.ActionVerificationContradicted, want: "The container is still running."},
		{name: "stop paused", operation: agentexec.DockerContainerOperationStop, state: "paused", running: false, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart, status: unified.ActionVerificationContradicted, want: "Pulse expected a stopped container. Docker reports it as paused."},
		{name: "stop paused but flagged running", operation: agentexec.DockerContainerOperationStop, state: "paused", running: true, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart, status: unified.ActionVerificationContradicted, want: "Pulse expected a stopped container. Docker reports it as paused."},
		{name: "stop confirmed", operation: agentexec.DockerContainerOperationStop, state: "exited", running: false, health: agentexec.DockerContainerHealthNone, startedAt: beforeStart, status: unified.ActionVerificationConfirmed, want: "The container is stopped."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := dockerResultFacts(now, true, true, true, true)
			facts.ContainerID = dockerLifecycleTestID
			req := agentexec.DockerContainerLifecyclePayload{ActionID: "action-1", Operation: tc.operation}
			observation := &dockerContainerPostconditionObservation{
				ObserverID: "docker-daemon:agent-1", TrustDomain: "docker-daemon:agent-1", Method: "typed_docker_daemon_observation",
				Snapshot:   agentexec.DockerContainerObservationSnapshot{ContainerID: dockerLifecycleTestID, State: tc.state, Running: tc.running, Health: tc.health, StartedAt: tc.startedAt, ObservedAt: now},
				ReceivedAt: now,
			}
			result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", req, facts, observation, now)
			if err != nil {
				t.Fatal(err)
			}
			truth := result.ActionResultV2
			if truth.Verification.Status != tc.status || truth.Verification.Summary != tc.want {
				t.Fatalf("verification = %q %q, want %q %q", truth.Verification.Status, truth.Verification.Summary, tc.status, tc.want)
			}
			_, past := dockerLifecycleVerbs(tc.operation)
			if want := "Docker " + past + " the container."; truth.Execution.Summary != want {
				t.Fatalf("execution summary = %q, want %q", truth.Execution.Summary, want)
			}
			if len(truth.Verification.Evidence) != 1 || !strings.HasPrefix(truth.Verification.Evidence[0].Summary, "Direct daemon observation:") {
				t.Fatalf("typed observation facts left the evidence: %#v", truth.Verification.Evidence)
			}
			if !strings.HasPrefix(result.Output, "Container lifecycle: phase=") {
				t.Fatalf("legacy output lost the typed facts: %q", result.Output)
			}
		})
	}
}

// The agent receipt does not carry health on the wire, so without a daemon
// observation the server cannot confirm a start or restart. The summary must
// say Pulse lacked the health information rather than blame Docker.
func TestDockerContainerAgentOnlySummaryAfterWireRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, state string
		running     bool
		want        string
	}{
		{name: "running", state: "running", running: true, want: "Docker reports the container as running. Pulse could not get its health information, so it could not confirm the result."},
		{name: "exited", state: "exited", running: false, want: "Docker reports the container as stopped. Pulse could not get its health information, so it could not confirm the result."},
		{name: "restarting", state: "restarting", running: true, want: "Docker reports the container as restarting, which usually means it keeps crashing. Pulse could not get its health information, so it could not confirm the result."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := dockerResultFacts(now, true, true, true, true)
			facts.After.State, facts.After.Running = tc.state, tc.running
			encoded, err := json.Marshal(facts)
			if err != nil {
				t.Fatal(err)
			}
			var wire agentexec.DockerContainerLifecycleResultPayload
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.After.Health != "" {
				t.Fatalf("fixture expects health to stay off the wire, got %q", wire.After.Health)
			}
			result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, wire, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			verification := result.ActionResultV2.Verification
			if verification.Status != unified.ActionVerificationInconclusive || verification.ReasonCode != "container_health_unknown" || verification.Summary != tc.want {
				t.Fatalf("verification = %q %q %q, want %q", verification.Status, verification.ReasonCode, verification.Summary, tc.want)
			}
			if len(verification.Evidence) != 1 || !strings.HasPrefix(verification.Evidence[0].Summary, "Container lifecycle: phase=") {
				t.Fatalf("agent readback facts left the evidence: %#v", verification.Evidence)
			}
		})
	}
}

func TestDockerContainerExecutionSummariesCarryUncertaintyAndAgentError(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name               string
		started, completed bool
		errText            string
		want               string
	}{
		{name: "refused", errText: "container preflight state no longer matches the request", want: "The agent did not restart the container. Agent said: container preflight state no longer matches the request."},
		{name: "partial", started: true, errText: "context deadline exceeded", want: "Docker was asked to restart the container, but did not confirm it, so the change may or may not have happened. Agent said: context deadline exceeded."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := dockerResultFacts(now, tc.started, tc.completed, false, true)
			facts.ReasonCode = agentexec.ActionRefusalTargetStateChanged
			facts.Error = tc.errText
			result, err := dockerContainerExecutionResult("app-container:fixture", "agent-1", agentexec.DockerContainerLifecyclePayload{Operation: agentexec.DockerContainerOperationRestart}, facts, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.ActionResultV2.Execution.Summary; got != tc.want {
				t.Fatalf("execution summary = %q, want %q", got, tc.want)
			}
			if !strings.Contains(result.Output, "error="+tc.errText) {
				t.Fatalf("legacy output lost the agent error: %q", result.Output)
			}
		})
	}
}

func TestDockerContainerUpdateSummariesSayWhatPulseSaw(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, agentState, state, health string
		agentRunning, running           bool
		observedID                      string
		status                          unified.ActionVerificationStatus
		want                            string
	}{
		{name: "running healthy", agentState: "running", agentRunning: true, state: "running", running: true, health: agentexec.DockerContainerHealthHealthy, observedID: dockerLifecycleTestID, status: unified.ActionVerificationConfirmed, want: "The updated container is running and healthy."},
		{name: "kept stopped", agentState: "exited", state: "exited", health: agentexec.DockerContainerHealthNone, observedID: dockerLifecycleTestID, status: unified.ActionVerificationConfirmed, want: "The updated container is in place. Docker reports it as stopped."},
		{name: "kept created", agentState: "created", state: "created", health: agentexec.DockerContainerHealthNone, observedID: dockerLifecycleTestID, status: unified.ActionVerificationConfirmed, want: "The updated container is in place. Docker reports it as created but not started."},
		{name: "health failing", agentState: "running", agentRunning: true, state: "running", running: true, health: agentexec.DockerContainerHealthUnhealthy, observedID: dockerLifecycleTestID, status: unified.ActionVerificationContradicted, want: "The updated container is running, but its health check is failing. Check its logs."},
		{name: "exited after readback", agentState: "running", agentRunning: true, state: "exited", health: agentexec.DockerContainerHealthNone, observedID: dockerLifecycleTestID, status: unified.ActionVerificationContradicted, want: "Docker reports the updated container as stopped."},
		{name: "health pending", agentState: "running", agentRunning: true, state: "running", running: true, health: agentexec.DockerContainerHealthStarting, observedID: dockerLifecycleTestID, status: unified.ActionVerificationInconclusive, want: "The updated container is running. Its health check was still starting when Pulse checked. Wait a moment, then refresh its status."},
		{name: "different container", agentState: "running", agentRunning: true, state: "running", running: true, health: agentexec.DockerContainerHealthHealthy, observedID: strings.Repeat("f", 64), status: unified.ActionVerificationContradicted, want: "Pulse found a different container than the one the update created."},
		{name: "readback unavailable", state: "running", running: true, health: agentexec.DockerContainerHealthHealthy, observedID: dockerLifecycleTestID, status: unified.ActionVerificationInconclusive, want: "The agent did not report the updated container's state, so Pulse could not confirm the update."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := agentexec.DockerContainerUpdateResultPayload{
				Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhaseComplete,
				MutationStarted: true, MutationCompleted: true, ReadbackRan: tc.agentState != "", NewContainerID: dockerLifecycleTestID,
				After: agentexec.DockerContainerLifecycleSnapshot{ContainerID: dockerLifecycleTestID, State: tc.agentState, Running: tc.agentRunning, ObservedAt: now},
			}
			observation := &dockerContainerPostconditionObservation{
				ObserverID: "docker-daemon:agent-1", TrustDomain: "docker-daemon:agent-1", Method: "typed_docker_daemon_observation",
				Snapshot:   agentexec.DockerContainerObservationSnapshot{ContainerID: tc.observedID, State: tc.state, Running: tc.running, Health: tc.health, ObservedAt: now},
				ReceivedAt: now,
			}
			result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, observation, now)
			if err != nil {
				t.Fatal(err)
			}
			truth := result.ActionResultV2
			if truth.Verification.Status != tc.status || truth.Verification.Summary != tc.want {
				t.Fatalf("verification = %q %q, want %q %q", truth.Verification.Status, truth.Verification.Summary, tc.status, tc.want)
			}
			if truth.Execution.Summary != "Docker replaced the container with one using the new image." {
				t.Fatalf("execution summary = %q", truth.Execution.Summary)
			}
			if !strings.HasPrefix(result.Output, "Container update: phase=") {
				t.Fatalf("legacy output lost the typed facts: %q", result.Output)
			}
		})
	}
}

func TestDockerContainerUpdateExecutionSummariesCarryUncertaintyAndAgentError(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name    string
		started bool
		errText string
		want    string
	}{
		{name: "not started", errText: "Failed to pull image: manifest unknown: ", want: "The update stopped before changing the container. Agent said: Failed to pull image: manifest unknown."},
		{name: "partial", started: true, errText: "context deadline exceeded", want: "The update started, but Pulse could not confirm it completed. Agent said: context deadline exceeded."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := agentexec.DockerContainerUpdateResultPayload{
				Operation: agentexec.DockerContainerOperationUpdate, ActionID: "action-update", ExecutionPhase: agentexec.DockerContainerPhasePreflight,
				MutationStarted: tc.started, Error: tc.errText,
			}
			result, err := dockerContainerUpdateExecutionResult("app-container:fixture", "agent-1", facts, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.ActionResultV2.Execution.Summary; got != tc.want {
				t.Fatalf("execution summary = %q, want %q", got, tc.want)
			}
		})
	}
}
