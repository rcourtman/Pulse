package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const (
	parkedBodySession = "synthetic-parked-body-session"
	parkedBodyToken   = "synthetic-parked-body-token.12345678"
)

// Authentication sees only the request headers. A client can send them while
// its credential is valid, hold back the end of the body and complete the
// request after logout, a password change, token deletion or expiry. The real
// routes must answer as they would for that withdrawn credential and must not
// issue or save anything.
func TestParkedRequestBodyFollowsCredentialWithdrawal(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	routes := map[string]struct{ path, body string }{
		"session": {"/api/security/tokens", `{"name":"parked","scopes":["*"]}`},
		"token":   {"/api/agent-install-command", `{"type":"host","enableCommands":true}`},
	}
	for _, test := range []struct {
		credential, change string
		want               int
	}{
		{"session", "none", http.StatusOK},
		{"session", "invalidated", http.StatusUnauthorized},
		{"session", "expired", http.StatusUnauthorized},
		{"session", "principal", http.StatusUnauthorized},
		{"session", "replaced", http.StatusUnauthorized},
		{"token", "none", http.StatusOK},
		{"token", "revoked", http.StatusUnauthorized},
		{"token", "rotated", http.StatusUnauthorized},
		{"token", "expired", http.StatusUnauthorized},
		{"token", "owner changed", http.StatusUnauthorized},
		// A still-valid token's lost privilege is the route's 403, not a 401.
		{"token", "unrelated scope removed", http.StatusOK},
	} {
		t.Run(test.credential+"/"+test.change, func(t *testing.T) {
			InitSessionStore(t.TempDir())
			caller := newTokenRecord(t, parkedBodyToken, []string{config.ScopeSettingsWrite, config.ScopeAgentExec, config.ScopeMonitoringRead}, nil)
			caller.ID, caller.OrgID = "fixture-parked-caller", "default"
			cfg := newTestConfigWithTokens(t, caller)
			cfg.AuthUser, cfg.AuthPass = "admin", "$2a$10$synthetic.parked.body.password.hash.fixture.value.0000"
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")

			route := routes[test.credential]
			paused := &tokenWithdrawalBody{Reader: strings.NewReader(route.body), started: make(chan struct{}), resume: make(chan struct{})}
			req := httptest.NewRequest(http.MethodPost, route.path, paused)
			req.Header.Set("Content-Type", "application/json")
			if test.credential == "session" {
				GetSessionStore().CreateSession(parkedBodySession, time.Hour, "fixture", "192.0.2.10", "admin")
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: parkedBodySession})
				req.Header.Set("X-CSRF-Token", generateCSRFToken(parkedBodySession))
			} else {
				req.Header.Set("X-API-Token", parkedBodyToken)
			}
			rec := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); router.Handler().ServeHTTP(rec, req) }()
			released := false
			defer func() {
				if !released {
					close(paused.resume)
				}
				<-done
			}()
			select {
			case <-paused.started:
			case <-done:
				t.Fatalf("request was refused before its body was read: status=%d body=%s", rec.Code, rec.Body.String())
			case <-time.After(5 * time.Second):
				t.Fatal("registered route did not reach its body reader")
			}

			if test.credential == "session" && test.change != "none" {
				mutateReadSession(parkedBodySession, test.change)
			}
			config.Mu.Lock()
			for i := range cfg.APITokens {
				if test.credential != "token" || cfg.APITokens[i].ID != caller.ID {
					continue
				}
				switch test.change {
				case "rotated":
					cfg.APITokens[i].Hash = "different-credential-hash"
				case "expired":
					expires := time.Now().Add(-time.Second)
					cfg.APITokens[i].ExpiresAt = &expires
				case "owner changed":
					cfg.APITokens[i].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "different-owner"}
				case "unrelated scope removed":
					cfg.APITokens[i].Scopes = []string{config.ScopeSettingsWrite, config.ScopeAgentExec}
				}
			}
			if test.change == "revoked" {
				cfg.RemoveAPIToken(caller.ID)
			}
			config.Mu.Unlock()
			before := tokenWithdrawalInventory(t, router)

			close(paused.resume)
			released = true
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not finish after its body resumed")
			}
			if rec.Code != test.want {
				t.Fatalf("parked request status=%d, want %d: %s", rec.Code, test.want, rec.Body.String())
			}
			issued := strings.Contains(rec.Body.String(), `"token":`)
			changed := string(tokenWithdrawalInventory(t, router)) != string(before)
			if test.want == http.StatusOK {
				if !issued || !changed {
					t.Error("a request whose credential stayed valid was not carried out")
				}
				return
			}
			// The same generic answer as a withheld protected read, not the
			// handler's "invalid body" and never a structured success.
			if rec.Body.String() != "Authentication required\n" || strings.Contains(rec.Header().Get("Content-Type"), "json") {
				t.Errorf("withdrawn credential was answered %q", rec.Body.String())
			}
			if issued {
				t.Error("withdrawn credential received a new credential")
			}
			if changed {
				t.Error("withdrawn credential changed the live inventory")
			}
		})
	}
}

// The withdrawal must reach a handler as a failed read, never as the end of a
// complete or omitted body, and only for the credential that admitted it.
func TestRequestBodyAuthorityAdmission(t *testing.T) {
	InitSessionStore(t.TempDir())
	GetSessionStore().CreateSession(parkedBodySession, time.Hour, "fixture", "192.0.2.10", "operator")
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, parkedBodyToken, []string{config.ScopeWildcard}, nil))
	newRequest := func(body io.Reader) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "http://192.0.2.1/api/security/tokens", body)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: parkedBodySession})
		return req
	}

	// Bodyless requests cannot be held open and carry no check.
	bodyless := newRequest(nil)
	if !CheckAuth(cfg, httptest.NewRecorder(), bodyless) || bodyless.Body != http.NoBody {
		t.Fatal("bodyless request was bound")
	}

	req := newRequest(strings.NewReader(`{"name":"parked"}`))
	if !CheckAuth(cfg, httptest.NewRecorder(), req) || !CheckAuth(cfg, httptest.NewRecorder(), req) {
		t.Fatal("session admission failed")
	}
	bound, ok := req.Body.(*authorityBoundBody)
	if !ok {
		t.Fatal("admitted body was not bound to its session")
	}
	if _, nested := bound.ReadCloser.(*authorityBoundBody); nested {
		t.Fatal("repeated admission nested the body check")
	}
	first := make([]byte, 4)
	if n, err := req.Body.Read(first); n != len(first) || err != nil {
		t.Fatalf("valid session read n=%d err=%v", n, err)
	}

	// An explicit token now owns the request; the session no longer decides.
	req.Header.Set("X-API-Token", parkedBodyToken)
	if !CheckAuth(cfg, httptest.NewRecorder(), req) {
		t.Fatal("token admission failed")
	}
	GetSessionStore().InvalidateSession(parkedBodySession)
	if n, err := req.Body.Read(first); n != len(first) || err != nil {
		t.Fatalf("token-owned body followed the unselected session: n=%d err=%v", n, err)
	}
	config.Mu.Lock()
	cfg.APITokens = nil
	config.Mu.Unlock()
	rest, err := io.ReadAll(req.Body)
	if len(rest) != 0 || !errors.Is(err, errRequestAuthorityWithdrawn) || errors.Is(err, io.EOF) {
		t.Fatalf("withdrawn token read %q, err=%v", rest, err)
	}
	// Withdrawal is final, whatever this request is admitted with afterwards.
	config.Mu.Lock()
	cfg.APITokens = []config.APITokenRecord{newTokenRecord(t, parkedBodyToken, []string{config.ScopeWildcard}, nil)}
	config.Mu.Unlock()
	if !CheckAuth(cfg, httptest.NewRecorder(), req) {
		t.Fatal("reissued token admission failed")
	}
	if _, err := req.Body.Read(first); !errors.Is(err, errRequestAuthorityWithdrawn) {
		t.Fatalf("withdrawn request resumed reading its body: %v", err)
	}
}
