package proxmox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise the actual client constructors and both password encodings, not
// just a synthetic Authorization header on a general-purpose HTTP client.
func TestInfrastructureCredentialRedirectCannotLeaveOrigin(t *testing.T) {
	for _, trust := range []string{"explicit-opt-out", "pin"} {
		for _, auth := range []string{"token", "ticket-and-csrf", "password-json", "password-form"} {
			t.Run(trust+"/"+auth, func(t *testing.T) {
				var sourceRequests, destinationRequests atomic.Int32
				sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					destinationRequests.Add(1)
					io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"data":{"ticket":"synthetic-ticket","CSRFPreventionToken":"synthetic-csrf"}}`))
				}))
				defer sink.Close()
				source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					sourceRequests.Add(1)
					if auth == "password-form" && r.Header.Get("Content-Type") == "application/json" {
						http.Error(w, "form required", http.StatusUnsupportedMediaType)
						return
					}
					http.Redirect(w, r, sink.URL+"/redirected", http.StatusTemporaryRedirect)
				}))
				defer source.Close()
				cfg := ClientConfig{Host: source.URL, TokenName: "synthetic@pam!pulse", TokenValue: "synthetic-token"}
				if trust == "pin" {
					cfg.VerifySSL = true
					digest := sha256.Sum256(source.Certificate().Raw)
					cfg.Fingerprint = hex.EncodeToString(digest[:])
				}
				var err error
				if strings.HasPrefix(auth, "password-") {
					cfg.TokenName, cfg.TokenValue = "", ""
					cfg.User, cfg.Password = "synthetic@pam", "synthetic-password"
					_, err = NewClient(cfg)
				} else {
					var client *Client
					client, err = NewClient(cfg)
					if err != nil {
						t.Fatal(err)
					}
					defer client.httpClient.CloseIdleConnections()
					if auth == "ticket-and-csrf" {
						client.auth.tokenName, client.auth.tokenValue = "", ""
						client.auth.ticket, client.auth.csrfToken = "synthetic-ticket", "synthetic-csrf"
						client.auth.expiresAt = time.Now().Add(time.Hour)
					}
					var response *http.Response
					response, err = client.request(context.Background(), "POST", "/synthetic-action", url.Values{"action": {"synthetic-action"}})
					if response != nil {
						response.Body.Close()
					}
				}
				wantSource := int32(1)
				if auth == "password-form" {
					wantSource = 2
				}
				if err == nil || !strings.Contains(err.Error(), "same origin") || destinationRequests.Load() != 0 || sourceRequests.Load() != wantSource {
					t.Fatalf("credential redirect: source=%d want=%d destination=%d err=%v",
						sourceRequests.Load(), wantSource, destinationRequests.Load(), err)
				}
			})
		}
	}
}

func TestInfrastructureTokenSameOriginProxyRedirect(t *testing.T) {
	var requests atomic.Int32
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") == "" {
			t.Error("same-origin request lost token authentication")
		}
		if r.URL.Path == "/api2/json/version" {
			http.Redirect(w, r, "/proxy/api2/json/version", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path != "/proxy/api2/json/version" {
			t.Errorf("unexpected proxy path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{}}`))
	}))
	defer source.Close()
	client, err := NewClient(ClientConfig{Host: source.URL, TokenName: "synthetic@pam!pulse", TokenValue: "synthetic-token"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.httpClient.CloseIdleConnections()
	response, err := client.get(context.Background(), "/version")
	if response != nil {
		response.Body.Close()
	}
	if err != nil || requests.Load() != 2 {
		t.Fatalf("same-origin proxy path stopped: requests=%d err=%v", requests.Load(), err)
	}
}
