package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

type hostRemovalLifecycleHTTPRuntime struct {
	config  *config.Config
	monitor *monitoring.Monitor
	router  *Router
}

func newHostRemovalLifecycleHTTPRuntime(
	t *testing.T,
	dataPath string,
	tokens []config.APITokenRecord,
) *hostRemovalLifecycleHTTPRuntime {
	t.Helper()
	cfg := &config.Config{
		DataPath:      dataPath,
		ConfigPath:    dataPath,
		MetricsDBPath: filepath.Join(dataPath, "lifecycle-metrics.db"),
		APITokens:     append([]config.APITokenRecord(nil), tokens...),
	}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "6.1.1")
	return &hostRemovalLifecycleHTTPRuntime{
		config:  cfg,
		monitor: monitor,
		router:  router,
	}
}

func (runtime *hostRemovalLifecycleHTTPRuntime) stop() {
	if runtime == nil {
		return
	}
	if runtime.router != nil {
		runtime.router.shutdownBackgroundWorkers()
		runtime.router = nil
	}
	if runtime.monitor != nil {
		runtime.monitor.Stop()
		runtime.monitor = nil
	}
}

func serveHostRemovalLifecycleRequest(
	t *testing.T,
	runtime *hostRemovalLifecycleHTTPRuntime,
	method string,
	path string,
	rawToken string,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Accept", "application/json")
	if rawToken != "" {
		req.Header.Set("X-API-Token", rawToken)
	}
	rec := httptest.NewRecorder()
	runtime.router.Handler().ServeHTTP(rec, req)
	return rec
}

func postHostRemovalLifecycleReport(
	t *testing.T,
	runtime *hostRemovalLifecycleHTTPRuntime,
	rawToken string,
	report agentshost.Report,
) (int, string, string) {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	rec := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodPost,
		"/api/agents/agent/report",
		rawToken,
		body,
	)
	var response struct {
		AgentID string `json:"agentId"`
	}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode report response: %v", err)
		}
	}
	return rec.Code, response.AgentID, rec.Body.String()
}

func TestHostAgentRemovalLifecycleThroughAuthenticatedRouterAndRestart(t *testing.T) {
	dataPath := t.TempDir()
	now := time.Now().UTC()
	const (
		adminRaw   = "issue-1581-admin-token-123.12345678"
		sharedRaw  = "issue-1581-shared-agent-token-123.12345678"
		expiredRaw = "issue-1581-expired-agent-token-123.12345678"
		wrongRaw   = "issue-1581-wrong-scope-token-123.12345678"
		revokedRaw = "issue-1581-revoked-token-123.12345678"
	)

	admin := newTokenRecord(t, adminRaw, []string{config.ScopeWildcard}, nil)
	shared := newTokenRecord(
		t,
		sharedRaw,
		[]string{config.ScopeAgentReport, config.ScopeAgentConfigRead},
		nil,
	)
	expired := newTokenRecord(t, expiredRaw, []string{config.ScopeAgentReport}, nil)
	expiredAt := now.Add(-time.Hour)
	expired.ExpiresAt = &expiredAt
	wrongScope := newTokenRecord(t, wrongRaw, []string{config.ScopeMonitoringRead}, nil)
	initialTokens := []config.APITokenRecord{admin, shared, expired, wrongScope}
	if err := config.NewConfigPersistence(dataPath).SaveAPITokens(initialTokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	targetReport := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:        "systemd-machine-id",
			MachineID: "systemd-machine-id",
			Hostname:  "target.local",
			Platform:  "linux",
		},
		Agent: agentshost.AgentInfo{
			ID:      "persisted-target-agent-id",
			Version: "6.1.1",
			Type:    "unified",
		},
		Timestamp: now,
	}
	keeperReport := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:        "keeper-machine-id",
			MachineID: "keeper-machine-id",
			Hostname:  "keeper.local",
			Platform:  "linux",
		},
		Agent: agentshost.AgentInfo{
			ID:      "keeper-agent-id",
			Version: "6.1.1",
			Type:    "unified",
		},
		Timestamp: now,
	}

	runtime := newHostRemovalLifecycleHTTPRuntime(t, dataPath, initialTokens)
	t.Cleanup(func() { runtime.stop() })

	targetStatus, targetID, targetBody := postHostRemovalLifecycleReport(t, runtime, sharedRaw, targetReport)
	if targetStatus != http.StatusOK {
		t.Fatalf("target report status = %d: %s", targetStatus, targetBody)
	}
	keeperStatus, keeperID, keeperBody := postHostRemovalLifecycleReport(t, runtime, sharedRaw, keeperReport)
	if keeperStatus != http.StatusOK {
		t.Fatalf("keeper report status = %d: %s", keeperStatus, keeperBody)
	}
	if targetID == "" || keeperID == "" || targetID == keeperID {
		t.Fatalf("active IDs target=%q keeper=%q, want distinct", targetID, keeperID)
	}

	deleteRec := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodDelete,
		"/api/agents/agent/"+targetID,
		adminRaw,
		nil,
	)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	if _, ok := runtime.config.ValidateAPIToken(sharedRaw); !ok {
		t.Fatal("shared token was revoked while the keeper still uses it")
	}
	if status, _, body := postHostRemovalLifecycleReport(t, runtime, sharedRaw, targetReport); status != http.StatusBadRequest {
		t.Fatalf("same-process stale target status = %d, want 400: %s", status, body)
	}
	configRec := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodGet,
		"/api/agents/agent/"+targetID+"/config",
		sharedRaw,
		nil,
	)
	if configRec.Code != http.StatusOK {
		t.Fatalf("shared-token keeper config status = %d, want 200: %s", configRec.Code, configRec.Body.String())
	}
	var configBody struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(configRec.Body).Decode(&configBody); err != nil {
		t.Fatalf("decode shared-token keeper config after removal: %v", err)
	}
	if configBody.AgentID != keeperID {
		t.Fatalf("removed target leaked through config as %q; want active keeper %q", configBody.AgentID, keeperID)
	}

	runtime.stop()
	reloadedTokens, err := config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens after restart: %v", err)
	}
	runtime = newHostRemovalLifecycleHTTPRuntime(t, dataPath, reloadedTokens)

	targetReport.Host.ID = targetReport.Agent.ID
	targetReport.Timestamp = now.Add(2 * time.Minute)
	if status, _, body := postHostRemovalLifecycleReport(t, runtime, sharedRaw, targetReport); status != http.StatusBadRequest {
		t.Fatalf("post-restart stale target status = %d, want 400: %s", status, body)
	}
	keeperReport.Timestamp = now.Add(2 * time.Minute)
	if status, gotID, body := postHostRemovalLifecycleReport(t, runtime, sharedRaw, keeperReport); status != http.StatusOK || gotID != keeperID {
		t.Fatalf("post-restart keeper = (%d, %q), want (200, %q): %s", status, gotID, keeperID, body)
	}

	for _, test := range []struct {
		name  string
		token string
		want  int
	}{
		{name: "revoked", token: revokedRaw, want: http.StatusUnauthorized},
		{name: "expired", token: expiredRaw, want: http.StatusUnauthorized},
		{name: "wrong scope", token: wrongRaw, want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe := targetReport
			probe.Host.ID = "probe-" + test.name
			probe.Host.MachineID = "probe-" + test.name
			probe.Host.Hostname = "probe-" + test.name + ".local"
			status, _, body := postHostRemovalLifecycleReport(t, runtime, test.token, probe)
			if status != test.want {
				t.Fatalf("status = %d, want %d: %s", status, test.want, body)
			}
		})
	}

	installRec := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodPost,
		"/api/agent-install-command",
		adminRaw,
		[]byte(`{"type":"host","enableCommands":false,"name":"issue-1581-reenroll"}`),
	)
	install := decodeHostAgentInstallResponse(t, installRec)
	freshRecord, freshRecordOK := runtime.config.ValidateAPIToken(install.Token)
	if install.Token == "" ||
		install.Record == nil ||
		!freshRecordOK ||
		!freshRecord.HasScope(config.ScopeAgentReport) {
		t.Fatalf("fresh install response lacks report token: %+v", install)
	}

	freshStatus, freshID, freshBody := postHostRemovalLifecycleReport(t, runtime, install.Token, targetReport)
	if freshStatus != http.StatusOK {
		t.Fatalf("fresh-token re-enrollment status = %d: %s", freshStatus, freshBody)
	}
	if freshID != targetID {
		t.Fatalf("fresh-token re-enrollment ID = %q, want canonical %q", freshID, targetID)
	}

	targetReport.Timestamp = now.Add(3 * time.Minute)
	if status, _, body := postHostRemovalLifecycleReport(t, runtime, sharedRaw, targetReport); status != http.StatusBadRequest {
		t.Fatalf("detached shared token status = %d, want 400: %s", status, body)
	}
	if hosts := runtime.monitor.GetLiveHostsSnapshot(); len(hosts) != 2 {
		t.Fatalf("detached shared token changed active inventory: %+v", hosts)
	}

	freshConfig := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodGet,
		"/api/agents/agent/"+targetID+"/config",
		install.Token,
		nil,
	)
	if freshConfig.Code != http.StatusOK {
		t.Fatalf("fresh-token config status = %d: %s", freshConfig.Code, freshConfig.Body.String())
	}
	oldConfig := serveHostRemovalLifecycleRequest(
		t,
		runtime,
		http.MethodGet,
		"/api/agents/agent/"+targetID+"/config",
		sharedRaw,
		nil,
	)
	if oldConfig.Code != http.StatusOK {
		t.Fatalf("shared-token keeper config status = %d, want 200: %s", oldConfig.Code, oldConfig.Body.String())
	}
	var oldConfigBody struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(oldConfig.Body).Decode(&oldConfigBody); err != nil {
		t.Fatalf("decode shared-token keeper config: %v", err)
	}
	if oldConfigBody.AgentID != keeperID {
		t.Fatalf("shared token resolved config for %q, want active keeper %q", oldConfigBody.AgentID, keeperID)
	}
}
