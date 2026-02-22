package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// HostedOrgAdminHandlers exposes hosted-mode-only admin endpoints for cross-tenant operations.
// These routes are intended for the hosted control plane and must not leak in self-hosted mode.
type HostedOrgAdminHandlers struct {
	persistence *config.MultiTenantPersistence
	hostedMode  bool
}

func NewHostedOrgAdminHandlers(persistence *config.MultiTenantPersistence, hostedMode bool) *HostedOrgAdminHandlers {
	return &HostedOrgAdminHandlers{
		persistence: persistence,
		hostedMode:  hostedMode,
	}
}

type hostedOrganizationSummary struct {
	OrgID       string    `json:"org_id"`
	DisplayName string    `json:"display_name"`
	OwnerUserID string    `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	Suspended   bool      `json:"suspended"`
	SoftDeleted bool      `json:"soft_deleted"`
}

func (h *HostedOrgAdminHandlers) HandleListOrganizations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	orgs, err := h.persistence.ListOrganizations()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "list_failed", "Failed to list organizations", nil)
		return
	}

	out := make([]hostedOrganizationSummary, 0, len(orgs))
	for _, org := range orgs {
		if org == nil {
			continue
		}
		normalizeOrganization(org)

		displayName := strings.TrimSpace(org.DisplayName)
		if displayName == "" {
			displayName = org.ID
		}

		status := models.NormalizeOrgStatus(org.Status)
		suspended := status == models.OrgStatusSuspended || org.SuspendedAt != nil
		softDeleted := status == models.OrgStatusPendingDeletion || org.DeletionRequestedAt != nil

		out = append(out, hostedOrganizationSummary{
			OrgID:       strings.TrimSpace(org.ID),
			DisplayName: displayName,
			OwnerUserID: strings.TrimSpace(org.OwnerUserID),
			CreatedAt:   org.CreatedAt.UTC(),
			Suspended:   suspended,
			SoftDeleted: softDeleted,
		})
	}

	writeJSON(w, http.StatusOK, out)
}
