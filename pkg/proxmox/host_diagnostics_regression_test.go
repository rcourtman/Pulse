package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

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
			client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"})
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
		client, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"})
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

func TestClusterRejectedEndpointDiagnosticsContainNoCredentials(t *testing.T) {
	for _, host := range []string{
		"https://synthetic-user:synthetic-secret@pve.invalid",
		"https://pve.invalid?token=synthetic-secret",
		"https://synthetic-user:synthetic-secret@pve.invalid:bad-port",
	} {
		for _, count := range []int{1, 2} {
			var captured bytes.Buffer
			prior := log.Logger
			log.Logger = zerolog.New(zerolog.SyncWriter(&captured)).Level(zerolog.DebugLevel)
			func() {
				defer func() { log.Logger = prior }()
				endpoints := []string{host}
				if count == 2 {
					endpoints = append(endpoints, "https://synthetic-user:synthetic-secret@other.invalid")
				}
				cc := NewClusterClient("fixture", ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-token"}, endpoints, nil)
				_, err := cc.GetNodes(context.Background())
				if err == nil {
					t.Error("rejected endpoints did not stop cluster routing")
				} else if strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), "synthetic-user") {
					t.Error("cluster error exposed rejected endpoint credentials")
				}
				for _, status := range cc.GetHealthStatusWithErrors() {
					if strings.Contains(status.LastError, "synthetic-secret") {
						t.Error("cluster health error exposed rejected input")
					}
				}
				cc.mu.Lock()
				for ep := range cc.lastHealthCheck {
					cc.lastHealthCheck[ep] = time.Now().Add(-time.Hour)
				}
				cc.mu.Unlock()
				cc.recoverUnhealthyNodes(context.Background())
				for ep := range cc.GetHealthStatus() {
					if !strings.Contains(ep, "synthetic-secret") {
						t.Error("diagnostic filtering rewrote configured routing identity")
					}
				}
			}()
			if strings.Contains(captured.String(), "synthetic-secret") || strings.Contains(captured.String(), "synthetic-user") {
				t.Error("cluster log exposed rejected endpoint credentials")
			}
			t.Logf("configured_endpoints=%d captured_log_bytes=%d", count, captured.Len())
		}
	}
}

func TestClusterDiagnosticHealthSnapshotKeepsEveryEndpoint(t *testing.T) {
	cc := &ClusterClient{nodeHealth: map[string]bool{
		"https://pve.invalid/tenant":                      true,
		"https://fixture:synthetic-secret@first.invalid":  false,
		"https://fixture:synthetic-secret@second.invalid": true,
	}}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	statuses := cc.endpointHealthDiagnosticsLocked()
	if len(statuses) != 3 || len(cc.nodeHealth) != 3 {
		t.Fatal("diagnostic labels collapsed membership")
	}
	invalid, healthy := 0, 0
	for _, status := range statuses {
		if status.Endpoint == "[invalid endpoint]" {
			invalid++
		}
		if status.Healthy {
			healthy++
		}
		if strings.Contains(status.Endpoint, "synthetic-secret") {
			t.Error("health diagnostic copied a raw map key")
		}
	}
	if invalid != 2 || healthy != 2 {
		t.Error("display snapshot lost endpoint status observations")
	}
}
