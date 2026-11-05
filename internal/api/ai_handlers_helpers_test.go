package api

import (
	"context"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubStateProvider struct{}

func (s *stubStateProvider) ReadSnapshot() models.StateSnapshot {
	return models.StateSnapshot{}
}

type fakeChatWrapper struct {
	*chat.Service
}

func newTestAISettingsHandlerLite() *AISettingsHandler {
	return &AISettingsHandler{
		defaultAIService: ai.NewService(nil, nil),
		aiServices:       make(map[string]*ai.Service),
	}
}

func TestCleanTargetHost(t *testing.T) {
	if got := cleanTargetHost("pve-node (The container's host is 'pve-node')"); got != "pve-node" {
		t.Fatalf("expected cleaned host, got %q", got)
	}
	if got := cleanTargetHost("pve-node extra"); got != "pve-node" {
		t.Fatalf("expected first token, got %q", got)
	}
	if got := cleanTargetHost("  pve-node "); got != "pve-node" {
		t.Fatalf("expected trimmed host, got %q", got)
	}
	if got := cleanTargetHost(""); got != "" {
		t.Fatalf("expected empty host")
	}
}

func TestAssistantToolAdapter_Errors(t *testing.T) {
	adapter := &assistantToolAdapter{handler: &AISettingsHandler{}}
	if _, _, err := adapter.ExecuteApprovedAssistantTool(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat handler is missing")
	}

	adapter.handler.chatHandler = &AIHandler{}
	if _, _, err := adapter.ExecuteApprovedAssistantTool(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat service is missing")
	}

	adapter.handler.chatHandler.defaultService = &fakeChatWrapper{}
	if _, _, err := adapter.ExecuteApprovedAssistantTool(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error for chat service type mismatch")
	}
}

func TestAISettingsHandler_Setters(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	stateProvider := &stubStateProvider{}
	handler.SetStateProvider(stateProvider)
	if handler.GetStateProvider() != stateProvider {
		t.Fatalf("expected state provider to be set")
	}

	breaker := &circuit.Breaker{}
	handler.SetCircuitBreaker(breaker)
	if handler.GetCircuitBreaker() != breaker {
		t.Fatalf("expected circuit breaker to be set")
	}

	learningStore := &learning.LearningStore{}
	handler.SetLearningStore(learningStore)
	if handler.GetLearningStore() != learningStore {
		t.Fatalf("expected learning store to be set")
	}

	forecastSvc := &forecast.Service{}
	handler.SetForecastService(forecastSvc)
	if handler.GetForecastService() != forecastSvc {
		t.Fatalf("expected forecast service to be set")
	}

	correlator := &proxmox.EventCorrelator{}
	handler.SetProxmoxCorrelator(correlator)
	if handler.GetProxmoxCorrelator() != correlator {
		t.Fatalf("expected correlator to be set")
	}

	engine := newTestRemediationEngine()
	handler.SetRemediationEngine(engine)
	if handler.GetRemediationEngine() != engine {
		t.Fatalf("expected remediation engine to be set")
	}
}

func TestAISettingsHandler_RemoveTenantService(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	handler.aiServices["org-1"] = ai.NewService(nil, nil)
	handler.aiServices["default"] = ai.NewService(nil, nil)

	handler.RemoveTenantService("org-1")
	if _, ok := handler.aiServices["org-1"]; ok {
		t.Fatalf("expected org-1 to be removed")
	}

	handler.RemoveTenantService("default")
	if _, ok := handler.aiServices["default"]; !ok {
		t.Fatalf("expected default to remain")
	}
}

func TestAISettingsHandler_RemoveTenantServiceClosesItsResourceStore(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	handler.aiServices["org-1"] = ai.NewService(config.NewConfigPersistence(t.TempDir()), nil)
	before := countGoroutinesInStack(t, "SQLiteResourceStore).startRetentionLoop")
	if before == 0 {
		t.Fatal("expected the tenant service to hold an open resource store")
	}

	handler.RemoveTenantService("org-1")

	deadline := time.Now().Add(5 * time.Second)
	for countGoroutinesInStack(t, "SQLiteResourceStore).startRetentionLoop") >= before {
		if time.Now().After(deadline) {
			t.Fatal("removed tenant service left its resource store open")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// countGoroutinesInStack counts goroutines whose stack mentions frame.
func countGoroutinesInStack(t *testing.T, frame string) int {
	t.Helper()
	var stacks strings.Builder
	if err := pprof.Lookup("goroutine").WriteTo(&stacks, 2); err != nil {
		t.Fatalf("read goroutine stacks: %v", err)
	}
	count := 0
	for _, goroutine := range strings.Split(stacks.String(), "\n\n") {
		if strings.Contains(goroutine, frame) {
			count++
		}
	}
	return count
}

func TestAISettingsHandler_IsAIEnabled(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	if handler.IsAIEnabled(context.Background()) {
		t.Fatalf("expected AI to be disabled by default")
	}
}
