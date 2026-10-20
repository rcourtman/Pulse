package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestDiagnosticsAPIStatusOutranksResponseText(t *testing.T) {
	for _, provider := range []string{"pve-api", "pbs-api", "pve-login", "pbs-login"} {
		for _, status := range []int{401, 403, 502} {
			t.Run(fmt.Sprintf("%s-%d", provider, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
					io.WriteString(w, "API error 401: permission denied; x509: fixture-certificate; connection timed out; private-fixture-body")
				}))
				defer server.Close()
				var err error
				switch provider {
				case "pve-api":
					client, initErr := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!reader", TokenValue: "fixture-token"})
					if initErr != nil {
						t.Fatal(initErr)
					}
					_, err = client.GetNodes(context.Background())
				case "pbs-api":
					client, initErr := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-token"})
					if initErr != nil {
						t.Fatal(initErr)
					}
					_, err = client.GetVersion(context.Background())
				case "pve-login":
					_, err = proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, User: "fixture@pve", Password: "fixture-password"})
				case "pbs-login":
					_, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, User: "fixture@pbs", Password: "fixture-password"})
				}
				if err == nil {
					t.Fatal("HTTP refusal was accepted")
				}
				failure := classifyDiagnosticsConnectionFailure(fmt.Errorf("fixture wrapper: %w", err), "probe failed")
				want := map[int]string{401: "proxmox_auth_rejected", 403: "proxmox_permission_denied", 502: "proxmox_api_error"}[status]
				if failure.kind != want {
					t.Fatalf("wire %d classified from quoted body as %q, want %q", status, failure.kind, want)
				}
				if strings.Contains(failure.message+failure.troubleshooting, "private-fixture-body") || strings.Contains(failure.message, "fixture-certificate") {
					t.Fatalf("diagnosis disclosed provider text: %+v", failure)
				}
				if status == 403 {
					for _, want := range []string{"user's and token's ACLs", "inheritance", "read privilege", "intended resource path", "intentional access boundaries", "do not rotate", "disable privilege separation", "grant Administrator"} {
						if !strings.Contains(failure.troubleshooting, want) {
							t.Errorf("403 guidance omits %q: %s", want, failure.troubleshooting)
						}
					}
				}
				if status == 401 && (!strings.Contains(failure.troubleshooting, "only when its invalidity is confirmed") || strings.Contains(failure.troubleshooting, "Rotate the node credentials")) {
					t.Fatalf("401 still prescribes speculative rotation: %s", failure.troubleshooting)
				}
				if status == 502 && !strings.Contains(failure.message, "HTTP 502") {
					t.Fatalf("outer gateway status lost: %+v", failure)
				}
			})
		}
	}
}

func TestDiagnosticsSerializedAPIStatusMarkers(t *testing.T) {
	for _, tc := range []struct{ message, kind string }{
		{"authentication error: API error 403 (Forbidden): provider says authentication failed", "proxmox_permission_denied"},
		{"poll failed: authentication error: API error 502: API error 403: permission denied", "proxmox_api_error"},
		{"authentication failed (status 403): API error 401: fixture", "proxmox_permission_denied"},
		{"API error 401 (Unauthorized): private-fixture-body", "proxmox_auth_rejected"},
		{"API error 502: connection timed out; x509: private-fixture-body", "proxmox_api_error"},
		{"API request failed (HTTP 429 Too Many Requests)", "proxmox_api_error"},
		{"poll failed: API request failed (HTTP 503 Service Unavailable): API error 401: fixture", "proxmox_api_error"},
		{"authentication failed: authentication failed (HTTP 403 Forbidden)", "proxmox_permission_denied"},
		{"API request failed (HTTP 401 Unauthorized)", "proxmox_auth_rejected"},
		{"API request failed (HTTP 503 Service Unavailable): could not read response body", "proxmox_api_error"},
		{"authentication failed (HTTP 429 Too Many Requests): response body exceeds 8388608 bytes", "proxmox_api_error"},
		{"poll failed: API request failed (HTTP 503): connection timed out", "proxmox_api_error"},
		{"API request failed (HTTP 4030): unrelated number", "connection_failed"},
		{"API request failed (HTTP 403suffix): unrelated identifier", "connection_failed"},
		{"Get https://HTTP-503.fixture:403: unexpected EOF", "connection_failed"},
		{"fixture HTTP 503: unexpected EOF", "connection_failed"},
		{"fixture response says API request failed (HTTP 503)", "connection_failed"},
		{"Get https://node401.fixture:403: unexpected EOF", "connection_failed"},
		{"fixture token-id-403401: unexpected EOF", "connection_failed"},
		{"API error 4030: fixture unrelated number", "connection_failed"},
		{"permission denied", "proxmox_permission_denied"},
		{"context deadline exceeded", "connection_timeout"},
		{"x509: certificate signed by unknown authority", "tls_certificate_mismatch"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			failure := classifyDiagnosticsConnectionFailure(fmt.Errorf("%s", tc.message), "probe failed")
			if failure.kind != tc.kind {
				t.Fatalf("%q = %q, want %q", tc.message, failure.kind, tc.kind)
			}
		})
	}
}
