package hostagent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

const dockerLifecycleTestContainerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestDockerLifecycleManagerRestartPerformsOneMutationAndBoundedReadback(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	before := time.Now().UTC().Add(-time.Minute).Truncate(time.Nanosecond)
	after := before.Add(time.Minute)
	var calls [][]string
	manager := &localDockerLifecycleManager{now: func() time.Time { return after }, run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch len(calls) {
		case 1:
			return []byte(fmt.Sprintf(`{"Status":"running","Running":true,"StartedAt":%q}`, before.Format(time.RFC3339Nano))), nil
		case 2:
			return []byte("0"), nil
		case 3:
			return []byte(dockerLifecycleTestContainerID), nil
		case 4:
			return []byte(fmt.Sprintf(`{"Status":"running","Running":true,"StartedAt":%q}`, after.Format(time.RFC3339Nano))), nil
		case 5:
			return []byte("0"), nil
		default:
			return nil, fmt.Errorf("unexpected command")
		}
	}}
	req := dockerLifecycleTestRequest(t, before)
	result := manager.Apply(context.Background(), req)
	if !result.MutationStarted || !result.MutationCompleted || !result.ReadbackRan {
		t.Fatalf("result facts = %#v", result)
	}
	mutations := 0
	for _, call := range calls {
		if len(call) > 1 && call[1] == "restart" {
			mutations++
		}
	}
	if mutations != 1 {
		t.Fatalf("restart mutations = %d, calls=%#v", mutations, calls)
	}
}

func TestDockerLifecycleManagerStaleBeforeStateIsTripleZero(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	before := time.Now().UTC().Add(-time.Minute)
	mutations := 0
	manager := &localDockerLifecycleManager{now: time.Now, run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			mutations++
		}
		if len(args) > 2 && strings.Contains(args[2], ".State") {
			return []byte(fmt.Sprintf(`{"Status":"exited","Running":false,"StartedAt":%q}`, before.Format(time.RFC3339Nano))), nil
		}
		return []byte("0"), nil
	}}
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, before))
	if result.MutationStarted || result.MutationCompleted || result.ReadbackRan || mutations != 0 {
		t.Fatalf("stale preflight was not triple-zero: result=%#v mutations=%d", result, mutations)
	}
}

func TestDockerLifecycleManagerFailedInspectIsTripleZero(t *testing.T) {
	t.Setenv("DOCKER_CONTEXT", "")
	manager := &localDockerLifecycleManager{now: time.Now, run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("inspect unavailable")
	}}
	result := manager.Apply(context.Background(), dockerLifecycleTestRequest(t, time.Now().UTC().Add(-time.Minute)))
	if result.MutationStarted || result.MutationCompleted || result.ReadbackRan {
		t.Fatalf("failed inspect was not triple-zero: %#v", result)
	}
}

func dockerLifecycleTestRequest(t *testing.T, startedAt time.Time) agentexec.DockerContainerLifecyclePayload {
	t.Helper()
	req := agentexec.DockerContainerLifecyclePayload{RequestID: "action.dispatch.1", ActionID: "action", Operation: agentexec.DockerContainerOperationRestart, Runtime: "docker", ContainerID: dockerLifecycleTestContainerID, ExpectedState: "running", ExpectedStartedAt: startedAt, Timeout: 10}
	if err := agentexec.BindDockerContainerLifecyclePayload(&req); err != nil {
		t.Fatal(err)
	}
	return req
}
