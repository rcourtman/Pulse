package licensing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceActivate_ExchangesLegacyJWTOutsideDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	setupTestPublicKey(t)

	svc := NewService()
	licenseKey, err := GenerateLicenseForTesting("strict-v6@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_test",
		Tier:      "pro",
		PlanKey:   "v5_lifetime_grandfathered",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "strict-v6@example.com",
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
		}

		var req ExchangeLegacyLicenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.LegacyLicenseKey != licenseKey {
			t.Fatalf("LegacyLicenseKey = %q, want %q", req.LegacyLicenseKey, licenseKey)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ActivateInstallationResponse{
			License: ActivateResponseLicense{
				LicenseID: "lic_test",
				State:     "active",
				Tier:      "pro",
			},
			Installation: ActivateResponseInstallation{
				InstallationID:    "inst_test",
				InstallationToken: "pit_live_test",
				Status:            "active",
			},
			Grant: GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_test",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	lic, err := svc.Activate(licenseKey)
	if err != nil {
		t.Fatalf("expected legacy JWT exchange in strict v6 mode, got %v", err)
	}
	if lic == nil {
		t.Fatal("expected non-nil license after exchange")
	}
	if !svc.IsActivated() {
		t.Fatal("expected strict v6 legacy exchange to produce activation state")
	}
	if got := svc.Current(); got == nil || got.Claims.LicenseID != "lic_test" {
		t.Fatalf("expected exchanged license to be active, got %#v", got)
	} else if got.Claims.PlanVersion != "v5_lifetime_grandfathered" {
		t.Fatalf("expected exchanged license plan_version to be preserved, got %q", got.Claims.PlanVersion)
	}
}

func TestServiceActivate_AllowsLegacyJWTInDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	svc := NewService()
	licenseKey, err := GenerateLicenseForTesting("dev-jwt@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	lic, err := svc.Activate(licenseKey)
	if err != nil {
		t.Fatalf("Activate in dev mode: %v", err)
	}
	if lic == nil {
		t.Fatal("expected non-nil license in dev mode")
	}
	if svc.IsActivated() {
		t.Fatal("expected dev JWT activation to remain non-activation-key mode")
	}
}

func TestServiceActivate_RejectsMalformedLegacyKeyOutsideDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	svc := NewService()
	_, err := svc.Activate("not-a-jwt-or-activation-key")
	if err == nil {
		t.Fatal("expected malformed legacy key rejection in strict v6 mode")
	}
	if !strings.Contains(err.Error(), "activation key") {
		t.Fatalf("expected activation-key guidance error, got %q", err.Error())
	}
}

func TestServiceStatusCanonicalizesJWTCloudPlanVersionAndLimits(t *testing.T) {
	svc := NewService()
	svc.license = &License{
		Claims: Claims{
			Tier:        TierCloud,
			PlanVersion: "cloud_v1",
			Limits: map[string]int64{
				"max_agents": 999,
			},
			SubState: SubStateActive,
		},
	}

	status := svc.Status()
	if status.PlanVersion != "cloud_starter" {
		t.Fatalf("status.PlanVersion=%q, want %q", status.PlanVersion, "cloud_starter")
	}
	if status.MaxAgents != 10 {
		t.Fatalf("status.MaxAgents=%d, want %d", status.MaxAgents, 10)
	}
}
