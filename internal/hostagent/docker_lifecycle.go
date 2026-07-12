package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

var dockerContextNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

type dockerLifecycleManager interface {
	Apply(context.Context, agentexec.DockerContainerLifecyclePayload) agentexec.DockerContainerLifecycleResultPayload
}

type dockerLifecycleCommandRunner func(context.Context, string, ...string) ([]byte, error)

type localDockerLifecycleManager struct {
	run dockerLifecycleCommandRunner
	now func() time.Time
}

func newLocalDockerLifecycleManager() *localDockerLifecycleManager {
	return &localDockerLifecycleManager{
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		now: time.Now,
	}
}

func (m *localDockerLifecycleManager) Apply(ctx context.Context, req agentexec.DockerContainerLifecyclePayload) (result agentexec.DockerContainerLifecycleResultPayload) {
	started := m.currentTime()
	result = agentexec.DockerContainerLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
		OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, ContainerID: req.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhasePreflight,
	}
	defer func() { result.Duration = time.Since(started).Milliseconds() }()
	if err := agentexec.ValidateDockerContainerLifecyclePayload(&req); err != nil {
		result.Error = "typed container lifecycle preflight refused"
		return result
	}
	before, err := m.inspect(ctx, req.Runtime, req.ContainerID)
	result.Before = before
	if err != nil {
		result.Error = "container preflight inspect unavailable"
		return result
	}
	if !dockerLifecycleBeforeMatches(req, before) {
		result.Error = "container preflight state no longer matches the request"
		return result
	}

	result.ExecutionPhase = agentexec.DockerContainerPhaseMutate
	result.MutationStarted = true
	verb := strings.TrimSuffix(req.Operation, "_container")
	if _, err := m.command(ctx, req.Runtime, verb, req.ContainerID); err != nil {
		result.Error = "container lifecycle mutation did not complete"
		return result
	}
	result.MutationCompleted = true
	result.ExecutionPhase = agentexec.DockerContainerPhaseVerify
	after, err := m.inspect(ctx, req.Runtime, req.ContainerID)
	result.After = after
	if err != nil {
		result.Error = "container postcondition inspect unavailable"
		return result
	}
	result.ReadbackRan = true
	if dockerLifecyclePostcondition(req, before, after) {
		result.ExecutionPhase = agentexec.DockerContainerPhaseComplete
		return result
	}
	result.Error = "container postcondition contradicted the requested state"
	return result
}

func (m *localDockerLifecycleManager) command(ctx context.Context, runtime string, args ...string) ([]byte, error) {
	if m == nil || m.run == nil {
		return nil, fmt.Errorf("container runtime command runner unavailable")
	}
	if runtime != "docker" && runtime != "podman" {
		return nil, fmt.Errorf("unsupported container runtime")
	}
	if runtime == "docker" {
		if dockerContext := strings.TrimSpace(os.Getenv("DOCKER_CONTEXT")); dockerContext != "" {
			if !dockerContextNamePattern.MatchString(dockerContext) {
				return nil, fmt.Errorf("invalid Docker context name")
			}
			args = append([]string{"--context", dockerContext}, args...)
		}
	}
	return m.run(ctx, runtime, args...)
}

func (m *localDockerLifecycleManager) inspect(ctx context.Context, runtime, containerID string) (agentexec.DockerContainerLifecycleSnapshot, error) {
	const format = `{{json .State}}`
	raw, err := m.command(ctx, runtime, "inspect", "--format", format, containerID)
	if err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, err
	}
	var state struct {
		Status       string `json:"Status"`
		Running      bool   `json:"Running"`
		StartedAt    string `json:"StartedAt"`
		RestartCount int    `json:"RestartCount"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &state); err != nil {
		return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("decode container state: %w", err)
	}
	startedAt := time.Time{}
	if value := strings.TrimSpace(state.StartedAt); value != "" && !strings.HasPrefix(value, "0001-") {
		startedAt, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return agentexec.DockerContainerLifecycleSnapshot{}, fmt.Errorf("decode container start time: %w", err)
		}
	}
	// Docker exposes RestartCount at the inspect root. Query it separately to
	// keep the state envelope bounded and avoid persisting labels or config.
	restartRaw, restartErr := m.command(ctx, runtime, "inspect", "--format", `{{.RestartCount}}`, containerID)
	if restartErr == nil {
		if count, parseErr := strconv.Atoi(strings.TrimSpace(string(restartRaw))); parseErr == nil && count >= 0 {
			state.RestartCount = count
		}
	}
	return agentexec.DockerContainerLifecycleSnapshot{
		ContainerID: strings.ToLower(containerID), State: strings.ToLower(strings.TrimSpace(state.Status)), Running: state.Running,
		StartedAt: startedAt.UTC(), RestartCount: state.RestartCount, ObservedAt: m.currentTime(),
	}, nil
}

func (m *localDockerLifecycleManager) currentTime() time.Time {
	if m != nil && m.now != nil {
		return m.now().UTC()
	}
	return time.Now().UTC()
}

func dockerLifecycleBeforeMatches(req agentexec.DockerContainerLifecyclePayload, before agentexec.DockerContainerLifecycleSnapshot) bool {
	if !strings.EqualFold(req.ContainerID, before.ContainerID) || !strings.EqualFold(req.ExpectedState, before.State) {
		return false
	}
	if !req.ExpectedStartedAt.IsZero() && !req.ExpectedStartedAt.Equal(before.StartedAt) {
		return false
	}
	return true
}

func dockerLifecyclePostcondition(req agentexec.DockerContainerLifecyclePayload, before, after agentexec.DockerContainerLifecycleSnapshot) bool {
	if !strings.EqualFold(req.ContainerID, after.ContainerID) {
		return false
	}
	switch req.Operation {
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
