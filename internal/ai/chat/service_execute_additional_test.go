package chat

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubServiceProvider struct {
	streamFn func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

type recordingAgentServer struct {
	calls atomic.Int32
}

func (s *recordingAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return []agentexec.ConnectedAgent{{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}}
}

func (s *recordingAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	s.calls.Add(1)
	return &agentexec.CommandResultPayload{
		RequestID: cmd.RequestID,
		Success:   true,
		Stdout:    "executed",
		ExitCode:  0,
	}, nil
}

type handoffUnifiedProvider struct {
	resources map[unifiedresources.ResourceType][]unifiedresources.Resource
}

func (p handoffUnifiedProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	return append([]unifiedresources.Resource(nil), p.resources[t]...)
}

func installTestApprovalStore(t *testing.T, req *approval.ApprovalRequest) {
	t.Helper()
	previous := approval.GetStore()
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DefaultTimeout:     10 * time.Minute,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("failed to create approval store: %v", err)
	}
	approval.SetStore(store)
	t.Cleanup(func() {
		approval.SetStore(previous)
	})
	if req != nil {
		if err := store.CreateApproval(req); err != nil {
			t.Fatalf("failed to create approval: %v", err)
		}
	}
}

func testHandoffActionPlan(now time.Time) unifiedresources.ActionPlan {
	return unifiedresources.ActionPlan{
		ActionID:          "act-handoff-123",
		RequestID:         "approval-123",
		Allowed:           true,
		RequiresApproval:  true,
		ApprovalPolicy:    unifiedresources.ApprovalAdmin,
		RollbackAvailable: true,
		Message:           "Plan created for workload restart. Execution requires approval.",
		PlannedAt:         now,
		ExpiresAt:         now.Add(10 * time.Minute),
		ResourceVersion:   "resource:sha256:handoff",
		PolicyVersion:     "policy:sha256:handoff",
		PlanHash:          "sha256:handoff",
		Preflight: &unifiedresources.ActionPreflight{
			Target:          "vm-100",
			CurrentState:    "web-server is degraded",
			IntendedChange:  "Restart the workload service",
			DryRunAvailable: false,
			DryRunSummary:   "No provider-supported dry run is available for this action.",
			SafetyChecks: []string{
				"Approval is scoped to this organization.",
			},
			VerificationSteps: []string{
				"Confirm workload health after restart.",
			},
			GeneratedAt: now,
		},
	}
}

func seedHandoffActionAudit(t *testing.T, store unifiedresources.ResourceStore, plan unifiedresources.ActionPlan, state unifiedresources.ActionState, result *unifiedresources.ExecutionResult) {
	t.Helper()
	if store == nil {
		t.Fatal("action audit store is nil")
	}
	updatedAt := plan.PlannedAt.Add(time.Minute)
	record := unifiedresources.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: plan.PlannedAt,
		UpdatedAt: updatedAt,
		State:     state,
		Request: unifiedresources.ActionRequest{
			RequestID:      plan.RequestID,
			ResourceID:     "vm-100",
			CapabilityName: "restart",
			Reason:         "Recover after confirmed workload saturation",
			RequestedBy:    "pulse_patrol",
		},
		Plan:   plan,
		Result: result,
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit failed: %v", err)
	}
}

func (s *stubServiceProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (s *stubServiceProvider) TestConnection(ctx context.Context) error { return nil }

func (s *stubServiceProvider) Name() string { return "stub" }

func (s *stubServiceProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (s *stubServiceProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if s.streamFn != nil {
		return s.streamFn(ctx, req, callback)
	}
	callback(providers.StreamEvent{
		Type: "content",
		Data: providers.ContentEvent{Text: "hello"},
	})
	callback(providers.StreamEvent{
		Type: "done",
		Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
	})
	return nil
}

func (s *stubServiceProvider) SupportsThinking(model string) bool { return false }

func TestService_ExecuteStream_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: loop,
		provider:    provider, // Required: per-request loops need a provider to create new instances
		started:     true,
	}

	var doneCount int
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
	}

	req := ExecuteRequest{
		SessionID: "sess-1",
		Prompt:    "hello",
	}
	if err := svc.ExecuteStream(context.Background(), req, callback); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if doneCount != 1 {
		t.Fatalf("expected done callback once, got %d", doneCount)
	}

	messages, err := store.GetMessages("sess-1")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestService_ExecuteStream_RequestAutonomousOverrideClampsToolExecutor(t *testing.T) {
	installTestApprovalStore(t, nil)

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	store.GetSessionFSM("sess-request-override").State = StateReading

	agentServer := &recordingAgentServer{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		Policy:       agentexec.DefaultPolicy(),
		AgentServer:  agentServer,
		ControlLevel: tools.ControlLevelAutonomous,
	})
	executor.SetContext("agent", "agent-1", true)

	providerCallCount := 0
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			providerCallCount++
			if providerCallCount == 1 {
				callback(providers.StreamEvent{
					Type: "tool_start",
					Data: providers.ToolStartEvent{ID: "call-df", Name: "pulse_control"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						ToolCalls: []providers.ToolCall{{
							ID:   "call-df",
							Name: "pulse_control",
							Input: map[string]interface{}{
								"type":    "command",
								"command": "df -h",
							},
						}},
					},
				})
				return nil
			}
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "done"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	svc := &Service{
		cfg: &config.AIConfig{
			ChatModel:    "openai:test",
			ControlLevel: config.ControlLevelAutonomous,
		},
		sessions:       store,
		executor:       executor,
		provider:       provider,
		started:        true,
		autonomousMode: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	approvalRequiredMode := false
	approvalSeen := false
	err = svc.ExecuteStream(ctx, ExecuteRequest{
		SessionID:      "sess-request-override",
		Prompt:         "check disk space",
		AutonomousMode: &approvalRequiredMode,
		MaxTurns:       2,
	}, func(event StreamEvent) {
		if event.Type == "approval_needed" {
			approvalSeen = true
			cancel()
		}
	})

	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected request to stop while waiting for approval, got %v", err)
	}
	if !approvalSeen {
		t.Fatal("expected approval_needed event from request-local approval mode")
	}
	if got := agentServer.calls.Load(); got != 0 {
		t.Fatalf("expected command not to execute before approval, got %d executions", got)
	}
}

func TestService_ExecuteStream_HandoffContextIsModelOnly(t *testing.T) {
	actionNow := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	actionPlan := testHandoffActionPlan(actionNow)
	installTestApprovalStore(t, &approval.ApprovalRequest{
		ID:         "approval-123",
		OrgID:      approval.DefaultOrgID,
		Command:    "systemctl restart workload.service",
		TargetType: "vm",
		TargetID:   "vm-100",
		TargetName: "web-server",
		RiskLevel:  approval.RiskMedium,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
		Plan:       &actionPlan,
	})

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	timelineStore := unifiedresources.NewMemoryStore()
	seedHandoffActionAudit(t, timelineStore, actionPlan, unifiedresources.ActionStatePending, nil)
	if err := timelineStore.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-123",
		ResourceID:    "vm-100",
		ObservedAt:    time.Now().Add(-10 * time.Minute),
		Kind:          unifiedresources.ChangeStateTransition,
		From:          "running",
		To:            "degraded",
		SourceType:    unifiedresources.SourcePulseDiff,
		SourceAdapter: unifiedresources.AdapterProxmox,
		Confidence:    unifiedresources.ConfidenceHigh,
		Reason:        "CPU saturation detected after backup job",
	}); err != nil {
		t.Fatalf("RecordChange failed: %v", err)
	}

	unifiedProvider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeVM: {{
			ID:        "vm-100",
			Type:      unifiedresources.ResourceTypeVM,
			Name:      "web-server",
			Status:    unifiedresources.StatusWarning,
			LastSeen:  actionNow.Add(-5 * time.Minute),
			UpdatedAt: actionNow.Add(-2 * time.Minute),
			Sources:   []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent},
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceAgent:   {Status: "online", LastSeen: actionNow.Add(-5 * time.Minute)},
				unifiedresources.SourceProxmox: {Status: "stale", LastSeen: actionNow.Add(-20 * time.Minute), Error: "raw provider endpoint unavailable"},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				CPU:    &unifiedresources.MetricValue{Percent: 91.4},
				Memory: &unifiedresources.MetricValue{Percent: 88},
			},
			ParentName:      "pve-1",
			ChildCount:      1,
			IncidentCount:   1,
			IncidentSummary: "CPU saturation after backup job",
			Capabilities: []unifiedresources.ResourceCapability{{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the workload service",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
				InternalHandler:      "internal-restart-handler",
			}},
		}},
	}}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		started:                 true,
		actionAuditStore:        timelineStore,
		unifiedResourceProvider: unifiedProvider,
	}

	req := ExecuteRequest{
		SessionID:      "sess-handoff",
		Prompt:         "What happened?",
		FindingID:      "finding-123",
		HandoffContext: "[Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup.",
		HandoffResources: []HandoffResource{{
			ID:   "vm-100",
			Name: "web-server",
			Type: "vm",
			Node: "pve-1",
		}},
		HandoffActions: []HandoffAction{{
			FindingID:          "finding-123",
			RecordID:           "record-123",
			ApprovalID:         "approval-123",
			FixID:              "fix-123",
			Description:        "Restart the workload service",
			RiskLevel:          "medium",
			TargetHost:         "pve-1",
			TargetResourceID:   "vm-100",
			TargetResourceName: "web-server",
			TargetResourceType: "vm",
			TargetNode:         "pve-1",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "What happened?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "[Finding Context]") {
		t.Fatalf("stored user message should not include handoff context: %q", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "[Action Context]") {
		t.Fatalf("stored user message should not include action context: %q", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "Recent Changes Across Infrastructure") {
		t.Fatalf("stored user message should not include timeline context: %q", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "Resource State Context") {
		t.Fatalf("stored user message should not include resource state context: %q", stored[0].Content)
	}
	storedFindingID, err := store.GetModelHandoffFindingID("sess-handoff")
	if err != nil {
		t.Fatalf("GetModelHandoffFindingID failed: %v", err)
	}
	if storedFindingID != "finding-123" {
		t.Fatalf("stored handoff finding ID = %q, want finding-123", storedFindingID)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	if !strings.Contains(modelUserContent, "[Finding Context]") {
		t.Fatalf("model user content missing handoff context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "User message: What happened?") {
		t.Fatalf("model user content missing clean user message: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "[Action Context]") {
		t.Fatalf("model user content missing action context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Approval ID: approval-123") {
		t.Fatalf("model user content missing approval reference: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Approval Status: pending") {
		t.Fatalf("model user content missing approval status: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "[Resource State Context]") {
		t.Fatalf("model user content missing resource state context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Status: warning") {
		t.Fatalf("model user content missing current resource status: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Source Health: agent: online") {
		t.Fatalf("model user content missing canonical source health: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Metrics: cpu 91.4%, memory 88%") {
		t.Fatalf("model user content missing canonical metric summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Incident Summary: count 1") {
		t.Fatalf("model user content missing resource incident summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Capabilities: restart (common; approval admin)") {
		t.Fatalf("model user content missing governed capability summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Resource State Boundary: Current resource state, source health, incidents, metrics, and capabilities are read-only canonical infrastructure context") {
		t.Fatalf("model user content missing resource state boundary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action ID: act-handoff-123") {
		t.Fatalf("model user content missing canonical action id: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action State: pending_approval") {
		t.Fatalf("model user content missing canonical action state: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Requested By: pulse_patrol") {
		t.Fatalf("model user content missing action requester: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Capability: restart") {
		t.Fatalf("model user content missing action capability: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Approval Policy: admin") {
		t.Fatalf("model user content missing action approval policy: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Requires Approval: true") {
		t.Fatalf("model user content missing action approval requirement: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Plan Expires At: 2026-05-06T12:10:00Z") {
		t.Fatalf("model user content missing action plan expiry: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Preflight: Restart the workload service") {
		t.Fatalf("model user content missing action preflight: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Action Reference Action Dry Run Summary: No provider-supported dry run is available for this action.") {
		t.Fatalf("model user content missing action dry-run summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "### Recent Changes Across Infrastructure") {
		t.Fatalf("model user content missing timeline context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "redacted by policy: **State transition** running") {
		t.Fatalf("model user content missing canonical timeline summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Timeline Boundary: Recent changes are read-only canonical resource timeline context") {
		t.Fatalf("model user content missing timeline boundary: %q", modelUserContent)
	}
	if strings.Contains(modelUserContent, "systemctl restart workload.service") {
		t.Fatalf("model user content must not infer raw command text: %q", modelUserContent)
	}
	if strings.Contains(modelUserContent, "internal-restart-handler") || strings.Contains(modelUserContent, "raw provider endpoint unavailable") {
		t.Fatalf("model user content leaked raw provider execution detail: %q", modelUserContent)
	}
}

func TestService_ExecuteStream_HandoffResourcePolicyContextIsModelOnly(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	state := models.StateSnapshot{
		VMs: []models.VM{{
			ID:          "vm:node1:104",
			VMID:        104,
			Name:        "finance-vm",
			Node:        "node1",
			Status:      "running",
			IPAddresses: []string{"10.0.0.40"},
			Tags:        []string{"pii"},
		}},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state)
	vmResources := registry.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one canonical VM resource, got %d", len(vmResources))
	}
	vmResource := vmResources[0]
	unifiedProvider := unifiedresources.NewUnifiedAIAdapter(registry)

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID:      "sess-handoff-policy",
		Prompt:         "What happened?",
		HandoffContext: "[Finding Context]\nID: finding-123\nConclusion: finance-vm is overloaded.",
		HandoffResources: []HandoffResource{{
			ID:   vmResource.ID,
			Name: "finance-vm",
			Type: "vm",
			Node: "node1",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff-policy")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "What happened?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "Resource Policy Context") {
		t.Fatalf("stored user message should not include resource policy context: %q", stored[0].Content)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	if !strings.Contains(modelUserContent, "[Resource Policy Context]") {
		t.Fatalf("model user content missing resource policy context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Policy: sensitivity=Restricted, routing=Local Only") {
		t.Fatalf("model user content missing canonical policy summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Redactions: Hostname, IP Address, Platform ID, Alias") {
		t.Fatalf("model user content missing canonical redaction summary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Policy Boundary: Resource policy is read-only data-handling context") {
		t.Fatalf("model user content missing policy boundary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "redacted by policy") {
		t.Fatalf("external provider request should redact raw governed resource identity: %q", modelUserContent)
	}
	if strings.Contains(modelUserContent, "finance-vm") || strings.Contains(modelUserContent, "10.0.0.40") {
		t.Fatalf("model user content leaked governed resource identity: %q", modelUserContent)
	}
}

func TestService_ExecuteStream_OperatorBriefingHandoffHonorsResourcePolicy(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	unifiedProvider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeVM: {{
			ID:     "vm-100",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "finance-vm",
			Status: unifiedresources.StatusWarning,
			Tags:   []string{"pii"},
			Canonical: &unifiedresources.CanonicalIdentity{
				DisplayName: "finance-vm",
				Hostname:    "finance-vm",
				PlatformID:  "vm-100",
				Aliases:     []string{"finance-payroll"},
			},
			Identity: unifiedresources.ResourceIdentity{
				Hostnames:   []string{"finance-vm"},
				IPAddresses: []string{"10.0.0.40"},
			},
			Proxmox: &unifiedresources.ProxmoxData{
				NodeName: "pve-secret",
			},
		}},
	}}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "ollama:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID: "sess-operator-briefing-policy",
		Prompt:    "What happened?",
		HandoffContext: strings.Join([]string{
			"[Operator Briefing]",
			"Briefing Source: Pulse Patrol structured finding",
			"Resource: finance-vm (vm) [vm-100] on pve-secret",
			"Current Conclusion: finance-vm on pve-secret saturated CPU during backup.",
			"Recommended Next Step: Review finance-payroll after backup completion.",
			"Operator Boundary: Treat Patrol data as product context for explanation and review.",
		}, "\n"),
		HandoffResources: []HandoffResource{{
			ID:   "vm-100",
			Name: "finance-vm",
			Type: "vm",
			Node: "pve-secret",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-operator-briefing-policy")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "What happened?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "[Operator Briefing]") {
		t.Fatalf("stored user message should not include operator briefing: %q", stored[0].Content)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	for _, expected := range []string{
		"[Operator Briefing]",
		"Resource: redacted by policy (vm) [redacted by policy] on redacted by policy",
		"Current Conclusion: redacted by policy on redacted by policy saturated CPU during backup.",
		"Recommended Next Step: Review redacted by policy after backup completion.",
		"[Resource Policy Context]",
		"Resource Policy: virtual machine resource; status warning; local-only context",
		"Policy Boundary: Resource policy is read-only data-handling context",
		"User message: What happened?",
	} {
		if !strings.Contains(modelUserContent, expected) {
			t.Fatalf("model user content missing %q: %q", expected, modelUserContent)
		}
	}
	for _, forbidden := range []string{"finance-vm", "vm-100", "pve-secret", "finance-payroll", "10.0.0.40"} {
		if strings.Contains(modelUserContent, forbidden) {
			t.Fatalf("model user content leaked governed resource identity %q: %q", forbidden, modelUserContent)
		}
	}
}

func TestService_ExecuteStream_HandoffResourceRelationshipContextIsModelOnly(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	now := time.Now()
	unifiedProvider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeVM: {{
			ID:     "finance-vm",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "finance-vm",
			Status: unifiedresources.StatusOnline,
			Tags:   []string{"pii"},
			Relationships: []unifiedresources.ResourceRelationship{{
				SourceID:   "finance-vm",
				TargetID:   "secret-storage",
				Type:       unifiedresources.RelDependsOn,
				Confidence: 0.85,
				Active:     true,
				Discoverer: "pulse_correlation",
				ObservedAt: now.Add(-30 * time.Minute),
				LastSeenAt: now.Add(-10 * time.Minute),
				Metadata:   map[string]any{"role": "database"},
			}},
		}},
		unifiedresources.ResourceTypeStorage: {{
			ID:     "secret-storage",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "secret-storage",
			Status: unifiedresources.StatusOnline,
			Tags:   []string{"backup"},
		}},
	}}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID:      "sess-handoff-relationship",
		Prompt:         "Why did this happen?",
		HandoffContext: "[Finding Context]\nID: finding-123\nConclusion: finance-vm has storage latency.",
		HandoffResources: []HandoffResource{{
			ID:   "finance-vm",
			Name: "finance-vm",
			Type: "vm",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff-relationship")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "Why did this happen?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "Resource Relationship Context") {
		t.Fatalf("stored user message should not include relationship context: %q", stored[0].Content)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	if !strings.Contains(modelUserContent, "[Resource Relationship Context]") {
		t.Fatalf("model user content missing relationship context: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "### Resource Relationships") {
		t.Fatalf("model user content missing canonical relationship heading: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Depends on") {
		t.Fatalf("model user content missing canonical relationship label: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "discoverer pulse_correlation") {
		t.Fatalf("model user content missing relationship provenance: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "metadata present") {
		t.Fatalf("model user content missing relationship metadata marker: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Relationship Boundary: Relationships are read-only canonical topology context") {
		t.Fatalf("model user content missing relationship boundary: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "redacted by policy") {
		t.Fatalf("external provider request should redact governed relationship identity: %q", modelUserContent)
	}
	if strings.Contains(modelUserContent, "finance-vm") || strings.Contains(modelUserContent, "secret-storage") {
		t.Fatalf("model user content leaked governed relationship identity: %q", modelUserContent)
	}
}

func TestService_ExecuteStream_ReusesModelHandoffContextAcrossFollowUps(t *testing.T) {
	actionNow := time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC)
	actionPlan := testHandoffActionPlan(actionNow)
	installTestApprovalStore(t, &approval.ApprovalRequest{
		ID:         "approval-123",
		OrgID:      approval.DefaultOrgID,
		Command:    "systemctl restart workload.service",
		TargetType: "vm",
		TargetID:   "vm-100",
		TargetName: "web-server",
		RiskLevel:  approval.RiskMedium,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
		Plan:       &actionPlan,
	})

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	timelineStore := unifiedresources.NewMemoryStore()
	seedHandoffActionAudit(t, timelineStore, actionPlan, unifiedresources.ActionStatePending, nil)
	if err := timelineStore.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-followup",
		ResourceID:    "vm-100",
		ObservedAt:    time.Now().Add(-5 * time.Minute),
		Kind:          unifiedresources.ChangeConfigUpdate,
		From:          "backup-window",
		To:            "backup-window+cpu-spike",
		SourceType:    unifiedresources.SourcePulseDiff,
		SourceAdapter: unifiedresources.AdapterProxmox,
		Confidence:    unifiedresources.ConfidenceHigh,
		Reason:        "backup job changed workload pressure",
	}); err != nil {
		t.Fatalf("RecordChange failed: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	var capturedRequests [][]providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedRequests = append(capturedRequests, append([]providers.Message(nil), req.Messages...))
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "noted"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:              &config.AIConfig{ChatModel: "openai:test"},
		sessions:         store,
		executor:         executor,
		agenticLoop:      loop,
		provider:         provider,
		started:          true,
		actionAuditStore: timelineStore,
	}

	handoffContext := "[Finding Context]\nID: finding-123\nConclusion: CPU saturated after backup."
	handoffActions := []HandoffAction{{
		FindingID:          "finding-123",
		RecordID:           "record-123",
		ApprovalID:         "approval-123",
		FixID:              "fix-123",
		Description:        "Restart the workload service",
		RiskLevel:          "medium",
		TargetHost:         "pve-1",
		TargetResourceID:   "vm-100",
		TargetResourceName: "web-server",
		TargetResourceType: "vm",
		TargetNode:         "pve-1",
	}}
	firstReq := ExecuteRequest{
		SessionID:      "sess-handoff-followup",
		Prompt:         "What happened?",
		HandoffContext: handoffContext,
		HandoffResources: []HandoffResource{{
			ID:   "vm-100",
			Name: "web-server",
			Type: "vm",
			Node: "pve-1",
		}},
		HandoffActions: handoffActions,
	}
	if err := svc.ExecuteStream(context.Background(), firstReq, func(StreamEvent) {}); err != nil {
		t.Fatalf("first ExecuteStream failed: %v", err)
	}

	secondReq := ExecuteRequest{
		SessionID: "sess-handoff-followup",
		Prompt:    "What should I do next?",
	}
	if err := svc.ExecuteStream(context.Background(), secondReq, func(StreamEvent) {}); err != nil {
		t.Fatalf("second ExecuteStream failed: %v", err)
	}

	stored, err := store.GetMessages("sess-handoff-followup")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	var storedUserMessages []string
	for _, msg := range stored {
		if msg.Role == "user" {
			storedUserMessages = append(storedUserMessages, msg.Content)
		}
	}
	if len(storedUserMessages) != 2 {
		t.Fatalf("stored user messages = %d, want 2", len(storedUserMessages))
	}
	for _, content := range storedUserMessages {
		if strings.Contains(content, "[Finding Context]") {
			t.Fatalf("stored user message should not include handoff context: %q", content)
		}
		if strings.Contains(content, "[Action Context]") {
			t.Fatalf("stored user message should not include action context: %q", content)
		}
		if strings.Contains(content, "Recent Changes Across Infrastructure") {
			t.Fatalf("stored user message should not include timeline context: %q", content)
		}
	}
	if storedUserMessages[0] != "What happened?" || storedUserMessages[1] != "What should I do next?" {
		t.Fatalf("stored user messages = %#v, want clean prompts", storedUserMessages)
	}

	if len(capturedRequests) != 2 {
		t.Fatalf("provider request count = %d, want 2", len(capturedRequests))
	}
	firstModelUserContent := latestProviderUserContent(t, capturedRequests[0])
	if !strings.Contains(firstModelUserContent, "[Finding Context]") {
		t.Fatalf("first provider turn missing handoff context: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "User message: What happened?") {
		t.Fatalf("first provider turn missing clean user message: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "[Action Context]") {
		t.Fatalf("first provider turn missing action context: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "Action Reference Approval ID: approval-123") {
		t.Fatalf("first provider turn missing approval reference: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "Action Reference Approval Status: pending") {
		t.Fatalf("first provider turn missing approval status: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "Action Reference Action ID: act-handoff-123") {
		t.Fatalf("first provider turn missing action id: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "Action Reference Action State: pending_approval") {
		t.Fatalf("first provider turn missing action state: %q", firstModelUserContent)
	}
	if !strings.Contains(firstModelUserContent, "### Recent Changes Across Infrastructure") {
		t.Fatalf("first provider turn missing timeline context: %q", firstModelUserContent)
	}
	secondModelUserContent := latestProviderUserContent(t, capturedRequests[1])
	if !strings.Contains(secondModelUserContent, "[Finding Context]") {
		t.Fatalf("follow-up provider turn missing stored handoff context: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "User message: What should I do next?") {
		t.Fatalf("follow-up provider turn missing clean user message: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "[Action Context]") {
		t.Fatalf("follow-up provider turn missing stored action context: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "Action Reference Approval ID: approval-123") {
		t.Fatalf("follow-up provider turn missing stored approval reference: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "Action Reference Approval Status: pending") {
		t.Fatalf("follow-up provider turn missing refreshed approval status: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "Action Reference Action ID: act-handoff-123") {
		t.Fatalf("follow-up provider turn missing refreshed action id: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "Action Reference Action State: pending_approval") {
		t.Fatalf("follow-up provider turn missing refreshed action state: %q", secondModelUserContent)
	}
	if !strings.Contains(secondModelUserContent, "### Recent Changes Across Infrastructure") {
		t.Fatalf("follow-up provider turn missing stored resource timeline context: %q", secondModelUserContent)
	}
	if strings.Contains(firstModelUserContent+secondModelUserContent, "systemctl restart workload.service") {
		t.Fatalf("provider turns must not include raw command text")
	}
}

func TestBuildHandoffResourceStateContextUsesCanonicalResourceState(t *testing.T) {
	now := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	parentID := "cluster:production"
	provider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeAgent: {{
			ID:        "agent:pve-1",
			Type:      unifiedresources.ResourceTypeAgent,
			Name:      "pve-1",
			Status:    unifiedresources.StatusWarning,
			LastSeen:  now.Add(-3 * time.Minute),
			UpdatedAt: now.Add(-1 * time.Minute),
			Sources:   []unifiedresources.DataSource{unifiedresources.SourceAgent, unifiedresources.SourceProxmox},
			SourceStatus: map[unifiedresources.DataSource]unifiedresources.SourceStatus{
				unifiedresources.SourceAgent:   {Status: "online", LastSeen: now.Add(-3 * time.Minute)},
				unifiedresources.SourceProxmox: {Status: "stale", LastSeen: now.Add(-15 * time.Minute), Error: "ssh://root@example.invalid/private"},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				CPU:    &unifiedresources.MetricValue{Percent: 92.5},
				Memory: &unifiedresources.MetricValue{Percent: 81},
				Disk:   &unifiedresources.MetricValue{Percent: 71.2},
			},
			ParentID:        &parentID,
			ParentName:      "production-cluster",
			ChildCount:      2,
			IncidentCount:   1,
			IncidentSummary: "pve-1 is saturated after backup traffic",
			Incidents: []unifiedresources.ResourceIncident{{
				NativeID: "native-secret-incident",
				Code:     "cpu_saturation",
				Summary:  "CPU saturation remains elevated",
			}},
			Capabilities: []unifiedresources.ResourceCapability{{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the workload",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
				InternalHandler:      "internal-restart-handler",
				Params: []unifiedresources.CapabilityParam{{
					Name:         "token",
					Type:         "string",
					IsSensitive:  true,
					DefaultValue: "secret-token",
				}},
			}},
		}},
	}}

	contextText := buildHandoffResourceStateContext([]HandoffResource{{
		Name: "pve-1",
		Type: "agent",
	}}, provider)
	for _, expected := range []string{
		"[Resource State Context]",
		"Resource State: pve-1",
		"Resource State ID: agent:pve-1",
		"Resource State Type: agent",
		"Resource State Status: warning",
		"Resource State Last Seen At: 2026-05-06T13:57:00Z",
		"Resource State Updated At: 2026-05-06T13:59:00Z",
		"Resource State Sources: agent, proxmox",
		"Resource State Source Health: agent: online",
		"proxmox: stale",
		"Resource State Metrics: cpu 92.5%, memory 81%, disk 71.2%",
		"Resource State Parent: production-cluster",
		"Resource State Child Count: 2",
		"Resource State Incident Summary: count 1; pve-1 is saturated after backup traffic; CPU saturation remains elevated (cpu_saturation)",
		"Resource State Capabilities: restart (common; approval admin): Restart the workload",
		"Resource State Boundary: Current resource state, source health, incidents, metrics, and capabilities are read-only canonical infrastructure context",
	} {
		if !strings.Contains(contextText, expected) {
			t.Fatalf("resource state context missing %q: %q", expected, contextText)
		}
	}
	for _, forbidden := range []string{
		"internal-restart-handler",
		"secret-token",
		"native-secret-incident",
		"ssh://root@example.invalid/private",
	} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("resource state context leaked raw execution/provider detail %q: %q", forbidden, contextText)
		}
	}
}

func TestBuildHandoffResourceTimelineContextUsesRelatedCanonicalChanges(t *testing.T) {
	now := time.Now()
	store := unifiedresources.NewMemoryStore()
	for _, change := range []unifiedresources.ResourceChange{
		{
			ID:               "change-direct",
			ResourceID:       "vm-100",
			ObservedAt:       now.Add(-2 * time.Minute),
			Kind:             unifiedresources.ChangeStateTransition,
			From:             "running",
			To:               "degraded",
			SourceType:       unifiedresources.SourcePulseDiff,
			SourceAdapter:    unifiedresources.AdapterProxmox,
			Confidence:       unifiedresources.ConfidenceHigh,
			RelatedResources: []string{"storage-1"},
		},
		{
			ID:               "change-related",
			ResourceID:       "storage-1",
			ObservedAt:       now.Add(-1 * time.Minute),
			Kind:             unifiedresources.ChangeActivity,
			To:               "backup completed",
			SourceType:       unifiedresources.SourcePlatformEvent,
			SourceAdapter:    unifiedresources.AdapterProxmox,
			Confidence:       unifiedresources.ConfidenceHigh,
			RelatedResources: []string{"vm-100"},
		},
	} {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s) failed: %v", change.ID, err)
		}
	}

	contextText := buildHandoffResourceTimelineContext([]HandoffResource{{
		ID:   "vm-100",
		Name: "web-server",
		Type: "vm",
	}}, nil, store, now)
	if !strings.Contains(contextText, "vm-100: **State transition** running") {
		t.Fatalf("timeline context missing direct change: %q", contextText)
	}
	if !strings.Contains(contextText, "storage-1: **Activity**") {
		t.Fatalf("timeline context missing related change: %q", contextText)
	}
	if strings.Contains(contextText, "Approval ID") {
		t.Fatalf("timeline context should not include approval metadata: %q", contextText)
	}
}

func TestBuildHandoffResourceTimelineContextResolvesCanonicalResourceReferences(t *testing.T) {
	now := time.Now()
	store := unifiedresources.NewMemoryStore()
	if err := store.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-canonical",
		ResourceID:    "vm:node1:104",
		ObservedAt:    now.Add(-3 * time.Minute),
		Kind:          unifiedresources.ChangeStateTransition,
		From:          "running",
		To:            "degraded",
		SourceType:    unifiedresources.SourcePulseDiff,
		SourceAdapter: unifiedresources.AdapterProxmox,
		Confidence:    unifiedresources.ConfidenceHigh,
	}); err != nil {
		t.Fatalf("RecordChange failed: %v", err)
	}

	provider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeVM: {{
			ID:     "vm:node1:104",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "finance-vm",
			Status: unifiedresources.StatusOnline,
		}},
	}}

	contextText := buildHandoffResourceTimelineContext([]HandoffResource{{
		Name: "finance-vm",
		Type: "vm",
	}}, provider, store, now)
	if !strings.Contains(contextText, "vm:node1:104: **State transition** running") {
		t.Fatalf("timeline context missing provider-resolved canonical change: %q", contextText)
	}
	if !strings.Contains(contextText, "Timeline Boundary: Recent changes are read-only canonical resource timeline context") {
		t.Fatalf("timeline context missing boundary: %q", contextText)
	}
}

func TestRefreshHandoffActionApprovalStatusRejectsCrossOrgApproval(t *testing.T) {
	installTestApprovalStore(t, &approval.ApprovalRequest{
		ID:         "approval-cross-org",
		OrgID:      "org-a",
		Command:    "systemctl restart workload.service",
		TargetType: "vm",
		TargetID:   "vm-100",
		TargetName: "web-server",
		RiskLevel:  approval.RiskHigh,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	})

	actions := refreshHandoffActionApprovalStatus([]HandoffAction{{
		ApprovalID:          "approval-cross-org",
		ApprovalStatus:      "pending",
		ApprovalRequestedAt: "2026-01-01T00:00:00Z",
		FixID:               "fix-123",
		Description:         "Restart the workload service",
	}}, "org-b")

	if len(actions) != 1 {
		t.Fatalf("actions = %#v, want one retained action reference", actions)
	}
	if actions[0].ApprovalStatus != "" || actions[0].ApprovalRequestedAt != "" {
		t.Fatalf("cross-org approval status should be cleared, got %#v", actions[0])
	}
	contextText := buildHandoffActionContext(actions)
	if strings.Contains(contextText, "Approval Status") {
		t.Fatalf("cross-org approval status leaked into action context: %q", contextText)
	}
	if strings.Contains(contextText, "systemctl restart workload.service") {
		t.Fatalf("raw command leaked into action context: %q", contextText)
	}
}

func TestRefreshHandoffActionStatusHydratesCanonicalActionAudit(t *testing.T) {
	actionNow := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
	actionPlan := testHandoffActionPlan(actionNow)
	store := unifiedresources.NewMemoryStore()
	seedHandoffActionAudit(t, store, actionPlan, unifiedresources.ActionStateCompleted, &unifiedresources.ExecutionResult{
		Success: true,
		Output:  "restart dispatched",
	})

	actions := refreshHandoffActionStatus([]HandoffAction{{
		ActionID:    "act-handoff-123",
		Description: "Restart the workload service",
	}}, approval.DefaultOrgID, store)

	if len(actions) != 1 {
		t.Fatalf("actions = %#v, want one action reference", actions)
	}
	if actions[0].ActionState != string(unifiedresources.ActionStateCompleted) {
		t.Fatalf("action state = %q, want completed", actions[0].ActionState)
	}
	if actions[0].ActionResult != "success" {
		t.Fatalf("action result = %q, want success", actions[0].ActionResult)
	}
	if actions[0].ActionCapability != "restart" || actions[0].ActionRequestedBy != "pulse_patrol" {
		t.Fatalf("action audit identity not hydrated: %#v", actions[0])
	}

	contextText := buildHandoffActionContext(actions)
	for _, expected := range []string{
		"Action Reference Action ID: act-handoff-123",
		"Action Reference Action State: completed",
		"Action Reference Action Result: success",
		"Action Reference Action Requested By: pulse_patrol",
		"Action Reference Action Capability: restart",
		"Action Reference Action Preflight: Restart the workload service",
	} {
		if !strings.Contains(contextText, expected) {
			t.Fatalf("action context missing %q: %q", expected, contextText)
		}
	}
	if strings.Contains(contextText, "restart dispatched") {
		t.Fatalf("action context should expose result state, not raw execution output: %q", contextText)
	}
}

func TestService_ClearModelHandoffContextInvalidatesUnpinnedActionScope(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := store.SetModelHandoffFindingID(session.ID, "finding-123"); err != nil {
		t.Fatalf("SetModelHandoffFindingID failed: %v", err)
	}
	if err := store.SetModelHandoffContext(session.ID, "[Finding Context]\nID: finding-123"); err != nil {
		t.Fatalf("SetModelHandoffContext failed: %v", err)
	}
	if err := store.SetModelHandoffResources(session.ID, []HandoffResource{{
		ID:   "vm-100",
		Name: "web-server",
		Type: "vm",
		Node: "pve-1",
	}}); err != nil {
		t.Fatalf("SetModelHandoffResources failed: %v", err)
	}

	resolvedCtx := store.GetResolvedContext(session.ID)
	resolvedCtx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "vm",
		ProviderUID: "100",
		HostName:    "pve-1",
		Name:        "web-server",
	})
	resolvedCtx.AddResolvedResource(tools.ResourceRegistration{
		Kind:        "vm",
		ProviderUID: "101",
		HostName:    "pve-1",
		Name:        "pinned-vm",
	})
	resolvedCtx.PinResource("vm:pve-1:101")

	svc := &Service{
		sessions: store,
		started:  true,
	}
	if err := svc.ClearModelHandoffContext(context.Background(), session.ID); err != nil {
		t.Fatalf("ClearModelHandoffContext failed: %v", err)
	}

	if got, err := store.GetModelHandoffFindingID(session.ID); err != nil {
		t.Fatalf("GetModelHandoffFindingID failed: %v", err)
	} else if got != "" {
		t.Fatalf("handoff finding ID after clear = %q, want empty", got)
	}
	if _, ok := resolvedCtx.GetResourceByID("vm:pve-1:100"); ok {
		t.Fatalf("stale handoff resource remained in resolved context")
	}
	if _, ok := resolvedCtx.GetResourceByID("vm:pve-1:101"); !ok {
		t.Fatalf("pinned user resource should survive handoff invalidation")
	}
}

func TestService_ExecuteStream_HandoffResourceHydratesResolvedContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm:node1:101", VMID: 101, Name: "web-server", Node: "node1", Status: "running"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state)
	vmResources := registry.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one canonical VM resource, got %d", len(vmResources))
	}
	vmResource := vmResources[0]
	unifiedProvider := unifiedresources.NewUnifiedAIAdapter(registry)

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID:      "sess-handoff-resource",
		Prompt:         "What should I do next?",
		HandoffContext: "[Finding Context]\nID: finding-123",
		HandoffResources: []HandoffResource{{
			ID:   vmResource.ID,
			Name: "web-server",
			Type: "vm",
			Node: "node1",
		}},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	resolved := store.GetResolvedContext("sess-handoff-resource")
	info, found := resolved.GetResolvedResourceByAlias("web-server")
	if !found {
		t.Fatalf("expected handoff resource to be registered by alias")
	}
	if !resolved.WasRecentlyAccessed(info.GetResourceID(), time.Minute) {
		t.Fatalf("expected handoff resource to be marked as explicitly accessed")
	}
	if _, err := resolved.ValidateResourceForAction(info.GetResourceID(), "restart"); err != nil {
		t.Fatalf("expected handoff VM to allow governed restart action: %v", err)
	}

	reloadedStore, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload session store: %v", err)
	}
	reloadedSvc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openai:test"},
		sessions:                reloadedStore,
		executor:                executor,
		agenticLoop:             loop,
		provider:                provider,
		unifiedResourceProvider: unifiedProvider,
		started:                 true,
	}
	followUpReq := ExecuteRequest{
		SessionID: "sess-handoff-resource",
		Prompt:    "Can you restart it?",
	}
	if err := reloadedSvc.ExecuteStream(context.Background(), followUpReq, func(StreamEvent) {}); err != nil {
		t.Fatalf("follow-up ExecuteStream failed: %v", err)
	}
	reloadedResolved := reloadedStore.GetResolvedContext("sess-handoff-resource")
	reloadedInfo, found := reloadedResolved.GetResolvedResourceByAlias("web-server")
	if !found {
		t.Fatalf("expected stored handoff resource to rehydrate by alias after session reload")
	}
	if _, err := reloadedResolved.ValidateResourceForAction(reloadedInfo.GetResourceID(), "restart"); err != nil {
		t.Fatalf("expected rehydrated handoff VM to allow governed restart action: %v", err)
	}
}

func latestProviderUserContent(t *testing.T, messages []providers.Message) string {
	t.Helper()

	for idx := len(messages) - 1; idx >= 0; idx-- {
		if messages[idx].Role == "user" {
			return messages[idx].Content
		}
	}
	t.Fatal("expected provider request to include a user message")
	return ""
}

func TestService_ExecuteStream_PrefetchMentionsAndOverrideModel(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	state := models.StateSnapshot{
		VMs: []models.VM{
			{ID: "vm:node1:101", VMID: 101, Name: "vm1", Node: "node1", Status: "running"},
		},
	}
	readState := unifiedresources.NewRegistry(nil)
	readState.IngestSnapshot(state)
	vmResources := readState.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one canonical VM resource, got %d", len(vmResources))
	}
	vmResourceID := vmResources[0].ID

	var capturedModel string
	svc := &Service{
		cfg:               &config.AIConfig{ChatModel: "openai:primary"},
		sessions:          store,
		executor:          executor,
		agenticLoop:       loop,
		contextPrefetcher: NewContextPrefetcher(readState, nil),
		provider:          provider,
		started:           true,
		providerFactory: func(modelStr string) (providers.StreamingProvider, error) {
			capturedModel = modelStr
			return provider, nil
		},
	}

	autonomous := true
	req := ExecuteRequest{
		SessionID:      "sess-2",
		Prompt:         "check @vm1",
		Model:          "openai:override",
		Mentions:       []StructuredMention{{ID: "vm:node1:101", Name: "vm1", Type: "vm", Node: "node1"}},
		MaxTurns:       1,
		AutonomousMode: &autonomous,
	}

	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if capturedModel != "openai:override" {
		t.Fatalf("expected override model to be used, got %q", capturedModel)
	}

	resolved := store.GetResolvedContext("sess-2")
	if !resolved.WasRecentlyAccessed(vmResourceID, time.Minute) {
		t.Fatal("expected explicit access to be recorded for structured mention")
	}
}

func TestService_ExecuteStream_PrefetchMentionsMarksVMwareUnifiedResourceAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	provider := &stubServiceProvider{}
	loop := NewAgenticLoop(provider, executor, "system")

	now := time.Now().UTC()
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestRecords(unifiedresources.SourceVMware, []unifiedresources.IngestRecord{{
		SourceID: "vc-1:vm:vm-201",
		Resource: unifiedresources.Resource{
			ID:         "vmware-vm-1",
			Type:       unifiedresources.ResourceTypeVM,
			Name:       "app-01",
			Status:     unifiedresources.StatusOnline,
			LastSeen:   now,
			UpdatedAt:  now,
			ParentName: "esxi-01.lab.local",
			VMware: &unifiedresources.VMwareData{
				ConnectionID:    "vc-1",
				ConnectionName:  "Lab VC",
				ManagedObjectID: "vm-201",
				EntityType:      "vm",
				RuntimeHostName: "esxi-01.lab.local",
			},
		},
		Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"app-01"}},
	}})
	vmResources := rr.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one VMware VM resource, got %d", len(vmResources))
	}
	vmResourceID := vmResources[0].ID

	svc := &Service{
		cfg:               &config.AIConfig{ChatModel: "openai:primary"},
		sessions:          store,
		executor:          executor,
		agenticLoop:       loop,
		contextPrefetcher: NewContextPrefetcher(rr, nil),
		provider:          provider,
		started:           true,
		providerFactory: func(modelStr string) (providers.StreamingProvider, error) {
			return provider, nil
		},
	}

	autonomous := true
	req := ExecuteRequest{
		SessionID:      "sess-vmware-mentions",
		Prompt:         "check @app-01",
		Mentions:       []StructuredMention{{ID: vmResourceID, Name: "app-01", Type: "vm"}},
		MaxTurns:       1,
		AutonomousMode: &autonomous,
	}

	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	resolved := store.GetResolvedContext("sess-vmware-mentions")
	if !resolved.WasRecentlyAccessed(vmResourceID, time.Minute) {
		t.Fatal("expected explicit access to be recorded for VMware structured mention")
	}
}

func TestService_ExecuteStream_AgenticPulseStorageSnapshotsToleratesMalformedRecoveryMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	completedAt := time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		ReadState:     &fakeCanonicalReadState{},
		RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{{
			ID:       "pve-snapshot:snap-100-before-upgrade",
			Provider: recovery.ProviderProxmoxPVE,
			Kind:     recovery.KindSnapshot,
			Mode:     recovery.ModeLocal,
			Outcome:  recovery.OutcomeSuccess,
			SubjectRef: &recovery.ExternalRef{
				Type:      "vm",
				Name:      "100",
				ID:        "100",
				Namespace: "pve1",
				Class:     "node1",
			},
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "100",
				ItemType:       "vm",
				ClusterLabel:   "pve1",
				NodeHostLabel:  "node1",
				EntityIDLabel:  "100",
				DetailsSummary: "before-upgrade",
			},
			CompletedAt: &completedAt,
		}}},
	})

	turn := 0
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderTool(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "call-snapshots",
							Name: "pulse_storage",
							Input: map[string]interface{}{
								"type":     "snapshots",
								"guest_id": "100",
								"instance": "pve1",
							},
						}},
					},
				})
				return nil
			case 2:
				var toolResult string
				for _, msg := range req.Messages {
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "call-snapshots" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"snapshot_name\":\"before-upgrade\"") {
					t.Fatalf("expected canonical snapshot name in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"pve1\"") || !strings.Contains(toolResult, "\"node\":\"node1\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "Recovered snapshot inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						InputTokens:  10,
						OutputTokens: 12,
					},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}

	req := ExecuteRequest{
		SessionID: "sess-storage-snapshots",
		Prompt:    "List recovery snapshots for guest 100 on pve1.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	messages, err := store.GetMessages(req.SessionID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"snapshot_name\":\"before-upgrade\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored tool result with canonical snapshot fallback, got %#v", messages)
	}
}

func TestService_ExecuteStream_AgenticPulseStorageBackupTasksToleratesMalformedRecoveryMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	startedAt := time.Date(2026, 2, 24, 11, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 2, 24, 11, 15, 0, 0, time.UTC)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{
		StateProvider: fakeStateProvider{},
		ReadState:     &fakeCanonicalReadState{},
		RecoveryPointsProvider: &fakeRecoveryPointsProvider{points: []recovery.RecoveryPoint{{
			ID:       "pve-task:task-101-backup",
			Provider: recovery.ProviderProxmoxPVE,
			Kind:     recovery.KindBackup,
			Mode:     recovery.ModeLocal,
			Outcome:  recovery.OutcomeSuccess,
			SubjectRef: &recovery.ExternalRef{
				Type:      "vm",
				Name:      "101",
				ID:        "101",
				Namespace: "pve1",
				Class:     "node1",
			},
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "101",
				ItemType:       "vm",
				ClusterLabel:   "pve1",
				NodeHostLabel:  "node1",
				EntityIDLabel:  "101",
				DetailsSummary: "completed successfully",
			},
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
		}}},
	})

	turn := 0
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			turn++
			switch turn {
			case 1:
				if !hasProviderTool(req.Tools, "pulse_storage") {
					t.Fatalf("expected pulse_storage tool to be available, got %#v", req.Tools)
				}
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						StopReason: "tool_use",
						ToolCalls: []providers.ToolCall{{
							ID:   "call-backup-tasks",
							Name: "pulse_storage",
							Input: map[string]interface{}{
								"type":     "backup_tasks",
								"guest_id": "101",
								"instance": "pve1",
								"status":   "OK",
							},
						}},
					},
				})
				return nil
			case 2:
				var toolResult string
				for _, msg := range req.Messages {
					if msg.ToolResult != nil && msg.ToolResult.ToolUseID == "call-backup-tasks" {
						toolResult = msg.ToolResult.Content
						break
					}
				}
				if toolResult == "" {
					t.Fatalf("expected pulse_storage tool result in second turn, got %#v", req.Messages)
				}
				if !strings.Contains(toolResult, "\"status\":\"OK\"") || !strings.Contains(toolResult, "\"type\":\"vm\"") {
					t.Fatalf("expected canonical backup task fields in tool result, got %s", toolResult)
				}
				if !strings.Contains(toolResult, "\"instance\":\"pve1\"") || !strings.Contains(toolResult, "\"node\":\"node1\"") {
					t.Fatalf("expected canonical placement labels in tool result, got %s", toolResult)
				}
				callback(providers.StreamEvent{
					Type: "content",
					Data: providers.ContentEvent{Text: "Recovered backup task inventory"},
				})
				callback(providers.StreamEvent{
					Type: "done",
					Data: providers.DoneEvent{
						InputTokens:  10,
						OutputTokens: 12,
					},
				})
				return nil
			default:
				t.Fatalf("unexpected extra provider turn %d", turn)
				return nil
			}
		},
	}

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openai:test"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}

	req := ExecuteRequest{
		SessionID: "sess-storage-backup-tasks",
		Prompt:    "List recovery backup tasks for guest 101 on pve1.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	messages, err := store.GetMessages(req.SessionID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.ToolResult != nil && strings.Contains(msg.ToolResult.Content, "\"status\":\"OK\"") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected stored tool result with canonical backup-task fallback, got %#v", messages)
	}
}
