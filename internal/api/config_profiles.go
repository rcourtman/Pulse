package api

import (
	"encoding/json"
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
	persistence *config.ConfigPersistence
	mu          sync.RWMutex
}

// NewConfigProfileHandler creates a new handler
func NewConfigProfileHandler(persistence *config.ConfigPersistence) *ConfigProfileHandler {
	return &ConfigProfileHandler{
		persistence: persistence,
	}
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
	} else {
		// ID parameters
		// Expecting /{id}
		id := strings.TrimPrefix(path, "/")
		if r.Method == http.MethodPut {
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
	var input models.AgentProfile
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	input.ID = uuid.New().String()
	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()

	profiles = append(profiles, input)

	if err := h.persistence.SaveAgentProfiles(profiles); err != nil {
		log.Error().Err(err).Msg("Failed to save profiles")
		http.Error(w, "Failed to save profile", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(input)
}

// UpdateProfile updates an existing profile
func (h *ConfigProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var input models.AgentProfile
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	profiles, err := h.persistence.LoadAgentProfiles()
	if err != nil {
		http.Error(w, "Failed to load profiles", http.StatusInternalServerError)
		return
	}

	found := false
	for i, p := range profiles {
		if p.ID == id {
			profiles[i].Name = input.Name
			profiles[i].Config = input.Config
			profiles[i].UpdatedAt = time.Now()
			input = profiles[i]
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

	json.NewEncoder(w).Encode(input)
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

	newProfiles := []models.AgentProfile{}
	for _, p := range profiles {
		if p.ID != id {
			newProfiles = append(newProfiles, p)
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

	input.UpdatedAt = time.Now()
	newAssignments = append(newAssignments, input)

	if err := h.persistence.SaveAgentProfileAssignments(newAssignments); err != nil {
		log.Error().Err(err).Msg("Failed to save assignments")
		http.Error(w, "Failed to save assignment", http.StatusInternalServerError)
		return
	}

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

	newAssignments := assignments[:0]
	for _, a := range assignments {
		if a.AgentID != agentID {
			newAssignments = append(newAssignments, a)
		}
	}

	if len(newAssignments) != len(assignments) {
		if err := h.persistence.SaveAgentProfileAssignments(newAssignments); err != nil {
			log.Error().Err(err).Msg("Failed to save assignments")
			http.Error(w, "Failed to save assignment", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
