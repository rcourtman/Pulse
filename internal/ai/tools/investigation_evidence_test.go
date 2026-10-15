package tools

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestInvestigationBackupInventoryUsesCanonicalArtifacts(t *testing.T) {
	ctx := context.Background()
	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	at := time.Unix(1800, 0).UTC()
	resourceID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "west", 101)
	otherResourceID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "east", 101)
	archives := []models.StorageBackup{
		{ID: "west-archive", VMID: 101, Instance: "west", Node: "pve1", Type: "qemu", Storage: "local", Time: at, Size: 1024},
		{ID: "east-archive", VMID: 101, Instance: "east", Node: "pve1", Type: "qemu", Storage: "local", Time: at, Size: 2048},
	}
	points := proxmoxmapper.FromPVEStorageBackups(archives, map[string]proxmoxmapper.GuestInfo{
		"west|pve1|101": {ResourceID: resourceID, ResourceType: unifiedresources.ResourceTypeVM},
		"east|pve1|101": {ResourceID: otherResourceID, ResourceType: unifiedresources.ResourceTypeVM},
	})
	points = append(points, proxmoxmapper.FromPBSBackups([]models.PBSBackup{{
		ID: "remote-archive", VMID: "101", BackupType: "vm", Instance: "pbs-west", Datastore: "daily", BackupTime: at, Size: 4096,
	}}, map[string][]proxmoxmapper.GuestCandidate{
		"vm:101": {{ResourceID: resourceID, ResourceType: unifiedresources.ResourceTypeVM, InstanceName: "west", NodeName: "pve1", VMID: 101, SourceID: "west:pve1:101"}},
	})...)
	// Inventory is provider-agnostic and excludes execution/snapshot evidence.
	points = append(points,
		recovery.RecoveryPoint{ID: "agent-backup", Provider: recovery.ProviderAgent, Kind: recovery.KindBackup, Mode: recovery.ModeRemote, Outcome: recovery.OutcomeUnknown, SubjectResourceID: resourceID},
		recovery.RecoveryPoint{ID: "pve-task:failure", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, SubjectResourceID: resourceID},
		recovery.RecoveryPoint{ID: "pve-snapshot:old", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindSnapshot, SubjectResourceID: resourceID},
	)
	require.NoError(t, store.UpsertPoints(ctx, points))
	foreign, err := manager.StoreForOrg("foreign")
	require.NoError(t, err)
	t.Cleanup(func() { _ = foreign.Close() })
	foreignPoint := points[0]
	foreignPoint.ID = "foreign-archive"
	require.NoError(t, foreign.UpsertPoints(ctx, []recovery.RecoveryPoint{foreignPoint}))
	executor := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider: NewRecoveryPointsToolAdapter(manager, "default"),
		// A legacy cache must not shadow, supplement or filter canonical records.
		BackupProvider: &stubBackupProvider{backups: models.Backups{PVE: models.PVEBackups{StorageBackups: archives}}},
	})
	result, err := executor.executeListBackups(ctx, map[string]interface{}{"resource_id": resourceID})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	var response struct {
		Points     []recovery.RecoveryPoint `json:"points"`
		Pagination PaginationInfo           `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
	require.Len(t, response.Points, 3)
	require.Equal(t, 3, response.Pagination.Total)
	byID := map[string]recovery.RecoveryPoint{}
	for _, point := range response.Points {
		require.Equal(t, resourceID, point.SubjectResourceID)
		require.Equal(t, recovery.KindBackup, point.Kind)
		byID[point.ID] = point
	}
	require.Contains(t, byID, "pve-backup:west-archive")
	require.Nil(t, byID["pve-backup:west-archive"].Verified, "uncollected verification remains unknown")
	require.Contains(t, byID, "pbs-backup:remote-archive")
	remote := byID["pbs-backup:remote-archive"]
	require.NotNil(t, remote.Verified)
	require.False(t, *remote.Verified)
	require.NotNil(t, remote.RepositoryRef)
	require.Equal(t, "daily", remote.RepositoryRef.Name)
	require.Equal(t, int64(4096), *remote.SizeBytes)
	require.Equal(t, at, *remote.CompletedAt)
	require.Contains(t, byID, "agent-backup")
}

type inventoryPointsProviderFunc func(context.Context, recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error)

func (f inventoryPointsProviderFunc) ListPoints(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	return f(ctx, opts)
}

func TestInvestigationBackupInventoryPagination(t *testing.T) {
	all := make([]recovery.RecoveryPoint, 5010)
	for i := range all {
		all[i] = recovery.RecoveryPoint{ID: fmt.Sprintf("archive-%04d", i), Kind: recovery.KindBackup, SubjectResourceID: "vm-canonical"}
	}
	for _, tc := range []struct{ offset, limit, wantOffset, wantLimit, wantCount int }{
		{499, 3, 499, 3, 3}, {5000, 2, 5000, 2, 2}, {5010, 3, 5010, 3, 0},
		{-1, 0, 0, recovery.DefaultListPageLimit, recovery.DefaultListPageLimit},
		{0, 900, 0, recovery.MaxListPageLimit, recovery.MaxListPageLimit},
	} {
		t.Run(fmt.Sprintf("offset-%d-limit-%d", tc.offset, tc.limit), func(t *testing.T) {
			var queries []recovery.ListPointsOptions
			executor := NewPulseToolExecutor(ExecutorConfig{RecoveryPointsProvider: inventoryPointsProviderFunc(func(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
				queries = append(queries, opts)
				require.Equal(t, recovery.KindBackup, opts.Kind)
				require.Equal(t, "vm-canonical", opts.SubjectResourceID)
				return recovery.PaginatePoints(all, opts.Page, opts.Limit), len(all), nil
			})})
			result, err := executor.executeListBackups(context.Background(), map[string]interface{}{"resource_id": "vm-canonical", "offset": tc.offset, "limit": tc.limit})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Content)
			var response struct {
				Points     []recovery.RecoveryPoint `json:"points"`
				Pagination PaginationInfo           `json:"pagination"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
			require.Len(t, response.Points, tc.wantCount)
			require.Equal(t, PaginationInfo{Total: len(all), Offset: tc.wantOffset, Limit: tc.wantLimit}, response.Pagination)
			if tc.wantCount > 0 {
				require.Equal(t, all[tc.wantOffset:tc.wantOffset+tc.wantCount], response.Points)
			}
			require.LessOrEqual(t, len(queries), 2, "offset pagination must not scan earlier pages")
		})
	}
}

func TestInvestigationBackupInventoryDoesNotHideReadFailures(t *testing.T) {
	for _, provider := range []RecoveryPointsProvider{nil, inventoryPointsProviderFunc(func(context.Context, recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
		return nil, 0, context.Canceled
	})} {
		executor := NewPulseToolExecutor(ExecutorConfig{RecoveryPointsProvider: provider, BackupProvider: &stubBackupProvider{}})
		result, err := executor.executeListBackups(context.Background(), nil)
		require.NoError(t, err)
		require.True(t, result.IsError, "unavailable inventory must not become an empty successful result")
	}
}

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
