package licensing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testInstallationStatusState(version int64) *ActivationState {
	return &ActivationState{
		InstallationID:      "inst_status",
		InstallationToken:   "pit_live_status",
		LicenseID:           "lic_status",
		GrantJTI:            "grt_status",
		LicenseVersion:      version,
		InstanceFingerprint: "fp-status",
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(message)
}

func TestInstallationStatusPollStartsImmediatelyAndIsIdempotent(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/grants/status" || r.Method != http.MethodPost {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pit_live_status" {
			t.Fatalf("Authorization=%q", got)
		}
		var req InstallationStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		if req.InstallationID != "inst_status" || req.InstanceFingerprint != "fp-status" || req.CurrentLicenseVersion != 3 || req.CurrentGrantJTI != "grt_status" {
			t.Fatalf("unexpected request: %+v", req)
		}
		json.NewEncoder(w).Encode(InstallationStatusResponse{
			LicenseVersion: 3,
			StatusPolicy:   StatusHints{IntervalSeconds: 300, JitterPercent: 20, RetryBaseSeconds: 30, RetryMaxSeconds: 600},
		})
	}))
	defer server.Close()

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetClientVersion("6.0.0")
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(3)
	svc.mu.Unlock()
	svc.StartInstallationStatusPoll(context.Background())
	svc.StartInstallationStatusPoll(context.Background())
	waitForCondition(t, time.Second, func() bool { return calls.Load() == 1 }, "immediate status request did not run")
	svc.StopInstallationStatusPoll()
	svc.StopInstallationStatusPoll()
	if calls.Load() != 1 {
		t.Fatalf("calls=%d want 1", calls.Load())
	}

	status := svc.Status().Synchronization
	if status == nil || status.Running || !status.Healthy || status.LastAttemptAt == nil || status.LastSuccessAt == nil || status.ConsecutiveFailures != 0 {
		t.Fatalf("unexpected synchronization status: %+v", status)
	}
}

func TestInstallationStatusPollCanRestart(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(InstallationStatusResponse{LicenseVersion: 1})
	}))
	defer server.Close()
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(1)
	svc.mu.Unlock()

	svc.StartInstallationStatusPoll(context.Background())
	waitForCondition(t, time.Second, func() bool { return calls.Load() == 1 }, "first immediate status request did not run")
	svc.StopInstallationStatusPoll()
	svc.StartInstallationStatusPoll(context.Background())
	waitForCondition(t, time.Second, func() bool { return calls.Load() == 2 }, "restart did not perform immediate status request")
	svc.StopInstallationStatusPoll()
}

func TestInstallationStatusStopCancelsInFlightRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		server.Close()
	}()
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(1)
	svc.mu.Unlock()
	svc.StartInstallationStatusPoll(context.Background())
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("status request did not start")
	}
	done := make(chan struct{})
	go func() {
		svc.StopInstallationStatusPoll()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel and join in-flight request")
	}
}

func TestCheckInstallationStatusSameVersionDoesNotRefresh(t *testing.T) {
	var statusCalls atomic.Int32
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/grants/status":
			statusCalls.Add(1)
			json.NewEncoder(w).Encode(InstallationStatusResponse{LicenseVersion: 4})
		case "/v1/grants/refresh":
			refreshCalls.Add(1)
			t.Fatal("same-version status must not refresh the grant")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(4)
	svc.mu.Unlock()
	if err := svc.checkInstallationStatusOnce(context.Background()); err != nil {
		t.Fatalf("checkInstallationStatusOnce: %v", err)
	}
	if statusCalls.Load() != 1 || refreshCalls.Load() != 0 {
		t.Fatalf("status=%d refresh=%d", statusCalls.Load(), refreshCalls.Load())
	}
}

func TestCheckInstallationStatusHigherVersionRefreshesAndPersists(t *testing.T) {
	setupTestPublicKey(t)
	newGrant := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:      "lic_status",
		InstallationID: "inst_status",
		LicenseVersion: 2,
		Tier:           "relay",
		State:          "active",
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		JTI:            "grt_new",
	})
	var refreshCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/grants/status":
			json.NewEncoder(w).Encode(InstallationStatusResponse{LicenseVersion: 2, RefreshRequired: true})
		case "/v1/grants/refresh":
			refreshCalls.Add(1)
			json.NewEncoder(w).Encode(RefreshGrantResponse{Grant: GrantEnvelope{
				JWT: newGrant, JTI: "grt_new", ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tmpDir, err := os.MkdirTemp("", "pulse-status-refresh-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	persistence, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("NewPersistence: %v", err)
	}
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(persistence)
	state := testInstallationStatusState(1)
	svc.mu.Lock()
	svc.activationState = state
	svc.license = &License{Claims: Claims{Tier: TierPro}}
	svc.mu.Unlock()
	if err := persistence.SaveActivationState(state); err != nil {
		t.Fatalf("SaveActivationState: %v", err)
	}

	if err := svc.checkInstallationStatusOnce(context.Background()); err != nil {
		t.Fatalf("checkInstallationStatusOnce: %v", err)
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh calls=%d want 1", refreshCalls.Load())
	}
	updated := svc.GetActivationState()
	if updated == nil || updated.LicenseVersion != 2 || updated.GrantJTI != "grt_new" {
		t.Fatalf("unexpected activation state: %+v", updated)
	}
	persisted, err := persistence.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState: %v", err)
	}
	if persisted.LicenseVersion != 2 || persisted.GrantJTI != "grt_new" {
		t.Fatalf("unexpected persisted state: %+v", persisted)
	}
}

func TestInstallationStatusExplicitRevocationClearsActivation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{
		{name: "token", status: http.StatusUnauthorized, code: "TOKEN_REVOKED"},
		{name: "installation", status: http.StatusUnauthorized, code: "INSTALLATION_REVOKED"},
		{name: "license", status: http.StatusForbidden, code: "LICENSE_REVOKED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": tc.code, "message": "revoked", "revoked": true}})
			}))
			defer server.Close()
			tmpDir, err := os.MkdirTemp("", "pulse-status-revoked-*")
			if err != nil {
				t.Fatalf("MkdirTemp: %v", err)
			}
			defer os.RemoveAll(tmpDir)
			persistence, err := NewPersistence(tmpDir)
			if err != nil {
				t.Fatalf("NewPersistence: %v", err)
			}
			svc := NewService()
			svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
			svc.SetPersistence(persistence)
			state := testInstallationStatusState(1)
			if err := persistence.SaveActivationState(state); err != nil {
				t.Fatalf("SaveActivationState: %v", err)
			}
			svc.mu.Lock()
			svc.activationState = state
			svc.license = &License{Claims: Claims{Tier: TierPro}}
			svc.mu.Unlock()
			svc.StartInstallationStatusPoll(context.Background())
			waitForCondition(t, 2*time.Second, func() bool { return !svc.IsActivated() }, "revocation did not clear activation")
			svc.StopInstallationStatusPoll()
			if svc.Current() != nil || persistence.ActivationStateExists() {
				t.Fatalf("revoked activation survived: current=%+v persisted=%v", svc.Current(), persistence.ActivationStateExists())
			}
		})
	}
}

func TestInstallationStatusAmbiguous401RetainsActivation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INVALID_TOKEN", "message": "unknown"}})
	}))
	defer server.Close()
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(1)
	svc.license = &License{Claims: Claims{Tier: TierPro}}
	svc.mu.Unlock()
	err := svc.checkInstallationStatusOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if isRevokedActivationError(err) || !svc.IsActivated() || svc.Current() == nil {
		t.Fatalf("ambiguous 401 must retain activation: err=%v", err)
	}
}

func TestInstallationStatusSuspensionDropsPaidAccessAndRecoversWithScopedActivation(t *testing.T) {
	setupTestPublicKey(t)
	initialGrant := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:      "lic_status",
		InstallationID: "inst_status",
		LicenseVersion: 1,
		Tier:           "pro",
		State:          "active",
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		JTI:            "grt_status",
	})
	recoveredGrant := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:      "lic_status",
		InstallationID: "inst_status",
		LicenseVersion: 2,
		Tier:           "relay",
		State:          "active",
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		JTI:            "grt_recovered",
	})
	var suspended atomic.Bool
	suspended.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/grants/status":
			if suspended.Load() {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
					"code": "LICENSE_SUSPENDED", "message": "paid access suspended",
				}})
				return
			}
			json.NewEncoder(w).Encode(InstallationStatusResponse{LicenseVersion: 2, RefreshRequired: true})
		case "/v1/grants/refresh":
			json.NewEncoder(w).Encode(RefreshGrantResponse{Grant: GrantEnvelope{
				JWT: recoveredGrant, JTI: "grt_recovered", ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	persistence, err := NewPersistence(t.TempDir())
	if err != nil {
		t.Fatalf("NewPersistence: %v", err)
	}
	state := testInstallationStatusState(1)
	state.GrantJWT = initialGrant
	if err := persistence.SaveActivationState(state); err != nil {
		t.Fatalf("SaveActivationState: %v", err)
	}
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.SetPersistence(persistence)
	svc.mu.Lock()
	svc.activationState = state
	svc.license = &License{Claims: Claims{Tier: TierPro}}
	svc.mu.Unlock()

	svc.StartInstallationStatusPoll(context.Background())
	waitForCondition(t, time.Second, func() bool { return svc.Current() == nil }, "suspension did not remove paid entitlements")
	svc.StopInstallationStatusPoll()
	if !svc.IsActivated() || !persistence.ActivationStateExists() {
		t.Fatal("suspension must preserve scoped activation for recovery")
	}
	persistedSuspension, err := persistence.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState after suspension: %v", err)
	}
	if persistedSuspension == nil || !persistedSuspension.PaidAccessSuspended {
		t.Fatalf("suspension marker was not persisted: %+v", persistedSuspension)
	}
	restarted := NewService()
	if err := restarted.RestoreActivation(persistedSuspension); err != nil {
		t.Fatalf("RestoreActivation after suspension: %v", err)
	}
	if !restarted.IsActivated() || restarted.Current() != nil {
		t.Fatalf("restart must preserve recovery authority without restoring paid access: activated=%v current=%+v", restarted.IsActivated(), restarted.Current())
	}
	if syncStatus := svc.Status().Synchronization; syncStatus == nil || !syncStatus.Healthy || syncStatus.ConsecutiveFailures != 0 {
		t.Fatalf("authoritative suspension should be synchronized, got %+v", syncStatus)
	}

	suspended.Store(false)
	if err := svc.checkInstallationStatusOnce(context.Background()); err != nil {
		t.Fatalf("recovery status check: %v", err)
	}
	if current := svc.Current(); current == nil || current.Claims.Tier != TierRelay {
		t.Fatalf("paid entitlements did not recover from signed grant: %+v", current)
	}
	if recovered := svc.GetActivationState(); recovered == nil || recovered.LicenseVersion != 2 || recovered.GrantJTI != "grt_recovered" {
		t.Fatalf("activation did not advance after recovery: %+v", recovered)
	} else if recovered.Continuity.DowngradeRetention == nil {
		t.Fatalf("suspended Pro-to-Relay recovery lost downgrade retention: %+v", recovered.Continuity)
	}
	persistedRecovery, err := persistence.LoadActivationState()
	if err != nil {
		t.Fatalf("LoadActivationState after recovery: %v", err)
	}
	if persistedRecovery == nil || persistedRecovery.PaidAccessSuspended {
		t.Fatalf("signed recovery grant did not clear persisted suspension: %+v", persistedRecovery)
	}
}

func TestInstallationStatusHintBoundsAndBackoff(t *testing.T) {
	svc := NewService()
	svc.SetInstallationStatusHints(StatusHints{IntervalSeconds: 1, JitterPercent: 90, RetryBaseSeconds: 1, RetryMaxSeconds: 99999})
	svc.mu.RLock()
	loop := svc.installationStatus
	svc.mu.RUnlock()
	loop.mu.Lock()
	if loop.interval != minInstallationStatusInterval || loop.jitterPercent != defaultInstallationStatusJitter || loop.retryBase != 5*time.Second || loop.retryMax != maxInstallationStatusBackoff {
		t.Fatalf("unexpected bounded hints: interval=%v jitter=%v base=%v max=%v", loop.interval, loop.jitterPercent, loop.retryBase, loop.retryMax)
	}
	loop.consecutiveFailures = 20
	loop.mu.Unlock()
	if delay := loop.nextDelay(); delay != maxInstallationStatusBackoff {
		t.Fatalf("backoff=%v want %v", delay, maxInstallationStatusBackoff)
	}
}

func TestInstallationStatusSnapshotMarksStaleSynchronizationUnhealthy(t *testing.T) {
	svc := NewService()
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(1)
	loop := newInstallationStatusPollLoop()
	svc.installationStatus = loop
	loop.mu.Lock()
	loop.running = true
	loop.lastAttemptAt = time.Now().Add(-20 * time.Minute)
	loop.lastSuccessAt = time.Now().Add(-20 * time.Minute)
	loop.nextCheckAt = time.Now().Add(-10 * time.Minute)
	loop.mu.Unlock()
	svc.mu.Unlock()

	status := svc.Status().Synchronization
	if status == nil || !status.Running || !status.Stale || status.Healthy {
		t.Fatalf("unexpected stale synchronization status: %+v", status)
	}
}

func TestInstallationStatusNoActivationExitsCleanly(t *testing.T) {
	svc := NewService()
	if err := svc.checkInstallationStatusOnce(context.Background()); !errors.Is(err, errNoActivationState) {
		t.Fatalf("error=%v want no activation", err)
	}
}

func TestLicenseServerClientCheckInstallationStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/grants/status" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pit_live_transport" {
			t.Fatalf("Authorization=%q", got)
		}
		var req InstallationStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode request: %v", err)
		}
		if req.InstallationID != "inst_transport" || req.CurrentLicenseVersion != 7 {
			t.Fatalf("request=%+v", req)
		}
		json.NewEncoder(w).Encode(InstallationStatusResponse{
			LicenseVersion:  8,
			RefreshRequired: true,
			StatusPolicy:    StatusHints{IntervalSeconds: 300},
		})
	}))
	defer server.Close()
	client := NewLicenseServerClient(server.URL)
	resp, err := client.CheckInstallationStatus(context.Background(), "pit_live_transport", InstallationStatusRequest{
		InstallationID:        "inst_transport",
		CurrentLicenseVersion: 7,
	})
	if err != nil {
		t.Fatalf("CheckInstallationStatus: %v", err)
	}
	if resp.LicenseVersion != 8 || !resp.RefreshRequired || resp.StatusPolicy.IntervalSeconds != 300 {
		t.Fatalf("response=%+v", resp)
	}
}

func TestRestoreActivationBackfillsSignedLicenseVersion(t *testing.T) {
	setupTestPublicKey(t)
	grant := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:      "lic_restore_version",
		InstallationID: "inst_restore_version",
		LicenseVersion: 9,
		Tier:           "pro",
		State:          "active",
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
	})
	svc := NewService()
	if err := svc.RestoreActivation(&ActivationState{
		InstallationID:      "inst_restore_version",
		InstallationToken:   "pit_live_restore",
		LicenseID:           "lic_restore_version",
		GrantJWT:            grant,
		InstanceFingerprint: "fp-restore",
	}); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}
	if state := svc.GetActivationState(); state == nil || state.LicenseVersion != 9 {
		t.Fatalf("activation state=%+v want signed version 9", state)
	}
}

func TestGrantRefreshAttemptsAreSerialized(t *testing.T) {
	setupTestPublicKey(t)
	grant := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:      "lic_serial",
		InstallationID: "inst_status",
		LicenseVersion: 2,
		Tier:           "pro",
		State:          "active",
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
	})
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int32
	var active atomic.Int32
	var maxActive atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		current := active.Add(1)
		defer active.Add(-1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		select {
		case entered <- struct{}{}:
			<-release
		default:
		}
		json.NewEncoder(w).Encode(RefreshGrantResponse{Grant: GrantEnvelope{
			JWT: grant, JTI: "grt_serial", ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
		}})
	}))
	defer server.Close()
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = testInstallationStatusState(1)
	svc.license = &License{Claims: Claims{Tier: TierPro}}
	svc.mu.Unlock()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); errs <- svc.refreshGrantOnce(context.Background()) }()
	<-entered
	go func() { defer wg.Done(); errs <- svc.refreshGrantOnce(context.Background()) }()
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("concurrent refresh reached server before first completed: calls=%d", calls.Load())
	}
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("refresh error: %v", err)
		}
	}
	if calls.Load() != 2 || maxActive.Load() != 1 {
		t.Fatalf("calls=%d max_active=%d want 2/1", calls.Load(), maxActive.Load())
	}
}
