package licensing

import (
	"context"
	"encoding/json"
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

func TestGrantRefreshLoop_RefreshesGrant(t *testing.T) {
	setupTestPublicKey(t)

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

func TestGrantRefreshLoop_401ClearsActivation(t *testing.T) {
	setupTestPublicKey(t)

	// This test exercises the full runGrantRefreshLoop 401 branch, which calls
	// clearActivationState to revert to free tier and exit the loop.
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

	// Set a very short refresh interval so the loop fires immediately.
	svc.SetRefreshHints(RefreshHints{IntervalSeconds: 60, JitterPercent: 0.01})
	// Override the loop's interval directly for instant firing.
	svc.mu.RLock()
	loop := svc.grantRefresh
	svc.mu.RUnlock()
	loop.mu.Lock()
	loop.refreshInterval = time.Millisecond
	loop.jitterPercent = 0
	loop.mu.Unlock()

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
	if got := lic.Claims.EffectiveLimits()["max_agents"]; got != int64(UnknownPlanDefaultAgentLimit) {
		t.Fatalf("EffectiveLimits()[max_agents]=%d, want %d", got, UnknownPlanDefaultAgentLimit)
	}

	status := svc.Status()
	if status.PlanVersion != "" {
		t.Fatalf("status.PlanVersion=%q, want empty", status.PlanVersion)
	}
	if status.MaxAgents != UnknownPlanDefaultAgentLimit {
		t.Fatalf("status.MaxAgents=%d, want %d", status.MaxAgents, UnknownPlanDefaultAgentLimit)
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
