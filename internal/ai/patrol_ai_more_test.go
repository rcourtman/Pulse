package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type noExecutorChatService struct{}

func (n *noExecutorChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "noop"}, nil
}

func (n *noExecutorChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (n *noExecutorChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	return &PatrolStreamResponse{}, nil
}

func (n *noExecutorChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (n *noExecutorChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (n *noExecutorChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

type streamChatService struct {
	executor *tools.PulseToolExecutor
}

func (s *streamChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "stream"}, nil
}

func (s *streamChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (s *streamChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	contentData, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: "hello"})
	callback(ChatStreamEvent{Type: "content", Data: contentData})

	thinkingData, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: "thinking"})
	callback(ChatStreamEvent{Type: "thinking", Data: thinkingData})

	toolStartData, _ := json.Marshal(struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Input    string `json:"input"`
		RawInput string `json:"raw_input"`
	}{
		ID:       "",
		Name:     "pulse_query",
		Input:    `{"action":"health"}`,
		RawInput: `{"action":"health"}`,
	})
	callback(ChatStreamEvent{Type: "tool_start", Data: toolStartData})

	toolEndData, _ := json.Marshal(struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Input    string `json:"input"`
		RawInput string `json:"raw_input"`
		Output   string `json:"output"`
		Success  bool   `json:"success"`
	}{
		ID:       "",
		Name:     "pulse_query",
		Input:    `{"action":"health"}`,
		RawInput: `{"action":"health"}`,
		Output:   `{"status":"ok"}`,
		Success:  true,
	})
	callback(ChatStreamEvent{Type: "tool_end", Data: toolEndData})

	toolEndMissingData, _ := json.Marshal(struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Input    string `json:"input"`
		RawInput string `json:"raw_input"`
		Output   string `json:"output"`
		Success  bool   `json:"success"`
	}{
		ID:      "missing",
		Name:    "pulse_metrics",
		Output:  `{"status":"ok"}`,
		Success: true,
	})
	callback(ChatStreamEvent{Type: "tool_end", Data: toolEndMissingData})

	return &PatrolStreamResponse{
		Content:      "analysis complete",
		InputTokens:  5,
		OutputTokens: 7,
	}, nil
}

func (s *streamChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (s *streamChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (s *streamChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func (s *streamChatService) GetExecutor() *tools.PulseToolExecutor {
	return s.executor
}

func samplePatrolState() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "node-1",
				Status: "online",
				CPU:    0.15,
				Memory: models.Memory{Usage: 20.0},
				Disk:   models.Disk{Usage: 10.0},
			},
		},
	}
}

func TestComputePatrolMaxTurns(t *testing.T) {
	if got := computePatrolMaxTurns(0, nil); got != patrolMinTurns {
		t.Fatalf("expected min turns %d, got %d", patrolMinTurns, got)
	}

	if got := computePatrolMaxTurns(1000, nil); got != patrolMaxTurnsLimit {
		t.Fatalf("expected max turns %d, got %d", patrolMaxTurnsLimit, got)
	}

	quickScope := &PatrolScope{Depth: PatrolDepthQuick}
	if got := computePatrolMaxTurns(0, quickScope); got != patrolQuickMinTurns {
		t.Fatalf("expected quick min turns %d, got %d", patrolQuickMinTurns, got)
	}
	if got := computePatrolMaxTurns(200, quickScope); got != patrolQuickMaxTurns {
		t.Fatalf("expected quick max turns %d, got %d", patrolQuickMaxTurns, got)
	}
}

func TestEnsureInvestigationToolCall_NoOp(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ctx := context.Background()

	var mu sync.Mutex
	completed := []ToolCallRecord{{ToolName: "pulse_query"}}
	raw := []string{"existing"}

	ps.ensureInvestigationToolCall(ctx, &tools.PulseToolExecutor{}, &mu, &completed, &raw, true)
	if len(completed) != 1 || len(raw) != 1 {
		t.Fatalf("expected no changes when investigation tool already present")
	}

	ps.ensureInvestigationToolCall(ctx, nil, &mu, &completed, &raw, true)
	if len(completed) != 1 || len(raw) != 1 {
		t.Fatalf("expected no changes when executor is nil")
	}
}

func TestGetPatrolSystemPrompt_ModeSwitch(t *testing.T) {
	svc := &Service{cfg: &config.AIConfig{PatrolAutoFix: true}}
	ps := NewPatrolService(svc, nil)
	prompt := ps.getPatrolSystemPrompt()
	if !strings.Contains(prompt, "Auto-Fix Mode") || !strings.Contains(prompt, "pulse_control") {
		t.Fatalf("expected auto-fix prompt, got: %s", prompt)
	}

	svc.cfg = &config.AIConfig{PatrolAutoFix: false}
	prompt = ps.getPatrolSystemPrompt()
	if !strings.Contains(prompt, "Observe Only Mode") {
		t.Fatalf("expected observe-only prompt, got: %s", prompt)
	}
}

func TestRunAIAnalysis_EarlyErrors(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		_, err := ps.runAIAnalysis(context.Background(), models.StateSnapshot{}, nil)
		if err == nil {
			t.Fatal("expected error when aiService is nil")
		}
	})

	t.Run("budget exceeded", func(t *testing.T) {
		store := cost.NewStore(cost.DefaultMaxDays)
		store.Record(cost.UsageEvent{
			Provider:     "openai",
			RequestModel: "gpt-4o-mini",
			InputTokens:  1_000_000,
			OutputTokens: 0,
		})
		svc := &Service{
			cfg:       &config.AIConfig{CostBudgetUSD30d: 0.01},
			costStore: store,
		}
		ps := NewPatrolService(svc, nil)
		_, err := ps.runAIAnalysis(context.Background(), models.StateSnapshot{}, nil)
		if err == nil || !strings.Contains(err.Error(), "patrol skipped") {
			t.Fatalf("expected budget error, got %v", err)
		}
	})

	t.Run("chat service nil", func(t *testing.T) {
		svc := &Service{}
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "chat service not available") {
			t.Fatalf("expected chat service error, got %v", err)
		}
	})

	t.Run("executor accessor missing", func(t *testing.T) {
		svc := &Service{}
		svc.SetChatService(&noExecutorChatService{})
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "executor access") {
			t.Fatalf("expected executor access error, got %v", err)
		}
	})

	t.Run("executor nil", func(t *testing.T) {
		svc := &Service{}
		svc.SetChatService(&mockChatService{executor: nil})
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "tool executor not available") {
			t.Fatalf("expected executor nil error, got %v", err)
		}
	})
}

func TestRunAIAnalysis_RetriesWithReducedSeedContextOnProviderLimit(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	promptLengths := make([]int, 0, 2)
	mockCS := &mockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			promptLengths = append(promptLengths, len(req.Prompt))
			if len(promptLengths) == 1 {
				return &PatrolStreamResponse{}, fmt.Errorf("API error (400): {\"error\":{\"message\":\"request (9126 tokens) exceeds the available context size (8192 tokens)\",\"type\":\"exceed_context_size_error\",\"n_ctx\":8192}}")
			}
			return &PatrolStreamResponse{
				Content:      "analysis complete",
				InputTokens:  10,
				OutputTokens: 5,
			}, nil
		},
	}
	svc.SetChatService(mockCS)

	var guests []models.VM
	for i := 0; i < 600; i++ {
		guests = append(guests, models.VM{
			ID:     fmt.Sprintf("vm-%d", i),
			VMID:   100 + i,
			Name:   fmt.Sprintf("very-long-guest-name-%03d-%s", i, strings.Repeat("x", 120)),
			Node:   "pve-1",
			Status: "running",
			CPU:    0.10,
			Memory: models.Memory{Usage: 20.0},
			Disk:   models.Disk{Usage: 15.0},
		})
	}
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-1", Status: "online", CPU: 0.20, Memory: models.Memory{Usage: 30.0}},
		},
		VMs: guests,
	}

	ps := NewPatrolService(svc, &mockStateProvider{state: state})
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	result, err := ps.runAIAnalysis(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("runAIAnalysis returned error: %v", err)
	}
	if result == nil || result.Response != "analysis complete" {
		t.Fatalf("unexpected analysis result: %#v", result)
	}
	if len(promptLengths) != 2 {
		t.Fatalf("expected 2 patrol attempts, got %d", len(promptLengths))
	}
	if promptLengths[1] >= promptLengths[0] {
		t.Fatalf("expected retry prompt to be smaller, got %d then %d", promptLengths[0], promptLengths[1])
	}
}

func TestPatrolSeedRetryBudgets_UsesProviderContextWindow(t *testing.T) {
	err := fmt.Errorf("API error (400): {\"error\":{\"message\":\"request exceeds the available context size (4096 tokens)\",\"type\":\"exceed_context_size_error\",\"n_ctx\":4096}}")

	got := patrolSeedRetryBudgets(err)
	want := []int{8192, 4096, 2048}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patrolSeedRetryBudgets() = %v, want %v", got, want)
	}
}

func TestRunAIAnalysis_SkipsNoOpRetryBudgetOnSmallProviderWindow(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	svc.provider = &mockProvider{}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	promptLengths := make([]int, 0, 2)
	mockCS := &mockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			promptLengths = append(promptLengths, len(req.Prompt))
			if len(promptLengths) == 1 {
				return &PatrolStreamResponse{}, fmt.Errorf("API error (400): {\"error\":{\"message\":\"request (5000 tokens) exceeds the available context size (4096 tokens)\",\"type\":\"exceed_context_size_error\",\"n_ctx\":4096}}")
			}
			return &PatrolStreamResponse{
				Content:      "analysis complete",
				InputTokens:  8,
				OutputTokens: 4,
			}, nil
		},
	}
	svc.SetChatService(mockCS)

	var guests []models.VM
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve-1", Status: "online", CPU: 0.20, Memory: models.Memory{Usage: 30.0}},
		},
	}

	ps := NewPatrolService(svc, &mockStateProvider{})
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})

	for i := 0; i < 600; i++ {
		guests = append(guests, models.VM{
			ID:     fmt.Sprintf("vm-%d", i),
			VMID:   100 + i,
			Name:   fmt.Sprintf("guest-%03d-%s", i, strings.Repeat("y", 32)),
			Node:   "pve-1",
			Status: "running",
			CPU:    0.10,
			Memory: models.Memory{Usage: 20.0},
			Disk:   models.Disk{Usage: 15.0},
		})
		state.VMs = guests
		seed, _ := ps.buildSeedContext(state, nil, nil)
		if l := len(seed); l > 4096 && l < 8192 {
			break
		}
	}

	seed, _ := ps.buildSeedContext(state, nil, nil)
	if len(seed) <= 4096 || len(seed) >= 8192 {
		t.Fatalf("expected seed context between 4096 and 8192 chars, got %d", len(seed))
	}

	ps.stateProvider = &mockStateProvider{state: state}
	result, err := ps.runAIAnalysis(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("runAIAnalysis returned error: %v", err)
	}
	if result == nil || result.Response != "analysis complete" {
		t.Fatalf("unexpected analysis result: %#v", result)
	}
	if len(promptLengths) != 2 {
		t.Fatalf("expected 2 patrol attempts, got %d", len(promptLengths))
	}
	if promptLengths[1] >= promptLengths[0] {
		t.Fatalf("expected retry prompt to be smaller, got %d then %d", promptLengths[0], promptLengths[1])
	}
	if promptLengths[1] > 4096 {
		t.Fatalf("expected retry prompt to skip the 8192-char no-op budget, got %d", promptLengths[1])
	}
}

func TestSeedResourceInventory_QuietSummary(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "node-1",
				Status: "online",
				CPU:    0.10,
				Memory: models.Memory{Usage: 10.0},
				Disk:   models.Disk{Usage: 15.0},
			},
			{
				ID:     "node-2",
				Name:   "node-2",
				Status: "online",
				CPU:    0.20,
				Memory: models.Memory{Usage: 20.0},
				Disk:   models.Disk{Usage: 25.0},
			},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), true, nil)
	if !strings.Contains(out, "# Nodes: All 2") {
		t.Fatalf("expected quiet node summary, got: %s", out)
	}
}

func TestRunAIAnalysis_StreamEvents(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc := &Service{}
	svc.SetChatService(&streamChatService{executor: executor})

	ps := NewPatrolService(svc, nil)
	scope := &PatrolScope{NoStream: true}

	res, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
	if err != nil {
		t.Fatalf("runAIAnalysis failed: %v", err)
	}
	if res == nil || res.Response == "" {
		t.Fatalf("expected analysis response, got %+v", res)
	}
	if len(res.ToolCalls) == 0 {
		t.Fatalf("expected tool calls to be recorded")
	}
}

func TestSeedResourceInventory_DetailedSections(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:             "node-1",
				Name:           "node-1",
				Status:         "online",
				CPU:            0.55,
				Memory:         models.Memory{Usage: 65.0},
				Disk:           models.Disk{Usage: 40.0},
				LoadAverage:    []float64{1.2, 0.9, 0.7},
				Uptime:         int64(3600),
				PendingUpdates: 5,
			},
		},
		VMs: []models.VM{
			{
				ID:         "vm-1",
				VMID:       101,
				Name:       "vm-1",
				Node:       "node-1",
				Status:     "running",
				CPU:        0.10,
				Memory:     models.Memory{Usage: 30.0},
				Disk:       models.Disk{Usage: 20.0},
				LastBackup: now.Add(-2 * time.Hour),
			},
		},
		Containers: []models.Container{
			{
				ID:     "ct-1",
				VMID:   200,
				Name:   "ct-1",
				Node:   "node-1",
				Status: "stopped",
				CPU:    0.05,
				Memory: models.Memory{Usage: 15.0},
				Disk:   models.Disk{Usage: 10.0},
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-1",
				Hostname: "docker-1",
				Containers: []models.DockerContainer{
					{Name: "web", State: "running", Health: "unhealthy"},
					{Name: "db", State: "exited"},
				},
			},
		},
		Storage: []models.Storage{
			{
				ID:     "store-1",
				Name:   "store-1",
				Type:   "zfs",
				Usage:  70.0,
				Used:   700 * 1024 * 1024,
				Total:  1000 * 1024 * 1024,
				Shared: true,
				Active: true,
				ZFSPool: &models.ZFSPool{
					State:          "DEGRADED",
					ReadErrors:     1,
					WriteErrors:    2,
					ChecksumErrors: 3,
				},
			},
		},
		CephClusters: []models.CephCluster{
			{
				Name:          "ceph-1",
				Health:        "HEALTH_WARN",
				HealthMessage: "OSD down",
				UsagePercent:  45.0,
				UsedBytes:     450 * 1024 * 1024,
				TotalBytes:    1000 * 1024 * 1024,
				NumOSDs:       3,
				NumOSDsUp:     2,
				NumOSDsIn:     3,
			},
		},
		PBSInstances: []models.PBSInstance{
			{
				Name: "pbs-1",
				Datastores: []models.PBSDatastore{
					{Name: "store", Usage: 55.0, Used: 550 * 1024 * 1024, Total: 1000 * 1024 * 1024},
				},
			},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, now, false, nil)
	for _, part := range []string{
		"# Node Metrics",
		"# Guest Metrics",
		"# Docker",
		"health=unhealthy",
		"# Storage",
		"ZFS errors",
		"# Ceph",
		"Message: OSD down",
		"# PBS Datastores",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("expected output to contain %q, got: %s", part, out)
		}
	}
}

func TestSeedHealthAndAlerts_NoIssues(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()

	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				Node:        "node-1",
				DevPath:     "/dev/sda",
				Model:       "disk",
				Health:      "UNKNOWN",
				Wearout:     100,
				Temperature: 40,
			},
		},
		ConnectionHealth: map[string]bool{
			"node-1": true,
			"node-2": true,
		},
	}

	out := ps.seedHealthAndAlerts(state, nil, cfg, now)
	if !strings.Contains(out, "No disk issues detected across 1 disks; SMART health is unknown for 1 disk(s).") {
		t.Fatalf("expected unknown SMART evidence summary, got: %s", out)
	}
	if strings.Contains(out, "SMART PASSED") {
		t.Fatalf("unknown SMART health must not be reported as passed, got: %s", out)
	}
	if !strings.Contains(out, "All 2 instances connected") {
		t.Fatalf("expected all connections summary, got: %s", out)
	}
}

func TestSeedHealthAndAlerts_AllDisksPassed(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()

	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				Node:        "node-1",
				DevPath:     "/dev/sda",
				Model:       "disk",
				Health:      "PASSED",
				Wearout:     100,
				Temperature: 40,
			},
		},
	}

	out := ps.seedHealthAndAlerts(state, nil, cfg, now)
	if !strings.Contains(out, "All 1 disks healthy (SMART PASSED/OK).") {
		t.Fatalf("expected passed SMART summary, got: %s", out)
	}
}

func TestSeedHealthAndAlerts_SMARTAttributeIssue(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()
	pending := int64(2)

	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				Node:        "node-1",
				DevPath:     "/dev/sda",
				Model:       "disk",
				Health:      "PASSED",
				Wearout:     100,
				Temperature: 40,
				SmartAttributes: &models.SMARTAttributes{
					PendingSectors: &pending,
				},
			},
		},
	}

	out := ps.seedHealthAndAlerts(state, nil, cfg, now)
	if strings.Contains(out, "All 1 disks healthy") {
		t.Fatalf("SMART counter evidence must not be summarized as all healthy, got: %s", out)
	}
	if !strings.Contains(out, "SMART Evidence") || !strings.Contains(out, "pending sectors=2") {
		t.Fatalf("expected SMART counter evidence in disk health table, got: %s", out)
	}
}

func TestSeedHealthAndAlerts_WithIssues(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()

	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				Node:        "node-1",
				DevPath:     "/dev/sda",
				Model:       "disk",
				Health:      "FAILED",
				Wearout:     10,
				Temperature: 60,
			},
		},
		ActiveAlerts: []models.Alert{
			{Level: "warning", Message: "CPU high", StartTime: now.Add(-time.Hour)},
		},
		RecentlyResolved: []models.ResolvedAlert{
			{Alert: models.Alert{Message: "Disk alert"}, ResolvedTime: now.Add(-2 * time.Hour)},
		},
		ConnectionHealth: map[string]bool{
			"node-1": true,
			"node-2": false,
		},
		KubernetesClusters: []models.KubernetesCluster{
			{Name: "k1", Nodes: []models.KubernetesNode{{Name: "n1"}}},
		},
		Hosts: []models.Host{
			{ID: "host-1", Hostname: "host-1"},
		},
	}

	out := ps.seedHealthAndAlerts(state, nil, cfg, now)
	for _, part := range []string{
		"# Disk Health",
		"/dev/sda",
		"# Active Alerts",
		"# Recently Resolved Alerts",
		"# Connections",
		"Disconnected: node-2",
		"# Kubernetes Clusters",
		"# Hosts",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("expected output to contain %q, got: %s", part, out)
		}
	}
}

func TestSeedIntelligenceContext_EmptyAnomalies(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	out := ps.seedIntelligenceContext(seedIntelligence{hasBaselineStore: true}, time.Now())
	if !strings.Contains(out, "No anomalies detected") {
		t.Fatalf("expected no anomalies message, got: %s", out)
	}
}

func TestSeedIntelligenceContext_WithData(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	now := time.Now()

	intel := seedIntelligence{
		hasBaselineStore: true,
		anomalies: []baseline.AnomalyReport{
			{
				ResourceID:   "node-1",
				ResourceName: "node-1",
				Metric:       "cpu",
				CurrentValue: 0.9,
				BaselineMean: 0.4,
				ZScore:       2.5,
				Severity:     baseline.AnomalyHigh,
			},
		},
		forecasts: []seedForecast{
			{name: "node-1", metric: "memory", severity: "warning", daysToFull: 10, dailyChange: 2.5, current: 75.0},
		},
		predictions: []FailurePrediction{
			{ResourceID: "node-1", EventType: "restart", DaysUntil: 3, Confidence: 0.9, Basis: "pattern"},
		},
		recentChanges: []memory.Change{
			{ResourceID: "vm-1", ResourceType: "vm", Description: "config changed", DetectedAt: now.Add(-time.Hour)},
		},
		correlations: []*Correlation{
			{SourceID: "node-1", TargetID: "vm-1", EventPattern: "cpu_high -> restart", AvgDelay: 10 * time.Minute, Confidence: 0.8, Occurrences: 4},
		},
	}

	out := ps.seedIntelligenceContext(intel, now)
	for _, part := range []string{
		"# Anomalies",
		"# Capacity Forecasts",
		"# Failure Predictions",
		"# Recent Infrastructure Changes",
		"# Known Resource Correlations",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("expected output to contain %q, got: %s", part, out)
		}
	}
}
