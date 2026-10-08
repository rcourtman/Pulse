package api

import (
	"encoding/json"
	"testing"
)

func TestChartResponsesUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		key  string
	}{
		{name: "chart_data", raw: EmptyChartResponse(), key: "data"},
		{name: "chart_node_data", raw: EmptyChartResponse(), key: "nodeData"},
		{name: "chart_storage_data", raw: EmptyChartResponse(), key: "storageData"},
		{name: "chart_docker_data", raw: EmptyChartResponse(), key: "dockerData"},
		{name: "chart_docker_host_data", raw: EmptyChartResponse(), key: "dockerHostData"},
		{name: "chart_agent_data", raw: EmptyChartResponse(), key: "agentData"},
		{name: "chart_guest_types", raw: EmptyChartResponse(), key: "guestTypes"},
		{name: "infra_node_data", raw: EmptyInfrastructureChartsResponse(), key: "nodeData"},
		{name: "infra_docker_host_data", raw: EmptyInfrastructureChartsResponse(), key: "dockerHostData"},
		{name: "infra_agent_data", raw: EmptyInfrastructureChartsResponse(), key: "agentData"},
		{name: "workload_data", raw: EmptyWorkloadChartsResponse(), key: "data"},
		{name: "workload_docker_data", raw: EmptyWorkloadChartsResponse(), key: "dockerData"},
		{name: "workload_guest_types", raw: EmptyWorkloadChartsResponse(), key: "guestTypes"},
		{name: "storage_pools", raw: EmptyStorageChartsResponse(), key: "pools"},
		{name: "storage_disks", raw: EmptyStorageChartsResponse(), key: "disks"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			if err != nil {
				t.Fatalf("marshal %s: %v", tc.name, err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}

			value, ok := decoded[tc.key].(map[string]any)
			if !ok || len(value) != 0 {
				t.Fatalf("expected %s to be an empty object, got %T (%v)", tc.key, decoded[tc.key], decoded[tc.key])
			}
		})
	}
}

func TestStorageChartsNormalizeNestedCollections(t *testing.T) {
	payload, err := json.Marshal(StorageChartsResponse{
		Pools: map[string]StoragePoolChartData{
			"pool1": {Name: "pool1"},
		},
		Disks: map[string]StorageDiskChartData{
			"disk1": {Name: "disk1"},
		},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized storage charts: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode normalized storage charts: %v", err)
	}

	pools := decoded["pools"].(map[string]any)
	pool := pools["pool1"].(map[string]any)
	for _, key := range []string{"usage", "used", "avail"} {
		values, ok := pool[key].([]any)
		if !ok || len(values) != 0 {
			t.Fatalf("expected pools.pool1.%s to be an empty array, got %T (%v)", key, pool[key], pool[key])
		}
	}

	disks := decoded["disks"].(map[string]any)
	disk := disks["disk1"].(map[string]any)
	values, ok := disk["temperature"].([]any)
	if !ok || len(values) != 0 {
		t.Fatalf("expected disks.disk1.temperature to be an empty array, got %T (%v)", disk["temperature"], disk["temperature"])
	}
}
