package licensing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRevocationPollLoop_StartStop(t *testing.T) {
	svc := NewService()

	// Empty token means polling is disabled — should not start.
	svc.StartRevocationPoll(context.Background(), "")
	svc.StopRevocationPoll() // Should be no-op.

	// Start and stop with a real token.
	svc.StartRevocationPoll(context.Background(), "feed_token")
	svc.StartRevocationPoll(context.Background(), "feed_token") // Duplicate start is no-op.
	svc.StopRevocationPoll()
	svc.StopRevocationPoll() // Duplicate stop is no-op.
}

func TestClientFetchRevocations(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("Method = %q, want GET", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer feed_token" {
				t.Errorf("Authorization = %q, want Bearer feed_token", got)
			}
			if r.URL.Query().Get("since_seq") != "5" {
				t.Errorf("since_seq = %q, want 5", r.URL.Query().Get("since_seq"))
			}

			json.NewEncoder(w).Encode(RevocationFeedResponse{
				FromSeq:   5,
				NextSeq:   10,
				LatestSeq: 10,
				HasMore:   false,
				Events: []RevocationEvent{
					{Seq: 6, Action: "revoke_license", LicenseID: "lic_bad"},
					{Seq: 10, Action: "bump_license_version", LicenseID: "lic_other", MinLicenseVersion: 3},
				},
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		resp, err := client.FetchRevocations(t.Context(), "feed_token", 5, 500)
		if err != nil {
			t.Fatalf("FetchRevocations failed: %v", err)
		}
		if len(resp.Events) != 2 {
			t.Fatalf("events = %d, want 2", len(resp.Events))
		}
		if resp.NextSeq != 10 {
			t.Errorf("NextSeq = %d, want 10", resp.NextSeq)
		}
	})

	t.Run("auth error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{
				"code":    "unauthorized",
				"message": "Invalid feed token",
			})
		}))
		defer server.Close()

		client := NewLicenseServerClient(server.URL)
		_, err := client.FetchRevocations(t.Context(), "bad_token", 0, 500)
		if err == nil {
			t.Fatal("expected error")
		}
		apiErr, ok := err.(*LicenseServerError)
		if !ok {
			t.Fatalf("expected *LicenseServerError, got %T", err)
		}
		if apiErr.StatusCode != 403 {
			t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
		}
	})
}

func TestHandleRevocationEvent_RevokeLicense(t *testing.T) {
	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_revoked",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	state := &ActivationState{
		InstallationID: "inst_test",
		LicenseID:      "lic_revoked",
		GrantJWT:       initialJWT,
	}
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	// Non-matching license should be ignored.
	svc.handleRevocationEvent(RevocationEvent{
		Action:    "revoke_license",
		LicenseID: "lic_other",
	})
	if !svc.IsActivated() {
		t.Error("non-matching revocation should not clear activation")
	}

	// Matching license should clear activation.
	svc.handleRevocationEvent(RevocationEvent{
		Action:    "revoke_license",
		LicenseID: "lic_revoked",
	})
	if svc.IsActivated() {
		t.Error("matching revocation should clear activation")
	}
	if svc.Current() != nil {
		t.Error("license should be nil after revocation")
	}
}

func TestHandleRevocationEvent_RevokeInstallation(t *testing.T) {
	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_test",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	state := &ActivationState{
		InstallationID: "inst_revoked",
		LicenseID:      "lic_test",
		GrantJWT:       initialJWT,
	}
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	svc.handleRevocationEvent(RevocationEvent{
		Action:         "revoke_installation",
		InstallationID: "inst_revoked",
	})
	if svc.IsActivated() {
		t.Error("matching installation revocation should clear activation")
	}
}

func TestHandleRevocationEvent_BumpLicenseVersion(t *testing.T) {
	// When bump_license_version fires, it should trigger an immediate grant refresh.
	var refreshCalled atomic.Int32
	newJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_bumped",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalled.Add(1)
		json.NewEncoder(w).Encode(RefreshGrantResponse{
			Grant: GrantEnvelope{
				JWT:       newJWT,
				JTI:       "grant_bumped",
				ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
			},
		})
	}))
	defer server.Close()

	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_bumped",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	state := &ActivationState{
		InstallationID:      "inst_bump",
		InstallationToken:   "pit_live_bump",
		LicenseID:           "lic_bumped",
		GrantJWT:            initialJWT,
		GrantJTI:            "grant_old",
		InstanceFingerprint: "fp-bump",
	}
	svc.mu.Lock()
	svc.activationState = state
	svc.mu.Unlock()
	if err := svc.RestoreActivation(state); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	// Non-matching license should not trigger refresh.
	svc.handleRevocationEvent(RevocationEvent{
		Action:            "bump_license_version",
		LicenseID:         "lic_other",
		MinLicenseVersion: 5,
	})
	time.Sleep(50 * time.Millisecond)
	if refreshCalled.Load() != 0 {
		t.Error("non-matching bump should not trigger refresh")
	}

	// Matching license should trigger async refresh.
	svc.handleRevocationEvent(RevocationEvent{
		Action:            "bump_license_version",
		LicenseID:         "lic_bumped",
		MinLicenseVersion: 5,
	})
	// Wait for the async goroutine.
	deadline := time.After(2 * time.Second)
	for refreshCalled.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for refresh after version bump")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestNextRevocationPollInterval(t *testing.T) {
	t.Run("zero failures returns near default", func(t *testing.T) {
		interval := nextRevocationPollInterval(0)
		low := defaultRevocationPollInterval - defaultRevocationPollInterval/5
		high := defaultRevocationPollInterval + defaultRevocationPollInterval/5
		if interval < low || interval > high {
			t.Errorf("interval = %v, want between %v and %v", interval, low, high)
		}
	})

	t.Run("first failure returns min backoff", func(t *testing.T) {
		interval := nextRevocationPollInterval(1)
		if interval != minRevocationPollBackoff {
			t.Errorf("interval = %v, want %v", interval, minRevocationPollBackoff)
		}
	})

	t.Run("capped at max", func(t *testing.T) {
		interval := nextRevocationPollInterval(20)
		if interval > maxRevocationPollBackoff {
			t.Errorf("interval = %v, want <= %v", interval, maxRevocationPollBackoff)
		}
	})
}

func TestPollRevocationsOnce_NoActivation(t *testing.T) {
	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient("http://localhost"))
	svc.mu.Lock()
	svc.revocationPoll = &revocationPollLoop{feedToken: "token"}
	svc.mu.Unlock()

	// No activation state — should return nil (nothing to check).
	err := svc.pollRevocationsOnce(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestPollRevocationsOnce_Pagination(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			json.NewEncoder(w).Encode(RevocationFeedResponse{
				FromSeq:   0,
				NextSeq:   5,
				LatestSeq: 10,
				HasMore:   true,
				Events:    []RevocationEvent{{Seq: 5, Action: "revoke_license", LicenseID: "lic_other"}},
			})
		} else {
			json.NewEncoder(w).Encode(RevocationFeedResponse{
				FromSeq:   5,
				NextSeq:   10,
				LatestSeq: 10,
				HasMore:   false,
				Events:    []RevocationEvent{{Seq: 10, Action: "revoke_license", LicenseID: "lic_other"}},
			})
		}
	}))
	defer server.Close()

	initialJWT := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_mine",
		Tier:      "pro",
		State:     "active",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	svc := NewService()
	svc.SetLicenseServerClient(NewLicenseServerClient(server.URL))
	svc.mu.Lock()
	svc.activationState = &ActivationState{
		InstallationID: "inst_test",
		LicenseID:      "lic_mine",
		GrantJWT:       initialJWT,
	}
	svc.revocationPoll = &revocationPollLoop{feedToken: "token"}
	svc.mu.Unlock()
	if err := svc.RestoreActivation(svc.activationState); err != nil {
		t.Fatalf("RestoreActivation: %v", err)
	}

	err := svc.pollRevocationsOnce(context.Background())
	if err != nil {
		t.Fatalf("pollRevocationsOnce: %v", err)
	}
	if page != 2 {
		t.Errorf("expected 2 pages fetched, got %d", page)
	}

	// Checkpoint should be at 10.
	svc.mu.RLock()
	loop := svc.revocationPoll
	svc.mu.RUnlock()
	loop.mu.Lock()
	seq := loop.lastSeq
	loop.mu.Unlock()
	if seq != 10 {
		t.Errorf("lastSeq = %d, want 10", seq)
	}
}
