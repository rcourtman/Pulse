package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestInvestigationRecoverySourceFollowsRuntimeMode(t *testing.T) {
	testutil.SetMockMode(t, false)
	manager := newRecoveryReadManager(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{{
		ID: "persisted-marker", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeRemote,
		Outcome: recovery.OutcomeFailed, SubjectResourceID: "vm-persisted",
	}}))
	adapter := tools.NewRecoveryPointsToolAdapter(manager, "default")
	for _, demo := range []bool{false, true, false} {
		require.NoError(t, mock.SetEnabled(demo))
		points, _, err := adapter.ListPoints(context.Background(), recovery.ListPointsOptions{SubjectResourceID: "vm-persisted"})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		NewRecoveryHandlers(manager).HandleListPoints(recorder, httptest.NewRequest(http.MethodGet,
			"/api/recovery/points?itemResourceId=vm-persisted", nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		if demo {
			require.Empty(t, points, "demo reads must not leak persisted tenant evidence")
			require.NotContains(t, recorder.Body.String(), "persisted-marker")
			graphPoints, _, err := adapter.ListPoints(context.Background(), recovery.ListPointsOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, graphPoints, "a manager composed before demo was enabled must see its current graph")
		} else {
			require.Len(t, points, 1)
			require.Equal(t, "persisted-marker", points[0].ID)
			require.Contains(t, recorder.Body.String(), "persisted-marker")
		}
	}
}

func TestInvestigationBackupTasksUseCommunityRecoverySource(t *testing.T) {
	testutil.SetMockMode(t, true)
	// Exercise the same composition used by Router, without a PVE client or
	// a seeded second database. The previous HTTP-only fixture branch failed
	// here even though isolated tests of persisted tools passed.
	manager := newRecoveryReadManager(nil)
	var selected recovery.RecoveryPoint
	for _, point := range mock.CurrentFixtureGraph().RecoveryPoints() {
		if point.Provider == recovery.ProviderProxmoxPVE && strings.HasPrefix(point.ID, "pve-task:") && point.Outcome == recovery.OutcomeFailed && point.Details["error"] != "" {
			selected = point
			break
		}
	}
	require.NotEmpty(t, selected.ID, "fixture must contain collected task failure text")
	recorder := httptest.NewRecorder()
	NewRecoveryHandlers(manager).HandleListPoints(recorder, httptest.NewRequest(http.MethodGet,
		"/api/recovery/points?platform=proxmox-pve&itemResourceId="+url.QueryEscape(selected.SubjectResourceID), nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), selected.Details["error"])
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{RecoveryPointsProvider: tools.NewRecoveryPointsToolAdapter(manager, "default")})
	result, err := executor.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type": "backup_tasks", "resource_id": selected.SubjectResourceID, "status": "error",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	var response struct {
		Tasks []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
	require.NotEmpty(t, response.Tasks)
	for _, task := range response.Tasks {
		require.Equal(t, strings.TrimPrefix(selected.ID, "pve-task:"), task.ID)
		require.Equal(t, selected.Details["error"], task.Error)
	}
}

func TestInvestigationBackupInventoryMatchesCommunityPoints(t *testing.T) {
	testutil.SetMockMode(t, true)
	manager := newRecoveryReadManager(config.NewMultiTenantPersistence(t.TempDir()))
	var selected recovery.RecoveryPoint
	for _, point := range mock.CurrentFixtureGraph().RecoveryPoints() {
		if point.Kind == recovery.KindBackup && point.SubjectResourceID != "" {
			selected = point
			break
		}
	}
	require.NotEmpty(t, selected.ID)
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{selected}))
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{RecoveryPointsProvider: tools.NewRecoveryPointsToolAdapter(manager, "default")})
	for _, demo := range []bool{true, false, true} {
		require.NoError(t, mock.SetEnabled(demo))
		for _, resourceID := range []string{selected.SubjectResourceID, "vm-missing"} {
			recorder := httptest.NewRecorder()
			NewRecoveryHandlers(manager).HandleListPoints(recorder, httptest.NewRequest(http.MethodGet,
				"/api/recovery/points?kind=backup&limit=500&itemResourceId="+url.QueryEscape(resourceID), nil))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			var apiResponse struct {
				Data []recovery.RecoveryPoint `json:"data"`
				Meta struct {
					Total int `json:"total"`
				} `json:"meta"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &apiResponse))
			result, err := executor.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
				"type": "backups", "resource_id": resourceID, "limit": 500,
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Content)
			var toolResponse struct {
				Points     []recovery.RecoveryPoint `json:"points"`
				Pagination tools.PaginationInfo     `json:"pagination"`
			}
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &toolResponse))
			require.Equal(t, apiResponse.Meta.Total, toolResponse.Pagination.Total)
			require.Equal(t, apiResponse.Data, toolResponse.Points, "inventory identity, repository, verification and provenance must match HTTP")
			if resourceID == selected.SubjectResourceID {
				require.NotEmpty(t, toolResponse.Points)
			} else {
				require.NotNil(t, toolResponse.Points)
				require.Empty(t, toolResponse.Points)
			}
		}
	}
}

type investigationHistoryResources struct{ resource unifiedresources.Resource }

func (p investigationHistoryResources) GetByType(kind unifiedresources.ResourceType) []unifiedresources.Resource {
	if p.resource.Type == kind {
		return []unifiedresources.Resource{p.resource}
	}
	return nil
}

func TestInvestigationStorageHistoryUsesCommunityMonitoringSource(t *testing.T) {
	testutil.SetMockMode(t, false)
	monitor, state, history := newTestMonitor(t)
	syncTestResourceStore(t, monitor, state)
	at := time.Now().UTC().Truncate(time.Second)
	history.AddStorageMetric("native-pool", "usage", 0, at.Add(-30*time.Minute))
	history.AddStorageMetric("native-pool", "usage", 83, at.Add(-10*time.Minute))
	router := &Router{monitor: monitor}
	recorder := httptest.NewRecorder()
	router.handleMetricsHistory(recorder, httptest.NewRequest(http.MethodGet,
		"/api/metrics-store/history?resourceType=storage&resourceId=native-pool&metric=usage&range=7d", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var apiHistory struct {
		Source string `json:"source"`
		Points []struct {
			Value float64 `json:"value"`
		} `json:"points"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &apiHistory))
	adapter := tools.NewMetricsHistoryToolAdapter(&metricsSourceWrapper{history: history, monitor: monitor}, monitor.GetUnifiedReadState())
	require.NotNil(t, adapter)
	resource := unifiedresources.Resource{ID: "storage-canonical", Name: "pool", Type: unifiedresources.ResourceTypeStorage,
		MetricsTarget: &unifiedresources.MetricsTarget{ResourceType: "storage", ResourceID: "native-pool"}}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{MetricsHistory: adapter,
		UnifiedResourceProvider: investigationHistoryResources{resource: resource}})
	result, err := executor.ExecuteTool(context.Background(), "pulse_metrics", map[string]interface{}{
		"type": "performance", "resource_id": resource.ID, "period": "7d",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	var toolHistory struct {
		ResourceID string                 `json:"resource_id"`
		Source     string                 `json:"source"`
		Points     []tools.RawMetricPoint `json:"points"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &toolHistory))
	require.Equal(t, resource.ID, toolHistory.ResourceID)
	require.Equal(t, apiHistory.Source, toolHistory.Source)
	require.Len(t, toolHistory.Points, 2)
	require.Len(t, apiHistory.Points, len(toolHistory.Points))
	for i, point := range toolHistory.Points {
		require.Equal(t, apiHistory.Points[i].Value, point.Value)
	}
}
