package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAssembleSeedWithinBudget_AllFit(t *testing.T) {
	ps := &PatrolService{}

	sections := []seedSection{
		{priority: 0, name: "p0", content: "P0 full\n"},
		{priority: 1, name: "p1", content: "P1 full\n"},
		{priority: 2, name: "p2", content: "P2 full\n", summary: "P2 summary\n"},
		{priority: 3, name: "p3", content: "P3 full\n"},
		{priority: 4, name: "p4", content: "P4 full\n"},
	}

	out := ps.assembleSeedWithinBudget(sections, 10_000)
	for _, expected := range []string{"P0 full", "P1 full", "P2 full", "P3 full", "P4 full"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected output to contain %q, got: %s", expected, out)
		}
	}
	if strings.Contains(out, "P2 summary") {
		t.Fatalf("expected full P2 content when all fit, got: %s", out)
	}
}

func TestAssembleSeedWithinBudget_P4DroppedFirst(t *testing.T) {
	ps := &PatrolService{}

	sections := []seedSection{
		{priority: 0, name: "p0", content: "P0\n" + strings.Repeat("a", 40)},
		{priority: 1, name: "p1", content: "P1\n" + strings.Repeat("b", 40)},
		{priority: 2, name: "p2", content: "P2 full\n" + strings.Repeat("c", 40), summary: "P2 summary\n" + strings.Repeat("d", 16)},
		{priority: 3, name: "p3", content: "P3\n" + strings.Repeat("e", 40)},
		{priority: 4, name: "p4", content: "P4\n" + strings.Repeat("f", 600)},
	}

	budget := seedTokenEstimate(sections[0].content) +
		seedTokenEstimate(sections[1].content) +
		seedTokenEstimate(sections[2].content) +
		seedTokenEstimate(sections[3].content)

	out := ps.assembleSeedWithinBudget(sections, budget)
	if !strings.Contains(out, "P3") {
		t.Fatalf("expected P3 section to remain, got: %s", out)
	}
	if strings.Contains(out, "P4") {
		t.Fatalf("expected P4 section to be dropped first, got: %s", out)
	}
}

func TestAssembleSeedWithinBudget_P2Summarized(t *testing.T) {
	ps := &PatrolService{}

	sections := []seedSection{
		{priority: 0, name: "p0", content: "P0\n" + strings.Repeat("a", 40)},
		{priority: 1, name: "p1", content: "P1\n" + strings.Repeat("b", 40)},
		{priority: 2, name: "p2", content: "P2 full marker\n" + strings.Repeat("c", 600), summary: "P2 summary marker\n" + strings.Repeat("d", 20)},
		{priority: 3, name: "p3", content: "P3\n" + strings.Repeat("e", 20)},
	}

	budget := seedTokenEstimate(sections[0].content) +
		seedTokenEstimate(sections[1].content) +
		seedTokenEstimate(sections[2].summary) +
		seedTokenEstimate(sections[3].content)

	out := ps.assembleSeedWithinBudget(sections, budget)
	if !strings.Contains(out, "P2 summary marker") {
		t.Fatalf("expected P2 summary content, got: %s", out)
	}
	if strings.Contains(out, "P2 full marker") {
		t.Fatalf("expected P2 full content to be replaced by summary, got: %s", out)
	}
	if !strings.Contains(out, "P3") {
		t.Fatalf("expected P3 section to still fit, got: %s", out)
	}
}

func TestAssembleSeedWithinBudget_P2SummaryExceedsBudget(t *testing.T) {
	ps := &PatrolService{}
	sections := []seedSection{
		{priority: 0, name: "p0", content: strings.Repeat("A", 100)},                                    // 25 tokens
		{priority: 2, name: "p2", content: strings.Repeat("B", 200), summary: strings.Repeat("C", 200)}, // 50 tokens each
	}

	// Budget: 30 tokens. P0 takes 25, leaving 5. Neither full nor summary P2 fits.
	result := ps.assembleSeedWithinBudget(sections, 30)
	if strings.Contains(result, "B") || strings.Contains(result, "C") {
		t.Fatal("P2 content should have been dropped when even summary exceeds budget")
	}
	if !strings.Contains(result, "A") {
		t.Fatal("P0 content should always be included")
	}
}

func TestAssembleSeedWithinBudget_P2NoSummaryOverBudget(t *testing.T) {
	ps := &PatrolService{}
	sections := []seedSection{
		{priority: 0, name: "p0", content: strings.Repeat("A", 100)},            // 25 tokens
		{priority: 2, name: "p2_no_summary", content: strings.Repeat("B", 200)}, // 50 tokens, no summary
	}

	// Budget: 30 tokens. P0 takes 25, P2 doesn't fit and has no summary.
	result := ps.assembleSeedWithinBudget(sections, 30)
	if strings.Contains(result, "B") {
		t.Fatal("P2 without summary should be dropped when over budget")
	}
}

func TestAssembleSeedWithinBudget_P3Dropped(t *testing.T) {
	ps := &PatrolService{}

	sections := []seedSection{
		{priority: 0, name: "p0", content: "P0\n" + strings.Repeat("a", 40)},
		{priority: 1, name: "p1", content: "P1\n" + strings.Repeat("b", 40)},
		{priority: 2, name: "p2", content: "P2 full marker\n" + strings.Repeat("c", 600), summary: "P2 summary marker\n" + strings.Repeat("d", 20)},
		{priority: 3, name: "p3", content: "P3 marker\n" + strings.Repeat("e", 100)},
		{priority: 4, name: "p4", content: "P4 marker\n" + strings.Repeat("f", 100)},
	}

	budget := seedTokenEstimate(sections[0].content) +
		seedTokenEstimate(sections[1].content) +
		seedTokenEstimate(sections[2].summary)

	out := ps.assembleSeedWithinBudget(sections, budget)
	if !strings.Contains(out, "P0") || !strings.Contains(out, "P1") {
		t.Fatalf("expected P0/P1 always present, got: %s", out)
	}
	if !strings.Contains(out, "P2 summary marker") {
		t.Fatalf("expected summarized P2 section, got: %s", out)
	}
	if strings.Contains(out, "P3 marker") || strings.Contains(out, "P4 marker") {
		t.Fatalf("expected P3 and P4 sections to be dropped, got: %s", out)
	}
}

func TestCalculateSeedBudget(t *testing.T) {
	t.Run("known patrol model", func(t *testing.T) {
		ps := &PatrolService{
			aiService: &Service{cfg: &config.AIConfig{PatrolModel: "openrouter:abab7"}},
		}

		if got := ps.calculateSeedBudget(); got != 160_000 {
			t.Fatalf("calculateSeedBudget() = %d, want %d", got, 160_000)
		}
	})

	t.Run("minimum floor", func(t *testing.T) {
		ps := &PatrolService{
			aiService: &Service{cfg: &config.AIConfig{PatrolModel: "openai:gpt-4"}},
		}

		if got := ps.calculateSeedBudget(); got != 4_096 {
			t.Fatalf("calculateSeedBudget() floor = %d, want %d", got, 4_096)
		}
	})

	t.Run("floor_clamped_to_half_context", func(t *testing.T) {
		ps := &PatrolService{
			aiService: &Service{cfg: &config.AIConfig{PatrolModel: "openai:gpt-4"}},
		}
		budget := ps.calculateSeedBudget()
		contextWindow := providers.ContextWindowTokens("openai:gpt-4")
		if budget > contextWindow {
			t.Fatalf("budget %d exceeds context window %d", budget, contextWindow)
		}
		if budget > contextWindow/2 {
			t.Fatalf("budget %d exceeds half of context window %d", budget, contextWindow/2)
		}
	})

	t.Run("default context window fallback", func(t *testing.T) {
		ps := &PatrolService{}
		if got := ps.calculateSeedBudget(); got != 92_000 {
			t.Fatalf("calculateSeedBudget() fallback = %d, want %d", got, 92_000)
		}
	})
}

func TestSeedResourceInventorySummary(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := PatrolConfig{
		AnalyzeNodes:   true,
		AnalyzeGuests:  true,
		AnalyzeStorage: true,
	}

	usedOne := int64(910)
	totalOne := int64(1000)
	usedTwo := int64(870)
	totalTwo := int64(1000)
	ps.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{
		getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
			if t != unifiedresources.ResourceTypeStorage {
				return nil
			}
			return []unifiedresources.Resource{
				{
					ID:     "store-1",
					Name:   "local-lvm",
					Status: unifiedresources.StatusOnline,
					Storage: &unifiedresources.StorageMeta{
						Type: "lvm",
					},
					Metrics: &unifiedresources.ResourceMetrics{
						Disk: &unifiedresources.MetricValue{Used: &usedOne, Total: &totalOne},
					},
				},
				{
					ID:     "store-2",
					Name:   "ceph-pool",
					Status: unifiedresources.StatusOnline,
					Storage: &unifiedresources.StorageMeta{
						Type: "ceph",
					},
					Metrics: &unifiedresources.ResourceMetrics{
						Disk: &unifiedresources.MetricValue{Used: &usedTwo, Total: &totalTwo},
					},
				},
			}
		},
	})

	state := modelsStateForSummary()
	scopedSet := map[string]bool{
		"node-1":   true,
		"node-2":   true,
		"qemu/100": true,
		"qemu/101": true,
		"qemu/102": true,
		"lxc/200":  true,
		"store-1":  true,
		"store-2":  true,
	}

	out := ps.seedResourceInventorySummary(state, scopedSet, cfg, time.Now(), nil)

	for _, expected := range []string{
		"# Infrastructure Summary (condensed)",
		"Nodes: 2 (online: 2)",
		"node-2 (CPU 91%)",
		"Guests: 4 (running: 3, stopped: 1)",
		"webserver-01 (CPU 89%)",
		"db-primary (Mem 94%)",
		"Storage: 2 pools (active: 2)",
		"local-lvm (91%)",
		"ceph-pool (87%)",
	} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected summary to contain %q, got:\n%s", expected, out)
		}
	}

	if strings.Contains(out, "| Node |") || strings.Contains(out, "| Name |") {
		t.Fatalf("expected condensed summary (no tables), got:\n%s", out)
	}
}

func seedTokenEstimate(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

func modelsStateForSummary() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", Status: "online", CPU: 0.35, Memory: models.Memory{Usage: 30}, Disk: models.Disk{Usage: 20}},
			{ID: "node-2", Name: "node-2", Status: "online", CPU: 0.91, Memory: models.Memory{Usage: 85}, Disk: models.Disk{Usage: 70}},
		},
		VMs: []models.VM{
			{ID: "qemu/100", Name: "webserver-01", Status: "running", CPU: 0.89, Memory: models.Memory{Usage: 52}, Disk: models.Disk{Usage: 60}},
			{ID: "qemu/101", Name: "db-primary", Status: "running", CPU: 0.30, Memory: models.Memory{Usage: 94}, Disk: models.Disk{Usage: 72}},
			{ID: "qemu/102", Name: "worker-01", Status: "stopped", CPU: 0.03, Memory: models.Memory{Usage: 18}, Disk: models.Disk{Usage: 20}},
		},
		Containers: []models.Container{
			{ID: "lxc/200", Name: "cache-01", Status: "running", CPU: 0.15, Memory: models.Memory{Usage: 45}, Disk: models.Disk{Usage: 82}},
		},
	}
}
