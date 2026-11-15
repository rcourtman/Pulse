package hostagent

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

func TestRunDockerContainerUpdateForwardsTheStepThatStoppedIt(t *testing.T) {
	payload := boundUpdatePayload(t)
	client := newUpdateTestClient(stubDockerUpdater{outcome: agentexec.DockerContainerUpdateOutcome{
		ContainerName: "app", Error: "Failed to pull image nginx:latest: toomanyrequests",
		ReasonCode: agentexec.ActionRefusalImagePullFailed,
	}})

	result := client.runDockerContainerUpdate(context.Background(), payload)
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(payload, result); err != nil {
		t.Fatalf("coded failure failed the wire contract: %v (%+v)", err, result)
	}
	if result.MutationStarted || result.ReasonCode != agentexec.ActionRefusalImagePullFailed {
		t.Fatalf("result = %+v, want the pull failure code before any change", result)
	}
}

func TestRunDockerContainerUpdateDropsTheStepCodeOnceAChangeStarted(t *testing.T) {
	payload := boundUpdatePayload(t)
	client := newUpdateTestClient(stubDockerUpdater{outcome: agentexec.DockerContainerUpdateOutcome{
		ContainerName: "app", Error: "Failed to rename container for backup: busy",
		RollbackAttempted: true, RolledBack: true, ReasonCode: agentexec.ActionRefusalContainerRenameFailed,
	}})

	result := client.runDockerContainerUpdate(context.Background(), payload)
	if err := agentexec.ValidateDockerContainerUpdateResultForRequest(payload, result); err != nil {
		t.Fatalf("rollback result failed the wire contract: %v (%+v)", err, result)
	}
	if !result.MutationStarted || result.ReasonCode != "" {
		t.Fatalf("result = %+v, want a started change with no refusal code", result)
	}
}
