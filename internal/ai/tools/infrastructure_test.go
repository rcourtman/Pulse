package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGetDiskHealth(t *testing.T) {
	diskHealthProv := &mockDiskHealthProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		DiskHealthProvider: diskHealthProv,
		ControlLevel:       ControlLevelReadOnly,
	})

	expectedHosts := []*unifiedresources.HostView{
		newHostView("host-resource-1", "Node 1", "host1", "node1", nil, nil, nil),
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
	stateProv.On("ReadSnapshot").Return(state)

	// Use pulse_metrics tool with type: "temperatures"
	result, err := exec.ExecuteTool(context.Background(), "pulse_metrics", map[string]interface{}{
		"type": "temperatures",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
}

// The temperature tool must see the same Proxmox-only observations as the
// resource API. A node does not need a second agent to expose its sensors.
func TestExecuteGetTemperaturesCanonicalSources(t *testing.T) {
	collected := time.Date(2026, 9, 5, 19, 19, 11, 0, time.UTC)
	hot := 94.0
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources([]unifiedresources.Resource{
		{ID: "agent-hot", Type: unifiedresources.ResourceTypeAgent, Name: "Hot node", Proxmox: &unifiedresources.ProxmoxData{
			NodeName: "hot-node", Temperature: &hot, TemperatureDetails: &models.Temperature{
				Available: true, HasCPU: true, CPUPackage: 94, CPUMax: 94, LastUpdate: collected,
				Cores: []models.CoreTemp{{Core: 0, Temp: 87}},
				NVMe:  []models.NVMeTemp{{Device: "nvme0", Temp: 60}},
				SMART: []models.DiskTemp{{Device: "sda", Temperature: 38}, {Device: "sdb", Temperature: 30, StandbySkipped: true}},
				GPU:   []models.GPUTemp{{Device: "gpu0", Edge: 55, Junction: 65}},
			},
		}, Agent: &unifiedresources.AgentData{Hostname: "hot-node", Sensors: &unifiedresources.HostSensorMeta{TemperatureCelsius: map[string]float64{"cpu_package": 80}, FanRPM: map[string]float64{"fan1": 2000}}}},
		{ID: "agent-empty", Type: unifiedresources.ResourceTypeAgent, Name: "empty", Proxmox: &unifiedresources.ProxmoxData{NodeName: "empty"}},
	})
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: registry, ControlLevel: ControlLevelReadOnly})
	for _, filter := range []map[string]interface{}{
		{}, {"host": "hot-node"}, {"host": "Hot node"}, {"host": "agent-hot"}, {"resource_id": "agent-hot"},
	} {
		result, err := executor.executeGetTemperatures(context.Background(), filter)
		require.NoError(t, err)
		var rows []struct {
			ResourceID string             `json:"resource_id"`
			Source     string             `json:"source"`
			CPU        map[string]float64 `json:"cpu_temps"`
			Disks      map[string]float64 `json:"disk_temps"`
			Fans       map[string]float64 `json:"fan_rpm"`
			Other      map[string]float64 `json:"other_temps"`
			Updated    string             `json:"last_updated"`
		}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &rows))
		require.Len(t, rows, 2)
		assert.Equal(t, "agent-hot", rows[0].ResourceID)
		assert.Equal(t, "agent-hot", rows[1].ResourceID)
		assert.Equal(t, "agent", rows[0].Source)
		assert.Equal(t, 80.0, rows[0].CPU["cpu_package"])
		assert.Equal(t, 2000.0, rows[0].Fans["fan1"])
		assert.Equal(t, "proxmox", rows[1].Source)
		assert.Equal(t, 94.0, rows[1].CPU["cpu_max"])
		assert.Equal(t, 87.0, rows[1].CPU["cpu_core_0"])
		assert.Equal(t, 60.0, rows[1].Disks["nvme0"])
		assert.Equal(t, 38.0, rows[1].Disks["sda"])
		assert.NotContains(t, rows[1].Disks, "sdb")
		assert.Equal(t, 65.0, rows[1].Other["gpu0_junction"])
		assert.Equal(t, collected.Format(time.RFC3339), rows[1].Updated)
	}
	for _, filter := range []map[string]interface{}{{"host": "empty"}, {"resource_id": "missing"}, {"host": "hot-node", "resource_id": "agent-empty"}} {
		result, err := executor.executeGetTemperatures(context.Background(), filter)
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "No temperature data available")
		assert.NotContains(t, result.Content[0].Text, "installed")
	}
}

func TestExecuteGetTemperaturesProxmoxWithoutAgent(t *testing.T) {
	state := &mockStateProvider{}
	state.On("ReadSnapshot").Return(models.StateSnapshot{Nodes: []models.Node{{
		ID: "cluster-node", Name: "hot-node", Status: "online",
		Temperature: &models.Temperature{Available: true, HasCPU: true, CPUPackage: 94},
	}}})
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: state, ControlLevel: ControlLevelReadOnly})
	result, err := executor.ExecuteTool(context.Background(), "pulse_metrics", map[string]interface{}{"type": "temperatures", "host": "hot-node"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var rows []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "proxmox", rows[0]["source"])
	assert.NotEmpty(t, rows[0]["resource_id"])
	assert.Equal(t, 94.0, rows[0]["cpu_temps"].(map[string]interface{})["cpu_package"])
}
