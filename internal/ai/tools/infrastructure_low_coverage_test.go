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
			{
				ID:       "qemu/pve1/node1/100",
				VMID:     100,
				Name:     "vm100",
				Node:     "node1",
				Instance: "pve1",
				Status:   "running",
			},
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

func TestBackupListResponsesUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		keys []string
	}{
		{name: "snapshots", raw: EmptySnapshotsResponse(), keys: []string{"snapshots"}},
		{name: "pbs_jobs", raw: EmptyPBSJobsResponse(), keys: []string{"jobs"}},
		{name: "backup_tasks", raw: EmptyBackupTasksListResponse(), keys: []string{"tasks"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(payload, &decoded))

			for _, key := range tc.keys {
				values, ok := decoded[key].([]any)
				if !ok || len(values) != 0 {
					t.Fatalf("expected %s.%s to be an empty array, got %T (%v)", tc.name, key, decoded[key], decoded[key])
				}
			}
		})
	}
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

func TestConnectionHealthResponseUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyConnectionHealthResponse())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))

	connections, ok := decoded["connections"].([]any)
	if !ok || len(connections) != 0 {
		t.Fatalf("expected connections to be an empty array, got %T (%v)", decoded["connections"], decoded["connections"])
	}
}

func TestExecuteGetNetworkStats(t *testing.T) {
	ctx := context.Background()
	speed := int64(1000)
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host1",
				Hostname: "host1",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "aa", Addresses: []string{"10.0.0.1"}, RXBytes: 1, TXBytes: 2, SpeedMbps: &speed},
				},
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "dock1",
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
				ID:       "host1",
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

func TestInfrastructureDiagnosticResponsesUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		keys []string
	}{
		{name: "network_stats", raw: EmptyNetworkStatsResponse(), keys: []string{"hosts"}},
		{name: "diskio_stats", raw: EmptyDiskIOStatsResponse(), keys: []string{"hosts"}},
		{name: "physical_disks", raw: EmptyPhysicalDisksResponse(), keys: []string{"disks"}},
		{name: "docker_services", raw: EmptyDockerServicesResponse(), keys: []string{"services"}},
		{name: "docker_tasks", raw: EmptyDockerTasksResponse(), keys: []string{"tasks"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(payload, &decoded))

			for _, key := range tc.keys {
				values, ok := decoded[key].([]any)
				if !ok || len(values) != 0 {
					t.Fatalf("expected %s.%s to be an empty array, got %T (%v)", tc.name, key, decoded[key], decoded[key])
				}
			}
		})
	}
}

func TestSharedToolSummaryOwnersUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		keys []string
	}{
		{name: "capabilities", raw: EmptyCapabilitiesResponse(), keys: []string{"protected_guests", "agents"}},
		{name: "agent_scope", raw: EmptyAgentScopeResponse(), keys: []string{"settings", "observed_modules"}},
		{name: "cluster_status", raw: EmptyClusterStatusResponse(), keys: []string{"clusters"}},
		{name: "recent_tasks", raw: EmptyRecentTasksResponse(), keys: []string{"tasks"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(payload, &decoded))

			for _, key := range tc.keys {
				switch value := decoded[key].(type) {
				case []any:
					require.Len(t, value, 0, "expected %s.%s to be an empty array", tc.name, key)
				case map[string]any:
					require.Len(t, value, 0, "expected %s.%s to be an empty object", tc.name, key)
				default:
					t.Fatalf("expected %s.%s to be canonical empty collection, got %T (%v)", tc.name, key, decoded[key], decoded[key])
				}
			}
		})
	}

	payload, err := json.Marshal(BackupsResponse{
		PBSServers: []PBSServerSummary{{Name: "pbs-1"}},
	}.NormalizeCollections())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	servers := decoded["pbs_servers"].([]any)
	server := servers[0].(map[string]any)
	datastores, ok := server["datastores"].([]any)
	if !ok || len(datastores) != 0 {
		t.Fatalf("expected pbs_servers[0].datastores to be an empty array, got %T (%v)", server["datastores"], server["datastores"])
	}

	payload, err = json.Marshal(StorageResponse{
		Pools: []StoragePoolSummary{{ID: "pool-1", Name: "local"}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	pools := decoded["pools"].([]any)
	pool := pools[0].(map[string]any)
	nodes, ok := pool["nodes"].([]any)
	if !ok || len(nodes) != 0 {
		t.Fatalf("expected pools[0].nodes to be an empty array, got %T (%v)", pool["nodes"], pool["nodes"])
	}

	payload, err = json.Marshal(PMGStatusResponse{
		Instances: []PMGInstanceSummary{{ID: "pmg-1", Name: "pmg-1"}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	instances := decoded["instances"].([]any)
	instance := instances[0].(map[string]any)
	pmgNodes, ok := instance["nodes"].([]any)
	if !ok || len(pmgNodes) != 0 {
		t.Fatalf("expected instances[0].nodes to be an empty array, got %T (%v)", instance["nodes"], instance["nodes"])
	}

	payload, err = json.Marshal(NetworkStatsResponse{
		Hosts: []HostNetworkStatsSummary{{Hostname: "host1", Interfaces: []NetworkInterfaceSummary{{Name: "eth0"}}}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	hosts := decoded["hosts"].([]any)
	host := hosts[0].(map[string]any)
	interfaces := host["interfaces"].([]any)
	iface := interfaces[0].(map[string]any)
	addrs, ok := iface["addresses"].([]any)
	if !ok || len(addrs) != 0 {
		t.Fatalf("expected interfaces[0].addresses to be an empty array, got %T (%v)", iface["addresses"], iface["addresses"])
	}

	payload, err = json.Marshal(DiskIOStatsResponse{
		Hosts: []HostDiskIOStatsSummary{{Hostname: "host1"}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	hosts = decoded["hosts"].([]any)
	host = hosts[0].(map[string]any)
	devices, ok := host["devices"].([]any)
	if !ok || len(devices) != 0 {
		t.Fatalf("expected hosts[0].devices to be an empty array, got %T (%v)", host["devices"], host["devices"])
	}

	payload, err = json.Marshal(HostRAIDStatusResponse{
		Hosts: []HostRAIDSummary{{Hostname: "host1", Arrays: []HostRAIDArraySummary{{Device: "md0"}}}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	hosts = decoded["hosts"].([]any)
	host = hosts[0].(map[string]any)
	arrays := host["arrays"].([]any)
	array := arrays[0].(map[string]any)
	raidDevices, ok := array["devices"].([]any)
	if !ok || len(raidDevices) != 0 {
		t.Fatalf("expected arrays[0].devices to be an empty array, got %T (%v)", array["devices"], array["devices"])
	}

	payload, err = json.Marshal(HostCephDetailsResponse{
		Hosts: []HostCephSummary{{Hostname: "host1"}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	hosts = decoded["hosts"].([]any)
	host = hosts[0].(map[string]any)
	cephPools, ok := host["pools"].([]any)
	if !ok || len(cephPools) != 0 {
		t.Fatalf("expected hosts[0].pools to be an empty array, got %T (%v)", host["pools"], host["pools"])
	}
	health := host["health"].(map[string]any)
	messages, ok := health["messages"].([]any)
	if !ok || len(messages) != 0 {
		t.Fatalf("expected hosts[0].health.messages to be an empty array, got %T (%v)", health["messages"], health["messages"])
	}

	payload, err = json.Marshal(ResourceDisksResponse{
		Resources: []ResourceDisksSummary{{ID: "vm-1", Disks: nil}},
	}.NormalizeCollections())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &decoded))
	resources := decoded["resources"].([]any)
	resource := resources[0].(map[string]any)
	disks, ok := resource["disks"].([]any)
	if !ok || len(disks) != 0 {
		t.Fatalf("expected resources[0].disks to be an empty array, got %T (%v)", resource["disks"], resource["disks"])
	}
}

func TestDiskHealthResponseUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyDiskHealthResponse())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))

	values, ok := decoded["hosts"].([]any)
	if !ok || len(values) != 0 {
		t.Fatalf("expected hosts to be an empty array, got %T (%v)", decoded["hosts"], decoded["hosts"])
	}

	payload, err = json.Marshal(DiskHealthResponse{
		Hosts: []HostDiskHealth{{
			Hostname: "host1",
		}},
	}.NormalizeCollections())
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(payload, &decoded))
	hosts, ok := decoded["hosts"].([]any)
	require.True(t, ok)
	require.Len(t, hosts, 1)

	host, ok := hosts[0].(map[string]any)
	require.True(t, ok)

	smart, ok := host["smart"].([]any)
	if !ok || len(smart) != 0 {
		t.Fatalf("expected smart to be an empty array, got %T (%v)", host["smart"], host["smart"])
	}
	raid, ok := host["raid"].([]any)
	if !ok || len(raid) != 0 {
		t.Fatalf("expected raid to be an empty array, got %T (%v)", host["raid"], host["raid"])
	}
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

func TestExecuteGetResourceDisks_SystemContainerTypeFilter(t *testing.T) {
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
		"type": "system-container",
	})
	require.NoError(t, err)

	var resp ResourceDisksResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Resources, 1)
	assert.Equal(t, "system-container", resp.Resources[0].Type)
	assert.Equal(t, "ct1", resp.Resources[0].ID)
}

func TestExecuteGetResourceDisks_RejectsLegacyLXCTypeFilter(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})

	result, err := exec.executeGetResourceDisks(ctx, map[string]interface{}{
		"type": "lxc",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "unsupported type")
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
	diskProvider.On("GetHosts").Return([]*unifiedresources.HostView{
		newHostView(
			"host-resource-1",
			"node1",
			"host1",
			"node1",
			nil,
			[]unifiedresources.HostRAIDMeta{
				{
					Device:         "/dev/md0",
					Level:          "raid1",
					State:          "clean",
					TotalDevices:   2,
					ActiveDevices:  2,
					WorkingDevices: 2,
					Devices: []unifiedresources.HostRAIDDeviceMeta{
						{Device: "/dev/sda", State: "active", Slot: 0},
					},
				},
			},
			nil,
		),
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
	diskProvider.On("GetHosts").Return([]*unifiedresources.HostView{
		newHostView(
			"host-resource-1",
			"node1",
			"host1",
			"node1",
			nil,
			nil,
			&unifiedresources.HostCephMeta{
				FSID: "fsid1",
				Health: unifiedresources.HostCephHealthMeta{
					Status: "HEALTH_OK",
					Checks: map[string]unifiedresources.HostCephCheckMeta{
						"CHECK_1": {Severity: "HEALTH_OK"},
					},
					Summary: []unifiedresources.HostCephHealthSummaryMeta{
						{Severity: "HEALTH_OK", Message: "ok"},
					},
				},
				MonMap: unifiedresources.HostCephMonitorMapMeta{
					NumMons: 1,
					Monitors: []unifiedresources.HostCephMonitorMeta{
						{Name: "mon1", Rank: 0, Addr: "1.2.3.4", Status: "leader"},
					},
				},
				MgrMap: unifiedresources.HostCephManagerMapMeta{
					Available: true,
					NumMgrs:   1,
					ActiveMgr: "mgr1",
					Standbys:  0,
				},
				OSDMap: unifiedresources.HostCephOSDMapMeta{NumOSDs: 2, NumUp: 2, NumIn: 2},
				PGMap:  unifiedresources.HostCephPGMapMeta{NumPGs: 1, BytesTotal: 10, BytesUsed: 5, BytesAvailable: 5, UsagePercent: 50},
				Pools: []unifiedresources.HostCephPoolMeta{
					{ID: 1, Name: "rbd", PercentUsed: 10},
				},
				CollectedAt: time.Now(),
			},
		),
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
