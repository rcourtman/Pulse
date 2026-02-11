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
	assert.Equal(t, "host1", resp.HostID)
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
