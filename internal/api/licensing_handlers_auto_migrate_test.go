package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestGetTenantComponents_AutoMigratesLegacyJWT(t *testing.T) {
	// Enable dev mode so legacy JWT validation skips signature check.
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	// Generate a test grant JWT that the mock exchange server will return.
	grantClaims := map[string]any{
		"iss":        "pulse-license-server",
		"aud":        "pulse-relay",
		"lid":        "lic_migrated",
		"iid":        "inst_migrated",
		"lv":         1,
		"st":         "active",
		"tier":       "pro",
		"plan":       "pro_monthly",
		"feat":       []string{"relay", "ai_patrol"},
		"max_agents": 15,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(72 * time.Hour).Unix(),
		"jti":        "grant_migrated",
	}
	grantPayload, _ := json.Marshal(grantClaims)

	// Build a minimal JWT (header.payload.sig) — signature isn't verified for grants.
	header := "eyJhbGciOiJFZERTQSJ9" // base64url({"alg":"EdDSA"})
	payload := base64RawURLEncode(grantPayload)
	grantJWT := header + "." + payload + "." + base64RawURLEncode([]byte("test-sig"))

	// Mock license server exchange endpoint.
	var exchangeCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/licenses/exchange" && r.Method == http.MethodPost {
			exchangeCalled.Add(1)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(pkglicensing.ExchangeLegacyResponse{
				Migration: pkglicensing.ExchangeMigrationInfo{
					Source:              "legacy_jwt",
					LegacyLID:           "old_lid",
					ResolvedV6LicenseID: "lic_migrated",
				},
				Installation: pkglicensing.ExchangeInstallation{
					InstallationID:    "inst_migrated",
					InstallationToken: "pit_live_migrated",
					Status:            "active",
				},
				Grant: pkglicensing.GrantEnvelope{
					JWT:       grantJWT,
					JTI:       "grant_migrated",
					ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
				},
				RefreshPolicy: pkglicensing.RefreshHints{
					IntervalSeconds: 21600,
					JitterPercent:   20,
				},
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Override the license server URL to point at the mock.
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	// Set up multi-tenant persistence with a legacy JWT on disk.
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	// Create a legacy test JWT.
	legacyJWT, err := pkglicensing.GenerateLicenseForTesting("user@example.com", pkglicensing.TierPro, 365*24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}

	// Persist the legacy JWT via the normal persistence path.
	cp, _ := mtp.GetPersistence("default")
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy JWT: %v", err)
	}

	// Create handlers — the first Service() call triggers getTenantComponents().
	// The legacy JWT is loaded synchronously; the exchange happens in the background.
	handlers := NewLicenseHandlers(mtp, false)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	svc := handlers.Service(ctx)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	// The service should have the legacy JWT immediately (synchronous path).
	lic := svc.Current()
	if lic == nil {
		t.Fatal("expected non-nil license immediately from legacy JWT")
	}
	if lic.Claims.Tier != pkglicensing.TierPro {
		t.Errorf("immediate Tier = %q, want pro", lic.Claims.Tier)
	}

	// Wait for the background exchange goroutine to complete.
	deadline := time.After(5 * time.Second)
	for exchangeCalled.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for background exchange")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Give a brief moment for the exchange result to be applied.
	time.Sleep(50 * time.Millisecond)

	// Verify the service is now on an activation-key license.
	if !svc.IsActivated() {
		t.Error("expected IsActivated=true after auto-migration")
	}

	// Verify the license tier from the grant.
	lic = svc.Current()
	if lic == nil {
		t.Fatal("expected non-nil license after auto-migration")
	}
	if lic.Claims.LicenseID != "lic_migrated" {
		t.Errorf("LicenseID = %q, want lic_migrated", lic.Claims.LicenseID)
	}

	// Verify the legacy JWT file was cleaned up.
	loaded, _ := persistence.Load()
	if loaded != "" {
		t.Error("expected legacy JWT file to be deleted after migration")
	}

	// Verify activation state was persisted for future restarts.
	activationState, loadActivationErr := persistence.LoadActivationState()
	if loadActivationErr != nil {
		t.Fatalf("LoadActivationState: %v", loadActivationErr)
	}
	if activationState == nil {
		t.Fatal("expected activation state to be persisted")
	}
	if activationState.InstallationID != "inst_migrated" {
		t.Errorf("InstallationID = %q, want inst_migrated", activationState.InstallationID)
	}

	// Verify the exchange was called exactly once (no concurrent duplicates).
	if n := exchangeCalled.Load(); n != 1 {
		t.Errorf("exchange called %d times, want 1", n)
	}

	// Clean up background loops.
	handlers.StopAllBackgroundLoops()
}

func TestGetTenantComponents_FallsBackToLegacyJWT_WhenExchangeFails(t *testing.T) {
	// Enable dev mode so legacy JWT validation skips signature check.
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	// Mock license server that always returns an error.
	var exchangeCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchangeCalled.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    "server_error",
			"message": "Internal server error",
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

	// The legacy JWT should be active immediately (synchronous path).
	lic := svc.Current()
	if lic == nil {
		t.Fatal("expected non-nil license from legacy JWT (synchronous load)")
	}
	if lic.Claims.Tier != pkglicensing.TierPro {
		t.Errorf("Tier = %q, want pro", lic.Claims.Tier)
	}

	// Wait for the background exchange attempt to complete (it will fail).
	deadline := time.After(5 * time.Second)
	for exchangeCalled.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for background exchange attempt")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// After exchange failure, service should still be on legacy JWT.
	if svc.IsActivated() {
		t.Error("expected IsActivated=false when exchange fails (should remain legacy JWT)")
	}

	// Legacy JWT should still be loaded.
	loaded, _ := persistence.Load()
	if loaded == "" {
		t.Error("expected legacy JWT file to still exist after failed exchange")
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
		"lid":        "lic_existing",
		"tier":       "pro",
		"st":         "active",
		"feat":       []string{"relay"},
		"max_agents": 10,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(72 * time.Hour).Unix(),
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
