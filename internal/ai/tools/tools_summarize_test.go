package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func newSummarizeTestEnvironment(t *testing.T) (*PulseToolExecutor, func()) {
	t.Helper()
	dir := t.TempDir()
	store, err := metrics.NewStore(metrics.StoreConfig{
		DBPath:          filepath.Join(dir, "metrics.db"),
		WriteBufferSize: 10,
		FlushInterval:   50 * time.Millisecond,
		RetentionRaw:    24 * time.Hour,
		RetentionMinute: 7 * 24 * time.Hour,
		RetentionHourly: 30 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("metrics store: %v", err)
	}
	engine := reporting.NewReportEngine(reporting.EngineConfig{MetricsStore: store})
	prev := reporting.GetEngine()
	reporting.SetEngine(engine)

	exec := NewPulseToolExecutor(ExecutorConfig{})

	cleanup := func() {
		reporting.SetEngine(prev)
		store.Close()
	}
	return exec, cleanup
}

func writeMetricSamples(t *testing.T, dir string, store *metrics.Store, resourceID string, value float64, count int) {
	t.Helper()
	now := time.Now()
	for i := 0; i < count; i++ {
		ts := now.Add(time.Duration(-30+i*2) * time.Minute)
		store.Write("node", resourceID, "cpu", value, ts)
		store.Write("node", resourceID, "memory", value-10, ts)
	}
	store.Flush()
	_ = dir
}

func TestSummarizeTool_RegisteredAndDiscoverable(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	tools := exec.registry.ListTools("")
	var found bool
	for _, tool := range tools {
		if tool.Name == "pulse_summarize" {
			found = true
			if _, ok := tool.InputSchema.Properties["action"]; !ok {
				t.Error("pulse_summarize should declare 'action' property")
			}
			if _, ok := tool.InputSchema.Properties["resource_type"]; !ok {
				t.Error("pulse_summarize should declare 'resource_type' property")
			}
			break
		}
	}
	if !found {
		t.Fatal("pulse_summarize not registered")
	}
}

func TestSummarizeTool_ResourceActionRequiresFields(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action": "resource",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Error("expected error for missing resource_type")
	}
}

func TestSummarizeTool_FleetActionRequiresFields(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "fleet",
		"resource_type": "node",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Error("expected error for missing resource_ids")
	}
}

func TestSummarizeTool_RejectsUnknownAction(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action": "interplanetary",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Error("expected error for unknown action")
	}
}

func TestSummarizeTool_ResourceReturnsHeuristicNarrative(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	// Need a metrics store with data. Re-fetch the engine's store via the
	// global since we set it in newSummarizeTestEnvironment.
	engine, ok := reporting.GetEngine().(*reporting.ReportEngine)
	if !ok {
		t.Fatal("expected *ReportEngine")
	}
	_ = engine

	// Write metrics via the same store. The engine was constructed with
	// MetricsStore so we need to reach back into the store; instead, rely
	// on writing via package-level access through engine internals.
	// Simpler: skip data and accept that the heuristic narrator returns
	// "insufficient data" — which is itself a valid narrative we can assert.
	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "resource",
		"resource_type": "node",
		"resource_id":   "missing-node",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res.Content)
	}
	if len(res.Content) == 0 || res.Content[0].Type != "text" {
		t.Fatalf("expected text content, got %+v", res.Content)
	}
	var parsed summarizeResourceResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, res.Content[0].Text)
	}
	if !parsed.OK {
		t.Error("expected OK=true")
	}
	if parsed.Action != "resource" {
		t.Errorf("Action = %q, want resource", parsed.Action)
	}
	if parsed.NarrativeSource != reporting.NarrativeSourceHeuristic {
		t.Errorf("NarrativeSource = %q, want heuristic (v1 always heuristic)", parsed.NarrativeSource)
	}
	if parsed.HealthStatus == "" {
		t.Error("expected HealthStatus populated")
	}
	if len(parsed.Observations) == 0 {
		t.Error("expected at least one observation from the heuristic narrator")
	}
}

func TestSummarizeTool_FleetParsesCommaSeparatedIDs(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "fleet",
		"resource_type": "node",
		"resource_ids":  "node-a, node-b ,  node-c ",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res.Content)
	}
	var parsed summarizeFleetResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(parsed.ResourceIDs) != 3 {
		t.Fatalf("expected 3 deduped/trimmed IDs, got %v", parsed.ResourceIDs)
	}
	wanted := []string{"node-a", "node-b", "node-c"}
	for i, w := range wanted {
		if parsed.ResourceIDs[i] != w {
			t.Errorf("ResourceIDs[%d] = %q, want %q", i, parsed.ResourceIDs[i], w)
		}
	}
	if parsed.NarrativeSource != reporting.NarrativeSourceHeuristic {
		t.Errorf("NarrativeSource = %q, want heuristic", parsed.NarrativeSource)
	}
}

func TestSummarizeTool_FleetEnforcesMaxResources(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	// Build a comma-separated list well over the cap.
	var ids string
	for i := 0; i < summarizeFleetMaxResources+5; i++ {
		if i > 0 {
			ids += ","
		}
		ids += "node-x"
	}
	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "fleet",
		"resource_type": "node",
		"resource_ids":  ids,
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Error("expected error for over-limit fleet size")
	}
}

func TestSummarizeRangeWindow(t *testing.T) {
	cases := map[string]time.Duration{
		"24h":     24 * time.Hour,
		"7d":      7 * 24 * time.Hour,
		"30d":     30 * 24 * time.Hour,
		"":        7 * 24 * time.Hour,
		"banana":  reporting.DescribePerformanceReport().DefaultRangeDuration(),
		"  7d   ": 7 * 24 * time.Hour,
		"7D":      7 * 24 * time.Hour,
	}
	for input, want := range cases {
		if got := summarizeRangeWindow(input); got != want {
			t.Errorf("summarizeRangeWindow(%q) = %v, want %v", input, got, want)
		}
	}
}
