package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

// TestGetTenantComponents_RetriesFailedExchangeInBackground covers the v5→v6
// upgrade path where the license server is unreachable at first boot: the
// startup exchange fails, and the background retry must complete the
// migration without a manual restart.
func TestGetTenantComponents_RetriesFailedExchangeInBackground(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	grantJWT, grantPublicKey, err := licensetestsupport.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_retry",
		Tier:      "pro",
		State:     "active",
		Features:  []string{"relay"},
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "retry@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	// Fail the first two exchange attempts with a retryable error, then
	// succeed: boot fails, retry #1 fails, retry #2 migrates.
	var exchangeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Errorf("path = %q, want /v1/licenses/exchange", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if exchangeCalls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":      "service_unavailable",
				"message":   "exchange unavailable",
				"retryable": true,
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID: "lic_retry",
				State:     "active",
				Tier:      "pro",
				Features:  []string{"relay"},
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_retry",
				InstallationToken: "pit_live_retry",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_retry",
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

	legacyJWT, err := licensetestsupport.GenerateLicenseForTesting("retry@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	handlers.legacyExchangeRetrySchedule = []time.Duration{10 * time.Millisecond}
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.IsActivated() {
		t.Fatal("expected startup exchange to fail before background retry")
	}

	deadline := time.Now().Add(5 * time.Second)
	for !svc.IsActivated() {
		if time.Now().After(deadline) {
			t.Fatalf("background retry never completed migration; exchange calls = %d", exchangeCalls.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if calls := exchangeCalls.Load(); calls != 3 {
		t.Errorf("exchange calls = %d, want 3 (boot + 2 retries)", calls)
	}
	if current := svc.Current(); current == nil || current.Claims.LicenseID != "lic_retry" {
		t.Fatalf("expected migrated license to be active, got %#v", current)
	}

	// Migration state must clear once the background retry succeeds.
	store := config.NewFileBillingStore(baseDir)
	waitForClearedMigration := time.Now().Add(2 * time.Second)
	for {
		state, err := store.GetBillingState("default")
		if err != nil {
			t.Fatalf("GetBillingState: %v", err)
		}
		if state == nil || state.CommercialMigration == nil {
			break
		}
		if time.Now().After(waitForClearedMigration) {
			t.Fatalf("commercial migration state never cleared: %+v", state.CommercialMigration)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestGetTenantComponents_StopsRetryingOnTerminalExchangeFailure pins that a
// terminal classification (e.g. 401 invalid key) halts the retry loop instead
// of hammering the license server forever.
func TestGetTenantComponents_StopsRetryingOnTerminalExchangeFailure(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	var exchangeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls := exchangeCalls.Add(1)
		if calls == 1 {
			// Retryable boot failure schedules the loop.
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":      "service_unavailable",
				"message":   "exchange unavailable",
				"retryable": true,
			})
			return
		}
		// Terminal failure on retry.
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    "invalid_license",
			"message": "license key rejected",
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

	legacyJWT, err := licensetestsupport.GenerateLicenseForTesting("terminal@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	handlers.legacyExchangeRetrySchedule = []time.Duration{10 * time.Millisecond}
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	if svc := handlers.Service(ctx); svc == nil {
		t.Fatal("expected non-nil service")
	}

	// Wait for the loop to hit the terminal failure and exit.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, running := handlers.legacyExchangeRetries.Load("default"); !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("retry loop still running after terminal failure")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if calls := exchangeCalls.Load(); calls != 2 {
		t.Errorf("exchange calls = %d, want 2 (boot + 1 terminal retry)", calls)
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.CommercialMigration == nil {
		t.Fatal("expected terminal commercial migration state to persist")
	}
	if state.CommercialMigration.State != pkglicensing.CommercialMigrationStateFailed {
		t.Fatalf("commercial_migration.state=%q, want %q", state.CommercialMigration.State, pkglicensing.CommercialMigrationStateFailed)
	}

	// No further calls should arrive after the loop stopped.
	time.Sleep(100 * time.Millisecond)
	if calls := exchangeCalls.Load(); calls != 2 {
		t.Errorf("exchange calls after stop = %d, want 2", calls)
	}
}

// TestGetTenantComponents_SurfacesUnreadablePersistedLicense pins that a
// license.enc that exists but cannot be decrypted produces a commercial
// migration notice instead of a silent downgrade to Community.
func TestGetTenantComponents_SurfacesUnreadablePersistedLicense(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// A license.enc sealed under key material this install cannot derive
	// (simulates a v5.0.0-era file whose machine-id material is gone).
	if err := os.MkdirAll(cp.GetConfigDir(), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	garbage := []byte("bm90LWEtcmVhbC1jaXBoZXJ0ZXh0LWJ1dC1iYXNlNjQtZGVjb2RhYmxl")
	if err := os.WriteFile(filepath.Join(cp.GetConfigDir(), pkglicensing.LicenseFileName), garbage, 0o600); err != nil {
		t.Fatalf("write undecryptable license file: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.IsActivated() {
		t.Fatal("expected no activation from an unreadable license file")
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.CommercialMigration == nil {
		t.Fatal("expected commercial migration state for unreadable persisted license")
	}
	if state.CommercialMigration.State != pkglicensing.CommercialMigrationStateFailed {
		t.Fatalf("commercial_migration.state=%q, want %q", state.CommercialMigration.State, pkglicensing.CommercialMigrationStateFailed)
	}
	if state.CommercialMigration.Reason != pkglicensing.CommercialMigrationReasonPersistedUnreadable {
		t.Fatalf("commercial_migration.reason=%q, want %q", state.CommercialMigration.Reason, pkglicensing.CommercialMigrationReasonPersistedUnreadable)
	}
}
