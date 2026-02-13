package account

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// WorkspaceProvisioner is the minimal interface needed by the MSP portal tenant handlers.
// Implemented by internal/cloudcp/stripe.Provisioner.
type WorkspaceProvisioner interface {
	ProvisionWorkspace(ctx context.Context, accountID, displayName string) (*registry.Tenant, error)
	DeprovisionWorkspaceContainer(ctx context.Context, tenant *registry.Tenant) error
}

// HandleListTenants lists all tenants for an account.
// Route: GET /api/accounts/{account_id}/tenants
func HandleListTenants(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		if accountID == "" {
			http.Error(w, "missing account_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		tenants, err := reg.ListByAccountID(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenants == nil {
			tenants = []*registry.Tenant{}
		}

		w.Header().Set("Content-Type", "application/json")
		encodeJSON(w, tenants)
	}
}

type createTenantRequest struct {
	DisplayName string `json:"display_name"`
}

// HandleCreateTenant creates a new tenant under an account.
// Route: POST /api/accounts/{account_id}/tenants
func HandleCreateTenant(reg *registry.TenantRegistry, provisioner WorkspaceProvisioner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		if accountID == "" {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("reason", "missing_account_id").
				Msg("Tenant creation failed")
			http.Error(w, "missing account_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("reason", "account_lookup_failed").
				Msg("Tenant creation failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("account_id", accountID).
				Str("reason", "account_not_found").
				Msg("Tenant creation failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		var req createTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("account_id", accountID).
				Str("reason", "invalid_json_body").
				Msg("Tenant creation failed")
			return
		}
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("account_id", accountID).
				Str("reason", "invalid_display_name").
				Msg("Tenant creation failed")
			http.Error(w, "invalid display_name", http.StatusBadRequest)
			return
		}
		if provisioner == nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("account_id", accountID).
				Str("display_name", displayName).
				Str("reason", "provisioner_unavailable").
				Msg("Tenant creation failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		tenant, err := provisioner.ProvisionWorkspace(r.Context(), accountID, displayName)
		if err != nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("display_name", displayName).
				Str("reason", "provision_failed").
				Msg("Tenant creation failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		auditEvent(r, "cp_tenant_create", "success").
			Str("account_id", accountID).
			Str("tenant_id", tenant.ID).
			Str("display_name", tenant.DisplayName).
			Str("state", string(tenant.State)).
			Msg("Tenant created")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		encodeJSON(w, tenant)
	}
}

type updateTenantRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Status      *string `json:"status,omitempty"`
	State       *string `json:"state,omitempty"`
}

func parseTenantState(s string) (registry.TenantState, bool) {
	switch registry.TenantState(strings.TrimSpace(s)) {
	case registry.TenantStateActive:
		return registry.TenantStateActive, true
	case registry.TenantStateSuspended:
		return registry.TenantStateSuspended, true
	default:
		return "", false
	}
}

func loadTenantForAccount(reg *registry.TenantRegistry, accountID, tenantID string) (*registry.Tenant, error) {
	t, err := reg.Get(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load tenant %q: %w", tenantID, err)
	}
	if t == nil {
		return nil, nil
	}
	if strings.TrimSpace(t.AccountID) == "" || t.AccountID != accountID {
		return nil, nil
	}
	return t, nil
}

// HandleUpdateTenant updates display name and/or state.
// Route: PATCH /api/accounts/{account_id}/tenants/{tenant_id}
func HandleUpdateTenant(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
		if accountID == "" || tenantID == "" {
			auditEvent(r, "cp_tenant_update", "failure").
				Str("reason", "missing_account_id_or_tenant_id").
				Msg("Tenant update failed")
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_tenant_update", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "account_lookup_failed").
				Msg("Tenant update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_tenant_update", "failure").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "account_not_found").
				Msg("Tenant update failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		tenant, err := reg.GetTenantForAccount(accountID, tenantID)
		if err != nil {
			auditEvent(r, "cp_tenant_update", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_lookup_failed").
				Msg("Tenant update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenant == nil {
			auditEvent(r, "cp_tenant_update", "failure").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_not_found").
				Msg("Tenant update failed")
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		var req updateTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			auditEvent(r, "cp_tenant_update", "failure").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "invalid_json_body").
				Msg("Tenant update failed")
			return
		}

		previousDisplayName := tenant.DisplayName
		previousState := tenant.State

		if req.DisplayName != nil {
			name := strings.TrimSpace(*req.DisplayName)
			if name == "" {
				auditEvent(r, "cp_tenant_update", "failure").
					Str("account_id", accountID).
					Str("tenant_id", tenantID).
					Str("reason", "invalid_display_name").
					Msg("Tenant update failed")
				http.Error(w, "invalid display_name", http.StatusBadRequest)
				return
			}
			tenant.DisplayName = name
		}

		stateVal := req.Status
		if stateVal == nil {
			stateVal = req.State
		}
		if stateVal != nil {
			st, ok := parseTenantState(*stateVal)
			if !ok {
				auditEvent(r, "cp_tenant_update", "failure").
					Str("account_id", accountID).
					Str("tenant_id", tenantID).
					Str("reason", "invalid_state").
					Msg("Tenant update failed")
				http.Error(w, "invalid status", http.StatusBadRequest)
				return
			}
			tenant.State = st
		}

		if err := reg.Update(tenant); err != nil {
			if isNotFoundErr(err) {
				auditEvent(r, "cp_tenant_update", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("tenant_id", tenantID).
					Str("reason", "tenant_not_found").
					Msg("Tenant update failed")
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
			auditEvent(r, "cp_tenant_update", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_update_failed").
				Msg("Tenant update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		auditEvent(r, "cp_tenant_update", "success").
			Str("account_id", accountID).
			Str("tenant_id", tenantID).
			Str("old_display_name", previousDisplayName).
			Str("new_display_name", tenant.DisplayName).
			Str("old_state", string(previousState)).
			Str("new_state", string(tenant.State)).
			Msg("Tenant updated")

		w.Header().Set("Content-Type", "application/json")
		encodeJSON(w, tenant)
	}
}

// HandleDeleteTenant soft-deletes a tenant and deprovisions its container if Docker is available.
// Route: DELETE /api/accounts/{account_id}/tenants/{tenant_id}
func HandleDeleteTenant(reg *registry.TenantRegistry, provisioner WorkspaceProvisioner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
		if accountID == "" || tenantID == "" {
			auditEvent(r, "cp_tenant_delete", "failure").
				Str("reason", "missing_account_id_or_tenant_id").
				Msg("Tenant deletion failed")
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "account_lookup_failed").
				Msg("Tenant deletion failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "account_not_found").
				Msg("Tenant deletion failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		tenant, err := reg.GetTenantForAccount(accountID, tenantID)
		if err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_lookup_failed").
				Msg("Tenant deletion failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenant == nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_not_found").
				Msg("Tenant deletion failed")
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		previousState := tenant.State
		tenant.State = registry.TenantStateDeleting
		if err := reg.Update(tenant); err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_mark_deleting_failed").
				Msg("Tenant deletion failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if provisioner != nil {
			if err := provisioner.DeprovisionWorkspaceContainer(r.Context(), tenant); err != nil {
				auditEvent(r, "cp_tenant_delete", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("tenant_id", tenantID).
					Str("reason", "deprovision_failed").
					Msg("Tenant deletion failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		tenant.ContainerID = ""
		tenant.State = registry.TenantStateDeleted
		if err := reg.Update(tenant); err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("tenant_id", tenantID).
				Str("reason", "tenant_finalize_delete_failed").
				Msg("Tenant deletion failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		auditEvent(r, "cp_tenant_delete", "success").
			Str("account_id", accountID).
			Str("tenant_id", tenantID).
			Str("old_state", string(previousState)).
			Str("new_state", string(tenant.State)).
			Msg("Tenant deleted")

		w.WriteHeader(http.StatusNoContent)
	}
}
