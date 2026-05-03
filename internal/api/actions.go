package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const maxActionPlanRequestBytes = 1 << 20

func (h *ResourceHandlers) HandlePlanAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req unified.ActionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxActionPlanRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"body": "request body must be a valid ActionRequest JSON object",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"body": "request body must contain one JSON object",
		})
		return
	}

	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	if req.ResourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", map[string]string{
			"resourceId": "resource id is required",
		})
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "resource_registry_unavailable", sanitizeErrorForClient(err, "Resource registry unavailable"), nil)
		return
	}

	resource, ok := registry.Get(req.ResourceID)
	if !ok || resource == nil {
		writeErrorResponse(w, http.StatusNotFound, "resource_not_found", "Resource not found", map[string]string{
			"resourceId": req.ResourceID,
		})
		return
	}

	plan, err := (actionplanner.Planner{}).Plan(req, *resource)
	if err != nil {
		if validationErr, ok := actionplanner.AsValidationError(err); ok {
			details := map[string]string{}
			if validationErr.Field != "" {
				details[validationErr.Field] = validationErr.Message
			}
			writeErrorResponse(w, http.StatusBadRequest, "invalid_action_request", "Invalid action planning request", details)
			return
		}
		if errors.Is(err, actionplanner.ErrCapabilityNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "capability_not_found", "Capability not found on resource", map[string]string{
				"resourceId":     req.ResourceID,
				"capabilityName": req.CapabilityName,
			})
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "action_plan_failed", sanitizeErrorForClient(err, "Action planning failed"), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plan); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "action_plan_encode_failed", "Failed to encode action plan", nil)
	}
}
