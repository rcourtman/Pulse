package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// Write a real HTTP/1.1 status line: WriteHeader would substitute Go's standard
// reason phrase and miss the provider-controlled field this regression covers.
func writeOIDCDiscoveryStatus(t *testing.T, w http.ResponseWriter, status int, reason string) {
	t.Helper()
	conn, buf, err := w.(http.Hijacker).Hijack()
	require.NoError(t, err)
	defer conn.Close()
	const body = "SYNTHETIC_BODY_PRIVATE_MARKER"
	_, err = fmt.Fprintf(buf, "HTTP/1.1 %d %s\r\nContent-Length: %d\r\nX-Provider-Debug: SYNTHETIC_HEADER_PRIVATE\r\nConnection: close\r\n\r\n%s", status, reason, len(body), body)
	require.NoError(t, err)
	require.NoError(t, buf.Flush())
}

func TestOIDCConnectionHTTPStatusPrivacy(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		reason string
	}{
		{"redirect without location", 302, "SYNTHETIC_REASON_PRIVATE"},
		{"unauthorised", 401, "SYNTHETIC_REASON_PRIVATE"},
		{"forbidden", 403, "SYNTHETIC_REASON_PRIVATE"},
		{"not found", 404, "Not Found"},
		{"limited", 429, "SYNTHETIC_REASON_PRIVATE"},
		{"server error", 500, "SYNTHETIC_REASON_PRIVATE"},
		{"custom unavailable", 503, "https://idp.example.test/private?secret=SYNTHETIC_REASON_PRIVATE"},
		{"long unavailable", 503, strings.Repeat("SYNTHETIC_REASON_PRIVATE_", 350)},
		{"markup", 503, "<script>SYNTHETIC_REASON_PRIVATE</script>"},
		{"empty", 503, ""},
		{"nonstandard code", 599, "SYNTHETIC_REASON_PRIVATE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests.Add(1)
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/realm/.well-known/openid-configuration", req.URL.Path)
				writeOIDCDiscoveryStatus(t, w, tt.status, tt.reason)
			}))
			defer server.Close()
			result := (&Router{}).testOIDCConnection(t.Context(), &OIDCTestConfig{IssuerURL: server.URL + "/realm"})
			require.False(t, result.Success)
			require.Nil(t, result.Details)
			require.Equal(t, fmt.Sprintf("HTTP %d", tt.status), result.Error)
			require.Equal(t, "OIDC discovery returned an unexpected HTTP status. Check the Issuer URL and the provider or proxy's access settings. Nothing was saved; sign-in was not tested.", result.Message)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			require.Less(t, len(encoded), 220, "upstream text must not affect the diagnostic size")
			for _, private := range []string{"SYNTHETIC_", server.URL, "idp.example.test", "<script>"} {
				require.NotContains(t, string(encoded), private)
			}
			require.Equal(t, int32(1), requests.Load(), "failed discovery must not retry or call token/JWKS endpoints")
		})
	}
}

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
