package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleVerifyTemperatureSSH(t *testing.T) {
	tempDir := t.TempDir()
	// Set HOME to tempDir so keys are generated there
	t.Setenv("HOME", tempDir)
	t.Setenv("PULSE_DATA_DIR", tempDir)
	// Allow key generation even if environment looks like container
	t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "true")

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		SSHPort:    2222, // Use non-standard port to avoid accidental connections
	}

	handler := newTestConfigHandlers(t, cfg)

	t.Run("invalid_method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/verify-ssh", nil)
		rec := httptest.NewRecorder()
		handler.HandleVerifyTemperatureSSH(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", rec.Code)
		}
	})

	t.Run("invalid_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/verify-ssh", bytes.NewReader([]byte("not json")))
		rec := httptest.NewRecorder()
		handler.HandleVerifyTemperatureSSH(rec, req)
		// It returns 200 OK with error message in body
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("Unable to parse")) {
			t.Error("expected error message in body")
		}
	})

	t.Run("empty_node_list", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"nodes": "", // Empty string
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/verify-ssh", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleVerifyTemperatureSSH(rec, req)
		if !bytes.Contains(rec.Body.Bytes(), []byte("No nodes to verify")) {
			t.Error("expected 'No nodes to verify' message")
		}
	})

	t.Run("connection_failure", func(t *testing.T) {
		// Ensure .ssh dir exists so key generation works (it might create it, but safe to ensure)
		os.MkdirAll(tempDir+"/.ssh", 0700)

		reqBody := map[string]interface{}{
			"nodes": "invalid-host-name-for-test",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/verify-ssh", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		// This will attempt to connect to "invalid-host-name-for-test" on port 2222
		handler.HandleVerifyTemperatureSSH(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}

		// respStr := rec.Body.String() // Unused
		if !bytes.Contains(rec.Body.Bytes(), []byte("Nodes pending configuration")) {
			t.Error("expected failure message")
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("invalid-host-name-for-test")) {
			t.Error("expected affected node in failure list")
		}
	})
}
