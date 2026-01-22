package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestConfigProfileHandlers(t *testing.T) {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	// Ensure default persistence exists
	_, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("Failed to initialize default persistence: %v", err)
	}

	handler := NewConfigProfileHandler(mtp)

	// 1. List Profiles (Empty)
	t.Run("ListProfilesEmpty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var profiles []models.AgentProfile
		if err := json.NewDecoder(rec.Body).Decode(&profiles); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(profiles) != 0 {
			t.Errorf("expected 0 profiles, got %d", len(profiles))
		}
	})

	var profileID string

	// 2. Create Profile
	t.Run("CreateProfile", func(t *testing.T) {
		profile := models.AgentProfile{
			Name: "Test Profile",
			Config: map[string]interface{}{
				"log_level": "debug",
				"interval":  "30s",
			},
		}
		body, _ := json.Marshal(profile)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var created models.AgentProfile
		if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if created.Name != "Test Profile" {
			t.Errorf("expected name 'Test Profile', got %q", created.Name)
		}
		if created.ID == "" {
			t.Error("expected non-empty ID")
		}
		profileID = created.ID
	})

	// 3. List Profiles (1 Profile)
	t.Run("ListProfilesOne", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var profiles []models.AgentProfile
		json.NewDecoder(rec.Body).Decode(&profiles)
		if len(profiles) != 1 {
			t.Errorf("expected 1 profile, got %d", len(profiles))
		}
		if profiles[0].ID != profileID {
			t.Errorf("expected ID %s, got %s", profileID, profiles[0].ID)
		}
	})

	// 4. Update Profile
	t.Run("UpdateProfile", func(t *testing.T) {
		update := models.AgentProfile{
			Name: "Updated Profile",
			Config: map[string]interface{}{
				"log_level": "info",
			},
		}
		body, _ := json.Marshal(update)
		req := httptest.NewRequest(http.MethodPut, "/"+profileID, bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var updated models.AgentProfile
		json.NewDecoder(rec.Body).Decode(&updated)
		if updated.Name != "Updated Profile" {
			t.Errorf("expected updated name, got %q", updated.Name)
		}
	})

	// 5. List Assignments (Empty)
	t.Run("ListAssignmentsEmpty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assignments", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var assignments []models.AgentProfileAssignment
		json.NewDecoder(rec.Body).Decode(&assignments)
		if len(assignments) != 0 {
			t.Errorf("expected 0 assignments, got %d", len(assignments))
		}
	})

	// 6. Assign Profile
	t.Run("AssignProfile", func(t *testing.T) {
		assignment := models.AgentProfileAssignment{
			AgentID:   "test-agent",
			ProfileID: profileID,
		}
		body, _ := json.Marshal(assignment)
		req := httptest.NewRequest(http.MethodPost, "/assignments", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var created models.AgentProfileAssignment
		json.NewDecoder(rec.Body).Decode(&created)
		if created.AgentID != "test-agent" || created.ProfileID != profileID {
			t.Errorf("assignment mismatch: %+v", created)
		}
	})

	// 7. List Assignments (1 Assignment)
	t.Run("ListAssignmentsOne", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assignments", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var assignments []models.AgentProfileAssignment
		if err := json.NewDecoder(rec.Body).Decode(&assignments); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(assignments) != 1 {
			t.Errorf("expected 1 assignment, got %d", len(assignments))
		}
	})

	// 8. Unassign Profile
	t.Run("UnassignProfile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/assignments/test-agent", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/assignments", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var assignments []models.AgentProfileAssignment
		if err := json.NewDecoder(rec.Body).Decode(&assignments); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(assignments) != 0 {
			t.Errorf("expected 0 assignments after unassign, got %d", len(assignments))
		}
	})

	// 9. Assign Profile (Cleanup on Delete)
	t.Run("AssignProfileForDeleteCleanup", func(t *testing.T) {
		assignment := models.AgentProfileAssignment{
			AgentID:   "cleanup-agent",
			ProfileID: profileID,
		}
		body, _ := json.Marshal(assignment)
		req := httptest.NewRequest(http.MethodPost, "/assignments", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	// 10. Delete Profile
	t.Run("DeleteProfile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/"+profileID, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		// Verify deleted
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var profiles []models.AgentProfile
		json.NewDecoder(rec.Body).Decode(&profiles)
		if len(profiles) != 0 {
			t.Errorf("expected 0 profiles after delete, got %d", len(profiles))
		}

		req = httptest.NewRequest(http.MethodGet, "/assignments", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		var assignments []models.AgentProfileAssignment
		if err := json.NewDecoder(rec.Body).Decode(&assignments); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(assignments) != 0 {
			t.Errorf("expected 0 assignments after profile delete, got %d", len(assignments))
		}
	})

	// 11. Get Schema
	t.Run("GetSchema", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/schema", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var defs []models.ConfigKeyDefinition
		if err := json.NewDecoder(rec.Body).Decode(&defs); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(defs) == 0 {
			t.Fatalf("expected schema definitions, got 0")
		}

		found := false
		for _, def := range defs {
			if def.Key == "interval" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected schema to include interval")
		}
	})

	// 12. Validate Config
	t.Run("ValidateConfig", func(t *testing.T) {
		payload := map[string]interface{}{
			"interval":    5,
			"unknown_key": true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result models.ValidationResult
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}

		if result.Valid {
			t.Fatalf("expected invalid result, got valid")
		}
		if len(result.Errors) == 0 {
			t.Fatalf("expected validation errors, got none")
		}
		if len(result.Warnings) == 0 {
			t.Fatalf("expected validation warnings, got none")
		}

		hasIntervalError := false
		for _, err := range result.Errors {
			if err.Key == "interval" {
				hasIntervalError = true
				break
			}
		}
		if !hasIntervalError {
			t.Errorf("expected interval error, got %+v", result.Errors)
		}

		hasUnknownWarning := false
		for _, warn := range result.Warnings {
			if warn.Key == "unknown_key" {
				hasUnknownWarning = true
				break
			}
		}
		if !hasUnknownWarning {
			t.Errorf("expected unknown_key warning, got %+v", result.Warnings)
		}
	})
}
