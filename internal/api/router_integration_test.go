package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	internalws "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type integrationServer struct {
	server  *httptest.Server
	monitor *monitoring.Monitor
	hub     *internalws.Hub
	config  *config.Config
}

func newIntegrationServer(t *testing.T) *integrationServer {
	return newIntegrationServerWithConfig(t, nil)
}

func newIntegrationServerWithConfig(t *testing.T, customize func(*config.Config)) *integrationServer {
	t.Helper()

	t.Setenv("PULSE_MOCK_MODE", "true")
	mock.SetEnabled(true)

	tmpDir := t.TempDir()
	cfg := &config.Config{
		BackendHost:       "127.0.0.1",
		BackendPort:       7655,
		ConfigPath:        tmpDir,
		DataPath:          tmpDir,
		DemoMode:          false,
		AllowedOrigins:    "*",
		ConcurrentPolling: true,
		EnvOverrides:      make(map[string]bool),
	}

	if customize != nil {
		customize(cfg)
	}

	var monitor *monitoring.Monitor
	hub := internalws.NewHub(func() interface{} {
		if monitor == nil {
			return models.StateSnapshot{}
		}
		return monitor.GetState().ToFrontend()
	})

	go hub.Run()

	var err error
	monitor, err = monitoring.New(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	monitor.SetMockMode(true)

	hub.SetStateGetter(func() interface{} {
		return monitor.GetState().ToFrontend()
	})

	version := readRuntimeVersion(t)
	if version == "" {
		version = "dev"
	}

	router := api.NewRouter(cfg, monitor, hub, func() error {
		monitor.SyncAlertState()
		return nil
	}, version)

	srv := httptest.NewServer(router.Handler())
	t.Cleanup(func() {
		srv.Close()
		mock.SetEnabled(false)
	})

	return &integrationServer{
		server:  srv,
		monitor: monitor,
		hub:     hub,
		config:  cfg,
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if payload["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %v", payload["status"])
	}
}

func TestVersionEndpointUsesRepoVersion(t *testing.T) {
	srv := newIntegrationServer(t)

	releaseVersion := readVersionFile(t)
	runtimeVersion := readRuntimeVersion(t)

	res, err := http.Get(srv.server.URL + "/api/version")
	if err != nil {
		t.Fatalf("version request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version response: %v", err)
	}

	actual, ok := payload["version"].(string)
	if !ok {
		t.Fatalf("version field missing or not a string: %v", payload["version"])
	}

	if strings.HasPrefix(actual, "0.0.0-") {
		// Development builds normalize to 0.0.0-<branch>[...], which is expected.
		return
	}

	normalizedActual := normalizeVersion(actual)
	if releaseVersion != "" && normalizedActual == normalizeVersion(releaseVersion) {
		return
	}

	if normalizedActual == normalizeVersion(runtimeVersion) {
		return
	}

	t.Fatalf("expected version to match release %q or runtime %q, got %s", releaseVersion, runtimeVersion, actual)
}

func TestAlertAcknowledge_AllowsPrintableAlertIDs(t *testing.T) {
	srv := newIntegrationServer(t)

	// This ID includes parentheses which were rejected in v5 RC builds due to overly strict
	// validation. The request should make it to the alert manager, which returns 404 because
	// the alert does not exist in this test environment.
	alertID := "docker(host)-container-unhealthy"
	escaped := url.PathEscape(alertID)

	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/"+escaped+"/acknowledge", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ack request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusNotFound, string(body))
	}
}

func TestStateEndpointReturnsMockData(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/state")
	if err != nil {
		t.Fatalf("state request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read state response: %v", err)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("unmarshal state response: %v", err)
	}

	nodes, ok := snapshot["nodes"].([]any)
	if !ok {
		t.Fatalf("state response missing nodes array: %s", string(body))
	}

	if len(nodes) == 0 {
		t.Fatalf("expected nodes in state response, got none")
	}
}

func TestProtectedEndpointsRequireAuthentication(t *testing.T) {
	passwordHash, err := internalauth.HashPassword("supersecret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.AuthUser = "admin"
		cfg.AuthPass = passwordHash
	})

	client := &http.Client{}

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/state"},
		{"GET", "/api/storage/test-storage"},
		{"GET", "/api/backups"},
		{"GET", "/api/updates/status"},
		{"POST", "/api/updates/apply"},
		{"GET", "/api/alerts/active"},
		{"GET", "/api/notifications/email"},
	}

	for _, ep := range endpoints {
		req, err := http.NewRequest(ep.method, srv.server.URL+ep.path, nil)
		if err != nil {
			t.Fatalf("build request for %s %s: %v", ep.method, ep.path, err)
		}

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("request for %s %s failed: %v", ep.method, ep.path, err)
		}
		_ = res.Body.Close()

		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 401/403 for %s %s, got %d", ep.method, ep.path, res.StatusCode)
		}
	}
}

func TestAPIOnlyModeRequiresToken(t *testing.T) {
	const rawToken = "apitoken-test-1234567890"

	tokenRecord, err := config.NewAPITokenRecord(rawToken, "test token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("create token record: %v", err)
	}

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.AuthUser = ""
		cfg.AuthPass = ""
		cfg.APITokenEnabled = true
		cfg.APITokens = []config.APITokenRecord{*tokenRecord}
	})

	client := &http.Client{}

	// Without token should be rejected.
	req, err := http.NewRequest("GET", srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("build unauthenticated request: %v", err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	_ = res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without API token, got %d", res.StatusCode)
	}

	// With the correct token should succeed.
	req, err = http.NewRequest("GET", srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("build authenticated request: %v", err)
	}
	req.Header.Set("X-API-Token", rawToken)

	res, err = client.Do(req)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid API token, got %d", res.StatusCode)
	}
}

func TestServerInfoEndpointReportsDevelopment(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/server/info")
	if err != nil {
		t.Fatalf("server info request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode server info response: %v", err)
	}

	isDev, ok := payload["isDevelopment"].(bool)
	if !ok {
		t.Fatalf("isDevelopment missing or not bool: %v", payload["isDevelopment"])
	}
	if !isDev {
		t.Fatalf("expected development mode to be true")
	}

	version, ok := payload["version"].(string)
	if !ok {
		t.Fatalf("version missing or not string: %v", payload["version"])
	}
	if version == "" {
		t.Fatalf("expected non-empty version string")
	}
}

func TestServerInfoEndpointMethodNotAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/server/info", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("server info POST request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req, err := http.NewRequest(method, srv.server.URL+"/api/health", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("health %s request failed: %v", method, err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: unexpected status: got %d want %d", method, res.StatusCode, http.StatusMethodNotAllowed)
		}
	}
}

func TestHealthEndpointHeadAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	req, err := http.NewRequest(http.MethodHead, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health HEAD request failed: %v", err)
	}
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("HEAD: unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}
}

func TestConfigNodesUsesMockTopology(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("config nodes request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var nodes []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&nodes); err != nil {
		t.Fatalf("decode config nodes response: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatalf("expected at least one mock node definition")
	}
}

func TestMockModeToggleEndpoint(t *testing.T) {
	srv := newIntegrationServer(t)

	if !mock.IsMockEnabled() {
		t.Fatalf("mock mode should be enabled at start of test")
	}

	disablePayload := bytes.NewBufferString(`{"enabled": false}`)
	res, err := http.Post(srv.server.URL+"/api/system/mock-mode", "application/json", disablePayload)
	if err != nil {
		t.Fatalf("disable mock mode request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status disabling mock mode: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var response struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode mock mode response: %v", err)
	}
	if response.Enabled {
		t.Fatalf("expected mock mode to be disabled")
	}
	if mock.IsMockEnabled() {
		t.Fatalf("mock mode global flag not disabled")
	}

	enablePayload := bytes.NewBufferString(`{"enabled": true}`)
	resEnable, err := http.Post(srv.server.URL+"/api/system/mock-mode", "application/json", enablePayload)
	if err != nil {
		t.Fatalf("enable mock mode request failed: %v", err)
	}
	defer resEnable.Body.Close()

	if resEnable.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status enabling mock mode: got %d want %d", resEnable.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(resEnable.Body).Decode(&response); err != nil {
		t.Fatalf("decode enable response: %v", err)
	}
	if !response.Enabled {
		t.Fatalf("expected mock mode to be enabled after re-enable call")
	}
}

func TestAuthenticatedEndpointsRequireToken(t *testing.T) {
	const apiToken = "test-token"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.APITokenEnabled = true
		record, err := config.NewAPITokenRecord(apiToken, "Integration test token", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	url := srv.server.URL + "/api/config/nodes"

	// Without token -> unauthorized
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", res.StatusCode)
	}

	// With wrong token -> still unauthorized
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-API-Token", "wrong-token")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request with wrong token failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", res.StatusCode)
	}

	// With correct token -> success
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create authenticated request: %v", err)
	}
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", res.StatusCode)
	}

	// Admin endpoint should reject without token and accept with token
	postURL := srv.server.URL + "/api/config/nodes"

	req, err = http.NewRequest(http.MethodPost, postURL, bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unauthenticated POST failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for POST without token, got %d", res.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPost, postURL, bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("create authenticated POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated POST failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected POST to require auth but got 401 even with valid token")
	}
	if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusForbidden && res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for authenticated POST: %d", res.StatusCode)
	}
}

func TestAPITokenQueryParameterRejected(t *testing.T) {
	const apiToken = "query-token-1234567890"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.APITokenEnabled = true
		record, err := config.NewAPITokenRecord(apiToken, "Query token test", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
	})

	queryURL := srv.server.URL + "/api/state?token=" + apiToken
	res, err := http.Get(queryURL)
	if err != nil {
		t.Fatalf("query parameter request failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token supplied via query string, got %d", res.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("create header-auth request: %v", err)
	}
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("header-auth request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with header token, got %d", res.StatusCode)
	}
}

func TestRecoveryEndpointRequiresDirectLoopback(t *testing.T) {
	srv := newIntegrationServer(t)

	generateBody := strings.NewReader(`{"action":"generate_token"}`)
	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/security/recovery", generateBody)
	if err != nil {
		t.Fatalf("create generate token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("generate token request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 generating token from loopback, got %d", res.StatusCode)
	}

	forwardedBody := strings.NewReader(`{"action":"generate_token"}`)
	reqForwarded, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/security/recovery", forwardedBody)
	if err != nil {
		t.Fatalf("create forwarded request: %v", err)
	}
	reqForwarded.Header.Set("Content-Type", "application/json")
	reqForwarded.Header.Set("X-Forwarded-For", "198.51.100.42")

	resForwarded, err := http.DefaultClient.Do(reqForwarded)
	if err != nil {
		t.Fatalf("forwarded recovery request failed: %v", err)
	}
	defer resForwarded.Body.Close()
	if resForwarded.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when forwarded headers present, got %d", resForwarded.StatusCode)
	}
}

func TestWebSocketSendsInitialState(t *testing.T) {
	srv := newIntegrationServer(t)

	wsURL := "ws" + strings.TrimPrefix(srv.server.URL, "http") + "/ws"

	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	readMsg := func() (string, map[string]any) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		typeVal, _ := msg["type"].(string)
		payload := map[string]any{}
		if raw, ok := msg["data"].(map[string]any); ok {
			payload = raw
		}
		return typeVal, payload
	}

	msgType, _ := readMsg()
	if msgType != "welcome" {
		t.Fatalf("expected welcome message, got %q", msgType)
	}

	msgType, payload := readMsg()
	if msgType != "initialState" {
		t.Fatalf("expected initialState message, got %q", msgType)
	}

	nodesVal, ok := payload["nodes"].([]any)
	if !ok || len(nodesVal) == 0 {
		t.Fatalf("initial state missing nodes: %v", payload["nodes"])
	}

	// Broadcast an additional state update and ensure clients receive it
	state := srv.monitor.GetState().ToFrontend()
	srv.hub.BroadcastState(state)

	msgType, payload = readMsg()
	if msgType != "rawData" {
		t.Fatalf("expected rawData broadcast, got %q", msgType)
	}
	if _, ok := payload["nodes"].([]any); !ok {
		t.Fatalf("broadcast payload missing nodes: %v", payload)
	}

	nodes := payload["nodes"].([]any)
	firstNode := nodes[0].(map[string]any)
	requiredNodeKeys := []string{"id", "displayName", "cpu", "memory", "status"}
	for _, key := range requiredNodeKeys {
		if _, ok := firstNode[key]; !ok {
			t.Fatalf("node payload missing key %q: %v", key, firstNode)
		}
	}

	dockerHosts, ok := payload["dockerHosts"].([]any)
	if !ok || len(dockerHosts) == 0 {
		t.Fatalf("expected dockerHosts slice in payload: %v", payload["dockerHosts"])
	}
	firstHost := dockerHosts[0].(map[string]any)
	if _, ok := firstHost["containers"].([]any); !ok {
		t.Fatalf("docker host missing containers array: %v", firstHost)
	}
}

func TestSessionCookieAllowsAuthenticatedAccess(t *testing.T) {
	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.APITokenEnabled = false
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	noCookieResp, err := http.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	noCookieResp.Body.Close()
	if noCookieResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", noCookieResp.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "super-secure-pass",
	})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}

	loginReq, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/login", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	sessionURL, _ := url.Parse(srv.server.URL)
	cookies := jar.Cookies(sessionURL)
	var hasSessionCookie bool
	for _, c := range cookies {
		if c.Name == "pulse_session" && c.Value != "" {
			hasSessionCookie = true
			break
		}
	}
	if !hasSessionCookie {
		t.Fatalf("expected pulse_session cookie after login")
	}

	authedResp, err := client.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer authedResp.Body.Close()
	if authedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", authedResp.StatusCode)
	}
}

func TestPublicURLDetectionUsesForwardedHeaders(t *testing.T) {
	const apiToken = "public-url-detection-token-12345"

	// Configure 127.0.0.1 as trusted proxy so X-Forwarded-* headers are read
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	api.ResetTrustedProxyConfigForTests()

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.APITokenEnabled = true
		record, err := config.NewAPITokenRecord(apiToken, "Public URL detection test", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
	})

	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "pulse.example.com")
	req.Header.Set("X-Forwarded-Port", "8443")
	req.Header.Set("X-API-Token", apiToken)

	res, err := srv.server.Client().Do(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	res.Body.Close()

	expected := "https://pulse.example.com:8443"
	if got := srv.config.PublicURL; got != expected {
		t.Fatalf("expected config public URL %q, got %q", expected, got)
	}

	if mgr := srv.monitor.GetNotificationManager(); mgr != nil {
		if actual := mgr.GetPublicURL(); actual != expected {
			t.Fatalf("expected notification manager public URL %q, got %q", expected, actual)
		}
	}
}

func TestPublicURLDetectionRespectsEnvOverride(t *testing.T) {
	const overrideURL = "https://from-env.example.com"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.PublicURL = overrideURL
		cfg.EnvOverrides["publicURL"] = true
	})

	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "ignored.example.org")

	res, err := srv.server.Client().Do(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	res.Body.Close()

	if got := srv.config.PublicURL; got != overrideURL {
		t.Fatalf("expected config public URL to remain %q, got %q", overrideURL, got)
	}

	if mgr := srv.monitor.GetNotificationManager(); mgr != nil {
		if actual := mgr.GetPublicURL(); actual != overrideURL {
			t.Fatalf("expected notification manager public URL %q, got %q", overrideURL, actual)
		}
	}
}

func readVersionFile(t *testing.T) string {
	t.Helper()

	versionPath := filepath.Join("..", "..", "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readRuntimeVersion(t *testing.T) string {
	t.Helper()

	info, err := updates.GetCurrentVersion()
	if err != nil {
		t.Fatalf("failed to determine current version: %v", err)
	}
	return strings.TrimSpace(info.Version)
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSuffix(v, "-dirty")
	// Strip pre-release metadata (after '-')
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	// Strip build metadata (after '+')
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}
	return v
}
