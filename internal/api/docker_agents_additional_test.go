package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func newDockerAgentHandlers(t *testing.T, cfg *config.Config) (*DockerAgentHandlers, *monitoring.Monitor) {
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
	handler := NewDockerAgentHandlers(nil, monitor, hub, cfg)
	return handler, monitor
}

func seedDockerHost(t *testing.T, monitor *monitoring.Monitor) string {
	t.Helper()

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-1",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-host",
			Name:             "Docker Host",
			MachineID:        "machine-1",
			DockerVersion:    "26.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 8 << 30,
			UptimeSeconds:    120,
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if host.ID == "" {
		t.Fatalf("expected host ID to be set")
	}
	return host.ID
}

func TestDockerAgentHandlers_HandleReport(t *testing.T) {
	handler, _ := newDockerAgentHandlers(t, nil)

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-2",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-host-2",
			Name:             "Docker Host 2",
			MachineID:        "machine-2",
			DockerVersion:    "26.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 4 << 30,
			UptimeSeconds:    60,
		},
		Timestamp: time.Now().UTC(),
	}
	body, _ := json.Marshal(report)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("success = %v, want true", resp["success"])
	}
	if resp["hostId"] == "" {
		t.Fatalf("expected hostId in response")
	}
}

func TestDockerAgentHandlers_HandleCommandAck(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	cmdStatus, err := monitor.QueueDockerHostStop(hostID)
	if err != nil {
		t.Fatalf("QueueDockerHostStop: %v", err)
	}

	reqBody := map[string]string{
		"hostId": hostID,
		"status": "completed",
	}
	body, _ := json.Marshal(reqBody)
	path := "/api/agents/docker/commands/" + cmdStatus.ID + "/ack"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleCommandAck(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleDockerHostActions(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/hosts/"+hostID+"/allow-reenroll", nil)
	rec := httptest.NewRecorder()

	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleDeleteHost(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/docker/hosts/"+hostID+"?force=true", nil)
	rec := httptest.NewRecorder()

	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleUnhideHost(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/unhide", nil)
	rec := httptest.NewRecorder()

	handler.HandleUnhideHost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleMarkPendingUninstall(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/pending-uninstall", nil)
	rec := httptest.NewRecorder()

	handler.HandleMarkPendingUninstall(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleSetCustomDisplayName(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	body := []byte(`{"displayName":"My Docker Host"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/display-name", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleSetCustomDisplayName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleContainerUpdate(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, &config.Config{DataPath: t.TempDir()})
	hostID := seedDockerHost(t, monitor)

	reqBody := map[string]string{
		"hostId":        hostID,
		"containerId":   "container-1",
		"containerName": "nginx",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/containers/container-1/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleContainerUpdate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDockerAgentHandlers_HandleContainerUpdate_Disabled(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), DisableDockerUpdateActions: true}
	handler, monitor := newDockerAgentHandlers(t, cfg)
	hostID := seedDockerHost(t, monitor)

	reqBody := map[string]string{
		"hostId":        hostID,
		"containerId":   "container-2",
		"containerName": "redis",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/containers/container-2/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleContainerUpdate(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestDockerAgentHandlers_HandleCheckUpdates(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/hosts/"+hostID+"/check-updates", nil)
	rec := httptest.NewRecorder()

	handler.HandleCheckUpdates(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Check for updates") {
		t.Fatalf("expected check updates message")
	}
}
