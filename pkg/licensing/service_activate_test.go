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
			} else if got.Claims.PlanVersion != tt.planKey {
				t.Fatalf("expected exchanged license plan_version to be preserved, got %q", got.Claims.PlanVersion)
			}
		})
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
		LicenseID:           "lic_continuity",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("Path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ActivateInstallationResponse{
			License: ActivateResponseLicense{
				LicenseID:           "lic_continuity",
				State:               "active",
				Tier:                "pro",
				MaxMonitoredSystems: 10,
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
	if state.Continuity.GrandfatheredMonitoredSystemsCapturedAt != 0 {
		t.Fatalf("GrandfatheredMonitoredSystemsCapturedAt=%d, want 0 before capture", state.Continuity.GrandfatheredMonitoredSystemsCapturedAt)
	}
}

func TestServiceCaptureLegacyMonitoredSystemGrandfatherFloorPersistsAndUpdatesStatus(t *testing.T) {
	setupTestPublicKey(t)

	tmpDir, err := os.MkdirTemp("", "pulse-service-floor-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:           "lic_floor",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetPersistence(p)
	state := &ActivationState{
		InstallationID:      "inst_floor",
		InstallationToken:   "pit_live_floor",
		LicenseID:           "lic_floor",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_floor",
		InstanceFingerprint: "fp-floor",
		Continuity: ActivationContinuity{
			LegacyMigration: true,
		},
	}
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	if err := svc.CaptureLegacyMonitoredSystemGrandfatherFloor(23); err != nil {
		t.Fatalf("CaptureLegacyMonitoredSystemGrandfatherFloor: %v", err)
	}

	status := svc.Status()
	if status.MaxMonitoredSystems != 23 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want 23", status.MaxMonitoredSystems)
	}

	current := svc.Current()
	if current == nil {
		t.Fatal("expected current license")
	}
	if got := current.Claims.EffectiveLimits()[MaxMonitoredSystemsLicenseGateKey]; got != 23 {
		t.Fatalf("EffectiveLimits()[max_monitored_systems]=%d, want 23", got)
	}

	loaded, err := p.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected persisted activation state")
	}
	if loaded.Continuity.GrandfatheredMaxMonitoredSystems != 23 {
		t.Fatalf("GrandfatheredMaxMonitoredSystems=%d, want 23", loaded.Continuity.GrandfatheredMaxMonitoredSystems)
	}
	if loaded.Continuity.GrandfatheredMonitoredSystemsCapturedAt == 0 {
		t.Fatal("expected persisted capture timestamp")
	}
}

func TestServiceStatus_ExposesMonitoredSystemContinuity(t *testing.T) {
	setupTestPublicKey(t)

	grantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:           "lic_continuity_status",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
	})

	tests := []struct {
		name               string
		continuity         ActivationContinuity
		wantPlanLimit      int
		wantEffectiveLimit int
		wantFloor          int
		wantCapturePending bool
		wantMaxSystems     int
	}{
		{
			name: "pending capture",
			continuity: ActivationContinuity{
				LegacyMigration: true,
			},
			wantPlanLimit:      10,
			wantEffectiveLimit: 10,
			wantFloor:          0,
			wantCapturePending: true,
			wantMaxSystems:     10,
		},
		{
			name: "captured floor",
			continuity: ActivationContinuity{
				LegacyMigration:                         true,
				GrandfatheredMaxMonitoredSystems:        23,
				GrandfatheredMonitoredSystemsCapturedAt: 123,
			},
			wantPlanLimit:      10,
			wantEffectiveLimit: 23,
			wantFloor:          23,
			wantCapturePending: false,
			wantMaxSystems:     23,
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
			if status.MaxMonitoredSystems != tt.wantMaxSystems {
				t.Fatalf("status.MaxMonitoredSystems=%d, want %d", status.MaxMonitoredSystems, tt.wantMaxSystems)
			}
			if status.MonitoredSystemContinuity == nil {
				t.Fatal("expected monitored-system continuity in status")
			}
			if status.MonitoredSystemContinuity.PlanLimit != tt.wantPlanLimit {
				t.Fatalf("PlanLimit=%d, want %d", status.MonitoredSystemContinuity.PlanLimit, tt.wantPlanLimit)
			}
			if status.MonitoredSystemContinuity.EffectiveLimit != tt.wantEffectiveLimit {
				t.Fatalf("EffectiveLimit=%d, want %d", status.MonitoredSystemContinuity.EffectiveLimit, tt.wantEffectiveLimit)
			}
			if status.MonitoredSystemContinuity.GrandfatheredFloor != tt.wantFloor {
				t.Fatalf("GrandfatheredFloor=%d, want %d", status.MonitoredSystemContinuity.GrandfatheredFloor, tt.wantFloor)
			}
			if status.MonitoredSystemContinuity.CapturePending != tt.wantCapturePending {
				t.Fatalf("CapturePending=%v, want %v", status.MonitoredSystemContinuity.CapturePending, tt.wantCapturePending)
			}
		})
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
				"max_monitored_systems": 999,
			},
			SubState: SubStateActive,
		},
	}

	status := svc.Status()
	if status.PlanVersion != "cloud_starter" {
		t.Fatalf("status.PlanVersion=%q, want %q", status.PlanVersion, "cloud_starter")
	}
	if status.MaxMonitoredSystems != 10 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want %d", status.MaxMonitoredSystems, 10)
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
	if status.MaxMonitoredSystems != UnknownPlanDefaultMonitoredSystemLimit {
		t.Fatalf("status.MaxMonitoredSystems=%d, want %d", status.MaxMonitoredSystems, UnknownPlanDefaultMonitoredSystemLimit)
	}
}

func TestServiceStatus_DevModeAdvertisesOnlyRuntimeEnabledFeaturesWithoutLicense(t *testing.T) {
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

	for _, feature := range devModeFeatures() {
		if !containsStringValue(status.Features, feature) {
			t.Fatalf("status.Features missing %q in dev mode: %v", feature, status.Features)
		}
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

func TestServiceStatus_DevModeIncludesMultiTenantWhenRuntimeEnabled(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MULTI_TENANT_ENABLED", "true")

	svc := NewService()
	status := svc.Status()

	if !containsStringValue(status.Features, FeatureMultiTenant) {
		t.Fatalf("status.Features missing %q when runtime flag is enabled: %v", FeatureMultiTenant, status.Features)
	}
	if !svc.HasFeature(FeatureMultiTenant) {
		t.Fatalf("HasFeature(%q)=false, want true when runtime flag is enabled", FeatureMultiTenant)
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
