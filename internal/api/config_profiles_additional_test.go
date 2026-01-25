package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newConfigProfileHandler(t *testing.T) (*ConfigProfileHandler, *config.ConfigPersistence) {
	t.Helper()
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence: %v", err)
	}
	handler := NewConfigProfileHandler(mtp)
	return handler, persistence
}

func createProfile(t *testing.T, handler *ConfigProfileHandler, name string, cfg models.AgentConfigMap) models.AgentProfile {
	t.Helper()
	profile := models.AgentProfile{
		Name:   name,
		Config: cfg,
	}
	body, _ := json.Marshal(profile)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), "username", "tester"))
	rec := httptest.NewRecorder()

	handler.CreateProfile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("CreateProfile status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var created models.AgentProfile
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return created
}

func TestConfigProfileHandler_GetProfile(t *testing.T) {
	handler, _ := newConfigProfileHandler(t)
	created := createProfile(t, handler, "Profile One", models.AgentConfigMap{"interval": "10s"})

	req := httptest.NewRequest(http.MethodGet, "/"+created.ID, nil)
	rec := httptest.NewRecorder()

	handler.GetProfile(rec, req, created.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got models.AgentProfile
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("profile ID = %s, want %s", got.ID, created.ID)
	}
}

func TestConfigProfileHandler_GetChangeLog_Filtered(t *testing.T) {
	handler, persistence := newConfigProfileHandler(t)
	created := createProfile(t, handler, "Profile Log", models.AgentConfigMap{"log_level": "debug"})

	change := models.ProfileChangeLog{
		ID:          "log-1",
		ProfileID:   created.ID,
		ProfileName: created.Name,
		Action:      "create",
		NewVersion:  1,
		User:        "tester",
	}
	if err := persistence.AppendProfileChangeLog(change); err != nil {
		t.Fatalf("AppendProfileChangeLog: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/changelog?profile_id="+created.ID, nil)
	rec := httptest.NewRecorder()

	handler.GetChangeLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var logs []models.ProfileChangeLog
	if err := json.NewDecoder(rec.Body).Decode(&logs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected change log entries")
	}
	if logs[0].ProfileID != created.ID {
		t.Fatalf("profile_id = %s, want %s", logs[0].ProfileID, created.ID)
	}
}

func TestConfigProfileHandler_DeploymentStatusLifecycle(t *testing.T) {
	handler, _ := newConfigProfileHandler(t)
	created := createProfile(t, handler, "Profile Deploy", models.AgentConfigMap{"feature": true})

	update := models.ProfileDeploymentStatus{
		AgentID:          "agent-1",
		ProfileID:        created.ID,
		AssignedVersion:  created.Version,
		DeployedVersion:  created.Version,
		DeploymentStatus: "deployed",
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/deployments", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.UpdateDeploymentStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/deployments?agent_id=agent-1", nil)
	rec = httptest.NewRecorder()
	handler.GetDeploymentStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var statuses []models.ProfileDeploymentStatus
	if err := json.NewDecoder(rec.Body).Decode(&statuses); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	if statuses[0].AgentID != "agent-1" {
		t.Fatalf("agent_id = %s, want agent-1", statuses[0].AgentID)
	}
}

func TestConfigProfileHandler_VersionsAndRollback(t *testing.T) {
	handler, _ := newConfigProfileHandler(t)
	created := createProfile(t, handler, "Profile Versioned", models.AgentConfigMap{"log_level": "debug"})

	update := models.AgentProfile{
		Name:   "Profile Versioned",
		Config: models.AgentConfigMap{"log_level": "info"},
	}
	updateBody, _ := json.Marshal(update)
	updateReq := httptest.NewRequest(http.MethodPut, "/"+created.ID, bytes.NewReader(updateBody))
	updateReq = updateReq.WithContext(context.WithValue(updateReq.Context(), "username", "tester"))
	updateRec := httptest.NewRecorder()
	handler.UpdateProfile(updateRec, updateReq, created.ID)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("UpdateProfile status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/"+created.ID+"/versions", nil)
	rec := httptest.NewRecorder()
	handler.GetProfileVersions(rec, req, created.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetProfileVersions status = %d", rec.Code)
	}
	var versions []models.AgentProfileVersion
	if err := json.NewDecoder(rec.Body).Decode(&versions); err != nil {
		t.Fatalf("decode versions: %v", err)
	}
	if len(versions) < 2 {
		t.Fatalf("expected multiple versions, got %d", len(versions))
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/"+created.ID+"/rollback/1", nil)
	rollbackReq = rollbackReq.WithContext(context.WithValue(rollbackReq.Context(), "username", "tester"))
	rollbackRec := httptest.NewRecorder()
	handler.RollbackProfile(rollbackRec, rollbackReq, created.ID, "1")
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("RollbackProfile status = %d, body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}

	var rolled models.AgentProfile
	if err := json.NewDecoder(rollbackRec.Body).Decode(&rolled); err != nil {
		t.Fatalf("decode rollback response: %v", err)
	}
	if rolled.Version != created.Version+2 {
		t.Fatalf("version = %d, want %d", rolled.Version, created.Version+2)
	}
	if rolled.Config["log_level"] != "debug" {
		t.Fatalf("config log_level = %v, want debug", rolled.Config["log_level"])
	}
}
