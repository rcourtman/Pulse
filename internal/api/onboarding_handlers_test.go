package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOnboardingQRPayloadStructure(t *testing.T) {
	router, rawToken, _ := newOnboardingContractRouter(t)
	wantRelayURL := "wss://relay.example.test/ws/app"

	req := httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/onboarding/qr", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()

	router.handleGetOnboardingQR(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload onboardingQRResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode QR response: %v", err)
	}

	if payload.Schema != onboardingSchemaVersion {
		t.Fatalf("unexpected schema: got %q want %q", payload.Schema, onboardingSchemaVersion)
	}
	if payload.InstanceURL != "https://pulse.example.test" {
		t.Fatalf("unexpected instance_url: %q", payload.InstanceURL)
	}
	if payload.Relay.URL != wantRelayURL {
		t.Fatalf("unexpected relay URL: got %q want %q", payload.Relay.URL, wantRelayURL)
	}
	if !payload.Relay.Enabled {
		t.Fatalf("expected relay.enabled=true")
	}
	if payload.AuthToken != rawToken {
		t.Fatalf("unexpected auth token in QR payload")
	}
	if payload.DeepLink == "" {
		t.Fatalf("expected deep_link to be populated")
	}
}

func TestOnboardingValidateSuccessAndFailure(t *testing.T) {
	router, rawToken, _ := newOnboardingContractRouter(t)

	successBody := fmt.Sprintf(`{"instance_id":"instance-local","relay_url":"%s","auth_token":"%s"}`, "wss://relay.example.test/ws/app", rawToken)
	successReq := httptest.NewRequest(http.MethodPost, "/api/onboarding/validate", strings.NewReader(successBody))
	successRec := httptest.NewRecorder()
	router.handleValidateOnboardingConnection(successRec, successReq)

	if successRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for success path, got %d", successRec.Code)
	}

	var successResp onboardingValidationResponse
	if err := json.NewDecoder(successRec.Body).Decode(&successResp); err != nil {
		t.Fatalf("decode success response: %v", err)
	}
	if !successResp.Success {
		t.Fatalf("expected success=true, got false with diagnostics: %#v", successResp.Diagnostics)
	}
	if hasOnboardingError(successResp.Diagnostics) {
		t.Fatalf("expected no error diagnostics on success path, got %#v", successResp.Diagnostics)
	}

	failureBody := `{"instance_id":"instance-local","relay_url":"https://relay.example.test/ws/instance","auth_token":"bad-token"}`
	failureReq := httptest.NewRequest(http.MethodPost, "/api/onboarding/validate", strings.NewReader(failureBody))
	failureRec := httptest.NewRecorder()
	router.handleValidateOnboardingConnection(failureRec, failureReq)

	if failureRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for failure path, got %d", failureRec.Code)
	}

	var failureResp onboardingValidationResponse
	if err := json.NewDecoder(failureRec.Body).Decode(&failureResp); err != nil {
		t.Fatalf("decode failure response: %v", err)
	}
	if failureResp.Success {
		t.Fatalf("expected success=false for invalid payload")
	}
	if !diagnosticCodePresent(failureResp.Diagnostics, "relay_url_invalid") {
		t.Fatalf("expected relay_url_invalid diagnostic, got %#v", failureResp.Diagnostics)
	}
	if !diagnosticCodePresent(failureResp.Diagnostics, "auth_token_invalid") {
		t.Fatalf("expected auth_token_invalid diagnostic, got %#v", failureResp.Diagnostics)
	}
}

func TestOnboardingDeepLinkFormat(t *testing.T) {
	router, rawToken, _ := newOnboardingContractRouter(t)

	req := httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/onboarding/deep-link", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.handleGetOnboardingDeepLink(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response onboardingDeepLinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode deep-link response: %v", err)
	}
	if response.URL == "" {
		t.Fatalf("expected deep-link URL in response")
	}

	parsed, err := url.Parse(response.URL)
	if err != nil {
		t.Fatalf("parse deep-link URL: %v", err)
	}
	if parsed.Scheme != "pulse" {
		t.Fatalf("unexpected deep-link scheme: %q", parsed.Scheme)
	}
	if parsed.Host != "connect" {
		t.Fatalf("unexpected deep-link host: %q", parsed.Host)
	}

	query := parsed.Query()
	if query.Get("schema") != onboardingSchemaVersion {
		t.Fatalf("unexpected deep-link schema: %q", query.Get("schema"))
	}
	if query.Get("instance_url") != "https://pulse.example.test" {
		t.Fatalf("unexpected deep-link instance_url: %q", query.Get("instance_url"))
	}
	if query.Get("relay_url") != "wss://relay.example.test/ws/app" {
		t.Fatalf("unexpected deep-link relay_url: got %q want %q", query.Get("relay_url"), "wss://relay.example.test/ws/app")
	}
	if query.Get("auth_token") != rawToken {
		t.Fatalf("unexpected deep-link auth_token")
	}
}

func TestNormalizeOnboardingRelayAppURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "instance endpoint becomes app endpoint",
			in:   "wss://relay.example.test/ws/instance",
			want: "wss://relay.example.test/ws/app",
		},
		{
			name: "bare relay host appends app endpoint",
			in:   "wss://relay.example.test",
			want: "wss://relay.example.test/ws/app",
		},
		{
			name: "existing app endpoint stays stable",
			in:   "wss://relay.example.test/ws/app",
			want: "wss://relay.example.test/ws/app",
		},
		{
			name: "invalid values fail closed to trimmed original",
			in:   "not-a-websocket-url",
			want: "not-a-websocket-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOnboardingRelayAppURL(tt.in); got != tt.want {
				t.Fatalf("normalizeOnboardingRelayAppURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func newOnboardingContractRouter(t *testing.T) (*Router, string, relay.Config) {
	t.Helper()

	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath:     dataPath,
		FrontendPort: 7655,
		PublicURL:    "https://pulse.example.test",
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		t.Fatalf("generate API token: %v", err)
	}

	record, err := config.NewAPITokenRecord(rawToken, "Onboarding test token", []string{config.ScopeWildcard})
	if err != nil {
		t.Fatalf("create token record: %v", err)
	}
	cfg.APITokens = append(cfg.APITokens, *record)
	cfg.SortAPITokens()

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(dataPath),
	}

	relayCfg := relay.Config{
		Enabled:             true,
		ServerURL:           "wss://relay.example.test/ws/instance",
		IdentityPublicKey:   "pub-key",
		IdentityFingerprint: "AA:BB:CC",
	}
	if err := router.persistence.SaveRelayConfig(relayCfg); err != nil {
		t.Fatalf("save relay config: %v", err)
	}

	return router, rawToken, relayCfg
}

func diagnosticCodePresent(diagnostics []onboardingDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
