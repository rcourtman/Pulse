package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleExportConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{Name: "pve1", Host: "https://pve1.local:8006"},
		},
	}

	handler := newTestConfigHandlers(t, cfg)

	// Ensure persistence directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Save initial config so export has something to read
	if err := handler.persistence.SaveNodesConfig(cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}
	// Also save empty settings to avoid nil pointer issues during export
	if err := handler.persistence.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatalf("Failed to save system settings: %v", err)
	}

	tests := []struct {
		name           string
		reqBody        ExportConfigRequest
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:           "success_valid_passphrase",
			reqBody:        ExportConfigRequest{Passphrase: "securepassword123"},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["status"] != "success" {
					t.Errorf("expected status success, got %v", resp["status"])
				}
				if resp["data"] == "" {
					t.Error("expected data to be present")
				}
			},
		},
		{
			name:           "fail_missing_passphrase",
			reqBody:        ExportConfigRequest{Passphrase: ""},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  nil,
		},
		{
			name:           "fail_short_passphrase",
			reqBody:        ExportConfigRequest{Passphrase: "short"},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/config/export", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.HandleExportConfig(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestHandleImportConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)
	// Mock reload function to avoid actual reload logic failure
	handler.reloadFunc = func() error { return nil }
	// Force usage of the persistence instance we prepared (dummyPersistence)
	// This helps isolate if the issue is with handler logic vs persistence initialization
	// But we need to do it AFTER dummyPersistence is created.
	// So we will do it further down.

	// Ensure persistence directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a valid export to verify import later
	// Save dummy config first
	dummyCfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve1",
				Host:       "https://10.0.0.1:8006",
				User:       "root@pam",
				Password:   "dummy-password",
				TokenName:  "test-token",
				TokenValue: "test-secret",
				VerifySSL:  false,
			},
		},
	}
	dummyPersistence := config.NewConfigPersistence(tempDir)
	handler.persistence = dummyPersistence // Override handler's persistence
	if err := dummyPersistence.SaveNodesConfig(dummyCfg.PVEInstances, dummyCfg.PBSInstances, dummyCfg.PMGInstances); err != nil {
		t.Fatalf("Failed to save dummy config: %v", err)
	}
	// Also save system settings to ensure defaults are present
	if err := dummyPersistence.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatalf("Failed to save dummy system settings: %v", err)
	}

	passphrase := "secure-passphrase"
	encryptedData, err := dummyPersistence.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("Failed to create export data for test: %v", err)
	}

	// Create a valid config.json so Load() works during import
	configJSONPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configJSONPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}

	tests := []struct {
		name           string
		reqBody        ImportConfigRequest
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success_valid_import",
			reqBody: ImportConfigRequest{
				Data:       encryptedData,
				Passphrase: passphrase,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["status"] != "success" {
					t.Errorf("expected status success, got %v", resp["status"])
				}
			},
		},
		{
			name: "fail_wrong_passphrase",
			reqBody: ImportConfigRequest{
				Data:       encryptedData,
				Passphrase: "wrongpassword123",
			},
			expectedStatus: http.StatusBadRequest, // Usually returns bad request on decryption fail
			checkResponse:  nil,
		},
		{
			name: "fail_missing_passphrase",
			reqBody: ImportConfigRequest{
				Data:       encryptedData,
				Passphrase: "",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  nil,
		},
		{
			name: "fail_missing_data",
			reqBody: ImportConfigRequest{
				Data:       "",
				Passphrase: passphrase,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			handler.HandleImportConfig(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}
