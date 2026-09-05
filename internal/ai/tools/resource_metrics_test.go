package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestPerformanceMetricsCanonicalIdentity(t *testing.T) {
	provider := newSummarizeStubProvider()
	history := &mockMetricsHistoryProvider{}
	history.On("GetResourceMetrics", "delly-node-id", 24*time.Hour).Return([]MetricPoint{{CPU: 23, Memory: 41}}, nil)
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	executor.unifiedResourceProvider = provider
	executor.metricsHistory = history
	for _, ref := range []string{"host-abc123", "delly", "delly-node-id"} {
		for _, kind := range []string{"agent", "node", ""} {
			result, err := executor.ExecuteTool(context.Background(), agentcapabilities.PulseMetricsToolName, map[string]interface{}{"type": "performance", "resource_id": ref, "resource_type": kind})
			require.NoError(t, err)
			require.False(t, result.IsError, "%+v", result)
			var response MetricsResponse
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
			require.Equal(t, "host-abc123", response.ResourceID)
			require.Len(t, response.Points, 1)
			require.Equal(t, 23.0, response.Points[0].CPU)
		}
	}
	history.AssertNumberOfCalls(t, "GetResourceMetrics", 9)
	provider.byType[unifiedresources.ResourceTypeAgent] = append(provider.byType[unifiedresources.ResourceTypeAgent], unifiedresources.Resource{ID: "agent-second", Type: unifiedresources.ResourceTypeAgent, Name: "delly"})
	for _, ref := range []string{"delly", "absent-resource"} {
		result, err := executor.executeGetMetrics(context.Background(), map[string]interface{}{"resource_id": ref})
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	history.AssertNumberOfCalls(t, "GetResourceMetrics", 9)
	history.AssertExpectations(t)
}

func TestDiskMetricsRoutingPreservesFilters(t *testing.T) {
	provider := newSummarizeStubProvider()
	provider.byType[unifiedresources.ResourceTypePhysicalDisk] = []unifiedresources.Resource{
		{ID: "disk-nvme", Type: unifiedresources.ResourceTypePhysicalDisk, ParentName: "Friendly host", Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"delly"}}, PhysicalDisk: &unifiedresources.PhysicalDiskMeta{DiskType: "nvme", Health: "PASSED", Temperature: 39}},
		{ID: "disk-sata", Type: unifiedresources.ResourceTypePhysicalDisk, ParentName: "Friendly host", Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"delly"}}, PhysicalDisk: &unifiedresources.PhysicalDiskMeta{DiskType: "sata", Health: "FAILED", Temperature: 52}},
	}
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	executor.unifiedResourceProvider = provider
	for _, tc := range []struct {
		format string
		count  int
	}{{"", 2}, {"nvme", 1}, {"sas", 0}} {
		result, err := executor.ExecuteTool(context.Background(), agentcapabilities.PulseMetricsToolName, map[string]interface{}{"type": "disks", "node": "delly", "disk_type": tc.format})
		require.NoError(t, err)
		require.False(t, result.IsError)
		var response PhysicalDisksResponse
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
		require.Len(t, response.Disks, tc.count)
		require.Equal(t, tc.count, response.Filtered)
	}
}

func TestQueryNodeAliasReturnsCanonicalAgent(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	executor.unifiedResourceProvider = newSummarizeStubProvider()
	result, err := executor.ExecuteTool(context.Background(), agentcapabilities.PulseQueryToolName, map[string]interface{}{"action": "get", "resource_type": "node", "resource_id": "delly"})
	require.NoError(t, err)
	require.False(t, result.IsError, "%+v", result)
	require.Contains(t, result.Content[0].Text, "host-abc123")
	require.Contains(t, result.Content[0].Text, `"type":"agent"`)
}
