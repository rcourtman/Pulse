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
