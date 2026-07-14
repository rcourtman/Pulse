package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCheckDockerUpdates(t *testing.T) {
	ctx := context.Background()

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeCheckDockerUpdates(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Docker update checking not available. Ensure updates provider is configured.", result.Content[0].Text)

	updatesProvider := &mockUpdatesProvider{}
	exec = NewPulseToolExecutor(ExecutorConfig{UpdatesProvider: updatesProvider})
	result, err = exec.executeCheckDockerUpdates(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "host is required")

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "host1", Hostname: "dock1", DisplayName: "Dock One"},
		},
	}

	updatesProvider.On("TriggerUpdateCheck", "host1").Return(DockerCommandStatus{
		ID:     "cmd1",
		Type:   "check",
		Status: "queued",
	}, nil).Once()

	exec = NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
	})
	result, err = exec.executeCheckDockerUpdates(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)

	var resp DockerCheckUpdatesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "host1", resp.TargetID)
	assert.Equal(t, "Dock One", resp.HostName)
	assert.Equal(t, "cmd1", resp.CommandID)

	updatesProvider.On("TriggerUpdateCheck", "host1").Return(DockerCommandStatus{}, errors.New("boom")).Once()
	exec = NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
	})
	result, err = exec.executeCheckDockerUpdates(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "Failed to trigger update check")
}

func TestPulseDockerToolSchemaRequiresHostForEveryAdvertisedAction(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	var dockerTool Tool
	for _, tool := range exec.registry.ListTools(exec.invocationPolicy()) {
		if tool.Name == "pulse_docker" {
			dockerTool = tool
			break
		}
	}

	require.Equal(t, "pulse_docker", dockerTool.Name)
	assert.Contains(t, dockerTool.InputSchema.Required, "action")
	assert.Contains(t, dockerTool.InputSchema.Required, "host")
	assert.Contains(t, dockerTool.Description, "Every advertised action requires")
	assert.Contains(t, dockerTool.InputSchema.Properties["host"].Description, "Required")
}

func TestExecuteCheckDockerUpdates_RetriesTransientError(t *testing.T) {
	origSleep := dockerUpdateQueueSleepFn
	dockerUpdateQueueSleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { dockerUpdateQueueSleepFn = origSleep })

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "host1", Hostname: "dock1", DisplayName: "Dock One"},
		},
	}

	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("TriggerUpdateCheck", "host1").Return(DockerCommandStatus{}, errors.New("queue temporarily unavailable")).Once()
	updatesProvider.On("TriggerUpdateCheck", "host1").Return(DockerCommandStatus{
		ID:     "cmd-retry",
		Type:   "check",
		Status: "queued",
	}, nil).Once()

	exec := NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
	})

	result, err := exec.executeCheckDockerUpdates(context.Background(), map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp DockerCheckUpdatesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "cmd-retry", resp.CommandID)
	updatesProvider.AssertExpectations(t)
}

func TestExecuteCheckDockerUpdates_DoesNotRetryNonTransientError(t *testing.T) {
	origSleep := dockerUpdateQueueSleepFn
	dockerUpdateQueueSleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { dockerUpdateQueueSleepFn = origSleep })

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "host1", Hostname: "dock1", DisplayName: "Dock One"},
		},
	}

	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("TriggerUpdateCheck", "host1").Return(DockerCommandStatus{}, errors.New("invalid host id")).Once()

	exec := NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
	})

	result, err := exec.executeCheckDockerUpdates(context.Background(), map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Failed to trigger update check")
	assert.NotContains(t, result.Content[0].Text, "after 3 attempts")
	updatesProvider.AssertNumberOfCalls(t, "TriggerUpdateCheck", 1)
}

func TestExecuteUpdateDockerContainer_RetriesTransientError(t *testing.T) {
	origSleep := dockerUpdateQueueSleepFn
	dockerUpdateQueueSleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { dockerUpdateQueueSleepFn = origSleep })
	origVerifySleep := dockerUpdateVerifySleepFn
	dockerUpdateVerifySleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { dockerUpdateVerifySleepFn = origVerifySleep })

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:          "host1",
				Hostname:    "dock1",
				DisplayName: "Dock One",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "/nginx"},
				},
			},
		},
	}

	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("IsUpdateActionsEnabled").Return(true).Once()
	updatesProvider.On("UpdateContainer", "host1", "c1", "nginx").Return(DockerCommandStatus{}, errors.New("i/o timeout")).Once()
	updatesProvider.On("UpdateContainer", "host1", "c1", "nginx").Return(DockerCommandStatus{
		ID:     "cmd-update",
		Type:   "update",
		Status: "queued",
	}, nil).Once()
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID:      "cmd-update",
		Type:    "update",
		Status:  "completed",
		Message: "Container nginx updated successfully",
	}, true)

	exec := NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
		ControlLevel:    ControlLevelAutonomous,
	})

	result, err := exec.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "dock1",
		"container": "c1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp DockerUpdateContainerResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "cmd-update", resp.CommandID)
	updatesProvider.AssertExpectations(t)
}

func newDockerUpdateExecutor(t *testing.T, updatesProvider *mockUpdatesProvider) *PulseToolExecutor {
	t.Helper()

	origVerifySleep := dockerUpdateVerifySleepFn
	dockerUpdateVerifySleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { dockerUpdateVerifySleepFn = origVerifySleep })

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:          "host1",
				Hostname:    "dock1",
				DisplayName: "Dock One",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "/nginx"},
				},
			},
		},
	}

	return NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
		ControlLevel:    ControlLevelAutonomous,
	})
}

func TestExecuteUpdateDockerContainer_VerifiedSuccess(t *testing.T) {
	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("IsUpdateActionsEnabled").Return(true).Once()
	updatesProvider.On("UpdateContainer", "host1", "c1", "nginx").Return(DockerCommandStatus{
		ID:     "cmd-update",
		Type:   "update_container",
		Status: "queued",
	}, nil).Once()
	// The command is dispatched and in progress first, then completes.
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID: "cmd-update", Status: "dispatched",
	}, true).Once()
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID: "cmd-update", Status: "in_progress", Message: "Pulling image nginx...",
	}, true).Once()
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID: "cmd-update", Status: "completed", Message: "Container nginx updated successfully",
	}, true).Once()

	exec := newDockerUpdateExecutor(t, updatesProvider)
	result, err := exec.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "dock1",
		"container": "c1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp DockerUpdateContainerResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "Update verified")
	require.NotNil(t, resp.Verification)
	assert.Equal(t, true, resp.Verification["confirmed"])
	assert.Equal(t, "verified", resp.Verification["outcome"])
	assert.Equal(t, "command_status", resp.Verification["method"])
	updatesProvider.AssertExpectations(t)
}

func TestExecuteUpdateDockerContainer_VerifiedFailure(t *testing.T) {
	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("IsUpdateActionsEnabled").Return(true).Once()
	updatesProvider.On("UpdateContainer", "host1", "c1", "nginx").Return(DockerCommandStatus{
		ID:     "cmd-update",
		Type:   "update_container",
		Status: "queued",
	}, nil).Once()
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID:            "cmd-update",
		Status:        "failed",
		FailureReason: "New container crashed immediately (exit code 1)",
	}, true).Once()

	exec := newDockerUpdateExecutor(t, updatesProvider)
	result, err := exec.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "dock1",
		"container": "c1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	var resp DockerUpdateContainerResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "failed")
	assert.Contains(t, resp.Message, "crashed immediately")
	require.NotNil(t, resp.Verification)
	assert.Equal(t, false, resp.Verification["confirmed"])
	assert.Equal(t, "failed", resp.Verification["outcome"])
	updatesProvider.AssertExpectations(t)
}

func TestExecuteUpdateDockerContainer_InconclusiveAfterWindow(t *testing.T) {
	updatesProvider := &mockUpdatesProvider{}
	updatesProvider.On("IsUpdateActionsEnabled").Return(true).Once()
	updatesProvider.On("UpdateContainer", "host1", "c1", "nginx").Return(DockerCommandStatus{
		ID:     "cmd-update",
		Type:   "update_container",
		Status: "queued",
	}, nil).Once()
	// The update never reaches a terminal state within the window.
	updatesProvider.On("GetCommandStatus", "cmd-update").Return(DockerCommandStatus{
		ID: "cmd-update", Status: "in_progress", Message: "Pulling image nginx...",
	}, true)

	exec := newDockerUpdateExecutor(t, updatesProvider)
	result, err := exec.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "dock1",
		"container": "c1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp DockerUpdateContainerResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.True(t, resp.Success) // queueing succeeded; completion is explicitly unverified
	assert.Contains(t, resp.Message, "NOT yet verified")
	require.NotNil(t, resp.Verification)
	assert.Equal(t, false, resp.Verification["confirmed"])
	assert.Equal(t, "inconclusive", resp.Verification["outcome"])
	updatesProvider.AssertNumberOfCalls(t, "GetCommandStatus", dockerUpdateVerifyMaxAttempts)
}
