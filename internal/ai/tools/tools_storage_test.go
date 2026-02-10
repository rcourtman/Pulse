package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubBackupProvider struct {
	backups models.Backups
	pbs     []models.PBSInstance
}

func (s *stubBackupProvider) GetBackups() models.Backups {
	return s.backups
}

func (s *stubBackupProvider) GetPBSInstances() []models.PBSInstance {
	return s.pbs
}

type stubStorageProvider struct {
	storage []models.Storage
}

func (s *stubStorageProvider) GetStorage() []models.Storage {
	return s.storage
}

type stubDiskHealthProvider struct {
	hosts []models.Host
}

func (s *stubDiskHealthProvider) GetHosts() []models.Host {
	return s.hosts
}

type stubUpdatesProvider struct {
	pending            []ContainerUpdateInfo
	enabled            bool
	triggerCalled      bool
	lastTriggerHost    string
	lastUpdateHost     string
	lastUpdateID       string
	lastUpdateName     string
	triggerStatus      DockerCommandStatus
	triggerErr         error
	updateStatus       DockerCommandStatus
	updateErr          error
	updateCheckEnabled bool
}

func (s *stubUpdatesProvider) GetPendingUpdates(hostID string) []ContainerUpdateInfo {
	s.lastTriggerHost = hostID
	return s.pending
}

func (s *stubUpdatesProvider) TriggerUpdateCheck(hostID string) (DockerCommandStatus, error) {
	s.triggerCalled = true
	s.lastTriggerHost = hostID
	return s.triggerStatus, s.triggerErr
}

func (s *stubUpdatesProvider) UpdateContainer(hostID, containerID, containerName string) (DockerCommandStatus, error) {
	s.lastUpdateHost = hostID
	s.lastUpdateID = containerID
	s.lastUpdateName = containerName
	return s.updateStatus, s.updateErr
}

func (s *stubUpdatesProvider) IsUpdateActionsEnabled() bool {
	return s.enabled
}

func TestExecuteListBackupsAndStorage(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	executor.backupProvider = &stubBackupProvider{
		backups: models.Backups{
			PBS: []models.PBSBackup{
				{
					VMID:       "100",
					BackupType: "vm",
					BackupTime: time.Unix(1000, 0),
					Instance:   "pbs1",
					Datastore:  "ds1",
					Size:       1024 * 1024 * 1024,
					Verified:   true,
					Protected:  true,
				},
			},
			PVE: models.PVEBackups{
				StorageBackups: []models.StorageBackup{
					{
						VMID:    101,
						Time:    time.Unix(1100, 0),
						Size:    2 * 1024 * 1024 * 1024,
						Storage: "local",
					},
				},
				BackupTasks: []models.BackupTask{
					{
						VMID:      101,
						Node:      "node1",
						Status:    "OK",
						StartTime: time.Unix(1200, 0),
					},
				},
			},
		},
		pbs: []models.PBSInstance{
			{
				Name:   "pbs1",
				Host:   "10.0.0.1",
				Status: "online",
				Datastores: []models.PBSDatastore{
					{
						Name:  "ds1",
						Usage: 50.0,
						Free:  1024 * 1024 * 1024,
					},
				},
			},
		},
	}

	result, _ := executor.executeListBackups(context.Background(), map[string]interface{}{})
	var backupsResp BackupsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &backupsResp); err != nil {
		t.Fatalf("decode backups response: %v", err)
	}
	if len(backupsResp.PBS) != 1 || backupsResp.PBS[0].SizeGB != 1 {
		t.Fatalf("unexpected PBS backups: %+v", backupsResp.PBS)
	}
	if len(backupsResp.PVE) != 1 || backupsResp.PVE[0].SizeGB != 2 {
		t.Fatalf("unexpected PVE backups: %+v", backupsResp.PVE)
	}
	if len(backupsResp.PBSServers) != 1 || len(backupsResp.PBSServers[0].Datastores) != 1 {
		t.Fatalf("unexpected PBS servers: %+v", backupsResp.PBSServers)
	}
	if len(backupsResp.RecentTasks) != 1 {
		t.Fatalf("unexpected recent tasks: %+v", backupsResp.RecentTasks)
	}

	executor.storageProvider = &stubStorageProvider{
		storage: []models.Storage{
			{
				ID:      "store1",
				Name:    "store1",
				Type:    "zfs",
				Status:  "active",
				Usage:   25.0,
				Used:    1024 * 1024 * 1024,
				Total:   4 * 1024 * 1024 * 1024,
				Free:    3 * 1024 * 1024 * 1024,
				Content: "images",
				Shared:  false,
				ZFSPool: &models.ZFSPool{
					Name:           "tank",
					State:          "ONLINE",
					ReadErrors:     0,
					WriteErrors:    0,
					ChecksumErrors: 0,
					Scan:           "scrub",
				},
			},
		},
	}
	// Ceph data comes from unified resource provider
	usedBytes := int64(2 * 1024 * 1024 * 1024 * 1024)
	totalBytes := int64(4 * 1024 * 1024 * 1024 * 1024)
	executor.unifiedResourceProvider = &stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				Name: "ceph1",
				Type: unifiedresources.ResourceTypeCeph,
				Ceph: &unifiedresources.CephMeta{
					HealthStatus:  "HEALTH_OK",
					HealthMessage: "ok",
					NumOSDs:       3,
					NumOSDsUp:     3,
					NumOSDsIn:     3,
					NumMons:       1,
					NumMgrs:       1,
				},
				Metrics: &unifiedresources.ResourceMetrics{
					Disk: &unifiedresources.MetricValue{
						Used:  &usedBytes,
						Total: &totalBytes,
					},
				},
			},
		},
	}

	result, _ = executor.executeListStorage(context.Background(), map[string]interface{}{})
	var storageResp StorageResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &storageResp); err != nil {
		t.Fatalf("decode storage response: %v", err)
	}
	if len(storageResp.Pools) != 1 || storageResp.Pools[0].ZFS == nil {
		t.Fatalf("unexpected storage pools: %+v", storageResp.Pools)
	}
	// Verify UsagePercent is passed through directly (not double-multiplied).
	// Storage.Usage is already in 0-100 range from safePercentage().
	if storageResp.Pools[0].UsagePercent != 25.0 {
		t.Errorf("storage pool UsagePercent = %v, want 25.0 (was double-multiplied before fix)", storageResp.Pools[0].UsagePercent)
	}
	if len(storageResp.CephClusters) != 1 || storageResp.CephClusters[0].UsedTB != 2 {
		t.Fatalf("unexpected ceph clusters: %+v", storageResp.CephClusters)
	}
}

func TestStorageUsagePercentNotDoubled(t *testing.T) {
	// Regression test: Usage is already 0-100 from safePercentage().
	// The bug was multiplying by 100 again, turning 25% into 2500%.
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})

	// PBS datastore path
	executor.backupProvider = &stubBackupProvider{
		pbs: []models.PBSInstance{
			{
				Name:   "pbs1",
				Host:   "10.0.0.1",
				Status: "online",
				Datastores: []models.PBSDatastore{
					{Name: "ds1", Usage: 12.3, Free: 1024 * 1024 * 1024},
				},
			},
		},
	}
	result, _ := executor.executeListBackups(context.Background(), map[string]interface{}{})
	var backupsResp BackupsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &backupsResp); err != nil {
		t.Fatalf("decode backups response: %v", err)
	}
	if len(backupsResp.PBSServers) != 1 || len(backupsResp.PBSServers[0].Datastores) != 1 {
		t.Fatalf("unexpected PBS servers: %+v", backupsResp.PBSServers)
	}
	got := backupsResp.PBSServers[0].Datastores[0].UsagePercent
	if got != 12.3 {
		t.Errorf("PBS datastore UsagePercent = %v, want 12.3 (double-multiplied = %v)", got, 12.3*100)
	}

	// Storage pool path
	executor.storageProvider = &stubStorageProvider{
		storage: []models.Storage{
			{
				ID: "s1", Name: "s1", Type: "dir", Status: "active",
				Usage: 45.7,
				Used:  1024 * 1024 * 1024, Total: 4 * 1024 * 1024 * 1024, Free: 3 * 1024 * 1024 * 1024,
			},
		},
	}
	result2, _ := executor.executeListStorage(context.Background(), map[string]interface{}{})
	var storageResp StorageResponse
	if err := json.Unmarshal([]byte(result2.Content[0].Text), &storageResp); err != nil {
		t.Fatalf("decode storage response: %v", err)
	}
	if len(storageResp.Pools) != 1 {
		t.Fatalf("unexpected pools: %+v", storageResp.Pools)
	}
	got2 := storageResp.Pools[0].UsagePercent
	if got2 != 45.7 {
		t.Errorf("storage pool UsagePercent = %v, want 45.7 (double-multiplied = %v)", got2, 45.7*100)
	}
}

func TestDockerUpdateTools(t *testing.T) {
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:          "host1",
				Hostname:    "docker1",
				DisplayName: "Docker One",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "/nginx"},
				},
			},
		},
	}

	stateProv := &mockStateProvider{}
	stateProv.On("GetState").Return(state)
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: stateProv,
		ControlLevel:  ControlLevelControlled,
	})
	updates := &stubUpdatesProvider{
		pending: []ContainerUpdateInfo{
			{HostID: "host1", ContainerID: "c1", ContainerName: "nginx", UpdateAvailable: true},
		},
		enabled: true,
		triggerStatus: DockerCommandStatus{
			ID:     "cmd1",
			Type:   "check",
			Status: "queued",
		},
		updateStatus: DockerCommandStatus{
			ID:     "cmd2",
			Type:   "update",
			Status: "queued",
		},
	}
	executor.updatesProvider = updates

	result, _ := executor.executeListDockerUpdates(context.Background(), map[string]interface{}{"host": "Docker One"})
	var listResp DockerUpdatesResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &listResp); err != nil {
		t.Fatalf("decode docker updates: %v", err)
	}
	if listResp.HostID != "host1" || listResp.Total != 1 {
		t.Fatalf("unexpected docker updates response: %+v", listResp)
	}

	result, _ = executor.executeCheckDockerUpdates(context.Background(), map[string]interface{}{"host": "Docker One"})
	var checkResp DockerCheckUpdatesResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &checkResp); err != nil {
		t.Fatalf("decode docker check response: %v", err)
	}
	if checkResp.CommandID != "cmd1" || checkResp.HostID != "host1" {
		t.Fatalf("unexpected check response: %+v", checkResp)
	}

	executor.controlLevel = ControlLevelAutonomous
	result, _ = executor.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "Docker One",
		"container": "c1",
	})
	var updateResp DockerUpdateContainerResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &updateResp); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updates.lastUpdateName != "nginx" || updateResp.CommandID != "cmd2" {
		t.Fatalf("unexpected update response: %+v", updateResp)
	}

	updates.enabled = false
	result, _ = executor.executeUpdateDockerContainer(context.Background(), map[string]interface{}{
		"host":      "Docker One",
		"container": "c1",
	})
	if !strings.Contains(result.Content[0].Text, "updates are disabled") {
		t.Fatalf("unexpected disabled response: %s", result.Content[0].Text)
	}
}
