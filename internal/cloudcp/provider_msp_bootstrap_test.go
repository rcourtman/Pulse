package cloudcp

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestBootstrapProviderMSPCreatesOwnerAccountAndMagicLink(t *testing.T) {
	cfg := testProviderMSPBootstrapConfig(t)

	result, err := BootstrapProviderMSP(context.Background(), cfg, ProviderMSPBootstrapOptions{
		AccountName:       "Acme MSP",
		OwnerEmail:        "OWNER@Example.COM",
		GenerateMagicLink: true,
	})
	if err != nil {
		t.Fatalf("BootstrapProviderMSP: %v", err)
	}
	if result.AccountID == "" {
		t.Fatal("AccountID = empty")
	}
	if result.AccountName != "Acme MSP" {
		t.Fatalf("AccountName = %q, want Acme MSP", result.AccountName)
	}
	if result.OwnerEmail != "owner@example.com" {
		t.Fatalf("OwnerEmail = %q, want owner@example.com", result.OwnerEmail)
	}
	if result.PlanVersion != "msp_growth" {
		t.Fatalf("PlanVersion = %q, want msp_growth", result.PlanVersion)
	}
	if result.PlanSource != ProviderMSPPlanSourceLicenseFile {
		t.Fatalf("PlanSource = %q, want %q", result.PlanSource, ProviderMSPPlanSourceLicenseFile)
	}
	if result.LicenseID != "lic_provider_msp_test" {
		t.Fatalf("LicenseID = %q, want lic_provider_msp_test", result.LicenseID)
	}
	if result.LicenseEmail != "provider@example.com" {
		t.Fatalf("LicenseEmail = %q, want provider@example.com", result.LicenseEmail)
	}
	if result.WorkspaceLimit != 15 {
		t.Fatalf("WorkspaceLimit = %d, want 15", result.WorkspaceLimit)
	}
	if !strings.HasPrefix(result.MagicLinkURL, "https://msp.example.com/auth/magic-link/verify?token=ml1_") {
		t.Fatalf("MagicLinkURL = %q", result.MagicLinkURL)
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	defer reg.Close()

	account, err := reg.GetAccount(result.AccountID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if account == nil || account.Kind != registry.AccountKindMSP {
		t.Fatalf("account = %#v, want MSP account", account)
	}
	user, err := reg.GetUserByEmail("owner@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user == nil || user.ID != result.OwnerUserID {
		t.Fatalf("user = %#v, result owner user id = %q", user, result.OwnerUserID)
	}
	membership, err := reg.GetMembership(result.AccountID, result.OwnerUserID)
	if err != nil {
		t.Fatalf("GetMembership: %v", err)
	}
	if membership == nil || membership.Role != registry.MemberRoleOwner {
		t.Fatalf("membership = %#v, want owner", membership)
	}
}

func TestBootstrapProviderMSPIsIdempotentForExistingMSPAccount(t *testing.T) {
	cfg := testProviderMSPBootstrapConfig(t)

	first, err := BootstrapProviderMSP(context.Background(), cfg, ProviderMSPBootstrapOptions{
		AccountName: "Acme MSP",
		OwnerEmail:  "owner@example.com",
	})
	if err != nil {
		t.Fatalf("first BootstrapProviderMSP: %v", err)
	}
	second, err := BootstrapProviderMSP(context.Background(), cfg, ProviderMSPBootstrapOptions{
		AccountName: "Acme MSP",
		OwnerEmail:  "owner@example.com",
	})
	if err != nil {
		t.Fatalf("second BootstrapProviderMSP: %v", err)
	}
	if second.AccountID != first.AccountID {
		t.Fatalf("AccountID changed from %q to %q", first.AccountID, second.AccountID)
	}
	if second.OwnerUserID != first.OwnerUserID {
		t.Fatalf("OwnerUserID changed from %q to %q", first.OwnerUserID, second.OwnerUserID)
	}
}

func TestBootstrapProviderMSPRejectsPulseHostedMode(t *testing.T) {
	cfg := testProviderMSPBootstrapConfig(t)
	cfg.ControlPlaneMode = ControlPlaneModePulseHosted

	_, err := BootstrapProviderMSP(context.Background(), cfg, ProviderMSPBootstrapOptions{
		AccountName: "Acme MSP",
		OwnerEmail:  "owner@example.com",
	})
	if err == nil {
		t.Fatal("expected provider mode error")
	}
	if !strings.Contains(err.Error(), "provider MSP bootstrap requires") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testProviderMSPBootstrapConfig(t *testing.T) *CPConfig {
	t.Helper()
	return &CPConfig{
		DataDir:                 t.TempDir(),
		Environment:             "production",
		ControlPlaneMode:        ControlPlaneModeProviderHostedMSP,
		BaseURL:                 "https://msp.example.com",
		ProviderMSPPlanVersion:  "msp_growth",
		ProviderMSPPlanSource:   ProviderMSPPlanSourceLicenseFile,
		ProviderMSPLicenseID:    "lic_provider_msp_test",
		ProviderMSPLicenseEmail: "Provider@Example.com",
	}
}
