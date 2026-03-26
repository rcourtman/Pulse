package portal

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	stripe "github.com/stripe/stripe-go/v82"
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
	mux.Handle(PortalDashboardPath, admin.AdminKeyMiddleware("secret-key", HandlePortalDashboard(reg)))
	mux.Handle(PortalWorkspacePath, admin.AdminKeyMiddleware("secret-key", HandlePortalWorkspaceDetail(reg)))
	return mux
}

func doRequest(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("X-Admin-Key", "secret-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func renderPortalHTML(t *testing.T, data portalPageData) string {
	t.Helper()
	rec := httptest.NewRecorder()
	renderPortalPage(rec, data.Nonce, data.Email, data.Accounts)
	if rec.Code != http.StatusOK {
		t.Fatalf("renderPortalPage returned %d", rec.Code)
	}
	return rec.Body.String()
}

func renderLoginHTML(t *testing.T, nonce string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	renderLoginPage(rec, nonce)
	if rec.Code != http.StatusOK {
		t.Fatalf("renderLoginPage returned %d", rec.Code)
	}
	return rec.Body.String()
}

func extractPortalBootstrapJSONFromHTML(t *testing.T, html string) string {
	t.Helper()
	startMarker := `<script id="pulse-account-bootstrap" type="application/json">`
	start := strings.Index(html, startMarker)
	if start < 0 {
		t.Fatal("bootstrap script tag not found in portal HTML")
	}
	start += len(startMarker)
	end := strings.Index(html[start:], `</script>`)
	if end < 0 {
		t.Fatal("bootstrap script closing tag not found in portal HTML")
	}
	return html[start : start+end]
}

func newPortalSessionFixture(t *testing.T) (*registry.TenantRegistry, *cpauth.Service, string, string, string) {
	t.Helper()

	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Portal Account"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "owner@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    userID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatal(err)
	}

	sessionSvc, err := cpauth.NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(sessionSvc.Close)

	token, err := sessionSvc.GenerateSessionToken(userID, "owner@example.com", cpauth.SessionTTL)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	return reg, sessionSvc, token, accountID, userID
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

// --- Billing portal redirect tests ---

func newBillingHandler(t *testing.T, reg *registry.TenantRegistry, apiKey string, returnURL string,
	mock func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error),
) http.HandlerFunc {
	t.Helper()
	h := &billingPortalHandler{
		reg: reg,
		cfg: BillingPortalConfig{
			StripeAPIKey: apiKey,
			ReturnURL:    returnURL,
		},
		createSession: mock,
	}
	return h.serveHTTP
}

func TestBillingPortalRedirect_Success(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Test Account"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_test_billing123",
	}); err != nil {
		t.Fatal(err)
	}

	var capturedParams *stripe.BillingPortalSessionParams
	handler := newBillingHandler(t, reg, "sk_test_key", "https://cloud.example.com/portal",
		func(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			capturedParams = params
			return &stripe.BillingPortalSession{URL: "https://billing.stripe.com/session/bps_test"}, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}
	if resp["url"] != "https://billing.stripe.com/session/bps_test" {
		t.Fatalf("url = %q, want stripe billing portal URL", resp["url"])
	}

	if capturedParams == nil {
		t.Fatal("expected createSession to be called")
	}
	if got := stripe.StringValue(capturedParams.Customer); got != "cus_test_billing123" {
		t.Fatalf("Customer = %q, want %q", got, "cus_test_billing123")
	}
	if got := stripe.StringValue(capturedParams.ReturnURL); got != "https://cloud.example.com/portal" {
		t.Fatalf("ReturnURL = %q, want %q", got, "https://cloud.example.com/portal")
	}
}

func TestBillingPortalRedirect_AdminRoleAllowed(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_test_admin",
	}); err != nil {
		t.Fatal(err)
	}

	handler := newBillingHandler(t, reg, "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			return &stripe.BillingPortalSession{URL: "https://billing.stripe.com/session/admin"}, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "admin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestBillingPortalRedirect_ForbiddenForTechRole(t *testing.T) {
	handler := newBillingHandler(t, newTestRegistry(t), "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id=a_test", nil)
	req.Header.Set("X-User-Role", "tech")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestBillingPortalRedirect_ForbiddenForReadOnlyRole(t *testing.T) {
	handler := newBillingHandler(t, newTestRegistry(t), "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id=a_test", nil)
	req.Header.Set("X-User-Role", "read_only")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestBillingPortalRedirect_MethodNotAllowed(t *testing.T) {
	handler := newBillingHandler(t, newTestRegistry(t), "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/portal/billing?account_id=a_test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestBillingPortalRedirect_MissingAccountID(t *testing.T) {
	handler := newBillingHandler(t, newTestRegistry(t), "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing", nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestBillingPortalRedirect_NoStripeAccount(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "No Billing"}); err != nil {
		t.Fatal(err)
	}

	handler := newBillingHandler(t, reg, "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestBillingPortalRedirect_StripeAPIKeyNotConfigured(t *testing.T) {
	handler := newBillingHandler(t, newTestRegistry(t), "", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id=a_test", nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestBillingPortalRedirect_StripeError(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_test_err",
	}); err != nil {
		t.Fatal(err)
	}

	handler := newBillingHandler(t, reg, "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			return nil, fmt.Errorf("stripe API unavailable")
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func TestBillingPortalRedirect_EmptyURL(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_test_empty",
	}); err != nil {
		t.Fatal(err)
	}

	handler := newBillingHandler(t, reg, "sk_test_key", "",
		func(*stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			return &stripe.BillingPortalSession{URL: ""}, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

// --- Portal page template rendering tests ---

func TestPortalPageTemplate_TeamManagementRendered(t *testing.T) {
	// Verify that accounts with CanManage=true render team management UI markup.
	data := portalPageData{
		Nonce: "test-nonce",
		Email: "admin@example.com",
		Accounts: []portalPageAccount{
			{
				ID:         "a_managed",
				Kind:       "cloud",
				KindLabel:  "Cloud",
				Name:       "My Cloud Account",
				Role:       "owner",
				CanManage:  true,
				HasBilling: false,
				Workspaces: nil,
			},
		},
	}

	html := renderPortalHTML(t, data)

	// Account-specific markup (only rendered when CanManage=true).
	mustContain := []string{
		`id="team-btn-a_managed"`,     // team button with account ID
		`id="team-section-a_managed"`, // team section div
		`data-actor-role="owner"`,     // actor role data attribute
		"Team members",                // section heading
		`id="team-list-a_managed"`,    // member list tbody
		`id="invite-email-a_managed"`, // invite email input
		`id="invite-role-a_managed"`,  // invite role select
		`data-action="toggle-team"`,
		`data-action="invite-member"`,
		`data-account-id="a_managed"`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(html, needle) {
			t.Errorf("expected %q in rendered HTML", needle)
		}
	}
}

func TestPortalPageTemplate_TeamManagementHiddenForNonManagers(t *testing.T) {
	// Accounts with CanManage=false should NOT render team management markup.
	data := portalPageData{
		Nonce: "test-nonce",
		Email: "viewer@example.com",
		Accounts: []portalPageAccount{
			{
				ID:         "a_readonly",
				Kind:       "cloud",
				KindLabel:  "Cloud",
				Name:       "Readonly Account",
				Role:       "read_only",
				CanManage:  false,
				HasBilling: false,
				Workspaces: nil,
			},
		},
	}

	html := renderPortalHTML(t, data)

	// Account-specific team markup must NOT appear.
	mustNotContain := []string{
		`id="team-btn-a_readonly"`,
		`id="team-section-a_readonly"`,
		`id="invite-email-a_readonly"`,
	}
	for _, needle := range mustNotContain {
		if strings.Contains(html, needle) {
			t.Errorf("%q should NOT appear for non-manager accounts", needle)
		}
	}
}

func TestPortalPageTemplate_ActorRolePassedToSection(t *testing.T) {
	// Verify the actor role is passed as a data attribute for JS permission checks.
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			data := portalPageData{
				Nonce: "n",
				Email: "test@example.com",
				Accounts: []portalPageAccount{
					{
						ID:        "a_test",
						Kind:      "cloud",
						KindLabel: "Cloud",
						Name:      "Test",
						Role:      role,
						CanManage: true,
					},
				},
			}
			html := renderPortalHTML(t, data)
			if !strings.Contains(html, `data-actor-role="`+role+`"`) {
				t.Errorf("expected data-actor-role=%q in rendered HTML", role)
			}
		})
	}
}

func TestPortalPageTemplate_AccountServicesRendered(t *testing.T) {
	data := portalPageData{
		Nonce:         "test-nonce",
		Email:         "owner@example.com",
		PublicSiteURL: "https://pulserelay.pro",
		SupportEmail:  "support@pulserelay.pro",
		Accounts: []portalPageAccount{
			{
				ID:        "a_test",
				Kind:      "cloud",
				KindLabel: "Cloud",
				Name:      "Test Account",
			},
		},
	}

	html := renderPortalHTML(t, data)

	mustContain := []string{
		"<title>Pulse Account</title>",
		"Pulse Account",
		`id="accounts-root"`,
		`id="pulse-account-bootstrap"`,
		`"commercial_api_base_url":"https://license.pulserelay.pro"`,
		fmt.Sprintf(`"portal_path":"%s"`, PortalPagePath),
		fmt.Sprintf(`"bootstrap_path":"%s"`, PortalBootstrapPath),
		fmt.Sprintf(`"logout_path":"%s"`, PortalLogoutPath),
		fmt.Sprintf(`"account_api_base_path":"%s"`, PortalAccountAPIBasePath),
		fmt.Sprintf(`"portal_api_base_path":"%s"`, PortalAPIBasePath),
		`"email":"owner@example.com"`,
		`"accounts":[{"id":"a_test"`,
		"Other account services",
		`id="open-manage-service"`,
		`id="open-retrieve-service"`,
		`id="open-refund-service"`,
		`id="open-data-service"`,
		`data-account-service-action="open-manage-service"`,
		`data-account-service-action="manage-inline-request"`,
		`data-account-service-action="data-delete-confirm"`,
		`id="manage-service-panel"`,
		`id="retrieve-service-panel"`,
		`id="refund-service-panel"`,
		`id="data-service-panel"`,
		`serviceFetch('/v1/manage/request'`,
		`serviceFetch('/v1/retrieve-license/request'`,
		`serviceFetch('/v1/self-refund'`,
		`serviceFetch('/v1/gdpr/request-export'`,
		`serviceFetch('/v1/gdpr/export'`,
		`serviceFetch('/v1/gdpr/request-delete'`,
		`serviceFetch('/v1/gdpr/confirm-delete'`,
		`if (!await refreshBootstrap())`,
		`refreshAccountTeamSection(accountID)`,
		`window.PulseAccountPortal = {`,
		`document.addEventListener('click'`,
		`document.addEventListener('change'`,
		`data-action="open-billing"`,
		`data-action="create-workspace"`,
		`data-action="workspace-manage"`,
		`href="https://pulserelay.pro/refund.html?email=owner%40example.com"`,
		"commercial account actions now live here",
	}
	for _, needle := range mustContain {
		if !strings.Contains(html, needle) {
			t.Errorf("expected %q in rendered HTML", needle)
		}
	}
	if strings.Contains(html, `const LICENSE_API_BASE = 'https://license.pulserelay.pro';`) {
		t.Errorf("expected commercial API base URL to be renderer-owned, not hardcoded in the asset")
	}
	if strings.Contains(html, `window.location.reload()`) {
		t.Errorf("expected workspace lifecycle to refresh the bootstrap contract instead of forcing a page reload")
	}
	if strings.Contains(html, `showToast('Member invited!');`+"\n"+`    loadTeam(accountID);`) {
		t.Errorf("expected membership mutations to refresh the account bootstrap instead of only repainting the team table")
	}
	if strings.Contains(html, `onclick="`) {
		t.Errorf("expected portal shell interactions to be delegated through data-action attributes instead of inline onclick handlers")
	}
	if strings.Contains(html, `assets/portal.js`) {
		t.Errorf("expected portal runtime to be split into explicit embedded shell/services assets instead of the old monolithic asset")
	}
	if strings.Contains(html, `await fetch('/auth/logout'`) {
		t.Errorf("expected portal paths to be renderer-owned, not hardcoded in the asset")
	}
}

func TestBuildPortalBootstrapJSON_Contract(t *testing.T) {
	bootstrapJSON, err := MarshalBootstrapJSON("owner@example.com", []portalPageAccount{
		{
			ID:         "a_test",
			Kind:       "cloud",
			KindLabel:  "Cloud",
			Name:       "Test Account",
			Role:       "owner",
			CanManage:  true,
			HasBilling: true,
			Workspaces: []portalPageWorkspace{
				{
					ID:          "t_one",
					DisplayName: "Tenant One",
					State:       "active",
					Healthy:     true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MarshalBootstrapJSON: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(bootstrapJSON), &payload); err != nil {
		t.Fatalf("unmarshal bootstrap JSON: %v", err)
	}

	if got := payload["email"]; got != "owner@example.com" {
		t.Fatalf("email = %#v, want owner@example.com", got)
	}
	if got := payload["commercial_api_base_url"]; got != "https://license.pulserelay.pro" {
		t.Fatalf("commercial_api_base_url = %#v", got)
	}
	if got := payload["portal_path"]; got != PortalPagePath {
		t.Fatalf("portal_path = %#v", got)
	}
	if got := payload["bootstrap_path"]; got != PortalBootstrapPath {
		t.Fatalf("bootstrap_path = %#v", got)
	}
	if got := payload["logout_path"]; got != PortalLogoutPath {
		t.Fatalf("logout_path = %#v", got)
	}
	if got := payload["account_api_base_path"]; got != PortalAccountAPIBasePath {
		t.Fatalf("account_api_base_path = %#v", got)
	}
	if got := payload["portal_api_base_path"]; got != PortalAPIBasePath {
		t.Fatalf("portal_api_base_path = %#v", got)
	}

	accounts, ok := payload["accounts"].([]any)
	if !ok || len(accounts) != 1 {
		t.Fatalf("accounts = %#v, want one account", payload["accounts"])
	}
	account, ok := accounts[0].(map[string]any)
	if !ok {
		t.Fatalf("account payload = %#v", accounts[0])
	}
	if got := account["id"]; got != "a_test" {
		t.Fatalf("account id = %#v", got)
	}
	if got := account["can_manage"]; got != true {
		t.Fatalf("account can_manage = %#v", got)
	}

	workspaces, ok := account["workspaces"].([]any)
	if !ok || len(workspaces) != 1 {
		t.Fatalf("workspaces = %#v, want one workspace", account["workspaces"])
	}
	workspace, ok := workspaces[0].(map[string]any)
	if !ok {
		t.Fatalf("workspace payload = %#v", workspaces[0])
	}
	if got := workspace["display_name"]; got != "Tenant One" {
		t.Fatalf("workspace display_name = %#v", got)
	}
	if got := workspace["healthy"]; got != true {
		t.Fatalf("workspace healthy = %#v", got)
	}
	if got := workspace["created_at"]; got != "0001-01-01T00:00:00Z" {
		t.Fatalf("workspace created_at = %#v", got)
	}
}

func TestHandlePortalBootstrap_Success(t *testing.T) {
	reg, sessionSvc, token, accountID, _ := newPortalSessionFixture(t)

	if err := reg.Create(&registry.Tenant{
		ID:            "t_bootstrap",
		AccountID:     accountID,
		DisplayName:   "Bootstrap Workspace",
		State:         registry.TenantStateActive,
		CreatedAt:     time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC),
		HealthCheckOK: true,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	HandlePortalBootstrap(sessionSvc, reg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal bootstrap response: %v", err)
	}
	if got := payload["email"]; got != "owner@example.com" {
		t.Fatalf("email = %#v", got)
	}
	if got := payload["portal_api_base_path"]; got != PortalAPIBasePath {
		t.Fatalf("portal_api_base_path = %#v", got)
	}
	if got := payload["bootstrap_path"]; got != PortalBootstrapPath {
		t.Fatalf("bootstrap_path = %#v", got)
	}
	accounts, ok := payload["accounts"].([]any)
	if !ok || len(accounts) != 1 {
		t.Fatalf("accounts = %#v", payload["accounts"])
	}
	account, ok := accounts[0].(map[string]any)
	if !ok {
		t.Fatalf("account = %#v", accounts[0])
	}
	workspaces, ok := account["workspaces"].([]any)
	if !ok || len(workspaces) != 1 {
		t.Fatalf("workspaces = %#v", account["workspaces"])
	}
	workspace, ok := workspaces[0].(map[string]any)
	if !ok {
		t.Fatalf("workspace = %#v", workspaces[0])
	}
	if got := workspace["display_name"]; got != "Bootstrap Workspace" {
		t.Fatalf("workspace display_name = %#v", got)
	}
	if got := workspace["created_at"]; got != "2026-03-25T10:00:00Z" {
		t.Fatalf("workspace created_at = %#v", got)
	}
}

func TestHandlePortalBootstrap_RequiresAuth(t *testing.T) {
	reg := newTestRegistry(t)
	sessionSvc, err := cpauth.NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(sessionSvc.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/portal/bootstrap", nil)
	rec := httptest.NewRecorder()
	HandlePortalBootstrap(sessionSvc, reg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandlePortalBootstrap_RevokedSessionUnauthorized(t *testing.T) {
	reg, sessionSvc, token, _, userID := newPortalSessionFixture(t)

	if _, err := reg.RevokeUserSessions(userID); err != nil {
		t.Fatalf("RevokeUserSessions: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	HandlePortalBootstrap(sessionSvc, reg).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPortalBootstrapHTMLAndHandlerStayInSync(t *testing.T) {
	reg, sessionSvc, token, accountID, _ := newPortalSessionFixture(t)

	if err := reg.Create(&registry.Tenant{
		ID:            "t_sync",
		AccountID:     accountID,
		DisplayName:   "Sync Workspace",
		State:         registry.TenantStateActive,
		CreatedAt:     time.Date(2026, 3, 25, 11, 0, 0, 0, time.UTC),
		HealthCheckOK: true,
	}); err != nil {
		t.Fatal(err)
	}

	claims, err := validatePortalSessionClaims(httptest.NewRequest(http.MethodGet, PortalPagePath, nil), nil, nil)
	if err == nil || claims != nil {
		t.Fatal("expected nil-session validation to require auth")
	}

	loadedAccounts, err := loadPortalAccountsForUser(reg, strings.TrimSpace("u_missing"))
	if err != nil {
		t.Fatalf("loadPortalAccountsForUser for missing user: %v", err)
	}
	if len(loadedAccounts) != 0 {
		t.Fatalf("missing user accounts = %d, want 0", len(loadedAccounts))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portal/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	HandlePortalBootstrap(sessionSvc, reg).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d", rec.Code, http.StatusOK)
	}

	claimsReq := httptest.NewRequest(http.MethodGet, PortalPagePath, nil)
	claimsReq.Header.Set("Authorization", "Bearer "+token)
	validClaims, err := validatePortalSessionClaims(claimsReq, sessionSvc, reg)
	if err != nil {
		t.Fatalf("validatePortalSessionClaims: %v", err)
	}
	accounts, err := loadPortalAccountsForUser(reg, validClaims.UserID)
	if err != nil {
		t.Fatalf("loadPortalAccountsForUser: %v", err)
	}
	html := renderPortalHTML(t, portalPageData{
		Nonce:                "test-nonce",
		Email:                validClaims.Email,
		PublicSiteURL:        defaultPublicSiteURL,
		SupportEmail:         defaultSupportEmail,
		CommercialAPIBaseURL: defaultCommercialAPIBaseURL,
		PortalPath:           defaultPortalPath,
		LogoutPath:           defaultLogoutPath,
		AccountAPIBasePath:   defaultAccountAPIBasePath,
		PortalAPIBasePath:    defaultPortalAPIBasePath,
		Accounts:             accounts,
		Styles:               portalStyles,
		ShellScript:          portalShellScript,
		ServicesScript:       portalServicesScript,
		BootstrapJSON:        mustBootstrapJSON(t, validClaims.Email, accounts),
	})

	handlerJSON := strings.TrimSpace(rec.Body.String())
	pageJSON := strings.TrimSpace(extractPortalBootstrapJSONFromHTML(t, html))
	if handlerJSON != pageJSON {
		t.Fatalf("bootstrap payload drift:\nhandler=%s\npage=%s", handlerJSON, pageJSON)
	}
}

func mustBootstrapJSON(t *testing.T, email string, accounts []portalPageAccount) template.JS {
	t.Helper()
	bootstrapJSON, err := MarshalBootstrapJSON(email, accounts)
	if err != nil {
		t.Fatalf("MarshalBootstrapJSON: %v", err)
	}
	return bootstrapJSON
}

func TestPortalLoginTemplate_UsesPulseAccountBranding(t *testing.T) {
	html := renderLoginHTML(t, "test-nonce")

	mustContain := []string{
		"<title>Pulse Account — Sign In</title>",
		"Pulse Account",
		"Sign in to manage Cloud workspaces, MSP access, and commercial account services.",
	}
	for _, needle := range mustContain {
		if !strings.Contains(html, needle) {
			t.Errorf("expected %q in rendered HTML", needle)
		}
	}
}

func TestBillingPortalRedirect_NoReturnURL(t *testing.T) {
	reg := newTestRegistry(t)
	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_test_noreturn",
	}); err != nil {
		t.Fatal(err)
	}

	var capturedParams *stripe.BillingPortalSessionParams
	handler := newBillingHandler(t, reg, "sk_test_key", "",
		func(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
			capturedParams = params
			return &stripe.BillingPortalSession{URL: "https://billing.stripe.com/session/bps_test"}, nil
		},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/billing?account_id="+accountID, nil)
	req.Header.Set("X-User-Role", "owner")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	if capturedParams == nil {
		t.Fatal("expected createSession to be called")
	}
	if capturedParams.ReturnURL != nil {
		t.Fatalf("ReturnURL = %q, want nil (no return URL configured)", stripe.StringValue(capturedParams.ReturnURL))
	}
}
