package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAlertAnalyzerForTest satisfies aicontracts.AlertAnalyzer for test assertions.
type mockAlertAnalyzerForTest struct{ enabled bool }

func (m *mockAlertAnalyzerForTest) OnAlertFired(aicontracts.AlertPayload) {}
func (m *mockAlertAnalyzerForTest) SetEnabled(e bool)                     { m.enabled = e }
func (m *mockAlertAnalyzerForTest) IsEnabled() bool                       { return m.enabled }
func (m *mockAlertAnalyzerForTest) Start()                                {}
func (m *mockAlertAnalyzerForTest) Stop()                                 {}

func newTestAISettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, agentServer *agentexec.Server) *AISettingsHandler {
	handler := NewAISettingsHandler(nil, nil, agentServer)
	handler.defaultConfig = cfg
	handler.defaultPersistence = persistence
	if persistence != nil {
		handler.defaultAIService = ai.NewService(persistence, agentServer)
		_ = handler.defaultAIService.LoadConfig()
	}
	return handler
}

func saveEnabledTestAIConfig(t *testing.T, persistence *config.ConfigPersistence) {
	t.Helper()

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))
}

func TestAIHandlersUseSafeRemediationCommercialCopy(t *testing.T) {
	files := []string{"ai_handlers.go", "router_routes_ai_relay.go"}
	for _, file := range files {
		source, err := os.ReadFile(file)
		require.NoError(t, err)
		text := string(source)
		require.NotContains(t, text, "Pulse Patrol Auto-Fix requires Pulse Pro")
		require.NotContains(t, text, "Auto-Fix requires Pulse Pro")
	}
}

func TestAISettingsHandler_AnthropicOAuthSetupFailsClosed(t *testing.T) {
	handler := &AISettingsHandler{}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{
			name:   "start",
			method: http.MethodGet,
			path:   "/api/ai/oauth/start",
			call:   handler.HandleOAuthStart,
		},
		{
			name:   "exchange",
			method: http.MethodPost,
			path:   "/api/ai/oauth/exchange",
			body:   `{"code":"code","state":"state"}`,
			call:   handler.HandleOAuthExchange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			tt.call(rec, req)

			require.Equal(t, http.StatusNotImplemented, rec.Code, rec.Body.String())
			var resp struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.False(t, resp.Success)
			require.Equal(t, "unsupported_anthropic_oauth", resp.Error)
			require.Contains(t, resp.Message, "Configure an Anthropic API key")
		})
	}
}

func TestAIHandlersDelegateApprovedToolInvocationParsingToSharedCapabilities(t *testing.T) {
	source, err := os.ReadFile("ai_handlers.go")
	require.NoError(t, err)
	aiHandlers := string(source)
	for _, fragment := range []string{
		"isMCPToolCall",
		"parseMCPToolCall",
		"splitToolArgs",
	} {
		require.NotContains(t, aiHandlers, fragment)
	}

	source, err = os.ReadFile("router_routes_ai_relay.go")
	require.NoError(t, err)
	relay := string(source)
	require.Contains(t, relay, "agentcapabilities.ParseTextToolInvocation(command)")
	require.NotContains(t, relay, "func parseMCPToolCall(")
	require.NotContains(t, relay, "func splitToolArgs(")
	require.NotContains(t, relay, "func isMCPToolCall(")
}

func TestAIHandlersUseSharedPatrolFindingAgentErrorCodes(t *testing.T) {
	source, err := os.ReadFile("ai_handlers.go")
	require.NoError(t, err)
	aiHandlers := string(source)

	for _, constant := range []string{
		"agentcapabilities.AgentErrCodePatrolUnavailable",
		"agentcapabilities.AgentErrCodeInvalidFindingRequest",
		"agentcapabilities.AgentErrCodeFindingNotFound",
		"agentcapabilities.AgentErrCodeFindingActionNotAllowed",
	} {
		require.Contains(t, aiHandlers, constant)
	}

	for _, literal := range []string{
		`writeJSONError(w, http.StatusServiceUnavailable, "` + agentcapabilities.AgentErrCodePatrolUnavailable + `"`,
		`writeJSONErrorWithDetails(w, http.StatusBadRequest, "` + agentcapabilities.AgentErrCodeInvalidFindingRequest + `"`,
		`writeJSONError(w, http.StatusNotFound, "` + agentcapabilities.AgentErrCodeFindingNotFound + `"`,
		`writeJSONError(w, http.StatusConflict, "` + agentcapabilities.AgentErrCodeFindingActionNotAllowed + `"`,
	} {
		require.NotContains(t, aiHandlers, literal)
	}
}

func TestAISettingsHandler_PatrolFindingLifecycleUsesAgentStableUnavailableEnvelope(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	handler := newTestAISettingsHandler(cfg, nil, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/acknowledge", strings.NewReader(`{"finding_id":"finding-1"}`))
	rec := httptest.NewRecorder()
	handler.HandleAcknowledgeFinding(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	var resp agentStableErrorEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, agentcapabilities.AgentErrCodePatrolUnavailable, resp.Error)
	require.Equal(t, "Pulse Patrol service not available", resp.Message)
}

func TestAISettingsHandler_ResolveFindingPersistsResolutionNote(t *testing.T) {
	handler, patrol, unifiedStore, _ := setupAIHandlerWithPatrol(t)
	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-resolve-note", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-resolve-note", detectedAt)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/resolve", strings.NewReader(`{"finding_id":"finding-resolve-note","resolution_note":" fixed after restarting the worker "}`))
	rec := httptest.NewRecorder()

	handler.HandleResolveFinding(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	patrolFinding := patrol.GetFindings().Get("finding-resolve-note")
	require.NotNil(t, patrolFinding)
	require.NotNil(t, patrolFinding.ResolvedAt)
	require.Equal(t, "fixed after restarting the worker", patrolFinding.UserNote)
}

func TestAISettingsHandler_PatrolAutonomyMonitorOnlyAllowsMonitor(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := `{"autonomy_level":"monitor","full_mode_unlocked":true,"investigation_budget":2,"investigation_timeout_sec":30}`
	req := newLoopbackRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdatePatrolAutonomyMonitorOnly(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Success  bool                   `json:"success"`
		Settings PatrolAutonomyResponse `json:"settings"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, config.PatrolAutonomyMonitor, resp.Settings.AutonomyLevel)
	require.False(t, resp.Settings.FullModeUnlocked)
	require.Equal(t, 5, resp.Settings.InvestigationBudget)
	require.Equal(t, 60, resp.Settings.InvestigationTimeoutSec)

	saved, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Equal(t, config.PatrolAutonomyMonitor, saved.PatrolAutonomyLevel)
	require.False(t, saved.PatrolFullModeUnlocked)
	require.Equal(t, 5, saved.PatrolInvestigationBudget)
	require.Equal(t, 60, saved.PatrolInvestigationTimeoutSec)

	premiumBody := `{"autonomy_level":"approval","investigation_budget":10,"investigation_timeout_sec":120}`
	premiumReq := newLoopbackRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(premiumBody))
	premiumRec := httptest.NewRecorder()
	handler.HandleUpdatePatrolAutonomyMonitorOnly(premiumRec, premiumReq)

	require.Equal(t, http.StatusPaymentRequired, premiumRec.Code, premiumRec.Body.String())
	require.Contains(t, premiumRec.Body.String(), "limited to Monitor")
}

func TestAISettingsProvidersProjectionCarriesSuggestedModelWireShape(t *testing.T) {
	payload, err := json.Marshal(aiProviderDefinitionResponses(nil))
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))

	var ollama map[string]any
	for _, entry := range decoded {
		if entry["id"] == config.AIProviderOllama {
			ollama = entry
			continue
		}
		// omitempty: providers without a blessing must not emit the keys.
		require.NotContains(t, entry, "suggested_model")
		require.NotContains(t, entry, "suggested_model_note")
		require.NotContains(t, entry, "suggested_model_equivalents")
	}
	require.NotNil(t, ollama)
	require.Equal(t, config.OllamaSuggestedPatrolModel, ollama["suggested_model"])
	require.NotEmpty(t, ollama["suggested_model_note"])
	require.NotEmpty(t, ollama["suggested_model_equivalents"])
}

func TestAISettingsHandler_PatrolReadinessFlagsReasoningOnlyModel(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.PatrolModel = "ollama:deepseek-r1:7b-llama-distill-q4_K_M"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	readiness := handler.buildPatrolReadiness(context.Background(), handler.GetAIService(context.Background()), true)

	require.Equal(t, patrolReadinessNotReady, readiness.Status)
	require.False(t, readiness.Ready)
	require.Equal(t, "ollama", readiness.Provider)
	require.Equal(t, "ollama:deepseek-r1:7b-llama-distill-q4_K_M", readiness.Model)
	require.Contains(t, readiness.Summary, "reasoning-only model family")

	var toolCheck *PatrolReadinessCheck
	for i := range readiness.Checks {
		if readiness.Checks[i].ID == "tools" {
			toolCheck = &readiness.Checks[i]
			break
		}
	}
	require.NotNil(t, toolCheck)
	require.Equal(t, patrolReadinessNotReady, toolCheck.Status)
}

func TestAISettingsHandler_UpdateSettingsPersistsNotReadyPatrolModelWithReadiness(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	model := "ollama:deepseek-r1:7b-llama-distill-q4_K_M"
	body, err := json.Marshal(AISettingsUpdateRequest{
		Enabled:       ptr(true),
		Model:         ptr(model),
		PatrolModel:   ptr(model),
		OllamaBaseURL: ptr("http://127.0.0.1:11434"),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Enabled)
	require.NotNil(t, payload.PatrolReadiness)
	require.Equal(t, patrolReadinessNotReady, payload.PatrolReadiness.Status)
	require.Equal(t, string(ai.PatrolFailureCauseModelUnsupportedTools), payload.PatrolReadiness.Cause)
	require.Contains(t, payload.PatrolReadiness.Summary, "reasoning-only model family")
	require.Equal(t, "ollama", payload.PatrolReadiness.Provider)
	require.Equal(t, model, payload.PatrolReadiness.Model)

	persisted, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.True(t, persisted.Enabled)
	require.Equal(t, model, persisted.PatrolModel)
}

func TestAISettingsHandler_PatrolConfigReadinessUsesControlLabel(t *testing.T) {
	readiness := patrolReadinessResponseFromConfigReadiness(ai.PatrolConfigReadiness{
		Status:   ai.PatrolReadinessWarning,
		Ready:    true,
		Cause:    ai.PatrolFailureCauseModelToolSupportUnverified,
		Summary:  "Ollama connectivity alone does not prove tool support.",
		Provider: "ollama",
		Model:    "ollama:llama3",
	})

	require.Equal(t, patrolReadinessWarning, readiness.Status)
	require.True(t, readiness.Ready)
	check := requirePatrolReadinessCheck(t, readiness, "configuration")
	require.Equal(t, "Patrol control", check.Label)
	require.NotContains(t, check.Label, "configuration")
}

func TestAISettingsHandler_UpdateSettingsDoesNotLockUnrelatedSavesBehindExistingPatrolReadiness(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	model := "ollama:deepseek-r1:7b-llama-distill-q4_K_M"
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = model
	aiCfg.PatrolModel = model
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		RequestTimeoutSeconds: ptr(120),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	persisted, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Equal(t, 120, persisted.RequestTimeoutSeconds)
	require.Equal(t, model, persisted.PatrolModel)
}

func TestAISettingsHandler_UpdateSettingsRefreshesAssistantToolVisibilityForControlChanges(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	controlRefreshes := 0
	handler.SetOnControlSettingsChange(func() {
		controlRefreshes++
	})

	unrelatedBody, err := json.Marshal(AISettingsUpdateRequest{
		RequestTimeoutSeconds: ptr(90),
	})
	require.NoError(t, err)
	unrelatedReq := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(unrelatedBody))
	unrelatedRec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(unrelatedRec, unrelatedReq)

	require.Equal(t, http.StatusOK, unrelatedRec.Code, unrelatedRec.Body.String())
	require.Equal(t, 0, controlRefreshes, "non-control settings must not refresh Assistant tool visibility")

	controlBody, err := json.Marshal(AISettingsUpdateRequest{
		ControlLevel: ptr(config.ControlLevelControlled),
	})
	require.NoError(t, err)
	controlReq := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(controlBody))
	controlRec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(controlRec, controlReq)

	require.Equal(t, http.StatusOK, controlRec.Code, controlRec.Body.String())
	require.Equal(t, 1, controlRefreshes, "control level changes must refresh Assistant tool visibility")

	protectedBody, err := json.Marshal(AISettingsUpdateRequest{
		ProtectedGuests: []string{"vm-101"},
	})
	require.NoError(t, err)
	protectedReq := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(protectedBody))
	protectedRec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(protectedRec, protectedReq)

	require.Equal(t, http.StatusOK, protectedRec.Code, protectedRec.Body.String())
	require.Equal(t, 2, controlRefreshes, "protected guest changes must refresh Assistant tool visibility")

	persisted, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Equal(t, config.ControlLevelControlled, persisted.GetControlLevel())
	require.Equal(t, []string{"vm-101"}, persisted.GetProtectedGuests())
}

func TestAISettingsHandler_PatrolReadinessBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		patrolAvailable bool
		configure       func(*config.AIConfig)
		wantStatus      string
		wantReady       bool
		wantCheckID     string
		wantCheck       string
		wantSummary     string
	}{
		{
			name:            "disabled assistant blocks patrol",
			patrolAvailable: true,
			wantStatus:      patrolReadinessNotReady,
			wantReady:       false,
			wantCheckID:     "enabled",
			wantCheck:       patrolReadinessNotReady,
			wantSummary:     "turned off",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = false
				aiCfg.Model = "ollama:llama3"
				aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:            "provider configured without model blocks patrol",
			patrolAvailable: true,
			wantStatus:      patrolReadinessNotReady,
			wantReady:       false,
			wantCheckID:     "model",
			wantCheck:       patrolReadinessNotReady,
			wantSummary:     "No concrete Patrol model",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:            "selected model provider must be configured",
			patrolAvailable: true,
			wantStatus:      patrolReadinessNotReady,
			wantReady:       false,
			wantCheckID:     "model",
			wantCheck:       patrolReadinessNotReady,
			wantSummary:     "provider is not configured",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "anthropic:claude-3-5-sonnet-latest"
				aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:            "ollama tool support is a warning",
			patrolAvailable: true,
			wantStatus:      patrolReadinessWarning,
			wantReady:       true,
			wantCheckID:     "tools",
			wantCheck:       patrolReadinessWarning,
			wantSummary:     "Ollama connectivity alone does not prove tool support",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "ollama:llama3"
				aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:            "anthropic model is ready",
			patrolAvailable: true,
			wantStatus:      patrolReadinessReady,
			wantReady:       true,
			wantCheckID:     "tools",
			wantCheck:       patrolReadinessReady,
			wantSummary:     "ready to run",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "anthropic:claude-3-5-sonnet-latest"
				aiCfg.AnthropicAPIKey = "test-key"
			},
		},
		{
			name:            "deepseek v4 flash is ready",
			patrolAvailable: true,
			wantStatus:      patrolReadinessReady,
			wantReady:       true,
			wantCheckID:     "tools",
			wantCheck:       patrolReadinessReady,
			wantSummary:     "ready to run",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "deepseek:deepseek-v4-flash"
				aiCfg.DeepSeekAPIKey = "test-key"
			},
		},
		{
			name:            "deepseek legacy aliases warn instead of silently ready",
			patrolAvailable: true,
			wantStatus:      patrolReadinessWarning,
			wantReady:       true,
			wantCheckID:     "tools",
			wantCheck:       patrolReadinessWarning,
			wantSummary:     "legacy alias currently routes to V4 Flash",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "deepseek:deepseek-chat"
				aiCfg.DeepSeekAPIKey = "test-key"
			},
		},
		{
			name:            "deepseek typo blocks before Patrol runtime",
			patrolAvailable: true,
			wantStatus:      patrolReadinessNotReady,
			wantReady:       false,
			wantCheckID:     "tools",
			wantCheck:       patrolReadinessNotReady,
			wantSummary:     "current official DeepSeek API catalog",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "deepseek:deepseek-v4-flush7pro"
				aiCfg.DeepSeekAPIKey = "test-key"
			},
		},
		{
			name:            "missing patrol service blocks before settings checks",
			patrolAvailable: false,
			wantStatus:      patrolReadinessNotReady,
			wantReady:       false,
			wantCheckID:     "service",
			wantCheck:       patrolReadinessNotReady,
			wantSummary:     "Pulse Patrol service is not available",
			configure: func(aiCfg *config.AIConfig) {
				aiCfg.Enabled = true
				aiCfg.Model = "anthropic:claude-3-5-sonnet-latest"
				aiCfg.AnthropicAPIKey = "test-key"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			cfg := &config.Config{DataPath: tmp}
			persistence := config.NewConfigPersistence(tmp)
			aiCfg := config.NewDefaultAIConfig()
			if tt.configure != nil {
				tt.configure(aiCfg)
			}
			require.NoError(t, persistence.SaveAIConfig(*aiCfg))

			handler := newTestAISettingsHandler(cfg, persistence, nil)
			readiness := handler.buildPatrolReadiness(context.Background(), handler.GetAIService(context.Background()), tt.patrolAvailable)

			require.Equal(t, tt.wantStatus, readiness.Status)
			require.Equal(t, tt.wantReady, readiness.Ready)
			require.Contains(t, readiness.Summary, tt.wantSummary)
			check := requirePatrolReadinessCheck(t, readiness, tt.wantCheckID)
			require.Equal(t, tt.wantCheck, check.Status)
		})
	}
}

func TestAISettingsHandler_PatrolReadinessMissingAssistantServiceUsesNativeSurfaceIdentity(t *testing.T) {
	t.Parallel()

	handler := &AISettingsHandler{}
	readiness := handler.buildPatrolReadiness(context.Background(), nil, true)

	require.Equal(t, patrolReadinessNotReady, readiness.Status)
	require.False(t, readiness.Ready)
	require.Equal(t, "Pulse Assistant runtime service is not available.", readiness.Summary)
	require.NotContains(t, readiness.Summary, "Pulse AI")

	check := requirePatrolReadinessCheck(t, readiness, "service")
	require.Equal(t, "Assistant runtime service", check.Label)
	require.Equal(t, "Pulse Assistant runtime service is not available.", check.Message)
	require.NotContains(t, check.Message, "Pulse AI")
}

func TestAISettingsHandler_PatrolReadinessNotReadyTakesPrecedenceOverWarnings(t *testing.T) {
	readiness := summarizePatrolReadiness("ollama", "ollama:llama3", []PatrolReadinessCheck{
		{ID: "tools", Status: patrolReadinessWarning, Message: "warning message"},
		{ID: "model", Status: patrolReadinessNotReady, Message: "not ready message"},
	})

	require.Equal(t, patrolReadinessNotReady, readiness.Status)
	require.False(t, readiness.Ready)
	require.Equal(t, "not ready message", readiness.Summary)
}

func requirePatrolReadinessCheck(t *testing.T, readiness PatrolReadinessResponse, id string) PatrolReadinessCheck {
	t.Helper()
	for _, check := range readiness.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("readiness check %q not found in %+v", id, readiness.Checks)
	return PatrolReadinessCheck{}
}

func TestAISettingsHandler_GetAndUpdateSettings_RoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// GET should return defaults if no config has been saved yet.
	{
		req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Enabled {
			t.Fatalf("expected default Enabled=false, got %+v", resp)
		}
		responseBody := rec.Body.String()
		if !strings.Contains(responseBody, `"available_models":[]`) ||
			!strings.Contains(responseBody, `"configured_providers":[]`) ||
			!strings.Contains(responseBody, `"protected_guests":[]`) {
			t.Fatalf("expected AI settings response collections in JSON body, got %s", responseBody)
		}
	}

	// Update settings to enable AI via Ollama.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			Enabled:         ptr(true),
			Model:           ptr("ollama:llama3"),
			OllamaBaseURL:   ptr("http://localhost:11434"),
			OllamaUsername:  ptr("unai"),
			OllamaPassword:  ptr("secret"),
			OllamaKeepAlive: ptr("24h"),
			ZaiBaseURL:      ptr("https://api.z.ai/api/coding/paas/v4"),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled || !resp.OllamaConfigured {
			t.Fatalf("expected enabled + ollama configured, got %+v", resp)
		}
		if resp.OllamaBaseURL != "http://localhost:11434" {
			t.Fatalf("unexpected ollama base url: %+v", resp)
		}
		if resp.ZaiBaseURL != "https://api.z.ai/api/coding/paas/v4" {
			t.Fatalf("unexpected zai base url: %+v", resp)
		}
		if resp.OllamaUsername != "unai" || !resp.OllamaPasswordSet {
			t.Fatalf("expected ollama auth state in response, got %+v", resp)
		}
		if resp.OllamaKeepAlive != "24h" {
			t.Fatalf("expected ollama keep alive in response, got %+v", resp)
		}
		responseBody := rec.Body.String()
		if !strings.Contains(responseBody, `"available_models":[]`) ||
			!strings.Contains(responseBody, `"configured_providers":[`) ||
			!strings.Contains(responseBody, `"protected_guests":[]`) {
			t.Fatalf("expected AI settings response collections in JSON body, got %s", responseBody)
		}
	}

	// GET again should reflect persisted updates.
	{
		req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !resp.Enabled || !resp.OllamaConfigured {
			t.Fatalf("expected enabled + ollama configured, got %+v", resp)
		}
		if resp.OllamaUsername != "unai" || !resp.OllamaPasswordSet {
			t.Fatalf("expected persisted ollama auth state, got %+v", resp)
		}
		if resp.OllamaKeepAlive != "24h" {
			t.Fatalf("expected persisted ollama keep alive, got %+v", resp)
		}
		responseBody := rec.Body.String()
		if !strings.Contains(responseBody, `"available_models":[]`) ||
			!strings.Contains(responseBody, `"configured_providers":[`) ||
			!strings.Contains(responseBody, `"protected_guests":[]`) {
			t.Fatalf("expected AI settings response collections in JSON body, got %s", responseBody)
		}
	}
}

func TestAISettingsHandler_GetSettingsClampsPaidControlsToEntitlements(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	aiCfg.ControlLevel = config.ControlLevelAutonomous
	aiCfg.PatrolAutoFix = true
	aiCfg.AlertTriggeredAnalysis = true
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, config.ControlLevelControlled, resp.ControlLevel)
	require.False(t, resp.PatrolAutoFix)
	require.False(t, resp.AlertTriggeredAnalysis)
}

func TestAISettingsHandler_UpdateSettings_OllamaKeepAlive(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		Enabled:         ptr(true),
		Model:           ptr("ollama:llama3"),
		OllamaBaseURL:   ptr("http://127.0.0.1:11434"),
		OllamaKeepAlive: ptr(""),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Empty(t, resp.OllamaKeepAlive)

	saved, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Empty(t, saved.OllamaKeepAlive)

	body, err = json.Marshal(AISettingsUpdateRequest{
		OllamaKeepAlive: ptr("24h"),
	})
	require.NoError(t, err)
	req = newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "24h", resp.OllamaKeepAlive)

	saved, err = persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Equal(t, "24h", saved.OllamaKeepAlive)
}

func TestAISettingsHandler_UpdateSettingsRejectsInvalidOllamaKeepAlive(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		OllamaKeepAlive: ptr("forever"),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "ollama_keep_alive")
}

func TestAISettingsHandler_UpdateSettings_PatrolAlertTriggerPolicy(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		PatrolAlertTriggerMinSeverity: ptr("warning"),
		PatrolAlertTriggerTypes:       ptr([]string{"CPU", " cpu ", "memory", ""}),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "warning", resp.PatrolAlertTriggerMinSeverity)
	require.Equal(t, []string{"cpu", "memory"}, resp.PatrolAlertTriggerTypes)

	saved, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.Equal(t, "warning", saved.PatrolAlertTriggerMinSeverity)
	require.Equal(t, []string{"cpu", "memory"}, saved.PatrolAlertTriggerTypes)
}

func TestAISettingsHandler_UpdateSettingsRejectsInvalidPatrolAlertTriggerMinSeverity(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		PatrolAlertTriggerMinSeverity: ptr("emergency"),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "patrol_alert_trigger_min_severity")
}

func TestAISettingsHandler_GetAIService_TenantPatrolUsesCanonicalProviders(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)

	defaultMonitor, _, _ := newTestMonitor(t)
	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default":  defaultMonitor,
		"tenant-1": tenantMonitor,
	})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetStateProvider(defaultMonitor)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatal("expected tenant AI service")
	}
	if svc.GetStateProvider() != nil {
		t.Fatal("expected tenant AI service to avoid snapshot provider bridge")
	}
	if svc.GetPatrolService() == nil {
		t.Fatal("expected tenant patrol service to initialize from canonical providers")
	}
}

func TestAISettingsResponse_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyAISettingsResponse())
	if err != nil {
		t.Fatalf("marshal empty AI settings response: %v", err)
	}
	if !strings.Contains(string(payload), `"available_models":[]`) {
		t.Fatalf("expected empty AI settings response to retain available_models, got %s", payload)
	}
	if !strings.Contains(string(payload), `"configured_providers":[]`) {
		t.Fatalf("expected empty AI settings response to retain configured_providers, got %s", payload)
	}
	if !strings.Contains(string(payload), `"protected_guests":[]`) {
		t.Fatalf("expected empty AI settings response to retain protected_guests, got %s", payload)
	}
}

func TestAISettingsHandler_GetHostedSettings_DoesNotBootstrapQuickstart(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")

	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	persistence, err := mtp.GetPersistence("default")
	require.NoError(t, err)

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: tmp}

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Enabled)
	assert.False(t, resp.Configured)
	assert.Empty(t, resp.Model)
	assert.False(t, persistence.HasAIConfig())
}

func TestAISettingsHandler_GetHostedTenantSettings_DoesNotBootstrapQuickstartFromDefaultBillingState(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")

	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	persistence, err := mtp.GetPersistence("t-tenant")
	require.NoError(t, err)

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: tmp}

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-tenant"))
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Enabled)
	assert.False(t, resp.Configured)
	assert.Empty(t, resp.Model)
	assert.False(t, persistence.HasAIConfig())
}

func TestAISettingsHandler_GetSettings_RetiresLegacyQuickstartAliases(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	persistence, err := mtp.GetPersistence("default")
	require.NoError(t, err)

	raw, err := json.Marshal(map[string]any{
		"enabled":        true,
		"model":          "quickstart:minimax-2.5m",
		"chat_model":     "quickstart:minimax-2.5m",
		"patrol_model":   "quickstart:minimax-2.5m",
		"auto_fix_model": "quickstart:minimax-2.5m",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(persistence.DataDir(), "ai.enc"), raw, 0o600))

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: tmp}

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Model)
	assert.Empty(t, resp.ChatModel)
	assert.Empty(t, resp.PatrolModel)
	assert.Empty(t, resp.AutoFixModel)
}

func TestAISettingsHandler_GetSettings_DoesNotExposeQuickstartSurface(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Configured)
	assert.NotContains(t, rec.Body.String(), "quickstart_")
	assert.NotContains(t, rec.Body.String(), "using_quickstart")
}

func TestAISettingsHandler_UpdateSettings_RequiresProviderBeforeEnable(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, _ := json.Marshal(AISettingsUpdateRequest{
		Enabled: ptr(true),
	})
	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Please configure a provider")
}

func TestAISettingsHandler_UpdateSettings_DoesNotBootstrapQuickstartWithActivationIdentity(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, _ := json.Marshal(AISettingsUpdateRequest{
		Enabled: ptr(true),
	})
	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Please configure a provider")
}

func TestAISettingsHandler_UpdateSettings_ResolvesProviderModelWhenOmitted(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:latest"},
					{"name": "mistral:latest"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, _ := json.Marshal(AISettingsUpdateRequest{
		Enabled:       ptr(true),
		OllamaBaseURL: ptr(ollama.URL),
	})
	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Enabled)
	assert.True(t, resp.OllamaConfigured)
	assert.Equal(t, "ollama:llama3:latest", resp.Model)

	saved, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "ollama:llama3:latest", saved.Model)
	assert.Equal(t, ollama.URL, saved.OllamaBaseURL)
}

func TestAISettingsHandler_UpdateSettings_OpenRouterKeySetAndClear(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set OpenRouter key.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			OpenRouterAPIKey: ptr("  sk-or-test-key  "),
			Model:            ptr("openrouter:openai/gpt-4o-mini"),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

		var resp AISettingsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.OpenRouterConfigured)
		assert.Contains(t, resp.ConfiguredProviders, config.AIProviderOpenRouter)

		saved, err := persistence.LoadAIConfig()
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Equal(t, "sk-or-test-key", saved.OpenRouterAPIKey)
	}

	// Clear OpenRouter key.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			ClearOpenRouterKey: ptr(true),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

		var resp AISettingsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.False(t, resp.OpenRouterConfigured)
		assert.NotContains(t, resp.ConfiguredProviders, config.AIProviderOpenRouter)

		saved, err := persistence.LoadAIConfig()
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Empty(t, saved.OpenRouterAPIKey)
	}
}

func TestAISettingsHandler_ListModels_Ollama(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:latest"},
					{"name": "tinyllama:latest"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/models", nil)
	rec := httptest.NewRecorder()
	handler.HandleListModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 models, got %+v", resp.Models)
	}
}

func TestAISettingsHandler_Execute_Ollama(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/chat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model":      "llama3",
				"created_at": time.Now().Format(time.RFC3339),
				"message": map[string]any{
					"role":    "assistant",
					"content": "hello from ollama",
				},
				"done":              true,
				"done_reason":       "stop",
				"prompt_eval_count": 3,
				"eval_count":        5,
			})
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, _ := json.Marshal(AIExecuteRequest{Prompt: "hi"})
	req := newLoopbackRequest(http.MethodPost, "/api/ai/execute", bytes.NewReader(body))
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()
	handler.HandleExecute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp AIExecuteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Content != "hello from ollama" || resp.Model == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ToolCalls == nil {
		t.Fatalf("expected tool_calls to normalize to an empty array")
	}
	if resp.PendingApprovals == nil {
		t.Fatalf("expected pending_approvals to normalize to an empty array")
	}
}

func TestAIExecuteResponse_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyAIExecuteResponse())
	if err != nil {
		t.Fatalf("marshal empty AI execute response: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty AI execute response to retain tool_calls array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"pending_approvals":[]`) {
		t.Fatalf("expected empty AI execute response to retain pending_approvals array, got %s", payload)
	}
}

func TestHandleExecute_RejectsLegacyHostTargetType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"prompt":"hi","target_type":"host","target_id":"agent-1"}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/execute", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleExecute(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid target_type")
}

func TestHandleExecuteStream_RejectsLegacyHostTargetType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"prompt":"hi","target_type":"host","target_id":"agent-1"}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/execute/stream", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleExecuteStream(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid target_type")
}

func TestNormalizeAIExecuteTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "system-container", want: "system-container"},
		{in: "SYSTEM-CONTAINER", want: "system-container"},
		{in: "container", want: "container"},
		{in: "system_container", want: "system_container"},
	}

	for _, tt := range tests {
		got := normalizeAIExecuteTargetType(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeAIExecuteTargetType(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if shared := agentcapabilities.NormalizeActionTargetType(tt.in); got != shared {
			t.Fatalf("normalizeAIExecuteTargetType(%q) = %q, shared contract returned %q", tt.in, got, shared)
		}
	}
}

func TestNormalizeAndValidateAIExecuteTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "empty allowed", in: "", want: ""},
		{name: "agent", in: "agent", want: "agent"},
		{name: "system container canonical", in: "SYSTEM-CONTAINER", want: "system-container"},
		{name: "vm", in: "vm", want: "vm"},
		{name: "legacy host rejected", in: "host", wantErr: "invalid target_type"},
		{name: "legacy container rejected", in: "container", wantErr: "invalid target_type"},
		{name: "legacy underscore container rejected", in: "system_container", wantErr: "invalid target_type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAndValidateAIExecuteTargetType(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeInvestigateAlertTargetType_StrictCanonicalV6(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "vm", in: "vm", want: "vm"},
		{name: "system container", in: "system-container", want: "system-container"},
		{name: "oci container maps to system-container", in: "oci-container", want: "system-container"},
		{name: "app container maps to agent", in: "app-container", want: "agent"},
		{name: "pod maps to agent", in: "pod", want: "agent"},
		{name: "k8s cluster maps to agent", in: "k8s-cluster", want: "agent"},
		{name: "node maps to agent", in: "node", want: "agent"},
		{name: "truenas alias maps to agent", in: "truenas", want: "agent"},
		{name: "legacy host rejected", in: "host", wantErr: "unsupported resource_type"},
		{name: "legacy guest rejected", in: "guest", wantErr: "unsupported resource_type"},
		{name: "legacy docker rejected", in: "docker", wantErr: "unsupported resource_type"},
		{name: "legacy container rejected", in: "container", wantErr: "unsupported resource_type"},
		{name: "legacy k8s alias rejected", in: "k8s", wantErr: "unsupported resource_type"},
		{name: "missing type", in: "", wantErr: "resource_type is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeInvestigateAlertTargetType(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestAISettingsHandler_TestConnection_Ollama(t *testing.T) {
	t.Parallel()

	versionHits := 0
	tagsHits := 0
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "unai" || password != "secret" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, username, password)
		}
		switch r.URL.Path {
		case "/api/version":
			versionHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			tagsHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3:latest"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	aiCfg.OllamaUsername = "unai"
	aiCfg.OllamaPassword = "secret"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if versionHits != 1 || tagsHits != 1 {
		t.Fatalf("expected one version check and one tags lookup for explicit Ollama model, got version=%d tags=%d", versionHits, tagsHits)
	}
}

func TestAISettingsHandler_TestProvider_Ollama(t *testing.T) {
	t.Parallel()

	versionHits := 0
	tagsHits := 0
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "unai" || password != "secret" {
			t.Fatalf("unexpected basic auth: ok=%v user=%q pass=%q", ok, username, password)
		}
		switch r.URL.Path {
		case "/api/version":
			versionHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			tagsHits++
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3:latest"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "openai:gpt-4o"
	aiCfg.PatrolModel = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	aiCfg.OllamaUsername = "unai"
	aiCfg.OllamaPassword = "secret"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success  bool   `json:"success"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Provider != "ollama" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if versionHits != 1 || tagsHits != 1 {
		t.Fatalf("expected one version check and one tags lookup for the resolved Ollama provider test, got version=%d tags=%d", versionHits, tagsHits)
	}
}

// ========================================
// HandleTestAIConnection edge-case tests
// ========================================

func TestAISettingsHandler_TestConnection_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAISettingsHandler_TestConnection_Failure(t *testing.T) {
	t.Parallel()

	// Ollama server that always returns 500 for /api/version
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	// Should still return 200 with success=false (not an HTTP error)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Model          string `json:"model"`
		Cause          string `json:"cause"`
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider connection issue", resp.Message)
	assert.Equal(t, "ollama:llama3", resp.Model)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderConnection), resp.Cause)
	assert.Contains(t, resp.Summary, "healthy connection to this provider")
	assert.Contains(t, resp.Recommendation, "Check provider reachability")
	assert.Equal(t, "open_provider_settings", resp.Action)
	assert.NotContains(t, rec.Body.String(), "Ollama returned status 500")
}

func TestAISettingsHandler_TestConnection_NoConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Don't save any AI config — service will have no configured provider
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestAIConnection(rec, req)

	// Should return 200 with success=false (connection test fails gracefully)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Cause          string `json:"cause"`
		Recommendation string `json:"recommendation"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider not ready", resp.Message)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderNotConfigured), resp.Cause)
	assert.Contains(t, resp.Recommendation, "Provider & Models settings page")
}

// ========================================
// HandleTestProvider edge-case tests
// ========================================

func TestAISettingsHandler_TestProvider_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAISettingsHandler_TestProvider_NotConfigured(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Configure Ollama only — so "openai" is unconfigured
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://192.0.2.1:11434" // TEST-NET, non-routable
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/openai", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Provider       string `json:"provider"`
		Cause          string `json:"cause"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider not configured", resp.Message)
	assert.Equal(t, "openai", resp.Provider)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderNotConfigured), resp.Cause)
	assert.Contains(t, resp.Recommendation, "configure the provider")
	assert.Equal(t, "open_provider_settings", resp.Action)
}

func TestAISettingsHandler_TestProvider_NoAIConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// No AI config file saved — LoadConfig returns a default config (not nil),
	// so the handler falls through to the HasProvider check which returns false
	// for the default (empty) config.
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Provider       string `json:"provider"`
		Cause          string `json:"cause"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider not configured", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderNotConfigured), resp.Cause)
	assert.Contains(t, resp.Recommendation, "Provider & Models settings page")
	assert.Equal(t, "open_provider_settings", resp.Action)
}

func TestAISettingsHandler_TestProvider_ConnectionFailure(t *testing.T) {
	t.Parallel()

	// Server that returns 500 for all requests
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		Cause          string `json:"cause"`
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider connection issue", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
	assert.Equal(t, "ollama:llama3", resp.Model)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderConnection), resp.Cause)
	assert.Contains(t, resp.Summary, "healthy connection to this provider")
	assert.Contains(t, resp.Recommendation, "Check provider reachability")
	assert.Equal(t, "open_provider_settings", resp.Action)
	assert.NotContains(t, rec.Body.String(), "Ollama returned status 500")
}

func TestAISettingsHandler_TestProvider_AuthFailureUsesSafeDiagnostic(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		Cause          string `json:"cause"`
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Provider authentication issue", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
	assert.Equal(t, "ollama:llama3", resp.Model)
	assert.Equal(t, string(ai.PatrolFailureCauseProviderAuth), resp.Cause)
	assert.Contains(t, resp.Summary, "credentials")
	assert.Contains(t, resp.Recommendation, "API key")
	assert.Equal(t, "open_provider_settings", resp.Action)
	assert.NotContains(t, rec.Body.String(), "Ollama returned status 401")
}

func TestAISettingsHandler_TestProvider_ModelUnavailableUsesSafeDiagnostic(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3:latest"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:missing:latest"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/test/ollama", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success        bool   `json:"success"`
		Message        string `json:"message"`
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		Cause          string `json:"cause"`
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
		Action         string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Selected model unavailable", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
	assert.Equal(t, "ollama:missing:latest", resp.Model)
	assert.Equal(t, string(ai.PatrolFailureCauseModelUnavailable), resp.Cause)
	assert.Contains(t, resp.Summary, "selected model is not available")
	assert.Contains(t, resp.Recommendation, "Choose one of the models")
	assert.Equal(t, "open_provider_settings", resp.Action)
	assert.NotContains(t, rec.Body.String(), "found: llama3:latest")
}

func TestAISettingsHandler_TestProvider_UsesRequestedModelOverride(t *testing.T) {
	t.Parallel()

	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "0.1.0"})
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "llama3:latest"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = ollama.URL
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(
		http.MethodPost,
		"/api/ai/test/ollama",
		strings.NewReader(`{"model":"ollama:missing:latest"}`),
	)
	rec := httptest.NewRecorder()
	handler.HandleTestProvider(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Cause    string `json:"cause"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Selected model unavailable", resp.Message)
	assert.Equal(t, "ollama", resp.Provider)
	assert.Equal(t, "ollama:missing:latest", resp.Model)
	assert.Equal(t, string(ai.PatrolFailureCauseModelUnavailable), resp.Cause)
	assert.NotContains(t, rec.Body.String(), "found: llama3:latest")
}

// ========================================
// HandleGetAICostSummary tests
// ========================================

func TestHandleGetAICostSummary_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetAICostSummary_NoAIService(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days        int    `json:"days"`
		PricingAsOf string `json:"pricing_as_of"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 default, got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_CustomDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary?days=7", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 7 {
		t.Fatalf("expected Days=7, got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_MaxDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Test that days > 365 is capped at 365
	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary?days=1000", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 365 {
		t.Fatalf("expected Days=365 (capped), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_NegativeDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary?days=-5", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for negative), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_ZeroDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary?days=0", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for zero), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_NonNumericDays(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary?days=abc", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Days != 30 {
		t.Fatalf("expected Days=30 (default for non-numeric), got %d", resp.Days)
	}
}

func TestHandleGetAICostSummary_ResponseShape(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	requiredFields := []string{"days", "pricing_as_of", "provider_models", "use_cases", "targets", "daily_totals", "totals"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// Verify all array fields are arrays (not null)
	arrayFields := []string{"provider_models", "use_cases", "targets", "daily_totals"}
	for _, field := range arrayFields {
		v, ok := resp[field]
		if !ok {
			t.Fatalf("missing %s", field)
		}
		if _, ok := v.([]interface{}); !ok {
			t.Fatalf("expected %s to be array, got %T", field, v)
		}
	}
}

func TestHandleGetAICostSummary_NoService_InvalidDays(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{DataPath: t.TempDir()}
	// Pass nil persistence so defaultAIService is nil — exercises the no-service stub path
	handler := newTestAISettingsHandler(cfg, nil, nil)

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"negative", "?days=-5", 30},
		{"zero", "?days=0", 30},
		{"non-numeric", "?days=abc", 30},
		{"over-max", "?days=1000", 365},
		{"valid", "?days=7", 7},
		{"default", "", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary"+tt.query, nil)
			rec := httptest.NewRecorder()
			handler.HandleGetAICostSummary(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			var resp struct {
				Days int `json:"days"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Days != tt.want {
				t.Fatalf("expected Days=%d, got %d", tt.want, resp.Days)
			}
		})
	}
}

func TestHandleGetAICostSummary_NoService_ResponseShape(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{DataPath: t.TempDir()}
	// nil persistence → nil defaultAIService → exercises no-service stub
	handler := newTestAISettingsHandler(cfg, nil, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAICostSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Days           int                    `json:"days"`
		RetentionDays  int                    `json:"retention_days"`
		EffectiveDays  int                    `json:"effective_days"`
		Truncated      bool                   `json:"truncated"`
		PricingAsOf    string                 `json:"pricing_as_of"`
		ProviderModels []interface{}          `json:"provider_models"`
		UseCases       []interface{}          `json:"use_cases"`
		Targets        []interface{}          `json:"targets"`
		DailyTotals    []interface{}          `json:"daily_totals"`
		Totals         map[string]interface{} `json:"totals"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Days != 30 {
		t.Errorf("expected Days=30, got %d", resp.Days)
	}
	if resp.RetentionDays != 365 {
		t.Errorf("expected RetentionDays=365, got %d", resp.RetentionDays)
	}
	if resp.EffectiveDays != 30 {
		t.Errorf("expected EffectiveDays=30, got %d", resp.EffectiveDays)
	}
	if resp.Truncated {
		t.Errorf("expected Truncated=false, got true")
	}

	// All array fields must be non-null arrays in the stub response
	if resp.ProviderModels == nil {
		t.Error("provider_models is null, expected empty array")
	}
	if resp.UseCases == nil {
		t.Error("use_cases is null, expected empty array")
	}
	if resp.Targets == nil {
		t.Error("targets is null, expected empty array")
	}
	if resp.DailyTotals == nil {
		t.Error("daily_totals is null, expected empty array")
	}
}

// ========================================
// HandleResetAICostHistory tests
// ========================================

func TestHandleResetAICostHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleResetAICostHistory_Success(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

// ========================================
// HandleExportAICostHistory tests
// ========================================

func TestHandleExportAICostHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/cost/export", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetSuppressionRules tests
// ========================================

func TestHandleGetSuppressionRules_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/suppressions", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetSuppressionRules(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleAddSuppressionRule tests
// ========================================

func TestHandleAddSuppressionRule_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/suppressions", nil)
	rec := httptest.NewRecorder()
	handler.HandleAddSuppressionRule(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestPatrolFindingsLimitFromQuery(t *testing.T) {
	t.Parallel()

	if got := patrolFindingsLimitFromQuery(url.Values{}, false); got != 0 {
		t.Fatalf("active-only default limit = %d, want 0", got)
	}
	if got := patrolFindingsLimitFromQuery(url.Values{}, true); got != defaultResolvedPatrolFindingsLimit {
		t.Fatalf("include-resolved default limit = %d, want %d", got, defaultResolvedPatrolFindingsLimit)
	}
	if got := patrolFindingsLimitFromQuery(url.Values{"limit": []string{"25"}}, true); got != 25 {
		t.Fatalf("explicit limit = %d, want 25", got)
	}
	if got := patrolFindingsLimitFromQuery(url.Values{"limit": []string{"9999"}}, true); got != maxPatrolFindingsLimit {
		t.Fatalf("oversized limit = %d, want %d", got, maxPatrolFindingsLimit)
	}
	if got := patrolFindingsLimitFromQuery(url.Values{"limit": []string{"0"}}, true); got != defaultResolvedPatrolFindingsLimit {
		t.Fatalf("invalid include-resolved limit = %d, want %d", got, defaultResolvedPatrolFindingsLimit)
	}
}

// ========================================
// HandleDeleteSuppressionRule tests
// ========================================

func TestHandleDeleteSuppressionRule_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/suppressions/rule-123", nil)
	rec := httptest.NewRecorder()
	handler.HandleDeleteSuppressionRule(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetDismissedFindings tests
// ========================================

func TestHandleGetDismissedFindings_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/dismissed", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetDismissedFindings(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetGuestKnowledge / HandleSaveGuestNote / HandleDeleteGuestNote / HandleClearGuestKnowledge
// tests have been consolidated into ai_handlers_knowledge_test.go
// ========================================

// ========================================
// HandleDebugContext tests
// ========================================

func TestHandleDebugContext_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/debug/context", nil)
	rec := httptest.NewRecorder()
	handler.HandleDebugContext(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// ========================================
// HandleGetConnectedAgents tests
// ========================================

func TestHandleGetConnectedAgents_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/agents", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetConnectedAgents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetConnectedAgents_NoAgentServer(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	// handler created with nil agentServer
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/agents", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetConnectedAgents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		Count int    `json:"count"`
		Note  string `json:"note"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 0 {
		t.Fatalf("expected count=0, got %d", resp.Count)
	}
	if resp.Note == "" {
		t.Fatalf("expected note to be present")
	}
}

// ========================================
// HandleRunCommand tests
// ========================================

func TestHandleRunCommand_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/run-command", nil)
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleRunCommand_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader([]byte(`{invalid json}`)))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestNormalizeRunCommandApprovalTarget_CanonicalizesAgentTarget(t *testing.T) {
	tests := []struct {
		name    string
		req     AIRunCommandRequest
		wantTyp string
		wantID  string
		wantErr string
	}{
		{
			name: "host target type rejected",
			req: AIRunCommandRequest{
				TargetType: "host",
				TargetID:   "Node-1",
			},
			wantErr: "unsupported target_type",
		},
		{
			name: "run_on_host forces agent target",
			req: AIRunCommandRequest{
				TargetType: "vm",
				RunOnHost:  true,
				TargetHost: "PVE-A",
			},
			wantTyp: "agent",
			wantID:  "pve-a",
		},
		{
			name: "system-container target preserved",
			req: AIRunCommandRequest{
				TargetType: "system-container",
				TargetID:   "201",
			},
			wantTyp: "system-container",
			wantID:  "201",
		},
		{
			name: "legacy container alias rejected",
			req: AIRunCommandRequest{
				TargetType: "container",
				TargetID:   "201",
			},
			wantErr: "unsupported target_type",
		},
		{
			name: "missing target host when run_on_host",
			req: AIRunCommandRequest{
				RunOnHost: true,
			},
			wantErr: "target_host is required when run_on_host is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotID, err := normalizeRunCommandApprovalTarget(tt.req)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTyp, gotType)
			require.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestHandleRunCommand_RequiresApprovalID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleRunCommand_ConsumesApproval(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(func(string, string, string) bool { return true }))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-1",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-1","command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.True(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsCommandMismatch(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(func(string, string, string) bool { return true }))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-2",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-2","command":"whoami","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsUnsupportedTargetType(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(func(string, string, string) bool { return true }))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	approval.SetStore(store)
	defer approval.SetStore(nil)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-docker-1",
		Command:    "docker restart web",
		TargetType: "docker",
		TargetID:   "host-1:web",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-docker-1","command":"docker restart web","target_type":"docker","target_id":"host-1:web","run_on_host":false}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported target_type")

	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
}

func TestHandleRunCommand_RejectsCrossOrgApproval(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, agentexec.NewServer(func(string, string, string) bool { return true }))

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)
	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(store)

	appReq := &approval.ApprovalRequest{
		ID:         "approval-org-a",
		OrgID:      "org-a",
		Command:    "uptime",
		TargetType: "vm",
		TargetID:   "vm-101",
	}
	require.NoError(t, store.CreateApproval(appReq))
	_, err = store.Approve(appReq.ID, "tester")
	require.NoError(t, err)

	body := []byte(`{"approval_id":"approval-org-a","command":"uptime","target_type":"vm","target_id":"vm-101","run_on_host":false}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/run-command", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "org-b")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandleRunCommand(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	stored, found := store.GetApproval(appReq.ID)
	require.True(t, found)
	require.False(t, stored.Consumed)
}

// ========================================
// HandleAnalyzeKubernetesCluster tests
// ========================================

func TestHandleAnalyzeKubernetesCluster_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/kubernetes/analyze", nil)
	rec := httptest.NewRecorder()
	handler.HandleAnalyzeKubernetesCluster(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleAnalyzeKubernetesCluster_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/kubernetes/analyze", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleAnalyzeKubernetesCluster(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleInvestigateAlert tests
// ========================================

func TestHandleInvestigateAlert_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/investigate", nil)
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleInvestigateAlert_InvalidBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader([]byte(`{invalid json}`)))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleInvestigateAlert_MissingAlertIdentifier(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleInvestigateAlert_RejectsLegacyHostResourceType(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"alertIdentifier":"alert-1","resource_id":"agent-1","resource_name":"node-1","resource_type":"host","alert_type":"cpu","level":"warning","message":"high cpu"}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unsupported resource_type "host"`)
	require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
}

func TestHandleInvestigateAlert_AcceptsCanonicalAlertIdentifier(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	saveEnabledTestAIConfig(t, persistence)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body := []byte(`{"alertIdentifier":"instance:node:100::metric/cpu","resource_id":"agent-1","resource_name":"node-1","resource_type":"host","alert_type":"cpu","level":"warning","message":"high cpu"}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/investigate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unsupported resource_type "host"`)
	require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
}

func TestHandleInvestigateAlert_ForcesApprovalBoundExecuteRequest(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("ai_handlers.go")
	require.NoError(t, err)
	text := string(source)

	require.Contains(t, text, "autonomousMode := false")
	require.Contains(t, text, "AutonomousMode:         &autonomousMode")
	require.Contains(t, text, "RequireCommandApproval: true")
}

func TestHandleInvestigateAlert_UsesVisibleIdleProgressStream(t *testing.T) {
	setMockModeForTest(t, true)
	withLegacyAssistantStreamIdleInterval(t, 2*time.Millisecond)

	handler := newTestAISettingsHandlerWithService(t)

	body := []byte(`{"alertIdentifier":"alert-1","resource_id":"node-1","resource_name":"node-1","resource_type":"agent","alert_type":"cpu","level":"warning","message":"high cpu"}`)
	req := newLoopbackRequest(http.MethodPost, "/api/ai/investigate-alert", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleInvestigateAlert(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assertLegacyAssistantStreamIdleBeforeTerminal(t, rec.Body.String())
}

// ========================================
// AISettingsHandler setter method tests
// ========================================

func TestAISettingsHandler_SetConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// SetConfig with nil should be a no-op
	handler.SetConfig(nil)
	require.Same(t, cfg, handler.defaultConfig)

	// SetConfig with new config should update the handler's config
	newCfg := &config.Config{DataPath: tmp}
	handler.SetConfig(newCfg)
	require.Same(t, newCfg, handler.defaultConfig)
}

func TestAISettingsHandler_GetAlertTriggeredAnalyzer(t *testing.T) {
	// Not parallel: mutates package-global createAlertAnalyzerFunc
	mock := &mockAlertAnalyzerForTest{}
	SetCreateAlertAnalyzer(func(_ aicontracts.AlertAnalyzerDeps) aicontracts.AlertAnalyzer {
		return mock
	})
	t.Cleanup(func() { SetCreateAlertAnalyzer(nil) })

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetAlertAnalyzerFactory(getCreateAlertAnalyzer())

	handler.SetStateProvider(&MockStateProvider{})

	analyzer := handler.GetAlertTriggeredAnalyzer(context.Background())
	require.NotNil(t, analyzer)

	again := handler.GetAlertTriggeredAnalyzer(context.Background())
	require.Same(t, mock, again)
}

func TestAISettingsHandler_StartPatrol(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.OllamaBaseURL = "http://localhost:11434"
	aiCfg.Model = "ollama:llama3"
	aiCfg.PatrolEnabled = true
	require.NoError(t, persistence.SaveAIConfig(*aiCfg))
	require.NoError(t, handler.defaultAIService.LoadConfig())

	handler.SetStateProvider(&MockStateProvider{})

	// Start patrol with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	handler.StartPatrol(ctx)

	patrol := handler.defaultAIService.GetPatrolService()
	require.NotNil(t, patrol)
	status := patrol.GetStatus()
	require.True(t, status.Enabled)

	cancel()
	assert.NotPanics(t, func() { handler.StopPatrol() })

	after := patrol.GetStatus()
	require.False(t, after.Running)
}

func TestAISettingsHandler_SetPatrolFindingsPersistence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil persistence should not panic
	err := handler.SetPatrolFindingsPersistence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAISettingsHandler_SetPatrolRunHistoryPersistence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Set nil persistence should not panic
	err := handler.SetPatrolRunHistoryPersistence(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAISettingsHandler_Approvals(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Initialize approval store
	approvalStore, _ := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(approvalStore)

	appID := "app-123"
	_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
		ID:          appID,
		Command:     "ls -la",
		RequestedBy: approval.RequesterPulsePatrol,
		Status:      approval.StatusPending,
	})

	t.Run("HandleGetApproval", func(t *testing.T) {
		req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals/"+appID, nil)
		rec := httptest.NewRecorder()
		handler.HandleGetApproval(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp approval.ApprovalRequest
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, appID, resp.ID)
		assert.Equal(t, approval.RequesterPulsePatrol, resp.RequestedBy)
	})

	t.Run("HandleListApprovals", func(t *testing.T) {
		req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals", nil)
		rec := httptest.NewRecorder()
		handler.HandleListApprovals(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Approvals []approval.ApprovalRequest `json:"approvals"`
			Stats     map[string]int             `json:"stats"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Len(t, resp.Approvals, 1)
		assert.Equal(t, appID, resp.Approvals[0].ID)
		assert.Equal(t, approval.RequesterPulsePatrol, resp.Approvals[0].RequestedBy)
		assert.Equal(t, 1, resp.Stats["pending"])
	})

	t.Run("HandleListApprovalsWithoutStoreReturnsEmptyCollections", func(t *testing.T) {
		approval.SetStore(nil)
		t.Cleanup(func() { approval.SetStore(approvalStore) })

		req := newLoopbackRequest(http.MethodGet, "/api/ai/approvals", nil)
		rec := httptest.NewRecorder()
		handler.HandleListApprovals(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Approvals []approval.ApprovalRequest `json:"approvals"`
			Stats     map[string]int             `json:"stats"`
		}
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.NotNil(t, resp.Approvals)
		require.Len(t, resp.Approvals, 0)
		assert.Equal(t, 0, resp.Stats["pending"])
		assert.Equal(t, 0, resp.Stats["executions"])
	})

	t.Run("HandleApproveCommand", func(t *testing.T) {
		req := newLoopbackRequest(http.MethodPost, "/api/ai/approvals/"+appID+"/approve", nil)
		rec := httptest.NewRecorder()
		handler.HandleApproveCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID)
		assert.Equal(t, approval.StatusApproved, app.Status)
	})

	t.Run("HandleDenyCommand", func(t *testing.T) {
		appID2 := "app-456"
		_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
			ID:      appID2,
			Command: "rm -rf /",
			Status:  approval.StatusPending,
		})

		body, _ := json.Marshal(map[string]string{"reason": "too dangerous"})
		req := newLoopbackRequest(http.MethodPost, "/api/ai/approvals/"+appID2+"/deny", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleDenyCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID2)
		assert.Equal(t, approval.StatusDenied, app.Status)
		assert.Equal(t, "too dangerous", app.DenyReason)
	})

	t.Run("HandleDenyCommand_RecordsRejectedInvestigationOutcome", func(t *testing.T) {
		findingID := "finding-denied-fix"
		handler.SetStateProvider(&MockStateProvider{})
		patrol := handler.defaultAIService.GetPatrolService()
		require.NotNil(t, patrol)
		patrol.GetFindings().Add(&ai.Finding{
			ID:                   findingID,
			ResourceID:           "vm:100",
			ResourceName:         "web-100",
			ResourceType:         "vm",
			Severity:             ai.FindingSeverityWarning,
			Category:             ai.FindingCategoryPerformance,
			Title:                "High CPU",
			Description:          "CPU stayed high.",
			InvestigationStatus:  string(ai.InvestigationStatusCompleted),
			InvestigationOutcome: string(ai.InvestigationOutcomeFixQueued),
		})
		appIDRejected := "app-investigation-deny"
		_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
			ID:         appIDRejected,
			ToolID:     "investigation_fix",
			Command:    "systemctl restart workload.service",
			TargetType: "investigation",
			TargetID:   findingID,
			TargetName: "web-100",
			Context:    "Automated fix from patrol investigation: restart workload",
			Status:     approval.StatusPending,
		})

		body, _ := json.Marshal(map[string]string{"reason": "outside maintenance"})
		req := newLoopbackRequest(http.MethodPost, "/api/ai/approvals/"+appIDRejected+"/deny", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleDenyCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		updated := patrol.GetFindings().Get(findingID)
		require.NotNil(t, updated)
		assert.Equal(t, string(ai.InvestigationOutcomeFixRejected), updated.InvestigationOutcome)
		assert.Equal(t, string(ai.FindingLoopStateNeedsAttention), updated.LoopState)
	})

	t.Run("HandleApproveCommand_AcceptsRelayMobileScope", func(t *testing.T) {
		appID3 := "app-789"
		_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
			ID:      appID3,
			Command: "pwd",
			Status:  approval.StatusPending,
		})

		req := newLoopbackRequest(http.MethodPost, "/api/ai/approvals/"+appID3+"/approve", nil)
		record := config.APITokenRecord{ID: "relay-mobile-approve", Scopes: []string{config.ScopeRelayMobileAccess}}
		attachAPITokenRecord(req, &record)
		rec := httptest.NewRecorder()
		handler.HandleApproveCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID3)
		assert.Equal(t, approval.StatusApproved, app.Status)
	})

	t.Run("HandleDenyCommand_AcceptsRelayMobileScope", func(t *testing.T) {
		appID4 := "app-999"
		_ = approvalStore.CreateApproval(&approval.ApprovalRequest{
			ID:      appID4,
			Command: "rm -rf /tmp",
			Status:  approval.StatusPending,
		})

		body, _ := json.Marshal(map[string]string{"reason": "not now"})
		req := newLoopbackRequest(http.MethodPost, "/api/ai/approvals/"+appID4+"/deny", bytes.NewReader(body))
		record := config.APITokenRecord{ID: "relay-mobile-deny", Scopes: []string{config.ScopeRelayMobileAccess}}
		attachAPITokenRecord(req, &record)
		rec := httptest.NewRecorder()
		handler.HandleDenyCommand(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		app, _ := approvalStore.GetApproval(appID4)
		assert.Equal(t, approval.StatusDenied, app.Status)
		assert.Equal(t, "not now", app.DenyReason)
	})
}

func TestAISettingsHandler_Approvals_RejectCrossOrgAccess(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmp,
		DisablePersistence: true,
	})
	require.NoError(t, err)

	prevStore := approval.GetStore()
	t.Cleanup(func() { approval.SetStore(prevStore) })
	approval.SetStore(store)

	withOrg := func(req *http.Request, orgID string) *http.Request {
		ctx := context.WithValue(req.Context(), OrgIDContextKey, orgID)
		return req.WithContext(ctx)
	}

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-get",
		OrgID:   "org-a",
		Command: "ls -la",
	}))

	getReq := withOrg(newLoopbackRequest(http.MethodGet, "/api/ai/approvals/cross-org-get", nil), "org-b")
	getRec := httptest.NewRecorder()
	handler.HandleGetApproval(getRec, getReq)
	require.Equal(t, http.StatusNotFound, getRec.Code)

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-approve",
		OrgID:   "org-a",
		Command: "uptime",
	}))

	approveReq := withOrg(newLoopbackRequest(http.MethodPost, "/api/ai/approvals/cross-org-approve/approve", nil), "org-b")
	approveRec := httptest.NewRecorder()
	handler.HandleApproveCommand(approveRec, approveReq)
	require.Equal(t, http.StatusNotFound, approveRec.Code)
	approveApp, ok := store.GetApproval("cross-org-approve")
	require.True(t, ok)
	require.Equal(t, approval.StatusPending, approveApp.Status)

	require.NoError(t, store.CreateApproval(&approval.ApprovalRequest{
		ID:      "cross-org-deny",
		OrgID:   "org-a",
		Command: "rm -rf /tmp",
	}))

	denyReq := withOrg(
		newLoopbackRequest(http.MethodPost, "/api/ai/approvals/cross-org-deny/deny", bytes.NewReader([]byte(`{"reason":"no"}`))),
		"org-b",
	)
	denyRec := httptest.NewRecorder()
	handler.HandleDenyCommand(denyRec, denyReq)
	require.Equal(t, http.StatusNotFound, denyRec.Code)
	denyApp, ok := store.GetApproval("cross-org-deny")
	require.True(t, ok)
	require.Equal(t, approval.StatusPending, denyApp.Status)
}

// ========================================
// aiSettingsUpdateRequiresPatrolPreflight tests
// ========================================

func TestAISettingsUpdateRequiresPatrolPreflight(t *testing.T) {
	t.Parallel()

	enabled := func(model, key string) *config.AIConfig {
		return &config.AIConfig{
			Enabled:        true,
			PatrolModel:    model,
			DeepSeekAPIKey: key,
		}
	}
	disabled := func() *config.AIConfig {
		return &config.AIConfig{Enabled: false, PatrolModel: "deepseek:deepseek-v4-flash"}
	}

	cases := []struct {
		name string
		old  *config.AIConfig
		new  *config.AIConfig
		want bool
	}{
		{
			name: "nil new → skip",
			new:  nil,
			want: false,
		},
		{
			name: "new disabled → skip",
			old:  enabled("deepseek:deepseek-v4-flash", "sk-old"),
			new:  disabled(),
			want: false,
		},
		{
			name: "new enabled with no patrol model → skip",
			new:  &config.AIConfig{Enabled: true, PatrolModel: ""},
			want: false,
		},
		{
			name: "first save (no prior config) → trigger",
			old:  nil,
			new:  enabled("deepseek:deepseek-v4-flash", "sk-new"),
			want: true,
		},
		{
			// Same predicate also drives the startup-seed path in
			// NewAISettingsHandler — passing nil for the prior config
			// represents "no in-memory cache yet, just booted." When
			// the loaded config has assistant + Patrol model, we should
			// preflight to populate the cache before the first
			// /api/settings/ai poll arrives so the UI's "last verified"
			// indicator is not blank after every restart.
			name: "startup seed (no prior cache, assistant enabled with patrol model) → trigger",
			old:  nil,
			new:  enabled("deepseek:deepseek-v4-flash", "sk-loaded-from-disk"),
			want: true,
		},
		{
			// Startup-seed must NOT fire when assistant is disabled —
			// otherwise we'd write a misleading "Pulse Assistant is not
			// enabled" entry into the cache for an operator who simply
			// hasn't enabled assistant yet.
			name: "startup seed when assistant disabled → skip",
			old:  nil,
			new:  &config.AIConfig{Enabled: false, PatrolModel: "deepseek:deepseek-v4-flash"},
			want: false,
		},
		{
			name: "assistant just enabled → trigger",
			old:  &config.AIConfig{Enabled: false, PatrolModel: "deepseek:deepseek-v4-flash", DeepSeekAPIKey: "sk-same"},
			new:  enabled("deepseek:deepseek-v4-flash", "sk-same"),
			want: true,
		},
		{
			name: "patrol model changed → trigger",
			old:  enabled("deepseek:deepseek-v4-flash", "sk-same"),
			new:  enabled("deepseek:deepseek-v4-pro", "sk-same"),
			want: true,
		},
		{
			name: "API key for patrol model's provider changed → trigger",
			old:  enabled("deepseek:deepseek-v4-flash", "sk-old"),
			new:  enabled("deepseek:deepseek-v4-flash", "sk-new"),
			want: true,
		},
		{
			name: "nothing relevant changed → skip",
			old:  enabled("deepseek:deepseek-v4-flash", "sk-same"),
			new:  enabled("deepseek:deepseek-v4-flash", "sk-same"),
			want: false,
		},
		{
			name: "unrelated provider key changed → skip",
			old: &config.AIConfig{
				Enabled:        true,
				PatrolModel:    "deepseek:deepseek-v4-flash",
				DeepSeekAPIKey: "sk-same",
				OpenAIAPIKey:   "sk-openai-old",
			},
			new: &config.AIConfig{
				Enabled:        true,
				PatrolModel:    "deepseek:deepseek-v4-flash",
				DeepSeekAPIKey: "sk-same",
				OpenAIAPIKey:   "sk-openai-new",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := aiSettingsUpdateRequiresPatrolPreflight(tc.old, tc.new)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// ========================================
// formatPatrolPreflightAge tests
// ========================================

func TestFormatPatrolPreflightAge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		age  time.Duration
		want string
	}{
		{age: 0, want: "just now"},
		{age: 30 * time.Second, want: "just now"},
		{age: 59 * time.Second, want: "just now"},
		{age: 1 * time.Minute, want: "1m ago"},
		{age: 5 * time.Minute, want: "5m ago"},
		{age: 59 * time.Minute, want: "59m ago"},
		{age: 1 * time.Hour, want: "1h ago"},
		{age: 12 * time.Hour, want: "12h ago"},
		{age: 23 * time.Hour, want: "23h ago"},
		{age: 24 * time.Hour, want: "1d ago"},
		{age: 72 * time.Hour, want: "3d ago"},
	}
	for _, tc := range cases {
		got := formatPatrolPreflightAge(tc.age)
		if got != tc.want {
			t.Errorf("formatPatrolPreflightAge(%v) = %q, want %q", tc.age, got, tc.want)
		}
	}
}

// ========================================
// HandlePatrolPreflight tests
// ========================================

func TestAISettingsHandler_PatrolPreflight_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/preflight", nil)
	rec := httptest.NewRecorder()
	handler.HandlePatrolPreflight(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAISettingsHandler_PatrolPreflight_AssistantDisabled(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = false
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/preflight", nil)
	rec := httptest.NewRecorder()
	handler.HandlePatrolPreflight(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success          bool   `json:"success"`
		Cause            string `json:"cause"`
		Message          string `json:"message"`
		ToolCallObserved bool   `json:"tool_call_observed"`
		DurationMs       int64  `json:"duration_ms"`
		Action           string `json:"action"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.False(t, resp.ToolCallObserved)
	assert.Equal(t, string(ai.PatrolFailureCauseAssistantDisabled), resp.Cause)
	assert.Equal(t, "open_provider_settings", resp.Action)
}

func TestAISettingsHandler_PatrolPreflight_RejectsInvalidProviderName(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/preflight",
		bytes.NewReader([]byte(`{"provider":"../etc/passwd"}`)))
	rec := httptest.NewRecorder()
	handler.HandlePatrolPreflight(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestOrchestratorAndChatAdaptersMapTheSameMessageFields keeps the deliberate
// GetMessages mirror between orchestratorChatAdapter (ai_handlers.go) and
// chatServiceAdapter (chat_service_adapter.go) honest: both convert the same
// chat-service messages onto separate output contracts, and a field mapped by
// one must be mapped by the other. chatServiceAdapter routes through
// adaptChatMessage so its API-facing tool calls stay on the shared provider
// shape instead of hand-copying a local duplicate.
func TestOrchestratorAndChatAdaptersMapTheSameMessageFields(t *testing.T) {
	for _, tc := range []struct {
		file     string
		fn       string
		label    string
		required []string
	}{
		{
			file:  "ai_handlers.go",
			fn:    "func (a *orchestratorChatAdapter) GetMessages(",
			label: "orchestratorChatAdapter.GetMessages",
			required: []string{
				"ID:",
				"Role:",
				"Content:",
				"ReasoningContent:",
				"Timestamp:",
				"aicontracts.OrchestratorToolCallInfoFromProvider(tc.ProviderToolCall())",
				"aicontracts.OrchestratorToolResultInfoFromProvider(*msg.ToolResult)",
				".NormalizeCollections()",
			},
		},
		{
			file:  "chat_service_adapter.go",
			fn:    "func adaptChatMessage(",
			label: "adaptChatMessage",
			required: []string{
				"ID:",
				"Role:",
				"Content:",
				"ReasoningContent:",
				"Timestamp:",
				"tc.ProviderToolCall()",
				"toolResult := *m.ToolResult",
				".NormalizeCollections()",
			},
		},
	} {
		source, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		text := string(source)
		start := strings.Index(text, tc.fn)
		if start < 0 {
			t.Fatalf("%s must define %q", tc.file, tc.fn)
		}
		end := strings.Index(text[start:], "\nfunc ")
		if end < 0 {
			end = len(text) - start
		}
		body := text[start : start+end]
		for _, required := range tc.required {
			if !strings.Contains(body, required) {
				t.Errorf("%s %s must map %q — keep the adapter mirror in sync", tc.file, tc.label, required)
			}
		}
	}

	chatAdapter, err := os.ReadFile("chat_service_adapter.go")
	if err != nil {
		t.Fatalf("read chat_service_adapter.go: %v", err)
	}
	if !strings.Contains(string(chatAdapter), "result[i] = adaptChatMessage(m)") {
		t.Error("chatServiceAdapter.GetMessages must route through adaptChatMessage")
	}
}
