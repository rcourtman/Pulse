package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestExecuteGetDiskHealth(t *testing.T) {
	diskHealthProv := &mockDiskHealthProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		DiskHealthProvider: diskHealthProv,
		ControlLevel:       ControlLevelReadOnly,
	})

	expectedHosts := []models.Host{
		{ID: "host1", Hostname: "node1", DisplayName: "Node 1"},
	}
	diskHealthProv.On("GetHosts").Return(expectedHosts)

	// Use pulse_storage tool with type: "disk_health"
	result, err := exec.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type": "disk_health",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestExecuteGetTemperatures(t *testing.T) {
	stateProv := &mockStateProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: stateProv,
		ControlLevel:  ControlLevelReadOnly,
	})

	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "host1", Hostname: "node1", DisplayName: "Node 1", Sensors: models.HostSensorSummary{
				TemperatureCelsius: map[string]float64{"CPU": 45.0},
			}},
		},
	}
	stateProv.On("GetState").Return(state)

	// Use pulse_metrics tool with type: "temperatures"
	result, err := exec.ExecuteTool(context.Background(), "pulse_metrics", map[string]interface{}{
		"type": "temperatures",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
}
