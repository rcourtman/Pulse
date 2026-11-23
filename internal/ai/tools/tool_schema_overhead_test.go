package tools

import (
	"encoding/json"
	"math"
	"sort"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestToolSchemaTokenOverheadMeasurement(t *testing.T) {
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes:              []models.Node{{ID: "node-1", Name: "pve1", Status: "online"}},
		DockerHosts:        []models.DockerHost{{ID: "docker-1", Hostname: "lab-host", Status: "online"}},
		KubernetesClusters: []models.KubernetesCluster{{ID: "k8s-1", Name: "cluster-1"}},
		PMGInstances:       []models.PMGInstance{{ID: "pmg-1", Name: "pmg-1"}},
	})

	exec := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: adapter,
		ControlLevel:            ControlLevelControlled,
	})

	allTools := exec.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{})
	if len(allTools) == 0 {
		t.Fatal("expected non-empty provider tool manifest")
	}

	type toolCost struct {
		name   string
		bytes  int
		tokens int
	}
	costs := make([]toolCost, 0, len(allTools))
	totalBytes := 0
	byName := make(map[string]toolCost, len(allTools))
	for _, tool := range allTools {
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("marshal tool %s: %v", tool.Name, err)
		}
		entry := toolCost{
			name:   tool.Name,
			bytes:  len(raw),
			tokens: int(math.Ceil(float64(len(raw)) / 4.0)),
		}
		costs = append(costs, entry)
		byName[tool.Name] = entry
		totalBytes += len(raw)
	}
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].bytes > costs[j].bytes
	})

	totalTokens := int(math.Ceil(float64(totalBytes) / 4.0))
	t.Logf("total tool schema overhead: %d tools, %d bytes (~%d tokens)", len(allTools), totalBytes, totalTokens)
	for _, c := range costs {
		t.Logf("  %-32s %5d bytes (~%4d tokens)", c.name, c.bytes, c.tokens)
	}

	// Verify optional integration tools contribute measurable schema overhead
	// that dynamic estate scoping eliminates when those integrations are absent.
	for _, name := range []string{
		agentcapabilities.PulseDockerToolName,
		agentcapabilities.PulseKubernetesToolName,
		agentcapabilities.PulsePMGToolName,
	} {
		entry, ok := byName[name]
		if !ok || entry.bytes == 0 {
			t.Fatalf("expected measurable schema overhead for %s", name)
		}
	}
	scopedSavings := byName[agentcapabilities.PulseKubernetesToolName].tokens + byName[agentcapabilities.PulsePMGToolName].tokens
	if scopedSavings == 0 {
		t.Fatal("expected non-zero token savings from scoping unused Kubernetes and PMG tools")
	}
	t.Logf("Docker-only estate savings (dropping pulse_kubernetes + pulse_pmg): ~%d tokens (%.1f%%)",
		scopedSavings, float64(scopedSavings)*100.0/float64(totalTokens))
}

func TestPulseToolExecutor_IntegrationPresence(t *testing.T) {
	emptyExec := NewPulseToolExecutor(ExecutorConfig{})
	if got := emptyExec.IntegrationPresence(); got.Populated {
		t.Fatalf("expected unpopulated presence without read state, got %+v", got)
	}

	emptyAdapter := unifiedresources.NewMonitorAdapter(nil)
	emptyAdapter.PopulateFromSnapshot(models.StateSnapshot{})
	emptySnapshotExec := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: emptyAdapter})
	if got := emptySnapshotExec.IntegrationPresence(); got.Populated {
		t.Fatalf("expected unpopulated presence on empty snapshot, got %+v", got)
	}

	dockerOnly := unifiedresources.NewMonitorAdapter(nil)
	dockerOnly.PopulateFromSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{ID: "docker-1", Hostname: "lab", Status: "online"}},
	})
	dockerExec := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: dockerOnly})
	got := dockerExec.IntegrationPresence()
	if !got.Populated || !got.Docker || got.Kubernetes || got.PMG {
		t.Fatalf("unexpected docker-only integration presence: %+v", got)
	}

	pveOnly := unifiedresources.NewMonitorAdapter(nil)
	pveOnly.PopulateFromSnapshot(models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-1", Name: "pve1", Status: "online"}},
	})
	pveExec := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: pveOnly})
	got = pveExec.IntegrationPresence()
	if !got.Populated || got.Docker || got.Kubernetes || got.PMG {
		t.Fatalf("unexpected pve-only integration presence: %+v", got)
	}
}
