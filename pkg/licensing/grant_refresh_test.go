package licensing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestGrantRefreshLoop_StartStop(t *testing.T) {
	svc := NewService()

	// Start without any activation state — should be a no-op internally but not panic.
	svc.StartGrantRefresh(context.Background())
	svc.StopGrantRefresh()

	// Start and stop multiple times should be safe.
	svc.StartGrantRefresh(context.Background())
	svc.StartGrantRefresh(context.Background()) // Duplicate start is a no-op.
	svc.StopGrantRefresh()
	svc.StopGrantRefresh() // Duplicate stop is a no-op.
}

func setImmediateGrantRefreshLoop(t *testing.T, svc *Service) {
	t.Helper()

	svc.SetRefreshHints(RefreshHints{IntervalSeconds: 60, JitterPercent: 0})
	svc.mu.RLock()
	loop := svc.grantRefresh
	svc.mu.RUnlock()
	if loop == nil {
		t.Fatal("expected grant refresh loop to be initialized")
	}

	loop.mu.Lock()
	loop.refreshInterval = time.Millisecond
	loop.jitterPercent = 0
	loop.mu.Unlock()
}

func TestGrantRefreshLoop_RefreshesGrant(t *testing.T) {
	setupTestPublicKey(t)
	const expectedClientVersion = "6.0.0-rc.1"

	// Set up a mock license server that serves a new grant on refresh.
	var refreshCount atomic.Int32
	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_refreshed",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		var req RefreshGrantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if req.ClientVersion != expectedClientVersion {
			t.Fatalf("ClientVersion = %q, want %q", req.ClientVersion, expectedClientVersion)
		}
		if req.Runtime == nil || req.Runtime.Build != RuntimeBuildCommunity {
			t.Fatalf("Runtime build = %#v, want %q", req.Runtime, RuntimeBuildCommunity)
		}
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_new",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	// Build the initial grant.
	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetClientVersion(expectedClientVersion)
	svc.mu.Lock()
	svc.activationState = &ActivationState{
		InstallationID:      "inst_test",
		InstallationToken:   "pit_live_test",
		LicenseID:           "lic_initial",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-test",
	}
	svc.mu.Unlock()

	// Restore the license from the initial grant so refreshGrantOnce has something to update.
	if err := svc.RestoreActivation(svc.activationState); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	// Perform a single manual refresh.
	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	if refreshCount.Load() != 1 {
		t.Errorf("refresh count = %d, want 1", refreshCount.Load())
	}

	// Verify the license was updated.
	lic := svc.Current()
	if lic == nil {
		t.Fatal("expected current license after refresh")
	}
	if lic.Claims.LicenseID != "lic_refreshed" {
		t.Errorf("LicenseID = %q, want lic_refreshed", lic.Claims.LicenseID)
	}

	// Verify activation state was updated.
	state := svc.GetActivationState()
	if state == nil {
		t.Fatal("expected activation state after refresh")
	}
	if state.GrantJTI != "grant_new" {
		t.Errorf("GrantJTI = %q, want grant_new", state.GrantJTI)
	}
}

func TestIsRevokedActivationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"revoked token", &LicenseServerError{StatusCode: 401, Code: "TOKEN_REVOKED"}, true},
		{"revoked token lowercase", &LicenseServerError{StatusCode: 401, Code: "token_revoked"}, true},
		{"revoked installation", &LicenseServerError{StatusCode: 401, Code: "INSTALLATION_REVOKED"}, true},
		{"revoked license", &LicenseServerError{StatusCode: 401, Code: "LICENSE_REVOKED"}, true},
		{"wrapped revoked token", fmt.Errorf("refresh grant: %w", &LicenseServerError{StatusCode: 401, Code: "TOKEN_REVOKED"}), true},
		{"expired token", &LicenseServerError{StatusCode: 401, Code: "TOKEN_EXPIRED"}, false},
		{"token not found", &LicenseServerError{StatusCode: 401, Code: "INVALID_TOKEN"}, false},
		{"401 without structured code", &LicenseServerError{StatusCode: 401, Code: "http_401"}, false},
		{"revocation code on non-401", &LicenseServerError{StatusCode: 403, Code: "TOKEN_REVOKED"}, false},
		{"plain error", fmt.Errorf("connection refused"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRevokedActivationError(tt.err); got != tt.want {
				t.Fatalf("isRevokedActivationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGrantRefreshLoop_NonRevocation401KeepsActivation(t *testing.T) {
	setupTestPublicKey(t)

	// A 401 without an explicit revocation code (token not found, expired,
	// transient server auth issues) must NOT wipe the licence — the grant's
	// own expiry plus grace governs degradation instead.
	var refreshCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    "INVALID_TOKEN",
			"message": "Installation token not found",
		})
	}))
	defer server.Close()

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_spurious_401",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	tmpDir, err := os.MkdirTemp("", "pulse-refresh-spurious-401-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(p)

	state := &ActivationState{
		InstallationID:      "inst_spurious",
		InstallationToken:   "pit_live_spurious",
		LicenseID:           "lic_spurious_401",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-spurious",
	}
	if err := p.SaveActivationState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	setImmediateGrantRefreshLoop(t, svc)
	svc.StartGrantRefresh(context.Background())

	// Wait for at least one failed refresh attempt.
	deadline := time.After(5 * time.Second)
	for refreshCount.Load() == 0 {
		select {
		case <-deadline:
			svc.StopGrantRefresh()
			t.Fatal("timed out waiting for refresh attempt")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	// Give the loop a moment to (incorrectly) clear state if it were going to.
	time.Sleep(50 * time.Millisecond)
	svc.StopGrantRefresh()

	if svc.Current() == nil {
		t.Fatal("expected licence to survive a non-revocation 401")
	}
	if !svc.IsActivated() {
		t.Fatal("expected activation to survive a non-revocation 401")
	}
	if !p.ActivationStateExists() {
		t.Fatal("expected persisted activation state to survive a non-revocation 401")
	}
}

func TestGrantRefreshLoop_401ClearsActivation(t *testing.T) {
	setupTestPublicKey(t)

	// This test exercises the runGrantRefreshLoop revocation branch: a 401
	// carrying an explicit revocation code calls clearActivationState to
	// revert to free tier and exit the loop.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"code":    "token_revoked",
			"message": "Installation revoked",
		})
	}))
	defer server.Close()

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_revoked",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	tmpDir, err := os.MkdirTemp("", "pulse-refresh-401-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(p)
	var callbackState atomic.Pointer[ActivationState]
	var callbackCalled atomic.Bool
	svc.SetActivationStateChangeCallback(func(state *ActivationState) {
		callbackCalled.Store(true)
		callbackState.Store(state)
	})

	state := &ActivationState{
		InstallationID:      "inst_revoked",
		InstallationToken:   "pit_live_revoked",
		LicenseID:           "lic_revoked",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-test",
	}
	if err := p.SaveActivationState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	setImmediateGrantRefreshLoop(t, svc)
	// Start the refresh loop — it should hit 401 and self-exit after clearing state.
	svc.StartGrantRefresh(context.Background())

	// Wait for the loop to exit (it should exit quickly after the 401).
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			svc.StopGrantRefresh()
			t.Fatal("timed out waiting for refresh loop to exit on 401")
		default:
		}
		// Check if activation was cleared.
		if svc.Current() == nil && !svc.IsActivated() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the license and activation state were cleared.
	if svc.Current() != nil {
		t.Error("expected nil license after 401 revocation")
	}
	if svc.IsActivated() {
		t.Error("expected activation cleared after 401 revocation")
	}
	if !callbackCalled.Load() {
		t.Fatal("expected activation-state callback after 401 revocation")
	}
	if state := callbackState.Load(); state != nil {
		t.Fatalf("expected nil activation-state callback after 401 revocation, got %+v", state)
	}

	// Verify the persisted state was also cleared.
	if p.ActivationStateExists() {
		t.Error("expected persisted activation state to be cleared after 401")
	}
}

func TestRefreshGrantOnce_NoClient(t *testing.T) {
	svc := NewService()
	svc.mu.Lock()
	svc.activationState = &ActivationState{InstallationID: "inst_test"}
	svc.mu.Unlock()

	err := svc.refreshGrantOnce(context.Background())
	if err == nil {
		t.Fatal("expected error when no client")
	}
}

func TestRefreshGrantOnce_NoState(t *testing.T) {
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient("http://localhost"))

	err := svc.refreshGrantOnce(context.Background())
	if err == nil {
		t.Fatal("expected error when no state")
	}
}

func TestNextRefreshInterval(t *testing.T) {
	svc := NewService()

	t.Run("no loop initialized returns default", func(t *testing.T) {
		interval := svc.nextRefreshInterval(0)
		// With no loop, returns exactly defaultRefreshInterval.
		if interval != defaultRefreshInterval {
			t.Errorf("interval = %v, want %v (defaultRefreshInterval)", interval, defaultRefreshInterval)
		}
	})

	// Initialize the loop.
	svc.StartGrantRefresh(context.Background())
	defer svc.StopGrantRefresh()

	t.Run("zero failures returns near default interval", func(t *testing.T) {
		interval := svc.nextRefreshInterval(0)
		// Should be within ±20% of 6h (default).
		low := time.Duration(float64(defaultRefreshInterval) * 0.8)
		high := time.Duration(float64(defaultRefreshInterval) * 1.2)
		if interval < low || interval > high {
			t.Errorf("interval = %v, want between %v and %v", interval, low, high)
		}
	})

	t.Run("first failure returns min backoff", func(t *testing.T) {
		interval := svc.nextRefreshInterval(1)
		if interval != minRefreshBackoff {
			t.Errorf("interval = %v, want %v", interval, minRefreshBackoff)
		}
	})

	t.Run("exponential backoff", func(t *testing.T) {
		interval := svc.nextRefreshInterval(3)
		// 30s * 2^2 = 120s = 2m
		want := minRefreshBackoff * 4
		if interval != want {
			t.Errorf("interval = %v, want %v", interval, want)
		}
	})

	t.Run("capped at max backoff", func(t *testing.T) {
		interval := svc.nextRefreshInterval(20)
		if interval > maxRefreshBackoff {
			t.Errorf("interval = %v, want <= %v", interval, maxRefreshBackoff)
		}
	})
}

func TestSetRefreshHints(t *testing.T) {
	svc := NewService()

	t.Run("valid hints", func(t *testing.T) {
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 3600, // 1h
			JitterPercent:   0.1,
		})

		// Verify by checking the refresh interval via nextRefreshInterval.
		interval := svc.nextRefreshInterval(0)
		low := time.Hour - time.Hour/10
		high := time.Hour + time.Hour/10
		if interval < low || interval > high {
			t.Errorf("interval = %v, want between %v and %v", interval, low, high)
		}
	})

	t.Run("interval clamped to minimum 1m", func(t *testing.T) {
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 5, // Too short
			JitterPercent:   0.1,
		})

		interval := svc.nextRefreshInterval(0)
		if interval < time.Minute-time.Duration(float64(time.Minute)*0.1) {
			t.Errorf("interval = %v, should be clamped to at least ~1m", interval)
		}
	})

	t.Run("interval clamped to maximum 24h", func(t *testing.T) {
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 100000, // Way too long
			JitterPercent:   0.1,
		})

		interval := svc.nextRefreshInterval(0)
		max := 24*time.Hour + 12*time.Hour
		if interval > max {
			t.Errorf("interval = %v, should be clamped to at most ~24h", interval)
		}
	})

	t.Run("jitter > 0.5 rejected", func(t *testing.T) {
		// Set known good jitter first.
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 3600,
			JitterPercent:   0.1,
		})

		// Try to set jitter > 0.5 — should be rejected.
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 3600,
			JitterPercent:   0.8,
		})

		// Verify the old jitter (0.1) is still in effect by sampling intervals.
		// With 0.1 jitter, interval should be within ±10% of 1h.
		// With 0.8 jitter it would be within ±80%, so values outside ±50% prove rejection.
		for i := 0; i < 50; i++ {
			interval := svc.nextRefreshInterval(0)
			low := time.Hour / 2  // 30m — would be exceeded if 0.8 jitter were accepted
			high := time.Hour * 2 // 2h
			if interval < low || interval > high {
				t.Errorf("iteration %d: interval = %v, jitter 0.8 should have been rejected", i, interval)
			}
		}
	})

	t.Run("negative interval ignored", func(t *testing.T) {
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: 3600,
			JitterPercent:   0.1,
		})
		// Now try negative.
		svc.SetRefreshHints(RefreshHints{
			IntervalSeconds: -100,
			JitterPercent:   0.1,
		})
		// Interval should still be ~3600s.
		interval := svc.nextRefreshInterval(0)
		if interval < 50*time.Minute || interval > 70*time.Minute {
			t.Errorf("interval = %v, negative hint should not change it", interval)
		}
	})
}

func TestRefreshGrantOnce_PersistsState(t *testing.T) {
	setupTestPublicKey(t)

	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_persisted",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_persisted",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "pulse-refresh-persist-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(p)

	state := &ActivationState{
		InstallationID:      "inst_persist",
		InstallationToken:   "pit_live_persist",
		LicenseID:           "lic_initial",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-persist",
	}
	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	// Refresh should persist the updated state.
	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	// Load from disk and verify.
	loaded, err := p.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected persisted state")
	}
	if loaded.GrantJTI != "grant_persisted" {
		t.Errorf("GrantJTI = %q, want grant_persisted", loaded.GrantJTI)
	}
	if loaded.LastRefreshedAt == 0 {
		t.Error("LastRefreshedAt should be set after refresh")
	}
}

func TestRefreshGrantOnce_PreservesLegacyMigrationContinuity(t *testing.T) {
	setupTestPublicKey(t)

	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_refreshed_floor",
		Tier:      "pro",
		PlanKey:   "legacy_migration_fallback",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_refreshed_floor",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "pulse-refresh-floor-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial_floor",
		Tier:      "pro",
		PlanKey:   "legacy_migration_fallback",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(p)

	state := &ActivationState{
		InstallationID:      "inst_refresh_floor",
		InstallationToken:   "pit_live_refresh_floor",
		LicenseID:           "lic_initial_floor",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old_floor",
		InstanceFingerprint: "fp-refresh-floor",
		Continuity: ActivationContinuity{
			LegacyMigration: true,
		},
	}
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	status := svc.Status()
	if status == nil || !status.Valid {
		t.Fatalf("expected valid status after refresh, got %+v", status)
	}

	loaded, err := p.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected persisted activation state")
	}
	if !loaded.Continuity.LegacyMigration {
		t.Fatalf("LegacyMigration=%v, want true", loaded.Continuity.LegacyMigration)
	}
}

func TestRefreshGrantOnce_MissingCloudPlanFailsClosed(t *testing.T) {
	setupTestPublicKey(t)

	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_cloud_unknown_plan",
		Tier:      "cloud",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_cloud_unknown",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	initialGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	state := &ActivationState{
		InstallationID:      "inst_cloud_unknown",
		InstallationToken:   "pit_live_cloud_unknown",
		LicenseID:           "lic_initial",
		GrantJWT:            initialGrantJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-cloud-unknown",
	}
	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	lic := svc.Current()
	if lic == nil {
		t.Fatal("expected current license after refresh")
	}
	if got := lic.Claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	if _, ok := lic.Claims.EffectiveLimits()["max_monitored_systems"]; ok {
		t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", lic.Claims.EffectiveLimits())
	}

	status := svc.Status()
	if status.PlanVersion != "" {
		t.Fatalf("status.PlanVersion=%q, want empty", status.PlanVersion)
	}
}

func TestRefreshGrantOnce_CallsLicenseChangeCallback(t *testing.T) {
	setupTestPublicKey(t)

	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_callback",
		Tier:      "relay",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_cb",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	var callbackLicense *License
	svc := NewService()
	svc.SetLicenseChangeCallback(func(lic *License) {
		callbackLicense = lic
	})
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	state := &ActivationState{
		InstallationID:      "inst_cb",
		InstallationToken:   "pit_live_cb",
		LicenseID:           "lic_initial",
		GrantJWT:            initialJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-cb",
	}
	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	if callbackLicense == nil {
		t.Fatal("license change callback was not invoked")
	}
	if callbackLicense.Claims.LicenseID != "lic_callback" {
		t.Errorf("callback license ID = %q, want lic_callback", callbackLicense.Claims.LicenseID)
	}
}

func TestRefreshGrantOnce_CallsActivationStateChangeCallback(t *testing.T) {
	setupTestPublicKey(t)

	newGrantJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_activation_callback",
		Tier:      "relay",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newGrantJWT,
				JTI:       "grant_activation_callback",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_initial_activation_callback",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	var callbackState *ActivationState
	svc := NewService()
	svc.SetActivationStateChangeCallback(func(state *ActivationState) {
		callbackState = state
	})
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))

	state := &ActivationState{
		InstallationID:      "inst_activation_callback",
		InstallationToken:   "pit_live_activation_callback",
		LicenseID:           "lic_initial_activation_callback",
		GrantJWT:            initialJWT,
		GrantJTI:            "grant_old_activation_callback",
		InstanceFingerprint: "fp-activation-callback",
	}
	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	callbackState = nil
	if err := svc.refreshGrantOnce(context.Background()); err != nil {
		t.Fatalf("refreshGrantOnce: %v", err)
	}

	if callbackState == nil {
		t.Fatal("activation-state callback was not invoked")
	}
	if callbackState.GrantJTI != "grant_activation_callback" {
		t.Fatalf("GrantJTI = %q, want grant_activation_callback", callbackState.GrantJTI)
	}
}

func TestGenerateFingerprint(t *testing.T) {
	fp1, err := generateFingerprint()
	if err != nil {
		t.Fatalf("generateFingerprint: %v", err)
	}

	// Should look like a UUID v4.
	if len(fp1) != 36 { // xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		t.Errorf("fingerprint length = %d, want 36", len(fp1))
	}

	// Two fingerprints should be different.
	fp2, err := generateFingerprint()
	if err != nil {
		t.Fatalf("generateFingerprint: %v", err)
	}
	if fp1 == fp2 {
		t.Error("two fingerprints should be different")
	}
}

func TestGrantRefreshDowngradeWindowSurvivesSameTierRefreshAndClearsOnUpgrade(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	downgraded := AdvanceDowngradeRetention(nil, TierPro, TierRelay, now)
	refreshed := AdvanceDowngradeRetention(downgraded, TierRelay, TierRelay, now.Add(time.Hour))
	if refreshed == nil || refreshed.DetectedAt != downgraded.DetectedAt || refreshed.PurgeEligibleAt != downgraded.PurgeEligibleAt {
		t.Fatalf("same-tier grant refresh changed downgrade window: %+v", refreshed)
	}
	if upgraded := AdvanceDowngradeRetention(refreshed, TierRelay, TierPro, now.Add(2*time.Hour)); upgraded != nil {
		t.Fatalf("upgraded grant retained purge state: %+v", upgraded)
	}
}
