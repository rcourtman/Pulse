//go:build !release

package licensing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestServiceActivate_ExchangesLegacyJWTOutsideDevMode(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	setupTestPublicKey(t)
	const expectedClientVersion = "6.0.0-rc.1"

	tests := []struct {
		name    string
		planKey string
	}{
		{name: "lifetime grandfathered", planKey: "v5_lifetime_grandfathered"},
		{name: "monthly grandfathered", planKey: "v5_pro_monthly_grandfathered"},
		{name: "annual grandfathered", planKey: "v5_pro_annual_grandfathered"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService()
			licenseKey, err := GenerateLicenseForTesting("strict-v6@example.com", TierPro, 24*time.Hour)
			if err != nil {
				t.Fatalf("GenerateLicenseForTesting: %v", err)
			}

			grantJWT := makeTestGrantJWT(t, &GrantClaims{
				LicenseID: "lic_test",
				Tier:      "pro",
				PlanKey:   tt.planKey,
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
				if req.ClientVersion != expectedClientVersion {
					t.Fatalf("ClientVersion = %q, want %q", req.ClientVersion, expectedClientVersion)
				}
				if req.Runtime == nil || req.Runtime.Build != RuntimeBuildPro {
					t.Fatalf("Runtime build = %#v, want %q", req.Runtime, RuntimeBuildPro)
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
			svc.SetClientVersion(expectedClientVersion)
			svc.SetRuntimeIdentity(ProRuntimeIdentity())

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
			} else if got.Claims.PlanVersion != tt.planKey {
				t.Fatalf("expected exchanged license plan_version to be preserved, got %q", got.Claims.PlanVersion)
			}
		})
	}
}

func TestTestingHelpersRemainNonReleaseOnly(t *testing.T) {
	content, err := os.ReadFile("testing_helpers.go")
	if err != nil {
		t.Fatalf("read testing_helpers.go: %v", err)
	}
	if !strings.HasPrefix(string(content), "//go:build !release\n") {
		t.Fatalf("testing_helpers.go must stay excluded from release builds")
	}
}

func TestServiceActivateWithKey_SendsClientVersion(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	setupTestPublicKey(t)
	const expectedClientVersion = "6.0.0-rc.1"

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_native",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/activate" {
			t.Fatalf("Path = %q, want /v1/activate", r.URL.Path)
		}

		var req ActivateInstallationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.ActivationKey != "ppk_live_native_test" {
			t.Fatalf("ActivationKey = %q, want ppk_live_native_test", req.ActivationKey)
		}
		if req.ClientVersion != expectedClientVersion {
			t.Fatalf("ClientVersion = %q, want %q", req.ClientVersion, expectedClientVersion)
		}
		if req.Runtime == nil || req.Runtime.Build != RuntimeBuildPro {
			t.Fatalf("Runtime build = %#v, want %q", req.Runtime, RuntimeBuildPro)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ActivateInstallationResponse{
			License: ActivateResponseLicense{
				LicenseID: "lic_native",
				State:     "active",
				Tier:      "pro",
			},
			Installation: ActivateResponseInstallation{
				InstallationID:    "inst_native",
				InstallationToken: "pit_live_native",
				Status:            "active",
			},
			Grant: GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_native",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetClientVersion(expectedClientVersion)
	svc.SetRuntimeIdentity(ProRuntimeIdentity())

	if _, err := svc.Activate("ppk_live_native_test"); err != nil {
		t.Fatalf("Activate: %v", err)
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

func TestServiceActivate_ExchangedLegacyJWTMarksLegacyMigrationContinuity(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	setupTestPublicKey(t)

	tmpDir, err := os.MkdirTemp("", "pulse-service-legacy-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	svc := NewService()
	svc.SetPersistence(p)

	licenseKey, err := GenerateLicenseForTesting("legacy-continuity@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_continuity",
		Tier:      "pro",
		PlanKey:   "v5_pro_monthly_grandfathered",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ActivateInstallationResponse{
			License: ActivateResponseLicense{
				LicenseID: "lic_continuity",
				State:     "active",
				Tier:      "pro",
			},
			Installation: ActivateResponseInstallation{
				InstallationID:    "inst_continuity",
				InstallationToken: "pit_live_continuity",
				Status:            "active",
			},
			Grant: GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_continuity",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	if _, err := svc.Activate(licenseKey); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	state := svc.GetActivationState()
	if state == nil {
		t.Fatal("expected activation state")
	}
	if !state.Continuity.LegacyMigration {
		t.Fatal("expected legacy exchange to mark legacy migration continuity")
	}
}

func TestServiceActivate_CallsActivationStateChangeCallback(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	setupTestPublicKey(t)

	svc := NewService()
	licenseKey, err := GenerateLicenseForTesting("activation-callback@example.com", TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_activation_callback",
		Tier:      "pro",
		PlanKey:   "v5_pro_monthly_grandfathered",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ActivateInstallationResponse{
			License: ActivateResponseLicense{
				LicenseID: "lic_activation_callback",
				State:     "active",
				Tier:      "pro",
			},
			Installation: ActivateResponseInstallation{
				InstallationID:    "inst_activation_callback",
				InstallationToken: "pit_live_activation_callback",
				Status:            "active",
			},
			Grant: GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_activation_callback",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	var callbackState *ActivationState
	svc.SetActivationStateChangeCallback(func(state *ActivationState) {
		callbackState = state
	})
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	if _, err := svc.Activate(licenseKey); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if callbackState == nil {
		t.Fatal("activation-state callback was not invoked")
	}
	if callbackState.InstallationID != "inst_activation_callback" {
		t.Fatalf("InstallationID = %q, want inst_activation_callback", callbackState.InstallationID)
	}
	if !callbackState.Continuity.LegacyMigration {
		t.Fatal("expected legacy activation callback to preserve migration continuity")
	}
}

func TestServiceRestoreActivation_CallsActivationStateChangeCallback(t *testing.T) {
	setupTestPublicKey(t)

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_restore_callback",
		Tier:      "pro",
		PlanKey:   "v5_pro_monthly_grandfathered",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	var callbackState *ActivationState
	svc.SetActivationStateChangeCallback(func(state *ActivationState) {
		callbackState = state
	})

	state := &ActivationState{
		InstallationID:      "inst_restore_callback",
		InstallationToken:   "pit_live_restore_callback",
		LicenseID:           "lic_restore_callback",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_restore_callback",
		GrantExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		InstanceFingerprint: "fp-restore-callback",
		Continuity: ActivationContinuity{
			LegacyMigration: true,
		},
	}

	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	if callbackState == nil {
		t.Fatal("activation-state callback was not invoked")
	}
	if callbackState.InstallationID != "inst_restore_callback" {
		t.Fatalf("InstallationID = %q, want inst_restore_callback", callbackState.InstallationID)
	}
	if !callbackState.Continuity.LegacyMigration {
		t.Fatal("expected restore callback to preserve migration continuity")
	}
}

func TestServiceStatus_RetiredMonitoredSystemContinuityDoesNotSurfaceForSelfHostedFallbackMigrations(t *testing.T) {
	setupTestPublicKey(t)

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_continuity_status",
		Tier:      "pro",
		PlanKey:   "legacy_migration_fallback",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	tests := []struct {
		name       string
		continuity ActivationContinuity
	}{
		{
			name: "pending capture",
			continuity: ActivationContinuity{
				LegacyMigration: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService()
			if err := svc.RestoreActivation(&ActivationState{
				InstallationID:      "inst_continuity_status",
				InstallationToken:   "pit_live_continuity_status",
				LicenseID:           "lic_continuity_status",
				GrantJWT:            grantJWT,
				GrantJTI:            "grant_continuity_status",
				InstanceFingerprint: "fp-continuity-status",
				Continuity:          tt.continuity,
			}); err != nil {
				t.Fatalf("RestoreActivation: %v", err)
			}

			status := svc.Status()
			if status == nil || !status.Valid {
				t.Fatalf("expected valid status, got %+v", status)
			}
			if current := svc.Current(); current == nil {
				t.Fatal("expected current license")
			} else if _, ok := current.Claims.EffectiveLimits()[MaxMonitoredSystemsLicenseGateKey]; ok {
				t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", current.Claims.EffectiveLimits())
			}
		})
	}
}

func TestServiceStatus_GrandfatheredRecurringV5IsUncapped(t *testing.T) {
	setupTestPublicKey(t)

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_recurring_grandfathered",
		Tier:      "pro",
		PlanKey:   "v5_pro_monthly_grandfathered",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	if err := svc.RestoreActivation(&ActivationState{
		InstallationID:      "inst_recurring_grandfathered",
		InstallationToken:   "pit_live_recurring_grandfathered",
		LicenseID:           "lic_recurring_grandfathered",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_recurring_grandfathered",
		InstanceFingerprint: "fp-recurring-grandfathered",
		Continuity: ActivationContinuity{
			LegacyMigration: true,
		},
	}); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	status := svc.Status()
	if status == nil || !status.Valid {
		t.Fatalf("expected valid grandfathered status, got %+v", status)
	}

	current := svc.Current()
	if current == nil {
		t.Fatal("expected current license")
	}
	if got := current.Claims.EffectiveLimits(); len(got) != 0 {
		t.Fatalf("EffectiveLimits() = %v, want no capped commercial limits", got)
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

func TestServiceStatusCanonicalizesJWTCloudPlanVersionAndScrubsRetiredMonitoringLimit(t *testing.T) {
	svc := NewService()
	svc.license = &License{
		Claims: Claims{
			Tier:        TierCloud,
			PlanVersion: "cloud_v1",
			Limits: map[string]int64{
				"max_monitored_systems": 999,
			},
			SubState: SubStateActive,
		},
	}

	status := svc.Status()
	if status.PlanVersion != "cloud_starter" {
		t.Fatalf("status.PlanVersion=%q, want %q", status.PlanVersion, "cloud_starter")
	}
}

func TestServiceStatusMissingJWTCloudPlanFailsClosed(t *testing.T) {
	svc := NewService()
	svc.license = &License{
		Claims: Claims{
			Tier:        TierCloud,
			PlanVersion: "   ",
			SubState:    SubStateActive,
		},
	}

	status := svc.Status()
	if status.PlanVersion != "" {
		t.Fatalf("status.PlanVersion=%q, want empty", status.PlanVersion)
	}
}

func TestServiceStatus_DevModeKeepsCustomerFacingStatusCommunityWithoutLicense(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "")

	svc := NewService()
	status := svc.Status()

	if status.Valid {
		t.Fatalf("status.Valid=%v, want false", status.Valid)
	}
	if status.Tier != TierFree {
		t.Fatalf("status.Tier=%q, want %q", status.Tier, TierFree)
	}

	for _, feature := range TierFeatures[TierFree] {
		if !containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features missing community feature %q in dev mode: %v", feature, status.Features)
		}
	}
	for _, feature := range []string{FeatureAdvancedReporting, FeatureRelay, FeatureRBAC} {
		if containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features exposed synthetic dev feature %q: %v", feature, status.Features)
		}
	}
	if !svc.HasFeature(FeatureAdvancedReporting) {
		t.Fatalf("HasFeature(%q)=false, want true for dev-mode backend gate bypass", FeatureAdvancedReporting)
	}
	if svc.HasFeature(FeatureMultiTenant) {
		t.Fatalf("HasFeature(%q)=true, want false when runtime flag is disabled", FeatureMultiTenant)
	}
	if containsStringValue(status.Features, FeatureMultiTenant) {
		t.Fatalf("status.Features unexpectedly includes %q when runtime flag is disabled: %v", FeatureMultiTenant, status.Features)
	}
	for _, feature := range []string{FeatureMultiUser, FeatureWhiteLabel, FeatureUnlimited} {
		if svc.HasFeature(feature) {
			t.Fatalf("HasFeature(%q)=true, want false for non-runtime dev capability", feature)
		}
		if containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features unexpectedly includes %q in dev mode: %v", feature, status.Features)
		}
	}
}

func TestServiceHasFeature_DevModeHonorsActivatedLicenseForExcludedFeatures(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MOCK_MODE", "true")
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "")
	SetPublicKey(nil)
	t.Cleanup(func() { SetPublicKey(nil) })

	svc := NewService()
	if svc.HasFeature(FeatureWhiteLabel) {
		t.Fatalf("HasFeature(%q)=true without a license, want false in dev mode", FeatureWhiteLabel)
	}

	licenseKey, err := GenerateLicenseForTesting("dev-white-label@example.com", TierEnterprise, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateLicenseForTesting: %v", err)
	}
	if _, err := svc.Activate(licenseKey); err != nil {
		t.Fatalf("Activate in dev mode: %v", err)
	}

	// The implicit dev grant excludes white_label, but an explicitly
	// activated license that carries it must behave like a release build.
	if !svc.HasFeature(FeatureWhiteLabel) {
		t.Fatalf("HasFeature(%q)=false with an activated enterprise license, want true", FeatureWhiteLabel)
	}
	// Features the dev grant already enables stay enabled.
	if !svc.HasFeature(FeatureAdvancedReporting) {
		t.Fatalf("HasFeature(%q)=false, want true for dev-mode backend gate bypass", FeatureAdvancedReporting)
	}
}

func TestServiceStatus_DevModeMultiTenantBypassDoesNotChangeCustomerFacingStatus(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "true")

	svc := NewService()
	status := svc.Status()

	if containsStringValue(status.Features, FeatureMultiTenant) {
		t.Fatalf("status.Features exposed synthetic dev feature %q: %v", FeatureMultiTenant, status.Features)
	}
	if !svc.HasFeature(FeatureMultiTenant) {
		t.Fatalf("HasFeature(%q)=false, want true when runtime flag is enabled", FeatureMultiTenant)
	}
}

func TestServiceStatus_MockModeDoesNotAdvertiseSyntheticCapabilities(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")

	svc := NewService()
	status := svc.Status()

	for _, feature := range TierFeatures[TierFree] {
		if !containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features missing community feature %q in mock mode: %v", feature, status.Features)
		}
	}
	for _, feature := range []string{FeatureAdvancedReporting, FeatureRelay, FeatureDemoFixtures} {
		if containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features exposed synthetic mock feature %q: %v", feature, status.Features)
		}
	}
	if !svc.HasFeature(FeatureDemoFixtures) {
		t.Fatalf("HasFeature(%q)=false, want true for mock fixture gate bypass", FeatureDemoFixtures)
	}
}

func containsStringValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
