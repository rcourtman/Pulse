package portal

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

type accountInfo struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"display_name"`
	Kind        registry.AccountKind `json:"kind"`
}

type workspaceSummaryItem struct {
	ID              string               `json:"id"`
	DisplayName     string               `json:"display_name"`
	State           registry.TenantState `json:"state"`
	HealthCheckOK   bool                 `json:"health_check_ok"`
	LastHealthCheck *time.Time           `json:"last_health_check"`
	CreatedAt       time.Time            `json:"created_at"`
}

type dashboardSummary struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Suspended int `json:"suspended"`
}

type dashboardResponse struct {
	Account    accountInfo            `json:"account"`
	Workspaces []workspaceSummaryItem `json:"workspaces"`
	Summary    dashboardSummary       `json:"summary"`
}

type workspaceDetailResponse struct {
	Account   accountInfo      `json:"account"`
	Workspace *registry.Tenant `json:"workspace"`
}

func accountIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.URL.Query().Get("account_id")); v != "" {
		return v
	}
	// Convenience for future callers; spec says query param is fine for now.
	if v := strings.TrimSpace(r.Header.Get("X-Account-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Account-Id")); v != "" {
		return v
	}
	return ""
}

// HandlePortalDashboard returns a portal-oriented dashboard response for an account.
// Route: GET /api/portal/dashboard?account_id=...
//
// Auth: admin-key for now (session auth in M-4).
func HandlePortalDashboard(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if reg == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accountID := accountIDFromRequest(r)
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

		resp := dashboardResponse{
			Account: accountInfo{
				ID:          a.ID,
				DisplayName: a.DisplayName,
				Kind:        a.Kind,
			},
			Workspaces: make([]workspaceSummaryItem, 0, len(tenants)),
		}

		for _, t := range tenants {
			if t == nil {
				continue
			}

			resp.Workspaces = append(resp.Workspaces, workspaceSummaryItem{
				ID:              t.ID,
				DisplayName:     t.DisplayName,
				State:           t.State,
				HealthCheckOK:   t.HealthCheckOK,
				LastHealthCheck: t.LastHealthCheck,
				CreatedAt:       t.CreatedAt,
			})

			resp.Summary.Total++

			switch t.State {
			case registry.TenantStateActive:
				resp.Summary.Active++
				if t.HealthCheckOK {
					resp.Summary.Healthy++
				} else {
					resp.Summary.Unhealthy++
				}
			case registry.TenantStateSuspended:
				resp.Summary.Suspended++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encodeJSON(w, resp)
	}
}

// HandlePortalWorkspaceDetail returns a portal-oriented detail response for a single tenant.
// Route: GET /api/portal/workspaces/{tenant_id}?account_id=...
//
// Auth: admin-key for now (session auth in M-4).
func HandlePortalWorkspaceDetail(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if reg == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accountID := accountIDFromRequest(r)
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

		t, err := reg.Get(tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if t == nil || strings.TrimSpace(t.AccountID) == "" || t.AccountID != accountID {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		resp := workspaceDetailResponse{
			Account: accountInfo{
				ID:          a.ID,
				DisplayName: a.DisplayName,
				Kind:        a.Kind,
			},
			Workspace: t,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encodeJSON(w, resp)
	}
}

func encodeJSON(w http.ResponseWriter, payload any) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal: encode JSON response")
	}
}
