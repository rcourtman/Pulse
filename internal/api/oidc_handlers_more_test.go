package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/oauth2"
)

func newTestOIDCConfig() *config.OIDCConfig {
	return &config.OIDCConfig{
		Enabled:       true,
		IssuerURL:     "https://issuer.example.com",
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		RedirectURL:   "https://app.example.com/api/oidc/callback",
		Scopes:        []string{"openid", "email"},
		UsernameClaim: "preferred_username",
		EmailClaim:    "email",
		GroupsClaim:   "groups",
	}
}

func newTestOIDCService(cfg *config.OIDCConfig, authURL, tokenURL string) *OIDCService {
	return &OIDCService{
		snapshot: oidcSnapshot{
			issuer:       cfg.IssuerURL,
			clientID:     cfg.ClientID,
			clientSecret: cfg.ClientSecret,
			redirectURL:  cfg.RedirectURL,
			scopes:       append([]string{}, cfg.Scopes...),
			caBundle:     cfg.CABundle,
		},
		oauth2Cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
			Scopes: append([]string{}, cfg.Scopes...),
		},
		stateStore: newOIDCStateStore(),
	}
}

func newOIDCRouterWithService(t *testing.T, authURL, tokenURL string) (*Router, *OIDCService) {
	t.Helper()
	cfg := newTestOIDCConfig()
	svc := newTestOIDCService(cfg, authURL, tokenURL)
	router := &Router{config: &config.Config{OIDC: cfg}, oidcService: svc}
	t.Cleanup(func() {
		if svc.stateStore != nil {
			svc.stateStore.Stop()
		}
	})
	return router, svc
}

func newOIDCDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                server.URL,
				"authorization_endpoint":                server.URL + "/authorize",
				"token_endpoint":                        server.URL + "/token",
				"jwks_uri":                              server.URL + "/keys",
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"keys":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestHandleOIDCLogin_MethodNotAllowed(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: newTestOIDCConfig()}}
	req := httptest.NewRequest(http.MethodPut, "/api/oidc/login", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleOIDCLogin_InvalidJSON(t *testing.T) {
	router, _ := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	req := httptest.NewRequest(http.MethodPost, "/api/oidc/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "invalid_request" {
		t.Fatalf("code = %v, want invalid_request", payload["code"])
	}
}

func TestHandleOIDCLogin_GetSuccess(t *testing.T) {
	router, svc := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	req := httptest.NewRequest(http.MethodGet, "/api/oidc/login?returnTo=/dashboard", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if u.Host != "auth.example.com" {
		t.Fatalf("unexpected auth host: %q", u.Host)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("expected state param in redirect")
	}
	entry, ok := svc.consumeState(state)
	if !ok {
		t.Fatalf("expected state entry to be stored")
	}
	if entry.ReturnTo != "/dashboard" {
		t.Fatalf("returnTo = %q, want /dashboard", entry.ReturnTo)
	}
}

func TestHandleOIDCLogin_PostSuccess(t *testing.T) {
	router, svc := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	body := strings.NewReader(`{"returnTo":"/home"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/oidc/login", body)
	rec := httptest.NewRecorder()

	router.handleOIDCLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload struct {
		AuthorizationURL string `json:"authorizationUrl"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AuthorizationURL == "" {
		t.Fatalf("expected authorizationUrl in response")
	}
	u, err := url.Parse(payload.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorizationUrl: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("expected state param in authorizationUrl")
	}
	entry, ok := svc.consumeState(state)
	if !ok {
		t.Fatalf("expected state entry to be stored")
	}
	if entry.ReturnTo != "/home" {
		t.Fatalf("returnTo = %q, want /home", entry.ReturnTo)
	}
}

func TestGetOIDCService_ReturnsCachedService(t *testing.T) {
	cfg := newTestOIDCConfig()
	svc := newTestOIDCService(cfg, "https://auth.example.com/authorize", "https://token.example.com")
	router := &Router{config: &config.Config{OIDC: cfg}, oidcService: svc}
	defer svc.stateStore.Stop()

	got, err := router.getOIDCService(context.Background(), cfg.RedirectURL)
	if err != nil {
		t.Fatalf("getOIDCService error: %v", err)
	}
	if got != svc {
		t.Fatalf("expected cached service to be returned")
	}
}

func TestGetOIDCService_ReplacesAndStopsPreviousService(t *testing.T) {
	discovery := newOIDCDiscoveryServer(t)
	defer discovery.Close()

	cfg := newTestOIDCConfig()
	cfg.IssuerURL = discovery.URL
	cfg.RedirectURL = "https://app.example.com/callback-old"

	old := newTestOIDCService(cfg, discovery.URL+"/authorize", discovery.URL+"/token")
	router := &Router{config: &config.Config{OIDC: cfg}, oidcService: old}

	got, err := router.getOIDCService(context.Background(), "https://app.example.com/callback-new")
	if err != nil {
		t.Fatalf("getOIDCService error: %v", err)
	}
	if got == old {
		t.Fatalf("expected a new OIDC service instance")
	}
	t.Cleanup(got.Stop)

	select {
	case <-old.stateStore.stopCleanup:
	default:
		t.Fatalf("expected previous OIDC service state store to be stopped")
	}
}

func TestHandleOIDCCallback_MethodNotAllowed(t *testing.T) {
	router := &Router{config: &config.Config{OIDC: newTestOIDCConfig()}}
	req := httptest.NewRequest(http.MethodPost, "/api/oidc/callback", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleOIDCCallback_ErrorParam(t *testing.T) {
	router, _ := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?error=access_denied", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=access_denied") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleOIDCCallback_MissingState(t *testing.T) {
	router, _ := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=missing_state") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleOIDCCallback_InvalidState(t *testing.T) {
	router, _ := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?state=invalid", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=invalid_state") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleOIDCCallback_MissingCode(t *testing.T) {
	router, svc := newOIDCRouterWithService(t, "https://auth.example.com/authorize", "")
	state, _, err := svc.newStateEntry("/dashboard")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?state="+url.QueryEscape(state), nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=missing_code") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
	if !strings.HasPrefix(location, "/dashboard") {
		t.Fatalf("expected redirect back to /dashboard, got %q", location)
	}
}

func TestHandleOIDCCallback_ExchangeFailed(t *testing.T) {
	tokenServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tokenServer.Close()

	router, svc := newOIDCRouterWithService(t, "https://auth.example.com/authorize", tokenServer.URL)
	svc.httpClient = tokenServer.Client()
	state, _, err := svc.newStateEntry("/dashboard")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?state="+url.QueryEscape(state)+"&code=abc", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=exchange_failed") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleOIDCCallback_MissingIDToken(t *testing.T) {
	tokenServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	router, svc := newOIDCRouterWithService(t, "https://auth.example.com/authorize", tokenServer.URL)
	svc.httpClient = tokenServer.Client()
	state, _, err := svc.newStateEntry("/dashboard")
	if err != nil {
		t.Fatalf("newStateEntry error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oidc/callback?state="+url.QueryEscape(state)+"&code=abc", nil)
	rec := httptest.NewRecorder()

	router.handleOIDCCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "oidc_error=missing_id_token") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}
