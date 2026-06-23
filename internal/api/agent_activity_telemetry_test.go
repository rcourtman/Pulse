package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRecordExternalAgentCapabilityActivityUsesCapabilityScope(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}

	readOnlyReq := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	attachAPITokenRecord(readOnlyReq, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	router.recordExternalAgentCapabilityActivity(readOnlyReq, agentcapabilities.FleetContextCapabilityName)

	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("read-only fleet context should record external-agent activity, got %d: %+v", len(history.Events), history.Events)
	}
	if history.Events[0].Surface != config.ExternalAgentActivitySurfaceAgentAPI ||
		history.Events[0].Activity != config.ExternalAgentActivityFleetContext {
		t.Fatalf("unexpected read-only external-agent activity event: %+v", history.Events[0])
	}

	router.recordExternalAgentCapabilityActivity(readOnlyReq, agentcapabilities.PlanActionCapabilityName)

	history, err = persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("read-only token should not record action capability activity, got %d: %+v", len(history.Events), history.Events)
	}

	actionReq := httptest.NewRequest(http.MethodPost, "/api/actions/plan", nil)
	attachAPITokenRecord(actionReq, &config.APITokenRecord{Scopes: []string{config.ScopeAIExecute}})
	router.recordExternalAgentCapabilityActivity(actionReq, agentcapabilities.PlanActionCapabilityName)

	history, err = persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected two external-agent activity events, got %d: %+v", len(history.Events), history.Events)
	}
	if history.Events[1].Activity != config.ExternalAgentActivityActionPlan {
		t.Fatalf("action capability activity = %q, want %q", history.Events[1].Activity, config.ExternalAgentActivityActionPlan)
	}
}

func TestRecordExternalAgentCapabilityActivityPreservesMCPAdapterSurface(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}

	mcpReq := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	mcpReq.Header.Set(agentcapabilities.AgentSurfaceHeader, agentcapabilities.AgentSurfacePulseMCP)
	attachAPITokenRecord(mcpReq, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	router.recordExternalAgentCapabilityActivity(mcpReq, agentcapabilities.FleetContextCapabilityName)

	plainReq := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	plainReq.Header.Set(agentcapabilities.AgentSurfaceHeader, "unknown-client")
	attachAPITokenRecord(plainReq, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	router.recordExternalAgentCapabilityActivity(plainReq, agentcapabilities.FleetContextCapabilityName)

	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected two external-agent activity events, got %d: %+v", len(history.Events), history.Events)
	}
	if history.Events[0].Surface != config.ExternalAgentActivitySurfacePulseMCP {
		t.Fatalf("MCP adapter event surface = %q, want %q", history.Events[0].Surface, config.ExternalAgentActivitySurfacePulseMCP)
	}
	if history.Events[1].Surface != config.ExternalAgentActivitySurfaceAgentAPI {
		t.Fatalf("unknown adapter event surface = %q, want %q", history.Events[1].Surface, config.ExternalAgentActivitySurfaceAgentAPI)
	}
}

func TestHandleAgentWorkflowPromptActivityRecordsMCPAdapterSurface(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}

	body := `{"name":"` + agentcapabilities.PulseWorkflowPromptOperationsLoop + `"}`
	req := httptest.NewRequest(http.MethodPost, agentcapabilities.AgentWorkflowPromptActivityPath, strings.NewReader(body))
	req.Header.Set(agentcapabilities.AgentSurfaceHeader, agentcapabilities.AgentSurfacePulseMCP)
	attachAPITokenRecord(req, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	rec := httptest.NewRecorder()

	router.HandleAgentWorkflowPromptActivity(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("HandleAgentWorkflowPromptActivity status = %d body=%q, want 204", rec.Code, rec.Body.String())
	}
	history, err := persistence.LoadWorkflowPromptActivityHistory()
	if err != nil {
		t.Fatalf("LoadWorkflowPromptActivityHistory: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("expected one workflow prompt activity event, got %d: %+v", len(history.Events), history.Events)
	}
	if history.Events[0].Surface != config.WorkflowPromptActivitySurfacePulseMCP ||
		history.Events[0].PromptName != agentcapabilities.PulseWorkflowPromptOperationsLoop {
		t.Fatalf("unexpected workflow prompt activity event: %+v", history.Events[0])
	}
}

func TestHandleAgentWorkflowPromptActivityRejectsUnknownPrompt(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}

	req := httptest.NewRequest(http.MethodPost, agentcapabilities.AgentWorkflowPromptActivityPath, strings.NewReader(`{"name":"unknown"}`))
	attachAPITokenRecord(req, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	rec := httptest.NewRecorder()

	router.HandleAgentWorkflowPromptActivity(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown prompt status = %d, want 400", rec.Code)
	}
	history, err := persistence.LoadWorkflowPromptActivityHistory()
	if err != nil {
		t.Fatalf("LoadWorkflowPromptActivityHistory: %v", err)
	}
	if len(history.Events) != 0 {
		t.Fatalf("unknown prompt should not record activity, got %+v", history.Events)
	}
}

func TestAPITokenCoversExternalAgentSurfaceRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	token := &config.APITokenRecord{
		Scopes:    []string{config.ScopeWildcard},
		ExpiresAt: &expiredAt,
	}

	if apiTokenCoversExternalAgentSurface(token, now, config.ScopeMonitoringRead) {
		t.Fatal("expired token should not cover the external-agent surface")
	}
}

func TestRecordExternalAgentCapabilityActivityKeepsProvisioningScopesRouteSpecific(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}

	readReq := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
	attachAPITokenRecord(readReq, &config.APITokenRecord{Scopes: []string{config.ScopeSettingsRead}})
	router.recordExternalAgentCapabilityActivity(readReq, agentcapabilities.ListNodesCapabilityName)
	router.recordExternalAgentCapabilityActivity(readReq, agentcapabilities.AddNodeCapabilityName)

	writeReq := httptest.NewRequest(http.MethodPost, "/api/config/nodes", nil)
	attachAPITokenRecord(writeReq, &config.APITokenRecord{Scopes: []string{config.ScopeSettingsWrite}})
	router.recordExternalAgentCapabilityActivity(writeReq, agentcapabilities.AddNodeCapabilityName)

	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected read list_nodes and write add_node activity only, got %d: %+v", len(history.Events), history.Events)
	}
	for _, event := range history.Events {
		if event.Activity != config.ExternalAgentActivityProvisioning {
			t.Fatalf("provisioning activity = %q, want %q", event.Activity, config.ExternalAgentActivityProvisioning)
		}
	}
}

func TestExternalAgentActivityForCanonicalPulseMCPSurfaceTools(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	capabilities := agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP)
	if len(capabilities) == 0 {
		t.Fatal("Pulse MCP surface must publish request/response tools")
	}
	for _, capability := range capabilities {
		activity, ok := externalAgentActivityForCapability(capability.Name)
		if !ok {
			t.Fatalf("MCP-published capability %q has no external-agent activity class", capability.Name)
		}
		if activity == "" {
			t.Fatalf("MCP-published capability %q maps to an empty external-agent activity class", capability.Name)
		}
	}
	if activity, ok := externalAgentActivityForCapability(agentcapabilities.EventSubscriptionCapabilityName); !ok || activity != config.ExternalAgentActivityEventStream {
		t.Fatalf("%s activity = %q, %v; want %s", agentcapabilities.EventSubscriptionCapabilityName, activity, ok, config.ExternalAgentActivityEventStream)
	}
}

func TestRecordExternalAgentCapabilityActivityUsesCapabilityClass(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{persistence: persistence}
	req := httptest.NewRequest(http.MethodPut, "/api/resources/vm:101/operator-state", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{Scopes: []string{config.ScopeWildcard}})

	router.recordExternalAgentCapabilityActivity(req, agentcapabilities.SetOperatorStateCapabilityName)
	router.recordExternalAgentCapabilityActivity(req, "unknown_capability")

	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(history.Events) != 1 {
		t.Fatalf("expected one external-agent activity event, got %d: %+v", len(history.Events), history.Events)
	}
	if history.Events[0].Activity != config.ExternalAgentActivityOperatorState {
		t.Fatalf("operator-state capability activity = %q, want %q", history.Events[0].Activity, config.ExternalAgentActivityOperatorState)
	}
}
