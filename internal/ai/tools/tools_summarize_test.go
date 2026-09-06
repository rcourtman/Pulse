package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

func TestSummarizeTool_RegisteredAndDiscoverable(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	tools := exec.registry.ListTools(InvocationPolicy{})
	var found bool
	for _, tool := range tools {
		if tool.Name == agentcapabilities.PulseSummarizeToolName {
			found = true
			if _, ok := tool.InputSchema.Properties["action"]; !ok {
				t.Errorf("%s should declare 'action' property", agentcapabilities.PulseSummarizeToolName)
			}
			if _, ok := tool.InputSchema.Properties["resource_type"]; !ok {
				t.Errorf("%s should declare 'resource_type' property", agentcapabilities.PulseSummarizeToolName)
			}
			break
		}
	}
	if !found {
		t.Fatalf("%s not registered", agentcapabilities.PulseSummarizeToolName)
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

	// No retained samples must remain an explicit empty evidence result.
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
	if parsed.Scope.Source != "retained_metrics" || parsed.Evidence == nil || len(parsed.Evidence.Metrics) != 0 {
		t.Fatalf("expected empty retained evidence, got %+v", parsed)
	}
	if parsed.Scope.TimeSemantics == "" {
		t.Fatal("even empty results must disclose the retained timestamp semantics")
	}
	for _, field := range []string{"health_status", "health_message", "observations", "recommendations"} {
		if strings.Contains(res.Content[0].Text, `"`+field+`"`) {
			t.Fatalf("empty evidence invented %s: %s", field, res.Content[0].Text)
		}
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
	if parsed.Scope.Source != "retained_metrics" || len(parsed.Evidence) != 3 {
		t.Fatalf("expected retained evidence for all resources: %+v", parsed)
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

// stubReportNarrator implements reporting.Narrator with a recorded
// invocation so the test can assert the tool delegates to it.
type stubReportNarrator struct {
	called   bool
	seen     reporting.NarrativeInput
	response reporting.Narrative
}

func (s *stubReportNarrator) Narrate(_ context.Context, in reporting.NarrativeInput) (reporting.Narrative, error) {
	s.called = true
	s.seen = in
	return s.response, nil
}

type stubFleetReportNarrator struct {
	called   bool
	seen     reporting.FleetNarrativeInput
	response reporting.FleetNarrative
}

func (s *stubFleetReportNarrator) NarrateFleet(_ context.Context, in reporting.FleetNarrativeInput) (reporting.FleetNarrative, error) {
	s.called = true
	s.seen = in
	return s.response, nil
}

func TestSummarizeTool_UsesReportNarratorWhenConfigured(t *testing.T) {
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
	defer store.Close()
	engine := reporting.NewReportEngine(reporting.EngineConfig{MetricsStore: store})
	prev := reporting.GetEngine()
	reporting.SetEngine(engine)
	defer reporting.SetEngine(prev)

	narrator := &stubReportNarrator{
		response: reporting.Narrative{
			Source:        reporting.NarrativeSourceAI,
			HealthStatus:  "HEALTHY",
			HealthMessage: "AI says fine",
			Observations: []reporting.NarrativeBullet{
				{Text: "AI bullet", Severity: reporting.NarrativeSeverityOK},
			},
			Recommendations: []string{"Continue monitoring"},
		},
	}
	exec := NewPulseToolExecutor(ExecutorConfig{
		ReportNarrator: narrator,
	})

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "resource",
		"resource_type": "node",
		"resource_id":   "node-with-narrator",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res.Content)
	}
	if narrator.called {
		t.Fatal("metric evidence must not invoke a second model")
	}
	var parsed summarizeResourceResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if parsed.Scope.Source != "retained_metrics" || parsed.Evidence == nil {
		t.Fatalf("expected measured evidence, got %+v", parsed)
	}
	if strings.Contains(res.Content[0].Text, "AI says fine") || strings.Contains(res.Content[0].Text, "health_status") {
		t.Fatal("narrator judgment leaked into metric evidence")
	}

}

func TestSummarizeTool_FleetDoesNotInvokeNestedNarrator(t *testing.T) {
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
	defer store.Close()
	engine := reporting.NewReportEngine(reporting.EngineConfig{MetricsStore: store})
	prev := reporting.GetEngine()
	reporting.SetEngine(engine)
	defer reporting.SetEngine(prev)

	fleet := &stubFleetReportNarrator{
		response: reporting.FleetNarrative{
			Source:           reporting.NarrativeSourceAI,
			HealthStatus:     "WARNING",
			HealthMessage:    "Memory creeping up",
			ExecutiveSummary: "AI fleet summary text",
			Outliers: []reporting.FleetOutlier{
				{ResourceID: "node-a", ResourceName: "alpha", Reason: "Memory at 92%", Severity: reporting.NarrativeSeverityWarning},
			},
			Recommendations: []string{"Review memory allocation"},
		},
	}
	exec := NewPulseToolExecutor(ExecutorConfig{
		ReportFleetNarrator: fleet,
	})

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "fleet",
		"resource_type": "node",
		"resource_ids":  "node-a,node-b",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res.Content)
	}
	if fleet.called {
		t.Fatal("fleet evidence must not invoke a second model")
	}
	var parsed summarizeFleetResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if parsed.Scope.Source != "retained_metrics" || len(parsed.Evidence) != 2 {
		t.Fatalf("expected both resources as evidence, got %+v", parsed)
	}
	if strings.Contains(res.Content[0].Text, "Memory creeping up") || strings.Contains(res.Content[0].Text, "outliers") {
		t.Fatal("fleet judgment leaked into metric evidence")
	}

}

// The production unified provider (the monitor adapter) must satisfy the
// on-demand metrics-target resolver, or fleet enumeration silently degrades
// to fixture-populated targets only.
var _ resourceMetricsTargetResolver = (*unifiedresources.MonitorAdapter)(nil)

// stubSummarizeResourceProvider implements UnifiedResourceProvider plus the
// on-demand metrics-target resolver the production monitor adapter exposes.
type stubSummarizeResourceProvider struct {
	byType  map[unifiedresources.ResourceType][]unifiedresources.Resource
	targets map[string]*unifiedresources.MetricsTarget
}

func (s *stubSummarizeResourceProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	return s.byType[t]
}

func (s *stubSummarizeResourceProvider) MetricsTargetForResource(resourceID string) *unifiedresources.MetricsTarget {
	return s.targets[resourceID]
}

func newSummarizeStubProvider() *stubSummarizeResourceProvider {
	return &stubSummarizeResourceProvider{
		byType: map[unifiedresources.ResourceType][]unifiedresources.Resource{
			unifiedresources.ResourceTypeAgent: {
				{
					ID:      "host-abc123",
					Type:    unifiedresources.ResourceTypeAgent,
					Name:    "delly",
					Status:  unifiedresources.StatusOnline,
					Proxmox: &unifiedresources.ProxmoxData{},
				},
			},
			unifiedresources.ResourceTypeVM: {
				{
					ID:     "vm-def456",
					Type:   unifiedresources.ResourceTypeVM,
					Name:   "media-server",
					Status: unifiedresources.StatusOnline,
				},
			},
		},
		targets: map[string]*unifiedresources.MetricsTarget{
			"host-abc123": {ResourceType: "node", ResourceID: "delly-node-id"},
			"vm-def456":   {ResourceType: "vm", ResourceID: "pve1:node:101"},
		},
	}
}

func TestSummarizeTool_FleetEnumeratesWhenIDsOmitted(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action": "fleet",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected omitted resource_ids to self-enumerate, got error: %+v", res.Content)
	}
	var parsed summarizeFleetResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !parsed.Enumerated {
		t.Error("expected enumerated=true")
	}
	if len(parsed.ResourceIDs) != 2 {
		t.Fatalf("expected 2 enumerated resources, got %v", parsed.ResourceIDs)
	}
	// Infrastructure enumerates before guests.
	if parsed.ResourceIDs[0] != "host-abc123" || parsed.ResourceIDs[1] != "vm-def456" {
		t.Errorf("ResourceIDs = %v, want infrastructure first", parsed.ResourceIDs)
	}
	if len(parsed.Resources) != 2 || parsed.Resources[0].Name != "delly" || parsed.Resources[1].Type != "vm" {
		t.Errorf("Resources = %+v", parsed.Resources)
	}
	// Pure Proxmox history follows its canonical node store coordinates.
	if parsed.Resources[0].Type != "node" {
		t.Errorf("host entry type = %q, want node", parsed.Resources[0].Type)
	}
}

func TestSummarizeTool_FleetEnumerationHonorsTypeFilter(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":        "fleet",
		"resource_type": "vm",
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
	if len(parsed.ResourceIDs) != 1 || parsed.ResourceIDs[0] != "vm-def456" {
		t.Errorf("ResourceIDs = %v, want only the VM", parsed.ResourceIDs)
	}
}

func TestSummarizeTool_FleetEnumerationCapsAndNotes(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	provider := newSummarizeStubProvider()
	var vms []unifiedresources.Resource
	for i := 0; i < summarizeFleetMaxResources+10; i++ {
		vms = append(vms, unifiedresources.Resource{
			ID:   fmt.Sprintf("vm-%03d", i),
			Type: unifiedresources.ResourceTypeVM,
			Name: fmt.Sprintf("guest-%03d", i),
		})
	}
	provider.byType[unifiedresources.ResourceTypeVM] = vms
	exec.SetUnifiedResourceProvider(provider)

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action": "fleet",
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
	if len(parsed.ResourceIDs) != summarizeFleetMaxResources {
		t.Errorf("expected cap at %d resources, got %d", summarizeFleetMaxResources, len(parsed.ResourceIDs))
	}
	if parsed.Note == "" {
		t.Error("expected truncation note when the enumeration exceeds the cap")
	}
}

func TestSummarizeTool_FleetEnumerationEmptyFleetErrorForbidsAskingOperator(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action": "fleet",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error when no resources are known")
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].Text, "do not ask the operator") {
		t.Errorf("empty-fleet error must steer the model away from operator questions, got %+v", res.Content)
	}
}

func TestSummarizeTool_FleetResolvesNamesAndTranslatesMetricsIDs(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())

	fleet := &stubFleetReportNarrator{
		response: reporting.FleetNarrative{Source: reporting.NarrativeSourceAI},
	}
	exec.reportFleetNarrator = fleet

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":       "fleet",
		"resource_ids": "delly, media-server",
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
	if len(parsed.ResourceIDs) != 2 || parsed.ResourceIDs[0] != "host-abc123" || parsed.ResourceIDs[1] != "vm-def456" {
		t.Errorf("names should resolve to canonical IDs, got %v", parsed.ResourceIDs)
	}
	if parsed.Enumerated {
		t.Error("explicit resource_ids must not report enumerated=true")
	}
}

func TestSummarizeTool_FleetUnresolvedIDsWithoutTypeErrorsWithRecoveryPath(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":       "fleet",
		"resource_ids": "no-such-thing",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for unresolvable IDs without a resource_type")
	}
	if !strings.Contains(res.Content[0].Text, "no resource_ids to enumerate the fleet automatically") {
		t.Errorf("error must point at the self-enumeration path, got %q", res.Content[0].Text)
	}
}

func TestSummarizeTool_ResourceResolvesNameWithoutType(t *testing.T) {
	exec, cleanup := newSummarizeTestEnvironment(t)
	defer cleanup()
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())

	res, err := exec.executeSummarize(context.Background(), map[string]interface{}{
		"action":      "resource",
		"resource_id": "delly",
	})
	if err != nil {
		t.Fatalf("executeSummarize: %v", err)
	}
	if res.IsError {
		t.Fatalf("known resource name should not require resource_type, got %+v", res.Content)
	}
	var parsed summarizeResourceResponse
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if parsed.ResourceID != "host-abc123" {
		t.Errorf("ResourceID = %q, want canonical host-abc123", parsed.ResourceID)
	}
	if parsed.ResourceType != "node" {
		t.Errorf("ResourceType = %q, want node", parsed.ResourceType)
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

func TestSummarizeToolReturnsHighUtilizationAsEvidenceWithoutDiagnosis(t *testing.T) {
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().Add(-time.Hour).Truncate(time.Minute)
	store.Write("node", "delly-node-id", "memory", 92, at)
	store.Flush()
	previous := reporting.GetEngine()
	reporting.SetEngine(reporting.NewReportEngine(reporting.EngineConfig{MetricsStore: store}))
	defer reporting.SetEngine(previous)
	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.SetUnifiedResourceProvider(newSummarizeStubProvider())
	for _, args := range []map[string]interface{}{
		{"action": "resource", "resource_id": "delly", "range": "24h"},
		{"action": "fleet", "resource_ids": "delly", "range": "24h"},
	} {
		result, err := exec.executeSummarize(context.Background(), args)
		if err != nil || result.IsError {
			t.Fatalf("evidence = %+v, %v", result, err)
		}
		text := result.Content[0].Text
		for _, absent := range []string{"health_status", "HEALTHY", "CRITICAL", "pressure", "recommendations"} {
			if strings.Contains(text, absent) {
				t.Fatalf("unexpected diagnosis %q in %s", absent, text)
			}
		}
		if !strings.Contains(text, `"mean":92`) || !strings.Contains(text, `"retained_points":1`) || !strings.Contains(text, `"disk_health"`) || !strings.Contains(text, `"alerts"`) {
			t.Fatalf("reading or unqueried source scope missing: %s", text)
		}
	}
}
