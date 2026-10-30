package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSSOProviderNameUTF8Boundary(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
	}{
		{"short", "Équipe 東京 🔐", "Équipe 東京 🔐"},
		{"ascii-exact", strings.Repeat("a", 128), strings.Repeat("a", 128)},
		{"ascii-over", strings.Repeat("a", 129), strings.Repeat("a", 128)},
		{"two-byte-exact", strings.Repeat("é", 64), strings.Repeat("é", 64)},
		{"two-byte-cut", strings.Repeat("a", 127) + "é", strings.Repeat("a", 127)},
		{"three-byte-cut-one", strings.Repeat("a", 127) + "界", strings.Repeat("a", 127)},
		{"three-byte-cut-two", strings.Repeat("a", 126) + "界", strings.Repeat("a", 126)},
		{"four-byte-cut-one", strings.Repeat("a", 127) + "🔐", strings.Repeat("a", 127)},
		{"four-byte-cut-two", strings.Repeat("a", 126) + "🔐", strings.Repeat("a", 126)},
		{"four-byte-cut-three", strings.Repeat("a", 125) + "🔐", strings.Repeat("a", 125)},
		{"mixed-boundary", strings.Repeat("界", 42) + "é-more", strings.Repeat("界", 42) + "é"},
		{"literal-replacement-kept", strings.Repeat("a", 125) + "\ufffd-more", strings.Repeat("a", 125) + "\ufffd"},
		{"literal-replacement-cut", strings.Repeat("a", 127) + "\ufffd", strings.Repeat("a", 127)},
		{"combining-codepoint-cut", strings.Repeat("a", 127) + "\u0301", strings.Repeat("a", 127)},
		{"trim-before-bound", " \t" + strings.Repeat("界", 43) + "\n", strings.Repeat("界", 42)},
		{"control-removal", " A\x00B\nC\x7f ", "ABC"},
		{"no-backfill-after-controls", "a\x00" + strings.Repeat("b", 126) + "é", "a" + strings.Repeat("b", 126)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeProviderName(tt.input)
			require.Equal(t, tt.want, got)
			require.True(t, utf8.ValidString(got))
			require.LessOrEqual(t, len(got), maxProviderNameLength, "the limit is bytes, not runes")
			require.Equal(t, got, sanitizeProviderName(got), "sanitization must be stable on a saved label")
		})
	}
}

// Exercise the registered Community routes and persisted/list/detail/public
// projections. Providers stay disabled during writes: no real IdP is contacted.
func TestSSOProviderNameHTTPRoundTrip(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			resetPersistentAuthStoresForTests()
			t.Cleanup(resetPersistentAuthStoresForTests)
			const rawToken = "sso-name-contract-token.12345678"
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil))
			r := NewRouter(cfg, nil, nil, nil, nil, "test")
			cleanupTestRouter(t, r)
			request := func(method, path string, body any, authenticated bool) *httptest.ResponseRecorder {
				t.Helper()
				data, err := json.Marshal(body)
				require.NoError(t, err)
				req := httptest.NewRequest(method, path, bytes.NewReader(data))
				req.RemoteAddr = ssoTestIP()
				req.Header.Set("Content-Type", "application/json")
				if authenticated {
					req.Header.Set("X-API-Token", rawToken)
				}
				rec := httptest.NewRecorder()
				r.Handler().ServeHTTP(rec, req)
				return rec
			}
			const collection = "/api/security/sso/providers"
			for _, tt := range []struct {
				name, suffix string
				prefix       int
			}{
				{"two-byte", "é", 127}, {"three-byte", "界", 126}, {"four-byte", "🔐", 125},
			} {
				t.Run(tt.name, func(t *testing.T) {
					p := config.SSOProvider{ID: "name-" + tt.name, Name: strings.Repeat("a", tt.prefix) + tt.suffix, DisplayName: strings.Repeat("界", 42) + "🔒suffix", Type: kind}
					if kind == config.SSOProviderTypeOIDC {
						p.OIDC = &config.OIDCProviderConfig{IssuerURL: "https://identity.example.test", ClientID: "synthetic-client", ClientSecret: "synthetic-name-secret", Scopes: []string{"openid", "email"}}
					} else {
						p.SAML = &config.SAMLProviderConfig{IDPSSOURL: "https://identity.example.test/sso", SPEntityID: "pulse", SPPrivateKey: "synthetic-name-private-key"}
					}
					rec := request(http.MethodPost, collection, p, true)
					require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
					assertLabels := func(rec *httptest.ResponseRecorder, wantName, wantDisplay string) {
						t.Helper()
						require.NotContains(t, rec.Body.String(), "synthetic-name-secret")
						require.NotContains(t, rec.Body.String(), "synthetic-name-private-key")
						var got SSOProviderResponse
						require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
						require.Equal(t, p.ID, got.ID)
						require.Equal(t, wantName, got.Name)
						require.Equal(t, wantDisplay, got.DisplayName)
					}
					assertLabels(rec, strings.Repeat("a", tt.prefix), strings.Repeat("界", 42))
					p.ID = "payload-id-must-not-replace-route-id"
					p.Name = strings.Repeat("é", 63) + "🔑suffix"
					p.DisplayName = "企業 " + strings.Repeat("é", 60) + "🔐suffix"
					p.OIDC, p.SAML = nil, nil // as in a flat list-based save: keep credentials/config
					pathID := "name-" + tt.name
					rec = request(http.MethodPut, collection+"/"+pathID, p, true)
					p.ID = pathID
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
					wantName, wantDisplay := strings.Repeat("é", 63), "企業 "+strings.Repeat("é", 60)
					assertLabels(rec, wantName, wantDisplay)
					rec = request(http.MethodGet, collection+"/"+pathID, nil, true)
					require.Equal(t, http.StatusOK, rec.Code)
					assertLabels(rec, wantName, wantDisplay)
					loaded, err := r.persistence.LoadSSOConfig()
					require.NoError(t, err)
					stored := loaded.GetProvider(pathID)
					require.NotNil(t, stored)
					require.Equal(t, wantName, stored.Name)
					require.Equal(t, wantDisplay, stored.DisplayName)
					if p.Type == config.SSOProviderTypeOIDC {
						require.Equal(t, "synthetic-name-secret", stored.OIDC.ClientSecret)
					} else {
						require.Equal(t, "synthetic-name-private-key", stored.SAML.SPPrivateKey)
					}
					// Publish an enabled snapshot only for public discovery. This
					// does not initialise or claim a working identity provider.
					applyErr, saveErr := r.commitSSOConfig(func(c *config.SSOConfig) error { c.GetProvider(pathID).Enabled = true; return nil })
					require.NoError(t, applyErr)
					require.NoError(t, saveErr)
					rec = request(http.MethodGet, "/api/security/status", nil, false)
					require.Equal(t, http.StatusOK, rec.Code)
					require.NotContains(t, rec.Body.String(), "synthetic-name-secret")
					require.NotContains(t, rec.Body.String(), "synthetic-name-private-key")
					var status struct {
						Providers []SSOProviderResponse `json:"ssoProviders"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
					require.Len(t, status.Providers, 1)
					require.Equal(t, wantName, status.Providers[0].Name)
					require.Equal(t, wantDisplay, status.Providers[0].DisplayName)
					rec = request(http.MethodGet, collection, nil, true)
					require.Equal(t, http.StatusOK, rec.Code)
					var list struct {
						Providers []SSOProviderResponse `json:"providers"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
					require.Len(t, list.Providers, 1)
					require.Equal(t, wantName, list.Providers[0].Name)
					require.Equal(t, wantDisplay, list.Providers[0].DisplayName)
					rec = request(http.MethodDelete, collection+"/"+pathID, nil, true)
					require.Equal(t, http.StatusNoContent, rec.Code)
				})
			}
		})
	}
}

func TestSSOProviderNamePresentationKeepsRuntime(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			f := newPresentationFixture(t, kind)
			r := f.router
			oidc, saml := r.oidcManager.GetService(f.provider.ID), r.samlManager.GetService(f.provider.ID)
			before := f.requests.Load()
			p := f.provider
			p.Name, p.DisplayName = strings.Repeat("界", 42)+"🔐suffix", strings.Repeat("é", 63)+"🔑suffix"
			rec := presentationPut(t, r, p)
			require.Equal(t, http.StatusOK, rec.Code)
			got := r.ensureSSOConfig().GetProvider(p.ID)
			require.Equal(t, strings.Repeat("界", 42), got.Name)
			require.Equal(t, strings.Repeat("é", 63), got.DisplayName)
			require.Equal(t, before, f.requests.Load(), "a label-only save must not refetch IdP metadata")
			require.Same(t, oidc, r.oidcManager.GetService(p.ID))
			require.Same(t, saml, r.samlManager.GetService(p.ID))
		})
	}
}
