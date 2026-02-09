package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const defaultSoftDeleteRetentionDays = 30

// OrgPersistenceProvider defines persistence operations needed for org lifecycle handlers.
type OrgPersistenceProvider interface {
	LoadOrganization(orgID string) (*models.Organization, error)
	SaveOrganization(org *models.Organization) error
	OrgExists(orgID string) bool
}

// OrgLifecycleHandlers provides hosted tenant lifecycle operations.
type OrgLifecycleHandlers struct {
	persistence OrgPersistenceProvider
	hostedMode  bool
}

func NewOrgLifecycleHandlers(persistence OrgPersistenceProvider, hostedMode bool) *OrgLifecycleHandlers {
	return &OrgLifecycleHandlers{
		persistence: persistence,
		hostedMode:  hostedMode,
	}
}

type suspendOrganizationRequest struct {
	Reason string `json:"reason"`
}

type softDeleteOrganizationRequest struct {
	RetentionDays *int `json:"retention_days"`
}

func (h *OrgLifecycleHandlers) HandleSuspendOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if orgID == "default" {
		writeErrorResponse(w, http.StatusBadRequest, "default_org_immutable", "Default organization cannot be suspended", nil)
		return
	}

	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	var req suspendOrganizationRequest
	if err := decodeOptionalLifecycleRequest(w, r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)

	oldStatus := models.NormalizeOrgStatus(org.Status)
	if oldStatus == models.OrgStatusSuspended {
		writeErrorResponse(w, http.StatusConflict, "already_suspended", "Organization is already suspended", nil)
		return
	}

	now := time.Now().UTC()
	org.Status = models.OrgStatusSuspended
	org.SuspendedAt = &now
	org.SuspendReason = req.Reason

	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update organization lifecycle", nil)
		return
	}

	h.logLifecycleChange(r, org.ID, oldStatus, org.Status, req.Reason)
	hosted.GetHostedMetrics().RecordLifecycleTransition(string(oldStatus), string(org.Status))
	writeJSON(w, http.StatusOK, org)
}

func (h *OrgLifecycleHandlers) HandleUnsuspendOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	oldStatus := models.NormalizeOrgStatus(org.Status)
	if oldStatus != models.OrgStatusSuspended {
		writeErrorResponse(w, http.StatusConflict, "not_suspended", "Organization is not suspended", nil)
		return
	}

	org.Status = models.OrgStatusActive
	org.SuspendedAt = nil
	org.SuspendReason = ""

	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update organization lifecycle", nil)
		return
	}

	h.logLifecycleChange(r, org.ID, oldStatus, org.Status, "")
	hosted.GetHostedMetrics().RecordLifecycleTransition(string(oldStatus), string(org.Status))
	writeJSON(w, http.StatusOK, org)
}

func (h *OrgLifecycleHandlers) HandleSoftDeleteOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if orgID == "default" {
		writeErrorResponse(w, http.StatusBadRequest, "default_org_immutable", "Default organization cannot be deleted", nil)
		return
	}

	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	var req softDeleteOrganizationRequest
	if err := decodeOptionalLifecycleRequest(w, r, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	retentionDays := defaultSoftDeleteRetentionDays
	if req.RetentionDays != nil {
		if *req.RetentionDays <= 0 {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_retention_days", "Retention days must be greater than zero", nil)
			return
		}
		retentionDays = *req.RetentionDays
	}

	oldStatus := models.NormalizeOrgStatus(org.Status)
	if oldStatus == models.OrgStatusPendingDeletion {
		writeErrorResponse(w, http.StatusConflict, "already_pending_deletion", "Organization is already pending deletion", nil)
		return
	}

	now := time.Now().UTC()
	org.Status = models.OrgStatusPendingDeletion
	org.DeletionRequestedAt = &now
	org.RetentionDays = retentionDays

	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update organization lifecycle", nil)
		return
	}

	h.logLifecycleChange(r, org.ID, oldStatus, org.Status, "soft_delete")
	hosted.GetHostedMetrics().RecordLifecycleTransition(string(oldStatus), string(org.Status))
	writeJSON(w, http.StatusOK, org)
}

func decodeOptionalLifecycleRequest(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func (h *OrgLifecycleHandlers) loadOrganization(orgID string) (*models.Organization, error) {
	if !isValidOrganizationID(orgID) {
		return nil, errOrgNotFound
	}
	if h.persistence == nil {
		return nil, errors.New("organization persistence is not configured")
	}
	if orgID != "default" && !h.persistence.OrgExists(orgID) {
		return nil, errOrgNotFound
	}

	org, err := h.persistence.LoadOrganization(orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, errOrgNotFound
	}
	if org.ID == "" {
		org.ID = orgID
	}
	if strings.TrimSpace(org.DisplayName) == "" {
		org.DisplayName = org.ID
	}
	org.Status = models.NormalizeOrgStatus(org.Status)
	normalizeOrganization(org)
	return org, nil
}

func (h *OrgLifecycleHandlers) writeLoadOrgError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errOrgNotFound):
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Organization not found", nil)
	default:
		writeErrorResponse(w, http.StatusInternalServerError, "org_load_failed", "Failed to load organization", nil)
	}
}

func (h *OrgLifecycleHandlers) logLifecycleChange(r *http.Request, orgID string, oldStatus, newStatus models.OrgStatus, reason string) {
	actor := strings.TrimSpace(auth.GetUser(r.Context()))
	if actor == "" {
		if token := getAPITokenRecordFromRequest(r); token != nil {
			if token.ID != "" {
				actor = "token:" + token.ID
			}
		}
	}
	if actor == "" {
		actor = "unknown"
	}

	log.Info().
		Str("org_id", orgID).
		Str("old_status", string(oldStatus)).
		Str("new_status", string(newStatus)).
		Str("reason", reason).
		Str("actor", actor).
		Msg("Organization lifecycle status changed")
}
