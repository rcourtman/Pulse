package account

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// workspaceCreateLocks provides per-account serialization for workspace
// creation inside one control-plane process. The registry still enforces the
// final workspace limit in the tenant-create transaction, so multi-instance
// deployments share the same source of truth.
var workspaceCreateLocks sync.Map // map[string]*sync.Mutex

func accountCreateLock(accountID string) *sync.Mutex {
	val, _ := workspaceCreateLocks.LoadOrStore(accountID, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// WorkspaceProvisioner is the minimal interface needed by the MSP portal tenant handlers.
// Implemented by internal/cloudcp/stripe.Provisioner.
type WorkspaceProvisioner interface {
	ProvisionWorkspace(ctx context.Context, accountID, displayName string) (*registry.Tenant, error)
	DeprovisionWorkspaceContainer(ctx context.Context, tenant *registry.Tenant) error
}

type ownerAwareWorkspaceProvisioner interface {
	ProvisionWorkspaceForOwner(ctx context.Context, accountID, displayName, ownerEmail string) (*registry.Tenant, error)
}

type WorkspaceLimitPolicy = registry.WorkspaceLimitPolicy

// Real provisioners receive the same explicit server policy as the preflight.
// A default plan string alone is not permission to use local MSP authority.
type policyAwareWorkspaceProvisioner interface {
	ProvisionWorkspaceForOwnerWithPolicy(context.Context, string, string, string, registry.WorkspaceLimitPolicy) (*registry.Tenant, error)
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
	return HandleCreateTenantWithWorkspaceLimitPolicy(reg, provisioner, WorkspaceLimitPolicy{})
}

func HandleCreateTenantWithWorkspaceLimitPolicy(reg *registry.TenantRegistry, provisioner WorkspaceProvisioner, workspaceLimitPolicy WorkspaceLimitPolicy) http.HandlerFunc {
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

		// Serialize workspace creation per account to prevent check-then-create races.
		mu := accountCreateLock(accountID)
		mu.Lock()
		limitErr, tenant, provisionErr := func() (*workspaceLimitError, *registry.Tenant, error) {
			defer mu.Unlock()
			if le := enforceWorkspaceLimit(reg, accountID, workspaceLimitPolicy); le != nil {
				return le, nil, nil
			}
			ownerEmail := strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Email")))
			var (
				t    *registry.Tenant
				pErr error
			)
			if policyAware, ok := provisioner.(policyAwareWorkspaceProvisioner); ok {
				t, pErr = policyAware.ProvisionWorkspaceForOwnerWithPolicy(r.Context(), accountID, displayName, ownerEmail, workspaceLimitPolicy)
			} else if ownerAware, ok := provisioner.(ownerAwareWorkspaceProvisioner); ok {
				t, pErr = ownerAware.ProvisionWorkspaceForOwner(r.Context(), accountID, displayName, ownerEmail)
			} else {
				t, pErr = provisioner.ProvisionWorkspace(r.Context(), accountID, displayName)
			}
			return nil, t, pErr
		}()
		if limitErr != nil {
			auditEvent(r, "cp_tenant_create", "failure").
				Str("account_id", accountID).
				Str("display_name", displayName).
				Str("reason", limitErr.reason).
				Int("current_count", limitErr.current).
				Int("limit", limitErr.limit).
				Msg("Tenant creation blocked by workspace limit")
			writeTenantCreateError(w, limitErr.statusCode, limitErr.reason, limitErr.message, limitErr.current, limitErr.limit)
			return
		}
		if provisionErr != nil {
			var admissionErr *registry.WorkspaceAdmissionError
			if errors.As(provisionErr, &admissionErr) {
				writeTenantCreateError(w, http.StatusForbidden, admissionErr.Reason, admissionErr.Message, 0, 0)
				return
			}
			if limitErr, ok := registry.WorkspaceLimitExceeded(provisionErr); ok {
				auditEvent(r, "cp_tenant_create", "failure").
					Err(provisionErr).
					Str("account_id", accountID).
					Str("display_name", displayName).
					Str("reason", "workspace_limit_reached").
					Int("current_count", limitErr.Current).
					Int("limit", limitErr.Limit).
					Msg("Tenant creation blocked by registry workspace limit")
				writeTenantCreateError(
					w,
					http.StatusForbidden,
					"workspace_limit_reached",
					fmt.Sprintf("workspace limit reached: %d of %d allowed", limitErr.Current, limitErr.Limit),
					limitErr.Current,
					limitErr.Limit,
				)
				return
			}
			auditEvent(r, "cp_tenant_create", "failure").
				Err(provisionErr).
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

// workspaceLimitError describes why workspace creation was blocked.
type workspaceLimitError struct {
	reason     string
	message    string
	statusCode int
	current    int
	limit      int
}

type tenantCreateErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Current int    `json:"current,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// writeTenantCreateError responds with a JSON error payload so the portal
// frontend can show the reason (plain http.Error text bodies were dropped by
// the portal API client and rendered as a generic failure).
func writeTenantCreateError(w http.ResponseWriter, statusCode int, reason, message string, current, limit int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	encodeJSON(w, tenantCreateErrorResponse{
		Error:   reason,
		Message: message,
		Current: current,
		Limit:   limit,
	})
}

// enforceWorkspaceLimit checks whether the account is allowed to create
// another workspace. Returns nil if creation is allowed, or a
// workspaceLimitError if blocked.
func enforceWorkspaceLimit(reg *registry.TenantRegistry, accountID string, policy WorkspaceLimitPolicy) *workspaceLimitError {
	admission, err := reg.ResolveWorkspaceAdmission(context.Background(), accountID, policy)
	if err != nil {
		var admissionErr *registry.WorkspaceAdmissionError
		if errors.As(err, &admissionErr) {
			return &workspaceLimitError{reason: admissionErr.Reason, message: admissionErr.Message, statusCode: http.StatusForbidden}
		}
		return &workspaceLimitError{reason: "billing_lookup_failed", message: "internal error", statusCode: http.StatusInternalServerError}
	}
	planVersion, limit := admission.PlanVersion, admission.Limit

	// Count current active (non-deleted, non-canceled) workspaces.
	// Caller must hold the per-account lock to prevent check-then-create races.
	current, err := reg.CountActiveByAccountID(accountID)
	if err != nil {
		return &workspaceLimitError{
			reason:     "count_failed",
			message:    "internal error",
			statusCode: http.StatusInternalServerError,
		}
	}

	if current >= limit {
		return &workspaceLimitError{
			reason:     "workspace_limit_reached",
			message:    fmt.Sprintf("workspace limit reached: %d of %d allowed for plan %q", current, limit, planVersion),
			statusCode: http.StatusForbidden,
			current:    current,
			limit:      limit,
		}
	}

	return nil
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

// WorkspaceBillingReconciler finishes an existing manual resume without an
// external billing request. It is provided by the control-plane provisioner.
type WorkspaceBillingReconciler interface {
	ResumeWorkspaceBilling(context.Context, *registry.Tenant, bool) error
}

// TenantMutationPolicy comes only from control-plane configuration. Missing
// billing data is not evidence that a Stripe-hosted account is locally billed.
type TenantMutationPolicy struct{ LocalMSPBilling bool }

// HandleUpdateTenant defaults to Stripe-backed authority. Only configured MSP
// control planes may opt into the existing no-Stripe local billing path.
func HandleUpdateTenant(reg *registry.TenantRegistry, billing ...WorkspaceBillingReconciler) http.HandlerFunc {
	return HandleUpdateTenantWithPolicy(reg, TenantMutationPolicy{}, billing...)
}

// HandleUpdateTenantWithPolicy updates display name and/or state.
// Route: PATCH /api/accounts/{account_id}/tenants/{tenant_id}
func HandleUpdateTenantWithPolicy(reg *registry.TenantRegistry, policy TenantMutationPolicy, billing ...WorkspaceBillingReconciler) http.HandlerFunc {
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

		// Preserve the existing not-found/account boundary before consuming a body.
		// This is admission only; the transaction reloads its own current snapshot.
		present, err := reg.GetTenantForAccount(accountID, tenantID)
		if err != nil {
			auditEvent(r, "cp_tenant_update", "failure").Err(err).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "tenant_lookup_failed").Msg("Tenant update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if present == nil {
			auditEvent(r, "cp_tenant_update", "failure").Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "tenant_not_found").Msg("Tenant update failed")
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		var req updateTenantRequest
		if err := decodeJSON(w, r, &req); err != nil {
			auditEvent(r, "cp_tenant_update", "failure").Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "invalid_json_body").Msg("Tenant update failed")
			return
		}
		if req.DisplayName != nil {
			name := strings.TrimSpace(*req.DisplayName)
			if name == "" {
				auditEvent(r, "cp_tenant_update", "failure").Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "invalid_display_name").Msg("Tenant update failed")
				http.Error(w, "invalid display_name", http.StatusBadRequest)
				return
			}
			req.DisplayName = &name
		}
		var next *registry.TenantState
		stateVal := req.Status
		if stateVal == nil {
			stateVal = req.State
		}
		if stateVal != nil {
			state, ok := parseTenantState(*stateVal)
			if !ok {
				auditEvent(r, "cp_tenant_update", "failure").Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "invalid_state").Msg("Tenant update failed")
				http.Error(w, "invalid status", http.StatusBadRequest)
				return
			}
			next = &state
		}
		// A positive resume is ordered with billing events, not a slow request body.
		// Both leases are nonblocking. Withdrawals never wait on a runtime recovery.
		ctx := r.Context()
		if next != nil && *next == registry.TenantStateActive {
			sa, err := reg.GetStripeAccount(accountID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if sa != nil {
				var authority *lifecycle.Lease
				ctx, authority, err = reg.AcquireBillingAuthority(ctx, sa.StripeCustomerID)
				if err != nil {
					http.Error(w, "Workspace billing is busy. Refresh before trying again.", http.StatusServiceUnavailable)
					return
				}
				defer authority.Close()
			}
		}

		ctx, lease, err := reg.AcquireRuntimeLifecycle(ctx, tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer lease.Close()
		r = r.WithContext(ctx)
		previous, tenant, err := reg.PatchTenantForAccount(ctx, accountID, tenantID, req.DisplayName, next, policy.LocalMSPBilling)
		if err != nil {
			status, message, reason := http.StatusInternalServerError, "internal error", "tenant_update_failed"
			if errors.Is(err, registry.ErrTenantNotFound) {
				status, message, reason = http.StatusNotFound, "tenant not found", "tenant_not_found"
			}
			if errors.Is(err, registry.ErrTenantStateConflict) {
				status, message, reason = http.StatusConflict, "Workspace state changed or its subscription is inactive. Refresh before trying again.", "tenant_state_conflict"
			}
			auditEvent(r, "cp_tenant_update", "failure").Err(err).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", reason).Msg("Tenant update failed")
			http.Error(w, message, status)
			return
		}
		if previous.State == registry.TenantStateSuspended && tenant.State == registry.TenantStateActive {
			var resumeErr error
			if len(billing) == 0 || billing[0] == nil {
				resumeErr = fmt.Errorf("workspace billing is unavailable")
			} else {
				resumeErr = billing[0].ResumeWorkspaceBilling(ctx, tenant, policy.LocalMSPBilling)
			}
			if resumeErr != nil {
				// Never roll back a newer withdrawal. A field-scoped re-hold only applies
				// while this request still owns an active/suspended lifecycle.
				suspended := registry.TenantStateSuspended
				_, _, holdErr := reg.PatchTenantForAccount(ctx, accountID, tenantID, nil, &suspended)
				auditEvent(r, "cp_tenant_update", "failure").Err(errors.Join(resumeErr, holdErr)).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "billing_resume_failed").Msg("Tenant resume unconfirmed")
				http.Error(w, "Workspace could not be resumed. Refresh before trying again.", http.StatusServiceUnavailable)
				return
			}
		}

		previousDisplayName, previousState := previous.DisplayName, previous.State

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

		ctx, lease, err := reg.AcquireRuntimeLifecycle(r.Context(), tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer lease.Close()
		r = r.WithContext(ctx)
		previous, tenant, err := reg.BeginTenantDeletionForAccount(ctx, accountID, tenantID)
		if err != nil {
			status, message, reason := http.StatusInternalServerError, "internal error", "tenant_mark_deleting_failed"
			if errors.Is(err, registry.ErrTenantNotFound) {
				status, message, reason = http.StatusNotFound, "tenant not found", "tenant_not_found"
			}
			auditEvent(r, "cp_tenant_delete", "failure").Err(err).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", reason).Msg("Tenant deletion failed")
			http.Error(w, message, status)
			return
		}
		previousState := previous.State
		// Revocation does not depend on Docker cleanup succeeding. A later billing
		// event cannot rotate this token because deleting/deleted stay terminal.
		if err := reg.RevokeHostedEntitlement(tenantID, time.Now().UTC()); err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").Err(err).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "tenant_revoke_failed").Msg("Tenant deletion failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenant.State == registry.TenantStateDeleted {
			auditEvent(r, "cp_tenant_delete", "success").Str("account_id", accountID).Str("tenant_id", tenantID).Str("old_state", string(previousState)).Str("new_state", string(tenant.State)).Msg("Tenant deleted")
			w.WriteHeader(http.StatusNoContent)
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

		_, tenant, err = reg.FinalizeTenantDeletionForAccount(ctx, accountID, tenantID)
		if err != nil {
			auditEvent(r, "cp_tenant_delete", "failure").Err(err).Str("account_id", accountID).Str("tenant_id", tenantID).Str("reason", "tenant_finalize_delete_failed").Msg("Tenant deletion failed")
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
