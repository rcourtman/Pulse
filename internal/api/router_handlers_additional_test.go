package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleConfig_MethodNotAllowed(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/config", nil)
	rec := httptest.NewRecorder()

	router.handleConfig(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleConfig_Success(t *testing.T) {
	router := &Router{config: &config.Config{AutoUpdateEnabled: true, UpdateChannel: "beta"}}
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	router.handleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["csrfProtection"] != false {
		t.Fatalf("expected csrfProtection=false, got %#v", payload["csrfProtection"])
	}
	if payload["autoUpdateEnabled"] != true {
		t.Fatalf("expected autoUpdateEnabled=true, got %#v", payload["autoUpdateEnabled"])
	}
	if payload["updateChannel"] != "beta" {
		t.Fatalf("expected updateChannel=beta, got %#v", payload["updateChannel"])
	}
}

func TestHandleSimpleStats(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	rec := httptest.NewRecorder()

	router.handleSimpleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Simple Pulse Stats") {
		t.Fatalf("expected stats page HTML, got %q", rec.Body.String())
	}
}

func TestHandleSocketIO_RedirectsForJS(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/socket.io/socket.io.js", nil)
	rec := httptest.NewRecorder()

	router.handleSocketIO(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "https://cdn.socket.io/4.8.1/socket.io.min.js" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleSocketIO_PollingHandshake(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling", nil)
	rec := httptest.NewRecorder()

	router.handleSocketIO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=UTF-8" {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "0{") || !strings.Contains(body, "\"sid\"") {
		t.Fatalf("unexpected polling handshake body: %q", body)
	}
}

func TestHandleSocketIO_PollingConnected(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling&sid=abc", nil)
	rec := httptest.NewRecorder()

	router.handleSocketIO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if body := rec.Body.String(); body != "6" {
		t.Fatalf("unexpected polling body: %q", body)
	}
}

func TestHandleSocketIO_DefaultRedirect(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/socket.io/?foo=bar", nil)
	rec := httptest.NewRecorder()

	router.handleSocketIO(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/ws" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleDownloadInstallScript_Fallback(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "scripts", "install-docker-agent.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	router := &Router{projectRoot: root}
	req := httptest.NewRequest(http.MethodGet, "/download/install-docker-agent.sh", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadInstallScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-cache") {
		t.Fatalf("expected no-cache header, got %q", cache)
	}
}

func TestHandleDownloadHostAgentInstallScript_Fallback(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "scripts", "install.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho host\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	router := &Router{projectRoot: root}
	req := httptest.NewRequest(http.MethodGet, "/download/install.sh", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadHostAgentInstallScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-cache") {
		t.Fatalf("expected no-cache header, got %q", cache)
	}
}

func TestHandleDownloadAgent_Found(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PULSE_BIN_DIR", binDir)
	payload := []byte("docker-agent-binary")
	filePath := filepath.Join(binDir, "pulse-docker-agent-linux-arm64")
	if err := os.WriteFile(filePath, payload, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/download/pulse-docker-agent?arch=arm64", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadAgent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	expected := fmt.Sprintf("%x", sha256.Sum256(payload))
	if checksum := rec.Header().Get("X-Checksum-Sha256"); checksum != expected {
		t.Fatalf("unexpected checksum header: %q", checksum)
	}
	if rec.Body.String() != string(payload) {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleDownloadAgent_NotFound(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PULSE_BIN_DIR", binDir)

	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/download/pulse-docker-agent", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadAgent(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDownloadScript_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "container-install", handler: router.handleDownloadContainerAgentInstallScript},
		{name: "host-install-ps", handler: router.handleDownloadHostAgentInstallScriptPS},
		{name: "host-uninstall", handler: router.handleDownloadHostAgentUninstallScript},
		{name: "host-uninstall-ps", handler: router.handleDownloadHostAgentUninstallScriptPS},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/download", nil)
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
			}
		})
	}
}
