//go:build linux

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// OIDC completion must keep the landed detached-persistence contract: its live
// decision is atomic, disk writes do not own the auth mutex, and a newer logout
// wins the last saved snapshot. The existing FIFO fixture stops a real write.
func TestOIDCRefreshCompletionReleasesMemoryBeforePersistence(t *testing.T) {
	for _, outcome := range []string{"success", "invalid-grant"} {
		t.Run(outcome, func(t *testing.T) {
			_, _, server := newSessionDiskRouter(t)
			store := GetSessionStore()
			for _, raw := range []string{"slow-disk-filler", "control"} {
				store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
			}
			store.CreateOIDCSession("target", time.Hour, "fixture", "127.0.0.1", "operator", &OIDCTokenInfo{RefreshToken: "synthetic-old-refresh", AccessTokenExp: time.Now().Add(-time.Minute), Issuer: "https://idp.example.test", ClientID: "synthetic-client"})
			attempt := store.beginOIDCRefresh("target", store.GetSession("target"))
			if attempt == nil {
				t.Fatal("live due refresh could not be claimed")
			}
			defer store.endOIDCRefresh("target", attempt)
			var result *OIDCRefreshResult
			if outcome == "success" {
				result = &OIDCRefreshResult{RefreshToken: "synthetic-rotated-refresh", Expiry: time.Now().Add(time.Hour)}
			}
			completed := make(chan bool, 1)
			resume := pauseSessionDiskWrite(t, store, func() { completed <- store.completeOIDCRefresh("target", attempt, result) })
			defer resume()
			if !store.mu.TryLock() {
				t.Error("OIDC completion holds the auth mutex during persistence")
			} else {
				store.mu.Unlock()
			}
			// These are actual registered routes for a separate, still-live browser
			// session, not a ValidateSession-only or mock persistence assertion.
			serve := func(raw, path string, want int) {
				t.Helper()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
				rec := httptest.NewRecorder()
				done := make(chan struct{})
				go func() { server.Config.Handler.ServeHTTP(rec, req); close(done) }()
				select {
				case <-done:
					if rec.Code != want {
						t.Errorf("%s status=%d want=%d", path, rec.Code, want)
					}
				case <-time.After(2 * time.Second):
					t.Error("registered browser read waits for OIDC completion persistence")
					resume()
					<-done
				}
			}
			for _, path := range []string{"/api/security/status", "/api/state/summary"} {
				serve("control", path, http.StatusOK)
			}
			live := store.GetSession("target")
			if outcome == "success" {
				if live == nil || live.OIDCRefreshToken != "synthetic-rotated-refresh" {
					t.Error("completed rotation was not visible before persistence")
				}
			} else if live != nil {
				t.Error("current invalid-grant did not withdraw live authority before persistence")
			}

			// Queue a logout behind the blocked completion save and observe the live
			// withdrawal before unblocking either writer. No scheduler sleep is an oracle.
			deleted := make(chan struct{})
			go func() { store.DeleteSession("target"); close(deleted) }()
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			deadline := time.After(2 * time.Second)
			for store.GetSession("target") != nil {
				select {
				case <-tick.C:
				case <-deadline:
					resume()
					<-deleted
					t.Fatal("newer logout could not withdraw while OIDC persistence was blocked")
				}
			}
			serve("target", "/api/state/summary", http.StatusUnauthorized)
			data := resume()
			if !<-completed {
				t.Error("current refresh completion was not applied")
			}
			select {
			case <-deleted:
			case <-time.After(5 * time.Second):
				t.Fatal("queued logout save did not finish")
			}
			var snapshot []sessionPersisted
			if err := json.Unmarshal(data, &snapshot); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("synthetic-rotated-refresh")) || bytes.Contains(data, []byte("synthetic-old-refresh")) {
				t.Error("detached OIDC persistence disclosed a refresh token")
			}
			hadTarget := false
			for _, entry := range snapshot {
				if entry.Key == sessionHash("target") {
					hadTarget = true
				}
			}
			if hadTarget != (outcome == "success") {
				t.Error("paused completion snapshot did not reflect its own rotation/invalidation")
			}
			restored := NewSessionStore(store.dataPath)
			defer restored.Shutdown()
			if restored.GetSession("target") != nil || restored.GetSession("control") == nil {
				t.Error("older OIDC save resurrected the logged-out session or lost the control")
			}
		})
	}
}
