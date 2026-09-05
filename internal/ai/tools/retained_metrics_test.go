package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
)

func TestPerformanceMetricsRetainedAcrossRestart(t *testing.T) {
	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 8 * 24 * time.Hour
	store, err := metrics.NewStore(cfg)
	require.NoError(t, err)
	at := time.Now().Add(-6 * time.Hour).Truncate(time.Second)
	for _, family := range []string{"node", "agent", "vm", "container"} {
		store.Write(family, family+"-native", "cpu", 37, at)
		store.Write(family, family+"-native", "memory", 62, at)
	}
	require.NoError(t, store.Close())
	store, err = metrics.NewStore(cfg)
	require.NoError(t, err)
	defer store.Close()
	adapter := NewMetricsHistoryToolAdapter(&fakeMetricsSource{}, &fakeReadState{}, store)
	for _, tc := range []struct{ kind, family string }{{"agent", "node"}, {"agent", "agent"}, {"vm", "vm"}, {"system-container", "container"}} {
		t.Run(tc.family, func(t *testing.T) {
			resource := unifiedresources.Resource{ID: "canonical-" + tc.family, Type: unifiedresources.ResourceType(tc.kind), Name: "machine-" + tc.family, MetricsTarget: &unifiedresources.MetricsTarget{ResourceType: tc.kind, ResourceID: tc.family + "-native"}}
			executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}, MetricsHistory: adapter, UnifiedResourceProvider: &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{resource}}})
			result, err := executor.executeGetMetrics(context.Background(), map[string]interface{}{"resource_id": resource.ID, "period": "24h"})
			require.NoError(t, err)
			require.False(t, result.IsError)
			var response MetricsResponse
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
			require.Equal(t, resource.ID, response.ResourceID)
			require.Len(t, response.Points, 1)
			require.Equal(t, at.Unix(), response.Points[0].Timestamp.Unix())
			require.Equal(t, 37.0, response.Points[0].CPU)
			require.Equal(t, 62.0, response.Points[0].Memory)
		})
	}
	// A store failure must not become a successful empty history or fall back to
	// shorter in-memory history, which could change the operator's conclusion.
	require.NoError(t, store.Close())
	_, err = adapter.GetResourceMetricsForTarget(unifiedresources.MetricsTarget{ResourceType: "agent", ResourceID: "node-native"}, 24*time.Hour)
	require.Error(t, err)
}

func TestCanonicalAgentCPUCountUsesAvailableSource(t *testing.T) {
	resource := unifiedresources.Resource{Type: unifiedresources.ResourceTypeAgent, Proxmox: &unifiedresources.ProxmoxData{CPUs: 12}}
	require.Equal(t, 12, canonicalAgentResponse(resource, nil).CPU.Cores)
	resource.Proxmox.CPUInfo = &unifiedresources.CPUInfo{Cores: 6, Sockets: 2}
	resource.Proxmox.CPUs = 0
	require.Equal(t, 12, canonicalAgentResponse(resource, nil).CPU.Cores)
	resource.Agent = &unifiedresources.AgentData{}
	require.Equal(t, 12, canonicalAgentResponse(resource, nil).CPU.Cores)
	resource.Agent.CPUCount = 16
	require.Equal(t, 16, canonicalAgentResponse(resource, nil).CPU.Cores)
}

func TestMergedMetricsAreChronological(t *testing.T) {
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	points := mergeMetricsByTimestamp(map[string][]RawMetricPoint{"cpu": {{Value: 3, Timestamp: start.Add(2 * time.Minute)}, {Value: 1, Timestamp: start}, {Value: 2, Timestamp: start.Add(time.Minute)}}})
	require.Len(t, points, 3)
	for i, point := range points {
		require.Equal(t, float64(i+1), point.CPU)
		require.Equal(t, start.Add(time.Duration(i)*time.Minute), point.Timestamp)
	}
}
