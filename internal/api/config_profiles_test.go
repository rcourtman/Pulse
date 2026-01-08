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
	persistence := config.NewConfigPersistence(tempDir)
	if err := persistence.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	handler := NewConfigProfileHandler(persistence)

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
}
