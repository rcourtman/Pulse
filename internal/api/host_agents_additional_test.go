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

	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleDeleteHost(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/host/"+hostID, nil)
	rec := httptest.NewRecorder()

	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleConfigPatch(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	body := []byte(`{"commandsEnabled":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/host/"+hostID+"/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_EnsureHostTokenMatch(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	state := monitorState(t, monitor)
	state.UpsertHost(models.Host{
		ID:       "host-3",
		Hostname: "host-3.local",
		TokenID:  "token-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/host/host-3/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-2",
		Scopes: []string{config.ScopeHostConfigRead},
	})
	rec := httptest.NewRecorder()

	ok := handler.ensureHostTokenMatch(rec, req, "host-3")
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

	body := []byte(`{"hostId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/uninstall", bytes.NewReader(body))
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

	body := []byte(`{"hostId":"` + hostID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/host/uninstall", bytes.NewReader(body))
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "token-2",
		Scopes: []string{config.ScopeHostReport},
	})
	rec := httptest.NewRecorder()

	handler.HandleUninstall(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestHostAgentHandlers_HandleLinkUnlink(t *testing.T) {
	handler, monitor := newHostAgentHandlers(t, nil)
	hostID := seedHostAgent(t, monitor)

	state := monitorState(t, monitor)
	state.UpdateNodes([]models.Node{{ID: "node-1", Name: "node-1"}})

	linkBody := []byte(`{"hostId":"` + hostID + `","nodeId":"node-1"}`)
	linkReq := httptest.NewRequest(http.MethodPost, "/api/agents/host/link", bytes.NewReader(linkBody))
	linkRec := httptest.NewRecorder()

	handler.HandleLink(linkRec, linkReq)
	if linkRec.Code != http.StatusOK {
		t.Fatalf("link status = %d, want 200: %s", linkRec.Code, linkRec.Body.String())
	}

	unlinkBody := []byte(`{"hostId":"` + hostID + `"}`)
	unlinkReq := httptest.NewRequest(http.MethodPost, "/api/agents/host/unlink", bytes.NewReader(unlinkBody))
	unlinkRec := httptest.NewRecorder()

	handler.HandleUnlink(unlinkRec, unlinkReq)
	if unlinkRec.Code != http.StatusOK {
		t.Fatalf("unlink status = %d, want 200: %s", unlinkRec.Code, unlinkRec.Body.String())
	}
}
