package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// Hold the request at the real body-read boundary, after initial route lookup.
// No sleeps, production hooks or persistence locks choose the interleaving.
type ssoUpdateHeldBody struct {
	io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (b *ssoUpdateHeldBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.Reader.Read(p)
}

func ssoUpdateJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func ssoUpdateRequest(r *Router, id string, body io.Reader) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.handleSSOProvider(rec, httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/"+id, body))
	return rec
}

func ssoStartHeldUpdate(t *testing.T, r *Router, id string, payload any) func() *httptest.ResponseRecorder {
	t.Helper()
	body := &ssoUpdateHeldBody{Reader: bytes.NewReader(ssoUpdateJSON(t, payload)), entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	done := make(chan struct{})
	var rec *httptest.ResponseRecorder
	go func() { defer close(done); rec = ssoUpdateRequest(r, id, body) }()
	finish := func() *httptest.ResponseRecorder {
		t.Helper()
		release.Do(func() { close(body.release) })
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("held SSO update did not finish")
		}
		return rec
	}
	t.Cleanup(func() { finish() })
	select {
	case <-body.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("SSO update did not enter body read")
	}
	return finish
}

func TestSSOUpdatePreservesLatestCommittedSettings(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		for _, mode := range []string{"flat", "null", "detail"} {
			t.Run(string(kind)+"/"+mode, func(t *testing.T) {
				r, original := seedUnrepresentedSSO(t, kind)
				oldSnapshot := r.ensureSSOConfig()
				oldBytes := ssoUpdateJSON(t, oldSnapshot)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(ssoUpdateJSON(t, providerToDetailResponse(&original, r.config.PublicURL, httptest.NewRequest("GET", "/", nil))), &payload))
				payload["name"] = "Renamed SSO"
				if mode == "flat" {
					delete(payload, string(kind))
				}
				if mode == "null" {
					payload[string(kind)] = nil
				}
				finish := ssoStartHeldUpdate(t, r, original.ID, payload)

				// A second complete HTTP save rotates credentials and every unrepresented
				// setting while the first edit is still reading its body.
				current := (&config.SSOConfig{Providers: []config.SSOProvider{original}}).Clone().Providers[0]
				var nested reflect.Value
				if kind == config.SSOProviderTypeOIDC {
					current.OIDC.ClientSecret = "synthetic-current-oidc-secret"
					current.OIDC.IssuerURL = "https://current.idp.example.test/oidc"
					nested = reflect.ValueOf(current.OIDC).Elem()
				} else {
					current.SAML.IDPSSOURL = "https://current.idp.example.test/sso"
					nested = reflect.ValueOf(current.SAML).Elem()
				}
				for _, field := range unrepresentedSSOFields(kind) {
					value := nested.FieldByIndex(field.Index)
					if value.Kind() == reflect.Bool {
						value.SetBool(!value.Bool())
					} else {
						value.SetString(value.String() + "-current")
					}
				}
				fast := ssoUpdateRequest(r, original.ID, bytes.NewReader(ssoUpdateJSON(t, current)))
				require.Equal(t, http.StatusOK, fast.Code, fast.Body.String())
				committed := r.ensureSSOConfig()
				committedBytes := ssoUpdateJSON(t, committed)
				slow := finish()
				require.Equal(t, http.StatusOK, slow.Code, slow.Body.String())
				require.Equal(t, oldBytes, ssoUpdateJSON(t, oldSnapshot), "initial published snapshot mutated")
				require.Equal(t, committedBytes, ssoUpdateJSON(t, committed), "intervening published snapshot mutated")

				saved, err := r.persistence.LoadSSOConfig()
				require.NoError(t, err)
				actual := saved.GetProvider(original.ID)
				require.NotNil(t, actual)
				require.Equal(t, "Renamed SSO", actual.Name)
				require.Equal(t, actual, r.ensureSSOConfig().GetProvider(original.ID), "live/durable state diverged")
				fresh := &Router{persistence: r.persistence, config: r.config}
				require.Equal(t, actual, fresh.ensureSSOConfig().GetProvider(original.ID), "reopen lost settings")
				var got reflect.Value
				if kind == config.SSOProviderTypeOIDC {
					got = reflect.ValueOf(actual.OIDC).Elem()
					if actual.OIDC.ClientSecret != current.OIDC.ClientSecret {
						t.Error("delayed blank-secret edit restored an obsolete OIDC credential")
					}
					require.True(t, actual.OIDC.ClientSecretSet)
				} else {
					got = reflect.ValueOf(actual.SAML).Elem()
				}
				for _, field := range unrepresentedSSOFields(kind) {
					if !reflect.DeepEqual(nested.FieldByIndex(field.Index).Interface(), got.FieldByIndex(field.Index).Interface()) {
						t.Errorf("delayed edit restored obsolete %s", field.Name)
					}
				}
				if mode != "detail" {
					if kind == config.SSOProviderTypeOIDC && actual.OIDC.IssuerURL != current.OIDC.IssuerURL {
						t.Error("flat edit restored obsolete OIDC issuer")
					}
					if kind == config.SSOProviderTypeSAML && actual.SAML.IDPSSOURL != current.SAML.IDPSSOURL {
						t.Error("flat edit restored obsolete SAML endpoint")
					}
				}
				list, detail := httptest.NewRecorder(), httptest.NewRecorder()
				r.handleListSSOProviders(list, httptest.NewRequest("GET", "/api/security/sso/providers", nil))
				r.handleGetSSOProvider(detail, httptest.NewRequest("GET", "/api/security/sso/providers/advanced", nil), original.ID)
				for _, rec := range []*httptest.ResponseRecorder{fast, slow, list, detail} {
					for _, secret := range []string{"synthetic-oidc-secret", "synthetic-current-oidc-secret", "synthetic-private-key-canary"} {
						require.NotContains(t, rec.Body.String(), secret)
					}
				}
				if kind == config.SSOProviderTypeOIDC && mode == "flat" {
					t.Logf("SSO_CONCURRENCY_FRAME %s", ssoUpdateJSON(t, map[string]json.RawMessage{"list": list.Body.Bytes(), "detail": detail.Body.Bytes(), "update": slow.Body.Bytes()}))
				}
			})
		}
	}
}

func TestSSOUpdateInterveningDeletionAndSaveFailure(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{true: "deleted", false: "save failure"}[deleted], func(t *testing.T) {
			r, original := seedUnrepresentedSSO(t, config.SSOProviderTypeOIDC)
			finish := ssoStartHeldUpdate(t, r, original.ID, map[string]any{"name": "Delayed edit", "type": "oidc"})
			current := original
			oidc := *original.OIDC
			oidc.ClientSecret = "synthetic-current-oidc-secret"
			current.OIDC = &oidc
			require.Equal(t, http.StatusOK, ssoUpdateRequest(r, original.ID, bytes.NewReader(ssoUpdateJSON(t, current))).Code)
			if deleted {
				rec := httptest.NewRecorder()
				r.handleSSOProvider(rec, httptest.NewRequest("DELETE", "/api/security/sso/providers/"+original.ID, nil))
				require.Equal(t, http.StatusNoContent, rec.Code)
			}
			before := ssoUpdateJSON(t, r.ensureSSOConfig())
			files := ssoAdmissionFiles(t, r.config.DataPath)
			if !deleted {
				r.persistence.SetFileSystem(failingWriteFileSystem{})
			}
			rec := finish()
			if deleted {
				require.Equal(t, http.StatusBadRequest, rec.Code)
			} else {
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			}
			require.Equal(t, before, ssoUpdateJSON(t, r.ensureSSOConfig()))
			require.Equal(t, files, ssoAdmissionFiles(t, r.config.DataPath))
		})
	}
}

func TestSSOUpdateLatestSecretRetainsLiveLoginBinding(t *testing.T) {
	f := newPresentationFixture(t, config.SSOProviderTypeOIDC)
	r, p := f.router, f.provider
	payload := providerToDetailResponse(&p, r.config.PublicURL, httptest.NewRequest("GET", "/", nil))
	payload.Name = "Delayed rename"
	finish := ssoStartHeldUpdate(t, r, p.ID, payload)
	current := p
	oidc := *p.OIDC
	oidc.ClientSecret = "synthetic-current-oidc-secret"
	current.OIDC = &oidc
	require.Equal(t, http.StatusOK, ssoUpdateRequest(r, p.ID, bytes.NewReader(ssoUpdateJSON(t, current))).Code)
	service := r.oidcManager.GetService(p.ID)
	require.NotNil(t, service)
	state, _, err := service.newStateEntry(p.ID, "/history")
	require.NoError(t, err)
	requests := f.requests.Load()
	require.Equal(t, http.StatusOK, finish().Code)
	require.Same(t, service, r.oidcManager.GetService(p.ID), "presentation-only merge replaced the latest live service")
	require.Equal(t, requests, f.requests.Load(), "presentation merge refetched IdP metadata")
	_, ok := service.peekState(state)
	require.True(t, ok, "delayed rename erased a pending current login")
	saved, err := r.persistence.LoadSSOConfig()
	require.NoError(t, err)
	require.True(t, saved.GetProvider(p.ID).OIDC.ClientSecret == oidc.ClientSecret, "delayed rename restored obsolete credential")
	require.True(t, service.Matches(ssoProviderToOIDCConfig(saved.GetProvider(p.ID), "")), "saved and live OIDC credential differ")
}

func TestSSOUpdateConcurrentExplicitSettingsStillReplace(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			r, p := seedUnrepresentedSSO(t, kind)
			nested := map[string]any{"clientSecret": "synthetic-explicit-secret", "caBundle": ""}
			if kind == config.SSOProviderTypeSAML {
				nested = map[string]any{"spPrivateKey": "", "wantAssertionsSigned": false}
			}
			finish := ssoStartHeldUpdate(t, r, p.ID, map[string]any{"name": p.Name, "type": kind, string(kind): nested})
			current := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
			if kind == config.SSOProviderTypeOIDC {
				current.OIDC.ClientSecret = "synthetic-current-oidc-secret"
				current.OIDC.CABundle = "/synthetic/current-trust.pem"
			} else {
				current.SAML.SPPrivateKey = "synthetic-current-signing-key"
				current.SAML.WantAssertionsSigned = true
			}
			require.Equal(t, http.StatusOK, ssoUpdateRequest(r, p.ID, bytes.NewReader(ssoUpdateJSON(t, current))).Code)
			require.Equal(t, http.StatusOK, finish().Code)
			saved, err := r.persistence.LoadSSOConfig()
			require.NoError(t, err)
			if kind == config.SSOProviderTypeOIDC {
				require.Equal(t, "synthetic-explicit-secret", saved.GetProvider(p.ID).OIDC.ClientSecret)
				require.Empty(t, saved.GetProvider(p.ID).OIDC.CABundle)
			} else {
				require.Empty(t, saved.GetProvider(p.ID).SAML.SPPrivateKey)
				require.False(t, saved.GetProvider(p.ID).SAML.WantAssertionsSigned)
			}
		})
	}
}

func TestSSOUpdateConcurrentRegisteredHTTP(t *testing.T) {
	for _, header := range []string{"X-API-Token", "Authorization"} {
		t.Run(header, func(t *testing.T) {
			resetPersistentAuthStoresForTests()
			t.Cleanup(resetPersistentAuthStoresForTests)
			const token = "sso-serialized-save-fixture.12345678"
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, token, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil))
			cfg.PublicURL = "https://pulse.example.test"
			r := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
			cleanupTestRouter(t, r)
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				req.RemoteAddr = ssoTestIP()
				if req.Header.Get("X-Synthetic-Held-Body") == "true" {
					req.Body = struct {
						io.Reader
						io.Closer
					}{&ssoUpdateHeldBody{Reader: req.Body, entered: entered, release: release}, req.Body}
				}
				r.Handler().ServeHTTP(w, req)
			}))
			t.Cleanup(server.Close)
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			client := &http.Client{Timeout: 15 * time.Second}
			type response struct {
				code int
				body []byte
				err  error
			}
			request := func(method, path string, body any, held bool) response {
				var data []byte
				if body != nil {
					var err error
					data, err = json.Marshal(body)
					if err != nil {
						return response{err: err}
					}
				}
				req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
				if err != nil {
					return response{err: err}
				}
				value := token
				if header == "Authorization" {
					value = "Bearer " + token
				}
				req.Header.Set(header, value)
				req.Header.Set("Content-Type", "application/json")
				if held {
					req.Header.Set("X-Synthetic-Held-Body", "true")
				}
				res, err := client.Do(req)
				if err != nil {
					return response{err: err}
				}
				defer res.Body.Close()
				raw, err := io.ReadAll(res.Body)
				return response{res.StatusCode, raw, err}
			}
			const collection = "/api/security/sso/providers"
			p := unrepresentedSSOFixture(config.SSOProviderTypeOIDC)
			p.SAML = nil
			created := request("POST", collection, p, false)
			require.NoError(t, created.err)
			require.Equal(t, http.StatusCreated, created.code)
			// Flat list payloads intentionally omit the full OIDC block.
			flat := providerToResponse(&p, cfg.PublicURL, httptest.NewRequest("GET", "/", nil))
			flat.Name = "Renamed SSO"
			done := make(chan response, 1)
			go func() { done <- request("PUT", collection+"/advanced", flat, true) }()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("registered update did not enter body read")
			}
			rotated := p
			oidc := *p.OIDC
			oidc.ClientSecret = "synthetic-current-oidc-secret"
			oidc.IssuerURL = "https://current.idp.example.test/oidc"
			rotated.OIDC = &oidc
			fast := request("PUT", collection+"/advanced", rotated, false)
			require.NoError(t, fast.err)
			require.Equal(t, http.StatusOK, fast.code)
			// An independent provider created in between must survive both saves.
			independent := rotated
			independent.ID, independent.Name = "independent", "Independent SSO"
			require.Equal(t, http.StatusCreated, request("POST", collection, independent, false).code)
			releaseOnce.Do(func() { close(release) })
			var slow response
			select {
			case slow = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("registered held update did not finish")
			}
			require.NoError(t, slow.err)
			require.Equal(t, http.StatusOK, slow.code)
			list, detail := request("GET", collection, nil, false), request("GET", collection+"/advanced", nil, false)
			require.NoError(t, list.err)
			require.NoError(t, detail.err)
			require.Equal(t, http.StatusOK, list.code)
			require.Equal(t, http.StatusOK, detail.code)
			loaded, err := r.persistence.LoadSSOConfig()
			require.NoError(t, err)
			actual := loaded.GetProvider("advanced")
			if actual.OIDC.ClientSecret != oidc.ClientSecret {
				t.Error("registered delayed edit restored obsolete credential")
			}
			if actual.OIDC.IssuerURL != oidc.IssuerURL {
				t.Error("registered delayed edit restored obsolete issuer")
			}
			require.Equal(t, independent, *loaded.GetProvider("independent"))
			for _, raw := range [][]byte{list.body, detail.body, slow.body, fast.body} {
				require.NotContains(t, string(raw), "synthetic-current-oidc-secret")
				require.NotContains(t, string(raw), "synthetic-oidc-secret")
			}
			if header == "X-API-Token" {
				t.Logf("SSO_HTTP_FRAME %s", ssoUpdateJSON(t, map[string]json.RawMessage{"list": list.body, "detail": detail.body, "update": slow.body}))
			}
		})
	}
}
