package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleTemperatureProxyInstallCommand(t *testing.T) {
	cfg := &config.Config{PublicURL: "https://pulse.example:7655"}
	router := &Router{config: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/temperature-proxy/install-command?node=pve-a", nil)
	rec := httptest.NewRecorder()

	router.handleTemperatureProxyInstallCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["node"] != "pve-a" {
		t.Fatalf("expected node pve-a, got %s", resp["node"])
	}

	command := resp["command"]
	if command == "" {
		t.Fatalf("command missing in response")
	}
	if !strings.Contains(command, cfg.PublicURL) {
		t.Fatalf("command does not include pulse URL: %s", command)
	}
	if !strings.Contains(command, "--ctid") {
		t.Fatalf("command missing expected --ctid flag: %s", command)
	}
	if !strings.Contains(command, "--pulse-server") {
		t.Fatalf("command missing expected --pulse-server flag: %s", command)
	}
}
