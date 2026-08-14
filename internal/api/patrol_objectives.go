package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const maxPatrolObjectiveRequestBytes = 16 * 1024

type patrolObjectiveCreateRequest struct {
	Brief           string   `json:"brief"`
	OptionalContext string   `json:"optional_context,omitempty"`
	ResourceIDs     []string `json:"resource_ids,omitempty"`
}

type patrolObjectiveUpdateRequest struct {
	Revision        uint64                    `json:"revision"`
	Brief           *string                   `json:"brief,omitempty"`
	OptionalContext *string                   `json:"optional_context,omitempty"`
	ResourceIDs     *[]string                 `json:"resource_ids,omitempty"`
	Status          *ai.PatrolObjectiveStatus `json:"status,omitempty"`
}

func (h *AISettingsHandler) HandlePatrolObjectives(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.TrimSuffix(r.URL.Path, "/") != "/api/ai/patrol/objectives" {
		writePatrolObjectiveError(w, ai.ErrPatrolObjectiveNotFound)
		return
	}
	store := h.patrolObjectiveStore(r)
	if store == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "patrol_objectives_unavailable", "Patrol objectives are unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		includeArchived, err := parsePatrolObjectiveIncludeArchived(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "patrol_objective_invalid_request", err.Error())
			return
		}
		writePatrolObjectiveJSON(w, http.StatusOK, map[string]any{
			"objectives": store.List(includeArchived, time.Now().UTC()),
		})
	case http.MethodPost:
		var request patrolObjectiveCreateRequest
		if err := decodePatrolObjectiveRequest(w, r, &request); err != nil {
			writeJSONError(w, http.StatusBadRequest, "patrol_objective_invalid_request", "Invalid Patrol objective request")
			return
		}
		objective, err := store.Create(ai.CreatePatrolObjectiveInput{
			Brief:           request.Brief,
			OptionalContext: request.OptionalContext,
			ResourceIDs:     request.ResourceIDs,
			Actor:           strings.TrimSpace(auth.GetUser(r.Context())),
		}, time.Now().UTC())
		if err != nil {
			writePatrolObjectiveError(w, err)
			return
		}
		LogAuditEventForTenant(GetOrgID(r.Context()), "patrol_objective_created", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Created Patrol objective "+objective.ID)
		h.queuePatrolObjectiveCoverage(r, objective)
		writePatrolObjectiveJSON(w, http.StatusCreated, objective)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (h *AISettingsHandler) HandlePatrolObjective(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := patrolObjectiveIDFromPath(r.URL.Path)
	if !ok {
		writePatrolObjectiveError(w, ai.ErrPatrolObjectiveNotFound)
		return
	}
	store := h.patrolObjectiveStore(r)
	if store == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "patrol_objectives_unavailable", "Patrol objectives are unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		objective, found := store.Get(id, time.Now().UTC())
		if !found {
			writePatrolObjectiveError(w, ai.ErrPatrolObjectiveNotFound)
			return
		}
		writePatrolObjectiveJSON(w, http.StatusOK, objective)
	case http.MethodPatch:
		var request patrolObjectiveUpdateRequest
		if err := decodePatrolObjectiveRequest(w, r, &request); err != nil {
			writeJSONError(w, http.StatusBadRequest, "patrol_objective_invalid_request", "Invalid Patrol objective request")
			return
		}
		objective, err := store.Update(id, ai.UpdatePatrolObjectiveInput{
			ExpectedRevision: request.Revision,
			Brief:            request.Brief,
			OptionalContext:  request.OptionalContext,
			ResourceIDs:      request.ResourceIDs,
			Status:           request.Status,
			Actor:            strings.TrimSpace(auth.GetUser(r.Context())),
		}, time.Now().UTC())
		if err != nil {
			writePatrolObjectiveError(w, err)
			return
		}
		LogAuditEventForTenant(GetOrgID(r.Context()), "patrol_objective_updated", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Updated Patrol objective "+objective.ID)
		h.queuePatrolObjectiveCoverage(r, objective)
		writePatrolObjectiveJSON(w, http.StatusOK, objective)
	case http.MethodDelete:
		revision, err := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("revision")), 10, 64)
		if err != nil || revision == 0 {
			writeJSONError(w, http.StatusBadRequest, "patrol_objective_invalid_request", "A valid revision is required")
			return
		}
		if err := store.Delete(id, revision); err != nil {
			writePatrolObjectiveError(w, err)
			return
		}
		LogAuditEventForTenant(GetOrgID(r.Context()), "patrol_objective_deleted", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Deleted Patrol objective "+id)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (h *AISettingsHandler) queuePatrolObjectiveCoverage(r *http.Request, objective ai.PatrolObjective) {
	if h == nil || r == nil || objective.Status != ai.PatrolObjectiveActive || objective.Coverage.State == ai.PatrolObjectiveCovered {
		return
	}
	service := h.GetAIService(r.Context())
	if service == nil {
		return
	}
	if patrol := service.GetPatrolService(); patrol != nil {
		patrol.QueueObjectiveCoverage(objective)
	}
}

func (h *AISettingsHandler) patrolObjectiveStore(r *http.Request) *ai.PatrolObjectiveStore {
	if h == nil || r == nil {
		return nil
	}
	service := h.GetAIService(r.Context())
	if service == nil {
		return nil
	}
	return service.GetPatrolObjectiveStore()
}

func decodePatrolObjectiveRequest(w http.ResponseWriter, r *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPatrolObjectiveRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func parsePatrolObjectiveIncludeArchived(r *http.Request) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("include_archived"))
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("include_archived must be true or false")
	}
	return value, nil
}

func patrolObjectiveIDFromPath(path string) (string, bool) {
	const prefix = "/api/ai/patrol/objectives/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") || len(id) > 128 {
		return "", false
	}
	return id, true
}

func writePatrolObjectiveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ai.ErrPatrolObjectiveNotFound):
		writeJSONError(w, http.StatusNotFound, "patrol_objective_not_found", "Patrol objective not found")
	case errors.Is(err, ai.ErrPatrolObjectiveConflict):
		writeJSONError(w, http.StatusConflict, "patrol_objective_revision_conflict", "Patrol objective changed; reload it before saving")
	case errors.Is(err, ai.ErrPatrolObjectiveLimit):
		writeJSONError(w, http.StatusConflict, "patrol_objective_limit_reached", "Patrol objective limit reached")
	case errors.Is(err, ai.ErrPatrolObjectiveInvalid):
		writeJSONError(w, http.StatusBadRequest, "patrol_objective_invalid_request", err.Error())
	default:
		log.Error().Err(err).Msg("Patrol objective operation failed")
		writeJSONError(w, http.StatusInternalServerError, "patrol_objective_persistence_failed", "Failed to persist Patrol objective")
	}
}

func writePatrolObjectiveJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := utils.WriteJSONResponse(w, payload); err != nil {
		log.Error().Err(err).Msg("Failed to write Patrol objective response")
	}
}
