package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

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

	req = normalizeActionRequestForAudit(req)
	store, err := h.getStore(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "action_audit_unavailable", "Action audit history is not available", nil)
		return
	}
	if err := persistActionPlanAudit(store, req, plan); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "action_audit_persist_failed", sanitizeErrorForClient(err, "Failed to persist action audit"), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plan); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "action_plan_encode_failed", "Failed to encode action plan", nil)
	}
}

func normalizeActionRequestForAudit(req unified.ActionRequest) unified.ActionRequest {
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	req.CapabilityName = strings.TrimSpace(req.CapabilityName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	return req
}

func persistActionPlanAudit(store unified.ResourceStore, req unified.ActionRequest, plan unified.ActionPlan) error {
	state := plannedActionState(plan)
	record := unified.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: plan.PlannedAt,
		UpdatedAt: plan.PlannedAt,
		State:     state,
		Request:   req,
		Plan:      plan,
	}
	if err := store.RecordActionAudit(record); err != nil {
		return err
	}

	existingEvents, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 100)
	if err != nil {
		return err
	}
	seenStates := map[unified.ActionState]bool{}
	for _, event := range existingEvents {
		seenStates[event.State] = true
	}

	if !seenStates[unified.ActionStatePlanned] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     unified.ActionStatePlanned,
			Actor:     req.RequestedBy,
			Message:   "Action plan created.",
		}); err != nil {
			return err
		}
	}
	if state != unified.ActionStatePlanned && !seenStates[state] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     state,
			Actor:     req.RequestedBy,
			Message:   "Action is waiting for approval before execution.",
		}); err != nil {
			return err
		}
	}
	return nil
}

func plannedActionState(plan unified.ActionPlan) unified.ActionState {
	if plan.RequiresApproval {
		return unified.ActionStatePending
	}
	return unified.ActionStatePlanned
}
