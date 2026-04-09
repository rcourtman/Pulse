package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type grandfatherFloorSupplementalProvider struct {
	settled bool
	readyAt time.Time
	records []unifiedresources.IngestRecord
}

func (p *grandfatherFloorSupplementalProvider) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	out := make([]unifiedresources.IngestRecord, len(p.records))
	copy(out, p.records)
	return out
}

func (p *grandfatherFloorSupplementalProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
}

func (p *grandfatherFloorSupplementalProvider) SupplementalInventoryReadyAt(*monitoring.Monitor, string) (time.Time, bool) {
	return p.readyAt, p.settled
}

func (p *grandfatherFloorSupplementalProvider) settle(count int) {
	now := time.Now().UTC()
	p.readyAt = now
	p.settled = true
	p.records = buildSupplementalGrandfatherFloorRecords(count, now)
}

func TestGetTenantComponents_AutoExchangesPersistedLegacyJWT(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_migrated",
		Tier:                "pro",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "user@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	var exchangeCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		exchangeCalled.Add(1)

		var req pkglicensing.ExchangeLegacyLicenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode exchange request: %v", err)
		}
		if req.LegacyLicenseKey == "" {
			t.Fatal("expected legacy license key in exchange request")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID:           "lic_migrated",
				State:               "active",
				Tier:                "pro",
				Features:            []string{"relay"},
				MaxMonitoredSystems: 10,
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_migrated",
				InstallationToken: "pit_live_migrated",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_migrated",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// Create and persist a legacy test JWT.
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	// Create handlers.
	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	if !svc.IsActivated() {
		t.Fatal("expected persisted legacy JWT to auto-exchange into activation state")
	}
	if exchangeCalled.Load() != 1 {
		t.Fatalf("expected one exchange call, got %d", exchangeCalled.Load())
	}
	if current := svc.Current(); current == nil || current.Claims.LicenseID != "lic_migrated" {
		t.Fatalf("expected migrated license to be active, got %#v", current)
	}
	if legacyLeft, err := persistence.Load(); err != nil {
		t.Fatalf("load preserved legacy JWT: %v", err)
	} else if legacyLeft != legacyJWT {
		t.Fatalf("expected migrated legacy JWT persistence to be preserved for downgrade, got %q", legacyLeft)
	}
	if activationState, err := persistence.LoadActivationState(); err != nil {
		t.Fatalf("load activation state: %v", err)
	} else if activationState == nil {
		t.Fatal("expected activation state after legacy exchange")
	}

	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_SkipsExchange_WhenActivationStateExists(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	// Mock server that should NOT be called if activation state exists.
	var exchangeCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/licenses/exchange" {
			exchangeCalled.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// Create both a legacy JWT and activation state on disk.
	// The activation state should take priority and no exchange should happen.
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}

	// Save a legacy JWT (shouldn't be used since activation state exists).
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	// Create a grant JWT for the activation state.
	grantClaims := map[string]any{
		"lid":                   "lic_existing",
		"tier":                  "pro",
		"st":                    "active",
		"feat":                  []string{"relay"},
		"max_monitored_systems": 10,
		"iat":                   time.Now().Unix(),
		"exp":                   time.Now().Add(72 * time.Hour).Unix(),
	}
	grantPayload, _ := json.Marshal(grantClaims)
	grantJWT := "eyJhbGciOiJFZERTQSJ9." + base64RawURLEncode(grantPayload) + "." + base64RawURLEncode([]byte("test-sig"))

	// Save activation state.
	activationState := &pkglicensing.ActivationState{
		InstallationID:      "inst_existing",
		InstallationToken:   "pit_live_existing",
		LicenseID:           "lic_existing",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_existing",
		GrantExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		InstanceFingerprint: "fp-existing",
		LicenseServerURL:    server.URL,
		ActivatedAt:         time.Now().Unix(),
		LastRefreshedAt:     time.Now().Unix(),
	}
	if err := persistence.SaveActivationState(activationState); err != nil {
		t.Fatalf("save activation state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	// Should have restored from activation state, NOT called exchange.
	// Give a brief moment for any potential background goroutine (shouldn't exist).
	time.Sleep(100 * time.Millisecond)
	if exchangeCalled.Load() != 0 {
		t.Error("exchange should NOT be called when activation state exists")
	}
	if !svc.IsActivated() {
		t.Error("expected IsActivated=true from restored activation state")
	}

	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_PersistsCommercialMigrationState_WhenAutoExchangeFails(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":      "service_unavailable",
			"message":   "exchange unavailable",
			"retryable": true,
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.IsActivated() {
		t.Fatal("expected activation to remain unset after failed exchange")
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.CommercialMigration == nil {
		t.Fatal("expected commercial migration state to be persisted")
	}
	if state.CommercialMigration.State != pkglicensing.CommercialMigrationStatePending {
		t.Fatalf("commercial_migration.state=%q, want %q", state.CommercialMigration.State, pkglicensing.CommercialMigrationStatePending)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handlers.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode entitlements payload: %v", err)
	}
	if payload.CommercialMigration == nil {
		t.Fatal("expected commercial_migration payload")
	}
	if payload.TrialEligible {
		t.Fatalf("trial_eligible=%v, want false", payload.TrialEligible)
	}
	if payload.TrialEligibilityReason != "commercial_migration_pending" {
		t.Fatalf("trial_eligibility_reason=%q, want %q", payload.TrialEligibilityReason, "commercial_migration_pending")
	}

	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_AutoExchangeGrandfathersObservedMonitoredSystems(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_floor_auto",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "floor-auto@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID:           "lic_floor_auto",
				State:               "active",
				Tier:                "pro",
				Features:            []string{"relay"},
				MaxMonitoredSystems: 10,
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_floor_auto",
				InstallationToken: "pit_live_floor_auto",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_floor_auto",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("floor-auto@example.com", pkglicensing.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy jwt: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	handlers.SetMonitors(buildGrandfatherFloorMonitor(23), nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if !svc.IsActivated() {
		t.Fatal("expected persisted legacy JWT to auto-exchange into activation state")
	}
	if got := svc.Status().MaxMonitoredSystems; got != 23 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want 23", got)
	}
	if activationState, err := persistence.LoadActivationState(); err != nil {
		t.Fatalf("load activation state: %v", err)
	} else if activationState == nil {
		t.Fatal("expected activation state after legacy exchange")
	} else {
		if !activationState.Continuity.LegacyMigration {
			t.Fatal("expected legacy migration continuity flag")
		}
		if activationState.Continuity.GrandfatheredMaxMonitoredSystems != 23 {
			t.Fatalf("GrandfatheredMaxMonitoredSystems=%d, want 23", activationState.Continuity.GrandfatheredMaxMonitoredSystems)
		}
		if activationState.Continuity.GrandfatheredMonitoredSystemsCapturedAt == 0 {
			t.Fatal("expected grandfather capture timestamp")
		}
	}

	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_BackfillsGrandfatherFloorAfterRestoreWhenMonitorArrivesLate(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_floor_restore",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "floor-restore@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.SaveActivationState(&pkglicensing.ActivationState{
		InstallationID:      "inst_floor_restore",
		InstallationToken:   "pit_live_floor_restore",
		LicenseID:           "lic_floor_restore",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_floor_restore",
		GrantExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		InstanceFingerprint: "fp-floor-restore",
		LicenseServerURL:    "https://license.pulserelay.pro",
		ActivatedAt:         time.Now().Add(-time.Hour).Unix(),
		LastRefreshedAt:     time.Now().Add(-time.Hour).Unix(),
		Continuity: pkglicensing.ActivationContinuity{
			LegacyMigration: true,
		},
	}); err != nil {
		t.Fatalf("save activation state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if got := svc.Status().MaxMonitoredSystems; got != 10 {
		t.Fatalf("initial status.MaxMonitoredSystems=%d, want 10 before floor capture", got)
	}

	handlers.SetMonitors(buildGrandfatherFloorMonitor(23), nil)

	deadline := time.Now().Add(8 * time.Second)
	for {
		loaded, err := persistence.LoadActivationState()
		if err != nil {
			t.Fatalf("load activation state: %v", err)
		}
		if loaded != nil &&
			loaded.Continuity.GrandfatheredMaxMonitoredSystems == 23 &&
			loaded.Continuity.GrandfatheredMonitoredSystemsCapturedAt != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected async grandfather capture after late monitor restore, last activation state=%+v", loaded)
		}
		time.Sleep(100 * time.Millisecond)
	}

	loaded, err := persistence.LoadActivationState()
	if err != nil {
		t.Fatalf("load activation state: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected activation state")
	}
	if loaded.Continuity.GrandfatheredMaxMonitoredSystems != 23 {
		t.Fatalf("GrandfatheredMaxMonitoredSystems=%d, want 23", loaded.Continuity.GrandfatheredMaxMonitoredSystems)
	}
	if loaded.Continuity.GrandfatheredMonitoredSystemsCapturedAt == 0 {
		t.Fatal("expected captured timestamp after late monitor restore")
	}
	if got := svc.Status().MaxMonitoredSystems; got != 23 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want 23 after late monitor capture", got)
	}

	handlers.StopAllBackgroundLoops()
}

func TestBillingReads_DoNotRestartLegacyGrandfatherReconcileLoop(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_floor_read_only",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "floor-read-only@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.SaveActivationState(&pkglicensing.ActivationState{
		InstallationID:      "inst_floor_read_only",
		InstallationToken:   "pit_live_floor_read_only",
		LicenseID:           "lic_floor_read_only",
		GrantJWT:            grantJWT,
		GrantJTI:            "grant_floor_read_only",
		GrantExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		InstanceFingerprint: "fp-floor-read-only",
		LicenseServerURL:    "https://license.pulserelay.pro",
		ActivatedAt:         time.Now().Add(-time.Hour).Unix(),
		LastRefreshedAt:     time.Now().Add(-time.Hour).Unix(),
		Continuity: pkglicensing.ActivationContinuity{
			LegacyMigration: true,
		},
	}); err != nil {
		t.Fatalf("save activation state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if value, ok := handlers.legacyGrandfatherReconcile.Load("default"); ok {
			if loop, ok := value.(*legacyGrandfatherReconcileLoop); ok && loop.isRunning() {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("expected restore-owned grandfather reconcile loop to start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	handlers.stopLegacyGrandfatherReconcileLoop("default")
	if _, ok := handlers.legacyGrandfatherReconcile.Load("default"); ok {
		t.Fatal("expected grandfather reconcile loop to stop before read-only check")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handlers.HandleLicenseStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("license status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	time.Sleep(100 * time.Millisecond)
	if _, ok := handlers.legacyGrandfatherReconcile.Load("default"); ok {
		t.Fatal("license status read restarted grandfather reconcile loop")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	handlers.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("license entitlements=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	time.Sleep(100 * time.Millisecond)
	if _, ok := handlers.legacyGrandfatherReconcile.Load("default"); ok {
		t.Fatal("license entitlements read restarted grandfather reconcile loop")
	}
}

func TestActivateLicenseKey_GrandfathersObservedMonitoredSystemsForLegacyMigration(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_floor_manual",
		Tier:                "pro",
		PlanKey:             "v5_pro_annual_grandfathered",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "floor-manual@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID:           "lic_floor_manual",
				State:               "active",
				Tier:                "pro",
				Features:            []string{"relay"},
				MaxMonitoredSystems: 10,
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_floor_manual",
				InstallationToken: "pit_live_floor_manual",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_floor_manual",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	handlers := createTestHandler(t)
	handlers.SetMonitors(buildGrandfatherFloorMonitor(23), nil)
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("floor-manual@example.com", pkglicensing.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}

	resp, err := handlers.activateLicenseKey(ctx, legacyJWT)
	if err != nil {
		t.Fatalf("activateLicenseKey: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}

	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if got := svc.Status().MaxMonitoredSystems; got != 23 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want 23", got)
	}
}

func TestGetTenantComponents_DelaysGrandfatherFloorUntilSupplementalInventorySettles(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID:           "lic_floor_supplemental",
		Tier:                "pro",
		PlanKey:             "v5_pro_monthly_grandfathered",
		State:               "active",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
		Email:               "floor-supplemental@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID:           "lic_floor_supplemental",
				State:               "active",
				Tier:                "pro",
				Features:            []string{"relay"},
				MaxMonitoredSystems: 10,
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_floor_supplemental",
				InstallationToken: "pit_live_floor_supplemental",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_floor_supplemental",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("floor-supplemental@example.com", pkglicensing.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy jwt: %v", err)
	}

	provider := &grandfatherFloorSupplementalProvider{}
	monitor := buildSupplementalGrandfatherFloorMonitor(provider)

	handlers := NewLicenseHandlers(mtp, false)
	handlers.SetMonitors(monitor, nil)
	t.Cleanup(handlers.StopAllBackgroundLoops)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	readStatus := func() pkglicensing.LicenseStatus {
		req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handlers.HandleLicenseStatus(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("license status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var status pkglicensing.LicenseStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode license status: %v", err)
		}
		return status
	}
	readEntitlements := func() EntitlementPayload {
		req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handlers.HandleEntitlements(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("license entitlements=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload EntitlementPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode entitlements: %v", err)
		}
		return payload
	}

	status := readStatus()
	if got := status.MaxMonitoredSystems; got != 10 {
		t.Fatalf("initial status.MaxMonitoredSystems=%d, want 10 while supplemental inventory is unsettled", got)
	}
	if status.MonitoredSystemContinuity == nil || !status.MonitoredSystemContinuity.CapturePending {
		t.Fatalf("expected pending continuity in status payload, got %+v", status.MonitoredSystemContinuity)
	}
	if status.MonitoredSystemContinuity.PlanLimit != 10 || status.MonitoredSystemContinuity.EffectiveLimit != 10 {
		t.Fatalf("unexpected pending continuity status payload: %+v", status.MonitoredSystemContinuity)
	}

	payload := readEntitlements()
	if payload.MonitoredSystemContinuity == nil || !payload.MonitoredSystemContinuity.CapturePending {
		t.Fatalf("expected pending continuity in entitlements payload, got %+v", payload.MonitoredSystemContinuity)
	}
	if len(payload.Limits) != 1 {
		t.Fatalf("expected one monitored-system limit, got %+v", payload.Limits)
	}
	if payload.Limits[0].CurrentAvailable == nil || *payload.Limits[0].CurrentAvailable {
		t.Fatalf("expected unavailable monitored-system usage while supplemental inventory is unsettled, got %+v", payload.Limits[0])
	}
	if payload.Limits[0].CurrentUnavailableReason != "supplemental_inventory_unsettled" {
		t.Fatalf("CurrentUnavailableReason=%q, want %q", payload.Limits[0].CurrentUnavailableReason, "supplemental_inventory_unsettled")
	}

	activationState, err := persistence.LoadActivationState()
	if err != nil {
		t.Fatalf("load activation state: %v", err)
	}
	if activationState == nil {
		t.Fatal("expected activation state after legacy exchange")
	}
	if activationState.Continuity.GrandfatheredMonitoredSystemsCapturedAt != 0 {
		t.Fatalf("GrandfatheredMonitoredSystemsCapturedAt=%d, want 0 before supplemental baseline settles", activationState.Continuity.GrandfatheredMonitoredSystemsCapturedAt)
	}

	provider.settle(23)
	status = readStatus()
	if got := status.MaxMonitoredSystems; got != 10 {
		t.Fatalf("stale supplemental store should not capture grandfather floor yet, got %d", got)
	}

	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	status = readStatus()
	if got := status.MaxMonitoredSystems; got != 10 {
		t.Fatalf("status read should not capture grandfather floor directly after canonical store rebuild, got %d", got)
	}

	deadline := time.Now().Add(8 * time.Second)
	for {
		activationState, err = persistence.LoadActivationState()
		if err != nil {
			t.Fatalf("reload activation state: %v", err)
		}
		if activationState != nil &&
			activationState.Continuity.GrandfatheredMaxMonitoredSystems == 23 &&
			activationState.Continuity.GrandfatheredMonitoredSystemsCapturedAt != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected async grandfather capture after canonical store rebuild, last activation state=%+v", activationState)
		}
		time.Sleep(100 * time.Millisecond)
	}

	status = readStatus()
	if got := status.MaxMonitoredSystems; got != 23 {
		t.Fatalf("status.MaxMonitoredSystems=%d, want 23 after async grandfather capture", got)
	}
	if status.MonitoredSystemContinuity == nil || status.MonitoredSystemContinuity.CapturePending {
		t.Fatalf("expected settled continuity in status payload after async capture, got %+v", status.MonitoredSystemContinuity)
	}
	if status.MonitoredSystemContinuity.GrandfatheredFloor != 23 || status.MonitoredSystemContinuity.EffectiveLimit != 23 {
		t.Fatalf("unexpected settled continuity status payload: %+v", status.MonitoredSystemContinuity)
	}

	payload = readEntitlements()
	if payload.MonitoredSystemContinuity == nil || payload.MonitoredSystemContinuity.CapturePending {
		t.Fatalf("expected settled continuity in entitlements payload after async capture, got %+v", payload.MonitoredSystemContinuity)
	}
	if len(payload.Limits) != 1 || payload.Limits[0].Current != 23 {
		t.Fatalf("expected settled monitored-system usage in entitlements payload, got %+v", payload.Limits)
	}
	if payload.Limits[0].CurrentAvailable == nil || !*payload.Limits[0].CurrentAvailable {
		t.Fatalf("expected available monitored-system usage after async capture, got %+v", payload.Limits[0])
	}
}

func buildGrandfatherFloorMonitor(count int) *monitoring.Monitor {
	now := time.Now().UTC()
	registry := unifiedresources.NewRegistry(nil)
	records := make([]unifiedresources.IngestRecord, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("host-%02d", i+1)
		hostname := fmt.Sprintf("legacy-%02d.lab.local", i+1)
		machineID := fmt.Sprintf("machine-%02d", i+1)
		records = append(records, unifiedresources.IngestRecord{
			SourceID: id,
			Resource: unifiedresources.Resource{
				ID:        id,
				Type:      unifiedresources.ResourceTypeAgent,
				Name:      hostname,
				Status:    unifiedresources.StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-" + id,
					Hostname:  hostname,
					MachineID: machineID,
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: machineID,
					Hostnames: []string{hostname},
				},
			},
		})
	}
	registry.IngestRecords(unifiedresources.SourceAgent, records)

	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	return monitor
}

func buildSupplementalGrandfatherFloorMonitor(provider *grandfatherFloorSupplementalProvider) *monitoring.Monitor {
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(nil))
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	return monitor
}

func buildSupplementalGrandfatherFloorRecords(
	count int,
	now time.Time,
) []unifiedresources.IngestRecord {
	records := make([]unifiedresources.IngestRecord, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("truenas-%02d", i+1)
		hostname := fmt.Sprintf("truenas-%02d.lab.local", i+1)
		records = append(records, unifiedresources.IngestRecord{
			SourceID: "system:" + id,
			Resource: unifiedresources.Resource{
				ID:        id,
				Type:      unifiedresources.ResourceTypeAgent,
				Name:      hostname,
				Status:    unifiedresources.StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Identity: unifiedresources.ResourceIdentity{
					Hostnames: []string{hostname},
				},
				TrueNAS: &unifiedresources.TrueNASData{
					Hostname: hostname,
				},
			},
		})
	}
	return records
}

// base64RawURLEncode is a helper for tests.
func base64RawURLEncode(data []byte) string {
	// Use the same encoding as JWT: base64 URL-safe without padding.
	const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 0, (len(data)*4+2)/3)
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		result = append(result, encodeURL[b0>>2])
		result = append(result, encodeURL[((b0&0x03)<<4)|(b1>>4)])
		if i+1 < len(data) {
			result = append(result, encodeURL[((b1&0x0f)<<2)|(b2>>6)])
		}
		if i+2 < len(data) {
			result = append(result, encodeURL[b2&0x3f])
		}
	}
	return string(result)
}
