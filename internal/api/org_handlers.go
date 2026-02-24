package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const (
	orgRequestBodyLimit          = 64 * 1024
	licenseFeatureMultiTenantKey = "multi_tenant"
)

var organizationIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type OrgHandlers struct {
	persistence  *config.MultiTenantPersistence
	mtMonitor    *monitoring.MultiTenantMonitor
	rbacProvider *TenantRBACProvider
	onDelete     func(ctx context.Context, orgID string) error
}

func NewOrgHandlers(
	persistence *config.MultiTenantPersistence,
	mtMonitor *monitoring.MultiTenantMonitor,
	rbacProvider ...*TenantRBACProvider,
) *OrgHandlers {
	var provider *TenantRBACProvider
	if len(rbacProvider) > 0 {
		provider = rbacProvider[0]
	}

	return &OrgHandlers{
		persistence:  persistence,
		mtMonitor:    mtMonitor,
		rbacProvider: provider,
	}
}

// SetOnDelete configures an optional callback invoked after org deletion.
func (h *OrgHandlers) SetOnDelete(callback func(ctx context.Context, orgID string) error) {
	if h == nil {
		return
	}
	h.onDelete = callback
}

type createOrganizationRequest struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type updateOrganizationRequest struct {
	DisplayName string `json:"displayName"`
}

type inviteMemberRequest struct {
	UserID string                  `json:"userId"`
	Role   models.OrganizationRole `json:"role"`
}

type createShareRequest struct {
	TargetOrgID  string                  `json:"targetOrgId"`
	ResourceType string                  `json:"resourceType"`
	ResourceID   string                  `json:"resourceId"`
	ResourceName string                  `json:"resourceName"`
	AccessRole   models.OrganizationRole `json:"accessRole"`
}

type incomingOrganizationShare struct {
	models.OrganizationShare
	SourceOrgID   string `json:"sourceOrgId"`
	SourceOrgName string `json:"sourceOrgName"`
}

func (h *OrgHandlers) HandleListOrgs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	orgs, err := h.persistence.ListOrganizations()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "list_failed", "Failed to list organizations", nil)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	filtered := make([]*models.Organization, 0, len(orgs))
	for _, org := range orgs {
		if org == nil {
			continue
		}
		normalizeOrganization(org)
		if h.canAccessOrg(username, token, org) {
			filtered = append(filtered, org)
		}
	}

	writeJSON(w, http.StatusOK, filtered)
}

func (h *OrgHandlers) HandleCreateOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if username == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "authentication_required", "Authentication required", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)
	var req createOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if !isValidOrganizationID(req.ID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_id", "Invalid organization ID", nil)
		return
	}
	if req.DisplayName == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_display_name", "Display name is required", nil)
		return
	}
	if h.persistence.OrgExists(req.ID) {
		writeErrorResponse(w, http.StatusConflict, "already_exists", "Organization already exists", nil)
		return
	}

	now := time.Now().UTC()
	org := &models.Organization{
		ID:          req.ID,
		DisplayName: req.DisplayName,
		CreatedAt:   now,
		OwnerUserID: username,
		Members: []models.OrganizationMember{
			{
				UserID:  username,
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: username,
			},
		},
	}

	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "create_failed", "Failed to create organization", nil)
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

func (h *OrgHandlers) HandleGetOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if !h.canAccessOrg(username, token, org) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "User is not a member of the organization", nil)
		return
	}

	writeJSON(w, http.StatusOK, org)
}

func (h *OrgHandlers) HandleUpdateOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if orgID == "default" {
		writeErrorResponse(w, http.StatusBadRequest, "default_org_immutable", "Default organization cannot be updated", nil)
		return
	}

	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !org.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)
	var req updateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_display_name", "Display name is required", nil)
		return
	}

	org.DisplayName = req.DisplayName
	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update organization", nil)
		return
	}

	writeJSON(w, http.StatusOK, org)
}

func (h *OrgHandlers) HandleDeleteOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
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

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !org.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}

	if err := h.persistence.DeleteOrganization(orgID); err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errOrgNotFound) {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "Organization not found", nil)
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, "delete_failed", "Failed to delete organization", nil)
		return
	}

	if h.mtMonitor != nil {
		h.mtMonitor.RemoveTenant(orgID)
	}
	if h.rbacProvider != nil && h.onDelete == nil {
		_ = h.rbacProvider.RemoveTenant(orgID)
	}
	if mgr := GetTenantAuditManager(); mgr != nil {
		mgr.RemoveTenantLogger(orgID)
	}
	if h.onDelete != nil {
		if err := h.onDelete(r.Context(), orgID); err != nil {
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Msg("Org deletion cleanup callback failed")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *OrgHandlers) HandleListMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if !h.canAccessOrg(username, token, org) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "User is not a member of the organization", nil)
		return
	}

	writeJSON(w, http.StatusOK, normalizeOrganizationMembers(org.Members))
}

func (h *OrgHandlers) HandleInviteMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	if orgID == "default" {
		writeErrorResponse(w, http.StatusBadRequest, "default_org_immutable", "Default organization members cannot be managed", nil)
		return
	}

	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !org.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)
	var req inviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_user", "Member user ID is required", nil)
		return
	}
	if req.Role == "" {
		req.Role = models.OrgRoleViewer
	}
	req.Role = models.NormalizeOrganizationRole(req.Role)
	if !models.IsValidOrganizationRole(req.Role) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_role", "Role must be owner, admin, editor, or viewer", nil)
		return
	}

	// Owner transfer is allowed only by the current owner.
	if req.Role == models.OrgRoleOwner && username != org.OwnerUserID {
		writeErrorResponse(w, http.StatusForbidden, "owner_required", "Only the organization owner can transfer ownership", nil)
		return
	}
	// The current owner cannot be demoted through member updates.
	if req.UserID == org.OwnerUserID && req.Role != models.OrgRoleOwner {
		writeErrorResponse(w, http.StatusBadRequest, "owner_role_immutable", "Use an ownership transfer to change the owner's role", nil)
		return
	}

	now := time.Now().UTC()

	// Ownership transfer: demote old owner to admin and promote target user to owner.
	if req.Role == models.OrgRoleOwner && req.UserID != org.OwnerUserID {
		for i := range org.Members {
			if org.Members[i].UserID == org.OwnerUserID {
				org.Members[i].Role = models.OrgRoleAdmin
				org.Members[i].AddedBy = username
				if org.Members[i].AddedAt.IsZero() {
					org.Members[i].AddedAt = now
				}
				break
			}
		}
		org.OwnerUserID = req.UserID
	}

	updated := false
	for i := range org.Members {
		if org.Members[i].UserID == req.UserID {
			org.Members[i].Role = req.Role
			org.Members[i].AddedBy = username
			if org.Members[i].AddedAt.IsZero() {
				org.Members[i].AddedAt = now
			}
			updated = true
			break
		}
	}
	if !updated {
		// Enforce max_users limit only for new member additions.
		if enforceUserLimitForMemberAdd(w, r.Context(), org) {
			return
		}
		org.Members = append(org.Members, models.OrganizationMember{
			UserID:  req.UserID,
			Role:    req.Role,
			AddedAt: now,
			AddedBy: username,
		})
	}

	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "invite_failed", "Failed to update organization members", nil)
		return
	}

	normalizedMembers := normalizeOrganizationMembers(org.Members)
	for _, member := range normalizedMembers {
		if member.UserID == req.UserID {
			writeJSON(w, http.StatusOK, member)
			return
		}
	}

	writeErrorResponse(w, http.StatusInternalServerError, "invite_failed", "Failed to update organization members", nil)
}

func (h *OrgHandlers) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	memberUserID := strings.TrimSpace(r.PathValue("userId"))
	if memberUserID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_user", "Member user ID is required", nil)
		return
	}
	if orgID == "default" {
		writeErrorResponse(w, http.StatusBadRequest, "default_org_immutable", "Default organization members cannot be managed", nil)
		return
	}

	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !org.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}
	if memberUserID == org.OwnerUserID {
		writeErrorResponse(w, http.StatusBadRequest, "owner_role_immutable", "Organization owner cannot be removed", nil)
		return
	}

	nextMembers := make([]models.OrganizationMember, 0, len(org.Members))
	removed := false
	for _, member := range org.Members {
		if member.UserID == memberUserID {
			removed = true
			continue
		}
		nextMembers = append(nextMembers, member)
	}
	if !removed {
		writeErrorResponse(w, http.StatusNotFound, "member_not_found", "Member not found", nil)
		return
	}

	org.Members = nextMembers
	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "member_remove_failed", "Failed to update organization members", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *OrgHandlers) HandleListShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	orgID := strings.TrimSpace(r.PathValue("id"))
	org, err := h.loadOrganization(orgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if !h.canAccessOrg(username, token, org) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "User is not a member of the organization", nil)
		return
	}

	writeJSON(w, http.StatusOK, normalizeOrganizationShares(org.SharedResources))
}

func (h *OrgHandlers) HandleListIncomingShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}

	targetOrgID := strings.TrimSpace(r.PathValue("id"))
	targetOrg, err := h.loadOrganization(targetOrgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if !h.canAccessOrg(username, token, targetOrg) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "User is not a member of the organization", nil)
		return
	}

	orgs, err := h.persistence.ListOrganizations()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "list_failed", "Failed to list organizations", nil)
		return
	}

	incoming := make([]incomingOrganizationShare, 0)
	for _, sourceOrg := range orgs {
		if sourceOrg == nil || sourceOrg.ID == targetOrgID {
			continue
		}
		for _, share := range normalizeOrganizationShares(sourceOrg.SharedResources) {
			if share.TargetOrgID != targetOrgID {
				continue
			}
			incoming = append(incoming, incomingOrganizationShare{
				OrganizationShare: share,
				SourceOrgID:       sourceOrg.ID,
				SourceOrgName:     sourceOrg.DisplayName,
			})
		}
	}

	writeJSON(w, http.StatusOK, incoming)
}

func (h *OrgHandlers) HandleCreateShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	sourceOrgID := strings.TrimSpace(r.PathValue("id"))
	sourceOrg, err := h.loadOrganization(sourceOrgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !sourceOrg.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, orgRequestBodyLimit)
	var req createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	req.TargetOrgID = strings.TrimSpace(req.TargetOrgID)
	req.ResourceType = strings.ToLower(strings.TrimSpace(req.ResourceType))
	req.ResourceID = strings.TrimSpace(req.ResourceID)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.AccessRole = models.NormalizeOrganizationRole(req.AccessRole)
	if req.AccessRole == "" {
		req.AccessRole = models.OrgRoleViewer
	}

	if req.TargetOrgID == "" || !isValidOrganizationID(req.TargetOrgID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_target_org", "Valid target organization ID is required", nil)
		return
	}
	if req.TargetOrgID == sourceOrgID {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_target_org", "Target organization must differ from source organization", nil)
		return
	}
	if req.ResourceType == "" || req.ResourceID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_resource", "Resource type and resource ID are required", nil)
		return
	}
	if !models.IsValidOrganizationRole(req.AccessRole) || req.AccessRole == models.OrgRoleOwner {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_access_role", "Access role must be admin, editor, or viewer", nil)
		return
	}

	if _, err := h.loadOrganization(req.TargetOrgID); err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	normalizedShares := normalizeOrganizationShares(sourceOrg.SharedResources)
	for i := range normalizedShares {
		if normalizedShares[i].TargetOrgID == req.TargetOrgID &&
			normalizedShares[i].ResourceType == req.ResourceType &&
			normalizedShares[i].ResourceID == req.ResourceID {
			normalizedShares[i].AccessRole = req.AccessRole
			normalizedShares[i].ResourceName = req.ResourceName
			normalizedShares[i].CreatedBy = username
			if normalizedShares[i].CreatedAt.IsZero() {
				normalizedShares[i].CreatedAt = time.Now().UTC()
			}
			sourceOrg.SharedResources = normalizedShares
			if err := h.persistence.SaveOrganization(sourceOrg); err != nil {
				writeErrorResponse(w, http.StatusInternalServerError, "share_create_failed", "Failed to save organization share", nil)
				return
			}
			writeJSON(w, http.StatusOK, normalizedShares[i])
			return
		}
	}

	share := models.OrganizationShare{
		ID:           generateOrganizationShareID(),
		TargetOrgID:  req.TargetOrgID,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		AccessRole:   req.AccessRole,
		CreatedAt:    time.Now().UTC(),
		CreatedBy:    username,
	}
	sourceOrg.SharedResources = append(normalizedShares, share)

	if err := h.persistence.SaveOrganization(sourceOrg); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "share_create_failed", "Failed to save organization share", nil)
		return
	}

	writeJSON(w, http.StatusCreated, share)
}

func (h *OrgHandlers) HandleDeleteShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.requireMultiTenantGate(w, r) {
		return
	}

	sourceOrgID := strings.TrimSpace(r.PathValue("id"))
	shareID := strings.TrimSpace(r.PathValue("shareId"))
	if shareID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_share", "Share ID is required", nil)
		return
	}

	sourceOrg, err := h.loadOrganization(sourceOrgID)
	if err != nil {
		h.writeLoadOrgError(w, err)
		return
	}

	username := auth.GetUser(r.Context())
	token := getAPITokenRecordFromRequest(r)
	if token != nil || strings.HasPrefix(username, "token:") {
		writeErrorResponse(w, http.StatusForbidden, "session_required", "Session-based user authentication is required", nil)
		return
	}
	if !sourceOrg.CanUserManage(username) {
		writeErrorResponse(w, http.StatusForbidden, "access_denied", "Admin role required for this organization", nil)
		return
	}

	shares := normalizeOrganizationShares(sourceOrg.SharedResources)
	nextShares := make([]models.OrganizationShare, 0, len(shares))
	removed := false
	for _, share := range shares {
		if share.ID == shareID {
			removed = true
			continue
		}
		nextShares = append(nextShares, share)
	}
	if !removed {
		writeErrorResponse(w, http.StatusNotFound, "share_not_found", "Organization share not found", nil)
		return
	}

	sourceOrg.SharedResources = nextShares
	if err := h.persistence.SaveOrganization(sourceOrg); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "share_delete_failed", "Failed to delete organization share", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var errOrgNotFound = errors.New("organization not found")

func (h *OrgHandlers) loadOrganization(orgID string) (*models.Organization, error) {
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
	normalizeOrganization(org)
	return org, nil
}

func (h *OrgHandlers) requireMultiTenantGate(w http.ResponseWriter, r *http.Request) bool {
	if !IsMultiTenantEnabled() {
		writeMultiTenantDisabledError(w)
		return false
	}
	if !hasMultiTenantFeatureForContext(r.Context()) {
		WriteLicenseRequired(w, licenseFeatureMultiTenantKey, "Multi-tenant access requires an Enterprise license")
		return false
	}
	return true
}

func (h *OrgHandlers) canAccessOrg(username string, token *config.APITokenRecord, org *models.Organization) bool {
	if org == nil {
		return false
	}
	if token != nil {
		return token.CanAccessOrg(org.ID)
	}
	if username == "" {
		return false
	}
	if org.ID == "default" {
		return true
	}
	return org.CanUserAccess(username)
}

func (h *OrgHandlers) writeLoadOrgError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errOrgNotFound):
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Organization not found", nil)
	default:
		writeErrorResponse(w, http.StatusInternalServerError, "org_load_failed", "Failed to load organization", nil)
	}
}

func normalizeOrganization(org *models.Organization) {
	if org == nil {
		return
	}
	org.Members = normalizeOrganizationMembers(org.Members)
	org.SharedResources = normalizeOrganizationShares(org.SharedResources)
	if strings.TrimSpace(org.OwnerUserID) == "" {
		return
	}
	for i := range org.Members {
		if org.Members[i].UserID == org.OwnerUserID {
			org.Members[i].Role = models.OrgRoleOwner
			return
		}
	}
	org.Members = append(org.Members, models.OrganizationMember{
		UserID:  org.OwnerUserID,
		Role:    models.OrgRoleOwner,
		AddedAt: time.Now().UTC(),
		AddedBy: org.OwnerUserID,
	})
}

func normalizeOrganizationMembers(members []models.OrganizationMember) []models.OrganizationMember {
	normalized := make([]models.OrganizationMember, 0, len(members))
	for _, member := range members {
		member.UserID = strings.TrimSpace(member.UserID)
		if member.UserID == "" {
			continue
		}
		member.Role = models.NormalizeOrganizationRole(member.Role)
		if !models.IsValidOrganizationRole(member.Role) {
			member.Role = models.OrgRoleViewer
		}
		normalized = append(normalized, member)
	}
	return normalized
}

func normalizeOrganizationShares(shares []models.OrganizationShare) []models.OrganizationShare {
	normalized := make([]models.OrganizationShare, 0, len(shares))
	for _, share := range shares {
		share.ID = strings.TrimSpace(share.ID)
		if share.ID == "" {
			share.ID = generateOrganizationShareID()
		}
		share.TargetOrgID = strings.TrimSpace(share.TargetOrgID)
		share.ResourceType = strings.ToLower(strings.TrimSpace(share.ResourceType))
		share.ResourceID = strings.TrimSpace(share.ResourceID)
		share.ResourceName = strings.TrimSpace(share.ResourceName)
		share.AccessRole = models.NormalizeOrganizationRole(share.AccessRole)
		if share.AccessRole == models.OrgRoleOwner || !models.IsValidOrganizationRole(share.AccessRole) {
			share.AccessRole = models.OrgRoleViewer
		}
		if share.TargetOrgID == "" || share.ResourceType == "" || share.ResourceID == "" {
			continue
		}
		normalized = append(normalized, share)
	}
	return normalized
}

func generateOrganizationShareID() string {
	return "shr-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func isValidOrganizationID(orgID string) bool {
	if orgID == "" || orgID == "." || orgID == ".." {
		return false
	}
	if filepath.Base(orgID) != orgID {
		return false
	}
	return organizationIDPattern.MatchString(orgID)
}
