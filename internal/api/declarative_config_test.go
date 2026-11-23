package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/declarativeconfig"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type declarativeConfigTestEnv struct {
	router       *Router
	monitor      *monitoring.Monitor
	persistence  *config.ConfigPersistence
	readToken    string
	writeToken   string
	monitorToken string
	secrets      []string
}

func declarativeIntPtr(v int) *int    { return &v }
func declarativeBoolPtr(v bool) *bool { return &v }

func newDeclarativeConfigTestEnv(t *testing.T) *declarativeConfigTestEnv {
	t.Helper()

	dataDir := t.TempDir()
	readToken := "declarative-read-token.12345678"
	writeToken := "declarative-write-token.12345678"
	monitorToken := "declarative-monitor-token.12345678"

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens: []config.APITokenRecord{
			newTokenRecord(t, readToken, []string{config.ScopeSettingsRead}, nil),
			newTokenRecord(t, writeToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil),
			newTokenRecord(t, monitorToken, []string{config.ScopeMonitoringRead}, nil),
		},
	}

	persistence := config.NewConfigPersistence(dataDir)
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New() error = %v", err)
	}
	t.Cleanup(monitor.Stop)

	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "1.0.0")
	router.persistence = persistence

	return &declarativeConfigTestEnv{
		router:       router,
		monitor:      monitor,
		persistence:  persistence,
		readToken:    readToken,
		writeToken:   writeToken,
		monitorToken: monitorToken,
	}
}

func (e *declarativeConfigTestEnv) seedRichState(t *testing.T) {
	t.Helper()

	const (
		smtpPassword     = "super-secret-smtp-pass-987"
		webhookSecretURL = "https://hooks.example.com/services/T001/B002/super-secret-webhook-token-456"
		webhookAuthHdr   = "Bearer super-secret-webhook-header-val"
		webhookCustomVal = "super-secret-custom-field-val"
		webhookSignKey   = "super-secret-signing-key-321"
		appriseAPIKey    = "super-secret-apprise-apikey-111"
		appriseConfigKey = "super-secret-apprise-cfgkey-222"
		appriseTarget    = "tgram://999888777:super-secret-telegram-token/123456"
		deadManPingURL   = "https://hc-ping.com/super-secret-deadman-uuid-999"
		httpBasicPass    = "super-secret-http-probe-pass"
		httpBearerTok    = "super-secret-http-probe-bearer"
		httpHeaderVal    = "super-secret-http-probe-header"
		httpBodyVal      = `{"apiKey":"super-secret-http-probe-body"}`
	)

	e.secrets = []string{
		smtpPassword,
		"super-secret-webhook-token-456",
		webhookAuthHdr,
		webhookCustomVal,
		webhookSignKey,
		appriseAPIKey,
		appriseConfigKey,
		"super-secret-telegram-token",
		"super-secret-deadman-uuid-999",
		httpBasicPass,
		httpBearerTok,
		httpHeaderVal,
		"super-secret-http-probe-body",
	}

	alertCfg := alerts.DefaultAlertConfig()
	alertCfg.Enabled = true
	alertCfg.ActivationState = alerts.ActivationActive
	alertCfg.NodeDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 77, Clear: 72}
	if err := e.persistence.SaveAlertConfig(alertCfg); err != nil {
		t.Fatalf("SaveAlertConfig() error = %v", err)
	}
	e.monitor.GetAlertManager().UpdateConfig(alertCfg)

	intentDoc := alerts.NewAlertIntentPolicyDocument()
	intentDoc.Defaults["*"] = alerts.AlertIntentRule{
		GraceSeconds:       declarativeIntPtr(45),
		HonorOperatorState: declarativeBoolPtr(true),
	}
	if err := e.persistence.SaveAlertIntentPolicies(intentDoc); err != nil {
		t.Fatalf("SaveAlertIntentPolicies() error = %v", err)
	}
	if err := e.monitor.GetAlertManager().LoadIntentPolicies(intentDoc); err != nil {
		t.Fatalf("LoadIntentPolicies() error = %v", err)
	}

	passCopy := httpBasicPass
	bearerCopy := httpBearerTok
	headerValCopy := httpHeaderVal
	bodyCopy := httpBodyVal
	targets := []config.AvailabilityTarget{
		{
			ID:                     "avail-api-health",
			Name:                   "API Health",
			TargetKind:             config.AvailabilityTargetService,
			Address:                "https://status.example.com",
			Protocol:               config.AvailabilityProbeHTTPS,
			Path:                   "/healthz",
			Enabled:                true,
			PollIntervalSecs:       30,
			TimeoutMillis:          3000,
			FailureThreshold:       2,
			ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal},
			HTTP: &config.AvailabilityHTTPConfig{
				Method: config.AvailabilityHTTPMethodPOST,
				Headers: []config.AvailabilityHTTPHeader{
					{ID: "hdr-auth", Name: "X-Probe-Secret", Value: &headerValCopy},
				},
				Authentication: config.AvailabilityHTTPAuthentication{
					Type:     config.AvailabilityHTTPAuthBasic,
					Username: "probe-user",
					Password: &passCopy,
				},
				Body:              &bodyCopy,
				ExpectedStatusMin: 200,
				ExpectedStatusMax: 299,
			},
		},
		{
			ID:                     "avail-api-bearer",
			Name:                   "API Bearer",
			TargetKind:             config.AvailabilityTargetService,
			Address:                "https://bearer.example.com",
			Protocol:               config.AvailabilityProbeHTTPS,
			Path:                   "/ready",
			Enabled:                true,
			PollIntervalSecs:       60,
			TimeoutMillis:          2000,
			FailureThreshold:       2,
			ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal},
			HTTP: &config.AvailabilityHTTPConfig{
				Method: config.AvailabilityHTTPMethodGET,
				Authentication: config.AvailabilityHTTPAuthentication{
					Type:        config.AvailabilityHTTPAuthBearer,
					BearerToken: &bearerCopy,
				},
				ExpectedStatusMin: 200,
				ExpectedStatusMax: 204,
			},
		},
	}
	if err := e.persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}

	emailCfg := notifications.EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "alerts@example.com",
		Password: smtpPassword,
		From:     "alerts@example.com",
		To:       []string{"ops@example.com"},
		TLS:      true,
	}
	if err := e.persistence.SaveEmailConfig(emailCfg); err != nil {
		t.Fatalf("SaveEmailConfig() error = %v", err)
	}
	if err := e.monitor.GetNotificationManager().SetEmailConfig(emailCfg); err != nil {
		t.Fatalf("SetEmailConfig() error = %v", err)
	}

	webhooks := []notifications.WebhookConfig{
		{
			ID:      "wh-ops",
			Name:    "Ops Slack",
			URL:     webhookSecretURL,
			Method:  http.MethodPost,
			Enabled: true,
			Service: "slack",
			Headers: map[string]string{
				"Authorization": webhookAuthHdr,
			},
			CustomFields: map[string]string{
				"token": webhookCustomVal,
			},
			SigningSecret: webhookSignKey,
		},
	}
	if err := e.persistence.SaveWebhooks(webhooks); err != nil {
		t.Fatalf("SaveWebhooks() error = %v", err)
	}
	if err := e.monitor.GetNotificationManager().ReplaceWebhooks(webhooks); err != nil {
		t.Fatalf("ReplaceWebhooks() error = %v", err)
	}

	appriseCfg := notifications.AppriseConfig{
		Enabled:        true,
		Mode:           notifications.AppriseModeHTTP,
		ServerURL:      "https://apprise.example.com/notify",
		ConfigKey:      appriseConfigKey,
		APIKey:         appriseAPIKey,
		APIKeyHeader:   "X-API-KEY",
		TimeoutSeconds: 15,
		Targets:        []string{appriseTarget},
	}
	if err := e.persistence.SaveAppriseConfig(appriseCfg); err != nil {
		t.Fatalf("SaveAppriseConfig() error = %v", err)
	}
	if err := e.monitor.GetNotificationManager().SetAppriseConfig(appriseCfg); err != nil {
		t.Fatalf("SetAppriseConfig() error = %v", err)
	}

	deadManCfg := notifications.DeadManConfig{
		PingURL: deadManPingURL,
	}
	if err := e.persistence.SaveDeadManConfig(deadManCfg); err != nil {
		t.Fatalf("SaveDeadManConfig() error = %v", err)
	}
	if err := e.monitor.UpdateDeadManConfig(deadManCfg); err != nil {
		t.Fatalf("UpdateDeadManConfig() error = %v", err)
	}

	store, err := e.router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore() error = %v", err)
	}
	for _, st := range []unified.ResourceOperatorState{
		{
			CanonicalID:        "vm:101",
			MonitoringMode:     unified.MonitoringModeNormal,
			LifecycleState:     unified.LifecycleStateActive,
			NeverAutoRemediate: true,
			Criticality:        unified.CriticalityHigh,
			Note:               "Primary database",
			SetAt:              time.Now().UTC(),
			SetBy:              "seed-admin",
		},
		{
			CanonicalID:    "vm:102",
			MonitoringMode: unified.MonitoringModeNormal,
			LifecycleState: unified.LifecycleStateActive,
			Criticality:    unified.CriticalityLow,
			Note:           "Temporary staging box",
			SetAt:          time.Now().UTC(),
			SetBy:          "seed-admin",
		},
	} {
		if _, err := unified.SetResourceOperatorStateWithMaintenanceLifecycle(store, st); err != nil {
			t.Fatalf("SetResourceOperatorStateWithMaintenanceLifecycle(%s) error = %v", st.CanonicalID, err)
		}
	}
}

func (e *declarativeConfigTestEnv) doRequest(method, path string, body []byte, token string, headers map[string]string) *httptest.ResponseRecorder {
	reader := bytes.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("X-API-Token", token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.router.Handler().ServeHTTP(rec, req)
	return rec
}

func assertNoLeakedSecrets(t *testing.T, label, payload string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(payload, secret) {
			t.Fatalf("%s leaked secret %q", label, secret)
		}
	}
}

func assertDeclarativeCopyHygiene(t *testing.T, label, text string) {
	t.Helper()
	if strings.Contains(text, "—") {
		t.Fatalf("%s contains forbidden em-dash: %q", label, text)
	}
	if strings.Contains(text, ";") {
		t.Fatalf("%s contains forbidden semicolon: %q", label, text)
	}
}

func TestDeclarativeConfigGetJSONAndYAMLNoSecretLeak(t *testing.T) {
	env := newDeclarativeConfigTestEnv(t)
	env.seedRichState(t)

	// 1. Default JSON response
	jsonRec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, nil)
	if jsonRec.Code != http.StatusOK {
		t.Fatalf("GET JSON status = %d, want 200, body = %s", jsonRec.Code, jsonRec.Body.String())
	}
	if ct := jsonRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("GET JSON Content-Type = %q, want application/json", ct)
	}
	assertNoLeakedSecrets(t, "GET JSON", jsonRec.Body.String(), env.secrets)

	docFromJSON, err := declarativeconfig.UnmarshalDocument(jsonRec.Body.Bytes())
	if err != nil {
		t.Fatalf("UnmarshalDocument(GET JSON) error = %v", err)
	}
	if docFromJSON.APIVersion != declarativeconfig.APIVersion || docFromJSON.Kind != declarativeconfig.Kind {
		t.Fatalf("unexpected header: apiVersion=%q kind=%q", docFromJSON.APIVersion, docFromJSON.Kind)
	}
	if docFromJSON.Alerts == nil || docFromJSON.Alerts.Config == nil || docFromJSON.Alerts.IntentPolicies == nil {
		t.Fatalf("expected alerts section populated, got %+v", docFromJSON.Alerts)
	}
	if docFromJSON.AvailabilityTargets == nil || len(*docFromJSON.AvailabilityTargets) != 2 {
		t.Fatalf("expected 2 availability targets, got %+v", docFromJSON.AvailabilityTargets)
	}
	if docFromJSON.OperatorStates == nil || len(*docFromJSON.OperatorStates) != 2 {
		t.Fatalf("expected 2 operator states, got %+v", docFromJSON.OperatorStates)
	}
	if docFromJSON.Notifications == nil || docFromJSON.Notifications.Email == nil || docFromJSON.Notifications.Webhooks == nil || docFromJSON.Notifications.Apprise == nil || docFromJSON.Notifications.DeadMan == nil {
		t.Fatalf("expected all notification subsections populated, got %+v", docFromJSON.Notifications)
	}

	// 2. Query param ?format=yaml
	yamlRec := env.doRequest(http.MethodGet, "/api/config/declarative?format=yaml", nil, env.readToken, nil)
	if yamlRec.Code != http.StatusOK {
		t.Fatalf("GET YAML status = %d, want 200, body = %s", yamlRec.Code, yamlRec.Body.String())
	}
	if ct := yamlRec.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
		t.Fatalf("GET YAML Content-Type = %q, want application/yaml; charset=utf-8", ct)
	}
	assertNoLeakedSecrets(t, "GET YAML", yamlRec.Body.String(), env.secrets)
	if _, err := declarativeconfig.UnmarshalDocument(yamlRec.Body.Bytes()); err != nil {
		t.Fatalf("UnmarshalDocument(GET YAML) error = %v", err)
	}

	// 3. Accept: application/yaml header
	acceptRec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, map[string]string{
		"Accept": "application/yaml",
	})
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("GET Accept YAML status = %d, want 200, body = %s", acceptRec.Code, acceptRec.Body.String())
	}
	if ct := acceptRec.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
		t.Fatalf("GET Accept YAML Content-Type = %q, want application/yaml; charset=utf-8", ct)
	}
	assertNoLeakedSecrets(t, "GET Accept YAML", acceptRec.Body.String(), env.secrets)

	// 4. Invalid format parameter -> 400
	badFmtRec := env.doRequest(http.MethodGet, "/api/config/declarative?format=xml", nil, env.readToken, nil)
	if badFmtRec.Code != http.StatusBadRequest {
		t.Fatalf("GET invalid format status = %d, want 400", badFmtRec.Code)
	}
	var errEnv agentcapabilities.ErrorEnvelope
	if err := json.Unmarshal(badFmtRec.Body.Bytes(), &errEnv); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if errEnv.Error != agentcapabilities.AgentErrCodeInvalidDeclarativeConfig {
		t.Fatalf("error code = %q, want %q", errEnv.Error, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig)
	}
	assertDeclarativeCopyHygiene(t, "bad format error", errEnv.Message)
}

func TestDeclarativeConfigPlanDeterministicNoMutationAndErrorCodes(t *testing.T) {
	env := newDeclarativeConfigTestEnv(t)
	env.seedRichState(t)

	getRec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getRec.Code)
	}
	doc, err := declarativeconfig.UnmarshalDocument(getRec.Body.Bytes())
	if err != nil {
		t.Fatalf("UnmarshalDocument() error = %v", err)
	}
	doc.Alerts.Config.NodeDefaults.CPU.Trigger = 88
	doc.Notifications.Email.Password = &declarativeconfig.SecretField{Value: "brand-new-secret-value-never-logged"}

	envelopeBody, err := json.Marshal(map[string]any{
		"document": doc,
		"prune":    false,
	})
	if err != nil {
		t.Fatalf("marshal plan envelope: %v", err)
	}

	planRec1 := env.doRequest(http.MethodPost, "/api/config/declarative/plan", envelopeBody, env.readToken, map[string]string{
		"Content-Type": "application/json",
	})
	if planRec1.Code != http.StatusOK {
		t.Fatalf("POST plan #1 status = %d, want 200, body = %s", planRec1.Code, planRec1.Body.String())
	}
	planRec2 := env.doRequest(http.MethodPost, "/api/config/declarative/plan", envelopeBody, env.readToken, map[string]string{
		"Content-Type": "application/json",
	})
	if planRec2.Code != http.StatusOK {
		t.Fatalf("POST plan #2 status = %d, want 200, body = %s", planRec2.Code, planRec2.Body.String())
	}

	assertNoLeakedSecrets(t, "plan response", planRec1.Body.String(), append(env.secrets, "brand-new-secret-value-never-logged"))

	var plan1, plan2 declarativeconfig.PlanResult
	if err := json.Unmarshal(planRec1.Body.Bytes(), &plan1); err != nil {
		t.Fatalf("decode plan1: %v", err)
	}
	if err := json.Unmarshal(planRec2.Body.Bytes(), &plan2); err != nil {
		t.Fatalf("decode plan2: %v", err)
	}
	if !strings.HasPrefix(plan1.PlanHash, "sha256:") || plan1.PlanHash != plan2.PlanHash {
		t.Fatalf("expected deterministic sha256 planHash, got %q vs %q", plan1.PlanHash, plan2.PlanHash)
	}
	if !plan1.Summary.HasChanges || len(plan1.Changes) == 0 {
		t.Fatalf("expected planned changes, got %+v", plan1.Summary)
	}

	// Verify raw YAML body with ?prune=true also works cleanly
	onlyOneState := []declarativeconfig.OperatorStateSpec{
		{
			CanonicalID:        "vm:101",
			MonitoringMode:     unified.MonitoringModeNormal,
			LifecycleState:     unified.LifecycleStateActive,
			NeverAutoRemediate: true,
			Criticality:        unified.CriticalityHigh,
			Note:               "Primary database",
		},
	}
	yamlDoc := declarativeconfig.Document{
		APIVersion:     declarativeconfig.APIVersion,
		Kind:           declarativeconfig.Kind,
		OperatorStates: &onlyOneState,
	}
	rawYAML, err := declarativeconfig.MarshalYAML(yamlDoc)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	yamlPlanRec := env.doRequest(http.MethodPost, "/api/config/declarative/plan?prune=true", rawYAML, env.readToken, map[string]string{
		"Content-Type": "application/yaml",
	})
	if yamlPlanRec.Code != http.StatusOK {
		t.Fatalf("POST raw YAML plan status = %d, want 200, body = %s", yamlPlanRec.Code, yamlPlanRec.Body.String())
	}
	var yamlPlan declarativeconfig.PlanResult
	if err := json.Unmarshal(yamlPlanRec.Body.Bytes(), &yamlPlan); err != nil {
		t.Fatalf("decode yamlPlan: %v", err)
	}
	if !yamlPlan.Prune || yamlPlan.Summary.Deletes != 1 {
		t.Fatalf("expected prune=true with 1 delete, got prune=%v summary=%+v", yamlPlan.Prune, yamlPlan.Summary)
	}

	// Verify state was NOT mutated by planning
	loadedAlerts, err := env.persistence.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	if loadedAlerts.NodeDefaults.CPU.Trigger != 77 {
		t.Fatalf("plan mutated alert config: got trigger=%v, want 77", loadedAlerts.NodeDefaults.CPU.Trigger)
	}
	store, err := env.router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	states, err := store.ListResourceOperatorStates()
	if err != nil {
		t.Fatalf("ListResourceOperatorStates: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("plan mutated operator states: got %d, want 2", len(states))
	}

	// Verify typed error mappings
	t.Run("invalid document", func(t *testing.T) {
		badBody := []byte(`{"document":{"apiVersion":"wrong/v0","kind":"MonitoringConfig"}}`)
		rec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", badBody, env.readToken, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
		}
		var errEnv agentcapabilities.ErrorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &errEnv); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if errEnv.Error != agentcapabilities.AgentErrCodeInvalidDeclarativeConfig {
			t.Fatalf("error = %q, want %q", errEnv.Error, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig)
		}
		assertDeclarativeCopyHygiene(t, "invalid doc error", errEnv.Message)
	})

	t.Run("unresolved env secret ref", func(t *testing.T) {
		unresolvedDoc := declarativeconfig.Document{
			APIVersion: declarativeconfig.APIVersion,
			Kind:       declarativeconfig.Kind,
			Notifications: &declarativeconfig.NotificationsSection{
				DeadMan: &declarativeconfig.DeadManSpec{
					PingURL: &declarativeconfig.SecretField{SecretRef: "env:PULSE_DEADMAN_URL"},
				},
			},
		}
		body, _ := json.Marshal(map[string]any{"document": unresolvedDoc})
		rec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", body, env.readToken, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
		}
		var errEnv agentcapabilities.ErrorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &errEnv); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if errEnv.Error != agentcapabilities.AgentErrCodeDeclarativeConfigUnresolvedSecretRef {
			t.Fatalf("error = %q, want %q", errEnv.Error, agentcapabilities.AgentErrCodeDeclarativeConfigUnresolvedSecretRef)
		}
		assertDeclarativeCopyHygiene(t, "unresolved secret error", errEnv.Message)
	})

	t.Run("preserved secret missing", func(t *testing.T) {
		missingWebhooks := []declarativeconfig.WebhookSpec{
			{
				ID:      "wh-new-missing-secret",
				Name:    "New Hook",
				Enabled: true,
				URL:     &declarativeconfig.SecretField{SecretRef: declarativeconfig.SecretRefPreserved},
			},
		}
		missingDoc := declarativeconfig.Document{
			APIVersion: declarativeconfig.APIVersion,
			Kind:       declarativeconfig.Kind,
			Notifications: &declarativeconfig.NotificationsSection{
				Webhooks: &missingWebhooks,
			},
		}
		body, _ := json.Marshal(map[string]any{"document": missingDoc})
		rec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", body, env.readToken, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
		}
		var errEnv agentcapabilities.ErrorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &errEnv); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if errEnv.Error != agentcapabilities.AgentErrCodeDeclarativeConfigSecretMissing {
			t.Fatalf("error = %q, want %q", errEnv.Error, agentcapabilities.AgentErrCodeDeclarativeConfigSecretMissing)
		}
		assertDeclarativeCopyHygiene(t, "missing secret error", errEnv.Message)
	})
}

func TestDeclarativeConfigApplyGuardsAndProExternalProbeGate(t *testing.T) {
	env := newDeclarativeConfigTestEnv(t)
	env.seedRichState(t)

	getRec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, nil)
	doc, err := declarativeconfig.UnmarshalDocument(getRec.Body.Bytes())
	if err != nil {
		t.Fatalf("UnmarshalDocument: %v", err)
	}
	doc.Alerts.Config.NodeDefaults.CPU.Trigger = 85

	// 1. Missing planHash and confirm: true -> 400 confirmation required
	noConfirmBody, _ := json.Marshal(map[string]any{"document": doc})
	noConfirmRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", noConfirmBody, env.writeToken, nil)
	if noConfirmRec.Code != http.StatusBadRequest {
		t.Fatalf("missing confirmation status = %d, want 400, body = %s", noConfirmRec.Code, noConfirmRec.Body.String())
	}
	var confirmErr agentcapabilities.ErrorEnvelope
	if err := json.Unmarshal(noConfirmRec.Body.Bytes(), &confirmErr); err != nil {
		t.Fatalf("decode confirmErr: %v", err)
	}
	if confirmErr.Error != agentcapabilities.AgentErrCodeDeclarativeConfigConfirmationRequired {
		t.Fatalf("error = %q, want %q", confirmErr.Error, agentcapabilities.AgentErrCodeDeclarativeConfigConfirmationRequired)
	}
	assertDeclarativeCopyHygiene(t, "confirmation required error", confirmErr.Message)

	// 2. Plan, then mutate state underneath, then apply with stale planHash -> 409 drift
	planBody, _ := json.Marshal(map[string]any{"document": doc})
	planRec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", planBody, env.readToken, nil)
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body = %s", planRec.Code, planRec.Body.String())
	}
	var initialPlan declarativeconfig.PlanResult
	if err := json.Unmarshal(planRec.Body.Bytes(), &initialPlan); err != nil {
		t.Fatalf("decode initialPlan: %v", err)
	}

	concurrentAlerts, err := env.persistence.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	concurrentAlerts.NodeDefaults.CPU.Trigger = 79
	if err := env.persistence.SaveAlertConfig(*concurrentAlerts); err != nil {
		t.Fatalf("SaveAlertConfig concurrent: %v", err)
	}

	staleApplyBody, _ := json.Marshal(map[string]any{
		"document": doc,
		"planHash": initialPlan.PlanHash,
	})
	driftRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", staleApplyBody, env.writeToken, nil)
	if driftRec.Code != http.StatusConflict {
		t.Fatalf("stale planHash status = %d, want 409, body = %s", driftRec.Code, driftRec.Body.String())
	}
	var driftErr agentcapabilities.ErrorEnvelope
	if err := json.Unmarshal(driftRec.Body.Bytes(), &driftErr); err != nil {
		t.Fatalf("decode driftErr: %v", err)
	}
	if driftErr.Error != agentcapabilities.AgentErrCodeDeclarativeConfigPlanDrift {
		t.Fatalf("error = %q, want %q", driftErr.Error, agentcapabilities.AgentErrCodeDeclarativeConfigPlanDrift)
	}
	if driftErr.Details["expectedPlanHash"] == "" || driftErr.Details["expectedPlanHash"] == initialPlan.PlanHash {
		t.Fatalf("expected fresh expectedPlanHash in details, got %+v", driftErr.Details)
	}
	assertDeclarativeCopyHygiene(t, "drift error", driftErr.Message)

	afterDriftAlerts, _ := env.persistence.LoadAlertConfig()
	if afterDriftAlerts.NodeDefaults.CPU.Trigger != 79 {
		t.Fatalf("drifted apply mutated alerts: got %v, want 79", afterDriftAlerts.NodeDefaults.CPU.Trigger)
	}

	// 3. FeatureExternalProbe Pro gate: remote probe assignment without Pro -> 402 before ANY write
	remoteTargets := []declarativeconfig.AvailabilityTargetSpec{
		{
			ID:                     "avail-remote-branch",
			Name:                   "Branch Gateway",
			TargetKind:             config.AvailabilityTargetService,
			Address:                "gateway.branch.example.com",
			Protocol:               config.AvailabilityProbeICMP,
			Enabled:                true,
			PollIntervalSecs:       30,
			TimeoutMillis:          1000,
			FailureThreshold:       2,
			ObservationLocationIDs: []string{config.AvailabilityAgentObservationLocationID("probe-host-1")},
		},
	}
	probeDoc := declarativeconfig.Document{
		APIVersion:          declarativeconfig.APIVersion,
		Kind:                declarativeconfig.Kind,
		Alerts:              doc.Alerts,
		AvailabilityTargets: &remoteTargets,
	}
	probeApplyBody, _ := json.Marshal(map[string]any{
		"document": probeDoc,
		"confirm":  true,
	})
	unlicensedRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", probeApplyBody, env.writeToken, nil)
	if unlicensedRec.Code != http.StatusPaymentRequired {
		t.Fatalf("unlicensed remote probe status = %d, want 402, body = %s", unlicensedRec.Code, unlicensedRec.Body.String())
	}
	var licPayload map[string]any
	if err := json.Unmarshal(unlicensedRec.Body.Bytes(), &licPayload); err != nil {
		t.Fatalf("decode 402 payload: %v", err)
	}
	if licPayload["error"] != "license_required" || licPayload["feature"] != featureExternalProbeValue {
		t.Fatalf("402 payload = %+v, want license_required for %q", licPayload, featureExternalProbeValue)
	}
	// Verify NO writes occurred before 402
	stillAlerts, _ := env.persistence.LoadAlertConfig()
	if stillAlerts.NodeDefaults.CPU.Trigger != 79 {
		t.Fatalf("402 gate failed after writing alerts: got %v, want 79", stillAlerts.NodeDefaults.CPU.Trigger)
	}
	stillTargets, _ := env.persistence.LoadAvailabilityTargets()
	if len(stillTargets) != 2 {
		t.Fatalf("402 gate failed after writing availability targets: got %d, want 2", len(stillTargets))
	}

	// 4. Activate Pro tier and retry remote probe apply -> 200 OK
	setLicenseTierForHandlersForTests(t, env.router.licenseHandlers, "default", pkglicensing.TierPro)
	licensedRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", probeApplyBody, env.writeToken, nil)
	if licensedRec.Code != http.StatusOK {
		t.Fatalf("licensed remote probe status = %d, want 200, body = %s", licensedRec.Code, licensedRec.Body.String())
	}
	savedTargets, err := env.persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets: %v", err)
	}
	if len(savedTargets) != 3 {
		t.Fatalf("expected 3 targets after Pro apply (prune=false), got %d", len(savedTargets))
	}
}

func TestDeclarativeConfigApplyAtomicReloadPreservesSecretsAndAudits(t *testing.T) {
	env := newDeclarativeConfigTestEnv(t)
	env.seedRichState(t)

	capture := &auditCaptureLogger{}
	prevLogger := audit.GetLogger()
	audit.SetLogger(capture)
	prevTenantMgr := GetTenantAuditManager()
	SetTenantAuditManager(nil)
	t.Cleanup(func() {
		audit.SetLogger(prevLogger)
		SetTenantAuditManager(prevTenantMgr)
	})

	getRec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getRec.Code)
	}
	doc, err := declarativeconfig.UnmarshalDocument(getRec.Body.Bytes())
	if err != nil {
		t.Fatalf("UnmarshalDocument: %v", err)
	}

	// Mutate across all 4 domains while keeping all secrets preserved via secretRef: "preserved"
	doc.Alerts.Config.NodeDefaults.CPU.Trigger = 91
	if doc.Alerts.IntentPolicies.ResourceTypes == nil {
		doc.Alerts.IntentPolicies.ResourceTypes = make(map[string]map[string]alerts.AlertIntentRule)
	}
	doc.Alerts.IntentPolicies.ResourceTypes["guest"] = map[string]alerts.AlertIntentRule{
		"state.offline": {GraceSeconds: declarativeIntPtr(180)},
	}
	for i := range *doc.AvailabilityTargets {
		if (*doc.AvailabilityTargets)[i].ID == "avail-api-health" {
			(*doc.AvailabilityTargets)[i].TimeoutMillis = 4500
		}
	}
	newOpStates := []declarativeconfig.OperatorStateSpec{
		{
			CanonicalID:        "vm:101",
			MonitoringMode:     unified.MonitoringModeExpectedOffline,
			LifecycleState:     unified.LifecycleStateActive,
			NeverAutoRemediate: true,
			Criticality:        unified.CriticalityHigh,
			Note:               "Primary database updated",
		},
		{
			CanonicalID:        "vm:103",
			MonitoringMode:     unified.MonitoringModeNormal,
			LifecycleState:     unified.LifecycleStateActive,
			NeverAutoRemediate: false,
			Criticality:        unified.CriticalityMedium,
			Note:               "Added declaratively",
		},
	}
	doc.OperatorStates = &newOpStates
	doc.Notifications.Email.SubjectPrefix = "[Declarative]"
	(*doc.Notifications.Webhooks)[0].Name = "Ops Slack Updated"
	doc.Notifications.Apprise.TimeoutSeconds = 25

	// Plan with prune=true so vm:102 gets cleared
	planBody, _ := json.Marshal(map[string]any{
		"document": doc,
		"prune":    true,
	})
	planRec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", planBody, env.readToken, nil)
	if planRec.Code != http.StatusOK {
		t.Fatalf("plan status = %d, body = %s", planRec.Code, planRec.Body.String())
	}
	var plan declarativeconfig.PlanResult
	if err := json.Unmarshal(planRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if !plan.Summary.HasChanges || plan.Summary.Deletes != 1 {
		t.Fatalf("expected changes with 1 delete, got %+v", plan.Summary)
	}

	// Apply with verified planHash
	applyBody, _ := json.Marshal(map[string]any{
		"document": doc,
		"prune":    true,
		"planHash": plan.PlanHash,
	})
	applyRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", applyBody, env.writeToken, nil)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200, body = %s", applyRec.Code, applyRec.Body.String())
	}
	assertNoLeakedSecrets(t, "apply response", applyRec.Body.String(), env.secrets)

	var applyResp declarativeConfigApplyResponse
	if err := json.Unmarshal(applyRec.Body.Bytes(), &applyResp); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	if !applyResp.Applied || applyResp.PlanHash != plan.PlanHash {
		t.Fatalf("unexpected apply response: %+v", applyResp)
	}

	// Verify persisted AND live-reloaded state + secret preservation across every domain
	if got := env.monitor.GetAlertManager().GetConfig().NodeDefaults.CPU.Trigger; got != 91 {
		t.Fatalf("live AlertManager CPU trigger = %v, want 91", got)
	}
	liveIntent := env.monitor.GetAlertManager().GetIntentPolicies()
	if guestRule, ok := liveIntent.ResourceTypes["guest"]["state.offline"]; !ok || guestRule.GraceSeconds == nil || *guestRule.GraceSeconds != 180 {
		t.Fatalf("live AlertManager intent policies not reloaded: %+v", liveIntent.ResourceTypes)
	}

	persistedTargets, err := env.persistence.LoadAvailabilityTargets()
	if err != nil {
		t.Fatalf("LoadAvailabilityTargets: %v", err)
	}
	var healthTarget *config.AvailabilityTarget
	var bearerTarget *config.AvailabilityTarget
	for i := range persistedTargets {
		switch persistedTargets[i].ID {
		case "avail-api-health":
			healthTarget = &persistedTargets[i]
		case "avail-api-bearer":
			bearerTarget = &persistedTargets[i]
		}
	}
	if healthTarget == nil || healthTarget.TimeoutMillis != 4500 {
		t.Fatalf("persisted health target mismatch: %+v", healthTarget)
	}
	if healthTarget.HTTP == nil || healthTarget.HTTP.Authentication.Password == nil || *healthTarget.HTTP.Authentication.Password != "super-secret-http-probe-pass" {
		t.Fatalf("HTTP probe password was not preserved across declarative apply")
	}
	if healthTarget.HTTP.Body == nil || !strings.Contains(*healthTarget.HTTP.Body, "super-secret-http-probe-body") {
		t.Fatalf("HTTP probe body was not preserved across declarative apply")
	}
	if bearerTarget == nil || bearerTarget.HTTP == nil || bearerTarget.HTTP.Authentication.BearerToken == nil || *bearerTarget.HTTP.Authentication.BearerToken != "super-secret-http-probe-bearer" {
		t.Fatalf("HTTP probe bearer token was not preserved across declarative apply")
	}

	liveEmail := env.monitor.GetNotificationManager().GetEmailConfig()
	if liveEmail.SubjectPrefix != "[Declarative]" || liveEmail.Password != "super-secret-smtp-pass-987" {
		t.Fatalf("live EmailConfig mismatch: prefix=%q hasPass=%v", liveEmail.SubjectPrefix, liveEmail.Password == "super-secret-smtp-pass-987")
	}

	liveWebhooks := env.monitor.GetNotificationManager().GetWebhooks()
	if len(liveWebhooks) != 1 || liveWebhooks[0].Name != "Ops Slack Updated" ||
		!strings.Contains(liveWebhooks[0].URL, "super-secret-webhook-token-456") ||
		liveWebhooks[0].SigningSecret != "super-secret-signing-key-321" {
		t.Fatalf("live Webhooks did not update or preserve secrets: %+v", liveWebhooks)
	}

	liveApprise := env.monitor.GetNotificationManager().GetAppriseConfig()
	if liveApprise.TimeoutSeconds != 25 || liveApprise.APIKey != "super-secret-apprise-apikey-111" ||
		liveApprise.ConfigKey != "super-secret-apprise-cfgkey-222" || len(liveApprise.Targets) != 1 {
		t.Fatalf("live AppriseConfig did not update or preserve secrets: timeout=%d", liveApprise.TimeoutSeconds)
	}

	liveDeadMan := env.monitor.DeadManConfig()
	if !strings.Contains(liveDeadMan.PingURL, "super-secret-deadman-uuid-999") {
		t.Fatalf("live DeadManConfig did not preserve ping URL")
	}

	store, err := env.router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if _, found, err := store.GetResourceOperatorState("vm:102"); err != nil || found {
		t.Fatalf("expected vm:102 pruned, found=%v err=%v", found, err)
	}
	if st101, found, err := store.GetResourceOperatorState("vm:101"); err != nil || !found || st101.MonitoringMode != unified.MonitoringModeExpectedOffline {
		t.Fatalf("expected vm:101 updated to expected_offline, got found=%v state=%+v err=%v", found, st101, err)
	}
	if st103, found, err := store.GetResourceOperatorState("vm:103"); err != nil || !found || st103.Note != "Added declaratively" {
		t.Fatalf("expected vm:103 created, got found=%v state=%+v err=%v", found, st103, err)
	}

	// Verify org-scoped audit event recorded with planHash and zero secrets/em-dashes/semicolons
	events, err := capture.Query(audit.QueryFilter{})
	if err != nil {
		t.Fatalf("audit.Query: %v", err)
	}
	var applyEvent *audit.Event
	for i := range events {
		if events[i].EventType == "declarative_config_apply" {
			applyEvent = &events[i]
			break
		}
	}
	if applyEvent == nil {
		t.Fatalf("expected declarative_config_apply audit event, got %+v", events)
	}
	if !applyEvent.Success {
		t.Fatalf("expected successful audit event, got %+v", applyEvent)
	}
	if !strings.Contains(applyEvent.Details, plan.PlanHash) {
		t.Fatalf("audit details missing planHash %q: %q", plan.PlanHash, applyEvent.Details)
	}
	assertNoLeakedSecrets(t, "audit details", applyEvent.Details, env.secrets)
	assertDeclarativeCopyHygiene(t, "audit details", applyEvent.Details)

	// Verify idempotent re-apply returns applied=false with hasChanges=false
	reapplyBody, _ := json.Marshal(map[string]any{
		"document": doc,
		"prune":    true,
		"confirm":  true,
	})
	reapplyRec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", reapplyBody, env.writeToken, nil)
	if reapplyRec.Code != http.StatusOK {
		t.Fatalf("reapply status = %d, body = %s", reapplyRec.Code, reapplyRec.Body.String())
	}
	var reapplyResp declarativeConfigApplyResponse
	if err := json.Unmarshal(reapplyRec.Body.Bytes(), &reapplyResp); err != nil {
		t.Fatalf("decode reapply: %v", err)
	}
	if reapplyResp.Applied || reapplyResp.Summary.HasChanges {
		t.Fatalf("expected idempotent reapply applied=false hasChanges=false, got %+v", reapplyResp)
	}
}

func TestDeclarativeConfigRBACScopesAndTelemetry(t *testing.T) {
	env := newDeclarativeConfigTestEnv(t)

	minimalDoc, _ := json.Marshal(map[string]any{
		"document": declarativeconfig.Document{
			APIVersion: declarativeconfig.APIVersion,
			Kind:       declarativeconfig.Kind,
		},
		"confirm": true,
	})
	minimalPlan, _ := json.Marshal(map[string]any{
		"document": declarativeconfig.Document{
			APIVersion: declarativeconfig.APIVersion,
			Kind:       declarativeconfig.Kind,
		},
	})

	// Unauthenticated -> 401
	for _, tc := range []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/config/declarative", nil},
		{http.MethodPost, "/api/config/declarative/plan", minimalPlan},
		{http.MethodPost, "/api/config/declarative/apply", minimalDoc},
	} {
		rec := env.doRequest(tc.method, tc.path, tc.body, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauth %s %s status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}

	// monitoring:read -> 403 on all 3
	for _, tc := range []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/config/declarative", nil},
		{http.MethodPost, "/api/config/declarative/plan", minimalPlan},
		{http.MethodPost, "/api/config/declarative/apply", minimalDoc},
	} {
		rec := env.doRequest(tc.method, tc.path, tc.body, env.monitorToken, nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("monitoring:read %s %s status = %d, want 403", tc.method, tc.path, rec.Code)
		}
	}

	// settings:read -> 200 on GET & plan, 403 on apply
	cliHeaders := map[string]string{
		agentcapabilities.AgentSurfaceHeader: agentcapabilities.AgentSurfacePulseCLI,
	}
	if rec := env.doRequest(http.MethodGet, "/api/config/declarative", nil, env.readToken, cliHeaders); rec.Code != http.StatusOK {
		t.Fatalf("settings:read GET status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if rec := env.doRequest(http.MethodPost, "/api/config/declarative/plan", minimalPlan, env.readToken, cliHeaders); rec.Code != http.StatusOK {
		t.Fatalf("settings:read plan status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if rec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", minimalDoc, env.readToken, cliHeaders); rec.Code != http.StatusForbidden {
		t.Fatalf("settings:read apply status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}

	// settings:write -> 200 on apply
	if rec := env.doRequest(http.MethodPost, "/api/config/declarative/apply", minimalDoc, env.writeToken, cliHeaders); rec.Code != http.StatusOK {
		t.Fatalf("settings:write apply status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	// Verify telemetry markers were recorded under ExternalAgentActivityProvisioning
	history, err := env.persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) < 3 {
		t.Fatalf("expected at least 3 provisioning telemetry events (get, plan, apply), got %d: %+v", len(history.Events), history.Events)
	}
	for _, ev := range history.Events {
		if ev.Activity != config.ExternalAgentActivityProvisioning {
			t.Fatalf("activity = %q, want %q", ev.Activity, config.ExternalAgentActivityProvisioning)
		}
		if ev.Surface != config.ExternalAgentActivitySurfacePulseCLI {
			t.Fatalf("surface = %q, want %q", ev.Surface, config.ExternalAgentActivitySurfacePulseCLI)
		}
	}

	// Verify externalAgentCapabilityActivity maps all 3 declarative capabilities cleanly
	for _, capName := range []string{
		agentcapabilities.GetDeclarativeConfigCapabilityName,
		agentcapabilities.PlanDeclarativeConfigCapabilityName,
		agentcapabilities.ApplyDeclarativeConfigCapabilityName,
	} {
		activity, scope, ok := externalAgentCapabilityActivity(capName)
		if !ok || activity != config.ExternalAgentActivityProvisioning || scope == "" {
			t.Fatalf("externalAgentCapabilityActivity(%q) = (%q, %q, %v), want provisioning activity", capName, activity, scope, ok)
		}
	}
}
