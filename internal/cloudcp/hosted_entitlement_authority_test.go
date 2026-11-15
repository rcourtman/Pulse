package cloudcp

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestHostedRefreshMissingBillingPolicyFollowsConfiguredMode(t *testing.T) {
	for _, mode := range []ControlPlaneMode{ControlPlaneModePulseHosted, ControlPlaneModePulseHostedMSP, ControlPlaneModeProviderHostedMSP} {
		t.Run(string(mode), func(t *testing.T) {
			_, priv, err := ed25519.GenerateKey(nil)
			if err != nil {
				t.Fatal(err)
			}
			reg, err := registry.NewTenantRegistry(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = reg.Close() })
			if err := reg.CreateAccount(&registry.Account{ID: "a_LOCALMSP", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			if err := reg.Create(&registry.Tenant{ID: "t-LOCALMSP", AccountID: "a_LOCALMSP", State: registry.TenantStateActive, PlanVersion: "msp_starter"}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := reg.StoreOrIssueHostedEntitlement("t-LOCALMSP", "etr_mode_fixture", time.Now()); err != nil {
				t.Fatal(err)
			}
			svc := NewHostedEntitlementsService(&CPConfig{ControlPlaneMode: mode, BaseURL: "https://cloud.example.com", TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv)}, reg)
			request := httptest.NewRequest(http.MethodPost, "/api/entitlements/refresh", strings.NewReader(`{"org_id":"default","instance_host":"t-localmsp.cloud.example.com","entitlement_refresh_token":"etr_mode_fixture"}`))
			rec := httptest.NewRecorder()
			NewHostedEntitlementHandlers(svc).HandleRefresh(rec, request)
			want := http.StatusOK
			if mode == ControlPlaneModePulseHosted {
				want = http.StatusServiceUnavailable
			}
			if rec.Code != want {
				t.Fatalf("status=%d want=%d", rec.Code, want)
			}
			if want != http.StatusOK && strings.Contains(rec.Body.String(), "entitlement_jwt") {
				t.Fatal("unavailable authority returned a lease")
			}
		})
	}
}
