package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

type contractCapturingStreamingProvider struct {
	lastRequest providers.ChatRequest
}

func (p *contractCapturingStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	p.lastRequest = req
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (p *contractCapturingStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	p.lastRequest = req
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn"}})
	return nil
}

func (p *contractCapturingStreamingProvider) TestConnection(ctx context.Context) error { return nil }
func (p *contractCapturingStreamingProvider) Name() string                             { return "contract-capture" }
func (p *contractCapturingStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}
func (p *contractCapturingStreamingProvider) SupportsThinking(model string) bool { return false }

func TestContract_WebSocketTrustedProxyHostedOrigin(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyCIDRsForTests()

	rawToken := "contract-ws-origin-forwarded-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?org_id=default"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://tenant.example.com")
	headers.Set("X-Forwarded-Proto", "https")
	headers.Set("X-Forwarded-Host", "tenant.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("expected websocket connection behind trusted proxy, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestContract_WireAIChatDependencies_WiresTrueNASAppActionProvider(t *testing.T) {
	router := &Router{
		trueNASPoller: monitoring.NewTrueNASPoller(nil, 0, nil),
	}
	service := &capturingAIService{}

	router.wireAIChatDependenciesForService(context.Background(), service)

	if service.appContainerActionProvider == nil {
		t.Fatal("expected TrueNAS app action provider to be wired into AI chat dependencies")
	}
	if service.appContainerReadProvider == nil {
		t.Fatal("expected TrueNAS app read provider to be wired into AI chat dependencies")
	}
	if service.appContainerConfigProvider == nil {
		t.Fatal("expected TrueNAS app config provider to be wired into AI chat dependencies")
	}
}

func TestContract_ChatServiceAdapterPatrolForwardsExecutionID(t *testing.T) {
	cfg := chat.Config{
		DataDir: t.TempDir(),
		AIConfig: &config.AIConfig{
			Enabled:     true,
			ChatModel:   "stub:model",
			PatrolModel: "stub:model",
		},
	}
	service := chat.NewService(cfg)
	provider := &contractCapturingStreamingProvider{}
	setUnexportedField(t, service, "providerFactory", func(string) (providers.StreamingProvider, error) {
		return provider, nil
	})
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })

	adapter := &chatServiceAdapter{svc: service}
	resp, err := adapter.ExecutePatrolStream(context.Background(), ai.PatrolExecuteRequest{
		Prompt:      "patrol",
		ExecutionID: "patrol-run-contract",
	}, func(ai.ChatStreamEvent) {})
	if err != nil {
		t.Fatalf("ExecutePatrolStream: %v", err)
	}
	if resp == nil || resp.Content == "" {
		t.Fatalf("expected patrol response content, got %#v", resp)
	}
	if provider.lastRequest.ExecutionID != "patrol-run-contract" {
		t.Fatalf("execution_id=%q want patrol-run-contract", provider.lastRequest.ExecutionID)
	}
}

func TestContract_AIQuickstartPayloadFieldsRemainCanonical(t *testing.T) {
	settingsBody, err := json.Marshal(AISettingsResponse{
		QuickstartCreditsRemaining: 7,
		QuickstartCreditsTotal:     25,
		UsingQuickstart:            true,
	})
	if err != nil {
		t.Fatalf("marshal AI settings response: %v", err)
	}
	if !bytes.Contains(settingsBody, []byte(`"quickstart_credits_remaining":7`)) {
		t.Fatalf("expected AI settings payload to expose quickstart_credits_remaining, got %s", settingsBody)
	}
	if !bytes.Contains(settingsBody, []byte(`"quickstart_credits_total":25`)) {
		t.Fatalf("expected AI settings payload to expose quickstart_credits_total, got %s", settingsBody)
	}
	if !bytes.Contains(settingsBody, []byte(`"using_quickstart":true`)) {
		t.Fatalf("expected AI settings payload to expose using_quickstart, got %s", settingsBody)
	}

	statusBody, err := json.Marshal(PatrolStatusResponse{
		QuickstartCreditsRemaining: 7,
		QuickstartCreditsTotal:     25,
		UsingQuickstart:            true,
	})
	if err != nil {
		t.Fatalf("marshal patrol status response: %v", err)
	}
	if !bytes.Contains(statusBody, []byte(`"quickstart_credits_remaining":7`)) {
		t.Fatalf("expected patrol status payload to expose quickstart_credits_remaining, got %s", statusBody)
	}
	if !bytes.Contains(statusBody, []byte(`"quickstart_credits_total":25`)) {
		t.Fatalf("expected patrol status payload to expose quickstart_credits_total, got %s", statusBody)
	}
	if !bytes.Contains(statusBody, []byte(`"using_quickstart":true`)) {
		t.Fatalf("expected patrol status payload to expose using_quickstart, got %s", statusBody)
	}
}

func TestContract_AISettingsUpdateQuickstartBootstrapJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	persistQuickstartActivationState(t, persistence)
	useTestQuickstartBootstrapServer(t, func(r *http.Request, reqBody map[string]any) {
		if got := strings.TrimSpace(r.Header.Get("Authorization")); got != "Bearer pit_live_test" {
			t.Fatalf("authorization=%q want Bearer pit_live_test", got)
		}
		instanceFingerprint, _ := reqBody["instance_fingerprint"].(string)
		if instanceFingerprint != "fp-live-test" {
			t.Fatalf("instance_fingerprint=%q want fp-live-test", instanceFingerprint)
		}
		if reqBody["use_case"] != "patrol" {
			t.Fatalf("use_case=%v want patrol", reqBody["use_case"])
		}
	})
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetQuickstartCredits(ai.NewPersistentQuickstartCreditManager(
		persistence,
		"default",
		func() *config.AIConfig {
			cfg, _ := persistence.LoadAIConfig()
			return cfg
		},
	))

	req := httptest.NewRequest(http.MethodPut, "/api/settings/ai/update", strings.NewReader(`{
		"enabled": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"quickstart:pulse-hosted",
		"chat_model":"quickstart:pulse-hosted",
		"patrol_model":"quickstart:pulse-hosted",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":25,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":25,
		"quickstart_credits_available":true,
		"using_quickstart":true
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_AISettingsUpdateProviderResolutionJSONSnapshot(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPut, "/api/settings/ai/update", strings.NewReader(fmt.Sprintf(`{
		"enabled": true,
		"ollama_base_url": %q
	}`, ollama.URL)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	want := fmt.Sprintf(`{
		"enabled":true,
		"model":"ollama:llama3:latest",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":true,
		"ollama_base_url":%q,
		"ollama_password_set":false,
		"configured_providers":["ollama"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":0,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":0,
		"quickstart_credits_available":false,
		"using_quickstart":false
	}`, ollama.URL)

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_PatrolStatusActivationRequiredSurface(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)
	persistence := handler.defaultPersistence
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler.defaultAIService.SetQuickstartCredits(ai.NewPersistentQuickstartCreditManager(
		persistence,
		"default",
		func() *config.AIConfig {
			cfg, _ := persistence.LoadAIConfig()
			return cfg
		},
	))
	if err := handler.defaultAIService.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode patrol status: %v", err)
	}
	if resp.RuntimeState != ai.PatrolRuntimeStateBlocked {
		t.Fatalf("runtime_state=%q want %q", resp.RuntimeState, ai.PatrolRuntimeStateBlocked)
	}
	if !resp.Enabled {
		t.Fatal("expected patrol status to remain enabled while activation is required")
	}
	if resp.Healthy {
		t.Fatal("expected activation-required patrol status to report healthy=false")
	}
	if resp.BlockedReason != ai.QuickstartActivationRequiredReason() {
		t.Fatalf("blocked_reason=%q want %q", resp.BlockedReason, ai.QuickstartActivationRequiredReason())
	}
	if resp.QuickstartCreditsRemaining != 0 || resp.QuickstartCreditsTotal != 0 {
		t.Fatalf("quickstart credits=%d/%d want 0/0", resp.QuickstartCreditsRemaining, resp.QuickstartCreditsTotal)
	}
	if resp.UsingQuickstart {
		t.Fatal("expected using_quickstart=false while activation is required")
	}
}

func TestContract_AISettingsActivationRequiredSurface(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetQuickstartCredits(ai.NewPersistentQuickstartCreditManager(
		persistence,
		"default",
		func() *config.AIConfig { return &config.AIConfig{Enabled: true} },
	))

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp AISettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ai settings: %v", err)
	}
	if resp.QuickstartBlockedReason != ai.QuickstartActivationRequiredReason() {
		t.Fatalf(
			"quickstart_blocked_reason=%q want %q",
			resp.QuickstartBlockedReason,
			ai.QuickstartActivationRequiredReason(),
		)
	}
	if resp.QuickstartCreditsRemaining != 0 || resp.QuickstartCreditsTotal != 0 {
		t.Fatalf("quickstart credits=%d/%d want 0/0", resp.QuickstartCreditsRemaining, resp.QuickstartCreditsTotal)
	}
	if resp.QuickstartCreditsAvailable {
		t.Fatal("expected quickstart_credits_available=false while activation is required")
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"quickstart_blocked_reason":"`+ai.QuickstartActivationRequiredReason()+`"`)) {
		t.Fatalf("expected AI settings payload to expose quickstart_blocked_reason, got %s", rec.Body.Bytes())
	}
}

func TestContract_AISettingsBYOKOverrideRetainsQuickstartInventoryJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "openai:gpt-4o"
	aiCfg.OpenAIAPIKey = "sk-openai-test"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetQuickstartCredits(&stubQuickstartCreditManager{
		remaining: 12,
		total:     pkglicensing.QuickstartCreditsTotal,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"openai:gpt-4o",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":true,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"configured_providers":["openai"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":25,
		"quickstart_credits_used":13,
		"quickstart_credits_remaining":12,
		"quickstart_credits_available":true,
		"using_quickstart":false
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_ChartMetricPointsPreserveMillisecondPrecision(t *testing.T) {
	pointTime := time.Date(2026, time.March, 31, 12, 0, 0, 987_000_000, time.UTC)

	converted := monitorPointsToAPI([]monitoring.MetricPoint{{
		Timestamp: pointTime,
		Value:     42,
	}})
	if len(converted) != 1 {
		t.Fatalf("expected one converted point, got %d", len(converted))
	}
	if converted[0].Timestamp != pointTime.UnixMilli() {
		t.Fatalf("expected millisecond timestamp %d, got %d", pointTime.UnixMilli(), converted[0].Timestamp)
	}
}

func TestContract_StorageChartsUseCanonicalMetricsTargetIDs(t *testing.T) {
	monitor, state, metricsHistory := newTestMonitor(t)
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)

	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "usage", 0.25, now)
	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "used", 3.57*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "avail", 11.03*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("pool:archive", "usage", 0.36, now)
	metricsHistory.AddStorageMetric("pool:archive", "used", 5.49*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("pool:archive", "avail", 4.51*1024*1024*1024*1024, now)

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateSnapshotAndSupplemental(state.GetSnapshot(), map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceVMware: {
			{
				SourceID: "vc-1:datastore:datastore-202",
				Resource: unifiedresources.Resource{
					ID:       "storage-vmware-1",
					Type:     unifiedresources.ResourceTypeStorage,
					Name:     "archive-tier",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					Storage: &unifiedresources.StorageMeta{
						Type:     "datastore",
						Platform: "vmware",
						Nodes:    []string{"esxi-01.lab.local"},
					},
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						EntityType:      "datastore",
						ManagedObjectID: "datastore-202",
						RuntimeHostName: "esxi-01.lab.local",
					},
				},
			},
		},
		unifiedresources.SourceTrueNAS: {
			{
				SourceID: "pool:archive",
				Resource: unifiedresources.Resource{
					ID:       "storage-truenas-1",
					Type:     unifiedresources.ResourceTypeStorage,
					Name:     "archive",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					Storage: &unifiedresources.StorageMeta{
						Type:     "zfs-pool",
						Platform: "truenas",
					},
					TrueNAS: &unifiedresources.TrueNASData{
						Hostname: "truenas-main",
					},
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/storage-charts?range=60", nil)
	rec := httptest.NewRecorder()
	router.handleStorageCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded StorageChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal storage charts response: %v", err)
	}
	if _, ok := decoded.Pools["vc-1:datastore:datastore-202"]; !ok {
		t.Fatalf("expected VMware datastore chart keyed by canonical metrics target, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["pool:archive"]; !ok {
		t.Fatalf("expected TrueNAS pool chart keyed by canonical metrics target, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["storage-vmware-1"]; ok {
		t.Fatalf("expected raw VMware resource id to stay out of chart payload, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["storage-truenas-1"]; ok {
		t.Fatalf("expected raw TrueNAS resource id to stay out of chart payload, got %v", decoded.Pools)
	}
}

func TestContract_TrueNASConnectionsDisabledMessageIsExplicit(t *testing.T) {
	setTrueNASFeatureForTest(t, false)
	handler, _, _ := newTrueNASHandlersForTest(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when TrueNAS integration is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "explicitly disabled") {
		t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
	}
}

func TestContract_TrueNASSavedConnectionTestsUpdateRuntimeSummary(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	connection := config.TrueNASInstance{
		ID:                 "conn-1",
		Name:               "tower",
		Host:               "truenas.local",
		Port:               443,
		APIKey:             "super-secret",
		UseHTTPS:           true,
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return poller }
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		return &fakeTrueNASClient{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved manual test to refresh poll summary, got %+v", summary.Poll)
	}
}

func TestContract_VMwareConnectionsDisabledMessageIsExplicit(t *testing.T) {
	setVMwareFeatureForTest(t, false)
	handler, _ := newVMwareHandlersForTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when VMware integration is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "explicitly disabled") {
		t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
	}
}

func TestContract_VMwareSavedConnectionTestsUpdateRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 3, VMs: 20, Datastores: 4, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved manual test to refresh runtime summary, got %+v", summary.Poll)
	}
	if summary.Observed == nil || summary.Observed.VMs != 20 {
		t.Fatalf("expected saved manual test to refresh observed summary, got %+v", summary.Observed)
	}
}

func TestContract_VMwareConnectionListCarriesObservedSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	collectedAt := time.Date(2026, 3, 30, 18, 0, 0, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", connection.ID, &vmware.InventorySummary{
		Hosts:      4,
		VMs:        24,
		Datastores: 6,
		VIRelease:  "8.0.3",
	}, collectedAt)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var responses []vmwareConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal vmware list response: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 vmware connection response, got %d", len(responses))
	}
	if responses[0].Password != "********" {
		t.Fatalf("expected redacted password in vmware list response, got %q", responses[0].Password)
	}
	if responses[0].Poll == nil || responses[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected poll summary in vmware list response, got %+v", responses[0])
	}
	if responses[0].Observed == nil {
		t.Fatalf("expected observed summary in vmware list response, got %+v", responses[0])
	}
	if got := responses[0].Observed.Hosts; got != 4 {
		t.Fatalf("observed hosts = %d, want 4", got)
	}
	if got := responses[0].Observed.VMs; got != 24 {
		t.Fatalf("observed vms = %d, want 24", got)
	}
	if got := responses[0].Observed.Datastores; got != 6 {
		t.Fatalf("observed datastores = %d, want 6", got)
	}
	if got := responses[0].Observed.VIRelease; got != "8.0.3" {
		t.Fatalf("observed viRelease = %q, want 8.0.3", got)
	}
	if responses[0].Observed.CollectedAt == nil || !responses[0].Observed.CollectedAt.Equal(collectedAt) {
		t.Fatalf("observed collectedAt = %+v, want %s", responses[0].Observed.CollectedAt, collectedAt.Format(time.RFC3339))
	}
}

func TestContract_InfrastructureChartsNormalizeLongRangeMixedCadence(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
		CPU:      0.75,
		Memory:   models.Memory{Usage: 42.0},
		Disk:     models.Disk{Usage: 55.0},
	}}
	syncTestResourceStore(t, monitor, state)

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-7 * 24 * time.Hour)
	seed := make([]metrics.WriteMetric, 0, 1200)
	appendMetric := func(ts time.Time, value float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "node",
				ResourceID:   "node-contract-1",
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		appendMetric(ts, 20)
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		appendMetric(ts, 40)
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		appendMetric(ts, 60)
	}
	appendMetric(now, 75)
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/infrastructure?range=7d", nil)
	rec := httptest.NewRecorder()
	router.handleInfrastructureCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded InfrastructureChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal infrastructure charts response: %v", err)
	}

	cpuSeries := decoded.NodeData["node-contract-1"]["cpu"]
	if len(cpuSeries) == 0 {
		t.Fatal("expected normalized cpu series")
	}
	if len(cpuSeries) > infrastructureSummaryMaxSeriesPoints {
		t.Fatalf("expected cpu series <= %d points, got %d", infrastructureSummaryMaxSeriesPoints, len(cpuSeries))
	}
	if cpuSeries[len(cpuSeries)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest cpu timestamp %d, got %d", now.UnixMilli(), cpuSeries[len(cpuSeries)-1].Timestamp)
	}
	if cpuSeries[len(cpuSeries)-1].Value != 75 {
		t.Fatalf("expected latest cpu value 75, got %.2f", cpuSeries[len(cpuSeries)-1].Value)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range cpuSeries {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected day-proportional recent summary buckets, got %d recent cpu points", recentCount)
	}
}

func TestContract_WorkloadChartsCapLongRangeMixedCadenceByTime(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
	}}
	state.VMs = []models.VM{{
		ID:         "vm-contract-1",
		VMID:       101,
		Name:       "vm-contract-1",
		Node:       "node-contract-1",
		Instance:   "pve1",
		Status:     "running",
		CPU:        0.75,
		Memory:     models.Memory{Usage: 42.0},
		Disk:       models.Disk{Usage: 55.0},
		NetworkIn:  128,
		NetworkOut: 256,
		DiskRead:   512,
		DiskWrite:  256,
	}}
	syncTestResourceStore(t, monitor, state)

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	vms := readState.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm view, got %d", len(vms))
	}
	sourceID := strings.TrimSpace(vms[0].SourceID())
	if sourceID == "" {
		t.Fatal("expected vm source ID")
	}

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-7 * 24 * time.Hour)
	seed := make([]metrics.WriteMetric, 0, 2000)
	appendMetric := func(ts time.Time, percentValue, rateValue float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        percentValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
		for _, metricType := range []string{"diskread", "diskwrite", "netin", "netout"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        rateValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		appendMetric(ts, 20, 50)
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		appendMetric(ts, 40, 75)
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		appendMetric(ts, 60, 100)
	}
	appendMetric(now, 75, 125)
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=7d&maxPoints=30", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal workload charts response: %v", err)
	}

	if len(decoded.ChartData) != 1 {
		t.Fatalf("expected 1 workload chart entry, got %d", len(decoded.ChartData))
	}
	var cpuSeries []MetricPoint
	for _, chartData := range decoded.ChartData {
		cpuSeries = chartData["cpu"]
		break
	}
	if len(cpuSeries) == 0 {
		t.Fatal("expected capped cpu series")
	}
	if len(cpuSeries) > 30 {
		t.Fatalf("expected cpu series <= 30 points, got %d", len(cpuSeries))
	}
	if cpuSeries[len(cpuSeries)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest cpu timestamp %d, got %d", now.UnixMilli(), cpuSeries[len(cpuSeries)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range cpuSeries {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 8 {
		t.Fatalf("expected day-proportional capped workload points, got %d recent cpu points", recentCount)
	}
}

func TestContract_WorkloadChartsUseCanonicalWorkloadIDsForProviderBackedVMs(t *testing.T) {
	monitor, state, history := newTestMonitor(t)
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	metricID := "vc-1:vm:vm-201"

	history.AddGuestMetric(metricID, "cpu", 51, now.Add(-10*time.Minute))
	history.AddGuestMetric(metricID, "memory", 64, now.Add(-5*time.Minute))
	history.AddGuestMetric(metricID, "disk", 43, now.Add(-3*time.Minute))
	history.AddGuestMetric(metricID, "netin", 1200, now.Add(-2*time.Minute))
	history.AddGuestMetric(metricID, "netout", 800, now.Add(-2*time.Minute))

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateSnapshotAndSupplemental(state.GetSnapshot(), map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceVMware: {
			{
				SourceID: metricID,
				Resource: unifiedresources.Resource{
					ID:       "vm-vmware-contract",
					Type:     unifiedresources.ResourceTypeVM,
					Name:     "warehouse-api-01",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					MetricsTarget: &unifiedresources.MetricsTarget{
						ResourceType: "vm",
						ResourceID:   metricID,
					},
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						EntityType:      "vm",
						ManagedObjectID: "vm-201",
					},
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil || len(readState.VMs()) != 1 || readState.VMs()[0] == nil {
		t.Fatalf("expected one provider-backed VM in read state, got %+v", readState)
	}
	resourceID, _, ok := vmChartRequest(readState.VMs()[0])
	if !ok {
		t.Fatal("expected canonical vm chart request")
	}

	router := &Router{monitor: monitor}

	workloadReq := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=1h", nil)
	workloadRec := httptest.NewRecorder()
	router.handleWorkloadCharts(workloadRec, workloadReq)

	if workloadRec.Code != http.StatusOK {
		t.Fatalf("expected workload charts 200, got %d: %s", workloadRec.Code, workloadRec.Body.String())
	}

	var workloadDecoded WorkloadChartsResponse
	if err := json.Unmarshal(workloadRec.Body.Bytes(), &workloadDecoded); err != nil {
		t.Fatalf("unmarshal workload charts response: %v", err)
	}
	if _, ok := workloadDecoded.ChartData[resourceID]; !ok {
		t.Fatalf("expected workload charts keyed by canonical workload id %q, got %v", resourceID, workloadDecoded.ChartData)
	}
	if _, ok := workloadDecoded.ChartData[metricID]; ok {
		t.Fatalf("expected provider metrics target %q to stay out of workload chart response keys", metricID)
	}
	if workloadDecoded.GuestTypes[resourceID] != "vm" {
		t.Fatalf("expected vm guest type for %q, got %q", resourceID, workloadDecoded.GuestTypes[resourceID])
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=1h", nil)
	summaryRec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(summaryRec, summaryReq)

	if summaryRec.Code != http.StatusOK {
		t.Fatalf("expected workloads summary 200, got %d: %s", summaryRec.Code, summaryRec.Body.String())
	}

	var summaryDecoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summaryDecoded); err != nil {
		t.Fatalf("unmarshal workloads summary response: %v", err)
	}
	if summaryDecoded.GuestCounts.Total != 1 || summaryDecoded.GuestCounts.Running != 1 {
		t.Fatalf("expected stable provider-backed guest counts, got %+v", summaryDecoded.GuestCounts)
	}
	if len(summaryDecoded.TopContributors.CPU) == 0 {
		t.Fatal("expected provider-backed cpu top contributor")
	}
	if summaryDecoded.TopContributors.CPU[0].ID != resourceID {
		t.Fatalf("expected workloads summary contributor id %q, got %+v", resourceID, summaryDecoded.TopContributors.CPU[0])
	}
	if summaryDecoded.TopContributors.CPU[0].ID == metricID {
		t.Fatalf("expected workloads summary contributor id to avoid raw metrics target %q", metricID)
	}
}

func TestContract_WorkloadsSummaryChartsNormalizeLongRangeMixedCadence(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
	}}
	state.VMs = []models.VM{{
		ID:         "vm-contract-1",
		VMID:       101,
		Name:       "vm-contract-1",
		Node:       "node-contract-1",
		Instance:   "pve1",
		Status:     "running",
		CPU:        0.75,
		Memory:     models.Memory{Usage: 42.0},
		Disk:       models.Disk{Usage: 55.0},
		NetworkIn:  128,
		NetworkOut: 256,
	}}
	syncTestResourceStore(t, monitor, state)

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	vms := readState.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm view, got %d", len(vms))
	}
	sourceID := strings.TrimSpace(vms[0].SourceID())
	if sourceID == "" {
		t.Fatal("expected vm source ID")
	}

	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-7 * 24 * time.Hour)
	seed := make([]metrics.WriteMetric, 0, 1500)
	appendMetric := func(ts time.Time, percentValue, rateValue float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        percentValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
		for _, metricType := range []string{"netin", "netout"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        rateValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for ts := windowStart; ts.Before(now.Add(-24 * time.Hour)); ts = ts.Add(65 * time.Minute) {
		appendMetric(ts, 20, 50)
	}
	for ts := now.Add(-24 * time.Hour); ts.Before(now.Add(-2 * time.Hour)); ts = ts.Add(2 * time.Minute) {
		appendMetric(ts, 40, 75)
	}
	for ts := now.Add(-2 * time.Hour); ts.Before(now); ts = ts.Add(time.Minute) {
		appendMetric(ts, 60, 100)
	}
	appendMetric(now, 75, 125)
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=7d", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal workloads summary charts response: %v", err)
	}

	if len(decoded.CPU.P50) == 0 {
		t.Fatal("expected normalized workload summary p50 series")
	}
	if len(decoded.CPU.P50) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected workload summary p50 <= %d points, got %d", workloadsSummaryMaxSeriesPoints, len(decoded.CPU.P50))
	}
	if len(decoded.CPU.P95) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected workload summary p95 <= %d points, got %d", workloadsSummaryMaxSeriesPoints, len(decoded.CPU.P95))
	}
	if decoded.CPU.P50[len(decoded.CPU.P50)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest workload summary timestamp %d, got %d", now.UnixMilli(), decoded.CPU.P50[len(decoded.CPU.P50)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range decoded.CPU.P50 {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected day-proportional summary buckets, got %d recent p50 points", recentCount)
	}
}

func TestContract_GenerateStyledMockSeries_UsesTimestampBasedCurve(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC).UnixMilli()

	coarse := generateStyledMockSeries(
		now,
		time.Hour,
		7,
		51.9,
		"dockerContainer",
		"orion-2-f54579833f9c",
		"memory",
	)
	fine := generateStyledMockSeries(
		now,
		time.Hour,
		13,
		51.9,
		"dockerContainer",
		"orion-2-f54579833f9c",
		"memory",
	)

	if len(coarse) != 7 || len(fine) != 13 {
		t.Fatalf("unexpected synthetic series lengths coarse=%d fine=%d", len(coarse), len(fine))
	}

	for i, point := range coarse {
		fineIndex := i * 2
		if fine[fineIndex].Timestamp != point.Timestamp {
			t.Fatalf(
				"expected shared timestamp at coarse[%d]=%d to match fine[%d]=%d",
				i,
				point.Timestamp,
				fineIndex,
				fine[fineIndex].Timestamp,
			)
		}
		if fine[fineIndex].Value != point.Value {
			t.Fatalf(
				"expected shared timestamp value at coarse[%d]=%f to match fine[%d]=%f",
				i,
				point.Value,
				fineIndex,
				fine[fineIndex].Value,
			)
		}
	}
}

func TestContract_PlatformMockToggleRebindsRuntimeConnectionsAndResources(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "false")
	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(false)
	t.Cleanup(func() {
		mock.SetEnabled(prevMock)
	})

	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	monitor.SetMockMode(false)

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")
	t.Cleanup(func() {
		router.shutdownBackgroundWorkers()
	})

	toggleReq := httptest.NewRequest(http.MethodPost, "/api/system/mock-mode", strings.NewReader(`{"enabled":true}`))
	toggleRec := httptest.NewRecorder()
	router.configHandlers.HandleUpdateMockMode(toggleRec, toggleReq)
	if toggleRec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, body=%s", toggleRec.Code, toggleRec.Body.String())
	}

	truenasListRec := httptest.NewRecorder()
	truenasListReq := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	router.trueNASHandlers.HandleList(truenasListRec, truenasListReq)
	if truenasListRec.Code != http.StatusOK {
		t.Fatalf("truenas connections status = %d, body=%s", truenasListRec.Code, truenasListRec.Body.String())
	}
	var truenasConnections []trueNASConnectionResponse
	if err := json.Unmarshal(truenasListRec.Body.Bytes(), &truenasConnections); err != nil {
		t.Fatalf("decode truenas connections: %v", err)
	}
	if len(truenasConnections) != 1 || truenasConnections[0].ID != "truenas-mock-1" {
		t.Fatalf("expected mock truenas connection, got %#v", truenasConnections)
	}

	vmwareListRec := httptest.NewRecorder()
	vmwareListReq := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	router.vmwareHandlers.HandleList(vmwareListRec, vmwareListReq)
	if vmwareListRec.Code != http.StatusOK {
		t.Fatalf("vmware connections status = %d, body=%s", vmwareListRec.Code, vmwareListRec.Body.String())
	}
	var vmwareConnections []vmwareConnectionResponse
	if err := json.Unmarshal(vmwareListRec.Body.Bytes(), &vmwareConnections); err != nil {
		t.Fatalf("decode vmware connections: %v", err)
	}
	if len(vmwareConnections) != 1 || vmwareConnections[0].ID != "vc-mock-1" {
		t.Fatalf("expected mock vmware connection, got %#v", vmwareConnections)
	}

	assertResourceSource := func(path string, wantSource unifiedresources.DataSource) {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.resourceHandlers.HandleListResources(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}

		var resp ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		for _, resource := range resp.Data {
			for _, source := range resource.Sources {
				if source == wantSource {
					return
				}
			}
		}
		t.Fatalf("expected %s response to include source %q, got %#v", path, wantSource, resp.Data)
	}

	assertResourceSource("/api/resources?source=truenas", unifiedresources.SourceTrueNAS)
	assertResourceSource("/api/resources?source=vmware-vsphere", unifiedresources.SourceVMware)

	assertResourceCount := func(path string, want int) {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.resourceHandlers.HandleListResources(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}

		var resp ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		if len(resp.Data) != want {
			t.Fatalf("%s returned %d resources, want %d", path, len(resp.Data), want)
		}
	}

	assertResourceCount("/api/resources?source=truenas&type=app-container", len(truenas.DefaultFixtures().Apps))
	assertResourceCount("/api/resources?source=vmware-vsphere&type=storage", len(vmware.DefaultFixtures().Datastores))
}

func TestContract_PlatformMockConnectionListsUseSharedFixtureMetadata(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setVMwareFeatureForTest(t, true)

	prevMock := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() {
		mock.SetEnabled(prevMock)
	})

	t.Run("truenas", func(t *testing.T) {
		fixture := mock.DefaultTrueNASConnectionFixture()
		if fixture.CollectedAt.IsZero() {
			t.Fatal("expected canonical truenas mock fixture timestamp")
		}

		handler, _, _ := newTrueNASHandlersForTest(t, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
		rec := httptest.NewRecorder()
		handler.HandleList(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var responses []trueNASConnectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
			t.Fatalf("decode truenas mock list response: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("expected 1 mock truenas connection, got %d", len(responses))
		}

		response := responses[0]
		if response.ID != fixture.ID || response.Name != fixture.Name || response.Host != fixture.Host || response.Port != fixture.Port {
			t.Fatalf("unexpected truenas mock connection metadata: got %+v want fixture %+v", response.TrueNASInstance, fixture)
		}
		if response.APIKey != "********" {
			t.Fatalf("expected redacted truenas api key, got %q", response.APIKey)
		}
		if response.Poll == nil || response.Poll.IntervalSeconds != fixture.PollIntervalSeconds {
			t.Fatalf("expected truenas poll interval %d, got %+v", fixture.PollIntervalSeconds, response.Poll)
		}
		if response.Poll.LastSuccessAt == nil || !response.Poll.LastSuccessAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected truenas last success at %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Poll)
		}
		if response.Observed == nil {
			t.Fatal("expected truenas observed summary")
		}
		if response.Observed.Host != fixture.Host ||
			response.Observed.ResourceID != fixture.ResourceID ||
			response.Observed.Systems != fixture.Systems ||
			response.Observed.StoragePools != fixture.StoragePools ||
			response.Observed.Datasets != fixture.Datasets ||
			response.Observed.Apps != fixture.Apps ||
			response.Observed.Disks != fixture.Disks ||
			response.Observed.RecoveryArtifacts != fixture.RecoveryArtifacts {
			t.Fatalf("unexpected truenas observed summary: got %+v want fixture %+v", response.Observed, fixture)
		}
		if response.Observed.CollectedAt == nil || !response.Observed.CollectedAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected truenas observed collectedAt %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Observed)
		}
	})

	t.Run("vmware", func(t *testing.T) {
		fixture := mock.DefaultVMwareConnectionFixture()
		if fixture.CollectedAt.IsZero() {
			t.Fatal("expected canonical vmware mock fixture timestamp")
		}

		handler, _ := newVMwareHandlersForTest(t)
		req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
		rec := httptest.NewRecorder()
		handler.HandleList(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var responses []vmwareConnectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
			t.Fatalf("decode vmware mock list response: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("expected 1 mock vmware connection, got %d", len(responses))
		}

		response := responses[0]
		if response.ID != fixture.ID || response.Name != fixture.Name || response.Host != fixture.Host || response.Port != fixture.Port || response.Username != fixture.Username {
			t.Fatalf("unexpected vmware mock connection metadata: got %+v want fixture %+v", response.VMwareVCenterInstance, fixture)
		}
		if response.Password != "********" {
			t.Fatalf("expected redacted vmware password, got %q", response.Password)
		}
		if response.Poll == nil || response.Poll.IntervalSeconds != fixture.PollIntervalSeconds {
			t.Fatalf("expected vmware poll interval %d, got %+v", fixture.PollIntervalSeconds, response.Poll)
		}
		if response.Poll.LastSuccessAt == nil || !response.Poll.LastSuccessAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected vmware last success at %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Poll)
		}
		if response.Observed == nil {
			t.Fatal("expected vmware observed summary")
		}
		if response.Observed.Hosts != fixture.Hosts ||
			response.Observed.VMs != fixture.VMs ||
			response.Observed.Datastores != fixture.Datastores ||
			response.Observed.VIRelease != fixture.VIRelease {
			t.Fatalf("unexpected vmware observed summary: got %+v want fixture %+v", response.Observed, fixture)
		}
		if response.Observed.CollectedAt == nil || !response.Observed.CollectedAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected vmware observed collectedAt %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Observed)
		}
	})
}

func TestContract_VMwareConnectionListCarriesDegradedObservedSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	collectedAt := time.Date(2026, 3, 31, 18, 15, 0, 0, time.UTC)
	handler.statusMu.Lock()
	handler.statuses = map[string]vmwareConnectionRuntimeStatus{
		connection.ID: {
			Poll: &monitoring.VMwareConnectionPollStatus{
				IntervalSeconds: 60,
				LastSuccessAt:   &collectedAt,
			},
			Observed: &monitoring.VMwareConnectionObservedSummary{
				CollectedAt: &collectedAt,
				Hosts:       4,
				VMs:         24,
				Datastores:  6,
				VIRelease:   "8.0.3",
				Degraded:    true,
				IssueCount:  3,
				Issues: []monitoring.VMwareConnectionObservedIssue{
					{
						Stage:       "signals",
						Category:    "permission",
						Message:     "VMware permissions are insufficient for HostSystem overall status",
						Occurrences: 2,
					},
					{
						Stage:       "topology",
						Category:    "unavailable",
						Message:     "VMware vm guest identity is temporarily unavailable",
						Occurrences: 1,
					},
				},
			},
		},
	}
	handler.statusMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var responses []vmwareConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal vmware degraded list response: %v", err)
	}
	if len(responses) != 1 || responses[0].Observed == nil {
		t.Fatalf("expected 1 vmware connection with degraded observed summary, got %+v", responses)
	}
	if !responses[0].Observed.Degraded {
		t.Fatalf("expected degraded observed summary, got %+v", responses[0].Observed)
	}
	if responses[0].Observed.IssueCount != 3 {
		t.Fatalf("observed issueCount = %d, want 3", responses[0].Observed.IssueCount)
	}
	if len(responses[0].Observed.Issues) != 2 {
		t.Fatalf("observed issues len = %d, want 2", len(responses[0].Observed.Issues))
	}
	if responses[0].Observed.Issues[0].Stage != "signals" || responses[0].Observed.Issues[0].Occurrences != 2 {
		t.Fatalf("unexpected first observed issue: %+v", responses[0].Observed.Issues[0])
	}
	if responses[0].Observed.Issues[1].Stage != "topology" || responses[0].Observed.Issues[1].Occurrences != 1 {
		t.Fatalf("unexpected second observed issue: %+v", responses[0].Observed.Issues[1])
	}
}

func TestContract_SSOTestRejectsMetadataURLWithUserinfo(t *testing.T) {
	called := make(chan struct{}, 1)
	metadataServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case called <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer metadataServer.Close()

	metadataURL, err := url.Parse(metadataServer.URL)
	if err != nil {
		t.Fatalf("parse metadata server url: %v", err)
	}
	metadataURL.User = url.UserPassword("user", "pass")

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: metadataURL.String(),
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected failed response, got success: %+v", resp)
	}

	select {
	case <-called:
		t.Fatal("expected SAML metadata URL with userinfo to be rejected before outbound fetch")
	default:
	}
}

func TestContract_SSOTestRejectsManualSLOURLWithUserinfo(t *testing.T) {
	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPSSOURL: "https://idp.example.com/sso",
			IDPSLOURL: "https://user:pass@idp.example.com/slo",
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected failed response, got success: %+v", resp)
	}
	if resp.Message != "Invalid SLO URL format" {
		t.Fatalf("expected invalid SLO URL message, got %+v", resp)
	}
}

func TestContract_SSOTestOIDCDiscoveryKeepsIssuerBasePath(t *testing.T) {
	var issuerURL string
	discoveryServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/realms/pulse/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": %q,
			"userinfo_endpoint": %q,
			"jwks_uri": %q,
			"scopes_supported": ["openid", "profile", "email"]
		}`, issuerURL, issuerURL+"/protocol/openid-connect/auth", issuerURL+"/protocol/openid-connect/token", issuerURL+"/protocol/openid-connect/userinfo", issuerURL+"/protocol/openid-connect/certs")
	}))
	defer discoveryServer.Close()
	issuerURL = discoveryServer.URL + "/auth/realms/pulse"

	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: issuerURL,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got %+v", resp)
	}
	if resp.Details == nil {
		t.Fatal("expected OIDC details in response")
	}
	if resp.Details.EntityID != issuerURL {
		t.Fatalf("issuer=%q, want %q", resp.Details.EntityID, issuerURL)
	}
}

func TestContract_SSOTestRejectsCrossOriginSAMLMetadataRedirect(t *testing.T) {
	targetCalled := make(chan struct{}, 1)
	target := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case targetCalled <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer target.Close()

	origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: origin.URL + "/metadata",
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	select {
	case <-targetCalled:
		t.Fatal("expected cross-origin SAML redirect to be rejected before fetching the target origin")
	default:
	}
}

func TestContract_SSOTestRejectsCrossOriginOIDCDiscoveryRedirect(t *testing.T) {
	targetCalled := make(chan struct{}, 1)
	var targetURL string
	target := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		select {
		case targetCalled <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": %q,
			"userinfo_endpoint": %q,
			"jwks_uri": %q,
			"scopes_supported": ["openid", "profile", "email"]
		}`, targetURL, targetURL+"/auth", targetURL+"/token", targetURL+"/userinfo", targetURL+"/jwks")
	}))
	defer target.Close()
	targetURL = target.URL

	origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: origin.URL,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	select {
	case <-targetCalled:
		t.Fatal("expected cross-origin OIDC redirect to be rejected before fetching the target origin")
	default:
	}
}

func TestContract_SSOLocalRedirectTargetsStayCanonical(t *testing.T) {
	if got := buildLocalRedirectTarget("https://evil.example.com/pwn", map[string]string{
		"oidc": "error",
	}); got != "/?oidc=error" {
		t.Fatalf("absolute redirect target = %q, want %q", got, "/?oidc=error")
	}

	if got := buildLocalRedirectTarget("/login?foo=bar#section", map[string]string{
		"saml": "success",
	}); got != "/login?foo=bar&saml=success#section" {
		t.Fatalf("local redirect target = %q, want %q", got, "/login?foo=bar&saml=success#section")
	}
}

func TestContract_FindingJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := now.Add(5 * time.Minute)
	resolvedAt := now.Add(10 * time.Minute)
	ackAt := now.Add(11 * time.Minute)
	snoozedUntil := now.Add(12 * time.Minute)
	lastInvestigated := now.Add(15 * time.Minute)
	lastRegression := now.Add(30 * time.Minute)

	payload := ai.Finding{
		ID:                     "finding-1",
		Key:                    "cpu-high",
		Severity:               ai.FindingSeverityCritical,
		Category:               ai.FindingCategoryPerformance,
		ResourceID:             "vm-100",
		ResourceName:           "web-server",
		ResourceType:           "vm",
		Node:                   "pve-1",
		Title:                  "High CPU usage",
		Description:            "CPU sustained above 95%",
		Recommendation:         "Investigate processes and load",
		Evidence:               "cpu=96%",
		Source:                 "ai-analysis",
		DetectedAt:             now,
		LastSeenAt:             lastSeen,
		ResolvedAt:             &resolvedAt,
		AutoResolved:           true,
		ResolveReason:          "No longer detected",
		AcknowledgedAt:         &ackAt,
		SnoozedUntil:           &snoozedUntil,
		AlertIdentifier:        "alert-1",
		DismissedReason:        "expected_behavior",
		UserNote:               "Runs nightly batch",
		TimesRaised:            4,
		Suppressed:             true,
		InvestigationSessionID: "inv-session-1",
		InvestigationStatus:    "completed",
		InvestigationOutcome:   "fix_queued",
		LastInvestigatedAt:     &lastInvestigated,
		InvestigationAttempts:  2,
		LoopState:              "remediation_planned",
		Lifecycle: []ai.FindingLifecycleEvent{
			{
				At:      now,
				Type:    "state_change",
				Message: "Moved to investigating",
				From:    "detected",
				To:      "investigating",
				Metadata: map[string]string{
					"from": "detected",
					"to":   "investigating",
				},
			},
		},
		RegressionCount:  1,
		LastRegressionAt: &lastRegression,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal finding: %v", err)
	}

	const want = `{
		"id":"finding-1",
		"key":"cpu-high",
		"severity":"critical",
		"category":"performance",
		"resource_id":"vm-100",
		"resource_name":"web-server",
		"resource_type":"vm",
		"node":"pve-1",
		"title":"High CPU usage",
		"description":"CPU sustained above 95%",
		"recommendation":"Investigate processes and load",
		"evidence":"cpu=96%",
		"source":"ai-analysis",
		"detected_at":"2026-02-08T13:14:15Z",
		"last_seen_at":"2026-02-08T13:19:15Z",
		"resolved_at":"2026-02-08T13:24:15Z",
		"auto_resolved":true,
		"resolve_reason":"No longer detected",
		"acknowledged_at":"2026-02-08T13:25:15Z",
		"snoozed_until":"2026-02-08T13:26:15Z",
		"alert_identifier":"alert-1",
		"dismissed_reason":"expected_behavior",
		"user_note":"Runs nightly batch",
		"times_raised":4,
		"suppressed":true,
		"investigation_session_id":"inv-session-1",
		"investigation_status":"completed",
		"investigation_outcome":"fix_queued",
		"last_investigated_at":"2026-02-08T13:29:15Z",
		"investigation_attempts":2,
		"loop_state":"remediation_planned",
		"lifecycle":[{"at":"2026-02-08T13:14:15Z","type":"state_change","message":"Moved to investigating","from":"detected","to":"investigating","metadata":{"from":"detected","to":"investigating"}}],
		"regression_count":1,
		"last_regression_at":"2026-02-08T13:44:15Z"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ApprovalJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	expires := now.Add(5 * time.Minute)
	decided := now.Add(2 * time.Minute)

	payload := approval.ApprovalRequest{
		ID:          "approval-1",
		ExecutionID: "exec-1",
		ToolID:      "tool-1",
		Command:     "rm -rf /tmp/cache",
		TargetType:  "agent",
		TargetID:    "host-1",
		TargetName:  "alpha",
		Context:     "Cleanup temporary cache",
		RiskLevel:   approval.RiskHigh,
		Status:      approval.StatusApproved,
		RequestedAt: now,
		ExpiresAt:   expires,
		DecidedAt:   &decided,
		DecidedBy:   "admin",
		DenyReason:  "not needed",
		CommandHash: "sha256:abc",
		Consumed:    true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal approval: %v", err)
	}

	const want = `{
		"id":"approval-1",
		"executionId":"exec-1",
		"toolId":"tool-1",
		"command":"rm -rf /tmp/cache",
			"targetType":"agent",
		"targetId":"host-1",
		"targetName":"alpha",
		"context":"Cleanup temporary cache",
		"riskLevel":"high",
		"status":"approved",
		"requestedAt":"2026-02-08T13:14:15Z",
		"expiresAt":"2026-02-08T13:19:15Z",
		"decidedAt":"2026-02-08T13:16:15Z",
		"decidedBy":"admin",
		"denyReason":"not needed",
		"commandHash":"sha256:abc",
		"consumed":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ApprovalListResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"approvals": []approval.ApprovalRequest{},
		"stats": map[string]int{
			"approved":   0,
			"denied":     0,
			"executions": 0,
			"expired":    0,
			"pending":    0,
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal approval list response: %v", err)
	}

	const want = `{
		"approvals":[],
		"stats":{
			"approved":0,
			"denied":0,
			"executions":0,
			"expired":0,
			"pending":0
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedSignupResponseJSONSnapshot(t *testing.T) {
	payload := hostedSignupResponse{
		OrgID:   "org-123",
		UserID:  "owner@example.com",
		Message: "Check your email for a magic link to finish signing in.",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal hosted signup response: %v", err)
	}

	const want = `{
		"org_id":"org-123",
		"user_id":"owner@example.com",
		"message":"Check your email for a magic link to finish signing in."
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_TrialStartHostedSignupRedirectContract(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: "https://billing.example.com/start-pro-trial?source=contract",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(
		context.WithValue(context.Background(), OrgIDContextKey, "default"),
	)
	rec := httptest.NewRecorder()
	h.HandleStartTrial(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	var payload APIError
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "trial_signup_required" {
		t.Fatalf("code=%q, want %q", payload.Code, "trial_signup_required")
	}
	if strings.TrimSpace(payload.Details["action_url"]) == "" {
		t.Fatal("expected action_url in contract payload")
	}
}

func TestContract_TrialStartRateLimitContract(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: "https://billing.example.com/start-pro-trial?source=contract",
	})
	h.trialLimiter = NewRateLimiter(1, time.Minute)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	firstRec := httptest.NewRecorder()
	h.HandleStartTrial(firstRec, firstReq)
	if firstRec.Code != http.StatusConflict {
		t.Fatalf("first status=%d, want %d: %s", firstRec.Code, http.StatusConflict, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	secondRec := httptest.NewRecorder()
	h.HandleStartTrial(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d, want %d: %s", secondRec.Code, http.StatusTooManyRequests, secondRec.Body.String())
	}

	retryAfter := secondRec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header")
	}

	var payload APIError
	if err := json.NewDecoder(secondRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "trial_rate_limited" {
		t.Fatalf("code=%q, want %q", payload.Code, "trial_rate_limited")
	}
	if payload.Details["retry_after_seconds"] != retryAfter {
		t.Fatalf("retry_after_seconds=%q, want %q", payload.Details["retry_after_seconds"], retryAfter)
	}
}

func TestContract_BillingStateQuickstartJSONSnapshot(t *testing.T) {
	grantedAt := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC).Unix()

	payload := billingState{
		Capabilities:               []string{"ai_autofix", "ai_patrol"},
		Limits:                     map[string]int64{"max_monitored_systems": 25},
		MetersEnabled:              []string{},
		PlanVersion:                "cloud_starter",
		SubscriptionState:          subscriptionStateActiveValue,
		QuickstartCreditsGranted:   true,
		QuickstartCreditsUsed:      3,
		QuickstartCreditsGrantedAt: &grantedAt,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal billing state: %v", err)
	}

	const want = `{
		"capabilities":["ai_autofix","ai_patrol"],
		"limits":{"max_monitored_systems":25},
		"meters_enabled":[],
		"plan_version":"cloud_starter",
		"subscription_state":"active",
		"quickstart_credits_granted":true,
		"quickstart_credits_used":3,
		"quickstart_credits_granted_at":1774449000
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedAISettingsAutoBootstrapJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")
	useTestQuickstartBootstrapServer(t, func(r *http.Request, reqBody map[string]any) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Fatalf("expected Bearer auth, got %q", authHeader)
		}
		if parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")), "."); len(parts) != 3 {
			t.Fatalf("expected entitlement JWT bearer token, got %q", authHeader)
		}
		if _, hasFingerprint := reqBody["instance_fingerprint"]; hasFingerprint {
			t.Fatalf("expected hosted quickstart bootstrap to omit instance_fingerprint, got %v", reqBody)
		}
		if got := reqBody["use_case"]; got != "patrol" {
			t.Fatalf("use_case=%v want patrol", got)
		}
	})

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: baseDir}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !persistence.HasAIConfig() {
		t.Fatal("expected hosted AI settings contract to persist canonical ai.enc bootstrap")
	}

	const want = `{
		"enabled":true,
		"model":"quickstart:pulse-hosted",
		"chat_model":"quickstart:pulse-hosted",
		"patrol_model":"quickstart:pulse-hosted",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":25,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":25,
		"quickstart_credits_available":true,
		"using_quickstart":true
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_AISettingsLegacyQuickstartAliasJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "quickstart:minimax-2.5m"
	aiCfg.ChatModel = "quickstart:minimax-2.5m"
	aiCfg.PatrolModel = "quickstart:minimax-2.5m"
	aiCfg.DiscoveryModel = "quickstart:minimax-2.5m"
	aiCfg.AutoFixModel = "quickstart:minimax-2.5m"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"quickstart:pulse-hosted",
		"chat_model":"quickstart:pulse-hosted",
		"patrol_model":"quickstart:pulse-hosted",
		"auto_fix_model":"quickstart:pulse-hosted",
		"configured":false,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":0,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":0,
		"quickstart_credits_available":false,
		"using_quickstart":false
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
	if bytes.Contains(rec.Body.Bytes(), []byte("quickstart:minimax-2.5m")) {
		t.Fatalf("expected AI settings payload to suppress legacy hosted quickstart aliases, got %s", rec.Body.Bytes())
	}
}

func TestContract_AISettingsOllamaAuthJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "openai:gpt-4o"
	aiCfg.PatrolModel = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://ollama.example:11434"
	aiCfg.OllamaUsername = "unai"
	aiCfg.OllamaPassword = "secret"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"openai:gpt-4o",
		"patrol_model":"ollama:llama3",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":true,
		"ollama_base_url":"http://ollama.example:11434",
		"ollama_username":"unai",
		"ollama_password_set":true,
		"configured_providers":["ollama"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":0,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":0,
		"quickstart_credits_available":false,
		"using_quickstart":false
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_VMInventoryExportCSVHeaders(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil)
	rec := httptest.NewRecorder()

	handler.HandleExportVMInventory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content type, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment; filename=\"vm-inventory-") {
		t.Fatalf("expected VM inventory attachment filename, got %q", got)
	}

	const want = "Resource ID,Instance,Node,Pool,VMID,VM Name,Status,CPU Cores,Memory Allocated Bytes,Disk Allocated Bytes,Disk Used Bytes,Disk Status Reason\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected VM inventory CSV header row:\nwant %q\ngot  %q", want, got)
	}
}

func TestContract_ReportingCatalogJSONSnapshot(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/catalog", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetReportingCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected json content type, got %q", got)
	}

	const want = `{
		"id":"advanced_reporting",
		"title":"Detailed Reporting",
		"description":"Generate performance reports and current-state exports across infrastructure and workloads.",
		"lockedState":{
			"title":"Advanced Reporting (Pro)",
			"description":"Generate PDF and CSV performance reports plus current-state VM inventory exports across infrastructure and workload resources."
		},
		"guidance":{
			"title":"Advanced Insights",
			"description":"Performance reports come from the historical metrics store, while VM inventory export captures the current runtime state for spreadsheet-friendly fleet reviews. Use reports for trends and the inventory export for current allocation and usage snapshots."
		},
		"performanceReport":{
			"id":"performance_reports",
			"title":"Performance Reports",
			"description":"Generate PDF summaries or CSV metric exports from historical monitoring data for one or more selected resources.",
			"singleResourceEndpoint":"/api/admin/reports/generate",
			"multiResourceEndpoint":"/api/admin/reports/generate-multi",
			"singleFilenamePrefix":"report",
			"singleFilenameSubject":"resource_id",
			"multiFilenamePrefix":"fleet-report",
			"filenameDateStyle":"utc_yyyymmdd",
			"formats":[
				{
					"value":"pdf",
					"label":"PDF Report"
				},
				{
					"value":"csv",
					"label":"CSV Data"
				}
			],
			"defaultFormat":"pdf",
			"ranges":[
				{
					"key":"24h",
					"label":"Last 24 Hours",
					"description":"Current-day operational summary for short-term regressions.",
					"windowHours":24
				},
				{
					"key":"7d",
					"label":"Last 7 Days",
					"description":"Weekly trend window for recent performance changes.",
					"windowHours":168
				},
				{
					"key":"30d",
					"label":"Last 30 Days",
					"description":"Monthly review window for sustained capacity or reliability shifts.",
					"windowHours":720
				}
			],
			"defaultRange":"24h",
			"multiResourceMax":50,
			"supportsMetricFilter":true,
			"supportsCustomTitle":true
		},
		"vmInventoryExport":{
			"id":"vm_inventory",
			"title":"VM Inventory Export",
			"description":"Export the current fleet-wide VM inventory as CSV using the canonical runtime model. Includes VM identity, placement, CPU, memory allocation, disk allocation, and disk usage columns.",
			"format":"csv",
			"exportEndpoint":"/api/admin/reports/inventory/vms/export",
			"filenamePrefix":"vm-inventory",
			"filenameDateStyle":"utc_yyyymmdd",
			"columns":[
				{
					"key":"resource_id",
					"label":"Resource ID",
					"description":"Canonical Pulse resource ID for the VM."
				},
				{
					"key":"instance",
					"label":"Instance",
					"description":"Configured Proxmox instance or cluster name."
				},
				{
					"key":"node",
					"label":"Node",
					"description":"Proxmox node currently hosting the VM."
				},
				{
					"key":"pool",
					"label":"Pool",
					"description":"Canonical Proxmox pool membership when the platform reports one."
				},
				{
					"key":"vmid",
					"label":"VMID",
					"description":"Numeric Proxmox VM identifier."
				},
				{
					"key":"vm_name",
					"label":"VM Name",
					"description":"Current VM display name from the runtime model."
				},
				{
					"key":"status",
					"label":"Status",
					"description":"Canonical runtime status for the VM."
				},
				{
					"key":"cpu_cores",
					"label":"CPU Cores",
					"description":"Allocated virtual CPU core count."
				},
				{
					"key":"memory_allocated_bytes",
					"label":"Memory Allocated Bytes",
					"description":"Configured memory allocation in bytes."
				},
				{
					"key":"disk_allocated_bytes",
					"label":"Disk Allocated Bytes",
					"description":"Total allocated disk capacity in bytes across the VM."
				},
				{
					"key":"disk_used_bytes",
					"label":"Disk Used Bytes",
					"description":"Current used disk bytes from the canonical runtime disk view."
				},
				{
					"key":"disk_status_reason",
					"label":"Disk Status Reason",
					"description":"Reason disk usage is partial or unavailable when the runtime cannot provide a full guest view."
				}
			]
		}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_PerformanceReportTransportUsesCatalogDefinition(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&metricType=+cpu+&title=+Node+report+",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	definition := reporting.DescribePerformanceReport()
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, fmt.Sprintf("attachment; filename=\"%s-node-1-", definition.SingleFilenamePrefix)) {
		t.Fatalf("expected canonical performance-report attachment filename, got %q", got)
	}
	if engine.lastReq.Format != definition.DefaultFormat {
		t.Fatalf("expected default format %q, got %q", definition.DefaultFormat, engine.lastReq.Format)
	}
	if engine.lastReq.MetricType != "cpu" || engine.lastReq.Title != "Node report" {
		t.Fatalf("expected trimmed canonical optional fields, got %+v", engine.lastReq)
	}
}

func TestContract_ReportingCatalogRouteAccessibleWithoutReportingFeature(t *testing.T) {
	rawToken := "reporting-catalog-contract-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/catalog", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for ungated reporting catalog route, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content type, got %q", got)
	}
}

func TestContract_PerformanceReportTransportUsesCatalogDefaultRange(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	end := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&end="+url.QueryEscape(end.Format(time.RFC3339)),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	definition := reporting.DescribePerformanceReport()
	if got := engine.lastReq.Start; !got.Equal(end.Add(-definition.DefaultRangeDuration())) {
		t.Fatalf("expected canonical default start time, got %s", got)
	}
	if !engine.lastReq.End.Equal(end) {
		t.Fatalf("expected canonical end time, got %s", engine.lastReq.End)
	}
}

func TestContract_PerformanceReportTransportRejectsInvalidTimeRange(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&start=2026-03-25T12:00:00Z&end=2026-03-25T11:00:00Z",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid-time-range response: %v", err)
	}
	if payload.Code != "invalid_time_range" {
		t.Fatalf("expected invalid_time_range code, got %q", payload.Code)
	}
	if payload.Error != "end must be after start" {
		t.Fatalf("expected canonical invalid_time_range message, got %q", payload.Error)
	}
}

func TestContract_PerformanceReportTransportRejectsOversizedMultiBody(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	padding := strings.Repeat("x", reportingMultiReportBodyMax)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/reporting/generate-multi",
		strings.NewReader(fmt.Sprintf(`{"resources":[{"resourceType":"vm","resourceId":"vm-1"}],"format":"pdf","title":"%s"}`, padding)),
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateMultiReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode oversized-body response: %v", err)
	}
	if payload.Code != "body_too_large" {
		t.Fatalf("expected body_too_large code, got %q", payload.Code)
	}
	if payload.Error != "Request body must be 1MB or less" {
		t.Fatalf("expected canonical oversized-body message, got %q", payload.Error)
	}
}

func TestContract_PerformanceReportTransportRejectsInvalidOptionalFieldWithStableCode(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&metricType="+url.QueryEscape("cpu usage"),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid optional-field response: %v", err)
	}
	if payload.Code != "invalid_metric_type" {
		t.Fatalf("expected invalid_metric_type code, got %q", payload.Code)
	}
	if payload.Error != "metricType must match [a-zA-Z0-9._:-]+ and be <= 64 chars" {
		t.Fatalf("expected canonical invalid_metric_type message, got %q", payload.Error)
	}
}

func TestContract_ReportingInvalidFormatErrorsUseCatalogDefinitions(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)

	reportReq := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?format=xlsx&resourceType=node&resourceId=node-1",
		nil,
	)
	reportRec := httptest.NewRecorder()
	handler.HandleGenerateReport(reportRec, reportReq)
	if reportRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid performance report format, got %d body=%s", reportRec.Code, reportRec.Body.String())
	}
	var reportPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(reportRec.Body).Decode(&reportPayload); err != nil {
		t.Fatalf("decode performance invalid-format response: %v", err)
	}
	if reportPayload.Error != reporting.DescribePerformanceReport().InvalidFormatError() {
		t.Fatalf("expected canonical performance invalid-format error, got %q", reportPayload.Error)
	}

	inventoryReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/reports/inventory/vms/export?format=pdf",
		nil,
	)
	inventoryRec := httptest.NewRecorder()
	handler.HandleExportVMInventory(inventoryRec, inventoryReq)
	if inventoryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid inventory format, got %d body=%s", inventoryRec.Code, inventoryRec.Body.String())
	}
	var inventoryPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(inventoryRec.Body).Decode(&inventoryPayload); err != nil {
		t.Fatalf("decode inventory invalid-format response: %v", err)
	}
	if inventoryPayload.Error != reporting.DescribeVMInventoryExport().InvalidFormatError() {
		t.Fatalf("expected canonical inventory invalid-format error, got %q", inventoryPayload.Error)
	}
}

func TestContract_HostedTenantAISettingsFallbackJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")
	useTestQuickstartBootstrapServer(t, func(r *http.Request, reqBody map[string]any) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Fatalf("expected Bearer auth, got %q", authHeader)
		}
		if parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")), "."); len(parts) != 3 {
			t.Fatalf("expected entitlement JWT bearer token, got %q", authHeader)
		}
		if _, hasFingerprint := reqBody["instance_fingerprint"]; hasFingerprint {
			t.Fatalf("expected hosted quickstart bootstrap to omit instance_fingerprint, got %v", reqBody)
		}
		if got := reqBody["use_case"]; got != "patrol" {
			t.Fatalf("use_case=%v want patrol", got)
		}
	})

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: baseDir}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-tenant"))
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !persistence.HasAIConfig() {
		t.Fatal("expected hosted tenant AI settings contract to persist tenant ai.enc bootstrap")
	}

	const want = `{
		"enabled":true,
		"model":"quickstart:pulse-hosted",
		"chat_model":"quickstart:pulse-hosted",
		"patrol_model":"quickstart:pulse-hosted",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"quickstart_credits_total":25,
		"quickstart_credits_used":0,
		"quickstart_credits_remaining":25,
		"quickstart_credits_available":true,
		"using_quickstart":true
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_StripeWebhookHandlersUseCanonicalRuntimeDataDir(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	persistence := config.NewMultiTenantPersistence(envDir)
	billingStore := config.NewFileBillingStore(envDir)
	rbacProvider := NewTenantRBACProvider(envDir)

	withExplicitDir := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, envDir)
	if got := filepath.Dir(withExplicitDir.deduper.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("explicit dedupe dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
	if got := filepath.Dir(withExplicitDir.index.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("explicit customer index dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}

	withEnvFallback := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, "")
	if got := filepath.Dir(withEnvFallback.deduper.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("env fallback dedupe dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
	if got := filepath.Dir(withEnvFallback.index.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("env fallback customer index dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
}

func TestContract_NotificationWebhookTestResponseJSONSnapshot(t *testing.T) {
	payload := map[string]interface{}{
		"success":  true,
		"status":   200,
		"response": "OK",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook test response: %v", err)
	}

	const want = `{
		"response":"OK",
		"status":200,
		"success":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_NotificationPushoverWebhookResponseJSONSnapshot(t *testing.T) {
	payload := map[string]interface{}{
		"id":      "hook-1",
		"name":    "Pushover",
		"url":     "https://api.pushover.net/1/messages.json",
		"service": "pushover",
		"customFields": map[string]string{
			"token": "app-token",
			"user":  "user-key",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pushover webhook response: %v", err)
	}

	const want = `{
		"customFields":{"token":"app-token","user":"user-key"},
		"id":"hook-1",
		"name":"Pushover",
		"service":"pushover",
		"url":"https://api.pushover.net/1/messages.json"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceIntelligenceIncludesRecentChanges(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	observedAt := time.Now().Add(-15 * time.Minute)
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:         "change-contract",
		ObservedAt: observedAt,
		ResourceID: "vm-100",
		Kind:       unifiedresources.ChangeRestart,
		SourceType: unifiedresources.SourcePlatformEvent,
		Reason:     "guest restarted",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	correlationDetector := correlation.NewDetector(correlation.Config{
		MinOccurrences:    1,
		CorrelationWindow: 2 * time.Hour,
		RetentionWindow:   24 * time.Hour,
	})
	correlationBase := observedAt.Add(-10 * time.Minute)
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "storage-1",
		ResourceName: "storage-1",
		ResourceType: "storage",
		EventType:    correlation.EventDiskFull,
		Timestamp:    correlationBase,
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		ResourceType: "vm",
		EventType:    correlation.EventRestart,
		Timestamp:    correlationBase.Add(1 * time.Minute),
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "storage-1",
		ResourceName: "storage-1",
		ResourceType: "storage",
		EventType:    correlation.EventDiskFull,
		Timestamp:    correlationBase.Add(2 * time.Minute),
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		ResourceType: "vm",
		EventType:    correlation.EventRestart,
		Timestamp:    correlationBase.Add(3 * time.Minute),
	})
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{ID: "public-1", Type: unifiedresources.ResourceTypeVM, Tags: []string{"public"}},
			{ID: "internal-1", Type: unifiedresources.ResourceTypeAgent, Agent: &unifiedresources.AgentData{Hostname: "agent-1"}},
		},
	})
	setUnexportedField(t, svc.GetPatrolService(), "correlationDetector", correlationDetector)
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence?resource_id=vm-100", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetIntelligence(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(recentChanges))
	}
	if _, ok := payload["policy_posture"]; ok {
		t.Fatal("did not expect policy_posture in resource intelligence response")
	}
	dependencies, ok := payload["dependencies"].([]interface{})
	if !ok {
		t.Fatalf("expected dependencies array in response, got %T", payload["dependencies"])
	}
	if len(dependencies) == 0 {
		t.Fatal("expected at least one dependency in response")
	}
	correlations, ok := payload["correlations"].([]interface{})
	if !ok {
		t.Fatalf("expected correlations array in response, got %T", payload["correlations"])
	}
	if len(correlations) == 0 {
		t.Fatal("expected at least one correlation in response")
	}
	firstCorrelation, ok := correlations[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected correlation object, got %T", correlations[0])
	}
	if firstCorrelation["event_pattern"] == "" {
		t.Fatal("expected correlation event_pattern in response")
	}
	if firstCorrelation["avg_delay"] == nil {
		t.Fatal("expected correlation avg_delay in response")
	}
	if firstCorrelation["confidence"] == nil {
		t.Fatal("expected correlation confidence in response")
	}
}

func TestContract_IntelligenceSummaryIncludesRecentChanges(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:         "change-summary",
		ObservedAt: time.Now().Add(-15 * time.Minute),
		ResourceID: "vm-100",
		Kind:       unifiedresources.ChangeRestart,
		SourceType: unifiedresources.SourcePlatformEvent,
		Reason:     "guest restarted",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				ID:   "public-1",
				Name: "public-vm",
				Type: unifiedresources.ResourceTypeVM,
				Tags: []string{"public"},
			},
			{
				ID:   "restricted-1",
				Name: "mail-gw",
				Type: unifiedresources.ResourceTypePMG,
				PMG:  &unifiedresources.PMGData{Hostname: "mail.internal"},
			},
		},
	})
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetIntelligence(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(recentChanges))
	}
	policyPosture, ok := payload["policy_posture"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected policy_posture object in response, got %T", payload["policy_posture"])
	}
	if got := int(policyPosture["total_resources"].(float64)); got != 2 {
		t.Fatalf("expected total_resources=2, got %d", got)
	}
}

func TestContract_RecentChangesEndpointUsesCanonicalTimeline(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-canonical",
		ObservedAt:    time.Now().Add(-25 * time.Minute),
		ResourceID:    "vm-canonical",
		Kind:          unifiedresources.ChangeRestart,
		From:          "running",
		To:            "restarting",
		SourceType:    unifiedresources.SourcePlatformEvent,
		SourceAdapter: unifiedresources.AdapterProxmox,
		Reason:        "guest restarted after maintenance",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				ID:   "vm-canonical",
				Name: "canonical-vm",
				Type: unifiedresources.ResourceTypeVM,
			},
		},
	})
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes?hours=1", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	changes, ok := payload["changes"].([]interface{})
	if !ok {
		t.Fatalf("expected changes array in response, got %T", payload["changes"])
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(changes))
	}
	change, ok := changes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object change, got %#v", changes[0])
	}
	if change["resource_name"] != "canonical-vm" {
		t.Fatalf("expected canonical resource name, got %#v", change["resource_name"])
	}
	if change["resource_type"] != string(unifiedresources.ResourceTypeVM) {
		t.Fatalf("expected resource type vm, got %#v", change["resource_type"])
	}
	if change["change_type"] != string(unifiedresources.ChangeRestart) {
		t.Fatalf("expected canonical change type, got %#v", change["change_type"])
	}
	if desc, ok := change["description"].(string); !ok || !strings.Contains(desc, "Restart") {
		t.Fatalf("expected canonical change description, got %#v", change["description"])
	}
}

func TestContract_AIIntelligenceCorrelationsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 30, 0, 0, time.UTC)
	payload := map[string]any{
		"correlations": []map[string]any{
			{
				"source_id":     "node-1",
				"source_name":   "node-1",
				"source_type":   "node",
				"target_id":     "vm-1",
				"target_name":   "vm-1",
				"target_type":   "vm",
				"event_pattern": "high_cpu -> restart",
				"occurrences":   1,
				"avg_delay":     "1m0s",
				"confidence":    0.1,
				"last_seen":     now,
				"description":   "When node-1 experiences high_cpu, vm-1 often follows within 1m0s",
			},
		},
		"count": 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal correlations response: %v", err)
	}

	const want = `{
		"correlations":[
			{
				"avg_delay":"1m0s",
				"confidence":0.1,
				"description":"When node-1 experiences high_cpu, vm-1 often follows within 1m0s",
				"event_pattern":"high_cpu -\u003e restart",
				"last_seen":"2026-03-18T17:30:00Z",
				"occurrences":1,
				"source_id":"node-1",
				"source_name":"node-1",
				"source_type":"node",
				"target_id":"vm-1",
				"target_name":"vm-1",
				"target_type":"vm"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MonitoredSystemLedgerJSONSnapshot(t *testing.T) {
	payload := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{
			{
				Name:   "Tower",
				Type:   "host",
				Status: "warning",
				StatusExplanation: MonitoredSystemLedgerStatusExplanation{
					Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
					Reasons: []MonitoredSystemLedgerStatusReason{
						{
							Kind:       "source-stale",
							Name:       "Tower",
							Type:       "host",
							Source:     "agent",
							Status:     "stale",
							ReportedAt: "2026-03-18T17:25:00Z",
							Summary:    "Agent data for Tower is stale (last reported 2026-03-18T17:25:00Z).",
						},
					},
				},
				LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
					Name:   "Tower",
					Type:   "host",
					Source: "agent",
					At:     "2026-03-18T17:30:00Z",
				},
				Source: "agent",
				Explanation: MonitoredSystemLedgerExplanation{
					Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
					Reasons: []MonitoredSystemLedgerExplanationReason{
						{
							Kind:    "standalone",
							Signal:  "single-top-level-view",
							Summary: "No overlapping top-level source matched this system.",
						},
					},
					Surfaces: []MonitoredSystemLedgerExplanationSurface{
						{
							Name:   "Tower",
							Type:   "host",
							Source: "agent",
						},
					},
				},
			},
		},
		Total: 1,
		Limit: 5,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal monitored system ledger response: %v", err)
	}

	const want = `{
		"systems":[
			{
				"name":"Tower",
				"type":"host",
				"status":"warning",
				"status_explanation":{
					"summary":"At least one included source is stale, so Pulse marks this monitored system as warning.",
					"reasons":[
						{
							"kind":"source-stale",
							"name":"Tower",
							"type":"host",
							"source":"agent",
							"status":"stale",
							"reported_at":"2026-03-18T17:25:00Z",
							"summary":"Agent data for Tower is stale (last reported 2026-03-18T17:25:00Z)."
						}
					]
				},
				"latest_included_signal":{
					"name":"Tower",
					"type":"host",
					"source":"agent",
					"at":"2026-03-18T17:30:00Z"
				},
				"source":"agent",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse sees one top-level host view from agent.",
					"reasons":[
						{
							"kind":"standalone",
							"signal":"single-top-level-view",
							"summary":"No overlapping top-level source matched this system."
						}
					],
					"surfaces":[
						{
							"name":"Tower",
							"type":"host",
							"source":"agent"
						}
					]
				}
			}
		],
		"total":1,
		"limit":5
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MonitoredSystemLedgerDoesNotEmitCompatibilityAliases(t *testing.T) {
	entry := monitoredSystemLedgerEntry(unifiedresources.MonitoredSystemRecord{
		Name:   "Tower",
		Type:   "host",
		Status: unifiedresources.StatusWarning,
		StatusExplanation: unifiedresources.MonitoredSystemStatusExplanation{
			Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
			Reasons: []unifiedresources.MonitoredSystemStatusReason{},
		},
		LastSeen: time.Date(2026, 3, 18, 17, 35, 0, 0, time.UTC),
		LatestIncludedSignal: unifiedresources.MonitoredSystemLatestSignal{
			Name:   "tower.local",
			Type:   "docker-host",
			Source: "docker",
			At:     time.Date(2026, 3, 18, 17, 30, 0, 0, time.UTC),
		},
		Source: "multiple",
		Explanation: unifiedresources.MonitoredSystemGroupingExplanation{
			Summary:  "Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
			Reasons:  []unifiedresources.MonitoredSystemGroupingReason{},
			Surfaces: []unifiedresources.MonitoredSystemGroupingSurface{},
		},
	})

	payload := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{entry},
		Total:   1,
		Limit:   5,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal monitored system ledger response: %v", err)
	}

	const want = `{
		"systems":[
			{
				"name":"Tower",
				"type":"host",
				"status":"warning",
				"status_explanation":{
					"summary":"At least one included source is stale, so Pulse marks this monitored system as warning.",
					"reasons":[]
				},
				"latest_included_signal":{
					"name":"tower.local",
					"type":"docker-host",
					"source":"docker",
					"at":"2026-03-18T17:30:00Z"
				},
				"source":"multiple",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
					"reasons":[],
					"surfaces":[]
				}
			}
		],
		"total":1,
		"limit":5
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResolveAuthEnvPathUsesCanonicalRuntimeDataDir(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	explicitDir := t.TempDir()
	if got := resolveAuthEnvPath(explicitDir); got != filepath.Join(explicitDir, ".env") {
		t.Fatalf("resolveAuthEnvPath(explicit) = %q, want %q", got, filepath.Join(explicitDir, ".env"))
	}

	if got := resolveAuthEnvPath(""); got != filepath.Join(envDir, ".env") {
		t.Fatalf("resolveAuthEnvPath(env fallback) = %q, want %q", got, filepath.Join(envDir, ".env"))
	}
}

func TestContract_ResolveAuthEnvWritePathsDeduplicatesCanonicalFallback(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	paths := resolveAuthEnvWritePaths("", "")
	if len(paths) != 1 {
		t.Fatalf("resolveAuthEnvWritePaths() len = %d, want 1", len(paths))
	}
	if want := filepath.Join(envDir, ".env"); paths[0] != want {
		t.Fatalf("resolveAuthEnvWritePaths()[0] = %q, want %q", paths[0], want)
	}
}

func TestContract_WriteAuthEnvFileFallsBackToDataPath(t *testing.T) {
	configPathFile := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(configPathFile, []byte("blocked"), 0600); err != nil {
		t.Fatalf("write blocked config path file: %v", err)
	}
	dataDir := t.TempDir()

	writtenPath, err := writeAuthEnvFile(configPathFile, dataDir, []byte("PULSE_AUTH_USER='pulse'\n"))
	if err != nil {
		t.Fatalf("writeAuthEnvFile() error = %v", err)
	}

	wantPath := filepath.Join(dataDir, ".env")
	if writtenPath != wantPath {
		t.Fatalf("writeAuthEnvFile() path = %q, want %q", writtenPath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("stat fallback auth env: %v", err)
	}
}

func TestContract_RecoveryTokenPersistenceJSONSnapshot(t *testing.T) {
	payload := []*RecoveryToken{
		{
			TokenHash: recoveryTokenHash("raw-recovery-token"),
			CreatedAt: time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC),
			ExpiresAt: time.Date(2026, 2, 8, 14, 14, 15, 0, time.UTC),
			Used:      true,
			UsedAt:    time.Date(2026, 2, 8, 13, 24, 15, 0, time.UTC),
			IP:        "192.168.1.10",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery token persistence: %v", err)
	}

	const want = `[
		{
			"token_hash":"59b5880d54ca8c991c09269834d59ea09ab4f467fd4d580a932cd70c5b993fa4",
			"created_at":"2026-02-08T13:14:15Z",
			"expires_at":"2026-02-08T14:14:15Z",
			"used":true,
			"used_at":"2026-02-08T13:24:15Z",
			"ip":"192.168.1.10"
		}
	]`

	assertJSONSnapshot(t, got, want)
}

func TestContract_PersistentAuthStoresRequireExplicitInitialization(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s should require explicit initialization", name)
			}
		}()
		fn()
	}

	assertPanics("session store", func() { _ = GetSessionStore() })
	assertPanics("csrf store", func() { _ = GetCSRFStore() })
	assertPanics("recovery token store", func() { _ = GetRecoveryTokenStore() })
}

func TestContract_HostedSessionAuthPrecedesAnonymousFallback(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	InitSessionStore(t.TempDir())

	store := GetSessionStore()
	sessionToken := generateSessionToken()
	store.CreateSession(sessionToken, 24*time.Hour, "contract-test", "127.0.0.1", "hosted-owner@example.com")
	record, err := config.NewAPITokenRecord("hosted-contract-token.12345678", "hosted-contract", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens/relay-mobile", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: sessionToken,
	})
	rec := httptest.NewRecorder()

	if !CheckAuth(cfg, rec, req) {
		t.Fatal("CheckAuth() = false, want true for valid hosted browser session")
	}
	if got := rec.Header().Get("X-Authenticated-User"); got != "hosted-owner@example.com" {
		t.Fatalf("X-Authenticated-User = %q, want hosted-owner@example.com", got)
	}
	if got := rec.Header().Get("X-Auth-Method"); got != "session" {
		t.Fatalf("X-Auth-Method = %q, want session", got)
	}
}

func TestContract_UniversalRateLimitStateIsScopedPerRouterConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	first := UniversalRateLimitMiddlewareWithConfig(newEndpointRateLimitConfig(), handler)
	second := UniversalRateLimitMiddlewareWithConfig(newEndpointRateLimitConfig(), handler)

	makeRequest := func(target http.Handler) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "198.51.100.25:12345"
		rec := httptest.NewRecorder()
		target.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 10; i++ {
		if rec := makeRequest(first); rec.Code != http.StatusOK {
			t.Fatalf("first router request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	if rec := makeRequest(first); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("first router overflow status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	if rec := makeRequest(second); rec.Code != http.StatusOK {
		t.Fatalf("second router first request status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestContract_RouterShutdownClosesOwnedPersistentAuthStoresAfterGlobalRebind(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	routerOne := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "1.0.0")
	routerTwo := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(routerTwo.shutdownBackgroundWorkers)

	if routerOne.sessionStore == nil || routerOne.csrfStore == nil {
		t.Fatal("routerOne should capture initialized persistent auth stores")
	}
	if routerOne.recoveryTokenStore == nil {
		t.Fatal("routerOne should capture initialized recovery token store")
	}
	if routerTwo.sessionStore == nil || routerTwo.csrfStore == nil {
		t.Fatal("routerTwo should capture initialized persistent auth stores")
	}
	if routerTwo.recoveryTokenStore == nil {
		t.Fatal("routerTwo should capture initialized recovery token store")
	}
	if routerOne.sessionStore == routerTwo.sessionStore {
		t.Fatal("router instances should not share the same session store after rebind")
	}
	if routerOne.csrfStore == routerTwo.csrfStore {
		t.Fatal("router instances should not share the same csrf store after rebind")
	}
	if routerOne.recoveryTokenStore == routerTwo.recoveryTokenStore {
		t.Fatal("router instances should not share the same recovery token store after rebind")
	}

	routerOne.shutdownBackgroundWorkers()

	select {
	case <-routerOne.sessionStore.workerDone:
	default:
		t.Fatal("routerOne session store worker should be closed after router shutdown")
	}

	select {
	case <-routerOne.csrfStore.workerDone:
	default:
		t.Fatal("routerOne csrf store worker should be closed after router shutdown")
	}

	select {
	case <-routerTwo.sessionStore.workerDone:
		t.Fatal("routerTwo session store should remain active when routerOne shuts down")
	default:
	}

	select {
	case <-routerTwo.csrfStore.workerDone:
		t.Fatal("routerTwo csrf store should remain active when routerOne shuts down")
	default:
	}

	select {
	case <-routerOne.recoveryTokenStore.stopCleanup:
	default:
		t.Fatal("routerOne recovery token store should be closed after router shutdown")
	}

	select {
	case <-routerTwo.recoveryTokenStore.stopCleanup:
		t.Fatal("routerTwo recovery token store should remain active when routerOne shuts down")
	default:
	}
}

func TestContract_HostedOrgManagerSessionCanMintRelayMobileToken(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := authpkg.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "platform-admin",
		AuthPass:   hashed,
	}

	mtp := config.NewMultiTenantPersistence(dataDir)
	org := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "operator-owner",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "operator-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)

	sessionToken := "relay-owner-session-" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "legacy-owner")

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/relay-mobile", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})

	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("relay mobile token route status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode relay mobile response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("expected relay mobile token in response")
	}
	if payload.Record.OwnerUserID != "legacy-owner" {
		t.Fatalf("ownerUserId = %q, want legacy-owner", payload.Record.OwnerUserID)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one stored token, got %d", len(cfg.APITokens))
	}
	if cfg.APITokens[0].OrgID != "org-a" {
		t.Fatalf("stored token orgId = %q, want org-a", cfg.APITokens[0].OrgID)
	}
}

func TestContract_RelayMobileScopeCanReadOnboardingDeepLink(t *testing.T) {
	rawToken := "relay-mobile-onboarding-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/deep-link", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "missing_scope") {
		t.Fatalf("relay mobile scope should satisfy onboarding deep-link gating, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestContract_RelayMobileScopeCannotReadApprovalDetail(t *testing.T) {
	rawToken := "relay-mobile-approval-detail-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals/approval-1", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("relay mobile scope should not satisfy approval detail gating, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
		t.Fatalf("relay mobile approval detail rejection should mention %q, got %s", config.ScopeAIExecute, rec.Body.String())
	}
}

func TestContract_UnifiedAgentReportResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"success":   true,
		"agentId":   "agent-123",
		"lastSeen":  "2026-02-08T13:14:15Z",
		"platform":  "linux",
		"osName":    "Debian GNU/Linux",
		"osVersion": "12",
		"config": map[string]any{
			"commandsEnabled": true,
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal unified agent report response: %v", err)
	}

	const want = `{
		"agentId":"agent-123",
		"config":{"commandsEnabled":true},
		"lastSeen":"2026-02-08T13:14:15Z",
		"osName":"Debian GNU/Linux",
		"osVersion":"12",
		"platform":"linux",
		"success":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedAgentLookupFailsClosedOnAmbiguousHostname(t *testing.T) {
	handler := newUnifiedAgentHandlerForTests(t,
		models.Host{ID: "host-1", Hostname: "webserver.corp.example.com", DisplayName: "Web One"},
		models.Host{ID: "host-2", Hostname: "webserver.other.example.com", DisplayName: "Web Two"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?hostname=webserver", nil)
	rec := httptest.NewRecorder()

	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected not found status for ambiguous hostname lookup, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ambiguous lookup error response: %v", err)
	}
	if resp.Code != "agent_not_found" {
		t.Fatalf("expected agent_not_found code, got %q", resp.Code)
	}
}

func TestContract_HostsShareResolvedIdentityTreatsLoopbackAliasAsSameNode(t *testing.T) {
	if !hostsShareResolvedIdentity("https://localhost:7655", "https://127.0.0.1:7655") {
		t.Fatal("expected localhost and loopback IP to resolve as the same host identity")
	}
	if hostsShareResolvedIdentity("https://192.0.2.10:7655", "https://192.0.2.11:7655") {
		t.Fatal("expected different IP endpoints to remain distinct host identities")
	}
}

func TestContract_DiagnosticsDockerPrepareTokenInstallCommandUsesLifecycleTransport(t *testing.T) {
	baseURL := "https://pulse.example.com/base"
	got := buildContainerRuntimeAgentInstallCommand(baseURL, "token-123")

	if !strings.Contains(got, posixShellQuote(baseURL+"/install.sh")) {
		t.Fatalf("install command missing normalized install script URL: %s", got)
	}
	if !strings.Contains(got, "--enable-host=false") {
		t.Fatalf("install command missing canonical host-disable flag: %s", got)
	}
	if strings.Contains(got, "--disable-host") {
		t.Fatalf("install command preserved stale disable-host flag: %s", got)
	}
	if !strings.Contains(got, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`) {
		t.Fatalf("install command missing governed root-or-sudo wrapper: %s", got)
	}
	if strings.Contains(got, "curl -fsSL "+posixShellQuote(baseURL+"/install.sh")+" | sudo bash -s --") {
		t.Fatalf("install command preserved raw sudo pipe instead of governed wrapper: %s", got)
	}
}

func TestContract_DiagnosticsDockerPrepareTokenOptionalAuthInstallCommandOmitsToken(t *testing.T) {
	got := buildContainerRuntimeAgentInstallCommand("http://pulse.example.com:7655/", "")

	if strings.Contains(got, "--token") {
		t.Fatalf("optional-auth install command preserved token flag: %s", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Fatalf("optional-auth install command missing insecure flag for plain HTTP Pulse URL: %s", got)
	}
}

func TestContract_SetupScriptURLCommandUsesFailFastQuotedTransport(t *testing.T) {
	url := "https://pulse.example.com/api/setup-script?type=pve&host=pve1.local"
	got := buildSetupScriptCommand(url, "token-123")

	if !strings.Contains(got, "curl -fsSL "+posixShellQuote(url)+" | ") {
		t.Fatalf("setup-script command missing canonical fail-fast transport: %s", got)
	}
	if !strings.Contains(got, `if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`) {
		t.Fatalf("setup-script command missing direct-root execution path: %s", got)
	}
	if !strings.Contains(got, `elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`) {
		t.Fatalf("setup-script command missing sudo execution path: %s", got)
	}
	if strings.Contains(got, "curl -sSL ") {
		t.Fatalf("setup-script command preserved stale non-fail-fast curl transport: %s", got)
	}
}

func TestContract_SetupScriptEmbedsFailFastGuidance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	script := rec.Body.String()
	if !strings.Contains(script, `PULSE_BOOTSTRAP_COMMAND_WITH_ENV='curl -fsSL '"'"'http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve'"'"' | `) {
		t.Fatalf("setup script missing canonical bootstrap command owner: %s", script)
	}
	if strings.Contains(script, `PULSE_BOOTSTRAP_COMMAND_WITH_ENV='curl -fsSL '"'"'http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve'"'"' | { if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`) {
		t.Fatalf("setup script bootstrap command should defer setup token to runtime hydration, got: %s", script)
	}
	if !strings.Contains(script, `echo "  $PULSE_BOOTSTRAP_COMMAND_WITH_ENV"`) {
		t.Fatalf("setup script missing bootstrap-command retry guidance: %s", script)
	}
	if !strings.Contains(script, `SETUP_SCRIPT_URL="http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve"`) {
		t.Fatalf("setup script missing canonical encoded retry URL: %s", script)
	}
	if !strings.Contains(script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-`) {
		t.Fatalf("setup script missing canonical PVE setup-token initialization before rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Root privileges required. Run as root (su -) and retry."`) {
		t.Fatalf("setup script missing canonical root requirement guidance: %s", script)
	}
	if !strings.Contains(script, `echo "This setup flow must run on the Proxmox host so Pulse can create"`) {
		t.Fatalf("setup script missing canonical off-host rerun guidance: %s", script)
	}
	if strings.Contains(script, `echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"`) || strings.Contains(script, `echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=YOUR_PVE_URL&pulse_url=$PULSE_URL\" | bash"`) {
		t.Fatalf("setup script preserved stale non-fail-fast guidance: %s", script)
	}
	if strings.Contains(script, `echo "Manual setup steps:"`) || strings.Contains(script, `echo "  2. In Pulse: Settings → Nodes → Add Node (enter token from above)"`) {
		t.Fatalf("setup script preserved stale off-host manual token flow: %s", script)
	}
	if !strings.Contains(script, `done <<< "$OLD_TOKENS_PVE"`) {
		t.Fatalf("setup script missing explicit pve old-token cleanup loop: %s", script)
	}
	if !strings.Contains(script, `done <<< "$OLD_TOKENS_PAM"`) {
		t.Fatalf("setup script missing explicit pam old-token cleanup loop: %s", script)
	}
	if strings.Contains(script, `done <<< "$OLD_TOKENS"`) {
		t.Fatalf("setup script preserved stale undefined old-token cleanup variable: %s", script)
	}
	if !strings.Contains(script, `pveum user token remove pulse-monitor@pve "$TOKEN"`) {
		t.Fatalf("setup script missing pve token cleanup command: %s", script)
	}
	if !strings.Contains(script, `pveum user token remove pulse-monitor@pam "$TOKEN"`) {
		t.Fatalf("setup script missing pam token cleanup command: %s", script)
	}
	if !strings.Contains(script, `TOKEN_MATCH_PREFIX="pulse-sentinel-url"`) {
		t.Fatalf("setup script missing canonical token-match prefix for cleanup discovery: %s", script)
	}
	if !strings.Contains(script, `grep -E "^${TOKEN_MATCH_PREFIX}(-[0-9]+)?$"`) {
		t.Fatalf("setup script missing canonical cleanup token discovery matcher: %s", script)
	}
	if !strings.Contains(script, `awk 'NR>3 {print $2}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("setup script missing exact PVE token rotation detection: %s", script)
	}
	if strings.Contains(script, `PULSE_IP_PATTERN=`) {
		t.Fatalf("setup script preserved stale ip-pattern cleanup discovery: %s", script)
	}
	if strings.Contains(script, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("setup script preserved stale broad PVE token rotation detection: %s", script)
	}
	if strings.Contains(script, `echo "Please run this script as root"`) {
		t.Fatalf("setup script preserved stale root-only guidance: %s", script)
	}
	if !strings.Contains(script, `grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'`) {
		t.Fatalf("setup script missing secure success detection: %s", script)
	}
	if strings.Contains(script, `grep -q "success"`) {
		t.Fatalf("setup script preserved broad success substring detection: %s", script)
	}
	if !strings.Contains(script, `curl -fsS -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("setup script missing fail-fast auto-register transport: %s", script)
	}
	if !strings.Contains(script, `"source":"script"`) {
		t.Fatalf("setup script missing canonical /api/auto-register source marker: %s", script)
	}
	if !strings.Contains(script, `REGISTER_RC=$?`) {
		t.Fatalf("setup script missing explicit auto-register curl exit-code handling: %s", script)
	}
	if !strings.Contains(script, `echo "⚠️  Auto-registration skipped: token value unavailable"`) {
		t.Fatalf("setup script missing fail-closed token-value-unavailable guidance: %s", script)
	}
	if strings.Contains(script, `curl -s -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("setup script preserved stale non-fail-fast auto-register transport: %s", script)
	}
	if !strings.Contains(script, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("setup script missing invalid setup-token guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("setup script missing fresh setup-token rerun guidance: %s", script)
	}
	if !strings.Contains(script, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("setup script missing PVE auth-failure state tracking: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("setup script missing PVE auth-failure completion guidance: %s", script)
	}
	if !strings.Contains(script, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("setup script missing PVE auth-failure footer guard: %s", script)
	}
	if !strings.Contains(script, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("setup script missing canonical auto-register failure continuation guidance: %s", script)
	}
	if strings.Contains(script, `echo "To enable auto-registration, add your API token to the setup URL"`) {
		t.Fatalf("setup script preserved stale API-token auth guidance: %s", script)
	}
	if strings.Contains(script, `echo "The provided API token was invalid"`) {
		t.Fatalf("setup script preserved stale invalid API-token guidance: %s", script)
	}
	if strings.Contains(script, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("setup script preserved stale split setup-token auth guidance: %s", script)
	}
	if strings.Contains(script, `echo "📝 For manual setup:"`) {
		t.Fatalf("setup script preserved stale numbered manual-setup fallback: %s", script)
	}
	if strings.Contains(script, `echo "   2. Add this node manually in Pulse Settings"`) {
		t.Fatalf("setup script preserved stale auto-register failure continuation guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script missing truthful manual completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("setup script missing token-create failure completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("setup script missing immediate token-create failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-create failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-extract failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Successfully registered with Pulse monitoring."`) {
		t.Fatalf("setup script missing canonical success messaging: %s", script)
	}
	if !strings.Contains(script, `echo "  Token Value: [See token output above]"`) {
		t.Fatalf("setup script missing canonical token placeholder guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Finish registration in Pulse using the manual setup details below."`) {
		t.Fatalf("setup script missing truthful manual registration guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script missing canonical manual-add heading: %s", script)
	}
	if !strings.Contains(script, `echo "Use these details in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("setup script missing canonical manual-add continuation guidance: %s", script)
	}
	if !strings.Contains(script, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("setup script missing canonical auto-register failure summary: %s", script)
	}
	if !strings.Contains(script, `echo "  Host URL: $SERVER_HOST"`) {
		t.Fatalf("setup script missing canonical manual host continuity: %s", script)
	}
	if strings.Contains(script, `echo "Manual setup instructions:"`) {
		t.Fatalf("setup script preserved stale manual setup heading: %s", script)
	}
	if strings.Contains(script, `echo "Node registered successfully"`) || strings.Contains(script, `echo "Node successfully registered with Pulse monitoring."`) || strings.Contains(script, `echo "✅ Successfully registered with Pulse!"`) || strings.Contains(script, `echo "Server successfully registered with Pulse monitoring."`) {
		t.Fatalf("setup script preserved stale success copy variants: %s", script)
	}
	if strings.Contains(script, `echo "   Token Value: [See above]"`) || strings.Contains(script, `echo "  Token Value: [Check the output above for the token or instructions]"`) {
		t.Fatalf("setup script preserved stale token placeholder guidance: %s", script)
	}
	if strings.Contains(script, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("setup script preserved stale auto-register failure summary: %s", script)
	}
	if strings.Contains(script, `PULSE_REG_TOKEN=your-token ./setup.sh`) {
		t.Fatalf("setup script preserved stale rerun token guidance: %s", script)
	}
	if strings.Contains(script, `echo "Manual registration may be required."`) {
		t.Fatalf("setup script preserved stale manual-registration token failure guidance: %s", script)
	}
	if strings.Contains(script, `echo "  Host URL: YOUR_PROXMOX_HOST:8006"`) {
		t.Fatalf("setup script preserved stale placeholder manual host guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("setup script missing token-extract failure completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-extract completion rerun guidance: %s", script)
	}
	if !strings.Contains(script, `if [ "$TOKEN_READY" = true ]; then
    attempt_auto_registration
else
    AUTO_REG_SUCCESS=false
fi`) {
		t.Fatalf("setup script does not skip PVE auto-registration when no usable token is ready: %s", script)
	}
	if !strings.Contains(script, `if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script does not gate PVE manual token details on usable token extraction: %s", script)
	}
	if strings.Contains(script, `elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script lets PVE token-extract failure fall through to completed token setup: %s", script)
	}

	pbsReq := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://sentinel-pbs:8007&pulse_url=http://sentinel-url:7656", nil)
	pbsRec := httptest.NewRecorder()

	handlers.HandleSetupScript(pbsRec, pbsReq)

	if pbsRec.Code != http.StatusOK {
		t.Fatalf("pbs status = %d, want %d", pbsRec.Code, http.StatusOK)
	}

	pbsScript := pbsRec.Body.String()
	if !strings.Contains(pbsScript, `echo "  Host URL: $HOST_URL"`) {
		t.Fatalf("setup script missing canonical PBS manual host continuity: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("setup script missing PBS token-create failure completion messaging: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("setup script missing PBS token-extract failure completion messaging: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: no setup token provided"`) {
		t.Fatalf("setup script missing PBS setup-token-skip guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-`) {
		t.Fatalf("setup script missing canonical PBS setup-token initialization before rerun guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: no setup token provided"
                    AUTO_REG_SUCCESS=false
                    REGISTER_RESPONSE=""
                    REGISTER_RC=1`) {
		t.Fatalf("setup script still forces fake PBS request-failure state after missing setup-token skip: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: token value unavailable"
                AUTO_REG_SUCCESS=false
                REGISTER_RESPONSE=""
                REGISTER_RC=1`) {
		t.Fatalf("setup script still forces fake PBS request-failure state after token-value skip: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$REGISTER_ATTEMPTED" != true ]; then`) {
		t.Fatalf("setup script does not distinguish skipped PBS auto-registration paths from attempted requests: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("setup script missing invalid PBS setup-token guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("setup script missing fresh PBS setup-token rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("setup script missing PBS auth-failure state tracking: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("setup script missing PBS auth-failure completion guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("setup script missing PBS auth-failure footer guard: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS immediate token-create failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-create failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-extract failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-extract completion rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) {
		t.Fatalf("setup script missing canonical PBS host binding: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `TOKEN_MATCH_PREFIX="pulse-sentinel-url"`) {
		t.Fatalf("pbs setup script missing canonical token-match prefix for cleanup discovery: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `grep -oE "${TOKEN_MATCH_PREFIX}(-[0-9]+)?" | sort -u || true`) {
		t.Fatalf("pbs setup script missing canonical cleanup token discovery matcher: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `awk '{print $1}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("pbs setup script missing exact token rotation detection: %s", pbsScript)
	}
	tokenCreateIndex := strings.Index(pbsScript, `TOKEN_CREATE_RC=$?`)
	bannerIndex := strings.Index(pbsScript, `echo "IMPORTANT: Copy the token value below - it's only shown once!"`)
	successBranchIndex := strings.Index(pbsScript, "else\n    TOKEN_CREATED=true")
	if tokenCreateIndex == -1 || bannerIndex == -1 || successBranchIndex == -1 {
		t.Fatalf("pbs setup script missing token-create truth markers: %s", pbsScript)
	}
	if bannerIndex < tokenCreateIndex {
		t.Fatalf("pbs setup script prints token-copy banner before token creation result is known: %s", pbsScript)
	}
	if bannerIndex < successBranchIndex {
		t.Fatalf("pbs setup script prints token-copy banner outside the successful token-create branch: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `PULSE_IP_PATTERN=`) {
		t.Fatalf("pbs setup script preserved stale ip-pattern cleanup discovery: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("pbs setup script preserved stale broad token rotation detection: %s", pbsScript)
	}
	if strings.Index(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) > strings.Index(pbsScript, `if [ -z "$PULSE_SETUP_TOKEN" ]; then`) {
		t.Fatalf("setup script binds PBS host too late for manual fallback continuity: %s", pbsScript)
	}
	if strings.Index(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) > strings.Index(pbsScript, `if [ "$TOKEN_CREATE_RC" -ne 0 ]; then`) {
		t.Fatalf("setup script binds PBS host too late for token-create failure continuity: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script does not gate PBS manual token details on usable token extraction: %s", pbsScript)
	}
	attemptBannerIndex := strings.Index(pbsScript, `echo "🔄 Attempting auto-registration with Pulse..."`)
	authTokenGateIndex := strings.Index(pbsScript, `if [ -n "$PULSE_SETUP_TOKEN" ]; then`)
	tokenSkipIndex := strings.Index(pbsScript, `echo "⚠️  Auto-registration skipped: token value unavailable"`)
	if attemptBannerIndex == -1 || authTokenGateIndex == -1 || tokenSkipIndex == -1 {
		t.Fatalf("setup script missing PBS auto-registration truth markers: %s", pbsScript)
	}
	if attemptBannerIndex < authTokenGateIndex {
		t.Fatalf("setup script prints PBS auto-registration attempt banner before the real request path: %s", pbsScript)
	}
	if attemptBannerIndex < tokenSkipIndex {
		t.Fatalf("setup script prints PBS auto-registration attempt banner before token-unavailable skip handling: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script lets PBS token-extract failure fall through to completed token setup: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "  Host URL: https://$SERVER_IP:8007"`) {
		t.Fatalf("setup script preserved stale PBS runtime-IP host guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "Manual registration may be required."`) {
		t.Fatalf("setup script preserved stale PBS manual-registration token failure guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("setup script preserved stale split PBS setup-token auth guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("setup script missing canonical PBS auto-register failure summary: %s", pbsScript)
	}
	if strings.Count(pbsScript, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) < 2 {
		t.Fatalf("setup script missing canonical PBS request-failure/manual-response continuity: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("setup script preserved stale PBS auto-register failure summary: %s", pbsScript)
	}
}

func TestContract_SetupScriptRequiresCanonicalHost(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Missing required parameter: host" {
		t.Fatalf("body = %q, want canonical missing host guidance", got)
	}
}

func TestContract_SetupScriptUsesCanonicalTypeAndHostValidation(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	invalidTypeReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pmg&host=https://node.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	invalidTypeRec := httptest.NewRecorder()
	handlers.HandleSetupScript(invalidTypeRec, invalidTypeReq)

	if invalidTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d, want %d", invalidTypeRec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(invalidTypeRec.Body.String()); got != "type must be 'pve' or 'pbs'" {
		t.Fatalf("invalid type body = %q, want canonical type guidance", got)
	}

	normalizedHostReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https://pve-node.example.internal&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	normalizedHostRec := httptest.NewRecorder()
	handlers.HandleSetupScript(normalizedHostRec, normalizedHostReq)

	if normalizedHostRec.Code != http.StatusOK {
		t.Fatalf("normalized host status = %d, want %d: %s", normalizedHostRec.Code, http.StatusOK, normalizedHostRec.Body.String())
	}
	body := normalizedHostRec.Body.String()
	if !strings.Contains(body, `SERVER_HOST="https://pve-node.example.internal:8006"`) {
		t.Fatalf("normalized host body missing canonical host, got: %s", truncate(body, 500))
	}
	if !strings.Contains(body, `SETUP_SCRIPT_URL="https://pulse.example.com:7655/api/setup-script?host=https%3A%2F%2Fpve-node.example.internal%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7655&type=pve"`) {
		t.Fatalf("normalized host body missing canonical rerun URL, got: %s", truncate(body, 700))
	}
}

func TestContract_SetupScriptRequiresCanonicalPulseURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://pve.local:8006", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Missing required parameter: pulse_url" {
		t.Fatalf("body = %q, want canonical missing pulse_url guidance", got)
	}
}

func TestContract_SetupScriptUsesCanonicalShellDownloadHeaders(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://sentinel-pbs:8007&pulse_url=http://sentinel-url:7656", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/x-shellscript; charset=utf-8" {
		t.Fatalf("setup-script content type = %q, want %q", got, "text/x-shellscript; charset=utf-8")
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=\"pulse-setup-pbs.sh\"" {
		t.Fatalf("setup-script content disposition = %q, want %q", got, "attachment; filename=\"pulse-setup-pbs.sh\"")
	}
}

func TestContract_SetupScriptDerivesRenderedServerNameFromCanonicalHost(t *testing.T) {
	handlers := newTestConfigHandlers(t, &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https://derived-pve.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	script := rec.Body.String()
	if !strings.Contains(script, "# Pulse Monitoring Setup Script for derived-pve.example.internal") {
		t.Fatalf("setup script missing derived canonical host label: %s", script)
	}
	if strings.Contains(script, "# Pulse Monitoring Setup Script for your-server") {
		t.Fatalf("setup script preserved placeholder server label for canonical host: %s", script)
	}
}

func TestContract_AssignProfileRejectsMissingProfile(t *testing.T) {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	handler := NewConfigProfileHandler(mtp)
	body := bytes.NewBufferString(`{"agent_id":"agent-1","profile_id":"missing-profile"}`)
	req := httptest.NewRequest(http.MethodPost, "/assignments", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Profile not found" {
		t.Fatalf("body = %q, want %q", got, "Profile not found")
	}
}

func TestContract_ResolveLoopbackAwarePublicBaseURLPreservesConfiguredHTTPS(t *testing.T) {
	cfg := &config.Config{
		PublicURL:    "https://public.example.com/base/",
		FrontendPort: 7655,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:7655"

	if got := resolveLoopbackAwarePublicBaseURL(req, cfg); got != "https://public.example.com/base" {
		t.Fatalf("baseURL = %q, want %q", got, "https://public.example.com/base")
	}
}

func TestContract_CanonicalPulseMonitorTokenNamePrefersPulseURL(t *testing.T) {
	got := buildPulseMonitorTokenName("https://public.example.com/base", "127.0.0.1:7655")
	if got != "pulse-public-example-com" {
		t.Fatalf("tokenName = %q, want %q", got, "pulse-public-example-com")
	}
}

func TestContract_FilterRecoveryPointsForRollupsIncludesNormalizedFilters(t *testing.T) {
	verified := true
	unverified := false
	points := []recovery.RecoveryPoint{
		{
			ID:                "point-1",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeSuccess,
			SubjectResourceID: "pod-1",
			Verified:          &verified,
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:    "pod-1",
				SubjectType:     "pod",
				ItemType:        "pod",
				ClusterLabel:    "prod-cluster",
				NodeHostLabel:   "worker-1",
				NamespaceLabel:  "default",
				RepositoryLabel: "repo-a",
				IsWorkload:      true,
			},
		},
		{
			ID:                "point-2",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeFailed,
			SubjectResourceID: "pod-2",
			Verified:          &unverified,
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "pod-2",
				SubjectType:    "pod",
				ItemType:       "pod",
				ClusterLabel:   "other-cluster",
				NodeHostLabel:  "worker-2",
				NamespaceLabel: "kube-system",
				IsWorkload:     true,
			},
		},
	}

	filtered := filterRecoveryPointsForRollups(points, recovery.ListPointsOptions{
		Query:          "repo-a",
		ItemType:       "pod",
		ClusterLabel:   "prod-cluster",
		NodeHostLabel:  "worker-1",
		NamespaceLabel: "default",
		Verification:   "verified",
		WorkloadOnly:   true,
	})

	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if got := filtered[0].SubjectResourceID; got != "pod-1" {
		t.Fatalf("subjectResourceID = %q, want %q", got, "pod-1")
	}
}

func TestContract_ParseRecoveryPlatformQueryPrefersCanonicalPlatformAlias(t *testing.T) {
	t.Parallel()

	if got := parseRecoveryPlatformQuery(url.Values{
		"platform": []string{" truenas "},
		"provider": []string{"proxmox-pve"},
	}); got != recovery.Provider("truenas") {
		t.Fatalf("parseRecoveryPlatformQuery(platform first) = %q, want %q", got, "truenas")
	}

	if got := parseRecoveryPlatformQuery(url.Values{
		"provider": []string{" proxmox-pbs "},
	}); got != recovery.Provider("proxmox-pbs") {
		t.Fatalf("parseRecoveryPlatformQuery(provider fallback) = %q, want %q", got, "proxmox-pbs")
	}
}

func TestContract_RecoveryPointPayloadUsesCanonicalPlatformField(t *testing.T) {
	payload := buildRecoveryPointPayload(recovery.RecoveryPoint{
		ID:       "point-1",
		Provider: recovery.Provider("truenas"),
		Kind:     recovery.Kind("snapshot"),
		Mode:     recovery.Mode("snapshot"),
		Outcome:  recovery.Outcome("success"),
	})

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery point payload: %v", err)
	}

	const want = `{
		"id":"point-1",
		"platform":"truenas",
		"provider":"truenas",
		"kind":"snapshot",
		"mode":"snapshot",
		"outcome":"success"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_RecoveryPointsMockPathReturnsCanonicalProviderBackedFixtures(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		mock.SetEnabled(false)
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			mock.SetEnabled(true)
			mock.SetMockConfig(previousConfig)
		}
	})

	t.Setenv("PULSE_MOCK_NODES", "1")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "0")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "0")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "0")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "0")
	t.Setenv("PULSE_MOCK_K8S_NODES", "0")
	t.Setenv("PULSE_MOCK_K8S_PODS", "0")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "0")

	mock.SetEnabled(false)
	mock.SetEnabled(true)

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/points?platform=truenas&limit=10", nil)
	rec := httptest.NewRecorder()

	NewRecoveryHandlers(nil).HandleListPoints(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp recoveryPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal recovery points response: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected provider-backed mock recovery points in response")
	}
	if resp.Data[0].Platform != recovery.Provider("truenas") {
		t.Fatalf("platform = %q, want %q", resp.Data[0].Platform, "truenas")
	}
	if resp.Data[0].Display == nil {
		t.Fatal("expected normalized recovery display payload on mock points response")
	}
}

func TestContract_RecoveryRollupPayloadUsesCanonicalPlatformsField(t *testing.T) {
	payload := buildRecoveryRollupPayload(recovery.ProtectionRollup{
		RollupID:    "rollup-1",
		LastOutcome: recovery.Outcome("success"),
		Providers: []recovery.Provider{
			recovery.Provider("proxmox-pbs"),
			recovery.Provider("truenas"),
		},
	})

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery rollup payload: %v", err)
	}

	const want = `{
		"rollupId":"rollup-1",
		"lastOutcome":"success",
		"platforms":["proxmox-pbs","truenas"],
		"providers":["proxmox-pbs","truenas"]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_BillingStateJSONSnapshot(t *testing.T) {
	payload := entitlements.BillingState{
		Capabilities:         []string{"relay", "mobile_app"},
		Limits:               map[string]int64{"max_monitored_systems": 10},
		MetersEnabled:        []string{"api_requests"},
		PlanVersion:          "cloud_starter",
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		StripePriceID:        "price_123",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal billing state: %v", err)
	}

	const want = `{
		"capabilities":["relay","mobile_app"],
		"limits":{"max_monitored_systems":10},
		"meters_enabled":["api_requests"],
		"plan_version":"cloud_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_123",
		"stripe_subscription_id":"sub_123",
		"stripe_price_id":"price_123"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedTenantEntitlementsFallbackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("t-tenant"); err != nil {
		t.Fatalf("init tenant persistence: %v", err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities:      []string{pkglicensing.FeatureRelay, pkglicensing.FeatureRBAC},
		Limits:            map[string]int64{"max_monitored_systems": 50},
		PlanVersion:       "msp_starter",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, true)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "t-tenant")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handlers.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode entitlements payload: %v", err)
	}
	if payload.SubscriptionState != string(pkglicensing.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, pkglicensing.SubStateActive)
	}
	if !sliceContainsString(payload.Capabilities, pkglicensing.FeatureRelay) {
		t.Fatalf("expected hosted tenant payload to include %q from default hosted billing state", pkglicensing.FeatureRelay)
	}
	foundMonitoredSystemLimit := false
	for _, limit := range payload.Limits {
		if limit.Key == pkglicensing.MaxMonitoredSystemsLicenseGateKey {
			foundMonitoredSystemLimit = true
			if limit.Limit != 50 {
				t.Fatalf("max_monitored_systems limit=%d, want 50", limit.Limit)
			}
		}
	}
	if !foundMonitoredSystemLimit {
		t.Fatalf("expected max_monitored_systems limit in payload, got %+v", payload.Limits)
	}
}

func TestContract_HostedTenantEntitlementRefreshFallsBackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("t-tenant"); err != nil {
		t.Fatalf("init tenant persistence: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
		}
		var req hostedTrialLeaseRefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if req.OrgID != "default" {
			t.Fatalf("req.OrgID=%q, want %q", req.OrgID, "default")
		}
		if req.InstanceHost != "pulse.example.com" {
			t.Fatalf("req.InstanceHost=%q, want %q", req.InstanceHost, "pulse.example.com")
		}
		if req.EntitlementRefreshToken != "etr_hosted_default" {
			t.Fatalf("req.EntitlementRefreshToken=%q, want %q", req.EntitlementRefreshToken, "etr_hosted_default")
		}

		entitlementJWT, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
			OrgID:             "default",
			InstanceHost:      "pulse.example.com",
			PlanVersion:       "msp_starter",
			SubscriptionState: pkglicensing.SubStateActive,
			Capabilities: []string{
				pkglicensing.FeatureRelay,
				pkglicensing.FeatureAIAutoFix,
			},
		})
		if err != nil {
			t.Fatalf("SignEntitlementLeaseToken: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: entitlementJWT,
		})
	}))
	defer refreshServer.Close()

	store := config.NewFileBillingStore(baseDir)
	expiredLease, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       "msp_starter",
		SubscriptionState: pkglicensing.SubStateActive,
		Capabilities:      []string{pkglicensing.FeatureRelay},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken(expired): %v", err)
	}
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		EntitlementJWT:          expiredLease,
		EntitlementRefreshToken: "etr_hosted_default",
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, true, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	refreshed, permanent, err := handlers.refreshHostedEntitlementLeaseOnce("t-tenant", nil)
	if err != nil {
		t.Fatalf("refreshHostedEntitlementLeaseOnce: %v", err)
	}
	if !refreshed || permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=true permanent=false", refreshed, permanent)
	}

	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if state == nil {
		t.Fatal("expected default billing state after hosted tenant refresh")
	}
	if state.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateActive)
	}
	if state.PlanVersion != "msp_starter" {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, "msp_starter")
	}
	if !sliceContainsString(state.Capabilities, pkglicensing.FeatureAIAutoFix) {
		t.Fatalf("expected default hosted billing state to include %q after tenant refresh, got %v", pkglicensing.FeatureAIAutoFix, state.Capabilities)
	}
}

func TestContract_EntitlementPayloadMonitoredSystemUsageJSONSnapshot(t *testing.T) {
	payload := buildEntitlementPayloadWithUsage(&licenseStatus{
		Valid:               true,
		Tier:                pkglicensing.TierPro,
		Features:            append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
		MaxMonitoredSystems: 15,
	}, string(pkglicensing.SubStateActive), entitlementUsageSnapshot{
		MonitoredSystems: 7,
		LegacyConnections: legacyConnectionCountsModel{
			ProxmoxNodes:       2,
			DockerHosts:        1,
			KubernetesClusters: 1,
		},
	}, nil)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal entitlement payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","ai_patrol","relay","mobile_app","push_notifications","long_term_metrics","ai_alerts","ai_autofix","kubernetes_ai","agent_profiles","advanced_sso","rbac","audit_logging","advanced_reporting"],
		"limits":[{"key":"max_monitored_systems","limit":15,"current":7,"state":"ok"}],
		"subscription_state":"active",
		"upgrade_reasons":[],
		"tier":"pro",
		"hosted_mode":false,
		"valid":true,
		"is_lifetime":false,
		"days_remaining":0,
		"trial_eligible":false,
		"max_history_days":90,
		"legacy_connections":{"proxmox_nodes":2,"docker_hosts":1,"kubernetes_clusters":1},
		"has_migration_gap":false
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedBillingStateFallbackJSONSnapshot(t *testing.T) {
	baseDir := t.TempDir()
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities:         []string{pkglicensing.FeatureRelay, pkglicensing.FeatureRBAC},
		Limits:               map[string]int64{"max_monitored_systems": 50},
		MetersEnabled:        []string{},
		PlanVersion:          "msp_starter",
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     "cus_hosted",
		StripeSubscriptionID: "sub_hosted",
		StripePriceID:        "price_hosted",
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewBillingStateHandlers(store, true)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/orgs/t-tenant/billing-state", nil)
	req.SetPathValue("id", "t-tenant")
	rec := httptest.NewRecorder()

	handlers.HandleGetBillingState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"capabilities":["relay","rbac"],
		"limits":{"max_monitored_systems":50},
		"meters_enabled":[],
		"plan_version":"msp_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_hosted",
		"stripe_subscription_id":"sub_hosted",
		"stripe_price_id":"price_hosted"
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_DemoModeCommercialSurfacePolicy(t *testing.T) {
	t.Run("hidden routes return not found", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("hidden commercial route should not reach downstream handler: %s %s", r.Method, r.URL.Path)
		})
		handler := DemoModeMiddleware(&config.Config{DemoMode: true}, next)

		testCases := []struct {
			method string
			path   string
		}{
			{method: http.MethodGet, path: "/api/license/status"},
			{method: http.MethodGet, path: "/api/license/features"},
			{method: http.MethodGet, path: "/api/license/commercial-posture"},
			{method: http.MethodGet, path: "/api/license/entitlements"},
			{method: http.MethodPost, path: "/api/license/activate"},
			{method: http.MethodPost, path: "/api/license/clear"},
			{method: http.MethodPost, path: "/api/license/trial/start"},
			{method: http.MethodGet, path: "/api/license/monitored-system-ledger"},
			{method: http.MethodGet, path: "/api/admin/orgs/t-tenant/billing-state"},
			{method: http.MethodPut, path: "/api/admin/orgs/t-tenant/billing-state"},
			{method: http.MethodGet, path: "/api/upgrade-metrics/stats"},
			{method: http.MethodPost, path: "/api/upgrade-metrics/events"},
			{method: http.MethodGet, path: licensePurchaseStartPath},
			{method: http.MethodGet, path: "/auth/trial-activate"},
		}

		for _, tc := range testCases {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s status=%d, want %d: %s", tc.method, tc.path, rec.Code, http.StatusNotFound, rec.Body.String())
			}
		}
	})

	t.Run("runtime capabilities stay available but sanitize public demo limit details", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

		handlers := createTestHandler(t)
		handlers.SetConfig(&config.Config{DemoMode: true})
		licenseKey, err := pkglicensing.GenerateLicenseForTesting("contract-demo@example.com", pkglicensing.TierPro, 24*time.Hour)
		if err != nil {
			t.Fatalf("GenerateLicenseForTesting: %v", err)
		}
		if _, err := handlers.Service(context.Background()).Activate(licenseKey); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil)
		rec := httptest.NewRecorder()
		handlers.HandleRuntimeCapabilities(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload RuntimeCapabilitiesPayload
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Capabilities) == 0 {
			t.Fatalf("expected sanitized runtime capabilities to preserve capabilities, got %+v", payload)
		}
		for _, limit := range payload.Limits {
			if limit.Limit != 0 || limit.Current != 0 || limit.State != "ok" {
				t.Fatalf("sanitized limit=%+v, want limit=0 current=0 state=ok", limit)
			}
		}
	})
}

func TestContract_HandoffExchangeJSONSnapshot(t *testing.T) {
	key := []byte("test-handoff-key")
	configDir := t.TempDir()
	secretsDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), key, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	handler := HandleHandoffExchange(configDir)
	tenantID := "tenant-contract"
	t.Setenv("PULSE_TENANT_ID", "")
	token := signHandoffToken(t, key, cloudHandoffClaims{
		AccountID: "acct-contract",
		Email:     "Operator.Owner+Mixed@PulseRelay.Pro",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-contract",
			Subject:   "user-contract",
			Issuer:    cloudHandoffIssuer,
			Audience:  jwt.ClaimStrings{tenantID},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?token="+token+"&format=json", nil)
	req.Host = tenantID + ".cloud.pulserelay.pro"
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", req.Host)
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"account_id":"acct-contract",
		"email":"operator.owner+mixed@pulserelay.pro",
		"exp":"placeholder",
		"jti":"jti-contract",
		"ok":true,
		"role":"owner",
		"tenant_id":"tenant-contract",
		"user_id":"user-contract"
	}`

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode handoff payload: %v", err)
	}
	if _, ok := payload["exp"].(string); !ok {
		t.Fatalf("exp missing or not a string: %+v", payload)
	}
	payload["exp"] = "placeholder"
	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal normalized handoff payload: %v", err)
	}

	assertJSONSnapshot(t, got, want)
}

func TestContract_TenantAIServiceAvoidsSnapshotProviderBridge(t *testing.T) {
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

func TestContract_HostedCloudHandoffEnsuresTenantOrganizationMembership(t *testing.T) {
	key := []byte("test-handoff-key")
	configDir := t.TempDir()
	resetSessionStoreForTests()
	t.Cleanup(resetSessionStoreForTests)
	resetCSRFStoreForTests()
	t.Cleanup(resetCSRFStoreForTests)
	InitSessionStore(configDir)
	InitCSRFStore(configDir)

	secretsDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("mkdir secrets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	tenantID := "tenant-contract-membership"
	mtp := config.NewMultiTenantPersistence(configDir)
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          tenantID,
		DisplayName: "Contract Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	token := signHandoffToken(t, key, cloudHandoffClaims{
		AccountID: "acct-contract-membership",
		Email:     "courtmanr@gmail.com",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-contract-membership",
			Subject:   "user-contract-membership",
			Issuer:    cloudHandoffIssuer,
			Audience:  jwt.ClaimStrings{tenantID},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	form := url.Values{}
	form.Set("token", token)
	t.Setenv("PULSE_HOSTED_MODE", "true")
	t.Setenv("PULSE_TENANT_ID", "")
	t.Setenv("PULSE_PUBLIC_URL", "")
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Host = tenantID + ".cloud.pulserelay.pro"
	req.RemoteAddr = "198.51.100.20:1234"
	rec := httptest.NewRecorder()

	HandleHandoffExchange(configDir).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		t.Fatalf("load organization: %v", err)
	}
	if org.OwnerUserID != "legacy-owner@example.com" {
		t.Fatalf("ownerUserID=%q, want %q", org.OwnerUserID, "legacy-owner@example.com")
	}
	if got := org.GetMemberRole("courtmanr@gmail.com"); got != models.OrgRoleOwner {
		t.Fatalf("member role=%q, want %q", got, models.OrgRoleOwner)
	}
}

func TestContract_HostedDirectCloudHandoffPreservesMembershipClaims(t *testing.T) {
	key := []byte("test-direct-handoff-key")
	configDir := t.TempDir()
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	if err := os.WriteFile(filepath.Join(configDir, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write direct handoff key: %v", err)
	}

	tenantID := "tenant-direct-contract"
	mtp := config.NewMultiTenantPersistence(configDir)
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          tenantID,
		DisplayName: "Direct Contract Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:     "courtmanr@gmail.com",
		TenantID:  tenantID,
		AccountID: "acct-direct-contract",
		UserID:    "user-direct-contract",
		Role:      "owner",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign direct handoff claims: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()

	HandleCloudHandoff(configDir).ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect=%q, want %q", got, "/")
	}

	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		t.Fatalf("load organization: %v", err)
	}
	if org.OwnerUserID != "legacy-owner@example.com" {
		t.Fatalf("ownerUserID=%q, want %q", org.OwnerUserID, "legacy-owner@example.com")
	}
	if got := org.GetMemberRole("courtmanr@gmail.com"); got != models.OrgRoleOwner {
		t.Fatalf("member role=%q, want %q", got, models.OrgRoleOwner)
	}
}

func TestContract_APITokenDeleteRejectsScopeEscalation(t *testing.T) {
	router := &Router{
		config: &config.Config{
			APITokens: []config.APITokenRecord{
				{
					ID:        "broad-token",
					Name:      "broad",
					Hash:      "hash-broad",
					CreatedAt: time.Now().Add(-time.Hour),
					Scopes:    []string{config.ScopeWildcard},
					OrgID:     "default",
				},
			},
		},
	}

	caller, err := config.NewAPITokenRecord(
		"limited-caller-token-123.12345678",
		"limited",
		[]string{config.ScopeSettingsWrite},
	)
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/security/tokens/broad-token", nil)
	req = req.WithContext(authpkg.WithAPIToken(req.Context(), caller))
	rec := httptest.NewRecorder()
	router.handleDeleteAPIToken(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), `Cannot delete token with scope "*"`) {
		t.Fatalf("expected delete scope-escalation contract message, got %q", rec.Body.String())
	}
	if len(router.config.APITokens) != 1 || router.config.APITokens[0].ID != "broad-token" {
		t.Fatalf("expected broader token to remain configured, got %+v", router.config.APITokens)
	}
}

func TestContract_OnboardingQRResponseJSONSnapshot(t *testing.T) {
	payload := onboardingQRResponse{
		Schema:      onboardingSchemaVersion,
		InstanceURL: "https://pulse.example.test",
		InstanceID:  "relay_abc123",
		Relay: onboardingRelayDetails{
			Enabled:             true,
			URL:                 "wss://relay.example.test/ws/app",
			IdentityFingerprint: "AA:BB:CC",
			IdentityPublicKey:   "base64-key",
		},
		AuthToken: "token-123",
		DeepLink:  "pulse://connect?schema=pulse-mobile-onboarding-v1&instance_url=https%3A%2F%2Fpulse.example.test&instance_id=relay_abc123&relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp&auth_token=token-123&identity_fingerprint=AA%3ABB%3ACC&identity_public_key=base64-key",
	}.normalizeCollections()

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal onboarding qr response: %v", err)
	}

	const want = `{
		"schema":"pulse-mobile-onboarding-v1",
		"instance_url":"https://pulse.example.test",
		"instance_id":"relay_abc123",
		"relay":{"enabled":true,"url":"wss://relay.example.test/ws/app","identity_fingerprint":"AA:BB:CC","identity_public_key":"base64-key"},
		"auth_token":"token-123",
		"deep_link":"pulse://connect?schema=pulse-mobile-onboarding-v1\u0026instance_url=https%3A%2F%2Fpulse.example.test\u0026instance_id=relay_abc123\u0026relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp\u0026auth_token=token-123\u0026identity_fingerprint=AA%3ABB%3ACC\u0026identity_public_key=base64-key",
		"diagnostics":[]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedRelayConfigResponseJSONSnapshot(t *testing.T) {
	router, _, instanceHost := newHostedRelayRuntimeTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/relay", nil)
	rec := httptest.NewRecorder()
	router.handleGetRelayConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := router.loadRelayConfigForRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadRelayConfigForRuntime() error = %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, instanceHost) {
		t.Fatalf("relay config response leaked hosted instance secret %q: %s", instanceHost, body)
	}
	if strings.Contains(body, cfg.IdentityPrivateKey) {
		t.Fatalf("relay config response leaked identity private key: %s", body)
	}

	want := fmt.Sprintf(`{
		"enabled":true,
		"server_url":"%s",
		"identity_public_key":"%s",
		"identity_fingerprint":"%s"
	}`, relay.DefaultServerURL, cfg.IdentityPublicKey, cfg.IdentityFingerprint)

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_UpdatePlanManualFallbackJSONSnapshot(t *testing.T) {
	payload := updates.UpdatePlan{
		CanAutoUpdate:   false,
		RequiresRoot:    false,
		RollbackSupport: true,
		EstimatedTime:   "5-10 minutes",
		Instructions: []string{
			"Check out or build Pulse 6.0.0-rc.1 in your development workspace.",
			"Stop the current development instance.",
			"Restart Pulse with the rebuilt binary or release artifact against the existing data directory.",
		},
		Prerequisites: []string{
			"A local development workspace for Pulse",
			"Build tooling for the target version",
			"A backup of the active data directory before replacing the binary",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update plan: %v", err)
	}

	const want = `{
		"canAutoUpdate":false,
		"instructions":["Check out or build Pulse 6.0.0-rc.1 in your development workspace.","Stop the current development instance.","Restart Pulse with the rebuilt binary or release artifact against the existing data directory."],
		"prerequisites":["A local development workspace for Pulse","Build tooling for the target version","A backup of the active data directory before replacing the binary"],
		"estimatedTime":"5-10 minutes",
		"requiresRoot":false,
		"rollbackSupport":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_EmptyUpdatePlanJSONSnapshot(t *testing.T) {
	payload := updates.EmptyUpdatePlan()

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal empty update plan: %v", err)
	}

	const want = `{
		"canAutoUpdate":false,
		"instructions":[],
		"prerequisites":[],
		"requiresRoot":false,
		"rollbackSupport":false
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_APITokenDTOJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastUsed := now.Add(30 * time.Minute)
	expires := now.Add(24 * time.Hour)

	payload := apiTokenDTO{
		ID:          "token-1",
		Name:        "Deploy token",
		Prefix:      "pulse_",
		Suffix:      "1234",
		CreatedAt:   now,
		LastUsedAt:  &lastUsed,
		ExpiresAt:   &expires,
		Scopes:      []string{"monitoring:read", "settings:write"},
		OwnerUserID: "owner@example.com",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal API token dto: %v", err)
	}

	const want = `{
		"id":"token-1",
		"name":"Deploy token",
		"prefix":"pulse_",
		"suffix":"1234",
		"createdAt":"2026-02-08T13:14:15Z",
		"lastUsedAt":"2026-02-08T13:44:15Z",
		"expiresAt":"2026-02-09T13:14:15Z",
		"scopes":["monitoring:read","settings:write"],
		"ownerUserId":"owner@example.com"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_APITokenScopeAliasNormalization(t *testing.T) {
	raw := []string{"host-agent:report", "host-agent:config:read", "host-agent:manage", "host-agent:enroll"}
	got, err := normalizeRequestedScopes(&raw)
	if err != nil {
		t.Fatalf("normalize requested scopes: %v", err)
	}

	want := []string{
		config.ScopeAgentConfigRead,
		config.ScopeAgentEnroll,
		config.ScopeAgentManage,
		config.ScopeAgentReport,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized scopes = %#v, want %#v", got, want)
	}

	for _, legacy := range raw {
		if strings.HasPrefix(legacy, "agent:") {
			t.Fatalf("expected legacy alias input, got canonical scope %q", legacy)
		}
	}
}

func TestContract_HostedSubscriptionRequiredErrorJSONSnapshot(t *testing.T) {
	rec := httptest.NewRecorder()

	writeHostedSubscriptionRequiredError(rec)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPaymentRequired)
	}

	const want = `{
		"error":"subscription_required",
		"message":"Your Cloud subscription is not active. Please check your billing status."
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_InstallScriptReleaseAssetURL(t *testing.T) {
	router := &Router{serverVersion: "v6.0.0-rc.1"}

	got, err := router.installScriptReleaseAssetURL("install.sh")
	if err != nil {
		t.Fatalf("install script release asset URL: %v", err)
	}

	const want = "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/install.sh"
	if got != want {
		t.Fatalf("install script release asset URL = %q, want %q", got, want)
	}
}

func TestContract_InstallScriptReleaseAssetURLUsesConfiguredRepo(t *testing.T) {
	t.Setenv("PULSE_GITHUB_REPO", "example/pulse-fork")

	router := &Router{serverVersion: "v6.0.0-rc.1"}

	got, err := router.installScriptReleaseAssetURL("install.sh")
	if err != nil {
		t.Fatalf("install script release asset URL: %v", err)
	}

	const want = "https://github.com/example/pulse-fork/releases/download/v6.0.0-rc.1/install.sh"
	if got != want {
		t.Fatalf("install script release asset URL = %q, want %q", got, want)
	}
}

func TestContract_InstallScriptReleaseAssetURLRejectsUnreleasedBuild(t *testing.T) {
	router := &Router{serverVersion: "dev"}

	if _, err := router.installScriptReleaseAssetURL("install.sh"); err == nil {
		t.Fatalf("expected development build to reject release asset lookup")
	}
}

func TestContract_InstallScriptReleaseAssetURLRejectsDevPrereleaseBuild(t *testing.T) {
	router := &Router{serverVersion: "v6.0.0-dev"}

	if _, err := router.installScriptReleaseAssetURL("install.sh"); err == nil {
		t.Fatalf("expected dev prerelease build to reject release asset lookup")
	}
}

func TestContract_ProxmoxInstallCommandIncludesInsecureForPlainHTTP(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "http://pulse.example.com:7655/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, "--url "+posixShellQuote("http://pulse.example.com:7655")) {
		t.Fatalf("install command missing canonical base URL: %s", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Fatalf("install command missing insecure flag for plain HTTP Pulse URL: %s", got)
	}
}

func TestContract_ProxmoxInstallCommandUsesPrivilegeEscalationWrapper(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`) {
		t.Fatalf("install command missing root-or-sudo wrapper: %s", got)
	}
	if !strings.Contains(got, `sudo bash -s --`) {
		t.Fatalf("install command missing sudo fallback: %s", got)
	}
	if strings.Contains(got, "| bash -s -- --url") {
		t.Fatalf("install command preserved raw bash pipe instead of governed wrapper: %s", got)
	}
}

func TestContract_OptionalAuthProxmoxInstallCommandOmitsToken(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if strings.Contains(got, "--token") {
		t.Fatalf("optional-auth install command preserved token flag: %s", got)
	}
	if !strings.Contains(got, "--url "+posixShellQuote("https://pulse.example.com")) {
		t.Fatalf("optional-auth install command missing canonical base URL: %s", got)
	}
}

func TestContract_ProxmoxInstallCommandNormalizesTrailingSlashBaseURL(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/base///",
		Token:              "token-123",
		InstallType:        "pbs",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, posixShellQuote("https://pulse.example.com/base/install.sh")) {
		t.Fatalf("install command missing normalized install script URL: %s", got)
	}
	if !strings.Contains(got, "--url "+posixShellQuote("https://pulse.example.com/base")) {
		t.Fatalf("install command missing normalized base URL: %s", got)
	}
	if strings.Contains(got, "//install.sh") {
		t.Fatalf("install command preserved double-slash install path: %s", got)
	}
}

func TestContract_SystemSettingsResponseJSONSnapshot(t *testing.T) {
	payload := EmptySystemSettingsResponse()
	payload.SystemSettings = config.SystemSettings{
		PVEPollingInterval:           30,
		PBSPollingInterval:           60,
		PMGPollingInterval:           60,
		BackupPollingInterval:        3600,
		UpdateChannel:                "rc",
		AutoUpdateEnabled:            false,
		AutoUpdateCheckInterval:      24,
		AutoUpdateTime:               "03:00",
		DiscoveryEnabled:             true,
		DiscoverySubnet:              "10.0.0.0/24",
		DiscoveryConfig:              config.DefaultDiscoveryConfig(),
		Theme:                        "dark",
		TemperatureMonitoringEnabled: true,
		DisableDockerUpdateActions:   true,
	}
	payload.EnvOverrides = map[string]bool{
		"PULSE_TELEMETRY": true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal system settings response: %v", err)
	}

	const want = `{
		"pvePollingInterval":30,
		"pbsPollingInterval":60,
		"pmgPollingInterval":60,
		"backupPollingInterval":3600,
		"updateChannel":"rc",
		"autoUpdateEnabled":false,
		"autoUpdateCheckInterval":24,
		"autoUpdateTime":"03:00",
		"discoveryEnabled":true,
		"discoverySubnet":"10.0.0.0/24",
		"discoveryConfig":{
			"environment_override":"auto",
			"subnet_blocklist":["169.254.0.0/16"],
			"max_hosts_per_scan":1024,
			"max_concurrent":50,
			"enable_reverse_dns":true,
			"scan_gateways":true,
			"dial_timeout_ms":1000,
			"http_timeout_ms":2000
		},
		"theme":"dark",
		"fullWidthMode":false,
		"allowEmbedding":false,
		"temperatureMonitoringEnabled":true,
		"hideLocalLogin":false,
		"disableDockerUpdateActions":true,
		"envOverrides":{"PULSE_TELEMETRY":true}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CachedDiscoveryResponseJSONSnapshot(t *testing.T) {
	response := map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"ip":   "10.0.0.1",
				"port": 8006,
				"type": "pve",
			},
		},
		"errors": []string{
			"Docker bridge network [10.0.0.2:8007]: request timed out",
		},
		"structured_errors": []map[string]interface{}{
			{
				"ip":         "10.0.0.2",
				"port":       8007,
				"phase":      "docker_bridge_network",
				"error_type": "timeout",
				"message":    "request timed out",
				"timestamp":  "2023-11-14T22:13:20Z",
			},
		},
		"environment": nil,
		"cached":      true,
		"updated":     int64(1700000010),
		"age":         float64(0),
	}

	got, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal cached discovery response: %v", err)
	}

	const want = `{
		"age":0,
		"cached":true,
		"environment":null,
		"errors":["Docker bridge network [10.0.0.2:8007]: request timed out"],
		"servers":[{"ip":"10.0.0.1","port":8006,"type":"pve"}],
		"structured_errors":[{"error_type":"timeout","ip":"10.0.0.2","message":"request timed out","phase":"docker_bridge_network","port":8007,"timestamp":"2023-11-14T22:13:20Z"}],
		"updated":1700000010
	}`

	var wantValue interface{}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal wanted discovery response: %v", err)
	}

	var gotValue interface{}
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got discovery response: %v", err)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("cached discovery response mismatch\nwant: %s\ngot:  %s", want, string(got))
	}
}

func TestContract_AutoRegisterRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "agent",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"agent"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterScriptRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal script auto-register request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"script"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterCheckRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve.local:8006",
		CandidateHosts:    []string{"https://pve.local:8006", "https://10.0.0.5:8006"},
		ServerName:        "pve-node-1",
		AuthToken:         "setup-token-123",
		Source:            "agent",
		CheckRegistration: true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register check request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"candidateHosts":["https://pve.local:8006","https://10.0.0.5:8006"],
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"agent",
		"checkRegistration":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterScriptRequestRequiresExplicitSourceMarker(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal script auto-register request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode script auto-register request: %v", err)
	}
	if decoded["source"] != "script" {
		t.Fatalf("source = %#v, want explicit script marker", decoded["source"])
	}
}

func TestContract_CanonicalAutoRegisterRequiresExplicitServerName(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal serverName-free auto-register request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode serverName-free auto-register request: %v", err)
	}
	if _, ok := decoded["serverName"]; ok {
		t.Fatalf("serverName = %#v, want omitted when caller does not send it", decoded["serverName"])
	}
}

func TestContract_CanonicalAutoRegisterSetupTokenAuthFailureText(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	requestBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		Source:     "script",
	}

	missingAuthJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal missing-auth auto-register request: %v", err)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(missingAuthJSON))
	missingRec := httptest.NewRecorder()
	handler.HandleAutoRegister(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing-auth status = %d, want 401", missingRec.Code)
	}
	if got := missingRec.Body.String(); got != "Pulse setup token required\n" {
		t.Fatalf("missing-auth body = %q, want canonical missing-setup-token guidance", got)
	}

	requestBody.AuthToken = "invalid-setup-token"
	invalidAuthJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal invalid-auth auto-register request: %v", err)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(invalidAuthJSON))
	invalidRec := httptest.NewRecorder()
	handler.HandleAutoRegister(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid-auth status = %d, want 401", invalidRec.Code)
	}
	if got := invalidRec.Body.String(); got != "Invalid or expired setup token\n" {
		t.Fatalf("invalid-auth body = %q, want canonical setup-token auth failure text", got)
	}

	const validSetupToken = "setup-token-123"
	tokenHash := authpkg.HashAPIToken(validSetupToken)
	handler.codeMutex.Lock()
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	requestBody.AuthToken = validSetupToken
	requestBody.TokenValue = ""
	mismatchedJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal mismatched-completion auto-register request: %v", err)
	}

	mismatchedReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(mismatchedJSON))
	mismatchedRec := httptest.NewRecorder()
	handler.HandleAutoRegister(mismatchedRec, mismatchedReq)
	if mismatchedRec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched-completion status = %d, want 400", mismatchedRec.Code)
	}
	if got := mismatchedRec.Body.String(); got != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("mismatched-completion body = %q, want canonical token-pair guidance", got)
	}
}

func TestContract_BootstrapTokenPersistenceJSONSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	token, created, path, err := loadOrCreateBootstrapToken(tempDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}
	if !created {
		t.Fatal("expected bootstrap token to be created")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted bootstrap token: %v", err)
	}
	snapshot := string(data)
	if !strings.Contains(snapshot, `"version":2`) {
		t.Fatalf("bootstrap token snapshot missing version: %s", snapshot)
	}
	if !strings.Contains(snapshot, `"token_ciphertext":"`) {
		t.Fatalf("bootstrap token snapshot missing ciphertext field: %s", snapshot)
	}
	if !strings.Contains(snapshot, `"token_hash":"`) {
		t.Fatalf("bootstrap token snapshot missing token hash field: %s", snapshot)
	}
	if strings.Contains(snapshot, token) {
		t.Fatalf("bootstrap token snapshot leaked raw token: %s", snapshot)
	}
}

func TestContract_QuickSecuritySetupBootstrapRetrievalGuidance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	router.initializeBootstrapToken()

	handler := handleQuickSecuritySetupFixed(router)
	body := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.40:54321"
	rec := httptest.NewRecorder()

	authLimiter.Reset("198.51.100.40")
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("quick setup status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "pulse bootstrap-token") {
		t.Fatalf("quick setup guidance = %q, want pulse bootstrap-token retrieval guidance", got)
	}
	if got := rec.Body.String(); strings.Contains(got, ".bootstrap_token") {
		t.Fatalf("quick setup guidance = %q, want no raw .bootstrap_token scraping guidance", got)
	}
}

func TestContract_ResetFirstRunSecurityResponseJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")

	record := newTokenRecord(t, "contract-reset-first-run-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"

	envPath, err := writeAuthEnvFile(cfg.ConfigPath, cfg.DataPath, []byte("PULSE_AUTH_USER='admin'\n"))
	if err != nil {
		t.Fatalf("writeAuthEnvFile: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	req := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	req.Header.Set("X-API-Token", "contract-reset-first-run-token-123.12345678")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reset-first-run status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var payload firstRunResetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode reset-first-run response: %v", err)
	}
	if strings.TrimSpace(payload.BootstrapToken) == "" {
		t.Fatal("reset-first-run response missing bootstrapToken")
	}
	if got, want := payload.BootstrapTokenPath, filepath.Join(cfg.DataPath, bootstrapTokenFilename); got != want {
		t.Fatalf("reset-first-run bootstrapTokenPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatalf("reset-first-run should remove auth env file, stat err = %v", err)
	}

	got, err := json.Marshal(firstRunResetResponse{
		BootstrapToken:     "bootstrap-token-placeholder",
		BootstrapTokenPath: "bootstrap-token-path-placeholder",
	})
	if err != nil {
		t.Fatalf("marshal reset-first-run response snapshot: %v", err)
	}

	const want = `{
		"bootstrapToken":"bootstrap-token-placeholder",
		"bootstrapTokenPath":"bootstrap-token-path-placeholder"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResetFirstRunSecurityClearsEnvBackedStatus(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	t.Setenv("PULSE_AUTH_USER", "admin")
	t.Setenv("PULSE_AUTH_PASS", "hashed-password")

	record := newTokenRecord(t, "contract-reset-first-run-token-456.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	resetReq := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	resetReq.Header.Set("X-API-Token", "contract-reset-first-run-token-456.12345678")
	resetRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset-first-run status = %d, want 200 (%s)", resetRec.Code, resetRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	statusRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200 (%s)", statusRec.Code, statusRec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode security status payload: %v", err)
	}
	if got, _ := payload["hasAuthentication"].(bool); got {
		t.Fatalf("hasAuthentication = %v, want false", payload["hasAuthentication"])
	}
	if got, _ := payload["bootstrapTokenPath"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("bootstrapTokenPath = %v, want non-empty", payload["bootstrapTokenPath"])
	}
}

func TestContract_SetupScriptURLResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":              "pve",
		"host":              "https://pve.local:8006",
		"url":               "https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve",
		"downloadURL":       "https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&setup_token=setup-token-123&type=pve",
		"scriptFileName":    "pulse-setup-pve.sh",
		"command":           "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"commandWithEnv":    "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"commandWithoutEnv": "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then bash; elif command -v sudo >/dev/null 2>&1; then sudo bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"expires":           int64(1900000000),
		"setupToken":        "setup-token-123",
		"tokenHint":         "set…123",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal setup-script-url response: %v", err)
	}

	const want = `{
		"command":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"commandWithEnv":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"commandWithoutEnv":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"downloadURL":"https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026setup_token=setup-token-123\u0026type=pve",
		"expires":1900000000,
		"host":"https://pve.local:8006",
		"scriptFileName":"pulse-setup-pve.sh",
		"setupToken":"setup-token-123",
		"tokenHint":"set…123",
		"type":"pve",
		"url":"https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SecurityStatusIncludesSessionCapabilitiesDemoMode(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.DemoMode = true

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode security status payload: %v", err)
	}

	sessionCapabilities, ok := payload["sessionCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("sessionCapabilities = %#v, want object", payload["sessionCapabilities"])
	}
	if got, _ := sessionCapabilities["demoMode"].(bool); !got {
		t.Fatalf("sessionCapabilities.demoMode = %v, want true", sessionCapabilities["demoMode"])
	}
}

func TestContract_SetupScriptURLRejectsNonCanonicalRequestJSON(t *testing.T) {
	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/setup-script-url",
		bytes.NewBufferString(`{"type":"pve","host":"pve.local","setupToken":"unexpected"}`),
	)
	rec := httptest.NewRecorder()

	handler.HandleSetupScriptURL(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Body.String(); got != "Invalid request\n" {
		t.Fatalf("body = %q, want canonical invalid-request guidance", got)
	}
}

func TestContract_SetupBootstrapRejectsPBSBackupPerms(t *testing.T) {
	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})

	setupScriptReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs.local:8007&pulse_url=https://pulse.example&backup_perms=true",
		nil,
	)
	setupScriptRec := httptest.NewRecorder()
	handler.HandleSetupScript(setupScriptRec, setupScriptReq)
	if setupScriptRec.Code != http.StatusBadRequest {
		t.Fatalf("setup-script status = %d, want 400", setupScriptRec.Code)
	}
	if got := setupScriptRec.Body.String(); got != "backup_perms is only supported for type 'pve'\n" {
		t.Fatalf("setup-script body = %q, want canonical backup-perms guidance", got)
	}

	setupURLReq := httptest.NewRequest(
		http.MethodPost,
		"/api/setup-script-url",
		bytes.NewBufferString(`{"type":"pbs","host":"pbs.local","backupPerms":true}`),
	)
	setupURLRec := httptest.NewRecorder()
	handler.HandleSetupScriptURL(setupURLRec, setupURLReq)
	if setupURLRec.Code != http.StatusBadRequest {
		t.Fatalf("setup-script-url status = %d, want 400", setupURLRec.Code)
	}
	if got := setupURLRec.Body.String(); got != "backupPerms is only supported for type 'pve'\n" {
		t.Fatalf("setup-script-url body = %q, want canonical backup-perms guidance", got)
	}
}

func TestContract_CanonicalAutoRegisterSourceContract(t *testing.T) {
	if !isCanonicalAutoRegisterSource("agent") {
		t.Fatalf("agent source should be accepted")
	}
	if !isCanonicalAutoRegisterSource("script") {
		t.Fatalf("script source should be accepted")
	}
	if isCanonicalAutoRegisterSource("manual") {
		t.Fatalf("manual source should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterTypeContract(t *testing.T) {
	if !isCanonicalAutoRegisterType("pve") {
		t.Fatalf("pve type should be accepted")
	}
	if !isCanonicalAutoRegisterType("pbs") {
		t.Fatalf("pbs type should be accepted")
	}
	if isCanonicalAutoRegisterType("pmg") {
		t.Fatalf("pmg type should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterTokenIDContract(t *testing.T) {
	if !isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!pulse-homelab") {
		t.Fatalf("canonical pve token id should be accepted")
	}
	if !isCanonicalAutoRegisterTokenID("pbs", "pulse-monitor@pbs!pulse-backup") {
		t.Fatalf("canonical pbs token id should be accepted")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!token") {
		t.Fatalf("non-pulse-managed pve token suffix should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse@pve!token") {
		t.Fatalf("non-canonical token id should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pbs!pulse-backup") {
		t.Fatalf("cross-type token id should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!") {
		t.Fatalf("empty canonical token suffix should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!pulse-") {
		t.Fatalf("empty pulse-managed token slug should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterMatchMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterMatchMessage("resolved host identity"); got != "Canonical auto-register matched existing node by resolved host identity" {
		t.Fatalf("resolved-host message = %q", got)
	}
	if got := canonicalAutoRegisterMatchMessage("DHCP continuity token identity"); got != "Canonical auto-register matched existing node by DHCP continuity token identity" {
		t.Fatalf("dhcp message = %q", got)
	}
	if got := canonicalAutoRegisterMatchMessage("host; updated token in-place"); got != "Canonical auto-register matched existing node by host; updated token in-place" {
		t.Fatalf("host-update message = %q", got)
	}
	if strings.Contains(canonicalAutoRegisterMatchMessage("resolved host identity"), "Secure auto-register") {
		t.Fatalf("canonical match message must not preserve secure auto-register wording")
	}
}

func TestContract_CanonicalAutoRegisterCompletionPayloadMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterCompletionPayloadMessage(); got != "Incomplete canonical auto-register token completion payload" {
		t.Fatalf("completion-payload message = %q", got)
	}
	if strings.Contains(canonicalAutoRegisterCompletionPayloadMessage(), "secure token completion") {
		t.Fatalf("canonical completion-payload message must not preserve secure wording")
	}
}

func TestContract_CanonicalAutoRegisterMissingFieldsMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterMissingFieldsMessage("", "", false, ""); got != "Missing required canonical auto-register fields: type, host, tokenId/tokenValue, serverName" {
		t.Fatalf("all-missing message = %q", got)
	}
	if got := canonicalAutoRegisterMissingFieldsMessage("pve", "https://pve.local:8006", true, ""); got != "Missing required canonical auto-register fields: serverName" {
		t.Fatalf("serverName-only message = %q", got)
	}
}

func TestContract_CanonicalAutoRegisterDirectValidationContract(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
	}

	missingServerJSON, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal missing-serverName canonical request: %v", err)
	}

	missingServerReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(missingServerJSON))
	missingServerRec := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(missingServerRec, missingServerReq, &reqBody, "127.0.0.1")
	if missingServerRec.Code != http.StatusBadRequest {
		t.Fatalf("missing-serverName status = %d, want 400", missingServerRec.Code)
	}
	if got := missingServerRec.Body.String(); got != "Missing required canonical auto-register fields: serverName\n" {
		t.Fatalf("missing-serverName body = %q, want canonical missing-field guidance", got)
	}

	reqBody.ServerName = "pve-node-1"
	reqBody.TokenValue = ""
	mismatchedReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	mismatchedRec := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(mismatchedRec, mismatchedReq, &reqBody, "127.0.0.1")
	if mismatchedRec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched-completion status = %d, want 400", mismatchedRec.Code)
	}
	if got := mismatchedRec.Body.String(); got != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("mismatched-completion body = %q, want canonical token-pair guidance", got)
	}
}

func TestContract_AutoRegisterResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "script",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-homelab",
		"tokenValue": "secret-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"script",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterWebSocketEventJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":      "pve",
		"host":      "https://pve.local:8006",
		"name":      "pve-node-1",
		"nodeId":    "pve-node-1",
		"nodeName":  "pve-node-1",
		"tokenId":   "pulse-monitor@pve!pulse-homelab",
		"hasToken":  true,
		"verifySSL": true,
		"status":    "connected",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register websocket event: %v", err)
	}

	const want = `{
		"hasToken":true,
		"host":"https://pve.local:8006",
		"name":"pve-node-1",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"status":"connected",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"type":"pve",
		"verifySSL":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterEventJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":      "pbs",
		"host":      "https://pbs.local:8007",
		"name":      "backup-node (2)",
		"nodeId":    "backup-node (2)",
		"nodeName":  "backup-node (2)",
		"tokenId":   "pulse-monitor@pbs!pulse-backup",
		"hasToken":  true,
		"verifySSL": true,
		"status":    "connected",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register websocket event: %v", err)
	}

	const want = `{
		"hasToken":true,
		"host":"https://pbs.local:8007",
		"name":"backup-node (2)",
		"nodeId":"backup-node (2)",
		"nodeName":"backup-node (2)",
		"status":"connected",
		"tokenId":"pulse-monitor@pbs!pulse-backup",
		"type":"pbs",
		"verifySSL":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterReusedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "script",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "existing-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"script",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"existing-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterCallerProvidedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-server",
		"tokenValue": "created-locally",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register caller-provided token response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-server",
		"tokenValue":"created-locally",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterRotatedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "rotated-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register rotated token response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"rotated-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterResponseUsesCanonicalStoredNodeIdentity(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 (2) registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1 (2)",
		"nodeName":   "pve-node-1 (2)",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "existing-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register disambiguated response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 (2) registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1 (2)",
		"nodeName":"pve-node-1 (2)",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"existing-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MetricsHistoryLiveFallbackJSONSnapshot(t *testing.T) {
	state := models.NewState()
	state.UpdateVMsForInstance("pve1", []models.VM{{
		ID:       "pve1:node1:101",
		VMID:     101,
		Name:     "vm-101",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.42,
		Memory: models.Memory{
			Usage: 55.0,
		},
		Disk: models.Disk{
			Usage: 33.0,
		},
	}})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:101&metric=cpu&start=2026-03-11T00:00:00Z&end=2026-03-12T00:00:00Z",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal metrics history response: %v", err)
	}

	points, ok := payload["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("unexpected points payload: %#v", payload["points"])
	}
	point, ok := points[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected point payload: %#v", points[0])
	}
	point["timestamp"] = float64(1700000000000)
	payload["range"] = "24h"
	payload["start"] = float64(1741651200000)
	payload["end"] = float64(1741737600000)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal normalized metrics history response: %v", err)
	}

	const want = `{
		"end":1741737600000,
		"metric":"cpu",
		"points":[
			{
				"max":42,
				"min":42,
				"timestamp":1700000000000,
				"value":42
			}
		],
		"range":"24h",
		"resourceId":"pve1:node1:101",
		"resourceType":"vm",
		"source":"live",
		"start":1741651200000
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MetricsHistoryPhysicalDiskIOLiveWindowUsesCanonicalDiskTarget(t *testing.T) {
	mh := monitoring.NewMetricsHistory(1000, time.Hour)
	now := time.Now().UTC().Truncate(time.Second)
	resourceID := "SERIAL884006359727"
	for i, value := range []float64{1.5, 2.25, 3.0} {
		mh.AddDiskMetric(resourceID, "diskread", value*1024*1024, now.Add(time.Duration(i-2)*10*time.Minute))
	}

	router := &Router{
		monitor: &monitoring.Monitor{},
	}
	setUnexportedField(t, router.monitor, "metricsHistory", mh)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=disk&resourceId="+resourceID+"&metric=diskread&range=30m",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ResourceType != "disk" {
		t.Fatalf("expected resourceType disk, got %q", resp.ResourceType)
	}
	if resp.ResourceId != resourceID {
		t.Fatalf("expected canonical disk resource id %q, got %q", resourceID, resp.ResourceId)
	}
	if resp.Metric != "diskread" {
		t.Fatalf("expected metric diskread, got %q", resp.Metric)
	}
	if resp.Range != "30m" {
		t.Fatalf("expected range 30m, got %q", resp.Range)
	}
	if resp.Source != "memory" {
		t.Fatalf("expected source memory, got %q", resp.Source)
	}
	if len(resp.Points) != 3 {
		t.Fatalf("expected 3 diskread points, got %d", len(resp.Points))
	}
}

func TestContract_PatrolStatusResponseJSONSnapshot(t *testing.T) {
	lastPatrolAt := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)
	lastActivityAt := lastPatrolAt.Add(5 * time.Minute)
	nextPatrolAt := lastPatrolAt.Add(6 * time.Hour)
	blockedAt := lastPatrolAt.Add(15 * time.Minute)

	payload := PatrolStatusResponse{
		RuntimeState:   ai.PatrolRuntimeStateBlocked,
		Running:        false,
		Enabled:        true,
		LastPatrolAt:   &lastPatrolAt,
		LastActivityAt: &lastActivityAt,
		TriggerStatus: &ai.TriggerStatus{
			Running:                true,
			PendingTriggers:        3,
			CurrentInterval:        300000,
			RecentEvents:           6,
			IsBusyMode:             true,
			AlertTriggersEnabled:   true,
			AnomalyTriggersEnabled: false,
		},
		NextPatrolAt:               &nextPatrolAt,
		LastDurationMs:             12345,
		ResourcesChecked:           18,
		FindingsCount:              3,
		ErrorCount:                 1,
		Healthy:                    false,
		IntervalMs:                 21600000,
		FixedCount:                 2,
		BlockedReason:              "Awaiting AI provider configuration",
		BlockedAt:                  &blockedAt,
		QuickstartCreditsRemaining: 7,
		QuickstartCreditsTotal:     pkglicensing.QuickstartCreditsTotal,
		UsingQuickstart:            true,
		LicenseRequired:            true,
		LicenseStatus:              "none",
		UpgradeURL:                 "https://pulserelay.pro/upgrade?feature=ai_autofix",
	}
	payload.Summary.Critical = 1
	payload.Summary.Warning = 2
	payload.Summary.Watch = 0
	payload.Summary.Info = 4

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal patrol status response: %v", err)
	}

	const want = `{
		"runtime_state":"blocked",
		"running":false,
		"enabled":true,
		"last_patrol_at":"2026-03-12T09:30:00Z",
		"last_activity_at":"2026-03-12T09:35:00Z",
		"trigger_status":{"running":true,"pending_triggers":3,"current_interval_ms":300000,"recent_events":6,"is_busy_mode":true,"alert_triggers_enabled":true,"anomaly_triggers_enabled":false},
		"next_patrol_at":"2026-03-12T15:30:00Z",
		"last_duration_ms":12345,
		"resources_checked":18,
		"findings_count":3,
		"error_count":1,
		"healthy":false,
		"interval_ms":21600000,
		"fixed_count":2,
		"blocked_reason":"Awaiting AI provider configuration",
		"blocked_at":"2026-03-12T09:45:00Z",
		"quickstart_credits_remaining":7,
		"quickstart_credits_total":25,
		"using_quickstart":true,
		"license_required":true,
		"license_status":"none",
		"upgrade_url":"https://pulserelay.pro/upgrade?feature=ai_autofix",
		"summary":{"critical":1,"warning":2,"watch":0,"info":4}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_PatrolRunRecordJSONSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(90 * time.Second)

	payload := ai.PatrolRunRecord{
		ID:                        "run-1",
		StartedAt:                 startedAt,
		CompletedAt:               completedAt,
		DurationMs:                90000,
		Type:                      "scoped",
		TriggerReason:             "alert_fired",
		ScopeResourceIDs:          []string{"seed-resource"},
		EffectiveScopeResourceIDs: []string{},
		ScopeResourceTypes:        []string{"vm"},
		ResourcesChecked:          4,
		NodesChecked:              0,
		GuestsChecked:             2,
		DockerChecked:             0,
		StorageChecked:            0,
		HostsChecked:              0,
		TrueNASChecked:            1,
		PBSChecked:                0,
		PMGChecked:                1,
		KubernetesChecked:         1,
		NewFindings:               0,
		ExistingFindings:          2,
		RejectedFindings:          1,
		ResolvedFindings:          1,
		AutoFixCount:              0,
		FindingsSummary:           "All clear",
		FindingIDs:                []string{},
		ErrorCount:                0,
		Status:                    "healthy",
		TriageFlags:               3,
		TriageSkippedLLM:          true,
		ToolCallCount:             0,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal patrol run record: %v", err)
	}

	const want = `{
		"id":"run-1",
		"started_at":"2026-03-12T10:00:00Z",
		"completed_at":"2026-03-12T10:01:30Z",
		"duration_ms":90000,
		"type":"scoped",
		"trigger_reason":"alert_fired",
		"scope_resource_ids":["seed-resource"],
		"effective_scope_resource_ids":[],
		"scope_resource_types":["vm"],
		"resources_checked":4,
		"nodes_checked":0,
		"guests_checked":2,
		"docker_checked":0,
		"storage_checked":0,
		"hosts_checked":0,
		"truenas_checked":1,
		"pbs_checked":0,
		"pmg_checked":1,
		"kubernetes_checked":1,
		"new_findings":0,
		"existing_findings":2,
		"rejected_findings":1,
		"resolved_findings":1,
		"findings_summary":"All clear",
		"finding_ids":[],
		"error_count":0,
		"status":"healthy",
		"triage_flags":3,
		"triage_skipped_llm":true,
		"tool_call_count":0
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ChatStreamEventJSONSnapshots(t *testing.T) {
	cases := []struct {
		name  string
		event chat.StreamEvent
		want  string
	}{
		{
			name: "content",
			event: mustStreamEvent(t, "content", chat.ContentData{
				Text: "hello",
			}),
			want: `{"type":"content","data":{"text":"hello"}}`,
		},
		{
			name: "explore_status",
			event: mustStreamEvent(t, "explore_status", chat.ExploreStatusData{
				Phase:   "started",
				Message: "Explore pre-pass running (read-only context).",
				Model:   "openai:explore-fast",
			}),
			want: `{"type":"explore_status","data":{"phase":"started","message":"Explore pre-pass running (read-only context).","model":"openai:explore-fast"}}`,
		},
		{
			name: "tool_start",
			event: mustStreamEvent(t, "tool_start", chat.ToolStartData{
				ID:       "tool-1",
				Name:     "pulse_read",
				Input:    `{"path":"/tmp/x.log"}`,
				RawInput: `{"path":"/tmp/x.log"}`,
			}),
			want: `{"type":"tool_start","data":{"id":"tool-1","name":"pulse_read","input":"{\"path\":\"/tmp/x.log\"}","raw_input":"{\"path\":\"/tmp/x.log\"}"}}`,
		},
		{
			name: "tool_end",
			event: mustStreamEvent(t, "tool_end", chat.ToolEndData{
				ID:       "tool-1",
				Name:     "pulse_read",
				Input:    `{"path":"/tmp/x.log"}`,
				RawInput: `{"path":"/tmp/x.log"}`,
				Output:   "ok",
				Success:  true,
			}),
			want: `{"type":"tool_end","data":{"id":"tool-1","name":"pulse_read","input":"{\"path\":\"/tmp/x.log\"}","raw_input":"{\"path\":\"/tmp/x.log\"}","output":"ok","success":true}}`,
		},
		{
			name: "approval_needed",
			event: mustStreamEvent(t, "approval_needed", chat.ApprovalNeededData{
				ApprovalID:  "approval-1",
				ToolID:      "tool-2",
				ToolName:    "pulse_exec",
				Command:     "systemctl restart nginx",
				RunOnHost:   true,
				TargetHost:  "node-1",
				Risk:        "high",
				Description: "Restart web service",
			}),
			want: `{"type":"approval_needed","data":{"approval_id":"approval-1","tool_id":"tool-2","tool_name":"pulse_exec","command":"systemctl restart nginx","run_on_host":true,"target_host":"node-1","risk":"high","description":"Restart web service"}}`,
		},
		{
			name: "question",
			event: mustStreamEvent(t, "question", chat.QuestionData{
				SessionID:  "session-1",
				QuestionID: "question-1",
				Questions: []chat.Question{
					{
						ID:       "target",
						Type:     "select",
						Question: "Which node should I inspect?",
						Header:   "Target",
						Options: []chat.QuestionOption{
							{Label: "Node A", Value: "node-a", Description: "Primary compute node"},
							{Label: "Node B", Value: "node-b", Description: "Replica node"},
						},
					},
				},
			}),
			want: `{"type":"question","data":{"session_id":"session-1","question_id":"question-1","questions":[{"id":"target","type":"select","question":"Which node should I inspect?","header":"Target","options":[{"label":"Node A","value":"node-a","description":"Primary compute node"},{"label":"Node B","value":"node-b","description":"Replica node"}]}]}}`,
		},
		{
			name: "done",
			event: mustStreamEvent(t, "done", chat.DoneData{
				SessionID:    "session-1",
				InputTokens:  120,
				OutputTokens: 80,
			}),
			want: `{"type":"done","data":{"session_id":"session-1","input_tokens":120,"output_tokens":80}}`,
		},
		{
			name: "error",
			event: mustStreamEvent(t, "error", chat.ErrorData{
				Message: "request failed",
			}),
			want: `{"type":"error","data":{"message":"request failed"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("marshal stream event: %v", err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_PushNotificationJSONSnapshots(t *testing.T) {
	cases := []struct {
		name    string
		payload relay.PushNotificationPayload
		want    string
	}{
		{
			name:    "patrol_finding",
			payload: relay.NewPatrolFindingNotification("finding-1", "warning", "capacity", "Disk pressure detected"),
			want:    `{"type":"patrol_finding","priority":"normal","title":"Disk pressure detected","body":"New warning capacity finding detected","action_type":"view_finding","action_id":"finding-1","category":"capacity","severity":"warning"}`,
		},
		{
			name:    "patrol_critical",
			payload: relay.NewPatrolFindingNotification("finding-2", "critical", "performance", "CPU saturation detected"),
			want:    `{"type":"patrol_critical","priority":"high","title":"CPU saturation detected","body":"New critical performance finding detected","action_type":"view_finding","action_id":"finding-2","category":"performance","severity":"critical"}`,
		},
		{
			name:    "approval_request",
			payload: relay.NewApprovalRequestNotification("approval-1", "Fix queued", "high"),
			want:    `{"type":"approval_request","priority":"high","title":"Fix queued","body":"A high-risk fix requires your approval","action_type":"approve_fix","action_id":"approval-1"}`,
		},
		{
			name:    "fix_completed_success",
			payload: relay.NewFixCompletedNotification("finding-3", "CPU saturation detected", true),
			want:    `{"type":"fix_completed","priority":"normal","title":"CPU saturation detected","body":"Fix applied successfully","action_type":"view_fix_result","action_id":"finding-3"}`,
		},
		{
			name:    "fix_completed_failed",
			payload: relay.NewFixCompletedNotification("finding-4", "Disk pressure detected", false),
			want:    `{"type":"fix_completed","priority":"normal","title":"Disk pressure detected","body":"Fix attempt failed — review needed","action_type":"view_fix_result","action_id":"finding-4"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal push payload: %v", err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_AlertJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := start.Add(3 * time.Minute)

	payload := alerts.Alert{
		ID:           "cluster/qemu/100-cpu",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "cluster/qemu/100",
		ResourceName: "test-vm",
		Node:         "pve-1",
		Instance:     "cpu0",
		Message:      "VM cpu at 95%",
		Value:        95.0,
		Threshold:    90.0,
		StartTime:    start,
		LastSeen:     lastSeen,
		Acknowledged: false,
		Metadata: map[string]interface{}{
			"resourceType":   "VM",
			"clearThreshold": 70.0,
			"unit":           "%",
			"monitorOnly":    true,
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal alert: %v", err)
	}

	const want = `{
		"id":"cluster/qemu/100-cpu",
		"type":"cpu",
		"level":"warning",
		"resourceId":"cluster/qemu/100",
		"resourceName":"test-vm",
		"node":"pve-1",
		"instance":"cpu0",
		"message":"VM cpu at 95%",
		"value":95,
		"threshold":90,
		"startTime":"2026-02-08T13:14:15Z",
		"lastSeen":"2026-02-08T13:17:15Z",
		"acknowledged":false,
		"metadata":{"clearThreshold":70,"monitorOnly":true,"resourceType":"VM","unit":"%"}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AlertAllFieldsJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := start.Add(3 * time.Minute)
	ackTime := start.Add(5 * time.Minute)
	lastNotified := start.Add(2 * time.Minute)
	escalationTimes := []time.Time{start.Add(1 * time.Minute), start.Add(3 * time.Minute)}

	payload := alerts.Alert{
		ID:              "cluster/qemu/100-cpu",
		Type:            "cpu",
		Level:           alerts.AlertLevelWarning,
		ResourceID:      "cluster/qemu/100",
		ResourceName:    "test-vm",
		Node:            "pve-1",
		NodeDisplayName: "Proxmox Node 1",
		Instance:        "cpu0",
		Message:         "VM cpu at 95%",
		Value:           95.0,
		Threshold:       90.0,
		StartTime:       start,
		LastSeen:        lastSeen,
		Acknowledged:    true,
		AckTime:         &ackTime,
		AckUser:         "admin",
		Metadata: map[string]interface{}{
			"resourceType":   "VM",
			"clearThreshold": 70.0,
			"unit":           "%",
			"monitorOnly":    true,
		},
		LastNotified:    &lastNotified,
		LastEscalation:  2,
		EscalationTimes: escalationTimes,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal alert with all fields: %v", err)
	}

	const want = `{
		"id":"cluster/qemu/100-cpu",
		"type":"cpu",
		"level":"warning",
		"resourceId":"cluster/qemu/100",
		"resourceName":"test-vm",
		"node":"pve-1",
		"nodeDisplayName":"Proxmox Node 1",
		"instance":"cpu0",
		"message":"VM cpu at 95%",
		"value":95,
		"threshold":90,
		"startTime":"2026-02-08T13:14:15Z",
		"lastSeen":"2026-02-08T13:17:15Z",
		"acknowledged":true,
		"ackTime":"2026-02-08T13:19:15Z",
		"ackUser":"admin",
		"metadata":{"clearThreshold":70,"monitorOnly":true,"resourceType":"VM","unit":"%"},
		"lastNotified":"2026-02-08T13:16:15Z",
		"lastEscalation":2,
		"escalationTimes":["2026-02-08T13:15:15Z","2026-02-08T13:17:15Z"]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ModelAlertJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	ackTime := start.Add(5 * time.Minute)
	resolvedTime := start.Add(10 * time.Minute)

	t.Run("alert", func(t *testing.T) {
		payload := models.Alert{
			ID:              "cluster/qemu/100-cpu",
			Type:            "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			Node:            "pve-1",
			NodeDisplayName: "Proxmox Node 1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Value:           95.0,
			Threshold:       90.0,
			StartTime:       start,
			Acknowledged:    true,
			AckTime:         &ackTime,
			AckUser:         "admin",
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal model alert: %v", err)
		}

		forbidden := []string{`"lastSeen"`, `"metadata"`, `"lastNotified"`, `"lastEscalation"`, `"escalationTimes"`}
		for _, field := range forbidden {
			if strings.Contains(string(got), field) {
				t.Fatalf("model alert json unexpectedly contains %s: %s", field, string(got))
			}
		}

		const want = `{
			"id":"cluster/qemu/100-cpu",
			"type":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"node":"pve-1",
			"nodeDisplayName":"Proxmox Node 1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"value":95,
			"threshold":90,
			"startTime":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackTime":"2026-02-08T13:19:15Z",
			"ackUser":"admin"
		}`

		assertJSONSnapshot(t, got, want)
	})

	t.Run("resolved_alert", func(t *testing.T) {
		payload := models.ResolvedAlert{
			Alert: models.Alert{
				ID:              "cluster/qemu/100-cpu",
				Type:            "cpu",
				Level:           "warning",
				ResourceID:      "cluster/qemu/100",
				ResourceName:    "test-vm",
				Node:            "pve-1",
				NodeDisplayName: "Proxmox Node 1",
				Instance:        "cpu0",
				Message:         "VM cpu at 95%",
				Value:           95.0,
				Threshold:       90.0,
				StartTime:       start,
				Acknowledged:    true,
				AckTime:         &ackTime,
				AckUser:         "admin",
			},
			ResolvedTime: resolvedTime,
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal model resolved alert: %v", err)
		}

		forbidden := []string{`"lastSeen"`, `"metadata"`, `"lastNotified"`, `"lastEscalation"`, `"escalationTimes"`}
		for _, field := range forbidden {
			if strings.Contains(string(got), field) {
				t.Fatalf("model resolved alert json unexpectedly contains %s: %s", field, string(got))
			}
		}

		const want = `{
			"id":"cluster/qemu/100-cpu",
			"type":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"node":"pve-1",
			"nodeDisplayName":"Proxmox Node 1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"value":95,
			"threshold":90,
			"startTime":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackTime":"2026-02-08T13:19:15Z",
			"ackUser":"admin",
			"resolvedTime":"2026-02-08T13:24:15Z"
		}`

		assertJSONSnapshot(t, got, want)
	})
}

func TestContract_IncidentJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	ackTime := start.Add(5 * time.Minute)
	closedAt := start.Add(10 * time.Minute)

	t.Run("open", func(t *testing.T) {
		payload := memory.Incident{
			ID:              "incident-1",
			AlertIdentifier: "cluster/qemu/100-cpu",
			AlertType:       "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			ResourceType:    "guest",
			Node:            "pve-1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Status:          memory.IncidentStatusOpen,
			OpenedAt:        start,
			Acknowledged:    true,
			AckUser:         "admin",
			AckTime:         &ackTime,
			Events: []memory.IncidentEvent{
				{
					ID:        "evt-1",
					Type:      memory.IncidentEventAlertFired,
					Timestamp: start.Add(1 * time.Minute),
					Summary:   "CPU alert fired",
					Details: map[string]interface{}{
						"type":      "cpu",
						"level":     "warning",
						"value":     95,
						"threshold": 90,
					},
				},
				{
					ID:        "evt-2",
					Type:      memory.IncidentEventAlertAcknowledged,
					Timestamp: start.Add(5 * time.Minute),
					Summary:   "Alert acknowledged",
					Details: map[string]interface{}{
						"user": "admin",
					},
				},
			},
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal open incident: %v", err)
		}

		const want = `{
			"id":"incident-1",
			"alertIdentifier":"cluster/qemu/100-cpu",
			"alertType":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"resourceType":"guest",
			"node":"pve-1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"status":"open",
			"openedAt":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackUser":"admin",
			"ackTime":"2026-02-08T13:19:15Z",
			"events":[
				{"id":"evt-1","type":"alert_fired","timestamp":"2026-02-08T13:15:15Z","summary":"CPU alert fired","details":{"level":"warning","threshold":90,"type":"cpu","value":95}},
				{"id":"evt-2","type":"alert_acknowledged","timestamp":"2026-02-08T13:19:15Z","summary":"Alert acknowledged","details":{"user":"admin"}}
			]
		}`

		assertJSONSnapshot(t, got, want)
	})

	t.Run("resolved", func(t *testing.T) {
		payload := memory.Incident{
			ID:              "incident-1",
			AlertIdentifier: "cluster/qemu/100-cpu",
			AlertType:       "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			ResourceType:    "guest",
			Node:            "pve-1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Status:          memory.IncidentStatusResolved,
			OpenedAt:        start,
			ClosedAt:        &closedAt,
			Acknowledged:    true,
			AckUser:         "admin",
			AckTime:         &ackTime,
			Events: []memory.IncidentEvent{
				{
					ID:        "evt-1",
					Type:      memory.IncidentEventAlertFired,
					Timestamp: start.Add(1 * time.Minute),
					Summary:   "CPU alert fired",
					Details: map[string]interface{}{
						"type":      "cpu",
						"level":     "warning",
						"value":     95,
						"threshold": 90,
					},
				},
				{
					ID:        "evt-2",
					Type:      memory.IncidentEventAlertAcknowledged,
					Timestamp: start.Add(5 * time.Minute),
					Summary:   "Alert acknowledged",
					Details: map[string]interface{}{
						"user": "admin",
					},
				},
			},
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal resolved incident: %v", err)
		}

		const want = `{
			"id":"incident-1",
			"alertIdentifier":"cluster/qemu/100-cpu",
			"alertType":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"resourceType":"guest",
			"node":"pve-1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"status":"resolved",
			"openedAt":"2026-02-08T13:14:15Z",
			"closedAt":"2026-02-08T13:24:15Z",
			"acknowledged":true,
			"ackUser":"admin",
			"ackTime":"2026-02-08T13:19:15Z",
			"events":[
				{"id":"evt-1","type":"alert_fired","timestamp":"2026-02-08T13:15:15Z","summary":"CPU alert fired","details":{"level":"warning","threshold":90,"type":"cpu","value":95}},
				{"id":"evt-2","type":"alert_acknowledged","timestamp":"2026-02-08T13:19:15Z","summary":"Alert acknowledged","details":{"user":"admin"}}
			]
		}`

		assertJSONSnapshot(t, got, want)
	})
}

func TestContract_IncidentEventTypeEnumSnapshot(t *testing.T) {
	type envelope struct {
		Type memory.IncidentEventType `json:"type"`
	}

	cases := []struct {
		name string
		typ  memory.IncidentEventType
		want string
	}{
		{name: "alert_fired", typ: memory.IncidentEventAlertFired, want: `{"type":"alert_fired"}`},
		{name: "alert_acknowledged", typ: memory.IncidentEventAlertAcknowledged, want: `{"type":"alert_acknowledged"}`},
		{name: "alert_unacknowledged", typ: memory.IncidentEventAlertUnacknowledged, want: `{"type":"alert_unacknowledged"}`},
		{name: "alert_resolved", typ: memory.IncidentEventAlertResolved, want: `{"type":"alert_resolved"}`},
		{name: "ai_analysis", typ: memory.IncidentEventAnalysis, want: `{"type":"ai_analysis"}`},
		{name: "command", typ: memory.IncidentEventCommand, want: `{"type":"command"}`},
		{name: "runbook", typ: memory.IncidentEventRunbook, want: `{"type":"runbook"}`},
		{name: "note", typ: memory.IncidentEventNote, want: `{"type":"note"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(envelope{Type: tc.typ})
			if err != nil {
				t.Fatalf("marshal incident event type %q: %v", tc.name, err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_AlertFieldNamingConsistency(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{name: "alerts.Alert", typ: reflect.TypeOf(alerts.Alert{})},
		{name: "memory.Incident", typ: reflect.TypeOf(memory.Incident{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.typ.NumField(); i++ {
				field := tc.typ.Field(i)
				if !field.IsExported() {
					continue
				}

				jsonTag := field.Tag.Get("json")
				if jsonTag == "" || jsonTag == "-" {
					continue
				}

				tagName := strings.Split(jsonTag, ",")[0]
				if strings.Contains(tagName, "_") {
					t.Fatalf("field %s on %s uses snake_case json tag %q", field.Name, tc.name, tagName)
				}
			}
		})
	}
}

func TestContract_AlertResourceTypeConsistency(t *testing.T) {
	cases := []struct {
		resourceType string
		want         []string
	}{
		{resourceType: "VM", want: []string{"vm", "guest"}},
		{resourceType: "Container", want: []string{}},
		{resourceType: "Node", want: []string{"node"}},
		{resourceType: "Agent", want: []string{"agent", "node"}},
		{resourceType: "Agent Disk", want: []string{}},
		{resourceType: "PBS", want: []string{"pbs", "node"}},
		{resourceType: "Docker Container", want: []string{}},
		{resourceType: "DockerHost", want: []string{}},
		{resourceType: "Docker Service", want: []string{}},
		{resourceType: "Storage", want: []string{"storage"}},
		{resourceType: "PMG", want: []string{"pmg", "node"}},
		{resourceType: "K8s", want: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.resourceType, func(t *testing.T) {
			got := alerts.CanonicalResourceTypeKeys(tc.resourceType)
			if len(tc.want) > 0 && len(got) == 0 {
				t.Fatalf("resource type %q returned no canonical keys", tc.resourceType)
			}
			if len(tc.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("canonical keys mismatch for %q: got %v want %v", tc.resourceType, got, tc.want)
			}
		})
	}
}

func TestContract_TenantResourcesDoNotFallbackToRawSnapshotSeeding(t *testing.T) {
	now := time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-default", Hostname: "default", Status: "online", LastSeen: now}},
	}})
	h.SetTenantStateProvider(tenantResourceStateProvider{snapshots: map[string]models.StateSnapshot{
		"acme": {
			Hosts:      []models.Host{{ID: "host-tenant-snapshot", Hostname: "tenant-snapshot", Status: "online", LastSeen: now}},
			LastUpdate: time.Time{},
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rec := httptest.NewRecorder()

	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	const want = `{"data":[],"meta":{"page":1,"limit":50,"total":0,"totalPages":0},"aggregations":{"total":0,"byType":{},"byStatus":{},"bySource":{}}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("tenant resource fallback contract = %s, want %s", got, want)
	}
}

func TestContract_ResourceListPolicyMetadata(t *testing.T) {
	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{
				ID:       "vm-sensitive",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "payments-vm",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Tags:     []string{"customer-data"},
				Identity: unifiedresources.ResourceIdentity{
					Hostnames:   []string{"payments.internal"},
					IPAddresses: []string{"10.0.0.44"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=vm", nil)
	rec := httptest.NewRecorder()

	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resp.Data))
	}

	resource := resp.Data[0]
	if resource.Canonical == nil {
		t.Fatal("expected canonical identity metadata in resource contract")
	}
	if strings.TrimSpace(resource.Canonical.DisplayName) == "" {
		t.Fatal("expected canonical display name in resource contract")
	}
	if resource.Policy == nil {
		t.Fatal("expected policy metadata in resource contract")
	}
	if got := resource.Policy.Sensitivity; got != unifiedresources.ResourceSensitivityRestricted {
		t.Fatalf("policy sensitivity = %q, want %q", got, unifiedresources.ResourceSensitivityRestricted)
	}
	if got := resource.Policy.Routing.Scope; got != unifiedresources.ResourceRoutingScopeLocalOnly {
		t.Fatalf("routing scope = %q, want %q", got, unifiedresources.ResourceRoutingScopeLocalOnly)
	}
	wantRedactions := []unifiedresources.ResourceRedactionHint{
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionIPAddress,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
	}
	if !reflect.DeepEqual(resource.Policy.Routing.Redact, wantRedactions) {
		t.Fatalf("policy redact = %#v, want %#v", resource.Policy.Routing.Redact, wantRedactions)
	}
	if got := resource.AISafeSummary; !strings.Contains(got, "virtual machine resource;") || !strings.Contains(got, "local-only context") {
		t.Fatalf("aiSafeSummary = %q", got)
	}
}

func TestContract_ResourceListCarriesTimelineAndCapabilityContracts(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	occurredAt := now.Add(-2 * time.Minute)

	payload := ResourcesResponse{
		Data: []unifiedresources.Resource{
			{
				ID:       "vm-42",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "web-42",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Capabilities: []unifiedresources.ResourceCapability{
					{
						Name:                 "restart",
						Type:                 unifiedresources.CapabilityTypeCommon,
						Description:          "Restart the VM",
						MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
						Params: []unifiedresources.CapabilityParam{
							{
								Name:        "force",
								Type:        "boolean",
								Required:    false,
								Description: "Restart without graceful shutdown",
							},
						},
					},
				},
				Relationships: []unifiedresources.ResourceRelationship{
					{
						SourceID:   "vm-42",
						TargetID:   "node-1",
						Type:       unifiedresources.RelRunsOn,
						Confidence: 1,
						Active:     true,
						Discoverer: "proxmox_adapter",
						ObservedAt: now,
						LastSeenAt: now,
						Metadata: map[string]any{
							"source":  "live",
							"cluster": "pve-prod",
						},
					},
				},
				RecentChanges: []unifiedresources.ResourceChange{
					{
						ID:               "chg-42",
						ObservedAt:       now,
						OccurredAt:       &occurredAt,
						ResourceID:       "vm-42",
						Kind:             unifiedresources.ChangeStateTransition,
						From:             "offline",
						To:               "online",
						SourceType:       unifiedresources.SourcePlatformEvent,
						SourceAdapter:    unifiedresources.AdapterProxmox,
						Confidence:       unifiedresources.ConfidenceHigh,
						RelatedResources: []string{"node-1"},
						Reason:           "vm started",
						Metadata: map[string]any{
							"source": "snapshot",
							"ticket": "INC-1234",
						},
					},
				},
				FacetCounts: unifiedresources.ResourceFacetCounts{
					RecentChanges: 1,
				},
			},
		},
		Meta: ResourcesMeta{
			Page:       1,
			Limit:      50,
			Total:      1,
			TotalPages: 1,
		},
		Aggregations: unifiedresources.ResourceStats{
			Total:    1,
			ByType:   map[unifiedresources.ResourceType]int{unifiedresources.ResourceTypeVM: 1},
			ByStatus: map[unifiedresources.ResourceStatus]int{unifiedresources.StatusOnline: 1},
			BySource: map[unifiedresources.DataSource]int{unifiedresources.SourceProxmox: 1},
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource response: %v", err)
	}

	const want = `{
		"data":[
			{
				"id":"vm-42",
				"type":"vm",
				"name":"web-42",
				"status":"online",
				"lastSeen":"2026-03-18T12:00:00Z",
				"updatedAt":"0001-01-01T00:00:00Z",
				"sources":null,
				"identity":{},
				"capabilities":[
					{
						"name":"restart",
						"type":"common",
						"description":"Restart the VM",
						"minimumApprovalLevel":"admin",
						"params":[
							{
								"name":"force",
								"type":"boolean",
								"required":false,
								"isSensitive":false,
								"description":"Restart without graceful shutdown"
							}
						]
					}
				],
				"relationships":[
					{
						"sourceId":"vm-42",
						"targetId":"node-1",
						"type":"runs_on",
						"confidence":1,
						"active":true,
						"discoverer":"proxmox_adapter",
						"observedAt":"2026-03-18T12:00:00Z",
						"lastSeenAt":"2026-03-18T12:00:00Z",
						"metadata":{"cluster":"pve-prod","source":"live"}
					}
				],
				"recentChanges":[
					{
						"id":"chg-42",
						"observedAt":"2026-03-18T12:00:00Z",
						"occurredAt":"2026-03-18T11:58:00Z",
						"resourceId":"vm-42",
						"kind":"state_transition",
						"from":"offline",
						"to":"online",
						"sourceType":"platform_event",
						"sourceAdapter":"proxmox_adapter",
						"confidence":"high",
						"relatedResources":["node-1"],
						"reason":"vm started",
						"metadata":{"source":"snapshot","ticket":"INC-1234"}
					}
				],
				"facetCounts":{
					"recentChanges":1
				}
			}
		],
		"meta":{"page":1,"limit":50,"total":1,"totalPages":1},
		"aggregations":{"total":1,"byType":{"vm":1},"byStatus":{"online":1},"bySource":{"proxmox":1}}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceListUsesTenantStateProviderAtStartup(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	router := NewRouter(cfg, nil, &monitoring.MultiTenantMonitor{}, nil, nil, "1.0.0")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?page=1&limit=100", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))

	router.resourceHandlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestContract_ResourceCapabilitiesJSONSnapshot(t *testing.T) {
	payload := struct {
		ResourceID   string                                `json:"resourceId"`
		Capabilities []unifiedresources.ResourceCapability `json:"capabilities"`
		Count        int                                   `json:"count"`
	}{
		ResourceID: "vm:42",
		Capabilities: []unifiedresources.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource capabilities response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"capabilities":[
			{
				"name":"restart",
				"type":"common",
				"description":"Restart the VM",
				"minimumApprovalLevel":"admin"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceRelationshipsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                                  `json:"resourceId"`
		Relationships []unifiedresources.ResourceRelationship `json:"relationships"`
		Count         int                                     `json:"count"`
	}{
		ResourceID: "vm:42",
		Relationships: []unifiedresources.ResourceRelationship{
			{
				SourceID:   "vm:42",
				TargetID:   "node-1",
				Type:       unifiedresources.RelRunsOn,
				Confidence: 1,
				Active:     true,
				Discoverer: "proxmox_adapter",
				ObservedAt: now,
				LastSeenAt: now,
				Metadata: map[string]any{
					"source":  "live",
					"cluster": "pve-prod",
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource relationships response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"relationships":[
			{
				"sourceId":"vm:42",
				"targetId":"node-1",
				"type":"runs_on",
				"confidence":1,
				"active":true,
				"discoverer":"proxmox_adapter",
				"observedAt":"2026-03-18T17:00:00Z",
				"lastSeenAt":"2026-03-18T17:00:00Z",
				"metadata":{"cluster":"pve-prod","source":"live"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "vm:42",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-42",
				ResourceID:       "vm:42",
				ObservedAt:       now,
				OccurredAt:       &now,
				Kind:             unifiedresources.ChangeStateTransition,
				From:             "offline",
				To:               "online",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-1"},
				Reason:           "vm started",
				Metadata:         map[string]any{"source": "snapshot", "ticket": "INC-1234"},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"recentChanges":[
			{
				"id":"chg-42",
				"observedAt":"2026-03-18T17:00:00Z",
				"occurredAt":"2026-03-18T17:00:00Z",
				"resourceId":"vm:42",
				"kind":"state_transition",
				"from":"offline",
				"to":"online",
				"sourceType":"platform_event",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-1"],
				"reason":"vm started",
				"metadata":{"source":"snapshot","ticket":"INC-1234"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineRelationshipJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 5, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "vm:42",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-relationship-42",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "vm:42",
				Kind:             unifiedresources.ChangeRelationship,
				From:             "node-1",
				To:               "node-2",
				SourceType:       unifiedresources.SourcePulseDiff,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"db:alpha", "service:beta"},
				Reason:           "relationship updated",
				Metadata: map[string]any{
					"edgeType": "depends_on",
					"active":   true,
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline relationship response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"recentChanges":[
			{
				"id":"chg-relationship-42",
				"observedAt":"2026-03-18T17:05:00Z",
				"occurredAt":"2026-03-18T17:05:00Z",
				"resourceId":"vm:42",
				"kind":"relationship_change",
				"from":"node-1",
				"to":"node-2",
				"sourceType":"pulse_diff",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["db:alpha","service:beta"],
				"reason":"relationship updated",
				"metadata":{"active":true,"edgeType":"depends_on"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineRestartJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 10, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "container:7",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-restart-7",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "container:7",
				Kind:             unifiedresources.ChangeRestart,
				From:             "online|docker.restartCount=1|docker.uptimeSeconds=3600",
				To:               "online|docker.restartCount=2|docker.uptimeSeconds=120",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterDocker,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node:1", "service:api"},
				Reason:           "resource restart detected",
				Metadata: map[string]any{
					"changedFields": []string{"docker.restartCount", "docker.uptimeSeconds"},
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline restart response: %v", err)
	}

	const want = `{
		"resourceId":"container:7",
		"recentChanges":[
			{
				"id":"chg-restart-7",
				"observedAt":"2026-03-18T17:10:00Z",
				"occurredAt":"2026-03-18T17:10:00Z",
				"resourceId":"container:7",
				"kind":"restart",
				"from":"online|docker.restartCount=1|docker.uptimeSeconds=3600",
				"to":"online|docker.restartCount=2|docker.uptimeSeconds=120",
				"sourceType":"platform_event",
				"sourceAdapter":"docker_adapter",
				"confidence":"high",
				"relatedResources":["node:1","service:api"],
				"reason":"resource restart detected",
				"metadata":{"changedFields":["docker.restartCount","docker.uptimeSeconds"]}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineAnomalyJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 12, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "storage:1",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-anomaly-1",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "storage:1",
				Kind:             unifiedresources.ChangeAnomaly,
				From:             "none",
				To:               "capacity_runway_low[warning]:PBS datastore archive is READ_ONLY",
				SourceType:       unifiedresources.SourcePulseDiff,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-2", "service:db"},
				Reason:           "resource incident changed",
				Metadata: map[string]any{
					"changedFields": []string{"incidents"},
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline anomaly response: %v", err)
	}

	const want = `{
		"resourceId":"storage:1",
		"recentChanges":[
			{
				"id":"chg-anomaly-1",
				"observedAt":"2026-03-18T17:12:00Z",
				"occurredAt":"2026-03-18T17:12:00Z",
				"resourceId":"storage:1",
				"kind":"metric_anomaly",
				"from":"none",
				"to":"capacity_runway_low[warning]:PBS datastore archive is READ_ONLY",
				"sourceType":"pulse_diff",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-2","service:db"],
				"reason":"resource incident changed",
				"metadata":{"changedFields":["incidents"]}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceFacetsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Counts        struct {
			RecentChanges              int                                          `json:"recentChanges"`
			RecentChangeKinds          map[unifiedresources.ChangeKind]int          `json:"recentChangeKinds"`
			RecentChangeSourceTypes    map[unifiedresources.ChangeSourceType]int    `json:"recentChangeSourceTypes"`
			RecentChangeSourceAdapters map[unifiedresources.ChangeSourceAdapter]int `json:"recentChangeSourceAdapters"`
		} `json:"counts"`
	}{
		ResourceID: "vm:42",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-42",
				ResourceID:       "vm:42",
				ObservedAt:       now,
				OccurredAt:       &now,
				Kind:             unifiedresources.ChangeStateTransition,
				From:             "offline",
				To:               "online",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-1"},
				Metadata: map[string]any{
					"source": "snapshot",
					"ticket": "INC-1234",
				},
			},
		},
		Counts: struct {
			RecentChanges              int                                          `json:"recentChanges"`
			RecentChangeKinds          map[unifiedresources.ChangeKind]int          `json:"recentChangeKinds"`
			RecentChangeSourceTypes    map[unifiedresources.ChangeSourceType]int    `json:"recentChangeSourceTypes"`
			RecentChangeSourceAdapters map[unifiedresources.ChangeSourceAdapter]int `json:"recentChangeSourceAdapters"`
		}{
			RecentChanges:     3,
			RecentChangeKinds: map[unifiedresources.ChangeKind]int{unifiedresources.ChangeRestart: 1, unifiedresources.ChangeAnomaly: 2},
			RecentChangeSourceTypes: map[unifiedresources.ChangeSourceType]int{
				unifiedresources.SourcePlatformEvent: 1,
				unifiedresources.SourcePulseDiff:     2,
			},
			RecentChangeSourceAdapters: map[unifiedresources.ChangeSourceAdapter]int{
				unifiedresources.AdapterProxmox: 1,
				unifiedresources.AdapterDocker:  2,
			},
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource facets response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"recentChanges":[
			{
				"id":"chg-42",
				"observedAt":"2026-03-18T17:00:00Z",
				"occurredAt":"2026-03-18T17:00:00Z",
				"resourceId":"vm:42",
				"kind":"state_transition",
				"from":"offline",
				"to":"online",
				"sourceType":"platform_event",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-1"],
				"metadata":{"source":"snapshot","ticket":"INC-1234"}
			}
		],
		"counts":{
			"recentChanges":3,
			"recentChangeKinds":{
				"metric_anomaly":2,
				"restart":1
			},
			"recentChangeSourceTypes":{
				"platform_event":1,
				"pulse_diff":2
			},
			"recentChangeSourceAdapters":{
				"docker_adapter":2,
				"proxmox_adapter":1
			}
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineRejectsInvalidSourceAdapter(t *testing.T) {
	_, err := unifiedresources.ParseResourceChangeFilters(nil, nil, []string{"unsupported_adapter"})
	if err == nil {
		t.Fatal("expected invalid sourceAdapter to be rejected")
	}
}

func TestContract_UnifiedActionAuditsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		Audits     []unifiedresources.ActionAuditRecord `json:"audits"`
		Count      int                                  `json:"count"`
		ResourceID string                               `json:"resourceId,omitempty"`
	}{
		Audits: []unifiedresources.ActionAuditRecord{
			{
				ID:        "action-1",
				CreatedAt: now,
				UpdatedAt: now,
				State:     unifiedresources.ActionStateCompleted,
				Request: unifiedresources.ActionRequest{
					RequestID:      "req-1",
					ResourceID:     "vm:42",
					CapabilityName: "restart",
					Reason:         "maintenance",
					RequestedBy:    "agent:ops",
				},
				Plan: unifiedresources.ActionPlan{
					ActionID:          "action-1",
					RequestID:         "req-1",
					Allowed:           true,
					RequiresApproval:  false,
					ApprovalPolicy:    unifiedresources.ApprovalNone,
					RollbackAvailable: false,
					PlannedAt:         now,
					ExpiresAt:         now.Add(5 * time.Minute),
					ResourceVersion:   "rv-1",
					PolicyVersion:     "pv-1",
					PlanHash:          "hash-1",
				},
				Approvals: []unifiedresources.ActionApprovalRecord{
					{
						Actor:     "admin@example.com",
						Method:    unifiedresources.MethodUI,
						Timestamp: now.Add(time.Minute),
						Outcome:   unifiedresources.OutcomeApproved,
						Reason:    "approved",
					},
				},
				Result: &unifiedresources.ExecutionResult{
					Success: true,
					Output:  "done",
				},
			},
		},
		Count:      1,
		ResourceID: "vm:42",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action audits response: %v", err)
	}

	const want = `{
		"audits":[
			{
				"id":"action-1",
				"createdAt":"2026-03-18T16:00:00Z",
				"updatedAt":"2026-03-18T16:00:00Z",
				"state":"completed",
				"request":{
					"requestId":"req-1",
					"resourceId":"vm:42",
					"capabilityName":"restart",
					"reason":"maintenance",
					"requestedBy":"agent:ops"
				},
				"plan":{
					"actionId":"action-1",
					"requestId":"req-1",
					"allowed":true,
					"requiresApproval":false,
					"approvalPolicy":"none",
					"rollbackAvailable":false,
					"plannedAt":"2026-03-18T16:00:00Z",
					"expiresAt":"2026-03-18T16:05:00Z",
					"resourceVersion":"rv-1",
					"policyVersion":"pv-1",
					"planHash":"hash-1"
				},
				"approvals":[
					{
						"actor":"admin@example.com",
						"method":"ui",
						"timestamp":"2026-03-18T16:01:00Z",
						"outcome":"approved",
						"reason":"approved"
					}
				],
				"result":{
					"success":true,
					"output":"done"
				}
			}
		],
		"count":1,
		"resourceId":"vm:42"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedActionLifecycleEventsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		ActionID string                                  `json:"actionId"`
		Events   []unifiedresources.ActionLifecycleEvent `json:"events"`
		Count    int                                     `json:"count"`
	}{
		ActionID: "action-1",
		Events: []unifiedresources.ActionLifecycleEvent{
			{
				ActionID:  "action-1",
				Timestamp: now,
				State:     unifiedresources.ActionStatePlanned,
				Actor:     "system",
				Message:   "planned",
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action lifecycle response: %v", err)
	}

	const want = `{
		"actionId":"action-1",
		"events":[
			{
				"actionId":"action-1",
				"timestamp":"2026-03-18T16:00:00Z",
				"state":"planned",
				"actor":"system",
				"message":"planned"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedExportAuditsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		Audits []unifiedresources.ExportAuditRecord `json:"audits"`
		Count  int                                  `json:"count"`
	}{
		Audits: []unifiedresources.ExportAuditRecord{
			{
				ID:           "export-1",
				Timestamp:    now,
				Actor:        "agent:ops",
				EnvelopeHash: "hash-1",
				Decision:     unifiedresources.ExportRedacted,
				Destination:  "local-llama",
				Redactions:   []string{"metadata.hostname"},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal export audits response: %v", err)
	}

	const want = `{
		"audits":[
			{
				"id":"export-1",
				"timestamp":"2026-03-18T16:00:00Z",
				"actor":"agent:ops",
				"envelopeHash":"hash-1",
				"decision":"redacted",
				"destination":"local-llama",
				"redactions":["metadata.hostname"]
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedAuditLimitCapsOversizedRequests(t *testing.T) {
	if got := parseAuditLimit("5000", 100); got != 1000 {
		t.Fatalf("parseAuditLimit oversized request = %d, want 1000", got)
	}
	if got := parseAuditLimit("250", 100); got != 250 {
		t.Fatalf("parseAuditLimit normal request = %d, want 250", got)
	}
}

func TestContract_EmbeddedFrontendWarningUsesCanonicalDevEntrypoints(t *testing.T) {
	path := filepath.Join("DO_NOT_EDIT_FRONTEND_HERE.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read embedded frontend warning: %v", err)
	}

	text := string(body)
	if !strings.Contains(text, "http://127.0.0.1:5173") {
		t.Fatalf("embedded frontend warning must point to the frontend dev shell on 5173")
	}
	if !strings.Contains(text, "http://127.0.0.1:7655") {
		t.Fatalf("embedded frontend warning must identify the backend on 7655")
	}
	if strings.Contains(text, "The dev server (port 7655) will hot-reload") {
		t.Fatalf("embedded frontend warning must not describe 7655 as the hot-reload dev server")
	}
}

func TestContract_ShippedSecurityDocReferencesStayLocal(t *testing.T) {
	if shippedSecurityDocPath != "/docs/SECURITY.md" {
		t.Fatalf("expected shipped security doc path, got %q", shippedSecurityDocPath)
	}
	if shippedSecurityContainerNoticeDocAnchor != "/docs/SECURITY.md#critical-security-notice-for-container-deployments" {
		t.Fatalf("expected shipped security container notice path, got %q", shippedSecurityContainerNoticeDocAnchor)
	}
}

func mustStreamEvent(t *testing.T, eventType string, data interface{}) chat.StreamEvent {
	t.Helper()

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal stream data: %v", err)
	}

	return chat.StreamEvent{
		Type: eventType,
		Data: raw,
	}
}

func assertJSONSnapshot(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotCompact bytes.Buffer
	var wantCompact bytes.Buffer
	if err := json.Compact(&gotCompact, got); err != nil {
		t.Fatalf("compact got json: %v", err)
	}
	if err := json.Compact(&wantCompact, []byte(want)); err != nil {
		t.Fatalf("compact want json: %v", err)
	}
	if gotCompact.String() != wantCompact.String() {
		t.Fatalf("json snapshot mismatch\nwant: %s\ngot:  %s", wantCompact.String(), gotCompact.String())
	}
}
