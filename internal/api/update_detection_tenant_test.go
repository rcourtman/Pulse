package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func dockerUpdateSnapshot(hostID string) models.StateSnapshot {
	return models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:          hostID,
			DisplayName: hostID,
			Containers: []models.DockerContainer{{
				ID:    hostID + "-c1",
				Name:  "/web",
				Image: "nginx:latest",
				UpdateStatus: &models.DockerContainerUpdateStatus{
					UpdateAvailable: true,
					LastChecked:     time.Now(),
				},
			}},
		}},
	}
}

func dockerUpdateReadState(hostID string) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(dockerUpdateSnapshot(hostID))
	return registry
}

func withOrg(req *http.Request, orgID string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
}

func TestTenantUpdateDetectionHandlersServeRequestOrganization(t *testing.T) {
	defaultMonitor, tenantMonitor := &monitoring.Monitor{}, &monitoring.Monitor{}
	defaultState, tenantState := dockerUpdateReadState("default-host"), dockerUpdateReadState("acme-host")
	handler := NewTenantUpdateDetectionHandlers(func(ctx context.Context) (*monitoring.Monitor, unifiedresources.ReadState) {
		switch GetOrgID(ctx) {
		case "", "default":
			return defaultMonitor, defaultState
		case "acme":
			return tenantMonitor, tenantState
		}
		return nil, nil
	})

	for orgID, wantHost := range map[string]string{"default": "default-host", "acme": "acme-host"} {
		rr := httptest.NewRecorder()
		handler.HandleGetInfraUpdates(rr, withOrg(httptest.NewRequest(http.MethodGet, "/api/infra-updates", nil), orgID))
		var resp infraUpdatesResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("org %s: decode: %v", orgID, err)
		}
		if resp.Total != 1 || resp.Updates[0].AgentID != wantHost {
			t.Fatalf("org %s: expected only %s updates, got %+v", orgID, wantHost, resp.Updates)
		}
	}

	// A known agent of another organization is not found from this one.
	rr := httptest.NewRecorder()
	handler.HandleGetInfraUpdateForResource(rr,
		withOrg(httptest.NewRequest(http.MethodGet, "/api/infra-updates/docker:default-host/default-host-c1", nil), "acme"),
		"docker:default-host/default-host-c1")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected default-org resource to be hidden from acme, got %d", rr.Code)
	}
}

func TestTenantUpdateDetectionHandlersFailClosedWithoutTenantMonitor(t *testing.T) {
	handler := NewTenantUpdateDetectionHandlers(func(context.Context) (*monitoring.Monitor, unifiedresources.ReadState) {
		return nil, nil
	})

	rr := httptest.NewRecorder()
	handler.HandleGetInfraUpdatesSummary(rr, withOrg(httptest.NewRequest(http.MethodGet, "/api/infra-updates/summary", nil), "acme"))
	var summary infraUpdatesSummaryResponse
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rr.Code != http.StatusOK || summary.TotalUpdates != 0 {
		t.Fatalf("expected empty summary, got %d %+v", rr.Code, summary)
	}

	rr = httptest.NewRecorder()
	handler.HandleTriggerInfraUpdateCheck(rr, withOrg(httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", strings.NewReader(`{"agentId":"default-host"}`)), "acme"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected no command to be queued without the tenant monitor, got %d", rr.Code)
	}
}

func TestRouterTenantMonitorAndReadStateNeverFallsBackToDefault(t *testing.T) {
	router := &Router{monitor: &monitoring.Monitor{}}

	if monitor, readState := router.tenantMonitorAndReadState(context.WithValue(context.Background(), OrgIDContextKey, "acme")); monitor != nil || readState != nil {
		t.Fatalf("expected no monitor or read state for an unresolved tenant, got %v %v", monitor, readState)
	}
	if monitor, _ := router.tenantMonitorAndReadState(context.WithValue(context.Background(), OrgIDContextKey, "default")); monitor != router.monitor {
		t.Fatalf("expected the default monitor for the default organization")
	}

	rr := httptest.NewRecorder()
	router.handleSchedulerHealth(rr, withOrg(httptest.NewRequest(http.MethodGet, "/api/monitoring/scheduler/health", nil), "acme"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected tenant scheduler health not to report the default monitor, got %d", rr.Code)
	}
}

func monitorWithDockerUpdate(t *testing.T, hostID string) *monitoring.Monitor {
	t.Helper()
	monitor, _, _ := newTestMonitor(t)
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(dockerUpdateSnapshot(hostID))
	setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))
	return monitor
}

// The routed check guards the router wiring itself: each organization's
// infra-update reads come from its own monitor, through the real middleware.
func TestRoutedInfraUpdatesServeRequestOrganization(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	const rawToken = "infra-updates-tenant-token-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "two-org-token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("create api token record: %v", err)
	}
	record.OrgIDs = []string{"default", "org-a"}

	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, APITokens: []config.APITokenRecord{*record}}
	cfg.SortAPITokens()
	if err := config.NewMultiTenantPersistence(dataDir).SaveOrganization(&models.Organization{ID: "org-a", DisplayName: "ORG-A"}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)
	router.monitor = monitorWithDockerUpdate(t, "default-host")
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"org-a": monitorWithDockerUpdate(t, "org-a-host")})
	router.mtMonitor = mtm
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)

	for orgID, wantHost := range map[string]string{"default": "default-host", "org-a": "org-a-host"} {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/infra-updates", nil)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}
		req.Header.Set("X-Pulse-Org-ID", orgID)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("org %s: request failed: %v", orgID, err)
		}
		var resp infraUpdatesResponse
		decodeErr := json.NewDecoder(res.Body).Decode(&resp)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || decodeErr != nil {
			t.Fatalf("org %s: status %d, decode %v", orgID, res.StatusCode, decodeErr)
		}
		if resp.Total != 1 || resp.Updates[0].AgentID != wantHost {
			t.Fatalf("org %s: expected only %s updates, got %+v", orgID, wantHost, resp.Updates)
		}
	}
}
