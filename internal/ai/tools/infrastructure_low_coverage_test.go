package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRecoveryPointsProvider struct {
	points []recovery.RecoveryPoint
}

func (s *stubRecoveryPointsProvider) ListPoints(_ context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	filtered := make([]recovery.RecoveryPoint, 0, len(s.points))
	for _, p := range s.points {
		if opts.Provider != "" && p.Provider != opts.Provider {
			continue
		}
		if opts.Kind != "" && p.Kind != opts.Kind {
			continue
		}
		filtered = append(filtered, p)
	}

	total := len(filtered)
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	if offset >= len(filtered) {
		return []recovery.RecoveryPoint{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], total, nil
}

func TestExecuteGetCephStatus(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeGetCephStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No Ceph clusters found. Ceph may not be configured or data is not yet available.", result.Content[0].Text)

	cephProvider := &stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				Name: "alpha",
				Type: unifiedresources.ResourceTypeCeph,
				Ceph: &unifiedresources.CephMeta{
					HealthStatus:  "HEALTH_OK",
					HealthMessage: "ok",
					NumOSDs:       3,
					NumOSDsUp:     2,
					NumOSDsIn:     3,
					NumMons:       1,
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{
		StateProvider:           &mockStateProvider{state: models.StateSnapshot{}},
		UnifiedResourceProvider: cephProvider,
	})

	result, err = exec.executeGetCephStatus(ctx, map[string]interface{}{
		"cluster": "beta",
	})
	require.NoError(t, err)
	assert.Equal(t, "Ceph cluster 'beta' not found.", result.Content[0].Text)

	result, err = exec.executeGetCephStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "alpha")
	assert.Contains(t, result.Content[0].Text, "HEALTH_OK")
}

func TestExecuteGetReplication(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeGetReplication(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No replication jobs found. Replication may not be configured.", result.Content[0].Text)

	now := time.Now()
	state := models.StateSnapshot{
		ReplicationJobs: []models.ReplicationJob{
			{
				ID:                    "rep1",
				GuestID:               101,
				GuestName:             "vm101",
				TargetNode:            "node2",
				Status:                "ok",
				LastSyncTime:          &now,
				LastSyncDurationHuman: "5s",
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetReplication(ctx, map[string]interface{}{
		"vm_id": "999",
	})
	require.NoError(t, err)
	assert.Equal(t, "No replication jobs found for VM 999.", result.Content[0].Text)

	result, err = exec.executeGetReplication(ctx, map[string]interface{}{
		"vm_id": "101",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "rep1")
}

func TestExecuteListSnapshots(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	state := models.StateSnapshot{
		VMs: []models.VM{
			{VMID: 100, Name: "vm100"},
		},
	}

	when := now.UTC()
	size := int64(0)
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
		RecoveryPointsProvider: &stubRecoveryPointsProvider{points: []recovery.RecoveryPoint{
			{
				ID:        "pve-snapshot:snap1",
				Provider:  recovery.ProviderProxmoxPVE,
				Kind:      recovery.KindSnapshot,
				Mode:      recovery.ModeSnapshot,
				Outcome:   recovery.OutcomeSuccess,
				StartedAt: &when,
				SizeBytes: &size,
				Details: map[string]any{
					"snapshotName": "before-upgrade",
					"description":  "",
					"vmState":      true,
					"type":         "vm",
					"instance":     "pve1",
					"node":         "node1",
					"vmid":         100,
				},
			},
		}},
	})
	result, err := exec.executeListSnapshots(ctx, map[string]interface{}{
		"guest_id": "100",
		"instance": "pve1",
	})
	require.NoError(t, err)

	var resp SnapshotsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Snapshots, 1)
	assert.Equal(t, "snap1", resp.Snapshots[0].ID)
	assert.Equal(t, "vm100", resp.Snapshots[0].VMName)
}

func TestExecuteListPBSJobs(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{})

	result, err := exec.executeListPBSJobs(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Backup provider not available.", result.Content[0].Text)

	backupProvider := &mockBackupProvider{}
	backupProvider.On("GetPBSInstances").Return([]models.PBSInstance{
		{
			ID: "pbs1",
			BackupJobs: []models.PBSBackupJob{
				{ID: "job1", Store: "store1", Status: "ok", VMID: "101"},
			},
		},
	})

	exec = NewPulseToolExecutor(ExecutorConfig{BackupProvider: backupProvider})
	result, err = exec.executeListPBSJobs(ctx, map[string]interface{}{
		"job_type": "backup",
	})
	require.NoError(t, err)

	var resp PBSJobsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Jobs, 1)
	assert.Equal(t, "job1", resp.Jobs[0].ID)
}

func TestExecuteGetConnectionHealth(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeGetConnectionHealth(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "No connection health data available.", result.Content[0].Text)

	state := models.StateSnapshot{
		ConnectionHealth: map[string]bool{
			"pve1": true,
			"pve2": false,
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetConnectionHealth(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp ConnectionHealthResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, 1, resp.Connected)
	assert.Equal(t, 1, resp.Disconnected)
}

func TestExecuteGetNetworkStats(t *testing.T) {
	ctx := context.Background()
	speed := int64(1000)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				Hostname: "host1",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "aa", Addresses: []string{"10.0.0.1"}, RXBytes: 1, TXBytes: 2, SpeedMbps: &speed},
				},
			},
		},
		DockerHosts: []models.DockerHost{
			{
				Hostname: "dock1",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", RXBytes: 3, TXBytes: 4},
				},
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetNetworkStats(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp NetworkStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Len(t, resp.Hosts, 2)

	result, err = exec.executeGetNetworkStats(ctx, map[string]interface{}{
		"host": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "No network statistics available for host 'missing'.", result.Content[0].Text)
}

func TestExecuteGetDiskIOStats(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				Hostname: "host1",
				DiskIO: []models.DiskIO{
					{Device: "sda", ReadBytes: 10, WriteBytes: 20},
				},
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetDiskIOStats(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp DiskIOStatsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Len(t, resp.Hosts, 1)
}

func TestExecuteListPhysicalDisks(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	diskProvider := &stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				ID:   "disk1",
				Name: "model",
				Type: unifiedresources.ResourceTypePhysicalDisk,
				PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
					DevPath:     "/dev/sda",
					Model:       "model",
					Serial:      "serial",
					WWN:         "wwn",
					DiskType:    "sata",
					SizeBytes:   1,
					Health:      "PASSED",
					Wearout:     10,
					Temperature: 30,
					RPM:         7200,
					Used:        "used",
				},
				ParentName: "node1",
				Tags:       []string{"sata", "passed", "node1"},
				LastSeen:   now,
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:           &mockStateProvider{state: models.StateSnapshot{}},
		UnifiedResourceProvider: diskProvider,
	})
	result, err := exec.executeListPhysicalDisks(ctx, map[string]interface{}{
		"type": "sata",
	})
	require.NoError(t, err)

	var resp PhysicalDisksResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Disks, 1)
	assert.Equal(t, "disk1", resp.Disks[0].ID)
}

func TestExecuteGetResourceDisks(t *testing.T) {
	ctx := context.Background()
	state := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:       "vm1",
				VMID:     101,
				Name:     "vm1",
				Instance: "pve1",
				Disks: []models.Disk{
					{Device: "vda", Usage: 85},
				},
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct1",
				VMID:     201,
				Name:     "ct1",
				Instance: "pve1",
				Disks: []models.Disk{
					{Device: "vda", Usage: 50},
				},
			},
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err := exec.executeGetResourceDisks(ctx, map[string]interface{}{
		"min_usage": 80.0,
	})
	require.NoError(t, err)

	var resp ResourceDisksResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Resources, 1)
	assert.Equal(t, "vm1", resp.Resources[0].ID)
}

func TestExecuteListBackupTasks(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeListBackupTasks(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	now := time.Now()
	state := models.StateSnapshot{
		VMs: []models.VM{
			{VMID: 101, Name: "vm101"},
		},
		Containers: []models.Container{
			{VMID: 201, Name: "ct201"},
		},
	}
	started := now.UTC()
	exec = NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
		RecoveryPointsProvider: &stubRecoveryPointsProvider{points: []recovery.RecoveryPoint{
			{
				ID:        "pve-task:task1",
				Provider:  recovery.ProviderProxmoxPVE,
				Kind:      recovery.KindBackup,
				Mode:      recovery.ModeLocal,
				Outcome:   recovery.OutcomeSuccess,
				StartedAt: &started,
				Details: map[string]any{
					"status":   "OK",
					"error":    "",
					"instance": "pve1",
					"node":     "node1",
					"vmid":     101,
					"type":     "",
				},
			},
			{
				ID:        "pve-task:task2",
				Provider:  recovery.ProviderProxmoxPVE,
				Kind:      recovery.KindBackup,
				Mode:      recovery.ModeLocal,
				Outcome:   recovery.OutcomeFailed,
				StartedAt: &started,
				Details: map[string]any{
					"status":   "FAIL",
					"error":    "boom",
					"instance": "pve1",
					"node":     "node2",
					"vmid":     201,
					"type":     "",
				},
			},
		}},
	})

	result, err = exec.executeListBackupTasks(ctx, map[string]interface{}{
		"guest_id": "101",
		"status":   "ok",
	})
	require.NoError(t, err)

	var resp BackupTasksListResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Tasks, 1)
	assert.Equal(t, "task1", resp.Tasks[0].ID)
}

func TestExecuteGetSwarmStatus(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeGetSwarmStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "host is required")

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "h1", Hostname: "dock1"},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetSwarmStatus(ctx, map[string]interface{}{
		"host": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "Docker host 'missing' not found.", result.Content[0].Text)

	result, err = exec.executeGetSwarmStatus(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	assert.Equal(t, "Docker host 'dock1' is not part of a Swarm cluster.", result.Content[0].Text)

	state.DockerHosts[0].Swarm = &models.DockerSwarmInfo{
		NodeID:      "node-1",
		NodeRole:    "manager",
		LocalState:  "active",
		ClusterID:   "cluster-1",
		ClusterName: "swarm-1",
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err = exec.executeGetSwarmStatus(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)

	var resp SwarmStatusResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	assert.Equal(t, "dock1", resp.Host)
	assert.Equal(t, "node-1", resp.Status.NodeID)
}

func TestExecuteListDockerServices(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeListDockerServices(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "host is required")

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "h1", Hostname: "dock1"},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeListDockerServices(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	assert.Equal(t, "No Docker services found on host 'dock1'. The host may not be a Swarm manager.", result.Content[0].Text)

	state.DockerHosts[0].Services = []models.DockerService{
		{ID: "svc1", Name: "web", Stack: "stack1", DesiredTasks: 1, RunningTasks: 1},
		{ID: "svc2", Name: "db", Stack: "stack2", DesiredTasks: 1, RunningTasks: 1},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeListDockerServices(ctx, map[string]interface{}{
		"host":  "dock1",
		"stack": "stack1",
	})
	require.NoError(t, err)

	var resp DockerServicesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Services, 1)
	assert.Equal(t, "svc1", resp.Services[0].ID)
}

func TestExecuteListDockerTasks(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeListDockerTasks(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "host is required")

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{ID: "h1", Hostname: "dock1"},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeListDockerTasks(ctx, map[string]interface{}{
		"host": "dock1",
	})
	require.NoError(t, err)
	assert.Equal(t, "No Docker tasks found on host 'dock1'. The host may not be a Swarm manager.", result.Content[0].Text)

	state.DockerHosts[0].Tasks = []models.DockerTask{
		{ID: "task1", ServiceID: "svc1", ServiceName: "web", DesiredState: "running", CurrentState: "running"},
		{ID: "task2", ServiceID: "svc2", ServiceName: "db", DesiredState: "running", CurrentState: "running"},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeListDockerTasks(ctx, map[string]interface{}{
		"host":    "dock1",
		"service": "web",
	})
	require.NoError(t, err)

	var resp DockerTasksResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Tasks, 1)
	assert.Equal(t, "task1", resp.Tasks[0].ID)
}

func TestExecuteGetHostRAIDStatus(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetHostRAIDStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Disk health provider not available.", result.Content[0].Text)

	diskProvider := &mockDiskHealthProvider{}
	diskProvider.On("GetHosts").Return([]models.Host{
		{
			ID:       "host1",
			Hostname: "node1",
			RAID: []models.HostRAIDArray{
				{
					Device:         "/dev/md0",
					Level:          "raid1",
					State:          "clean",
					TotalDevices:   2,
					ActiveDevices:  2,
					WorkingDevices: 2,
					Devices: []models.HostRAIDDevice{
						{Device: "/dev/sda", State: "active", Slot: 0},
					},
				},
			},
		},
	})

	exec = NewPulseToolExecutor(ExecutorConfig{DiskHealthProvider: diskProvider})
	result, err = exec.executeGetHostRAIDStatus(ctx, map[string]interface{}{
		"host": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "No RAID arrays found for host 'missing'.", result.Content[0].Text)

	result, err = exec.executeGetHostRAIDStatus(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp HostRAIDStatusResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Hosts, 1)
	assert.Equal(t, "node1", resp.Hosts[0].Hostname)
}

func TestExecuteGetHostCephDetails(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetHostCephDetails(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Disk health provider not available.", result.Content[0].Text)

	diskProvider := &mockDiskHealthProvider{}
	diskProvider.On("GetHosts").Return([]models.Host{
		{
			ID:       "host1",
			Hostname: "node1",
			Ceph: &models.HostCephCluster{
				FSID: "fsid1",
				Health: models.HostCephHealth{
					Status: "HEALTH_OK",
					Checks: map[string]models.HostCephCheck{
						"CHECK_1": {Severity: "HEALTH_OK"},
					},
					Summary: []models.HostCephHealthSummary{
						{Severity: "HEALTH_OK", Message: "ok"},
					},
				},
				MonMap: models.HostCephMonitorMap{
					NumMons: 1,
					Monitors: []models.HostCephMonitor{
						{Name: "mon1", Rank: 0, Addr: "1.2.3.4", Status: "leader"},
					},
				},
				MgrMap: models.HostCephManagerMap{
					Available: true,
					NumMgrs:   1,
					ActiveMgr: "mgr1",
					Standbys:  0,
				},
				OSDMap: models.HostCephOSDMap{NumOSDs: 2, NumUp: 2, NumIn: 2},
				PGMap:  models.HostCephPGMap{NumPGs: 1, BytesTotal: 10, BytesUsed: 5, BytesAvailable: 5, UsagePercent: 50},
				Pools: []models.HostCephPool{
					{ID: 1, Name: "rbd", PercentUsed: 10},
				},
				CollectedAt: time.Now(),
			},
		},
	})

	exec = NewPulseToolExecutor(ExecutorConfig{DiskHealthProvider: diskProvider})
	result, err = exec.executeGetHostCephDetails(ctx, map[string]interface{}{
		"host": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "No Ceph data found for host 'missing'.", result.Content[0].Text)

	result, err = exec.executeGetHostCephDetails(ctx, map[string]interface{}{})
	require.NoError(t, err)

	var resp HostCephDetailsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Hosts, 1)
	assert.Equal(t, "node1", resp.Hosts[0].Hostname)
}
