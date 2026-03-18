package stripe

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

func newTestProvisioner(t *testing.T, reg *registry.TenantRegistry, tenantsDir string, dockerMgr *docker.Manager, allowDockerless bool) *Provisioner {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	return NewProvisioner(
		reg,
		tenantsDir,
		dockerMgr,
		nil,
		"https://cloud.example.com",
		nil,
		"",
		allowDockerless,
		WithTrialActivationPrivateKey(base64.StdEncoding.EncodeToString(privateKey)),
	)
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

	p := newTestProvisioner(t, reg, tenantsDir, dockerMgr, false)
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

	failProvisioner := newTestProvisioner(t, reg, tenantsDir, newFailingDockerManager(t), false)
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

	retryProvisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)
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

func TestProvisionWorkspaceSetsHostedRuntimeOwnershipForImmutableFiles(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()

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
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:            accountID,
		PlanVersion:          "msp_starter",
		SubscriptionState:    "active",
		StripeCustomerID:     "cus_test_msp_owned",
		StripeSubscriptionID: "sub_test_msp_owned",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	p := newTestProvisioner(t, reg, tenantsDir, nil, true)
	var chowned []string
	p.chownFile = func(path string, uid, gid int) error {
		chowned = append(chowned, path)
		if uid != hostedTenantRuntimeUID || gid != hostedTenantRuntimeGID {
			t.Fatalf("unexpected hosted runtime ownership target uid=%d gid=%d", uid, gid)
		}
		return nil
	}

	tenant, err := p.ProvisionWorkspace(context.Background(), accountID, "Tenant One")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}

	want := map[string]bool{
		filepath.Join(tenantsDir, tenant.ID, "billing.json"):           true,
		filepath.Join(tenantsDir, tenant.ID, ".cloud_handoff_key"):     true,
		filepath.Join(tenantsDir, tenant.ID, "secrets", "handoff.key"): true,
	}
	for _, path := range chowned {
		delete(want, path)
	}
	if len(want) != 0 {
		t.Fatalf("missing hosted runtime ownership paths: %v", want)
	}
}

func TestHandleCheckoutSetsHostedRuntimeOwnershipForImmutableFiles(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()

	p := newTestProvisioner(t, reg, tenantsDir, nil, true)
	var chowned []string
	p.chownFile = func(path string, uid, gid int) error {
		chowned = append(chowned, path)
		if uid != hostedTenantRuntimeUID || gid != hostedTenantRuntimeGID {
			t.Fatalf("unexpected hosted runtime ownership target uid=%d gid=%d", uid, gid)
		}
		return nil
	}

	session := CheckoutSession{
		Customer:      "cus_checkout_owned",
		Subscription:  "sub_checkout_owned",
		CustomerEmail: "owner@example.com",
		Metadata:      map[string]string{"plan_version": "cloud_starter"},
	}
	if err := p.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID(session.Customer)
	if err != nil {
		t.Fatalf("GetByStripeCustomerID: %v", err)
	}
	if tenant == nil {
		t.Fatal("expected tenant to exist after checkout")
	}

	want := map[string]bool{
		filepath.Join(tenantsDir, tenant.ID, "orgs"):                        true,
		filepath.Join(tenantsDir, tenant.ID, "orgs", tenant.ID):             true,
		filepath.Join(tenantsDir, tenant.ID, "orgs", tenant.ID, "org.json"): true,
		filepath.Join(tenantsDir, tenant.ID, "billing.json"):                true,
		filepath.Join(tenantsDir, tenant.ID, ".cloud_handoff_key"):          true,
		filepath.Join(tenantsDir, tenant.ID, "secrets", "handoff.key"):      true,
	}
	for _, path := range chowned {
		delete(want, path)
	}
	if len(want) != 0 {
		t.Fatalf("missing hosted runtime ownership paths: %v", want)
	}
}

func TestBuildSeededTenantOrganization_UsesDeterministicOldestOwnerFallback(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	p := newTestProvisioner(t, reg, tenantsDir, nil, true)

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

	createOwner := func(email string, createdAt int64) {
		t.Helper()
		userID, err := registry.GenerateUserID()
		if err != nil {
			t.Fatalf("GenerateUserID: %v", err)
		}
		if err := reg.CreateUser(&registry.User{ID: userID, Email: email}); err != nil {
			t.Fatalf("CreateUser(%s): %v", email, err)
		}
		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    userID,
			Role:      registry.MemberRoleOwner,
			CreatedAt: time.Unix(createdAt, 0).UTC(),
		}); err != nil {
			t.Fatalf("CreateMembership(%s): %v", email, err)
		}
	}

	createOwner("new-owner@example.com", 200)
	createOwner("old-owner@example.com", 100)

	org, err := p.buildSeededTenantOrganization(accountID, "t-example", "Example", "")
	if err != nil {
		t.Fatalf("buildSeededTenantOrganization: %v", err)
	}
	if org.OwnerUserID != "old-owner@example.com" {
		t.Fatalf("org.OwnerUserID = %q, want %q", org.OwnerUserID, "old-owner@example.com")
	}
}

func TestBuildSeededTenantOrganization_PrefersExplicitFallbackOwner(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	p := newTestProvisioner(t, reg, tenantsDir, nil, true)

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

	createMember := func(email string, role registry.MemberRole, createdAt int64) {
		t.Helper()
		userID, err := registry.GenerateUserID()
		if err != nil {
			t.Fatalf("GenerateUserID: %v", err)
		}
		if err := reg.CreateUser(&registry.User{ID: userID, Email: email}); err != nil {
			t.Fatalf("CreateUser(%s): %v", email, err)
		}
		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    userID,
			Role:      role,
			CreatedAt: time.Unix(createdAt, 0).UTC(),
		}); err != nil {
			t.Fatalf("CreateMembership(%s): %v", email, err)
		}
	}

	createMember("legacy-owner@example.com", registry.MemberRoleOwner, 100)
	createMember("operator@example.com", registry.MemberRoleAdmin, 200)

	org, err := p.buildSeededTenantOrganization(accountID, "t-example", "Example", "operator@example.com")
	if err != nil {
		t.Fatalf("buildSeededTenantOrganization: %v", err)
	}
	if org.OwnerUserID != "operator@example.com" {
		t.Fatalf("org.OwnerUserID = %q, want %q", org.OwnerUserID, "operator@example.com")
	}
	if org.GetMemberRole("operator@example.com") != models.OrgRoleOwner {
		t.Fatalf("org role for operator@example.com = %q, want %q", org.GetMemberRole("operator@example.com"), models.OrgRoleOwner)
	}
}

func TestProvisionWorkspaceSeedsOrganizationMembershipsFromAccountMembers(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()

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
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:            accountID,
		PlanVersion:          "msp_starter",
		SubscriptionState:    "active",
		StripeCustomerID:     "cus_test_msp_members",
		StripeSubscriptionID: "sub_test_msp_members",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	createMember := func(email string, role registry.MemberRole) {
		t.Helper()
		userID, err := registry.GenerateUserID()
		if err != nil {
			t.Fatalf("GenerateUserID: %v", err)
		}
		if err := reg.CreateUser(&registry.User{ID: userID, Email: email}); err != nil {
			t.Fatalf("CreateUser(%s): %v", email, err)
		}
		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    userID,
			Role:      role,
		}); err != nil {
			t.Fatalf("CreateMembership(%s): %v", email, err)
		}
	}

	createMember("owner@acmemsp.com", registry.MemberRoleOwner)
	createMember("admin@acmemsp.com", registry.MemberRoleAdmin)
	createMember("tech@acmemsp.com", registry.MemberRoleTech)
	createMember("viewer@acmemsp.com", registry.MemberRoleReadOnly)

	p := newTestProvisioner(t, reg, tenantsDir, nil, true)
	tenant, err := p.ProvisionWorkspace(context.Background(), accountID, "Tenant One")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}

	mtp := config.NewMultiTenantPersistence(p.tenantDataDir(tenant.ID))
	org, err := mtp.LoadOrganizationStrict(tenant.ID)
	if err != nil {
		t.Fatalf("LoadOrganizationStrict(%s): %v", tenant.ID, err)
	}
	if org.OwnerUserID != "owner@acmemsp.com" {
		t.Fatalf("org.OwnerUserID = %q, want %q", org.OwnerUserID, "owner@acmemsp.com")
	}

	wantRoles := map[string]models.OrganizationRole{
		"owner@acmemsp.com":  models.OrgRoleOwner,
		"admin@acmemsp.com":  models.OrgRoleAdmin,
		"tech@acmemsp.com":   models.OrgRoleEditor,
		"viewer@acmemsp.com": models.OrgRoleViewer,
	}
	if len(org.Members) != len(wantRoles) {
		t.Fatalf("org members = %+v, want %d entries", org.Members, len(wantRoles))
	}
	for email, wantRole := range wantRoles {
		if got := org.GetMemberRole(email); got != wantRole {
			t.Fatalf("org role for %q = %q, want %q", email, got, wantRole)
		}
	}
}

func TestHandleCheckoutDockerlessFallbackWhenDockerDaemonUnavailable(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()

	p := newTestProvisioner(t, reg, tenantsDir, newFailingDockerManager(t), true)
	session := CheckoutSession{
		Customer:      "cus_dockerless_123",
		Subscription:  "sub_dockerless_123",
		CustomerEmail: "owner@example.com",
	}

	if err := p.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID(session.Customer)
	if err != nil {
		t.Fatalf("GetByStripeCustomerID: %v", err)
	}
	if tenant == nil {
		t.Fatal("expected tenant to be created")
	}
	if tenant.State != registry.TenantStateActive {
		t.Fatalf("state = %q, want %q", tenant.State, registry.TenantStateActive)
	}
	if tenant.ContainerID != "" {
		t.Fatalf("container id = %q, want empty in dockerless fallback mode", tenant.ContainerID)
	}
}
