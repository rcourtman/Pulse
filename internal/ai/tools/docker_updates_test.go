package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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

	exec = NewPulseToolExecutor(ExecutorConfig{
		UpdatesProvider: updatesProvider,
		StateProvider:   &mockStateProvider{state: state},
		ControlLevel:    ControlLevelSuggest,
	})
	result, err = exec.executeCheckDockerUpdates(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "POST /api/agents/docker/hosts/host1/check-updates")

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
