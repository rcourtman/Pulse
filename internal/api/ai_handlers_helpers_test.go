package api

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
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

func TestIsMCPToolCall(t *testing.T) {
	if !isMCPToolCall("pulse_control_guest(guest_id='102')") {
		t.Fatalf("expected MCP tool call to be detected")
	}
	if !isMCPToolCall("default_api:pulse_get_resource(id='1')") {
		t.Fatalf("expected MCP tool call with default_api prefix")
	}
	if isMCPToolCall("echo hello") {
		t.Fatalf("expected non-tool command to be false")
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

func TestSplitToolArgs(t *testing.T) {
	args := "action='start', guest_id=\"102\", note='hello, world', path=\"/tmp/a,b\", escaped=\"\\\"quote\\\"\""
	parts := splitToolArgs(args)
	expected := []string{
		"action='start'",
		"guest_id=\"102\"",
		"note='hello, world'",
		"path=\"/tmp/a,b\"",
		"escaped=\"\\\"quote\\\"\"",
	}
	if len(parts) != len(expected) {
		t.Fatalf("expected %d parts, got %d", len(expected), len(parts))
	}
	for i := range expected {
		if strings.TrimSpace(parts[i]) != expected[i] {
			t.Fatalf("expected part %q, got %q", expected[i], parts[i])
		}
	}
}

func TestParseMCPToolCall(t *testing.T) {
	tool, args, err := parseMCPToolCall("default_api:pulse_control_guest(guest_id=\"102\", action='start')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "pulse_control_guest" {
		t.Fatalf("expected tool name pulse_control_guest, got %q", tool)
	}
	if args["guest_id"] != "102" || args["action"] != "start" {
		t.Fatalf("unexpected args: %#v", args)
	}

	tool, args, err = parseMCPToolCall("pulse_run_command()")
	if err != nil {
		t.Fatalf("unexpected error for empty args: %v", err)
	}
	if tool != "pulse_run_command" || len(args) != 0 {
		t.Fatalf("expected empty args, got %#v", args)
	}

	if _, _, err = parseMCPToolCall("pulse_control_guest"); err == nil {
		t.Fatalf("expected error for missing parenthesis")
	}
	if _, _, err = parseMCPToolCall("pulse_control_guest("); err == nil {
		t.Fatalf("expected error for missing closing parenthesis")
	}
}

func TestMCPToolAdapter_Errors(t *testing.T) {
	adapter := &mcpToolAdapter{handler: &AISettingsHandler{}}
	if _, _, err := adapter.ExecuteMCPTool(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat handler is missing")
	}

	adapter.handler.chatHandler = &AIHandler{}
	if _, _, err := adapter.ExecuteMCPTool(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat service is missing")
	}

	adapter.handler.chatHandler.defaultService = &fakeChatWrapper{}
	if _, _, err := adapter.ExecuteMCPTool(context.Background(), "pulse_control_guest()", ""); err == nil {
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

func TestAISettingsHandler_IsAIEnabled(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	if handler.IsAIEnabled(context.Background()) {
		t.Fatalf("expected AI to be disabled by default")
	}
}
