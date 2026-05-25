package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleListPBSBackupsReturnsArtifacts(t *testing.T) {
	state := models.NewState()
	backupTime := time.Date(2026, 5, 25, 1, 34, 25, 0, time.UTC)
	state.UpdatePBSBackups("pbs-main", []models.PBSBackup{
		{
			ID:         "pbs-main/main/minipc/ct/112/2026-05-25T01:34:25Z",
			Instance:   "pbs-main",
			Datastore:  "main",
			Namespace:  "minipc",
			BackupType: "ct",
			VMID:       "112",
			BackupTime: backupTime,
			Size:       8_589_934_592,
			Protected:  true,
			Verified:   true,
			Files:      []string{"index.json.blob", "root.pxar.didx"},
			Owner:      "backup@pbs",
		},
		{
			ID:         "pbs-main/main/delly/vm/101/2026-05-24T01:00:00Z",
			Instance:   "pbs-main",
			Datastore:  "main",
			Namespace:  "delly",
			BackupType: "vm",
			VMID:       "101",
			BackupTime: backupTime.Add(-24 * time.Hour),
			Size:       2_147_483_648,
			Verified:   true,
			Files:      []string{"index.json.blob", "drive-scsi0.img.fidx"},
		},
	})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/backups/pbs?namespace=minipc", nil)
	rec := httptest.NewRecorder()

	router.handleListPBSBackups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response PBSBackupsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Meta.TotalBackups != 1 {
		t.Fatalf("expected one filtered backup, got %d", response.Meta.TotalBackups)
	}
	if len(response.Data.Backups) != 1 {
		t.Fatalf("expected one backup row, got %d", len(response.Data.Backups))
	}

	got := response.Data.Backups[0]
	if got.Namespace != "minipc" || got.VMID != "112" {
		t.Fatalf("unexpected backup row: %+v", got)
	}
	if got.Size != 8_589_934_592 || !got.Protected || !got.Verified {
		t.Fatalf("PBS artifact fields were not preserved: %+v", got)
	}
	if len(got.Files) != 2 {
		t.Fatalf("expected normalized file list, got %#v", got.Files)
	}
}

func TestHandleListPBSBackupsInitializesEmptyCollections(t *testing.T) {
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", models.NewState())

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/backups/pbs", nil)
	rec := httptest.NewRecorder()

	router.handleListPBSBackups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response PBSBackupsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Backups == nil {
		t.Fatalf("expected empty PBS backups list to be encoded as []")
	}
	if response.Meta.TotalBackups != 0 {
		t.Fatalf("expected zero backups, got %d", response.Meta.TotalBackups)
	}
}
