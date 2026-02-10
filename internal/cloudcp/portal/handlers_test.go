package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func newTestMux(reg *registry.TenantRegistry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/api/portal/dashboard", admin.AdminKeyMiddleware("secret-key", HandlePortalDashboard(reg)))
	mux.Handle("/api/portal/workspaces/{tenant_id}", admin.AdminKeyMiddleware("secret-key", HandlePortalWorkspaceDetail(reg)))
	return mux
}

func doRequest(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("X-Admin-Key", "secret-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type dashboardResp struct {
	Account struct {
		ID          string               `json:"id"`
		DisplayName string               `json:"display_name"`
		Kind        registry.AccountKind `json:"kind"`
	} `json:"account"`
	Workspaces []struct {
		ID              string               `json:"id"`
		DisplayName     string               `json:"display_name"`
		State           registry.TenantState `json:"state"`
		HealthCheckOK   bool                 `json:"health_check_ok"`
		LastHealthCheck *time.Time           `json:"last_health_check"`
		CreatedAt       time.Time            `json:"created_at"`
	} `json:"workspaces"`
	Summary struct {
		Total     int `json:"total"`
		Active    int `json:"active"`
		Healthy   int `json:"healthy"`
		Unhealthy int `json:"unhealthy"`
		Suspended int `json:"suspended"`
	} `json:"summary"`
}

func TestPortalDashboard(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Example MSP"}); err != nil {
		t.Fatal(err)
	}

	tenantActiveID, err := registry.GenerateTenantID()
	if err != nil {
		t.Fatal(err)
	}
	tenantSuspendedID, err := registry.GenerateTenantID()
	if err != nil {
		t.Fatal(err)
	}

	created1 := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	created2 := time.Date(2026, 2, 10, 11, 0, 0, 0, time.UTC)
	lastCheck := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)

	if err := reg.Create(&registry.Tenant{
		ID:              tenantActiveID,
		AccountID:       accountID,
		DisplayName:     "Acme Dental",
		State:           registry.TenantStateActive,
		CreatedAt:       created1,
		LastHealthCheck: &lastCheck,
		HealthCheckOK:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(&registry.Tenant{
		ID:            tenantSuspendedID,
		AccountID:     accountID,
		DisplayName:   "Suspended Workspace",
		State:         registry.TenantStateSuspended,
		CreatedAt:     created2,
		HealthCheckOK: false,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/dashboard?account_id="+accountID, nil)
	rec := doRequest(t, mux, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp dashboardResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}

	if resp.Account.ID != accountID {
		t.Fatalf("account.id = %q, want %q", resp.Account.ID, accountID)
	}
	if resp.Account.DisplayName != "Example MSP" {
		t.Fatalf("account.display_name = %q, want %q", resp.Account.DisplayName, "Example MSP")
	}
	if resp.Account.Kind != registry.AccountKindMSP {
		t.Fatalf("account.kind = %q, want %q", resp.Account.Kind, registry.AccountKindMSP)
	}

	if len(resp.Workspaces) != 2 {
		t.Fatalf("workspaces len = %d, want %d", len(resp.Workspaces), 2)
	}

	// Make assertions order-independent.
	sort.Slice(resp.Workspaces, func(i, j int) bool { return resp.Workspaces[i].ID < resp.Workspaces[j].ID })
	wsByID := map[string]dashboardRespWorkspace{}

	// local helper type for easier indexing
	for _, ws := range resp.Workspaces {
		wsByID[ws.ID] = dashboardRespWorkspace{
			ID:              ws.ID,
			DisplayName:     ws.DisplayName,
			State:           ws.State,
			HealthCheckOK:   ws.HealthCheckOK,
			LastHealthCheck: ws.LastHealthCheck,
			CreatedAt:       ws.CreatedAt,
		}
	}

	active := wsByID[tenantActiveID]
	if active.ID == "" {
		t.Fatalf("missing active workspace id %q", tenantActiveID)
	}
	if active.DisplayName != "Acme Dental" {
		t.Fatalf("active.display_name = %q, want %q", active.DisplayName, "Acme Dental")
	}
	if active.State != registry.TenantStateActive {
		t.Fatalf("active.state = %q, want %q", active.State, registry.TenantStateActive)
	}
	if !active.HealthCheckOK {
		t.Fatalf("active.health_check_ok = false, want true")
	}
	if active.LastHealthCheck == nil || !active.LastHealthCheck.Equal(lastCheck) {
		t.Fatalf("active.last_health_check = %v, want %v", active.LastHealthCheck, lastCheck)
	}
	if !active.CreatedAt.Equal(created1) {
		t.Fatalf("active.created_at = %v, want %v", active.CreatedAt, created1)
	}

	susp := wsByID[tenantSuspendedID]
	if susp.ID == "" {
		t.Fatalf("missing suspended workspace id %q", tenantSuspendedID)
	}
	if susp.State != registry.TenantStateSuspended {
		t.Fatalf("suspended.state = %q, want %q", susp.State, registry.TenantStateSuspended)
	}

	if resp.Summary.Total != 2 {
		t.Fatalf("summary.total = %d, want %d", resp.Summary.Total, 2)
	}
	if resp.Summary.Active != 1 {
		t.Fatalf("summary.active = %d, want %d", resp.Summary.Active, 1)
	}
	if resp.Summary.Healthy != 1 {
		t.Fatalf("summary.healthy = %d, want %d", resp.Summary.Healthy, 1)
	}
	if resp.Summary.Unhealthy != 0 {
		t.Fatalf("summary.unhealthy = %d, want %d", resp.Summary.Unhealthy, 0)
	}
	if resp.Summary.Suspended != 1 {
		t.Fatalf("summary.suspended = %d, want %d", resp.Summary.Suspended, 1)
	}
}

type dashboardRespWorkspace struct {
	ID              string
	DisplayName     string
	State           registry.TenantState
	HealthCheckOK   bool
	LastHealthCheck *time.Time
	CreatedAt       time.Time
}

func TestPortalDashboardEmpty(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Empty MSP"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/dashboard?account_id="+accountID, nil)
	rec := doRequest(t, mux, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp dashboardResp
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}

	if len(resp.Workspaces) != 0 {
		t.Fatalf("workspaces len = %d, want %d", len(resp.Workspaces), 0)
	}
	if resp.Summary.Total != 0 || resp.Summary.Active != 0 || resp.Summary.Healthy != 0 || resp.Summary.Unhealthy != 0 || resp.Summary.Suspended != 0 {
		t.Fatalf("summary = %+v, want all zeros", resp.Summary)
	}
}

func TestPortalWorkspaceDetail(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Example MSP"}); err != nil {
		t.Fatal(err)
	}

	tenantID, err := registry.GenerateTenantID()
	if err != nil {
		t.Fatal(err)
	}

	created := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	lastCheck := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	if err := reg.Create(&registry.Tenant{
		ID:              tenantID,
		AccountID:       accountID,
		DisplayName:     "Acme Dental",
		State:           registry.TenantStateActive,
		CreatedAt:       created,
		LastHealthCheck: &lastCheck,
		HealthCheckOK:   true,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/workspaces/"+tenantID+"?account_id="+accountID, nil)
	rec := doRequest(t, mux, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Account struct {
			ID          string               `json:"id"`
			DisplayName string               `json:"display_name"`
			Kind        registry.AccountKind `json:"kind"`
		} `json:"account"`
		Workspace registry.Tenant `json:"workspace"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}

	if resp.Account.ID != accountID {
		t.Fatalf("account.id = %q, want %q", resp.Account.ID, accountID)
	}
	if resp.Workspace.ID != tenantID {
		t.Fatalf("workspace.id = %q, want %q", resp.Workspace.ID, tenantID)
	}
	if resp.Workspace.AccountID != accountID {
		t.Fatalf("workspace.account_id = %q, want %q", resp.Workspace.AccountID, accountID)
	}
	if resp.Workspace.DisplayName != "Acme Dental" {
		t.Fatalf("workspace.display_name = %q, want %q", resp.Workspace.DisplayName, "Acme Dental")
	}
	if resp.Workspace.State != registry.TenantStateActive {
		t.Fatalf("workspace.state = %q, want %q", resp.Workspace.State, registry.TenantStateActive)
	}
	if !resp.Workspace.HealthCheckOK {
		t.Fatalf("workspace.health_check_ok = false, want true")
	}
	if resp.Workspace.LastHealthCheck == nil || !resp.Workspace.LastHealthCheck.Equal(lastCheck) {
		t.Fatalf("workspace.last_health_check = %v, want %v", resp.Workspace.LastHealthCheck, lastCheck)
	}
	if !resp.Workspace.CreatedAt.Equal(created) {
		t.Fatalf("workspace.created_at = %v, want %v", resp.Workspace.CreatedAt, created)
	}
}
