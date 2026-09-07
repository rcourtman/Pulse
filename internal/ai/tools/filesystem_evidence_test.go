package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func TestQueryPreservesFilesystemScopeAndUnavailableMeasurements(t *testing.T) {
	for _, provider := range []bool{false, true} {
		snapshot := commandEvidenceSnapshot()
		snapshot.DockerHosts[0].Containers[0].Filesystems = []filesystem.Observation{
			{Mountpoint: "/cache", Source: "linux-proc-root-statfs", ObservedAt: time.Now().UTC(), Type: "tmpfs", Usage: &filesystem.Usage{CapacityBytes: 8 << 20}},
			{Mountpoint: "/data", Source: "linux-proc-root-statfs", ObservedAt: time.Now().UTC(), Error: "permission denied"},
		}
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(snapshot)
		cfg := ExecutorConfig{ReadState: registry, ControlLevel: ControlLevelReadOnly}
		if provider {
			cfg.UnifiedResourceProvider = &registryUnifiedQueryProvider{registry}
		}
		executor := NewPulseToolExecutor(cfg)
		id := snapshot.DockerHosts[0].Containers[0].Name
		if provider {
			id = registry.ListByType(unifiedresources.ResourceTypeAppContainer)[0].ID
		}
		result, err := executor.executeQuery(context.Background(), map[string]interface{}{"action": "get", "resource_type": "app-container", "resource_id": id})
		if err != nil || result.IsError {
			t.Fatalf("provider=%v: %v %+v", provider, err, result)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
			t.Fatal(err)
		}
		rows, ok := decoded["filesystems"].([]any)
		if !ok || len(rows) != 2 {
			t.Fatalf("filesystem evidence missing: %+v", decoded)
		}
		full := rows[0].(map[string]any)
		unavailable := rows[1].(map[string]any)
		usage := full["usage"].(map[string]any)
		if full["mountpoint"] != "/cache" || full["type"] != "tmpfs" || usage["capacityBytes"] != float64(8<<20) || usage["availableBytes"] != float64(0) {
			t.Fatalf("filesystem scope/measurement changed: %+v", full)
		}
		if _, exists := unavailable["usage"]; exists || unavailable["error"] != "permission denied" {
			t.Fatalf("unavailable read became measurement: %+v", unavailable)
		}
		if _, exists := decoded["disk"]; exists {
			t.Fatal("resource query invented container-wide disk usage")
		}
		t.Logf("FILESYSTEM_MODEL_EVIDENCE provider=%v %s", provider, result.Content[0].Text)
	}
}
