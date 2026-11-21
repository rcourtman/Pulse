package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPulseStorageBackupTasksDoesNotInferBlankStatus(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	start, finish := now.Add(-18*time.Minute), now.Add(-2*time.Minute)
	rows := []models.BackupTask{
		{ID: "expired", VMID: 100, ObservedAt: now.Add(-8 * time.Minute)},
		{ID: "unclocked", VMID: 100},
		{ID: "fresh", VMID: 100, ObservedAt: now.Add(-time.Minute)},
		{ID: "ended-blank", VMID: 100, EndTime: finish, ObservedAt: now},
		{ID: "parent", ObservedAt: now},
		{ID: "explicit-running", VMID: 101, Status: "running", ObservedAt: now},
		{ID: "success", VMID: 101, Status: "OK", EndTime: finish},
		{ID: "warning", VMID: 101, Status: "WARNINGS: 1", EndTime: finish},
		{ID: "failure", VMID: 102, Status: "ERROR: archive unavailable", EndTime: finish, Error: "fixture write failed"},
	}
	for i := range rows {
		rows[i].Instance, rows[i].Node, rows[i].Type, rows[i].StartTime = "lab", "pve1", "vzdump", start
	}

	points, err := proxmoxmapper.FromPVEBackupTasksWithEvidence(rows, nil, now)
	require.NoError(t, err)
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	require.NoError(t, store.RecordBackupHistory(ctx, points, []model.BackupHistoryCollection{{
		Instance: "lab", Provider: recovery.ProviderProxmoxPVE, ObservedAt: now, Complete: true, MaxGap: 5 * time.Minute,
	}}))

	for _, tt := range []struct {
		name string
		cfg  ExecutorConfig
	}{
		{name: "retained", cfg: ExecutorConfig{RecoveryPointsProvider: NewRecoveryPointsToolAdapter(manager, "default")}},
		{name: "legacy", cfg: ExecutorConfig{BackupProvider: &stubBackupProvider{backups: models.Backups{PVE: models.PVEBackups{BackupTasks: rows}}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.StateProvider = &mockStateProvider{}
			exec := NewPulseToolExecutor(tt.cfg)
			read := func(status string) BackupTasksListResponse {
				t.Helper()
				result, err := exec.ExecuteTool(ctx, "pulse_storage", map[string]interface{}{"type": "backup_tasks", "status": status})
				require.NoError(t, err)
				require.False(t, result.IsError, result.Content)
				var response BackupTasksListResponse
				require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
				return response
			}
			unfiltered := read("")
			statusByID := make(map[string]string, len(rows))
			for _, task := range unfiltered.Tasks {
				statusByID[task.ID] = task.Status
			}
			assert.Equal(t, len(rows), unfiltered.Total)
			assert.Equal(t, map[string]string{
				"expired": "UNKNOWN", "unclocked": "UNKNOWN", "fresh": "UNKNOWN", "ended-blank": "UNKNOWN", "parent": "UNKNOWN",
				"explicit-running": "running", "success": "OK", "warning": "WARNINGS: 1", "failure": "ERROR: archive unavailable",
			}, statusByID)
			running := read("running")
			assert.Equal(t, 1, running.Filtered, "blank-status rows must never match the running filter")
			require.Len(t, running.Tasks, 1)
			assert.Equal(t, "explicit-running", running.Tasks[0].ID)
			failed := read("error")
			require.Len(t, failed.Tasks, 1)
			assert.Equal(t, "failure", failed.Tasks[0].ID)
			assert.Equal(t, finish, failed.Tasks[0].EndTime)
			assert.Equal(t, "fixture write failed", failed.Tasks[0].Error)
			ok := read("success")
			require.Len(t, ok.Tasks, 1)
			assert.Equal(t, "success", ok.Tasks[0].ID)
		})
	}
}
