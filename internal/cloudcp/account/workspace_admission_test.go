package account

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

func TestCreateWorkspaceUsesCurrentSubscriptionState(t *testing.T) {
	for _, state := range []string{"active", "trial", "past_due", "grace", "suspended", "expired", "canceled", "unexpected"} {
		t.Run(state, func(t *testing.T) {
			reg := newTestRegistry(t)
			dir := t.TempDir()
			if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: "a-admission", StripeCustomerID: "cus_admission", PlanVersion: "msp_growth", SubscriptionState: state}); err != nil {
				t.Fatal(err)
			}
			mux, _ := newTestTenantMux(t, reg, dir)
			rec := doRequest(t, mux, httptest.NewRequest(http.MethodPost, "/api/accounts/a-admission/tenants", bytes.NewBufferString(`{"display_name":"New Client"}`)))
			allowed := state == "active" || state == "trial" || state == "past_due" || state == "grace"
			if allowed {
				if rec.Code != http.StatusCreated {
					t.Fatalf("valid subscription refused: %d/%s", rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("inactive state admitted: %d/%s", rec.Code, rec.Body.String())
			}
			var payload tenantCreateErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Error != "subscription_inactive" && payload.Error != "subscription_canceled" {
				t.Fatalf("lost actionable policy error: %+v", payload)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refusal wrote tenant files: %+v/%v", entries, err)
			}
		})
	}
}

func TestCreateWorkspaceReturnsFinalAdmissionRefusalWithoutFallback(t *testing.T) {
	reg := newTestRegistry(t)
	dir := t.TempDir()
	if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
		t.Fatal(err)
	}
	sa := &registry.StripeAccount{AccountID: "a-admission", StripeCustomerID: "cus_admission", PlanVersion: "msp_growth", SubscriptionState: "active"}
	if err := reg.CreateStripeAccount(sa); err != nil {
		t.Fatal(err)
	}
	mux, _ := newTestTenantMuxWithWorkspaceLimitPolicy(t, reg, dir, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_max"}, cpstripe.WithAdmissionCheck(func(context.Context) error { sa.SubscriptionState = "suspended"; return reg.UpdateStripeAccount(sa) }))
	rec := doRequest(t, mux, httptest.NewRequest(http.MethodPost, "/api/accounts/a-admission/tenants", bytes.NewBufferString(`{"display_name":"New Client"}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("final refusal became success/generic failure: %d/%s", rec.Code, rec.Body.String())
	}
	var payload tenantCreateErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != "subscription_inactive" {
		t.Fatalf("final refusal reason lost: %+v", payload)
	}
	tenants, err := reg.ListByAccountID("a-admission")
	if err != nil || len(tenants) != 0 {
		t.Fatalf("refusal created workspace: %+v/%v", tenants, err)
	}
}
