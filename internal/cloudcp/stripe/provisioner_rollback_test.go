package stripe

import (
	"context"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newStripeTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	regDir := t.TempDir()
	reg, err := registry.NewTenantRegistry(regDir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func newFailingDockerManager(t *testing.T) *docker.Manager {
	t.Helper()
	t.Setenv("DOCKER_HOST", "unix:///tmp/pulse-missing-docker.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	mgr, err := docker.NewManager(docker.ManagerConfig{
		Image:      "pulse:test",
		Network:    "bridge",
		BaseDomain: "cloud.example.com",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}

func TestProvisionWorkspaceRollbackOnContainerFailure(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	dockerMgr := newFailingDockerManager(t)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindMSP,
		DisplayName: "Acme MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	p := NewProvisioner(reg, tenantsDir, dockerMgr, nil, "https://cloud.example.com", nil, "", false)
	if _, err := p.ProvisionWorkspace(context.Background(), accountID, "Tenant One"); err == nil {
		t.Fatal("expected container startup error")
	}

	tenants, err := reg.ListByAccountID(accountID)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}
	if len(tenants) != 0 {
		t.Fatalf("expected no tenant records after rollback, got %d", len(tenants))
	}

	entries, err := os.ReadDir(tenantsDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", tenantsDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no tenant dirs after rollback, got %d", len(entries))
	}
}

func TestHandleCheckoutRollbackAllowsRetry(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()

	failProvisioner := NewProvisioner(reg, tenantsDir, newFailingDockerManager(t), nil, "https://cloud.example.com", nil, "", false)
	session := CheckoutSession{
		Customer:      "cus_retry_123",
		Subscription:  "sub_retry_123",
		CustomerEmail: "owner@example.com",
	}

	if err := failProvisioner.HandleCheckout(context.Background(), session); err == nil {
		t.Fatal("expected container startup error")
	}

	byCustomer, err := reg.GetByStripeCustomerID("cus_retry_123")
	if err != nil {
		t.Fatalf("GetByStripeCustomerID: %v", err)
	}
	if byCustomer != nil {
		t.Fatalf("expected tenant rollback to delete registry record, got %+v", byCustomer)
	}

	entries, err := os.ReadDir(tenantsDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", tenantsDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no tenant dirs after rollback, got %d", len(entries))
	}

	retryProvisioner := NewProvisioner(reg, tenantsDir, nil, nil, "https://cloud.example.com", nil, "", true)
	if err := retryProvisioner.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("retry HandleCheckout: %v", err)
	}

	created, err := reg.GetByStripeCustomerID("cus_retry_123")
	if err != nil {
		t.Fatalf("GetByStripeCustomerID after retry: %v", err)
	}
	if created == nil {
		t.Fatal("expected tenant to exist after successful retry")
	}
	if created.State != registry.TenantStateActive {
		t.Fatalf("state = %q, want %q", created.State, registry.TenantStateActive)
	}
}
