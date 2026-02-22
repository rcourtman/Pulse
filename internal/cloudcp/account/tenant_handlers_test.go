package account

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

func newTestTenantMux(t *testing.T, reg *registry.TenantRegistry, tenantsDir string) (*http.ServeMux, *cpstripe.Provisioner) {
	t.Helper()

	mux := http.NewServeMux()
	provisioner := cpstripe.NewProvisioner(reg, tenantsDir, nil, nil, "https://cloud.example.com", nil, "", true)

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
