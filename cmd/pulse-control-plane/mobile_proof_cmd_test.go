package main

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestLooksLikeMobileProofAccount(t *testing.T) {
	tests := []struct {
		name    string
		account *registry.Account
		want    bool
	}{
		{
			name:    "nil",
			account: nil,
			want:    false,
		},
		{
			name: "explicit proof display",
			account: &registry.Account{
				ID:          "a_live",
				DisplayName: "Pulse Mobile Proof",
			},
			want: true,
		},
		{
			name: "synthetic msp proof id",
			account: &registry.Account{
				ID:          "a_msp_prod_fix_20260313111348",
				DisplayName: "Pulse",
			},
			want: true,
		},
		{
			name: "customer-shaped account",
			account: &registry.Account{
				ID:          "a_W3PT2W0YFR",
				DisplayName: "Customer Account",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeMobileProofAccount(tt.account); got != tt.want {
				t.Fatalf("looksLikeMobileProofAccount() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestLooksLikeMobileProofTenant(t *testing.T) {
	tests := []struct {
		name    string
		tenant  *registry.Tenant
		account *registry.Account
		want    bool
	}{
		{
			name:   "nil",
			tenant: nil,
			want:   false,
		},
		{
			name: "proof display",
			tenant: &registry.Tenant{
				ID:          "t-123",
				DisplayName: "Pulse Mobile GA Proof",
			},
			want: true,
		},
		{
			name: "proof account fallback",
			tenant: &registry.Tenant{
				ID:        "t-123",
				AccountID: "a_msp_prod_fix_20260313111348",
			},
			account: &registry.Account{
				ID: "a_msp_prod_fix_20260313111348",
			},
			want: true,
		},
		{
			name: "customer tenant",
			tenant: &registry.Tenant{
				ID:          "t-123",
				AccountID:   "a_customer",
				DisplayName: "Customer Workspace",
			},
			account: &registry.Account{
				ID:          "a_customer",
				DisplayName: "Customer Account",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeMobileProofTenant(tt.tenant, tt.account); got != tt.want {
				t.Fatalf("looksLikeMobileProofTenant() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestMobileProofBaseDomainFromURL(t *testing.T) {
	tests := map[string]string{
		"https://cloud.pulserelay.pro":       "cloud.pulserelay.pro",
		"http://cloud.pulserelay.pro:8080/x": "cloud.pulserelay.pro",
		"cloud.pulserelay.pro/path":          "cloud.pulserelay.pro",
		" cloud.pulserelay.pro ":             "cloud.pulserelay.pro",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := mobileProofBaseDomainFromURL(input); got != want {
				t.Fatalf("mobileProofBaseDomainFromURL(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestMobileProofCommandExposesAccountAndWorkspaceLifecycle(t *testing.T) {
	cmd := newMobileProofCmd()
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, name := range []string{"create-account", "create-workspace", "delete-workspace"} {
		if !found[name] {
			t.Fatalf("mobile-proof subcommand %q is not registered", name)
		}
	}
}
