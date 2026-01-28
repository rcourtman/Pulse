package tools

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPulseToolExecutor_ExecuteListBackups(t *testing.T) {
	backupProv := &mockBackupProvider{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		BackupProvider: backupProv,
		ControlLevel:   ControlLevelReadOnly,
	})

	expectedBackups := models.Backups{
		PBS: []models.PBSBackup{
			{VMID: "101", BackupType: "vm", BackupTime: time.Now(), Instance: "pbs1", Datastore: "ds1", Size: 1024 * 1024 * 1024},
		},
		PVE: models.PVEBackups{
			StorageBackups: []models.StorageBackup{
				{VMID: 102, Time: time.Now(), Size: 2 * 1024 * 1024 * 1024, Storage: "local"},
			},
			BackupTasks: []models.BackupTask{
				{VMID: 101, Node: "node1", Status: "OK", StartTime: time.Now()},
			},
		},
	}
	backupProv.On("GetBackups").Return(expectedBackups)
	backupProv.On("GetPBSInstances").Return([]models.PBSInstance{})

	// Use pulse_storage tool with type: "backups"
	result, err := exec.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type": "backups",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestPulseToolExecutor_ExecuteControlGuest(t *testing.T) {
	stateProv := &mockStateProvider{}
	agentSrv := &mockAgentServer{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: stateProv,
		AgentServer:   agentSrv,
		ControlLevel:  ControlLevelAutonomous,
	})

	state := models.StateSnapshot{
		VMs: []models.VM{
			{VMID: 100, Name: "test-vm", Node: "node1", Status: "running", Instance: "pve1"},
		},
	}
	stateProv.On("GetState").Return(state)

	agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
		{AgentID: "agent1", Hostname: "node1"},
	})

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "qm stop 100"
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "OK",
		ExitCode: 0,
	}, nil)

	// Use pulse_control tool with type: "guest"
	result, err := exec.ExecuteTool(context.Background(), "pulse_control", map[string]interface{}{
		"type":     "guest",
		"guest_id": "100",
		"action":   "stop",
	})
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "Successfully executed 'stop'")
}

func TestPulseToolExecutor_ExecuteControlDocker(t *testing.T) {
	stateProv := &mockStateProvider{}
	agentSrv := &mockAgentServer{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: stateProv,
		AgentServer:   agentSrv,
		ControlLevel:  ControlLevelAutonomous,
	})

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "host1",
				Hostname: "docker1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "nginx"},
				},
			},
		},
	}
	stateProv.On("GetState").Return(state)

	agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
		{AgentID: "agent1", Hostname: "docker1"},
	})

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "docker restart nginx"
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "OK",
		ExitCode: 0,
	}, nil)

	// Use pulse_docker tool with action: "control"
	result, err := exec.ExecuteTool(context.Background(), "pulse_docker", map[string]interface{}{
		"action":    "control",
		"container": "nginx",
		"operation": "restart",
	})
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "Successfully executed 'docker restart'")
}
