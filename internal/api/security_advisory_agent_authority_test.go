package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func agentAuthorityRouter(t *testing.T, withMonitor bool, tokens ...config.APITokenRecord) (*Router, *monitoring.Monitor) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	hash, err := internalauth.HashPassword("OriginalPass!1")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "instance-admin", AuthPass: hash, APITokens: tokens}
	InitSessionStore(dir)
	var monitor *monitoring.Monitor
	if withMonitor {
		monitor, err = monitoring.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(monitor.Stop)
	}
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "test")
	cleanupTestRouter(t, router)
	return router, monitor
}

func agentAuthorityRequest(t *testing.T, method, path, body, session, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: session})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(session))
	}
	if token != "" {
		req.Header.Set("X-API-Token", token)
	}
	return req
}

func TestAgentIngestRoutesRequireAuthorityForSessions(t *testing.T) {
	const hostToken = "host-report-fixture.12345678"
	const dockerToken = "docker-report-fixture.12345678"
	const kubeToken = "kubernetes-report-fixture.12345678"
	const otherToken = "monitor-read-fixture.12345678"
	router, _ := agentAuthorityRouter(t, false,
		newTokenRecord(t, hostToken, []string{config.ScopeAgentReport}, nil),
		newTokenRecord(t, dockerToken, []string{config.ScopeDockerReport}, nil),
		newTokenRecord(t, kubeToken, []string{config.ScopeKubernetesReport}, nil),
		newTokenRecord(t, otherToken, []string{config.ScopeMonitoringRead}, nil),
	)
	viewer := generateSessionToken()
	GetSessionStore().CreateSession(viewer, time.Hour, "browser", "127.0.0.1", "viewer")
	admin := generateSessionToken()
	GetSessionStore().CreateSession(admin, time.Hour, "browser", "127.0.0.1", "instance-admin")
	for _, tc := range []struct{ method, path, token string }{
		{http.MethodPost, "/api/agents/agent/report", hostToken},
		{http.MethodPost, "/api/agents/host/report", hostToken},
		{http.MethodPost, "/api/agents/docker/report", dockerToken},
		{http.MethodPost, "/api/agents/kubernetes/report", kubeToken},
		{http.MethodPost, "/api/agents/docker/commands/test/ack", dockerToken},
		{http.MethodGet, "/api/agents/agent/host-1/config", hostToken},
		{http.MethodGet, "/api/agents/host/host-1/config", hostToken},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, identity := range []struct {
				name, session, token string
				denied               bool
			}{
				{"viewer", viewer, "", true},
				{"unrelated token", "", otherToken, true},
				{"administrator", admin, "", false},
				{"agent token", "", tc.token, false},
			} {
				t.Run(identity.name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					router.Handler().ServeHTTP(rec, agentAuthorityRequest(t, tc.method, tc.path, "{", identity.session, identity.token))
					if identity.denied && rec.Code != http.StatusForbidden {
						t.Fatalf("status=%d, want 403: %s", rec.Code, rec.Body.String())
					}
					if !identity.denied && (rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden) {
						t.Fatalf("authorized identity denied: status=%d %s", rec.Code, rec.Body.String())
					}
				})
			}
		})
	}
}

func TestAgentConfigProtectsAvailabilityCredentialsFromViewer(t *testing.T) {
	const agentToken = "agent-config-fixture.12345678"
	record := newTokenRecord(t, agentToken, []string{config.ScopeAgentConfigRead}, nil)
	router, monitor := agentAuthorityRouter(t, true, record)
	hostID := seedUnifiedAgentHost(t, monitor)
	for _, host := range monitor.GetLiveHostsSnapshot() {
		if host.ID == hostID {
			host.TokenID = record.ID
			monitorState(t, monitor).UpsertHost(host)
		}
	}
	monitor.SetLicenseChecker(func(string) bool { return true })
	secret := "synthetic-probe-credential"
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID: "probe", Name: "fixture", Address: "example.invalid", Protocol: config.AvailabilityProbeHTTPS,
		Enabled: true, ProbeAgentID: hostID,
		HTTP: &config.AvailabilityHTTPConfig{Authentication: config.AvailabilityHTTPAuthentication{
			Type: config.AvailabilityHTTPAuthBearer, BearerToken: &secret,
		}},
	})
	if err := config.NewConfigPersistence(router.config.DataPath).SaveAvailabilityTargets([]config.AvailabilityTarget{target}); err != nil {
		t.Fatal(err)
	}
	viewer := generateSessionToken()
	GetSessionStore().CreateSession(viewer, time.Hour, "browser", "127.0.0.1", "viewer")
	admin := generateSessionToken()
	GetSessionStore().CreateSession(admin, time.Hour, "browser", "127.0.0.1", "instance-admin")
	for _, identity := range []struct {
		name, session, token string
		allowed              bool
	}{
		{"viewer", viewer, "", false},
		{"administrator", admin, "", true},
		{"bound agent", "", agentToken, true},
	} {
		t.Run(identity.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			path := "/api/agents/agent/" + hostID + "/config"
			router.Handler().ServeHTTP(rec, agentAuthorityRequest(t, http.MethodGet, path, "", identity.session, identity.token))
			if identity.allowed {
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), secret) {
					t.Fatalf("authorized config did not carry assigned credential: status=%d body=%s", rec.Code, rec.Body.String())
				}
			} else if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), secret) {
				t.Fatalf("viewer received config: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAgentReportRejectsViewerWithoutChangingState(t *testing.T) {
	const token = "host-ingest-fixture.12345678"
	router, monitor := agentAuthorityRouter(t, true,
		newTokenRecord(t, token, []string{config.ScopeAgentReport}, nil),
	)
	viewer := generateSessionToken()
	GetSessionStore().CreateSession(viewer, time.Hour, "browser", "127.0.0.1", "viewer")
	body := `{"agent":{"id":"fixture-agent","version":"1"},"host":{"id":"fixture-host","hostname":"fixture.example.invalid","platform":"linux"}}`
	path := "/api/agents/agent/report"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, agentAuthorityRequest(t, http.MethodPost, path, body, viewer, ""))
	if rec.Code != http.StatusForbidden || len(monitor.GetLiveHostsSnapshot()) != 0 {
		t.Fatalf("viewer report status=%d hosts=%v", rec.Code, monitor.GetLiveHostsSnapshot())
	}
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, agentAuthorityRequest(t, http.MethodPost, path, body, "", token))
	if rec.Code != http.StatusOK || len(monitor.GetLiveHostsSnapshot()) != 1 {
		t.Fatalf("agent report status=%d hosts=%v body=%s", rec.Code, monitor.GetLiveHostsSnapshot(), rec.Body.String())
	}
}

func TestAgentIngestRoutesRespectProxyRolesAndLocalBootstrap(t *testing.T) {
	for _, mode := range []string{"proxy", "local bootstrap"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{DataPath: dir, ConfigPath: dir}
			if mode == "proxy" {
				cfg.ProxyAuthSecret = "synthetic-proxy-secret"
				cfg.ProxyAuthUserHeader = "X-Remote-User"
				cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
				cfg.ProxyAuthAdminRole = "admin"
			}
			router := NewRouter(cfg, nil, nil, nil, nil, "test")
			cleanupTestRouter(t, router)
			for _, path := range []string{
				"/api/agents/agent/report", "/api/agents/host/report",
				"/api/agents/docker/report", "/api/agents/kubernetes/report",
				"/api/agents/docker/commands/test/ack", "/api/agents/agent/host-1/config",
			} {
				method := http.MethodPost
				if strings.HasSuffix(path, "/config") {
					method = http.MethodGet
				}
				roles := []string{"viewer", "admin"}
				if mode == "local bootstrap" {
					roles = []string{"local"}
				}
				for _, role := range roles {
					req := newLoopbackRequest(method, path, strings.NewReader("{"))
					if mode == "proxy" {
						req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
						req.Header.Set("X-Remote-User", "proxy-user")
						req.Header.Set("X-Remote-Roles", role)
					}
					rec := httptest.NewRecorder()
					router.Handler().ServeHTTP(rec, req)
					if role == "viewer" && rec.Code != http.StatusForbidden {
						t.Errorf("%s proxy viewer status=%d, want 403", path, rec.Code)
					} else if role != "viewer" && (rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden) {
						t.Errorf("%s %s denied: status=%d %s", path, role, rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
}

func TestAgentIngestRoutesRespectOrganizationSessionRoles(t *testing.T) {
	router, _ := agentAuthorityRouter(t, false)
	org := &models.Organization{ID: "org-a", OwnerUserID: "tenant-owner", Members: []models.OrganizationMember{
		{UserID: "tenant-viewer", Role: models.OrgRoleViewer},
	}}
	for _, identity := range []struct {
		user    string
		allowed bool
	}{{"tenant-owner", true}, {"tenant-viewer", false}} {
		for _, path := range []string{"/api/agents/agent/report", "/api/agents/agent/host-1/config"} {
			method := http.MethodPost
			if strings.HasSuffix(path, "/config") {
				method = http.MethodGet
			}
			session := generateSessionToken()
			GetSessionStore().CreateSession(session, time.Hour, "browser", "127.0.0.1", identity.user)
			req := agentAuthorityRequest(t, method, path, "{", session, "")
			req = req.WithContext(context.WithValue(req.Context(), apicontext.OrgContextKey, org))
			rec := httptest.NewRecorder()
			// The tenant middleware normally installs this organization context.
			router.mux.ServeHTTP(rec, req)
			if identity.allowed && (rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden) {
				t.Errorf("%s %s denied: status=%d %s", identity.user, path, rec.Code, rec.Body.String())
			} else if !identity.allowed && rec.Code != http.StatusForbidden {
				t.Errorf("%s %s status=%d, want 403", identity.user, path, rec.Code)
			}
		}
	}
}
