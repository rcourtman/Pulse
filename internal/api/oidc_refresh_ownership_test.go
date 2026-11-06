package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// All credentials and principals in these fixtures are synthetic. The channel
// barrier places replacement, withdrawal or expiry inside the actual IdP call;
// sleeps and favorable scheduling are not the test oracle.
type oidcRefreshOwnershipFixture struct {
	cfg              *config.Config
	store            *SessionStore
	token            string
	initial          *SessionData
	calls            atomic.Int32
	discoveryFailure atomic.Bool
	entered          chan struct{}
	release          chan struct{}
	once             sync.Once
}

func newOIDCRefreshOwnershipFixture(t *testing.T, blocked, failure bool) *oidcRefreshOwnershipFixture {
	t.Helper()
	f := &oidcRefreshOwnershipFixture{token: "synthetic-ownership-session", entered: make(chan struct{}), release: make(chan struct{})}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			if f.discoveryFailure.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer": base, "authorization_endpoint": base + "/auth",
				"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
			})
		case "/token":
			call := f.calls.Add(1)
			if call == 1 {
				close(f.entered)
				if blocked {
					<-f.release
				}
			}
			if failure || call > 1 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "synthetic-new-access", "refresh_token": "synthetic-new-refresh",
				"expires_in": 3600, "token_type": "Bearer",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	// Unblock before the server's registered Close cleanup, even on a failure.
	t.Cleanup(f.unblock)
	dir := t.TempDir()
	InitSessionStore(dir)
	f.store = GetSessionStore()
	f.cfg = &config.Config{DataPath: dir, ConfigPath: dir}
	setSSOAuthSnapshot(f.cfg, &config.SSOConfig{Providers: []config.SSOProvider{{
		ID: "synthetic-provider", Type: config.SSOProviderTypeOIDC, Enabled: true,
		OIDC: &config.OIDCProviderConfig{IssuerURL: server.URL, ClientID: "synthetic-client", ClientSecret: "synthetic-secret", Scopes: []string{"openid"}},
	}}})
	f.store.CreateOIDCSession(f.token, time.Hour, "fixture-agent", "127.0.0.1", "original-principal", &OIDCTokenInfo{
		RefreshToken: "synthetic-old-refresh", AccessTokenExp: time.Now().Add(-time.Minute), Issuer: server.URL, ClientID: "synthetic-client",
	})
	f.initial = f.store.GetSession(f.token)
	return f
}

func (f *oidcRefreshOwnershipFixture) unblock() { f.once.Do(func() { close(f.release) }) }

func (f *oidcRefreshOwnershipFixture) start(t *testing.T) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
	}()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not reach the bounded IdP barrier")
	}
	return done
}

func (f *oidcRefreshOwnershipFixture) finish(t *testing.T, done <-chan struct{}) {
	t.Helper()
	f.unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not finish after the IdP barrier was released")
	}
}

func TestOIDCRefreshOwnershipQueuedSnapshot(t *testing.T) {
	f := newOIDCRefreshOwnershipFixture(t, false, false)
	f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
	updated := f.store.GetSession(f.token)
	if updated == nil || updated.OIDCRefreshToken != "synthetic-new-refresh" || updated.OIDCTokenRefreshing {
		t.Fatal("first refresh did not complete normally")
	}
	// A request can capture the old session before the active refresh finishes,
	// then enter only after the single-flight guard has been released.
	f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
	if calls := f.calls.Load(); calls != 1 {
		t.Errorf("stale queued snapshot reached the IdP: calls=%d, want 1", calls)
	}
	if !reflect.DeepEqual(f.store.GetSession(f.token), updated) {
		t.Error("stale queued refresh changed or invalidated the newer session")
	}
}

func TestOIDCRefreshOwnershipBeforeStart(t *testing.T) {
	for _, state := range []string{"deleted", "expired", "replaced", "tokens-updated", "nil-snapshot", "store-replaced"} {
		t.Run(state, func(t *testing.T) {
			f := newOIDCRefreshOwnershipFixture(t, false, false)
			defer func() {
				if recover() != nil {
					t.Error("obsolete refresh panicked instead of skipping unavailable session")
				}
			}()
			current := f.store
			switch state {
			case "deleted":
				f.store.DeleteSession(f.token)
			case "expired":
				f.store.mu.Lock()
				f.store.sessions[sessionHash(f.token)].ExpiresAt = time.Now().Add(-time.Minute)
				f.store.mu.Unlock()
			case "replaced":
				f.store.CreateSession(f.token, time.Hour, "replacement-agent", "127.0.0.1", "replacement-principal")
			case "tokens-updated":
				f.store.UpdateOIDCTokens(f.token, "synthetic-independent-refresh", time.Now().Add(time.Hour))
			case "store-replaced":
				InitSessionStore(t.TempDir())
				current = GetSessionStore()
				current.mu.Lock()
				copy := *f.initial
				current.sessions[sessionHash(f.token)] = &copy
				current.mu.Unlock()
				current.save()
			case "nil-snapshot":
				// Invalid input must not start work or panic.
				f.initial = nil
			}
			before := current.GetSession(f.token)
			f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
			if calls := f.calls.Load(); calls != 0 {
				t.Errorf("obsolete/unavailable refresh reached the IdP: calls=%d, want 0", calls)
			}
			if !reflect.DeepEqual(current.GetSession(f.token), before) {
				t.Error("obsolete refresh changed the current session")
			}
		})
	}
}

func TestOIDCRefreshOwnershipDuringRequest(t *testing.T) {
	for _, failure := range []bool{false, true} {
		outcome := "success"
		if failure {
			outcome = "invalid-grant"
		}
		for _, state := range []string{"deleted", "expired", "replaced", "tokens-updated", "store-replaced"} {
			t.Run(outcome+"/"+state, func(t *testing.T) {
				f := newOIDCRefreshOwnershipFixture(t, true, failure)
				done := f.start(t)
				current := f.store
				switch state {
				case "deleted":
					current.DeleteSession(f.token)
				case "expired":
					current.mu.Lock()
					current.sessions[sessionHash(f.token)].ExpiresAt = time.Now().Add(-time.Minute)
					current.mu.Unlock()
					current.save()
				case "replaced":
					current.CreateSession(f.token, time.Hour, "replacement-agent", "127.0.0.1", "replacement-principal")
				case "tokens-updated":
					current.UpdateOIDCTokens(f.token, "synthetic-independent-refresh", time.Now().Add(time.Hour))
				case "store-replaced":
					InitSessionStore(t.TempDir())
					current = GetSessionStore()
					current.CreateSession(f.token, time.Hour, "replacement-agent", "127.0.0.1", "replacement-principal")
				}
				before := current.GetSession(f.token)
				if before != nil {
					// Refresh bookkeeping must be released even when its result is stale.
					before.OIDCTokenRefreshing = false
				}
				f.finish(t, done)
				if !reflect.DeepEqual(current.GetSession(f.token), before) {
					t.Error("late IdP outcome changed a withdrawn, expired or newer session")
				}
				if state == "expired" && current.ValidateSession(f.token) {
					t.Error("late IdP response revived an expired browser session")
				}
				restored := NewSessionStore(current.dataPath)
				t.Cleanup(restored.Shutdown)
				want := before
				if state == "expired" {
					want = nil
				}
				gotJSON, err := json.Marshal(restored.GetSession(f.token))
				if err != nil {
					t.Fatal(err)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotJSON) != string(wantJSON) {
					t.Error("late refresh changed persisted session authority")
				}
			})
		}
	}
}

func TestOIDCRefreshOwnershipSingleFlight(t *testing.T) {
	f := newOIDCRefreshOwnershipFixture(t, true, false)
	done := f.start(t)
	var callers sync.WaitGroup
	for i := 0; i < 12; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
		}()
	}
	callers.Wait()
	if calls := f.calls.Load(); calls != 1 {
		t.Errorf("concurrent refresh reached the IdP %d times, want 1", calls)
	}
	f.finish(t, done)
	if session := f.store.GetSession(f.token); session == nil || session.OIDCRefreshToken != "synthetic-new-refresh" || session.OIDCTokenRefreshing {
		t.Fatal("single active refresh did not complete and release its claim")
	}
}

func TestOIDCRefreshOwnershipDiscoveryRetry(t *testing.T) {
	f := newOIDCRefreshOwnershipFixture(t, false, false)
	f.discoveryFailure.Store(true)
	f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
	if f.calls.Load() != 0 || !reflect.DeepEqual(f.store.GetSession(f.token), f.initial) {
		t.Fatal("failed discovery changed the session or retained its refresh claim")
	}
	f.discoveryFailure.Store(false)
	f.store.refreshOIDCSessionTokens(f.cfg, f.token, f.initial)
	if session := f.store.GetSession(f.token); session == nil || session.OIDCRefreshToken != "synthetic-new-refresh" || session.OIDCTokenRefreshing || f.calls.Load() != 1 {
		t.Fatal("a discovery failure prevented the next legitimate refresh")
	}
}
