package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
)

func TestInvestigationReadsCanonicalBackupFailureWithoutAgent(t *testing.T) {
	ctx := context.Background()
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	at := time.Now().Add(-time.Hour).UTC()
	// Use the real mapper and persisted query, so a stale KindBackup filter
	// cannot pass by asking an in-memory stub to ignore query constraints.
	points := proxmoxmapper.FromPVEBackupTasks([]models.BackupTask{{
		ID: "failed-attempt", VMID: 101, Node: "pve1", Instance: "west", Type: "qemu",
		Status: "failed", StartTime: at, EndTime: at.Add(time.Minute),
		Error: "backup write failed: repository connection timed out",
	}}, nil)
	require.Len(t, points, 1)
	require.Equal(t, recovery.KindOther, points[0].Kind)
	points[0].SubjectResourceID = "vm-canonical"
	other := points[0]
	other.ID = "pve-task:another-guest"
	other.SubjectResourceID = "vm-other-cluster"
	artifact := points[0]
	artifact.ID = "pve-backup:artifact"
	artifact.Kind = recovery.KindBackup
	artifact.Outcome = recovery.OutcomeSuccess
	require.NoError(t, store.UpsertPoints(ctx, []recovery.RecoveryPoint{points[0], other, artifact}))
	foreign, err := manager.StoreForOrg("another-tenant")
	require.NoError(t, err)
	t.Cleanup(func() { _ = foreign.Close() })
	foreignPoint := points[0]
	foreignPoint.ID = "pve-task:foreign"
	require.NoError(t, foreign.UpsertPoints(ctx, []recovery.RecoveryPoint{foreignPoint}))
	executor := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider: NewRecoveryPointsToolAdapter(manager, "default"),
	})
	for _, filter := range []string{"", "error", "failed"} {
		result, err := executor.executeListBackupTasks(ctx, map[string]interface{}{
			"resource_id": "vm-canonical", "status": filter,
		})
		require.NoError(t, err)
		require.False(t, result.IsError, result.Content)
		var response BackupTasksListResponse
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
		require.Len(t, response.Tasks, 1)
		require.Equal(t, "failed-attempt", response.Tasks[0].ID)
		require.Equal(t, points[0].Details["error"], response.Tasks[0].Error)
		require.Equal(t, at.Unix(), response.Tasks[0].StartTime.Unix())
	}
}

func TestStoragePerformanceReadsRetainedHistoryAcrossRestart(t *testing.T) {
	cfg := metrics.DefaultConfig(filepath.Join(t.TempDir(), "metrics"))
	cfg.RetentionRaw = 8 * 24 * time.Hour
	store, err := metrics.NewStore(cfg)
	require.NoError(t, err)
	at := time.Now().Add(-6 * time.Hour).Truncate(time.Second)
	store.Write("storage", "native-pool", "usage", 0, at)
	store.Write("storage", "native-pool", "usage", 83, at.Add(time.Hour))
	require.NoError(t, store.Close())
	store, err = metrics.NewStore(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	adapter := NewMetricsHistoryToolAdapter(&fakeMetricsSource{}, &fakeReadState{}, store)
	resource := unifiedresources.Resource{ID: "storage-canonical", Type: unifiedresources.ResourceTypeStorage, Name: "pool", MetricsTarget: &unifiedresources.MetricsTarget{ResourceType: "storage", ResourceID: "native-pool"}}
	executor := NewPulseToolExecutor(ExecutorConfig{MetricsHistory: adapter, UnifiedResourceProvider: &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{resource}}})
	result, err := executor.executeGetMetrics(context.Background(), map[string]interface{}{"resource_id": resource.ID, "resource_type": "storage", "period": "7d"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	var response struct {
		ResourceID string                   `json:"resource_id"`
		Metric     string                   `json:"metric"`
		Source     string                   `json:"source"`
		Points     []map[string]interface{} `json:"points"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
	require.Equal(t, resource.ID, response.ResourceID)
	require.Equal(t, "usage", response.Metric)
	require.Equal(t, "store", response.Source)
	require.Len(t, response.Points, 2)
	require.Equal(t, 0.0, response.Points[0]["value"])
	require.Equal(t, 83.0, response.Points[1]["value"])
	require.NotContains(t, response.Points[0], "cpu")
	require.NotContains(t, response.Points[0], "memory")
	require.NoError(t, store.Close())
	result, err = executor.executeGetMetrics(context.Background(), map[string]interface{}{"resource_id": resource.ID, "period": "7d"})
	require.NoError(t, err)
	require.True(t, result.IsError, "store error must not become empty successful history")
}
