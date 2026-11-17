package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// All IdP traffic and principals are synthetic. Count every remote request,
// not just a successful discovery, through the registered public route.
func newOIDCCallbackRequestFixture(t *testing.T, providerID string, warm bool) (*Router, *atomic.Int32, *atomic.Bool, string) {
	t.Helper()
	var calls atomic.Int32
	var unavailable atomic.Bool
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if unavailable.Load() {
			http.Error(w, "synthetic discovery outage", http.StatusServiceUnavailable)
			return
		}
		base := "http://" + req.Host
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": base, "authorization_endpoint": base + "/authorize",
			"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
		})
	}))
	t.Cleanup(idp.Close)
	provider := newRecoveryTestProvider(providerID, true, "")
	provider.OIDC.IssuerURL = idp.URL
	r := newRecoveryTestRouter(t, []config.SSOProvider{provider}, nil)
	r.mux = http.NewServeMux()
	r.registerAuthSecurityInstallRoutes()
	t.Cleanup(func() { r.oidcManager.RemoveService(providerID) })
	pending := ""
	if warm {
		redirect := "https://pulse.example.test/api/oidc/" + providerID + "/callback"
		if err := r.oidcManager.EnsureProvider(t.Context(), providerID, &provider, redirect); err != nil {
			t.Fatal(err)
		}
		var err error
		pending, _, err = r.oidcManager.GetService(providerID).newStateEntry(providerID, "/existing-login")
		if err != nil {
			t.Fatal(err)
		}
	}
	return r, &calls, &unavailable, pending
}

func oidcCallbackRequest(r *Router, providerID, host string, query url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "https://"+host+"/api/oidc/"+providerID+"/callback?"+query.Encode(), nil)
	req.RemoteAddr = "192.0.2.87:4567"
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)
	return rec
}

func TestSSOOIDCCallbackRequestRefusedBeforeProviderWork(t *testing.T) {
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		for _, state := range []string{"missing", "unknown", "expired", "consumed"} {
			if !warm && (state == "expired" || state == "consumed") {
				continue
			}
			for _, outage := range []bool{false, true} {
				condition := "available"
				if outage {
					condition = "outage"
				}
				t.Run(mode+"/"+state+"/"+condition, func(t *testing.T) {
					r, calls, unavailable, pending := newOIDCCallbackRequestFixture(t, "callback-gate", warm)
					previousAuditManager := GetTenantAuditManager()
					SetTenantAuditManager(nil)
					t.Cleanup(func() { SetTenantAuditManager(previousAuditManager) })
					auditLog := &testAuditLogger{}
					setAuditLogger(t, auditLog)
					service := r.oidcManager.GetService("callback-gate")
					query := url.Values{"code": {"synthetic-code"}}
					want := "invalid_state"
					switch state {
					case "missing":
						want = "missing_state"
					case "unknown":
						query.Set("state", "never-issued")
					case "expired":
						query.Set("state", "synthetic-expired")
						service.stateStore.Put("synthetic-expired", &oidcStateEntry{ProviderID: "callback-gate", ExpiresAt: time.Now().Add(-time.Minute)})
					case "consumed":
						token, _, err := service.newStateEntry("callback-gate", "/used-login")
						if err != nil {
							t.Fatal(err)
						}
						service.consumeState(token)
						query.Set("state", token)
					}
					unavailable.Store(outage)
					before := calls.Load()
					// A different derived callback origin must not rediscover/replace a
					// service for a callback that has no usable pending authorization.
					rec := oidcCallbackRequest(r, "callback-gate", "alternate-pulse.example.test", query)
					if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?oidc=error&oidc_error="+want || len(rec.Result().Cookies()) != 0 {
						t.Fatalf("unexpected refused callback: status=%d location=%q cookies=%d", rec.Code, rec.Header().Get("Location"), len(rec.Result().Cookies()))
					}
					if calls.Load() != before || r.oidcManager.GetService("callback-gate") != service {
						t.Fatal("unusable callback reached the provider or replaced its service")
					}
					wantDetails := "Invalid or expired state"
					if state == "missing" {
						wantDetails = "Missing state parameter"
					}
					if len(auditLog.events) != 1 || auditLog.events[0].EventType != "sso_oidc_login" || auditLog.events[0].Success || auditLog.events[0].Details != wantDetails {
						t.Fatalf("refusal lost its existing audit diagnostic: %+v", auditLog.events)
					}
					if warm {
						if entry, ok := service.peekState(pending); !ok || entry.ReturnTo != "/existing-login" {
							t.Fatal("unusable callback discarded another pending login")
						}
						select {
						case <-service.stateStore.stopCleanup:
							t.Fatal("unusable callback stopped the live state owner")
						default:
						}
					}
				})
			}
		}
	}
}

func TestSSOOIDCCallbackRequestProviderErrorBeforeInitialization(t *testing.T) {
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		for _, hasState := range []bool{false, true} {
			name := "no-state"
			if hasState {
				name = "with-state"
			}
			t.Run(mode+"/"+name, func(t *testing.T) {
				r, calls, unavailable, pending := newOIDCCallbackRequestFixture(t, "callback-gate", warm)
				service := r.oidcManager.GetService("callback-gate")
				unavailable.Store(true)
				// A missing trust file would fail any initialization. A provider denial
				// is already complete and has never required or consumed login state.
				r.ssoConfig.Providers[0].OIDC.CABundle = filepath.Join(t.TempDir(), "missing-ca.pem")
				query := url.Values{"error": {"access_denied"}}
				if hasState {
					query.Set("state", pending)
					if pending == "" {
						query.Set("state", "never-issued")
					}
				}
				before := calls.Load()
				rec := oidcCallbackRequest(r, "callback-gate", "alternate-pulse.example.test", query)
				if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/?oidc=error&oidc_error=access_denied" || len(rec.Result().Cookies()) != 0 {
					t.Fatalf("provider denial was masked: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
				}
				if calls.Load() != before || r.oidcManager.GetService("callback-gate") != service {
					t.Fatal("provider denial initialized or replaced its service")
				}
				if warm {
					if _, ok := service.peekState(pending); !ok {
						t.Fatal("provider denial consumed another pending login")
					}
				}
			})
		}
	}
}

func TestSSOOIDCCallbackRequestTrustPrecedence(t *testing.T) {
	r, calls, _, pending := newOIDCCallbackRequestFixture(t, "callback-gate", true)
	original := r.oidcManager.GetService("callback-gate")
	r.ssoConfig.Providers[0].OIDC.CABundle = filepath.Join(t.TempDir(), "missing-ca.pem")
	for _, tc := range []struct{ name, state, want string }{
		{"unknown", "never-issued", "invalid_state"},
		{"pending", pending, "oidc_init_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := oidcCallbackRequest(r, "callback-gate", "pulse.example.test", url.Values{"state": {tc.state}, "code": {"synthetic-code"}})
			if rec.Header().Get("Location") != "/?oidc=error&oidc_error="+tc.want || len(rec.Result().Cookies()) != 0 {
				t.Fatalf("trust admission target=%q", rec.Header().Get("Location"))
			}
			if calls.Load() != 1 || r.oidcManager.GetService("callback-gate") != original {
				t.Fatal("invalid trust reached discovery or replaced the owner")
			}
			if _, ok := original.peekState(pending); !ok {
				t.Fatal("failed trust admission lost recoverable pending state")
			}
		})
	}
}

func TestSSOOIDCCallbackRequestAdmissionCompatibility(t *testing.T) {
	for _, providerID := range []string{"callback-gate", config.LegacyOIDCProviderID} {
		t.Run(providerID, func(t *testing.T) {
			r, calls, _, _ := newOIDCCallbackRequestFixture(t, providerID, false)
			path := "https://pulse.example.test/api/oidc/" + providerID + "/callback"
			if providerID == config.LegacyOIDCProviderID {
				path = "https://pulse.example.test/api/oidc/callback"
			}
			for _, tc := range []struct{ name, path, want string }{
				{"missing", path + "?code=unused", "missing_state"},
				{"unknown", path + "?state=never-issued&code=unused", "invalid_state"},
				{"provider-error", path + "?error=access_denied", "access_denied"},
				{"unknown-provider", "https://pulse.example.test/api/oidc/unknown/callback?state=never-issued", "provider_not_found"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					r.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
					if rec.Header().Get("Location") != "/?oidc=error&oidc_error="+tc.want || calls.Load() != 0 || r.oidcManager.GetService(providerID) != nil {
						t.Fatalf("admission target=%q calls=%d", rec.Header().Get("Location"), calls.Load())
					}
				})
			}
			r.ssoConfig.Providers[0].Enabled = false
			rec := oidcCallbackRequest(r, providerID, "pulse.example.test", url.Values{"state": {"never-issued"}})
			if !strings.Contains(rec.Header().Get("Location"), "provider_not_found") || calls.Load() != 0 {
				t.Fatal("disabled provider bypassed admission")
			}
		})
	}
}
