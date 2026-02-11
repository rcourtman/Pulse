package kubernetesagent

import (
	"testing"

	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func TestParsePodSummaryMetricsPayload(t *testing.T) {
	raw := []byte(`{
		"pods": [
			{
				"podRef": {"name": "api-0", "namespace": "default"},
				"network": {"rxBytes": 1200, "txBytes": 2400},
				"ephemeral-storage": {"usedBytes": 4096, "capacityBytes": 16384}
			},
			{
				"podRef": {"name": "worker-1", "namespace": "ops"},
				"network": {"rxBytes": 200, "txBytes": 500},
				"volume": [{"usedBytes": 1024, "capacityBytes": 8192}]
			}
		]
	}`)

	usage, err := parsePodSummaryMetricsPayload(raw)
	if err != nil {
		t.Fatalf("parsePodSummaryMetricsPayload: %v", err)
	}

	apiUsage, ok := usage["default/api-0"]
	if !ok {
		t.Fatalf("expected usage for default/api-0")
	}
	if apiUsage.NetworkRxBytes != 1200 || apiUsage.NetworkTxBytes != 2400 {
		t.Fatalf("unexpected network usage: %+v", apiUsage)
	}
	if apiUsage.EphemeralStorageUsedBytes != 4096 || apiUsage.EphemeralStorageCapacityBytes != 16384 {
		t.Fatalf("unexpected ephemeral storage usage: %+v", apiUsage)
	}

	workerUsage, ok := usage["ops/worker-1"]
	if !ok {
		t.Fatalf("expected usage for ops/worker-1")
	}
	if workerUsage.EphemeralStorageUsedBytes != 1024 || workerUsage.EphemeralStorageCapacityBytes != 8192 {
		t.Fatalf("expected volume fallback for ephemeral storage fields, got %+v", workerUsage)
	}
}

func TestMergePodSummaryUsage(t *testing.T) {
	podUsage := map[string]agentsk8s.PodUsage{
		"default/api-0": {
			CPUMilliCores: 250,
			MemoryBytes:   2048,
		},
	}

	summary := map[string]podSummaryUsage{
		"default/api-0": {
			NetworkRxBytes:                1000,
			NetworkTxBytes:                1500,
			EphemeralStorageUsedBytes:     5000,
			EphemeralStorageCapacityBytes: 20000,
		},
		"default/new-pod": {
			NetworkRxBytes: 300,
		},
	}

	mergePodSummaryUsage(podUsage, summary)

	merged, ok := podUsage["default/api-0"]
	if !ok {
		t.Fatalf("expected merged usage for existing pod")
	}
	if merged.CPUMilliCores != 250 || merged.MemoryBytes != 2048 {
		t.Fatalf("expected cpu/memory to be preserved, got %+v", merged)
	}
	if merged.NetworkRxBytes != 1000 || merged.NetworkTxBytes != 1500 {
		t.Fatalf("expected network usage merged, got %+v", merged)
	}
	if merged.EphemeralStorageUsedBytes != 5000 || merged.EphemeralStorageCapacityBytes != 20000 {
		t.Fatalf("expected ephemeral storage merged, got %+v", merged)
	}

	newPodUsage, ok := podUsage["default/new-pod"]
	if !ok {
		t.Fatalf("expected summary-only pod usage to be added")
	}
	if newPodUsage.NetworkRxBytes != 300 {
		t.Fatalf("unexpected summary-only pod usage: %+v", newPodUsage)
	}
}

func TestHasPodUsage(t *testing.T) {
	if hasPodUsage(agentsk8s.PodUsage{}) {
		t.Fatal("expected empty usage to be false")
	}
	if !hasPodUsage(agentsk8s.PodUsage{NetworkRxBytes: 1}) {
		t.Fatal("expected network-only usage to be true")
	}
}

func TestSummaryNodeNames_DedupesAndCaps(t *testing.T) {
	nodes := []agentsk8s.Node{
		{Name: " node-a "},
		{Name: ""},
		{Name: "node-b"},
		{Name: "node-a"},
		{Name: "node-c"},
	}

	names, total := summaryNodeNames(nodes, 2)
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}
	if names[0] != "node-a" || names[1] != "node-b" {
		t.Fatalf("unexpected node selection order: %+v", names)
	}
}

func TestSummaryNodeNames_UnboundedWhenMaxNonPositive(t *testing.T) {
	nodes := []agentsk8s.Node{
		{Name: "node-a"},
		{Name: "node-b"},
	}

	names, total := summaryNodeNames(nodes, 0)
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}
}
