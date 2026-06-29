package tools

import (
	"context"
	"encoding/json"
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

	mediaErrors := int64(3)
	expectedHosts := []models.Host{
		{ID: "host1", Hostname: "node1", DisplayName: "Node 1", Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device: "/dev/nvme0n1",
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					MediaErrors: &mediaErrors,
				},
			}},
		}},
	}
	diskHealthProv.On("GetHosts").Return(expectedHosts)

	// Use pulse_storage tool with type: "disk_health"
	result, err := exec.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type": "disk_health",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	var response DiskHealthResponse
	assert.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
	if assert.Len(t, response.Hosts, 1) && assert.Len(t, response.Hosts[0].SMART, 1) {
		if assert.NotNil(t, response.Hosts[0].SMART[0].Attributes) && assert.NotNil(t, response.Hosts[0].SMART[0].Attributes.MediaErrors) {
			assert.Equal(t, int64(3), *response.Hosts[0].SMART[0].Attributes.MediaErrors)
		}
	}
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
