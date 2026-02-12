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

		var req createTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}
		displayName := strings.TrimSpace(req.DisplayName)
		if displayName == "" {
			http.Error(w, "invalid display_name", http.StatusBadRequest)
			return
		}
		if provisioner == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		tenant, err := provisioner.ProvisionWorkspace(r.Context(), accountID, displayName)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

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
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
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

		tenant, err := loadTenantForAccount(reg, accountID, tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenant == nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		var req updateTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}

		if req.DisplayName != nil {
			name := strings.TrimSpace(*req.DisplayName)
			if name == "" {
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
				http.Error(w, "invalid status", http.StatusBadRequest)
				return
			}
			tenant.State = st
		}

		if err := reg.Update(tenant); err != nil {
			if isNotFoundErr(err) {
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

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
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
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

		tenant, err := loadTenantForAccount(reg, accountID, tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenant == nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		tenant.State = registry.TenantStateDeleting
		if err := reg.Update(tenant); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if provisioner != nil {
			if err := provisioner.DeprovisionWorkspaceContainer(r.Context(), tenant); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		tenant.ContainerID = ""
		tenant.State = registry.TenantStateDeleted
		if err := reg.Update(tenant); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
