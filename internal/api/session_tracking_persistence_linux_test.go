//go:build linux

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func trackingDiskFixture(t *testing.T) (*Router, map[string][]string) {
	t.Helper()
	resetSessionTracking()
	t.Cleanup(resetSessionTracking)
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(t, &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "synthetic-admin", AuthPass: hash}, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	tokens := make(map[string][]string)
	create := func(raw, user string, tracked bool) {
		router.sessionStore.CreateSession(raw, time.Hour, "fixture", "192.0.2.1", user)
		tokens[raw] = []string{router.csrfStore.GenerateCSRFToken(raw), router.csrfStore.GenerateCSRFToken(raw)}
		if tracked {
			TrackUserSession(user, raw)
		}
	}
	create("slow-disk-filler", "filler", false)
	create("control-warm", "synthetic-admin", true)
	create("control-cold", "synthetic-admin", false)
	for i := 0; i < maxSessionsPerUser; i++ {
		create(fmt.Sprintf("target-%02d", i), "retired-user", true)
	}
	create("target-new", "retired-user", false)
	create("target-untracked", "retired-user", false)
	// Tracked legacy-only entries must still lose their complete CSRF set.
	TrackUserSession("legacy-user", "legacy-only")
	tokens["legacy-only"] = []string{router.csrfStore.GenerateCSRFToken("legacy-only")}
	return router, tokens
}

func trackingRequest(router *Router, raw, csrf string, write bool) *httptest.ResponseRecorder {
	method, path, body := http.MethodGet, "/api/security/tokens", ""
	if write {
		method, body = http.MethodPost, "{not-json"
	}
	request := httptest.NewRequest(method, "https://pulse.example.test"+path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: raw})
	request.Header.Set("Origin", "https://pulse.example.test")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()
	router.Handler().ServeHTTP(response, request)
	return response
}

// The unchanged witness runs on the original and repaired runtime. These are
// acknowledged real writes, not sleeping mocks. An unrelated registered write
// must reach body validation while account withdrawal/oldest-session eviction
// is waiting for either store's disk. The paused operation must not return early.
func TestSessionTrackingPersistenceDoesNotBlockUnrelatedRequests(t *testing.T) {
	for _, operation := range []string{"revoke", "evict"} {
		for _, disk := range []string{"session", "csrf"} {
			t.Run(operation+"/"+disk, func(t *testing.T) {
				router, tokens := trackingDiskFixture(t)
				action := func() { InvalidateUserSessions("retired-user") }
				if operation == "evict" {
					action = func() { TrackUserSession("retired-user", "target-new") }
				}
				var resume func() []byte
				if disk == "session" {
					resume = pauseSessionDiskWrite(t, router.sessionStore, action)
				} else {
					resume = pauseCSRFDiskWrite(t, router.csrfStore, action)
				}
				if !sessionsMu.TryLock() {
					t.Error("account persistence retains the shared username/tracking lock")
				} else {
					sessionsMu.Unlock()
				}
				if router.sessionStore.ValidateSession("target-00") {
					t.Error("withdrawn session remains live during persistence")
				}
				for _, token := range tokens["target-00"] {
					if router.csrfStore.ValidateCSRFToken("target-00", token) {
						t.Error("withdrawn CSRF set remains live during session persistence")
					}
				}
				done := make(chan struct{})
				var warm, cold, removed string
				var response, rejected *httptest.ResponseRecorder
				go func() {
					defer close(done)
					warm = GetSessionUsername("control-warm")
					cold = GetSessionUsername("control-cold")
					removed = GetSessionUsername("target-00")
					response = trackingRequest(router, "control-warm", tokens["control-warm"][0], true)
					rejected = trackingRequest(router, "target-00", tokens["target-00"][0], false)
				}()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Error("unrelated attribution/registered request waits for account persistence")
					resume()
					<-done
				}
				if warm != "synthetic-admin" || cold != "synthetic-admin" || removed != "" {
					t.Error("tracking lost control attribution or retained withdrawn attribution")
				}
				if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "CSRF") {
					t.Errorf("valid registered write did not reach body validation: %d", response.Code)
				}
				if rejected.Code != http.StatusUnauthorized {
					t.Errorf("withdrawn registered request = %d, want 401", rejected.Code)
				}
				resume()
				loaded := NewSessionStore(router.sessionStore.dataPath)
				t.Cleanup(loaded.Shutdown)
				csrf := loadCSRFDiskStore(t, router.csrfStore.dataPath)
				for raw, set := range tokens {
					withdrawn := raw == "target-00" || (operation == "revoke" && strings.HasPrefix(raw, "target-"))
					if raw != "legacy-only" && loaded.ValidateSession(raw) == withdrawn {
						t.Errorf("restart changed session withdrawal boundary for %s", raw)
					}
					for _, token := range set {
						if csrf.ValidateCSRFToken(raw, token) == withdrawn {
							t.Errorf("restart changed CSRF withdrawal boundary for %s", raw)
						}
					}
				}
			})
		}
	}
}
