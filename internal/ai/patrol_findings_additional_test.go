package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type patrolTestStateProvider struct {
	state models.StateSnapshot
}

func (p *patrolTestStateProvider) GetState() models.StateSnapshot {
	return p.state
}

// patrolMockChatService implements ChatServiceProvider and chatServiceExecutorAccessor for testing.
type patrolMockChatService struct {
	executor                *tools.PulseToolExecutor
	executePatrolStreamFunc func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error)
}

func (m *patrolMockChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "mock-session"}, nil
}

func (m *patrolMockChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (m *patrolMockChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	if m.executePatrolStreamFunc != nil {
		return m.executePatrolStreamFunc(ctx, req, callback)
	}
	return &PatrolStreamResponse{}, nil
}

func (m *patrolMockChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (m *patrolMockChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *patrolMockChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func (m *patrolMockChatService) GetExecutor() *tools.PulseToolExecutor {
	return m.executor
}

func TestPatrolService_DismissFinding(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	f := &Finding{
		ID:           "f1",
		Key:          "cpu-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		ResourceType: "node",
		Title:        "High CPU",
	}
	ps.findings.Add(f)

	if err := ps.DismissFinding("f1", "not_an_issue", "expected during maintenance"); err != nil {
		t.Fatalf("dismiss finding: %v", err)
	}
	stored := ps.findings.Get("f1")
	if stored == nil || stored.DismissedReason != "not_an_issue" {
		t.Fatalf("expected finding to be dismissed, got %+v", stored)
	}
	if stored.UserNote != "expected during maintenance" {
		t.Fatalf("expected dismissal note to be preserved, got %q", stored.UserNote)
	}

	if err := ps.DismissFinding("f1", "bad_reason", ""); err == nil {
		t.Fatal("expected error for invalid dismissal reason")
	}
}

func TestPatrolFindingCreatorAdapter_IsActionable(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	lowState := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-low",
				Name:   "node-low",
				Status: "online",
				CPU:    0.20,
				Memory: models.Memory{Usage: 30.0},
			},
		},
	}
	lowAdapter := newPatrolFindingCreatorAdapter(ps, lowState)
	_, _, err := lowAdapter.CreateFinding(tools.PatrolFindingInput{
		Key:          "cpu-high",
		Severity:     "warning",
		Category:     "performance",
		ResourceID:   "node-low",
		ResourceName: "node-low",
		ResourceType: "node",
		Title:        "High CPU",
		Description:  "CPU usage is high",
		Evidence:     "CPU > 90%",
	})
	if err == nil {
		t.Fatal("expected low CPU warning finding to be rejected")
	}

	highState := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-high",
				Name:   "node-high",
				Status: "online",
				CPU:    0.90,
				Memory: models.Memory{Usage: 80.0},
			},
		},
	}
	highAdapter := newPatrolFindingCreatorAdapter(ps, highState)
	_, _, err = highAdapter.CreateFinding(tools.PatrolFindingInput{
		Key:          "cpu-high",
		Severity:     "warning",
		Category:     "performance",
		ResourceID:   "node-high",
		ResourceName: "node-high",
		ResourceType: "node",
		Title:        "High CPU",
		Description:  "CPU usage is high",
		Evidence:     "CPU > 90%",
	})
	if err != nil {
		t.Fatalf("expected high CPU warning finding to be accepted, got %v", err)
	}
}

func TestPatrolService_ForcePatrol_RecordsRun(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	mockCS := &patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			_, _, _ = creator.CreateFinding(tools.PatrolFindingInput{
				Severity:     "warning",
				Category:     "performance",
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Title:        "High CPU",
				Description:  "CPU usage is high",
				Evidence:     "CPU > 90%",
			})
			return &PatrolStreamResponse{Content: "Analysis complete"}, nil
		},
	}
	svc.SetChatService(mockCS)

	stateProvider := &patrolTestStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1", Status: "running", CPU: 0.95},
			},
			Nodes: []models.Node{
				{ID: "node-1", Name: "pve-1", Status: "online"},
			},
		},
	}

	ps := NewPatrolService(svc, stateProvider)
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      10 * time.Minute,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	before := ps.runHistoryStore.Count()
	ps.ForcePatrol(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ps.runHistoryStore.Count() > before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected ForcePatrol to record a run (count stayed at %d)", before)
}

// --- Configurable threshold tests ---

func TestActionabilityThreshold_DefaultFallback(t *testing.T) {
	// Zero thresholds → should fall back to old defaults (50/60/70)
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{} // all zero

	adapter := newPatrolFindingCreatorAdapter(ps, models.StateSnapshot{})

	if got := adapter.actionabilityThreshold("cpu", "node"); got != 50.0 {
		t.Errorf("cpu threshold = %v, want 50.0", got)
	}
	if got := adapter.actionabilityThreshold("memory", "vm"); got != 60.0 {
		t.Errorf("guest memory threshold = %v, want 60.0", got)
	}
	if got := adapter.actionabilityThreshold("memory", "node"); got != 60.0 {
		t.Errorf("node memory threshold (zero → default) = %v, want 60.0", got)
	}
	if got := adapter.actionabilityThreshold("disk", "vm"); got != 70.0 {
		t.Errorf("disk threshold = %v, want 70.0", got)
	}
	if got := adapter.actionabilityThreshold("storage", "storage"); got != 70.0 {
		t.Errorf("storage threshold = %v, want 70.0", got)
	}
}

func TestActionabilityThreshold_CustomThresholds(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{
		NodeCPUWatch:   65,
		NodeMemWatch:   70,
		GuestMemWatch:  72,
		GuestDiskWatch: 80,
		StorageWatch:   75,
	}

	adapter := newPatrolFindingCreatorAdapter(ps, models.StateSnapshot{})

	if got := adapter.actionabilityThreshold("cpu", "node"); got != 65.0 {
		t.Errorf("cpu threshold = %v, want 65.0", got)
	}
	if got := adapter.actionabilityThreshold("memory", "node"); got != 70.0 {
		t.Errorf("node memory threshold = %v, want 70.0", got)
	}
	if got := adapter.actionabilityThreshold("memory", "vm"); got != 72.0 {
		t.Errorf("guest memory threshold = %v, want 72.0", got)
	}
	if got := adapter.actionabilityThreshold("memory", "container"); got != 72.0 {
		t.Errorf("container memory threshold = %v, want 72.0 (guest)", got)
	}
	if got := adapter.actionabilityThreshold("disk", "vm"); got != 80.0 {
		t.Errorf("disk threshold = %v, want 80.0", got)
	}
	if got := adapter.actionabilityThreshold("storage", "storage"); got != 75.0 {
		t.Errorf("storage threshold = %v, want 75.0", got)
	}
}

func TestIsActionable_CustomThresholdsRespected(t *testing.T) {
	// Set a high CPU watch threshold (85%) - value at 60% should be rejected
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{NodeCPUWatch: 85}

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 0.60, Memory: models.Memory{Usage: 30}},
		},
	}
	adapter := newPatrolFindingCreatorAdapter(ps, state)

	finding := &Finding{
		Key:          "cpu-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceType: "node",
		Title:        "High CPU usage",
	}

	if adapter.isActionable(finding) {
		t.Fatal("finding at 60% CPU should be rejected with 85% threshold")
	}

	// Now set lower threshold (40%) - same 60% value should pass
	ps.thresholds = PatrolThresholds{NodeCPUWatch: 40}
	adapter2 := newPatrolFindingCreatorAdapter(ps, state)

	if !adapter2.isActionable(finding) {
		t.Fatal("finding at 60% CPU should be accepted with 40% threshold")
	}
}

func TestIsActionable_NodeMemoryUsesNodeThreshold(t *testing.T) {
	// A node at 78% memory should pass with NodeMemWatch=75 but would fail
	// with GuestMemWatch=80. This test verifies the correct threshold is selected.
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{
		NodeMemWatch:  75, // node threshold: 75%
		GuestMemWatch: 80, // guest threshold: 80%
	}

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 0.40, Memory: models.Memory{Used: 78, Total: 100}},
		},
	}
	adapter := newPatrolFindingCreatorAdapter(ps, state)

	nodeFinding := &Finding{
		Key:          "memory-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceType: "node",
		Title:        "High memory usage",
	}

	// 78% > NodeMemWatch (75%) → should be actionable
	if !adapter.isActionable(nodeFinding) {
		t.Fatal("node at 78% memory should pass with NodeMemWatch=75")
	}

	// Same memory level on a VM should be rejected (78% < GuestMemWatch 80%)
	vmState := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm-1", Name: "vm-1", CPU: 0.40, Memory: models.Memory{Usage: 78}},
		},
	}
	vmAdapter := newPatrolFindingCreatorAdapter(ps, vmState)

	vmFinding := &Finding{
		Key:          "memory-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-1",
		ResourceType: "vm",
		Title:        "High memory usage",
	}

	// 78% < GuestMemWatch (80%) → should be rejected
	if vmAdapter.isActionable(vmFinding) {
		t.Fatal("VM at 78% memory should be rejected with GuestMemWatch=80")
	}
}

func TestIsBaselineAnomaly_NoStore(t *testing.T) {
	// No baseline store → always returns false (safe fallback)
	ps := NewPatrolService(nil, nil)
	adapter := newPatrolFindingCreatorAdapter(ps, models.StateSnapshot{})

	if adapter.isBaselineAnomaly("node-1", "cpu", 45.0) {
		t.Fatal("expected false when no baseline store is set")
	}
}

func TestIsActionable_AnomalyBypass(t *testing.T) {
	// Set up a baseline store with learned baselines.
	// CPU at 45% is below the 50% default threshold, but is anomalous
	// relative to a baseline of mean=20, stddev=5 (z-score = 5).
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{} // defaults: CPU watch = 0 → fallback 50%

	bsCfg := DefaultBaselineConfig()
	bsCfg.MinSamples = 5 // Lower for test
	bs := NewBaselineStore(bsCfg)

	// Learn a baseline: mean ~20%, stddev ~2.5%
	points := make([]BaselineMetricPoint, 50)
	for i := 0; i < 50; i++ {
		points[i] = BaselineMetricPoint{Value: 18.0 + float64(i%5), Timestamp: time.Now().Add(-time.Duration(i) * time.Hour)}
	}
	_ = bs.Learn("node-1", "node", "cpu", points)
	ps.baselineStore = bs

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 0.45, Memory: models.Memory{Usage: 30}},
		},
	}
	adapter := newPatrolFindingCreatorAdapter(ps, state)

	finding := &Finding{
		Key:        "cpu-high",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "node-1",
		Title:      "High CPU usage",
	}

	// CPU at 45% is below 50% threshold BUT is anomalous (z-score ~5+)
	if !adapter.isActionable(finding) {
		t.Fatal("finding below threshold but anomalous should be allowed through")
	}
}

func TestIsActionable_BelowThresholdNoAnomaly(t *testing.T) {
	// CPU at 45% below 50% threshold, baseline mean is also ~45% → not anomalous → rejected.
	ps := NewPatrolService(nil, nil)
	ps.thresholds = PatrolThresholds{} // defaults

	bsCfg := DefaultBaselineConfig()
	bsCfg.MinSamples = 5
	bs := NewBaselineStore(bsCfg)

	// Learn a baseline: mean ~45%, close to current value
	points := make([]BaselineMetricPoint, 50)
	for i := 0; i < 50; i++ {
		points[i] = BaselineMetricPoint{Value: 43.0 + float64(i%5), Timestamp: time.Now().Add(-time.Duration(i) * time.Hour)}
	}
	_ = bs.Learn("node-1", "node", "cpu", points)
	ps.baselineStore = bs

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 0.45, Memory: models.Memory{Usage: 30}},
		},
	}
	adapter := newPatrolFindingCreatorAdapter(ps, state)

	finding := &Finding{
		Key:        "cpu-high",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "node-1",
		Title:      "High CPU usage",
	}

	if adapter.isActionable(finding) {
		t.Fatal("finding below threshold with no anomaly should be rejected")
	}
}

func TestIsActionable_EscapeHatchesPreserved(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", CPU: 0.10, Memory: models.Memory{Usage: 10}},
		},
	}
	adapter := newPatrolFindingCreatorAdapter(ps, state)

	// Critical severity always passes
	critical := &Finding{
		Key:        "cpu-high",
		Severity:   FindingSeverityCritical,
		Category:   FindingCategoryPerformance,
		ResourceID: "node-1",
		Title:      "Critical CPU",
	}
	if !adapter.isActionable(critical) {
		t.Fatal("critical finding should always pass")
	}

	// Backup category always passes
	backup := &Finding{
		Key:        "backup-missing",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryBackup,
		ResourceID: "node-1",
		Title:      "Missing backup",
	}
	if !adapter.isActionable(backup) {
		t.Fatal("backup finding should always pass")
	}

	// Reliability category always passes
	reliability := &Finding{
		Key:        "node-offline",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryReliability,
		ResourceID: "node-1",
		Title:      "Node offline",
	}
	if !adapter.isActionable(reliability) {
		t.Fatal("reliability finding should always pass")
	}
}

func TestPatrolService_GenerateRemediationSteps(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	cases := []struct {
		name     string
		category FindingCategory
		title    string
		wantLen  int
	}{
		{name: "performance-cpu", category: FindingCategoryPerformance, title: "High CPU usage", wantLen: 4},
		{name: "capacity-disk", category: FindingCategoryCapacity, title: "Disk space low", wantLen: 4},
		{name: "reliability-offline", category: FindingCategoryReliability, title: "Service offline", wantLen: 4},
		{name: "backup-failed", category: FindingCategoryBackup, title: "Backup failed", wantLen: 4},
		{name: "security", category: FindingCategorySecurity, title: "Vulnerability detected", wantLen: 4},
		{name: "general", category: FindingCategoryGeneral, title: "Config drift detected", wantLen: 4},
	}

	for _, c := range cases {
		finding := &Finding{
			ID:         "f-" + c.name,
			ResourceID: "res-1",
			Category:   c.category,
			Title:      c.title,
		}
		steps := ps.generateRemediationSteps(finding)
		if len(steps) != c.wantLen {
			t.Fatalf("%s: expected %d steps, got %d", c.name, c.wantLen, len(steps))
		}
	}

	unknown := &Finding{
		ID:         "f-unknown",
		ResourceID: "res-1",
		Category:   FindingCategory("mystery"),
		Title:      "Unknown issue",
	}
	steps := ps.generateRemediationSteps(unknown)
	if len(steps) != 3 {
		t.Fatalf("unknown category: expected 3 generic steps, got %d", len(steps))
	}
}

func TestPatrolService_GenerateRemediationPlan(t *testing.T) {
	engine := remediation.NewEngine(remediation.DefaultEngineConfig())
	ps := NewPatrolService(nil, nil)
	ps.SetRemediationEngine(engine)

	finding := &Finding{
		ID:             "finding-1",
		Key:            "service-restart",
		Severity:       FindingSeverityWarning,
		Category:       FindingCategoryReliability,
		ResourceID:     "vm-101",
		ResourceName:   "web",
		ResourceType:   "vm",
		Title:          "Unexpected restart detected",
		Description:    "Service restarted unexpectedly",
		Recommendation: "Investigate restart cause",
	}

	ps.generateRemediationPlan(finding)

	plan := engine.GetPlanForFinding(finding.ID)
	if plan == nil {
		t.Fatal("expected remediation plan to be created")
	}
	if plan.RiskLevel == "" {
		t.Fatal("expected risk level to be set on plan")
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected warnings to be added to plan")
	}
}

func TestPatrolFindingCreatorAdapter_ResolveFindingAndChecks(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	finding := &Finding{
		ID:           "resolve-1",
		Key:          "cpu-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		ResourceType: "node",
		Title:        "High CPU",
	}
	ps.findings.Add(finding)

	var resolvedID string
	ps.unifiedFindingResolver = func(id string) {
		resolvedID = id
	}

	adapter := newPatrolFindingCreatorAdapter(ps, models.StateSnapshot{})
	if err := adapter.ResolveFinding(finding.ID, "resolved in test"); err != nil {
		t.Fatalf("ResolveFinding failed: %v", err)
	}
	if resolvedID != finding.ID {
		t.Fatalf("expected unified resolver to be called with %s, got %s", finding.ID, resolvedID)
	}
	if len(adapter.getResolvedIDs()) != 1 {
		t.Fatalf("expected resolved IDs to be tracked")
	}

	if adapter.HasCheckedFindings() {
		t.Fatal("expected HasCheckedFindings to be false before GetActiveFindings")
	}
	_ = adapter.GetActiveFindings("", "warning")
	if !adapter.HasCheckedFindings() {
		t.Fatal("expected HasCheckedFindings to be true after GetActiveFindings")
	}
}
