package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type stubServiceProvider struct {
	streamFn func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error
}

type sessionAwareStateProvider struct {
	readSeen                    *atomic.Bool
	sessionSeen                 *atomic.Bool
	inventoryProgressSeen       *atomic.Bool
	readBeforeSession           *atomic.Bool
	readBeforeInventoryProgress *atomic.Bool
}

func (p sessionAwareStateProvider) ReadSnapshot() models.StateSnapshot {
	if p.readSeen != nil {
		p.readSeen.Store(true)
	}
	if p.sessionSeen != nil && !p.sessionSeen.Load() && p.readBeforeSession != nil {
		p.readBeforeSession.Store(true)
	}
	if p.inventoryProgressSeen != nil && !p.inventoryProgressSeen.Load() && p.readBeforeInventoryProgress != nil {
		p.readBeforeInventoryProgress.Store(true)
	}
	return models.StateSnapshot{}
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

func TestHydrateHandoffActionFromApprovalCopiesSafeLifecycleMetadata(t *testing.T) {
	requestedAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	expiresAt := requestedAt.Add(10 * time.Minute)
	decidedAt := requestedAt.Add(2 * time.Minute)
	planExpiresAt := requestedAt.Add(5 * time.Minute)

	action := HandoffAction{FindingID: "finding-123"}
	HydrateHandoffActionFromApproval(&action, &approval.ApprovalRequest{
		ID:          "approval-123",
		ToolID:      "investigation_fix",
		Command:     "systemctl restart workload.service",
		TargetType:  "vm",
		TargetID:    "vm-100",
		TargetName:  "web-server",
		RiskLevel:   approval.RiskHigh,
		Status:      approval.StatusApproved,
		RequestedAt: requestedAt,
		ExpiresAt:   expiresAt,
		DecidedAt:   &decidedAt,
		Consumed:    true,
		Plan: &unifiedresources.ActionPlan{
			ActionID:         "action-123",
			RequiresApproval: true,
			ApprovalPolicy:   unifiedresources.ApprovalAdmin,
			Message:          "Restart after the backup window clears.",
			ExpiresAt:        planExpiresAt,
			Preflight: &unifiedresources.ActionPreflight{
				IntendedChange: "Restart workload service",
				DryRunSummary:  "No provider-supported dry run is available for this action.",
			},
		},
	})

	if action.ApprovalID != "approval-123" || action.ApprovalStatus != "approved" || !action.ApprovalConsumed {
		t.Fatalf("approval lifecycle was not hydrated: %#v", action)
	}
	if action.ActionRequestedBy != approval.RequesterPulsePatrol {
		t.Fatalf("approval requester identity was not hydrated: %#v", action)
	}
	if action.ApprovalRequestedAt != requestedAt.Format(time.RFC3339) ||
		action.ApprovalExpiresAt != expiresAt.Format(time.RFC3339) ||
		action.ApprovalDecidedAt != decidedAt.Format(time.RFC3339) {
		t.Fatalf("approval timestamps were not hydrated: %#v", action)
	}
	if action.ActionID != "action-123" ||
		action.ActionApprovalPolicy != "admin" ||
		!action.ActionRequiresApproval ||
		action.ActionPlanExpiresAt != planExpiresAt.Format(time.RFC3339) {
		t.Fatalf("action plan metadata was not hydrated: %#v", action)
	}
	if action.RiskLevel != "high" ||
		action.TargetResourceID != "vm-100" ||
		action.TargetResourceName != "web-server" ||
		action.TargetResourceType != "vm" {
		t.Fatalf("approval target metadata was not hydrated: %#v", action)
	}
	safeContext := strings.Join([]string{
		action.ActionPlanMessage,
		action.ActionPreflight,
		action.ActionDryRunSummary,
		action.Description,
	}, " ")
	if strings.Contains(safeContext, "systemctl restart workload.service") {
		t.Fatalf("approval command leaked into handoff action: %#v", action)
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

func TestService_ListSessionsRefreshesHandoffActionSummary(t *testing.T) {
	now := time.Now().UTC().Add(-2 * time.Minute)
	decidedAt := now.Add(2 * time.Minute)
	actionPlan := testHandoffActionPlan(now)
	installTestApprovalStore(t, &approval.ApprovalRequest{
		ID:          "approval-123",
		OrgID:       approval.DefaultOrgID,
		Command:     "systemctl restart workload.service",
		TargetType:  "vm",
		TargetID:    "vm-100",
		TargetName:  "web-server",
		RiskLevel:   approval.RiskMedium,
		Status:      approval.StatusApproved,
		RequestedAt: now,
		DecidedAt:   &decidedAt,
		ExpiresAt:   now.Add(10 * time.Minute),
		Plan:        &actionPlan,
	})
	if _, err := approval.GetStore().Approve("approval-123", "operator@example.com"); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
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
	if err := store.SetModelHandoffActions(session.ID, []HandoffAction{{
		FindingID:              "finding-123",
		ApprovalID:             "approval-123",
		ApprovalStatus:         "pending",
		ActionID:               actionPlan.ActionID,
		ActionState:            "awaiting_approval",
		ActionRequiresApproval: true,
		ActionPreflight:        "systemctl restart workload.service",
		ActionResult:           "raw command output",
		RiskLevel:              "medium",
		TargetResourceID:       "vm-100",
		TargetResourceName:     "web-server",
		TargetResourceType:     "vm",
	}}); err != nil {
		t.Fatalf("SetModelHandoffActions failed: %v", err)
	}

	actionStore := unifiedresources.NewMemoryStore()
	seedHandoffActionAudit(t, actionStore, actionPlan, unifiedresources.ActionStateCompleted, &unifiedresources.ExecutionResult{
		Success: true,
		Output:  "raw command output",
	})
	svc := &Service{
		sessions:         store,
		actionAuditStore: actionStore,
		started:          true,
	}

	sessions, err := svc.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].HandoffSummary == nil {
		t.Fatalf("sessions = %#v, want one handoff summary", sessions)
	}
	summary := sessions[0].HandoffSummary
	if summary.LastKnownApprovalStatus != "approved" {
		t.Fatalf("approval status = %q, want approved", summary.LastKnownApprovalStatus)
	}
	if summary.LastKnownActionState != string(unifiedresources.ActionStateCompleted) {
		t.Fatalf("action state = %q, want completed", summary.LastKnownActionState)
	}
	if summary.RequiresApproval || summary.ActionCount != 1 {
		t.Fatalf("action summary = %#v, want completed action context without approval requirement", summary)
	}

	actions, err := store.GetModelHandoffActions(session.ID)
	if err != nil {
		t.Fatalf("GetModelHandoffActions failed: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %#v, want one refreshed action", actions)
	}
	safeContext := strings.Join([]string{
		actions[0].ActionPreflight,
		actions[0].ActionDryRunSummary,
		actions[0].ActionResult,
	}, " ")
	if strings.Contains(safeContext, "systemctl restart workload.service") || strings.Contains(safeContext, "raw command output") {
		t.Fatalf("refreshed handoff action leaked command details: %#v", actions[0])
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
	var sessionIDs []string
	callback := func(event StreamEvent) {
		if event.Type == "done" {
			doneCount++
		}
		if event.Type == "session" {
			var data SessionData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal session event: %v", err)
			}
			sessionIDs = append(sessionIDs, data.ID)
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
	if !reflect.DeepEqual(sessionIDs, []string{"sess-1"}) {
		t.Fatalf("session events = %#v, want sess-1", sessionIDs)
	}

	messages, err := store.GetMessages("sess-1")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestService_ExecuteStream_StoresSelectedModelRouteOnUserTurn(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	const selectedRoute = "openrouter:qwen/qwen3.7-plus"
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "restored route answer"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	service := NewService(Config{
		AIConfig: &config.AIConfig{ChatModel: "openai:gpt-4o"},
	})
	service.sessions = store
	service.provider = provider
	var requestedProviderModel string
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		requestedProviderModel = model
		if model == selectedRoute {
			return provider, nil
		}
		return nil, errors.New("unexpected model " + model)
	}
	service.started = true

	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "sess-selected-model",
		Prompt:    "continue with this provider",
		Model:     selectedRoute,
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if requestedProviderModel != selectedRoute {
		t.Fatalf("provider factory model = %q, want %q", requestedProviderModel, selectedRoute)
	}

	messages, err := store.GetMessages("sess-selected-model")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("first message role = %q, want user", messages[0].Role)
	}
	if messages[0].Model != selectedRoute {
		t.Fatalf("user message model = %q, want %q", messages[0].Model, selectedRoute)
	}
	if messages[1].Role != "assistant" {
		t.Fatalf("second message role = %q, want assistant", messages[1].Role)
	}
	if messages[1].Model != selectedRoute {
		t.Fatalf("assistant message model = %q, want %q", messages[1].Model, selectedRoute)
	}
}

func TestServiceExecuteStreamMockModeStreamsToolFixtureWithoutProviderCall(t *testing.T) {
	originalMockMode := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(originalMockMode) })
	mockruntime.SetEnabled(true)
	originalPace := mockAssistantStreamPace
	mockAssistantStreamPace = 0
	t.Cleanup(func() { mockAssistantStreamPace = originalPace })

	var providerFactoryCalls atomic.Int32
	var providerStreamCalls atomic.Int32
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			providerStreamCalls.Add(1)
			return errors.New("provider should not stream in mock mode")
		},
	}
	service := NewService(Config{
		AIConfig: &config.AIConfig{
			Enabled: true,
		},
		StateProvider: &mockStateProvider{},
		DataDir:       t.TempDir(),
	})
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		providerFactoryCalls.Add(1)
		return provider, nil
	}

	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var events []StreamEvent
	if err := service.ExecuteStream(ctx, ExecuteRequest{
		SessionID: "mock-fixture-session",
		Prompt:    "show me the assistant stream",
	}, func(event StreamEvent) {
		events = append(events, event)
	}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if got := providerFactoryCalls.Load(); got != 0 {
		t.Fatalf("provider factory calls = %d, want 0", got)
	}
	if got := providerStreamCalls.Load(); got != 0 {
		t.Fatalf("provider ChatStream calls = %d, want 0", got)
	}

	var eventTypes []string
	for _, event := range events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantTypes := []string{
		"session",
		"workflow_state",
		"workflow_state",
		"tool_start",
		"tool_progress",
		"tool_progress",
		"tool_end",
		"tool_start",
		"tool_progress",
		"tool_progress",
		"tool_end",
		"workflow_state",
		"content",
		"content",
		"content",
		"content",
		"done",
	}
	if !reflect.DeepEqual(eventTypes, wantTypes) {
		t.Fatalf("event types = %#v, want %#v", eventTypes, wantTypes)
	}

	var toolStart ToolStartData
	if err := json.Unmarshal(events[3].Data, &toolStart); err != nil {
		t.Fatalf("unmarshal tool_start: %v", err)
	}
	if toolStart.ID != mockAssistantQueryToolID || toolStart.Name != mockAssistantQueryToolName {
		t.Fatalf("tool_start = %#v, want mock pulse_query", toolStart)
	}

	var toolProgress ToolProgressData
	if err := json.Unmarshal(events[4].Data, &toolProgress); err != nil {
		t.Fatalf("unmarshal tool_progress: %v", err)
	}
	if toolProgress.ID != mockAssistantQueryToolID || toolProgress.Phase != "running" {
		t.Fatalf("tool_progress = %#v, want running mock tool", toolProgress)
	}

	var toolEnd ToolEndData
	if err := json.Unmarshal(events[6].Data, &toolEnd); err != nil {
		t.Fatalf("unmarshal tool_end: %v", err)
	}
	if toolEnd.ID != mockAssistantQueryToolID || toolEnd.Name != mockAssistantQueryToolName || !toolEnd.Success {
		t.Fatalf("tool_end = %#v, want successful mock pulse_query", toolEnd)
	}

	var readToolStart ToolStartData
	if err := json.Unmarshal(events[7].Data, &readToolStart); err != nil {
		t.Fatalf("unmarshal read tool_start: %v", err)
	}
	if readToolStart.ID != mockAssistantReadToolID || readToolStart.Name != mockAssistantReadToolName {
		t.Fatalf("read tool_start = %#v, want mock pulse_read", readToolStart)
	}

	var readToolProgress ToolProgressData
	if err := json.Unmarshal(events[8].Data, &readToolProgress); err != nil {
		t.Fatalf("unmarshal read tool_progress: %v", err)
	}
	if readToolProgress.ID != mockAssistantReadToolID || !strings.Contains(readToolProgress.RawInput, "pulse_read(") {
		t.Fatalf("read tool_progress = %#v, want raw provider-style pulse_read input", readToolProgress)
	}

	var readToolEnd ToolEndData
	if err := json.Unmarshal(events[10].Data, &readToolEnd); err != nil {
		t.Fatalf("unmarshal read tool_end: %v", err)
	}
	if readToolEnd.ID != mockAssistantReadToolID || readToolEnd.Name != mockAssistantReadToolName || !readToolEnd.Success {
		t.Fatalf("read tool_end = %#v, want successful mock pulse_read", readToolEnd)
	}

	var done DoneData
	if err := json.Unmarshal(events[len(events)-1].Data, &done); err != nil {
		t.Fatalf("unmarshal done: %v", err)
	}
	if done.SessionID != "mock-fixture-session" || done.Model != mockAssistantModelRoute {
		t.Fatalf("done = %#v, want mock model and session", done)
	}

	messages, err := service.GetMessages(ctx, "mock-fixture-session")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want user and assistant", len(messages))
	}
	assistantMessage := messages[1]
	if assistantMessage.Role != "assistant" || assistantMessage.Model != mockAssistantModelRoute {
		t.Fatalf("assistant message = %#v, want persisted mock assistant", assistantMessage)
	}
	if len(assistantMessage.ToolCalls) != 2 {
		t.Fatalf("assistant tool calls = %d, want 2", len(assistantMessage.ToolCalls))
	}
	if assistantMessage.ToolCalls[0].Name != mockAssistantQueryToolName {
		t.Fatalf("assistant tool call = %#v, want mock pulse_query", assistantMessage.ToolCalls[0])
	}
	if assistantMessage.ToolCalls[0].Success == nil || !*assistantMessage.ToolCalls[0].Success {
		t.Fatalf("assistant tool call success = %#v, want true", assistantMessage.ToolCalls[0].Success)
	}
	if assistantMessage.ToolCalls[1].Name != mockAssistantReadToolName {
		t.Fatalf("assistant tool call = %#v, want mock pulse_read", assistantMessage.ToolCalls[1])
	}
	if assistantMessage.ToolCalls[1].Success == nil || !*assistantMessage.ToolCalls[1].Success {
		t.Fatalf("assistant tool call success = %#v, want true", assistantMessage.ToolCalls[1].Success)
	}
}

func TestService_ExecuteStream_EmitsProviderStartupBeforeProviderCall(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var providerStartSeen atomic.Bool
	var providerCalledBeforeStart atomic.Bool
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			if !providerStartSeen.Load() {
				providerCalledBeforeStart.Store(true)
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
		},
	}
	service := NewService(Config{
		AIConfig: &config.AIConfig{ChatModel: "openrouter:qwen/qwen3.7-plus"},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "sess-provider-start",
		Prompt:    "hello",
	}, func(event StreamEvent) {
		if event.Type != "workflow_state" {
			return
		}
		var data WorkflowStateData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("unmarshal workflow state: %v", err)
		}
		if data.Phase == "provider_start" {
			providerStartSeen.Store(true)
			if data.Message != "OpenRouter is starting the response." {
				t.Fatalf("provider_start message = %q", data.Message)
			}
			if data.Provider != config.AIProviderOpenRouter {
				t.Fatalf("provider_start provider = %q, want %q", data.Provider, config.AIProviderOpenRouter)
			}
			if data.Model != "openrouter:qwen/qwen3.7-plus" {
				t.Fatalf("provider_start model = %q, want OpenRouter route", data.Model)
			}
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if !providerStartSeen.Load() {
		t.Fatal("provider_start workflow event was not emitted")
	}
	if providerCalledBeforeStart.Load() {
		t.Fatal("provider was called before provider_start workflow event reached the stream")
	}
}

func TestService_ExecuteStream_EmitsSessionBeforeInventoryPrefetch(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var sessionSeen atomic.Bool
	var inventoryProgressSeen atomic.Bool
	var stateReadSeen atomic.Bool
	var stateReadBeforeSession atomic.Bool
	var stateReadBeforeInventoryProgress atomic.Bool
	provider := &stubServiceProvider{}
	service := NewService(Config{
		AIConfig: &config.AIConfig{ChatModel: "openai:test"},
		StateProvider: sessionAwareStateProvider{
			readSeen:                    &stateReadSeen,
			sessionSeen:                 &sessionSeen,
			inventoryProgressSeen:       &inventoryProgressSeen,
			readBeforeSession:           &stateReadBeforeSession,
			readBeforeInventoryProgress: &stateReadBeforeInventoryProgress,
		},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "sess-early-session",
		Prompt:    "how many devices are in this",
	}, func(event StreamEvent) {
		if event.Type == "session" {
			sessionSeen.Store(true)
		}
		if event.Type == "workflow_state" {
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal workflow state: %v", err)
			}
			if data.Phase == "context" && data.Message == "Reading current Pulse inventory with pulse_query." {
				inventoryProgressSeen.Store(true)
			}
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if !sessionSeen.Load() {
		t.Fatal("session event was not emitted")
	}
	if !stateReadSeen.Load() {
		t.Fatal("inventory prefetch did not read state")
	}
	if !inventoryProgressSeen.Load() {
		t.Fatal("inventory progress event was not emitted")
	}
	if stateReadBeforeSession.Load() {
		t.Fatal("inventory prefetch read state before the browser-visible session event")
	}
	if stateReadBeforeInventoryProgress.Load() {
		t.Fatal("inventory prefetch read state before the inventory progress event")
	}
}

func TestService_ExecuteStream_SuppressesSessionEventWhenCallerAlreadySentIt(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	provider := &stubServiceProvider{}
	service := NewService(Config{
		AIConfig: &config.AIConfig{ChatModel: "openai:test"},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	var sessionEvents int
	var doneEvents int
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID:            "sess-session-suppressed",
		Prompt:               "hello",
		SuppressSessionEvent: true,
	}, func(event StreamEvent) {
		switch event.Type {
		case "session":
			sessionEvents++
		case "done":
			doneEvents++
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if sessionEvents != 0 {
		t.Fatalf("session events = %d, want 0", sessionEvents)
	}
	if doneEvents != 1 {
		t.Fatalf("done events = %d, want 1", doneEvents)
	}
}

func TestService_ExecuteStream_DoesNotFallbackWhenSelectedProviderFailsBeforeVisibleOutput(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var fallbackCalled atomic.Bool
	primary := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			return errors.New("API error (401): unauthorized")
		},
	}

	service := NewService(Config{
		AIConfig: &config.AIConfig{
			ChatModel:        "openrouter:openai/gpt-4o-mini",
			DiscoveryModel:   "gemini:gemini-test",
			OpenRouterAPIKey: "sk-or-test",
			GeminiAPIKey:     "gemini-test",
		},
		StateProvider: &mockStateProvider{},
	})
	service.sessions = store
	service.provider = primary
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		fallbackCalled.Store(true)
		return nil, errors.New("unexpected model " + model)
	}
	service.started = true

	var content strings.Builder
	var errorEvents int
	var fallbackEvents int
	var doneModel string
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "no-fallback-before-visible-output",
		Prompt:    "reply",
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal content: %v", err)
			}
			content.WriteString(data.Text)
		case "error":
			errorEvents++
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal workflow state: %v", err)
			}
			if data.Phase == "provider_fallback" {
				fallbackEvents++
			}
		case "done":
			var data DoneData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal done: %v", err)
			}
			doneModel = data.Model
		}
	})
	if err == nil {
		t.Fatal("expected selected provider error")
	}
	if got := content.String(); got != "" {
		t.Fatalf("content = %q, want no hidden alternate-route response", got)
	}
	if errorEvents != 1 {
		t.Fatalf("error events = %d, want selected provider failure event", errorEvents)
	}
	if fallbackEvents != 0 {
		t.Fatalf("fallback workflow events = %d, want 0", fallbackEvents)
	}
	if doneModel != "" {
		t.Fatalf("done model = %q, want no done event", doneModel)
	}
	if fallbackCalled.Load() {
		t.Fatal("providerFactory should not be called for a hidden alternate route")
	}

	messages, err := store.GetMessages("no-fallback-before-visible-output")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	for _, msg := range messages {
		if strings.Contains(msg.Content, "unauthorized") {
			t.Fatalf("stored messages leaked hidden provider error: %#v", messages)
		}
	}
}

func TestService_ExecuteStream_DoesNotFallbackToSameModelGatewayRoute(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var primaryCalls atomic.Int32
	var providerFactoryCalls atomic.Int32
	primary := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			primaryCalls.Add(1)
			return errors.New("dial tcp: i/o timeout")
		},
	}

	service := NewService(Config{
		AIConfig: &config.AIConfig{
			ChatModel:        "deepseek:deepseek-v4-pro",
			OpenRouterAPIKey: "sk-or-test",
			DeepSeekAPIKey:   "deepseek-test",
		},
		StateProvider: &mockStateProvider{},
	})
	service.sessions = store
	service.provider = primary
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		providerFactoryCalls.Add(1)
		return nil, errors.New("unexpected model " + model)
	}
	service.started = true

	var content strings.Builder
	var fallbackEvents int
	var doneModel string
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "no-fallback-to-same-model-gateway",
		Prompt:    "reply",
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal content: %v", err)
			}
			content.WriteString(data.Text)
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal workflow state: %v", err)
			}
			if data.Phase == "provider_fallback" {
				fallbackEvents++
			}
		case "done":
			var data DoneData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal done: %v", err)
			}
			doneModel = data.Model
		}
	})
	if err == nil {
		t.Fatal("expected selected provider error")
	}
	if got := content.String(); got != "" {
		t.Fatalf("content = %q, want no gateway fallback response", got)
	}
	if fallbackEvents != 0 {
		t.Fatalf("fallback workflow events = %d, want 0", fallbackEvents)
	}
	if doneModel != "" {
		t.Fatalf("done model = %q, want no done event", doneModel)
	}
	if got := primaryCalls.Load(); got != 2 {
		t.Fatalf("primary provider calls = %d, want two selected-route retry calls", got)
	}
	if got := providerFactoryCalls.Load(); got != 0 {
		t.Fatalf("provider factory calls = %d, want no automatic gateway provider", got)
	}
}

func TestService_ChatProviderAttemptUsesSelectedRouteOnly(t *testing.T) {
	service := &Service{}

	attempt, err := service.chatProviderAttempt(
		"openrouter:qwen/qwen3.7-plus",
		"openrouter:qwen/qwen3.7-plus",
		&stubServiceProvider{},
	)
	if err != nil {
		t.Fatalf("chatProviderAttempt failed: %v", err)
	}
	if attempt.Model != "openrouter:qwen/qwen3.7-plus" {
		t.Fatalf("attempt model = %q, want selected route", attempt.Model)
	}
	if attempt.Provider == nil {
		t.Fatal("selected configured route should reuse the configured provider")
	}
}

func TestService_ChatProviderAttemptDoesNotAddGatewayEquivalentRoute(t *testing.T) {
	service := &Service{}

	attempt, err := service.chatProviderAttempt(
		"deepseek:deepseek-v4-pro",
		"deepseek:deepseek-v4-pro",
		&stubServiceProvider{},
	)
	if err != nil {
		t.Fatalf("chatProviderAttempt failed: %v", err)
	}
	if attempt.Model != "deepseek:deepseek-v4-pro" {
		t.Fatalf("attempt model = %q, want selected route only", attempt.Model)
	}
	if attempt.Provider == nil {
		t.Fatal("selected configured route should reuse the configured provider")
	}
}

func TestService_ExecuteStream_UsesStableChatDefaultWithoutLiveCatalog(t *testing.T) {
	var catalogCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			catalogCalls.Add(1)
		}
		http.Error(w, "model catalog should not be called during chat startup", http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	service := NewService(Config{
		AIConfig: &config.AIConfig{
			OpenAIAPIKey:  "sk-test",
			OpenAIBaseURL: server.URL + "/v1",
		},
	})
	service.sessions = store
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		if want := config.DefaultModelForProvider(config.AIProviderOpenAI); model != want {
			return nil, errors.New("unexpected model " + model)
		}
		return &stubServiceProvider{}, nil
	}
	service.provider = nil
	service.started = true

	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "stable-chat-default-no-catalog",
		Prompt:    "hello",
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if calls := catalogCalls.Load(); calls != 0 {
		t.Fatalf("chat startup called provider model catalog %d times", calls)
	}
}

func TestService_ExecuteStream_DoesNotFallbackAfterVisibleProviderOutput(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	primary := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "partial"},
			})
			return errors.New("API error (503): upstream unavailable")
		},
	}
	fallbackCalled := false

	service := NewService(Config{
		AIConfig: &config.AIConfig{
			ChatModel:        "openrouter:openai/gpt-4o-mini",
			DiscoveryModel:   "gemini:gemini-test",
			OpenRouterAPIKey: "sk-or-test",
			GeminiAPIKey:     "gemini-test",
		},
		StateProvider: &mockStateProvider{},
	})
	service.sessions = store
	service.provider = primary
	service.providerFactory = func(model string) (providers.StreamingProvider, error) {
		fallbackCalled = true
		return &stubServiceProvider{}, nil
	}
	service.started = true

	var content strings.Builder
	var errorEvents int
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "no-fallback-after-visible-output",
		Prompt:    "reply",
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal content: %v", err)
			}
			content.WriteString(data.Text)
		case "error":
			errorEvents++
		}
	})
	if err == nil {
		t.Fatal("expected provider error after visible output")
	}
	if got := content.String(); got != "partial" {
		t.Fatalf("content = %q, want primary partial output only", got)
	}
	if errorEvents != 1 {
		t.Fatalf("error events = %d, want 1", errorEvents)
	}
	if fallbackCalled {
		t.Fatal("fallback provider should not be called after visible primary output")
	}
}

func TestService_ExecuteStream_InteractiveChatLetsModelChooseTools(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	sawTools := false
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			sawTools = len(req.Tools) > 0
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Assistant is ready."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	exploreEvents := 0
	unexpectedWorkflowEvents := 0
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "interactive-chat-model-choice",
		Prompt:    "show me the logs for vm 100",
	}, func(event StreamEvent) {
		switch event.Type {
		case "explore_status":
			exploreEvents++
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal workflow state: %v", err)
			}
			if data.Phase != "prepare" && data.Phase != "provider_start" {
				unexpectedWorkflowEvents++
			}
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if !sawTools {
		t.Fatal("expected interactive chat to expose governed tools and let the model choose")
	}
	if exploreEvents != 0 {
		t.Fatalf("did not expect Pulse-owned explore events before model choice, got %d", exploreEvents)
	}
	if unexpectedWorkflowEvents != 0 {
		t.Fatalf("did not expect Pulse-owned workflow events before model choice, got %d", unexpectedWorkflowEvents)
	}
}

func TestService_ExecuteStream_InventoryCountAnswersFromCanonicalStateWithoutProvider(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var providerCalled atomic.Bool
	var capturedProviderTools []providers.Tool
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			providerCalled.Store(true)
			capturedProviderTools = append([]providers.Tool(nil), req.Tools...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "provider-backed count"},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}
	state := models.StateSnapshot{
		Nodes: []models.Node{{
			ID:       "node1",
			Name:     "node1",
			Instance: "pve1",
			Status:   "online",
		}},
		VMs: []models.VM{{
			ID:       "qemu/pve1/node1/100",
			Name:     "vm1",
			VMID:     100,
			Instance: "pve1",
			Status:   "running",
			Node:     "node1",
		}},
		Containers: []models.Container{{
			ID:       "lxc/pve1/node1/201",
			Name:     "ct1",
			VMID:     201,
			Instance: "pve1",
			Status:   "running",
			Node:     "node1",
		}},
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-1",
			Hostname: "docker-host-1",
			Containers: []models.DockerContainer{{
				ID:     "docker-container-1",
				Name:   "nginx",
				State:  "running",
				Image:  "nginx",
				Health: "healthy",
			}},
		}},
		KubernetesClusters: []models.KubernetesCluster{{
			ID:     "cluster-1",
			Name:   "prod-cluster",
			Status: "online",
			Nodes: []models.KubernetesNode{{
				Name:  "worker-1",
				UID:   "node-1",
				Ready: true,
				Roles: []string{"worker"},
			}},
			Pods: []models.KubernetesPod{{
				UID:       "pod-1",
				Name:      "api-6f8d5c",
				Namespace: "default",
				Phase:     "Running",
				Restarts:  2,
				OwnerKind: "Deployment",
				OwnerName: "api",
			}},
			Deployments: []models.KubernetesDeployment{{
				UID:             "deploy-1",
				Name:            "api",
				Namespace:       "default",
				DesiredReplicas: 3,
				ReadyReplicas:   2,
			}},
		}},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state)

	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{state: state},
		ReadState:     unifiedresources.NewMonitorAdapter(registry),
		AgentServer:   &mockAgentServer{},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	var content strings.Builder
	var doneSeen bool
	var doneModel string
	var countedProgressSeen bool
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "inventory-count-fast-path",
		Prompt:    "how many devices in this",
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal content: %v", err)
			}
			content.WriteString(data.Text)
		case "done":
			doneSeen = true
			var data DoneData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal done: %v", err)
			}
			doneModel = data.Model
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal workflow state: %v", err)
			}
			if data.Phase == "context_ready" && data.Message == "Counted current Pulse inventory." {
				countedProgressSeen = true
			}
		}
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if providerCalled.Load() {
		t.Fatal("expected deterministic inventory count to avoid provider call")
	}
	if !doneSeen {
		t.Fatal("done event was not emitted")
	}
	if doneModel != assistantLocalInventoryModelRoute {
		t.Fatalf("expected local inventory completion model, got %q", doneModel)
	}
	if !countedProgressSeen {
		t.Fatal("inventory count progress event was not emitted")
	}
	answer := content.String()
	for _, want := range []string{
		"Pulse currently sees 9 visible inventory items",
		"3 compute resources",
		"1 Proxmox node",
		"1 VM",
		"1 system container",
		"2 Docker inventory items",
		"1 Docker host",
		"1 Docker container",
		"4 Kubernetes inventory items",
		"1 cluster",
		"1 node",
		"1 deployment",
		"1 pod",
	} {
		if !strings.Contains(answer, want) {
			t.Fatalf("expected direct inventory answer to contain %q, got %q", want, answer)
		}
	}
	messages, err := store.GetMessages("inventory-count-fast-path")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 || messages[1].Role != "assistant" || messages[1].Content != answer {
		t.Fatalf("expected direct assistant answer to be persisted, got %#v", messages)
	}

	providerCalled.Store(false)
	capturedProviderTools = nil
	content.Reset()
	doneSeen = false
	doneModel = ""
	countedProgressSeen = false
	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "inventory-count-explicit-command",
		Prompt:    "Use read-only tools only. On node delly, count entries in /dev with `ls /dev | wc -l`; answer with the number only.",
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal explicit content: %v", err)
			}
			content.WriteString(data.Text)
		case "done":
			doneSeen = true
			var data DoneData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal explicit done: %v", err)
			}
			doneModel = data.Model
		case "workflow_state":
			var data WorkflowStateData
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("unmarshal explicit workflow state: %v", err)
			}
			if data.Phase == "context_ready" && data.Message == "Counted current Pulse inventory." {
				countedProgressSeen = true
			}
		}
	})
	if err != nil {
		t.Fatalf("explicit command ExecuteStream failed: %v", err)
	}
	if !providerCalled.Load() {
		t.Fatal("expected explicit command count to reach provider")
	}
	if !doneSeen {
		t.Fatal("explicit command turn did not emit done")
	}
	if doneModel == assistantLocalInventoryModelRoute {
		t.Fatalf("explicit command count used local inventory route %q", doneModel)
	}
	if countedProgressSeen {
		t.Fatal("explicit command count should not emit deterministic local inventory progress")
	}
	if got := content.String(); !strings.Contains(got, "provider-backed count") {
		t.Fatalf("expected provider-backed content, got %q", got)
	}
	set := toolNameSet(capturedProviderTools)
	if !set["pulse_read"] {
		t.Fatalf("expected explicit command count to expose pulse_read, got %#v", set)
	}
}

func TestService_ExecuteStream_PrefetchedInventoryIncludesCompleteTopologyDetail(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var capturedReq providers.ChatRequest
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedReq = req
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Inventory breakdown ready."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	state := models.StateSnapshot{
		Nodes: []models.Node{{
			ID:       "node1",
			Name:     "node1",
			Instance: "pve1",
			Status:   "online",
		}},
		VMs: []models.VM{{
			ID:       "qemu/pve1/node1/100",
			Name:     "vm1",
			VMID:     100,
			Instance: "pve1",
			Status:   "running",
			Node:     "node1",
		}},
	}
	for i := 1; i <= 13; i++ {
		status := "running"
		if i == 1 {
			status = "stopped"
		}
		state.Containers = append(state.Containers, models.Container{
			ID:       fmt.Sprintf("lxc/pve1/node1/%d", 200+i),
			Name:     fmt.Sprintf("ct%d", i),
			VMID:     200 + i,
			Instance: "pve1",
			Status:   status,
			Node:     "node1",
		})
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state)
	service := NewService(Config{
		AIConfig:      &config.AIConfig{ChatModel: "openai:gpt-4o", ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{state: state},
		ReadState:     unifiedresources.NewMonitorAdapter(registry),
		AgentServer:   &mockAgentServer{},
	})
	service.sessions = store
	service.provider = provider
	service.unifiedResourceProvider = handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeVM: {{
			ID:       "vm-100",
			Type:     unifiedresources.ResourceTypeVM,
			Name:     "vm1",
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"vm1"}},
			Proxmox:  &unifiedresources.ProxmoxData{VMID: 100, NodeName: "node1"},
		}},
		unifiedresources.ResourceTypeSystemContainer: {{
			ID:       "ct-201",
			Type:     unifiedresources.ResourceTypeSystemContainer,
			Name:     "ct1",
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"ct1"}},
			Proxmox:  &unifiedresources.ProxmoxData{VMID: 201, NodeName: "node1"},
		}, {
			ID:       "ct-213",
			Type:     unifiedresources.ResourceTypeSystemContainer,
			Name:     "ct13",
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"ct13"}},
			Proxmox:  &unifiedresources.ProxmoxData{VMID: 213, NodeName: "node1"},
		}},
	}}
	service.started = true

	err = service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "inventory-detail-fast-path",
		Prompt:    "give me a detailed inventory breakdown by node",
	}, func(event StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if len(capturedReq.Tools) != 0 {
		t.Fatalf("expected prefetched inventory detail turn to withhold tools, got %d", len(capturedReq.Tools))
	}
	if len(capturedReq.Messages) == 0 {
		t.Fatal("expected provider request messages")
	}
	lastMessage := capturedReq.Messages[len(capturedReq.Messages)-1].Content
	for _, want := range []string{
		"per-node and workload detail",
		"answer_label fields are UI-visible inventory labels",
		"Do not shorten answer_label values",
		"includes every Proxmox node, VM, system container",
		`"proxmox":{"nodes":[`,
		`"nodes":[`,
		`"answer_label":"Node node1"`,
		`"name":"node1"`,
		`"vms":[`,
		`"answer_label":"VM 100 vm1"`,
		`"name":"vm1"`,
		`"containers":[`,
		`"answer_label":"CT 201 ct1"`,
		`"name":"ct1"`,
		`"answer_label":"CT 213 ct13"`,
		`"name":"ct13"`,
		"User message: give me a detailed inventory breakdown by node",
	} {
		if !strings.Contains(lastMessage, want) {
			t.Fatalf("expected prefetched inventory context to contain %q, got %q", want, lastMessage)
		}
	}
	for _, forbidden := range []string{
		`"policy":`,
		`"ai_safe_summary"`,
		`"routing":`,
		`"sensitivity":`,
		`redacted`,
	} {
		if strings.Contains(lastMessage, forbidden) {
			t.Fatalf("expected prefetched inventory context to omit policy metadata %q, got %q", forbidden, lastMessage)
		}
	}
}

func TestResourceContextTurnIsContextOnly(t *testing.T) {
	resources := []HandoffResource{{ID: "system-container:ha-node:101", Name: "homeassistant", Type: "system-container"}}
	metadata := HandoffMetadata{Kind: "resource_context"}

	tests := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name:   "context summary defaults to no tools",
			prompt: "What does Pulse already know about this resource?",
			want:   true,
		},
		{
			name:   "explicit no tools wins",
			prompt: "Before using any tools, tell me the discovery readiness.",
			want:   true,
		},
		{
			name:   "natural automation question stays context first",
			prompt: "How long before my blinds automation runs?",
			want:   true,
		},
		{
			name:   "explicit read attempt allows tools",
			prompt: "Make one read-only pulse_read attempt against the attached resource.",
			want:   false,
		},
		{
			name:   "explicit discovery request allows tools",
			prompt: "Run discovery for this resource now.",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceContextTurnIsContextOnly(tt.prompt, resources, metadata); got != tt.want {
				t.Fatalf("resourceContextTurnIsContextOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_ExecuteStream_ResourceContextToolManifestPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	var toolCounts []int
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			toolCounts = append(toolCounts, len(req.Tools))
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "Assistant is ready."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	service := NewService(Config{
		AIConfig:      &config.AIConfig{ControlLevel: config.ControlLevelControlled},
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
	})
	service.sessions = store
	service.provider = provider
	service.started = true

	handoffResources := []HandoffResource{{
		ID:   "system-container:ha-node:101",
		Name: "homeassistant",
		Type: "system-container",
		Node: "ha-node",
	}}
	handoffMetadata := HandoffMetadata{Kind: "resource_context"}

	if err := service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID:        "resource-context-tool-policy",
		Prompt:           "What does Pulse already know about this resource?",
		HandoffResources: handoffResources,
		HandoffMetadata:  handoffMetadata,
	}, func(StreamEvent) {}); err != nil {
		t.Fatalf("context-only ExecuteStream failed: %v", err)
	}
	if err := service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID:       "resource-context-tool-policy",
		Prompt:          "Before using any tools, tell me whether Pulse has fresh discovery data for this attached resource.",
		HandoffMetadata: handoffMetadata,
	}, func(StreamEvent) {}); err != nil {
		t.Fatalf("metadata-only follow-up ExecuteStream failed: %v", err)
	}
	storedResources, err := store.GetModelHandoffResources("resource-context-tool-policy")
	if err != nil {
		t.Fatalf("GetModelHandoffResources failed: %v", err)
	}
	if len(storedResources) != 1 || storedResources[0].ID != handoffResources[0].ID {
		t.Fatalf("stored handoff resources = %#v, want preserved resource scope", storedResources)
	}
	if err := service.ExecuteStream(context.Background(), ExecuteRequest{
		SessionID: "resource-context-tool-policy",
		Prompt:    "Make one read-only pulse_read attempt against the attached resource.",
	}, func(StreamEvent) {}); err != nil {
		t.Fatalf("explicit-read ExecuteStream failed: %v", err)
	}

	if len(toolCounts) != 3 {
		t.Fatalf("toolCounts = %#v, want three provider calls", toolCounts)
	}
	if toolCounts[0] != 0 {
		t.Fatalf("context-only turn exposed %d tools, want 0", toolCounts[0])
	}
	if toolCounts[1] != 0 {
		t.Fatalf("metadata-only follow-up exposed %d tools, want 0", toolCounts[1])
	}
	if toolCounts[2] == 0 {
		t.Fatal("explicit read turn exposed no tools")
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
	if strings.Contains(stored[0].Content, "Resource Context Pack") {
		t.Fatalf("stored user message should not include resource context pack: %q", stored[0].Content)
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
	if !strings.Contains(modelUserContent, "[Resource Context Pack]") {
		t.Fatalf("model user content missing shared resource context pack: %q", modelUserContent)
	}
	if !strings.Contains(modelUserContent, "Context Pack Boundary: Resource context facts are read-only") {
		t.Fatalf("model user content missing shared context-pack boundary: %q", modelUserContent)
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

func TestBuildHandoffResourceContextPackUsesSharedDiscoverySections(t *testing.T) {
	now := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	parentID := "agent:pve-1"
	provider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeAppContainer: {{
			ID:         "app-container:homeassistant",
			Type:       unifiedresources.ResourceTypeAppContainer,
			Name:       "homeassistant",
			Status:     unifiedresources.StatusOnline,
			Technology: "docker",
			UpdatedAt:  now.Add(-1 * time.Minute),
			ParentID:   &parentID,
			ParentName: "ha-lxc",
			DiscoveryTarget: &unifiedresources.DiscoveryTarget{
				ResourceType: "app-container",
				AgentID:      "agent:pve-1",
				ResourceID:   "homeassistant",
				Hostname:     "homeassistant.local",
			},
			Docker: &unifiedresources.DockerData{
				Runtime:        "docker",
				RuntimeVersion: "27.0",
				ContainerState: "running",
				Health:         "healthy",
				Ports: []unifiedresources.DockerPortMeta{
					{PrivatePort: 8123, PublicPort: 8123, Protocol: "tcp"},
				},
				Mounts: []unifiedresources.DockerMountMeta{
					{Source: "/srv/homeassistant/secret-config", Destination: "/config", RW: true},
				},
				Labels: map[string]string{
					"com.example.env": "TOKEN=should-not-leak",
				},
			},
			Capabilities: []unifiedresources.ResourceCapability{{
				Name:                 "restart_container",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
				Platform:             "docker",
				Params: []unifiedresources.CapabilityParam{{
					Name:         "token",
					IsSensitive:  true,
					DefaultValue: "secret-token",
				}},
			}},
		}},
	}}
	store := unifiedresources.NewMemoryStore()
	if err := store.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-ha-restart",
		ResourceID:    "app-container:homeassistant",
		ObservedAt:    now.Add(-30 * time.Second),
		Kind:          unifiedresources.ChangeRestart,
		From:          "running",
		To:            "restarted",
		SourceType:    unifiedresources.SourcePulseDiff,
		SourceAdapter: unifiedresources.AdapterDocker,
		Confidence:    unifiedresources.ConfidenceHigh,
		Reason:        "container restarted after update",
	}); err != nil {
		t.Fatalf("RecordChange failed: %v", err)
	}

	contextText := buildHandoffResourceContextPack([]HandoffResource{{
		ID:   "app-container:homeassistant",
		Name: "homeassistant",
		Type: "app-container",
		Node: "ha-lxc",
	}}, provider, store, now)

	for _, expected := range []string{
		"[Resource Context Pack]",
		"Resource Context: homeassistant (app-container)",
		"Context Section: Runtime and Discovery",
		"Context Fact: Runtime and Discovery / Ports: 8123:8123/tcp",
		"Context Fact: Runtime and Discovery / Mounts: 1",
		"Context Fact: Safety and Operations / Capability 1: restart_container; approval=admin; platform=docker; 1 sensitive params hidden",
		"Context Fact: Recent Changes / Change 1: restart; running -> restarted; reason=container restarted after update",
		"Context Pack Boundary: Resource context facts are read-only",
	} {
		if !strings.Contains(contextText, expected) {
			t.Fatalf("resource context pack missing %q: %q", expected, contextText)
		}
	}

	for _, forbidden := range []string{
		"/srv/homeassistant/secret-config",
		"/config",
		"TOKEN=should-not-leak",
		"secret-token",
	} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("resource context pack leaked raw unsafe detail %q: %q", forbidden, contextText)
		}
	}
}

func TestService_ExecuteStream_ResourceContextHandoffDirectiveAndOutputRedaction(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	resource := unifiedresources.Resource{
		ID:        "system-container:ha-node:101",
		Type:      unifiedresources.ResourceTypeSystemContainer,
		Name:      "homeassistant",
		Status:    unifiedresources.StatusOnline,
		UpdatedAt: time.Date(2026, 6, 4, 16, 0, 0, 0, time.UTC),
		Tags:      []string{"sensitive"},
		Identity: unifiedresources.ResourceIdentity{
			Hostnames: []string{"ha.internal"},
		},
		Storage: &unifiedresources.StorageMeta{
			Path: "/var/lib/homeassistant",
		},
		DiscoveryTarget: &unifiedresources.DiscoveryTarget{
			ResourceType: "system-container",
			AgentID:      "ha-node",
			ResourceID:   "101",
			Hostname:     "ha.internal",
		},
	}
	unifiedProvider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeSystemContainer: {resource},
	}}

	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: unifiedProvider})
	var capturedMessages []providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages = append([]providers.Message(nil), req.Messages...)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "I found /var/lib/homeassistant on ha.internal."},
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
		unifiedResourceProvider: unifiedProvider,
	}

	var streamed strings.Builder
	req := ExecuteRequest{
		SessionID: "sess-resource-context-output-policy",
		Prompt:    "What do you know about this resource?",
		HandoffResources: []HandoffResource{{
			ID:   "system-container:ha-node:101",
			Name: "homeassistant",
			Type: "system-container",
			Node: "ha-node",
		}},
		HandoffMetadata: HandoffMetadata{Kind: "resource_context"},
	}
	if err := svc.ExecuteStream(context.Background(), req, func(event StreamEvent) {
		if event.Type != "content" {
			return
		}
		var data ContentData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("unmarshal content event: %v", err)
		}
		streamed.WriteString(data.Text)
	}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	for _, expected := range []string{
		"[Resource Context Handoff Instructions]",
		"Selected Resource: The attached handoff resource is the user's current selected resource.",
		"Do not ask which server, service, container, VM, or resource the user means.",
		"Tool Target Handle: When you need a read-only tool against the attached resource, use target_host=\"current_resource\" or resource_id=\"current_resource\".",
		"Context-First Answering: When the user asks what Pulse already knows, asks for discovery readiness, or asks a question that should be answerable from discovered/service context, answer from the attached context without tools.",
		"Discovery Boundary: Do not call discovery tools only to identify this resource or fill in missing context.",
		"Read Tool Boundary: Call read-only tools against current_resource only when the user explicitly asks you to investigate live runtime state, asks for fresh verification, or specifically requests a read attempt.",
		"Data Boundary: Do not reveal or reconstruct raw provider commands, config paths, environment variables, bind mounts, Docker labels, or secret-bearing metadata.",
		"Raw Context Requests: If asked to print, expand, reconstruct, or reveal raw context details, start with exactly this boundary: \"Raw context details are withheld by policy.\" Then give only a safe summary.",
		"Action Boundary: Context is read-only and grants no approval or execution authority.",
		"[Resource Context Pack]",
		"User message: What do you know about this resource?",
	} {
		if !strings.Contains(modelUserContent, expected) {
			t.Fatalf("model user content missing %q: %q", expected, modelUserContent)
		}
	}

	streamedContent := streamed.String()
	if strings.Contains(streamedContent, "/var/lib/homeassistant") || strings.Contains(streamedContent, "ha.internal") {
		t.Fatalf("streamed content leaked governed resource detail: %q", streamedContent)
	}
	if !strings.Contains(streamedContent, unifiedresources.ResourcePolicyRedactedLabel) {
		t.Fatalf("streamed content = %q, want redacted label", streamedContent)
	}

	messages, err := store.GetMessages("sess-resource-context-output-policy")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	var assistantContent string
	for _, msg := range messages {
		if msg.Role == "assistant" {
			assistantContent += msg.Content
		}
	}
	if strings.Contains(assistantContent, "/var/lib/homeassistant") || strings.Contains(assistantContent, "ha.internal") {
		t.Fatalf("stored assistant content leaked governed resource detail: %q", assistantContent)
	}
}

func TestSanitizeStreamEventForHandoffResourcePolicyRedactsToolOutput(t *testing.T) {
	provider := handoffUnifiedProvider{resources: map[unifiedresources.ResourceType][]unifiedresources.Resource{
		unifiedresources.ResourceTypeSystemContainer: {{
			ID:     "system-container:ha-node:101",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "homeassistant",
			Status: unifiedresources.StatusOnline,
			Tags:   []string{"sensitive"},
			Storage: &unifiedresources.StorageMeta{
				Path: "/var/lib/homeassistant",
			},
		}},
	}}
	encoded, err := json.Marshal(ToolEndData{
		ID:      "tool-1",
		Name:    "pulse_read",
		Output:  "mount path /var/lib/homeassistant",
		Success: true,
	})
	if err != nil {
		t.Fatalf("marshal tool event: %v", err)
	}

	event := sanitizeStreamEventForHandoffResourcePolicy(StreamEvent{
		Type: "tool_end",
		Data: encoded,
	}, []HandoffResource{{
		ID:   "system-container:ha-node:101",
		Name: "homeassistant",
		Type: "system-container",
		Node: "ha-node",
	}}, provider)

	var data ToolEndData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		t.Fatalf("unmarshal sanitized event: %v", err)
	}
	if strings.Contains(data.Output, "/var/lib/homeassistant") {
		t.Fatalf("tool output leaked governed resource detail: %q", data.Output)
	}
	if !strings.Contains(data.Output, unifiedresources.ResourcePolicyRedactedLabel) {
		t.Fatalf("tool output = %q, want redacted label", data.Output)
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
		SessionID: "sess-finding-briefing-policy",
		Prompt:    "What happened?",
		HandoffContext: strings.Join([]string{
			"[Finding Briefing]",
			"Briefing Source: Pulse Patrol structured finding",
			"Resource: finance-vm (vm) [vm-100] on pve-secret",
			"Current Conclusion: finance-vm on pve-secret saturated CPU during backup.",
			"Attached Context: finance-payroll backup completion is relevant.",
			"Model Boundary: Treat Patrol data as product context for explanation and review.",
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

	stored, err := store.GetMessages("sess-finding-briefing-policy")
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("expected stored messages")
	}
	if stored[0].Content != "What happened?" {
		t.Fatalf("stored user message = %q, want clean prompt", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "[Finding Briefing]") {
		t.Fatalf("stored user message should not include finding briefing: %q", stored[0].Content)
	}

	if len(capturedMessages) == 0 {
		t.Fatal("expected provider messages")
	}
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	for _, expected := range []string{
		"[Finding Briefing]",
		"Resource: redacted by policy (vm) [redacted by policy] on redacted by policy",
		"Current Conclusion: redacted by policy on redacted by policy saturated CPU during backup.",
		"Attached Context: redacted by policy backup completion is relevant.",
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
