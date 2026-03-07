package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func newHostAgentHandlers(t *testing.T, cfg *config.Config) (*HostAgentHandlers, *monitoring.Monitor) {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{DataPath: t.TempDir()}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	hub := websocket.NewHub(nil)
	handler := NewHostAgentHandlers(nil, monitor, hub)
	return handler, monitor
}

func monitorState(t *testing.T, monitor *monitoring.Monitor) *models.State {
	t.Helper()

	v := reflect.ValueOf(monitor).Elem().FieldByName("state")
	ptr := unsafe.Pointer(v.UnsafeAddr())
	return reflect.NewAt(v.Type(), ptr).Elem().Interface().(*models.State)
}

func seedHostAgent(t *testing.T, monitor *monitoring.Monitor) string {
	t.Helper()

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-1",
			Version: "1.0.0",
		},
		Host: agentshost.HostInfo{
			ID:       "machine-1",
			Hostname: "host-1.local",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.ID == "" {
		t.Fatalf("expected host ID to be set")
	}
	return host.ID
}

func TestHostAgentHandlers_HandleReport(t *testing.T) {
	handler, _ := newHostAgentHandlers(t, nil)

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-2",
			Version: "1.0.0",
		},
		Host: agentshost.HostInfo{
			ID:       "machine-2",
			Hostname: "host-2.local",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleReport_EnforcesMaxAgentsForNewHostsOnly(t *testing.T) {
	setMaxAgentsLicenseForTests(t, 1)

	handler, monitor := newHostAgentHandlers(t, nil)
	existingHostID := seedHostAgent(t, monitor)
	if existingHostID == "" {
		t.Fatalf("expected seeded host ID")
	}

	// Existing host should continue to report at the limit.
	existingReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-1",
			Version: "1.0.1",
		},
		Host: agentshost.HostInfo{
			ID:       "machine-1",
			Hostname: "host-1.local",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
	}
	existingBody, _ := json.Marshal(existingReport)
	existingReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(existingBody))
	existingRec := httptest.NewRecorder()
	handler.HandleReport(existingRec, existingReq)
	if existingRec.Code != http.StatusOK {
		t.Fatalf("existing host report should pass at limit, got %d: %s", existingRec.Code, existingRec.Body.String())
	}

	// New host should be blocked.
	newReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-2",
			Version: "1.0.0",
		},
		Host: agentshost.HostInfo{
			ID:       "machine-2",
			Hostname: "host-2.local",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
	}
	newBody, _ := json.Marshal(newReport)
	newReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(newBody))
	newRec := httptest.NewRecorder()
	handler.HandleReport(newRec, newReq)
	if newRec.Code != http.StatusPaymentRequired {
		t.Fatalf("new host should be blocked at limit, got %d: %s", newRec.Code, newRec.Body.String())
	}
}

func TestHostAgentHandlers_HandleDeleteHost(t *testing.T) {
	for _, prefix := range []string{"/api/agents/agent/", "/api/agents/host/"} {
		handler, monitor := newHostAgentHandlers(t, nil)
		hostID := seedHostAgent(t, monitor)

		req := httptest.NewRequest(http.MethodDelete, prefix+hostID, nil)
		rec := httptest.NewRecorder()

		handler.HandleDeleteHost(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 for %s: %s", rec.Code, prefix, rec.Body.String())
		}
	}
}

func TestHostAgentHandlers_HandleConfigPatch(t *testing.T) {
	for _, prefix := range []string{"/api/agents/agent/", "/api/agents/host/"} {
		handler, monitor := newHostAgentHandlers(t, nil)
		hostID := seedHostAgent(t, monitor)

		body := []byte(`{"commandsEnabled":true}`)
		req := httptest.NewRequest(http.MethodPatch, prefix+hostID+"/config", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.HandleConfig(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 for %s: %s", rec.Code, prefix, rec.Body.String())
		}
	}
}

func TestHostAgentHandlers_EnsureAgentTokenMatch(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	state := monitorState(t, monitor)
	state.UpsertHost(models.Host{
		ID:       "host-3",
		Hostname: "host-3.local",
		TokenID:  "token-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/host-3/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-2",
		Scopes: []string{config.ScopeAgentConfigRead},
	})
	rec := httptest.NewRecorder()

	ok := handler.ensureAgentTokenMatch(rec, req, "host-3")
	if ok {
		t.Fatalf("expected token mismatch")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHostAgentHandlers_HandleUninstall(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	body := []byte(`{"agentId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleUninstallRejectsTokenMismatch(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	state := monitorState(t, monitor)
	state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "host-token-mismatch.local",
		TokenID:  "token-1",
	})

	body := []byte(`{"agentId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(body))
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-2",
		Scopes: []string{config.ScopeAgentReport},
	})
	rec := httptest.NewRecorder()

	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleUninstall_ResponseBody(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	body := []byte(`{"agentId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool   `json:"success"`
		AgentID string `json:"agentId"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
	if resp.AgentID != hostID {
		t.Errorf("expected agentId=%q, got %q", hostID, resp.AgentID)
	}
	if resp.Message == "" {
		t.Errorf("expected non-empty message")
	}
}

func TestHostAgentHandlers_HandleUninstall_FullLifecycle(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	state := monitorState(t, monitor)

	// Seed a host via report
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-lifecycle",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:       "machine-lifecycle",
			Hostname: "lifecycle.local",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
	}
	token := &config.APITokenRecord{ID: "token-lifecycle", Name: "Lifecycle Token"}

	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	hostID := host.ID

	// Set connection health (as evaluateHostAgents would)
	state.SetConnectionHealth("host-"+hostID, true)

	// Verify host exists in state
	snap := state.GetSnapshot()
	found := false
	for _, h := range snap.Hosts {
		if h.ID == hostID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected host %q in state before uninstall", hostID)
	}
	if _, ok := snap.ConnectionHealth["host-"+hostID]; !ok {
		t.Fatalf("expected connection health entry before uninstall")
	}

	// Uninstall
	body := []byte(`{"agentId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/uninstall", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("uninstall status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	// Verify host is gone from state
	snap = state.GetSnapshot()
	for _, h := range snap.Hosts {
		if h.ID == hostID {
			t.Fatalf("host %q should not exist in state after uninstall", hostID)
		}
	}

	// Verify connection health is cleared
	if _, ok := snap.ConnectionHealth["host-"+hostID]; ok {
		t.Fatalf("connection health for %q should be cleared after uninstall", hostID)
	}
}

func TestHostAgentHandlers_HandleLinkUnlink(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	state := monitorState(t, monitor)
	state.UpdateNodes([]models.Node{{ID: "node-1", Name: "node-1"}})

	linkBody := []byte(`{"agentId":"` + hostID + `","nodeId":"node-1"}`)
	linkReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/link", bytes.NewReader(linkBody))
	linkRec := httptest.NewRecorder()

	handler.HandleLink(linkRec, linkReq)
	if linkRec.Code != http.StatusOK {
		t.Fatalf("link status = %d, want 200: %s", linkRec.Code, linkRec.Body.String())
	}

	unlinkBody := []byte(`{"agentId":"` + hostID + `"}`)
	unlinkReq := httptest.NewRequest(http.MethodPost, "/api/agents/agent/unlink", bytes.NewReader(unlinkBody))
	unlinkRec := httptest.NewRecorder()

	handler.HandleUnlink(unlinkRec, unlinkReq)
	if unlinkRec.Code != http.StatusOK {
		t.Fatalf("unlink status = %d, want 200: %s", unlinkRec.Code, unlinkRec.Body.String())
	}
}
