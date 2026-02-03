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
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func TestHandleBackups_MethodNotAllowed(t *testing.T) {
	router := &Router{monitor: nil}
	req := httptest.NewRequest(http.MethodPost, "/api/backups", nil)
	rec := httptest.NewRecorder()

	router.handleBackups(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleBackups_Success(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.PVEBackups = models.PVEBackups{
		BackupTasks:    []models.BackupTask{{ID: "task-2"}},
		StorageBackups: []models.StorageBackup{{ID: "storage-1"}},
		GuestSnapshots: []models.GuestSnapshot{{ID: "snap-1"}},
	}
	state.PBSBackups = []models.PBSBackup{{ID: "pbs-1"}}
	state.PMGBackups = []models.PMGBackup{{ID: "pmg-1"}}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rec := httptest.NewRecorder()

	router.handleBackups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload struct {
		Backups     models.Backups         `json:"backups"`
		PVEBackups  models.PVEBackups      `json:"pveBackups"`
		PBSBackups  []models.PBSBackup     `json:"pbsBackups"`
		PMGBackups  []models.PMGBackup     `json:"pmgBackups"`
		BackupTasks []models.BackupTask    `json:"backupTasks"`
		Storage     []models.StorageBackup `json:"storageBackups"`
		GuestSnaps  []models.GuestSnapshot `json:"guestSnapshots"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Backups.PBS) != 1 || payload.Backups.PBS[0].ID != "pbs-1" {
		t.Fatalf("unexpected backups PBS data: %#v", payload.Backups.PBS)
	}
	if len(payload.PVEBackups.BackupTasks) != 1 || payload.PVEBackups.BackupTasks[0].ID != "task-2" {
		t.Fatalf("unexpected pveBackups data: %#v", payload.PVEBackups.BackupTasks)
	}
	if len(payload.BackupTasks) != 1 || payload.BackupTasks[0].ID != "task-2" {
		t.Fatalf("unexpected backupTasks data: %#v", payload.BackupTasks)
	}
	if len(payload.Storage) != 1 || payload.Storage[0].ID != "storage-1" {
		t.Fatalf("unexpected storageBackups data: %#v", payload.Storage)
	}
	if len(payload.GuestSnaps) != 1 || payload.GuestSnaps[0].ID != "snap-1" {
		t.Fatalf("unexpected guestSnapshots data: %#v", payload.GuestSnaps)
	}
	if len(payload.PBSBackups) != 1 || payload.PBSBackups[0].ID != "pbs-1" {
		t.Fatalf("unexpected pbsBackups data: %#v", payload.PBSBackups)
	}
	if len(payload.PMGBackups) != 1 || payload.PMGBackups[0].ID != "pmg-1" {
		t.Fatalf("unexpected pmgBackups data: %#v", payload.PMGBackups)
	}
}

func TestHandleBackupsPVE_Empty(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.PVEBackups = models.PVEBackups{}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/backups/pve", nil)
	rec := httptest.NewRecorder()

	router.handleBackupsPVE(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload struct {
		Backups []models.StorageBackup `json:"backups"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Backups == nil || len(payload.Backups) != 0 {
		t.Fatalf("expected empty backups array, got %#v", payload.Backups)
	}
}

func TestHandleBackupsPBS_Empty(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.PBSInstances = nil

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/backups/pbs", nil)
	rec := httptest.NewRecorder()

	router.handleBackupsPBS(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload struct {
		Instances []models.PBSInstance `json:"instances"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Instances == nil || len(payload.Instances) != 0 {
		t.Fatalf("expected empty instances array, got %#v", payload.Instances)
	}
}

func TestHandleSnapshots_Empty(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.PVEBackups = models.PVEBackups{}

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/snapshots", nil)
	rec := httptest.NewRecorder()

	router.handleSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload struct {
		Snapshots []models.GuestSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Snapshots == nil || len(payload.Snapshots) != 0 {
		t.Fatalf("expected empty snapshots array, got %#v", payload.Snapshots)
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
