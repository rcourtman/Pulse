package account

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newTestTenantMux(t *testing.T, reg *registry.TenantRegistry, tenantsDir string) (*http.ServeMux, *cpstripe.Provisioner) {
	t.Helper()

	mux := http.NewServeMux()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	provisioner := cpstripe.NewProvisioner(
		reg,
		tenantsDir,
		nil,
		nil,
		"https://cloud.example.com",
		nil,
		"",
		true,
		cpstripe.WithTrialActivationPrivateKey(base64.StdEncoding.EncodeToString(privateKey)),
	)

	listTenants := HandleListTenants(reg)
	createTenant := HandleCreateTenant(reg, provisioner)
	updateTenant := HandleUpdateTenant(reg)
	deleteTenant := HandleDeleteTenant(reg, provisioner)

	collection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTenants(w, r)
		case http.MethodPost:
			createTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	tenant := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			updateTenant(w, r)
		case http.MethodDelete:
			deleteTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/api/accounts/{account_id}/tenants", admin.AdminKeyMiddleware("secret-key", collection))
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}", admin.AdminKeyMiddleware("secret-key", tenant))
	return mux, provisioner
}

// createTestStripeAccount is a helper that creates a StripeAccount for the
// given account ID and plan version so that workspace limit enforcement can
// look up the billing record.
func createTestStripeAccount(t *testing.T, reg *registry.TenantRegistry, accountID, planVersion string) {
	t.Helper()
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:         accountID,
		StripeCustomerID:  "cus_test_" + accountID,
		PlanVersion:       planVersion,
		SubscriptionState: "active",
	}); err != nil {
		t.Fatalf("create stripe account: %v", err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, _ := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test MSP"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_starter")

	body := `{"display_name":"Acme Dental"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got registry.Tenant
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.AccountID != accountID {
		t.Fatalf("account_id = %q, want %q", got.AccountID, accountID)
	}
	if got.DisplayName != "Acme Dental" {
		t.Fatalf("display_name = %q, want %q", got.DisplayName, "Acme Dental")
	}

	keyPath := filepath.Join(tenantsDir, got.ID, "secrets", "handoff.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("handoff.key missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("handoff.key perms = %o, want %o", info.Mode().Perm(), 0o600)
	}
	b, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read handoff.key: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("handoff.key size = %d, want 32", len(b))
	}
}

func TestCreateWorkspace_SeedsOwnerFromAuthenticatedUserEmail(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, _ := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test MSP"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_starter")

	existingOwnerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: existingOwnerID, Email: "legacy-owner@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    existingOwnerID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatal(err)
	}

	body := `{"display_name":"Acme Dental"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	req.Header.Set("X-User-Email", "operator@example.com")
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got registry.Tenant
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	mtp := config.NewMultiTenantPersistence(filepath.Join(tenantsDir, got.ID))
	org, err := mtp.LoadOrganizationStrict(got.ID)
	if err != nil {
		t.Fatalf("LoadOrganizationStrict(%s): %v", got.ID, err)
	}
	if org.OwnerUserID != "operator@example.com" {
		t.Fatalf("org.OwnerUserID = %q, want %q", org.OwnerUserID, "operator@example.com")
	}
	if org.GetMemberRole("operator@example.com") != models.OrgRoleOwner {
		t.Fatalf("org role for operator@example.com = %q, want %q", org.GetMemberRole("operator@example.com"), models.OrgRoleOwner)
	}
	if org.GetMemberRole("legacy-owner@example.com") != models.OrgRoleOwner {
		t.Fatalf("org role for legacy-owner@example.com = %q, want %q", org.GetMemberRole("legacy-owner@example.com"), models.OrgRoleOwner)
	}
}

func TestListWorkspaces(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test MSP"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_starter")

	t1, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client One")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client Two")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/tenants", nil)
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got []*registry.Tenant
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tenants, got %d (%+v)", len(got), got)
	}

	ids := map[string]bool{}
	for _, tt := range got {
		if tt.AccountID != accountID {
			t.Fatalf("tenant account_id = %q, want %q", tt.AccountID, accountID)
		}
		ids[tt.ID] = true
	}
	if !ids[t1.ID] || !ids[t2.ID] {
		t.Fatalf("missing ids: got=%v want=%q,%q", ids, t1.ID, t2.ID)
	}
}

func TestDeleteWorkspace(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test MSP"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_starter")

	tenant, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/tenants/"+tenant.ID, nil)
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	t2, err := reg.Get(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if t2 == nil {
		t.Fatal("expected tenant to exist")
	}
	if t2.State != registry.TenantStateDeleted {
		t.Fatalf("state = %q, want %q", t2.State, registry.TenantStateDeleted)
	}
}

func TestTenantBelongsToAccount(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	account1, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	account2, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: account1, Kind: registry.AccountKindMSP, DisplayName: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: account2, Kind: registry.AccountKindMSP, DisplayName: "A2"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, account1, "msp_starter")
	createTestStripeAccount(t, reg, account2, "msp_starter")

	tenant, err := provisioner.ProvisionWorkspace(context.Background(), account1, "Client")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"display_name":"New Name"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+account2+"/tenants/"+tenant.ID, bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 404/403 (body=%q)", rec.Code, rec.Body.String())
	}
}

func TestCreateWorkspace_BlockedWhenNoBillingRecord(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, _ := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "No Billing MSP"}); err != nil {
		t.Fatal(err)
	}
	// Deliberately NOT creating a StripeAccount.

	body := `{"display_name":"Should Fail"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCreateWorkspace_MSPStarterLimitEnforced(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "MSP Starter"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_starter") // limit = 10

	// Create 10 workspaces (the limit for msp_starter).
	for i := 0; i < 10; i++ {
		if _, err := provisioner.ProvisionWorkspace(context.Background(), accountID, fmt.Sprintf("Client %d", i+1)); err != nil {
			t.Fatalf("provision workspace %d: %v", i+1, err)
		}
	}

	// The 11th should be blocked.
	body := `{"display_name":"Client 11"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCreateWorkspace_DeletedWorkspacesDoNotCount(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Cloud User"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "cloud_starter") // limit = 1

	// Create 1 workspace (the limit for cloud_starter).
	tenant, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "My Workspace")
	if err != nil {
		t.Fatal(err)
	}

	// Mark it as deleted.
	tenant.State = registry.TenantStateDeleted
	if err := reg.Update(tenant); err != nil {
		t.Fatal(err)
	}

	// Should now be able to create another (deleted ones don't count).
	body := `{"display_name":"New Workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestCreateWorkspace_IndividualCloudLimitedToOne(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Cloud User"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "cloud_power") // limit = 1

	// Create the first workspace.
	if _, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "My Workspace"); err != nil {
		t.Fatal(err)
	}

	// Second should be blocked.
	body := `{"display_name":"Extra Workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCreateWorkspace_MSPGrowthHigherLimit(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "MSP Growth"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "msp_growth") // limit = 25

	// Create 10 workspaces — well under the 25 limit.
	for i := 0; i < 10; i++ {
		if _, err := provisioner.ProvisionWorkspace(context.Background(), accountID, fmt.Sprintf("Client %d", i+1)); err != nil {
			t.Fatalf("provision workspace %d: %v", i+1, err)
		}
	}

	// 11th should succeed (limit is 25).
	body := `{"display_name":"Client 11"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestCreateWorkspace_LegacyCloudAliasUsesCanonicalWorkspaceLimit(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, provisioner := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Legacy Cloud User"}); err != nil {
		t.Fatal(err)
	}
	createTestStripeAccount(t, reg, accountID, "cloud-v1")

	if _, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Primary Workspace"); err != nil {
		t.Fatal(err)
	}

	body := `{"display_name":"Blocked Workspace"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCreateWorkspace_BlockedWhenSubscriptionCanceled(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	mux, _ := newTestTenantMux(t, reg, tenantsDir)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Canceled MSP"}); err != nil {
		t.Fatal(err)
	}
	// Create a StripeAccount with canceled subscription state.
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:         accountID,
		StripeCustomerID:  "cus_test_canceled_" + accountID,
		PlanVersion:       "msp_starter",
		SubscriptionState: "canceled",
	}); err != nil {
		t.Fatalf("create stripe account: %v", err)
	}

	body := `{"display_name":"Should Fail"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
