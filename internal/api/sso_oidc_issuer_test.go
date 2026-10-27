package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// TestOIDCConnectionIssuerParity compares the diagnostic with the actual
// sign-in service, not a second implementation of the issuer comparison.
func TestOIDCConnectionIssuerParity(t *testing.T) {
	for _, tt := range []struct {
		name       string
		configured string
		discovered string
		omit       bool
		wantError  string
	}{
		{name: "exact", configured: "/realms/pulse", discovered: "/realms/pulse"},
		{name: "exact trailing slash", configured: "/realms/pulse/", discovered: "/realms/pulse/"},
		{name: "configured trailing slash", configured: "/realms/pulse/", discovered: "/realms/pulse", wantError: "issuer_mismatch"},
		{name: "discovered trailing slash", configured: "/realms/pulse", discovered: "/realms/pulse/", wantError: "issuer_mismatch"},
		{name: "case mismatch", configured: "/realms/pulse", discovered: "/realms/Pulse", wantError: "issuer_mismatch"},
		{name: "different realm", configured: "/realms/pulse", discovered: "/realms/other", wantError: "issuer_mismatch"},
		{name: "different authority", configured: "/realms/pulse", discovered: "https://foreign.example.test/realms/pulse", wantError: "issuer_mismatch"},
		{name: "omitted issuer", configured: "/realms/pulse", omit: true, wantError: "missing_discovery_issuer"},
		{name: "empty issuer", configured: "/realms/pulse", wantError: "missing_discovery_issuer"},
		{name: "whitespace issuer", configured: "/realms/pulse", discovered: " ", wantError: "issuer_mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests.Add(1)
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, strings.TrimRight(tt.configured, "/")+"/.well-known/openid-configuration", req.URL.Path)
				base := "http://" + req.Host
				document := map[string]interface{}{
					"authorization_endpoint": base + "/authorize",
					"token_endpoint":         base + "/token",
					"jwks_uri":               base + "/jwks",
				}
				if !tt.omit {
					issuer := tt.discovered
					if strings.HasPrefix(issuer, "/") {
						issuer = base + issuer
					}
					document["issuer"] = issuer
				}
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(document))
			}))
			defer server.Close()
			issuer := server.URL + tt.configured

			response := (&Router{}).testOIDCConnection(t.Context(), &OIDCTestConfig{IssuerURL: issuer})
			require.Equal(t, tt.wantError == "", response.Success)
			require.Equal(t, tt.wantError, response.Error)
			if tt.wantError == "" {
				require.Equal(t, issuer, response.Details.EntityID)
				require.Contains(t, response.Message, "Client credentials and sign-in have not been tested.")
			} else {
				require.Nil(t, response.Details, "untrusted discovery values must not be presented as validated details")
				require.NotContains(t, response.Message, server.URL)
				require.NotContains(t, response.Message, "foreign.example.test")
			}

			service, err := NewOIDCService(t.Context(), &config.OIDCConfig{
				Enabled: true, IssuerURL: issuer, ClientID: "synthetic-client",
				ClientSecret: "synthetic-not-tested", RedirectURL: "https://pulse.example.test/api/oidc/callback",
				Scopes: []string{"openid"},
			})
			if service != nil {
				defer service.stateStore.Stop()
			}
			require.Equal(t, response.Success, err == nil, "diagnostic and real sign-in discovery disagree: %v", err)
			require.Equal(t, int32(2), requests.Load(), "each discovery is one read-only fetch, without token or JWKS requests")
		})
	}
}
