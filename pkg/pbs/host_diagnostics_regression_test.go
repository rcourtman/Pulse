package pbs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Non-parallel: temporarily replace the package logger, then restore it before
// parallel tests can run. Only synthetic URLs/credentials enter this fixture.
func TestClientRejectedHostDiagnosticsContainNoCredentials(t *testing.T) {
	for index, host := range []string{
		"https://synthetic-user:synthetic-secret@pve.invalid",
		"http://synthetic-user:synthetic-secret@pve.invalid",
		"synthetic-user:synthetic-secret@pve.invalid",
		"https://pve.invalid?token=synthetic-secret",
		"http://pve.invalid#synthetic-secret",
		"pve.invalid?token=synthetic-secret",
		"https://synthetic-user:synthetic-secret@pve.invalid:bad-port",
		"https://pve.invalid/%invalid?token=synthetic-secret",
	} {
		// Use a safe index rather than the supplied host as the test name/output.
		t.Run(fmt.Sprintf("case-%d", index), func(t *testing.T) {
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
			defer func() { log.Logger = prior }()
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token"})
			if client != nil || err == nil {
				t.Fatal("invalid URL must be refused before authentication")
			}
			if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-user") {
				t.Error("constructor diagnostic exposed rejected URL credentials")
			}
			if strings.Contains(captured.String(), "synthetic-secret") || strings.Contains(captured.String(), "synthetic-user") {
				t.Error("constructor log exposed rejected URL credentials")
			}
			t.Logf("refused=true captured_log_bytes=%d private_error=%t", captured.Len(), !strings.Contains(err.Error(), "synthetic-secret"))
		})
	}
}

func TestClientAcceptedHostDiagnosticsRemainUseful(t *testing.T) {
	for _, host := range []string{"pve.invalid/tenant", "http://pve.invalid/tenant", "https://pve.invalid/tenant"} {
		var captured bytes.Buffer
		prior := log.Logger
		log.Logger = zerolog.New(&captured).Level(zerolog.DebugLevel)
		client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pbs!pulse", TokenValue: "synthetic-token"})
		log.Logger = prior
		if err != nil {
			t.Fatal(err)
		}
		defer client.httpClient.CloseIdleConnections()
		if !strings.HasSuffix(client.baseURL, "/tenant/api2/json") {
			t.Fatal("diagnostic filtering changed the route")
		}
		if strings.Contains(captured.String(), "synthetic-token") {
			t.Fatal("accepted diagnostic disclosed a token")
		}
		decoder := json.NewDecoder(&captured)
		for decoder.More() {
			var event map[string]interface{}
			if err := decoder.Decode(&event); err != nil {
				t.Fatal(err)
			}
			if value, ok := event["host"].(string); ok {
				parsed, err := url.Parse(value)
				if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
					t.Fatal("host diagnostic is not a validated base URL")
				}
			}
		}
	}
}

// Initial authentication must obey the same caller lifetime as ordinary reads.
func TestClientConstructionHonoursContext(t *testing.T) {
	testClientConstructionHonoursContext(t)
}
