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

	result, err := exec.ExecuteTool(context.Background(), "pulse_get_disk_health", map[string]interface{}{})
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

	result, err := exec.ExecuteTool(context.Background(), "pulse_get_temperatures", map[string]interface{}{})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
}
