package pmg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Non-parallel: restore the global logger before parallel tests can run.
// No listener or provider is used: invalid hosts must fail before login.
func TestClientRejectedHostDiagnosticsContainNoCredentials(t *testing.T) {
	for index, host := range []string{
		"https://synthetic-user:synthetic-secret@pmg.invalid",
		"http://synthetic-user:synthetic-secret@pmg.invalid",
		"synthetic-user:synthetic-secret@pmg.invalid",
		"https://pmg.invalid?token=synthetic-secret",
		"http://pmg.invalid#synthetic-secret",
		"pmg.invalid?token=synthetic-secret",
		"https://synthetic-user:synthetic-secret@pmg.invalid:bad-port",
		"http://synthetic-user:synthetic-secret@pmg.invalid:bad-port",
		"https://pmg.invalid/%invalid?token=synthetic-secret",
		"http://pmg.invalid/%invalid?token=synthetic-secret",
	} {
		for _, mode := range []string{"token", "password"} {
			t.Run(fmt.Sprintf("case-%d/%s", index, mode), func(t *testing.T) {
				var captured bytes.Buffer
				prior := log.Logger
				log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
				defer func() { log.Logger = prior }()
				cfg := ClientConfig{Host: host, User: "fixture@pmg", Password: "synthetic-password"}
				if mode == "token" {
					cfg.Password, cfg.TokenName, cfg.TokenValue = "", "fixture@pmg!pulse", "synthetic-token"
				}
				client, err := NewClient(cfg)
				if client != nil || err == nil {
					t.Fatal("invalid URL must be refused before authentication")
				}
				for _, marker := range []string{"synthetic-secret", "synthetic-user", "synthetic-token", "synthetic-password"} {
					if strings.Contains(err.Error(), marker) {
						t.Error("constructor error disclosed rejected URL or authentication input")
					}
					if strings.Contains(captured.String(), marker) {
						t.Error("constructor log disclosed rejected URL or authentication input")
					}
				}
				t.Logf("refused=true log_bytes=%d", captured.Len())
			})
		}
	}
}

func TestClientAcceptedHostDiagnosticsRemainUseful(t *testing.T) {
	for _, host := range []string{"pmg.invalid/tenant", "http://pmg.invalid:8006/tenant", "https://pmg.invalid:8443/tenant", "https://[2001:db8::1]:8006/tenant"} {
		for _, trust := range []struct {
			name, fingerprint string
			verify            bool
		}{
			{"ca", "", true},
			{"pin", strings.Repeat("a", 64), true},
			{"explicit-opt-out", "", false},
		} {
			t.Run(host+"/"+trust.name, func(t *testing.T) {
				var captured bytes.Buffer
				prior := log.Logger
				log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
				defer func() { log.Logger = prior }()
				client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pmg!pulse", TokenValue: "synthetic-token", VerifySSL: trust.verify, Fingerprint: trust.fingerprint, Timeout: 23 * time.Second})
				if err != nil {
					t.Fatal(err)
				}
				defer client.httpClient.CloseIdleConnections()
				if !strings.HasSuffix(client.baseURL, "/tenant/api2/json") || client.config.Fingerprint != trust.fingerprint || client.config.VerifySSL != trust.verify || client.auth.tokenValue != "synthetic-token" || client.httpClient.Timeout != 23*time.Second {
					t.Fatal("diagnostic filtering changed routing, credentials, trust or timeout")
				}
				transport := client.httpClient.Transport.(*http.Transport)
				if transport.TLSClientConfig.InsecureSkipVerify != (trust.fingerprint != "" || !trust.verify) || (trust.fingerprint != "" && transport.TLSClientConfig.VerifyPeerCertificate == nil) || client.httpClient.CheckRedirect == nil {
					t.Fatal("diagnostic filtering changed the TLS or redirect policy")
				}
				if strings.Contains(captured.String(), "synthetic-token") {
					t.Fatal("accepted diagnostic disclosed authentication input")
				}
				var events []map[string]interface{}
				decoder := json.NewDecoder(&captured)
				for decoder.More() {
					var event map[string]interface{}
					if err := decoder.Decode(&event); err != nil {
						t.Fatal(err)
					}
					events = append(events, event)
				}
				if strings.HasPrefix(host, "http://") {
					if len(events) != 1 || events[0]["host"] != host || events[0]["level"] != "warn" {
						t.Fatal("accepted HTTP endpoint lost its useful warning")
					}
				} else if len(events) != 0 {
					t.Fatal("HTTPS construction gained an unnecessary diagnostic")
				}
			})
		}
	}
}

// Token construction is socket-free. URL parsing must never reinterpret an
// explicit scheme as an HTTPS hostname before an authenticated request is built.
func TestClientEndpointAuthorityNormalization(t *testing.T) {
	for _, tc := range []struct{ name, host, want string }{
		{"upper-https", "HTTPS://pve.example.invalid:8443/tenant", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"mixed-https", "hTtPs://pve.example.invalid:8443/tenant/", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"upper-http", "HTTP://pve.example.invalid:8080/tenant", "http://pve.example.invalid:8080/tenant/api2/json"},
		{"trimmed-bare", "  pve.example.invalid:8443/tenant  ", "https://pve.example.invalid:8443/tenant/api2/json"},
		{"ipv6", "HTTPS://[2001:db8::1]:8443/tenant", "https://[2001:db8::1]:8443/tenant/api2/json"},
		{"zoned-ipv6", "HTTPS://[fe80::1%25ZoneA]:8443/tenant", "https://[fe80::1%25ZoneA]:8443/tenant/api2/json"},
		{"ordinary", "https://pve.example.invalid:8443/tenant", "https://pve.example.invalid:8443/tenant/api2/json"},
	} {
		for _, trust := range []struct {
			name, pin string
			verify    bool
		}{{"ca", "", true}, {"pin", strings.Repeat("a", 64), true}, {"explicit-opt-out", "", false}} {
			t.Run(tc.name+"/"+trust.name, func(t *testing.T) {
				cfg := ClientConfig{Host: tc.host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: trust.verify, Fingerprint: trust.pin, Timeout: 3 * time.Second}
				client, err := NewClient(cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer client.httpClient.CloseIdleConnections()
				if client.baseURL != tc.want {
					t.Errorf("request destination = %q, want %q", client.baseURL, tc.want)
				}
				if client.config.VerifySSL != cfg.VerifySSL || client.config.Fingerprint != cfg.Fingerprint || client.auth.tokenValue != cfg.TokenValue || client.httpClient.Timeout != cfg.Timeout {
					t.Error("URL normalization changed trust, credentials or request budget")
				}
				transport := client.httpClient.Transport.(*http.Transport)
				if transport.TLSClientConfig.InsecureSkipVerify != (trust.pin != "" || !trust.verify) || (trust.pin != "" && transport.TLSClientConfig.VerifyPeerCertificate == nil) || client.httpClient.CheckRedirect == nil {
					t.Error("URL normalization changed TLS or redirect policy")
				}
			})
		}
	}
}

func TestClientEndpointExplicitSchemeRefused(t *testing.T) {
	for _, host := range []string{
		"ftp://pve.example.invalid:8006/tenant", "ws://pve.example.invalid:8006/tenant", "wss://pve.example.invalid:8006/tenant",
		"file://pve.example.invalid/tenant", "ssh://pve.example.invalid:22/tenant", "https://fixture:synthetic-secret@pve.example.invalid",
		"HTTPS://fixture:synthetic-secret@pve.example.invalid", "HTTPS://pve.example.invalid?token=synthetic-secret", "HTTPS://pve.example.invalid#synthetic-secret",
	} {
		t.Run(host, func(t *testing.T) {
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pam!pulse", TokenValue: "synthetic-token", VerifySSL: true})
			if err == nil || client != nil {
				if client != nil {
					client.httpClient.CloseIdleConnections()
				}
				t.Fatal("invalid endpoint became an authenticated client")
			}
			if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-token") {
				t.Error("rejected URL diagnostic disclosed fixture credentials")
			}
		})
	}
}
