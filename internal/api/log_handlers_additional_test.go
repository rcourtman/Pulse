package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
)

type noFlushWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *noFlushWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *noFlushWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func TestLogHandlers_HandleStreamLogs_RequiresFlusher(t *testing.T) {
	handler := NewLogHandlers(&config.Config{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/logs/stream", nil)
	w := &noFlushWriter{}

	handler.HandleStreamLogs(w, req)

	if w.code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.code)
	}
}

func TestLogHandlers_HandleStreamLogs_SendsEvents(t *testing.T) {
	handler := NewLogHandlers(&config.Config{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/logs/stream", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleStreamLogs(rr, req)
		close(done)
	}()

	_, _ = logging.GetBroadcaster().Write([]byte("test-log-line"))
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stream handler to finish")
	}

	body := rr.Body.String()
	if !strings.Contains(body, "data:") {
		t.Fatalf("expected SSE data in response, got %q", body)
	}
	if !strings.Contains(body, "test-log-line") {
		t.Fatalf("expected log line in response, got %q", body)
	}
}

func TestLogHandlers_HandleDownloadBundle(t *testing.T) {
	logFile := filepathWithTemp(t, "pulse.log")
	if err := os.WriteFile(logFile, []byte("log-line"), 0600); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	t.Setenv("TEST_SECRET", "super-secret")

	cfg := &config.Config{
		LogFile:                 logFile,
		AuthPass:                "password",
		APIToken:                "token",
		ProxyAuthSecret:         "proxy-secret",
		PVEInstances:            []config.PVEInstance{{Password: "pve-pass", TokenValue: "pve-token"}},
		PBSInstances:            []config.PBSInstance{{Password: "pbs-pass", TokenValue: "pbs-token"}},
		PMGInstances:            []config.PMGInstance{{Password: "pmg-pass", TokenValue: "pmg-token"}},
		SuppressedEnvMigrations: []string{"hash"},
	}

	handler := NewLogHandlers(cfg, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs/bundle", nil)
	handler.HandleDownloadBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}

	var haveLog, haveSystem bool
	var systemInfo []byte
	for _, f := range zr.File {
		switch f.Name {
		case "pulse.log", "pulse-tail.log":
			haveLog = true
		case "system-info.json":
			haveSystem = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open system-info.json: %v", err)
			}
			defer rc.Close()
			systemInfo, _ = io.ReadAll(rc)
		}
	}
	if !haveLog {
		t.Fatalf("expected log file in bundle")
	}
	if !haveSystem {
		t.Fatalf("expected system-info.json in bundle")
	}

	var payload struct {
		Config config.Config `json:"Config"`
		Env    []string      `json:"Env"`
	}
	if err := json.Unmarshal(systemInfo, &payload); err != nil {
		t.Fatalf("unmarshal system-info: %v", err)
	}

	if payload.Config.AuthPass != "[REDACTED]" {
		t.Fatalf("expected AuthPass redacted, got %q", payload.Config.AuthPass)
	}
	if payload.Config.APIToken != "[REDACTED]" {
		t.Fatalf("expected APIToken redacted, got %q", payload.Config.APIToken)
	}
	if payload.Config.ProxyAuthSecret != "[REDACTED]" {
		t.Fatalf("expected ProxyAuthSecret redacted, got %q", payload.Config.ProxyAuthSecret)
	}
	if payload.Config.PVEInstances[0].Password != "[REDACTED]" {
		t.Fatalf("expected PVE password redacted, got %q", payload.Config.PVEInstances[0].Password)
	}

	var redactedEnv bool
	for _, env := range payload.Env {
		if strings.HasPrefix(env, "TEST_SECRET=") {
			redactedEnv = strings.Contains(env, "[REDACTED]")
			break
		}
	}
	if !redactedEnv {
		t.Fatalf("expected TEST_SECRET to be redacted, got env: %#v", payload.Env)
	}
}

func TestLogHandlers_HandleSetLevelAndGetLevel(t *testing.T) {
	originalLevel := logging.GetGlobalLevel()
	t.Cleanup(func() {
		logging.SetGlobalLevel(originalLevel)
	})

	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveSystemSettings(config.SystemSettings{LogLevel: "info"}); err != nil {
		t.Fatalf("save system settings: %v", err)
	}
	handler := NewLogHandlers(cfg, persistence)

	req := httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	handler.HandleSetLevel(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader(`{"level":"invalid"}`))
	rr = httptest.NewRecorder()
	handler.HandleSetLevel(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader(`{"level":"debug"}`))
	rr = httptest.NewRecorder()
	handler.HandleSetLevel(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected config log level updated, got %q", cfg.LogLevel)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/logs/level", nil)
	rr = httptest.NewRecorder()
	handler.HandleGetLevel(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "debug") {
		t.Fatalf("expected response to include debug level, got %q", rr.Body.String())
	}
}

func filepathWithTemp(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
