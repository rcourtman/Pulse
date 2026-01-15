package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ConfigProfileHandler handles configuration profile operations
type ConfigProfileHandler struct {
	persistence       *config.ConfigPersistence
	validator         *models.ProfileValidator
	mu                sync.RWMutex
	suggestionHandler *ProfileSuggestionHandler
}

// NewConfigProfileHandler creates a new handler
func NewConfigProfileHandler(persistence *config.ConfigPersistence) *ConfigProfileHandler {
	return &ConfigProfileHandler{
		persistence: persistence,
		validator:   models.NewProfileValidator(),
	}
}

// SetAIHandler sets the AI handler for profile suggestions
func (h *ConfigProfileHandler) SetAIHandler(aiHandler *AIHandler) {
	h.suggestionHandler = NewProfileSuggestionHandler(h.persistence, aiHandler)
}

// ServeHTTP implements the http.Handler interface
func (h *ConfigProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple routing
	path := strings.TrimSuffix(r.URL.Path, "/")

	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			h.ListProfiles(w, r)
			return
		} else if r.Method == http.MethodPost {
			h.CreateProfile(w, r)
			return
		}
	} else if path == "/assignments" {
		if r.Method == http.MethodGet {
			h.ListAssignments(w, r)
			return
		} else if r.Method == http.MethodPost {
			h.AssignProfile(w, r)
			return
		}
	} else if strings.HasPrefix(path, "/assignments/") {
		if r.Method == http.MethodDelete {
			agentID := strings.TrimPrefix(path, "/assignments/")
			h.UnassignProfile(w, r, agentID)
			return
		}
	} else if path == "/schema" {
		// GET /schema - Return config key definitions
		if r.Method == http.MethodGet {
			h.GetConfigSchema(w, r)
			return
		}
	} else if path == "/validate" {
		// POST /validate - Validate a config without saving
		if r.Method == http.MethodPost {
			h.ValidateConfig(w, r)
			return
		}
	} else if path == "/suggestions" {
		// POST /suggestions - AI-assisted profile suggestion
		if r.Method == http.MethodPost {
			if h.suggestionHandler != nil {
				h.suggestionHandler.HandleSuggestProfile(w, r)
			} else {
				http.Error(w, "AI service not configured", http.StatusServiceUnavailable)
			}
			return
		}
	} else if path == "/changelog" {
		// GET /changelog - Return profile change history
		if r.Method == http.MethodGet {
			h.GetChangeLog(w, r)
			return
		}
	} else if path == "/deployments" {
		// GET /deployments - Return deployment status
		// POST /deployments - Update deployment status from agent
		if r.Method == http.MethodGet {
			h.GetDeploymentStatus(w, r)
			return
		} else if r.Method == http.MethodPost {
			h.UpdateDeploymentStatus(w, r)
			return
		}
	} else if strings.HasSuffix(path, "/versions") {
		// GET /{id}/versions - Get version history for a profile
		id := strings.TrimSuffix(path, "/versions")
		id = strings.TrimPrefix(id, "/")
		if r.Method == http.MethodGet {
			h.GetProfileVersions(w, r, id)
			return
		}
	} else if strings.Contains(path, "/rollback/") {
		// POST /{id}/rollback/{version} - Rollback to a specific version
		parts := strings.Split(path, "/")
		if len(parts) >= 3 && r.Method == http.MethodPost {
			// parts: ["", "id", "rollback", "version"]
			id := parts[1]
			version := parts[len(parts)-1]
			h.RollbackProfile(w, r, id, version)
			return
		}
	} else {
		// ID parameters
		// Expecting /{id}
		id := strings.TrimPrefix(path, "/")
		if r.Method == http.MethodGet {
			h.GetProfile(w, r, id)
			return
		} else if r.Method == http.MethodPut {
			h.UpdateProfile(w, r, id)
			return
		} else if r.Method == http.MethodDelete {
			h.DeleteProfile(w, r, id)
			return
		}
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// ListProfiles returns all profiles
func (h *ConfigProfileHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load profiles")
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}
	// Return empty array instead of null
	if profiles == nil {
		profiles = []models.AgentProfile{}
	}
	json.NewEncoder(w).Encode(profiles)
}

// CreateProfile creates a new profile
func (h *ConfigProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		models.AgentProfile
		ChangeNote string `json:"change_note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Validate configuration
	if input.Config != nil {
		result := h.validator.Validate(input.Config)
		if !result.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "validation_failed",
				"message":  "Configuration validation failed",
				"errors":   result.Errors,
				"warnings": result.Warnings,
			})
			return
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	// Get username from context
	username := getUsernameFromRequest(r)

	input.ID = uuid.New().String()
	input.Version = 1
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()
	input.CreatedBy = username
	input.UpdatedBy = username

	profiles = append(profiles, input.AgentProfile)

	if err := h.persistence.SaveAgentProfiles(profiles); err != nil {
		log.Error().Err(err).Msg("Failed to save profiles")
		http.Error(w, "Failed to save profile", http.StatusInternalServerError)
		return
	}

	// Save initial version to history
	version := models.AgentProfileVersion{
		ProfileID:   input.ID,
		Version:     1,
		Name:        input.Name,
		Description: input.Description,
		Config:      input.Config,
		ParentID:    input.ParentID,
		CreatedAt:   input.CreatedAt,
		CreatedBy:   username,
		ChangeNote:  input.ChangeNote,
	}
	h.saveVersionHistory(version)

	// Log change
	h.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   input.ID,
		ProfileName: input.Name,
		Action:      "create",
		NewVersion:  1,
		User:        username,
		Timestamp:   time.Now(),
		Details:     input.ChangeNote,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(input.AgentProfile)
}

// getUsernameFromRequest extracts the username from the request context
func getUsernameFromRequest(r *http.Request) string {
	if username, ok := r.Context().Value("username").(string); ok {
		return username
	}
	return ""
}

// UpdateProfile updates an existing profile
func (h *ConfigProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var input struct {
		models.AgentProfile
		ChangeNote string `json:"change_note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate configuration
	if input.Config != nil {
		result := h.validator.Validate(input.Config)
		if !result.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "validation_failed",
				"message":  "Configuration validation failed",
				"errors":   result.Errors,
				"warnings": result.Warnings,
			})
			return
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	username := getUsernameFromRequest(r)
	found := false
	var oldVersion int
	var updatedProfile models.AgentProfile

	for i, p := range profiles {
		if p.ID == id {
			oldVersion = p.Version
			profiles[i].Name = input.Name
			profiles[i].Description = input.Description
			profiles[i].Config = input.Config
			profiles[i].ParentID = input.ParentID
			profiles[i].Version = p.Version + 1
			profiles[i].UpdatedAt = time.Now()
			profiles[i].UpdatedBy = username
			updatedProfile = profiles[i]
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	if err := h.persistence.SaveAgentProfiles(profiles); err != nil {
		log.Error().Err(err).Msg("Failed to save profiles")
		http.Error(w, "Failed to save profile", http.StatusInternalServerError)
		return
	}

	// Save new version to history
	version := models.AgentProfileVersion{
		ProfileID:   id,
		Version:     updatedProfile.Version,
		Name:        updatedProfile.Name,
		Description: updatedProfile.Description,
		Config:      updatedProfile.Config,
		ParentID:    updatedProfile.ParentID,
		CreatedAt:   updatedProfile.UpdatedAt,
		CreatedBy:   username,
		ChangeNote:  input.ChangeNote,
	}
	h.saveVersionHistory(version)

	// Log change
	h.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   id,
		ProfileName: updatedProfile.Name,
		Action:      "update",
		OldVersion:  oldVersion,
		NewVersion:  updatedProfile.Version,
		User:        username,
		Timestamp:   time.Now(),
		Details:     input.ChangeNote,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedProfile)
}

// DeleteProfile deletes a profile
func (h *ConfigProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	var deletedProfile *models.AgentProfile
	newProfiles := []models.AgentProfile{}
	for _, p := range profiles {
		if p.ID != id {
			newProfiles = append(newProfiles, p)
		} else {
			deletedProfile = &p
		}
	}

	if len(newProfiles) == len(profiles) {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	if err := h.persistence.SaveAgentProfiles(newProfiles); err != nil {
		log.Error().Err(err).Msg("Failed to save profiles")
		http.Error(w, "Failed to delete profile", http.StatusInternalServerError)
		return
	}

	assignments, err := h.persistence.LoadAgentProfileAssignments()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load assignments for profile cleanup")
		http.Error(w, "Failed to delete profile assignments", http.StatusInternalServerError)
		return
	}

	cleaned := assignments[:0]
	for _, a := range assignments {
		if a.ProfileID != id {
			cleaned = append(cleaned, a)
		}
	}

	if len(cleaned) != len(assignments) {
		if err := h.persistence.SaveAgentProfileAssignments(cleaned); err != nil {
			log.Error().Err(err).Msg("Failed to clean up assignments for deleted profile")
			http.Error(w, "Failed to delete profile assignments", http.StatusInternalServerError)
			return
		}
	}

	// Log deletion
	username := getUsernameFromRequest(r)
	if deletedProfile != nil {
		h.logChange(models.ProfileChangeLog{
			ID:          uuid.New().String(),
			ProfileID:   id,
			ProfileName: deletedProfile.Name,
			Action:      "delete",
			OldVersion:  deletedProfile.Version,
			User:        username,
			Timestamp:   time.Now(),
		})
	}

	w.WriteHeader(http.StatusOK)
}

// ListAssignments returns all assignments
func (h *ConfigProfileHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	assignments, err := h.persistence.LoadAgentProfileAssignments()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load assignments")
		http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
		return
	}
	// Return empty array instead of null
	if assignments == nil {
		assignments = []models.AgentProfileAssignment{}
	}
	json.NewEncoder(w).Encode(assignments)
}

// AssignProfile assigns a profile to an agent
func (h *ConfigProfileHandler) AssignProfile(w http.ResponseWriter, r *http.Request) {
	var input models.AgentProfileAssignment
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.AgentID == "" || input.ProfileID == "" {
		http.Error(w, "AgentID and ProfileID are required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	assignments, err := h.persistence.LoadAgentProfileAssignments()
	if err != nil {
		http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
		return
	}

	// Remove existing assignment for this agent if exists
	newAssignments := []models.AgentProfileAssignment{}
	for _, a := range assignments {
		if a.AgentID != input.AgentID {
			newAssignments = append(newAssignments, a)
		}
	}

	username := getUsernameFromRequest(r)
	input.UpdatedAt = time.Now()
	input.AssignedBy = username
	newAssignments = append(newAssignments, input)

	if err := h.persistence.SaveAgentProfileAssignments(newAssignments); err != nil {
		log.Error().Err(err).Msg("Failed to save assignments")
		http.Error(w, "Failed to save assignment", http.StatusInternalServerError)
		return
	}

	// Get profile name for logging
	profiles, _ := h.persistence.LoadAgentProfiles()
	var profileName string
	for _, p := range profiles {
		if p.ID == input.ProfileID {
			profileName = p.Name
			break
		}
	}

	// Log assignment
	h.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   input.ProfileID,
		ProfileName: profileName,
		Action:      "assign",
		AgentID:     input.AgentID,
		User:        username,
		Timestamp:   time.Now(),
	})

	json.NewEncoder(w).Encode(input)
}

// UnassignProfile removes a profile assignment for an agent.
func (h *ConfigProfileHandler) UnassignProfile(w http.ResponseWriter, r *http.Request, agentID string) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		http.Error(w, "AgentID is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	assignments, err := h.persistence.LoadAgentProfileAssignments()
	if err != nil {
		http.Error(w, "Failed to load assignments", http.StatusInternalServerError)
		return
	}

	var removedAssignment *models.AgentProfileAssignment
	newAssignments := []models.AgentProfileAssignment{}
	for _, a := range assignments {
		if a.AgentID != agentID {
			newAssignments = append(newAssignments, a)
		} else {
			removedAssignment = &a
		}
	}

	if len(newAssignments) != len(assignments) {
		if err := h.persistence.SaveAgentProfileAssignments(newAssignments); err != nil {
			log.Error().Err(err).Msg("Failed to save assignments")
			http.Error(w, "Failed to save assignment", http.StatusInternalServerError)
			return
		}

		// Log unassignment
		if removedAssignment != nil {
			username := getUsernameFromRequest(r)

			// Get profile name for logging
			profiles, _ := h.persistence.LoadAgentProfiles()
			var profileName string
			for _, p := range profiles {
				if p.ID == removedAssignment.ProfileID {
					profileName = p.Name
					break
				}
			}

			h.logChange(models.ProfileChangeLog{
				ID:          uuid.New().String(),
				ProfileID:   removedAssignment.ProfileID,
				ProfileName: profileName,
				Action:      "unassign",
				AgentID:     agentID,
				User:        username,
				Timestamp:   time.Now(),
			})
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetProfile returns a single profile by ID
func (h *ConfigProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request, id string) {
	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load profiles")
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	for _, p := range profiles {
		if p.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(p)
			return
		}
	}

	http.Error(w, "Profile not found", http.StatusNotFound)
}

// GetConfigSchema returns the configuration key definitions
func (h *ConfigProfileHandler) GetConfigSchema(w http.ResponseWriter, r *http.Request) {
	definitions := models.GetConfigKeyDefinitions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(definitions)
}

// ValidateConfig validates a configuration without saving
func (h *ConfigProfileHandler) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	var config models.AgentConfigMap
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result := h.validator.Validate(config)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetChangeLog returns profile change history
func (h *ConfigProfileHandler) GetChangeLog(w http.ResponseWriter, r *http.Request) {
	logs, err := h.persistence.LoadProfileChangeLogs()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load change logs")
		http.Error(w, "Failed to load change logs", http.StatusInternalServerError)
		return
	}

	// Filter by profile_id if specified
	profileID := r.URL.Query().Get("profile_id")
	if profileID != "" {
		filtered := []models.ProfileChangeLog{}
		for _, entry := range logs {
			if entry.ProfileID == profileID {
				filtered = append(filtered, entry)
			}
		}
		logs = filtered
	}

	// Return empty array instead of null
	if logs == nil {
		logs = []models.ProfileChangeLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// GetDeploymentStatus returns deployment status for all agents
func (h *ConfigProfileHandler) GetDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.persistence.LoadProfileDeploymentStatus()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load deployment status")
		http.Error(w, "Failed to load deployment status", http.StatusInternalServerError)
		return
	}

	// Filter by agent_id if specified
	agentID := r.URL.Query().Get("agent_id")
	if agentID != "" {
		filtered := []models.ProfileDeploymentStatus{}
		for _, s := range status {
			if s.AgentID == agentID {
				filtered = append(filtered, s)
			}
		}
		status = filtered
	}

	// Return empty array instead of null
	if status == nil {
		status = []models.ProfileDeploymentStatus{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// UpdateDeploymentStatus updates deployment status from an agent
func (h *ConfigProfileHandler) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	var input models.ProfileDeploymentStatus
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.AgentID == "" || input.ProfileID == "" {
		http.Error(w, "AgentID and ProfileID are required", http.StatusBadRequest)
		return
	}

	// Validate deployment status
	validStatuses := []string{"pending", "deployed", "failed"}
	validStatus := false
	for _, s := range validStatuses {
		if input.DeploymentStatus == s {
			validStatus = true
			break
		}
	}
	if !validStatus {
		http.Error(w, "Invalid deployment status. Must be 'pending', 'deployed', or 'failed'", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	statuses, err := h.persistence.LoadProfileDeploymentStatus()
	if err != nil {
		http.Error(w, "Failed to load deployment status", http.StatusInternalServerError)
		return
	}

	// Update or add the status
	found := false
	for i, s := range statuses {
		if s.AgentID == input.AgentID && s.ProfileID == input.ProfileID {
			statuses[i].DeployedVersion = input.DeployedVersion
			statuses[i].DeploymentStatus = input.DeploymentStatus
			statuses[i].ErrorMessage = input.ErrorMessage
			statuses[i].LastDeployedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		input.LastDeployedAt = time.Now()
		statuses = append(statuses, input)
	}

	if err := h.persistence.SaveProfileDeploymentStatus(statuses); err != nil {
		log.Error().Err(err).Msg("Failed to save deployment status")
		http.Error(w, "Failed to save deployment status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(input)
}

// GetProfileVersions returns version history for a profile
func (h *ConfigProfileHandler) GetProfileVersions(w http.ResponseWriter, r *http.Request, profileID string) {
	versions, err := h.persistence.LoadAgentProfileVersions()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load profile versions")
		http.Error(w, "Failed to load profile versions", http.StatusInternalServerError)
		return
	}

	// Filter by profile ID
	filtered := []models.AgentProfileVersion{}
	for _, v := range versions {
		if v.ProfileID == profileID {
			filtered = append(filtered, v)
		}
	}

	// Return empty array instead of null
	if filtered == nil {
		filtered = []models.AgentProfileVersion{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// RollbackProfile rolls back a profile to a specific version
func (h *ConfigProfileHandler) RollbackProfile(w http.ResponseWriter, r *http.Request, profileID string, versionStr string) {
	var targetVersion int
	if _, err := fmt.Sscanf(versionStr, "%d", &targetVersion); err != nil {
		http.Error(w, "Invalid version number", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Load version history to find the target version
	versions, err := h.persistence.LoadAgentProfileVersions()
	if err != nil {
		http.Error(w, "Failed to load profile versions", http.StatusInternalServerError)
		return
	}

	var targetVersionData *models.AgentProfileVersion
	for i, v := range versions {
		if v.ProfileID == profileID && v.Version == targetVersion {
			targetVersionData = &versions[i]
			break
		}
	}

	if targetVersionData == nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}

	// Load current profiles
	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	username := getUsernameFromRequest(r)
	found := false
	var oldVersion int
	var updatedProfile models.AgentProfile

	for i, p := range profiles {
		if p.ID == profileID {
			oldVersion = p.Version
			profiles[i].Name = targetVersionData.Name
			profiles[i].Description = targetVersionData.Description
			profiles[i].Config = targetVersionData.Config
			profiles[i].ParentID = targetVersionData.ParentID
			profiles[i].Version = p.Version + 1
			profiles[i].UpdatedAt = time.Now()
			profiles[i].UpdatedBy = username
			updatedProfile = profiles[i]
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	if err := h.persistence.SaveAgentProfiles(profiles); err != nil {
		log.Error().Err(err).Msg("Failed to save profiles after rollback")
		http.Error(w, "Failed to rollback profile", http.StatusInternalServerError)
		return
	}

	// Save new version to history
	version := models.AgentProfileVersion{
		ProfileID:   profileID,
		Version:     updatedProfile.Version,
		Name:        updatedProfile.Name,
		Description: updatedProfile.Description,
		Config:      updatedProfile.Config,
		ParentID:    updatedProfile.ParentID,
		CreatedAt:   updatedProfile.UpdatedAt,
		CreatedBy:   username,
		ChangeNote:  fmt.Sprintf("Rolled back to version %d", targetVersion),
	}
	h.saveVersionHistory(version)

	// Log rollback
	h.logChange(models.ProfileChangeLog{
		ID:          uuid.New().String(),
		ProfileID:   profileID,
		ProfileName: updatedProfile.Name,
		Action:      "rollback",
		OldVersion:  oldVersion,
		NewVersion:  updatedProfile.Version,
		User:        username,
		Timestamp:   time.Now(),
		Details:     fmt.Sprintf("Rolled back from version %d to version %d", oldVersion, targetVersion),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedProfile)
}

// saveVersionHistory saves a version to the history
func (h *ConfigProfileHandler) saveVersionHistory(version models.AgentProfileVersion) {
	versions, err := h.persistence.LoadAgentProfileVersions()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load version history")
		return
	}

	versions = append(versions, version)

	if err := h.persistence.SaveAgentProfileVersions(versions); err != nil {
		log.Error().Err(err).Msg("Failed to save version history")
	}
}

// logChange logs a profile change to the change log
func (h *ConfigProfileHandler) logChange(entry models.ProfileChangeLog) {
	if err := h.persistence.AppendProfileChangeLog(entry); err != nil {
		log.Error().Err(err).Msg("Failed to log profile change")
	}
}
