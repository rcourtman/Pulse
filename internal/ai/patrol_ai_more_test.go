package ai

import (
	"context"
	"encoding/json"
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
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
		PBSInstances: []models.PBSInstance{
			{
				Name: "pbs-1",
				Datastores: []models.PBSDatastore{
					{Name: "store", Usage: 55.0, Used: 550 * 1024 * 1024, Total: 1000 * 1024 * 1024},
				},
			},
		},
	}

	usedBytes := int64(450 * 1024 * 1024)
	totalBytes := int64(1000 * 1024 * 1024)
	storeUsedBytes := int64(700 * 1024 * 1024)
	storeTotalBytes := int64(1000 * 1024 * 1024)
	ps.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{
		getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
			if t == unifiedresources.ResourceTypeStorage {
				return []unifiedresources.Resource{
					{
						ID:     "store-1",
						Name:   "store-1",
						Type:   unifiedresources.ResourceTypeStorage,
						Status: unifiedresources.StatusWarning,
						Storage: &unifiedresources.StorageMeta{
							Type:              "zfs",
							Shared:            true,
							IsZFS:             true,
							ZFSPoolState:      "DEGRADED",
							ZFSReadErrors:     1,
							ZFSWriteErrors:    2,
							ZFSChecksumErrors: 3,
						},
						Metrics: &unifiedresources.ResourceMetrics{
							Disk: &unifiedresources.MetricValue{
								Used:    &storeUsedBytes,
								Total:   &storeTotalBytes,
								Percent: 70.0,
							},
						},
					},
				}
			}
			if t == unifiedresources.ResourceTypeCeph {
				return []unifiedresources.Resource{
					{
						Name: "ceph-1",
						Type: unifiedresources.ResourceTypeCeph,
						Ceph: &unifiedresources.CephMeta{
							HealthStatus:  "HEALTH_WARN",
							HealthMessage: "OSD down",
							NumOSDs:       3,
							NumOSDsUp:     2,
							NumOSDsIn:     3,
						},
						Metrics: &unifiedresources.ResourceMetrics{
							Disk: &unifiedresources.MetricValue{
								Used:  &usedBytes,
								Total: &totalBytes,
							},
						},
					},
				}
			}
			return nil
		},
	})

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

	ps.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{
		getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
			if t == unifiedresources.ResourceTypePhysicalDisk {
				return []unifiedresources.Resource{
					{
						Name:       "disk",
						Type:       unifiedresources.ResourceTypePhysicalDisk,
						ParentName: "node-1",
						PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
							DevPath:     "/dev/sda",
							Model:       "disk",
							Health:      "PASSED",
							Wearout:     100,
							Temperature: 40,
						},
					},
				}
			}
			return nil
		},
	})

	state := models.StateSnapshot{
		ConnectionHealth: map[string]bool{
			"node-1": true,
			"node-2": true,
		},
	}

	out := ps.seedHealthAndAlerts(state, nil, cfg, now)
	if !strings.Contains(out, "All 1 disks healthy") {
		t.Fatalf("expected healthy disk summary, got: %s", out)
	}
	if !strings.Contains(out, "All 2 instances connected") {
		t.Fatalf("expected all connections summary, got: %s", out)
	}
}

func TestSeedHealthAndAlerts_WithIssues(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	now := time.Now()

	ps.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{
		getByTypeFunc: func(t unifiedresources.ResourceType) []unifiedresources.Resource {
			if t == unifiedresources.ResourceTypePhysicalDisk {
				return []unifiedresources.Resource{
					{
						Name:       "disk",
						Type:       unifiedresources.ResourceTypePhysicalDisk,
						ParentName: "node-1",
						PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
							DevPath:     "/dev/sda",
							Model:       "disk",
							Health:      "FAILED",
							Wearout:     10,
							Temperature: 60,
						},
					},
				}
			}
			return nil
		},
	})

	state := models.StateSnapshot{
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
