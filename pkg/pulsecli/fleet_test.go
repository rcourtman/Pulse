package pulsecli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFleetConnectionsCommandFetchesCanonicalConnections(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/connections" {
			t.Fatalf("path = %s, want /api/connections", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"connections": [
				{
					"id": "agent:host-1",
					"type": "agent",
					"name": "host-1",
					"address": "host-1",
					"state": "active",
					"enabled": true,
					"surfaces": ["host"],
					"scope": {"host": true},
					"source": "agent",
					"fleet": {
						"enrollmentState": "enrolled",
						"livenessState": "active",
						"versionDrift": "behind",
						"adapterHealth": "healthy",
						"configRollout": "reported",
						"credentialStatus": "verified",
						"updateStatus": "update-available",
						"remoteControl": "enabled"
					},
					"capabilities": {
						"supportsPause": false,
						"supportsScope": false,
						"supportsTest": false
					}
				}
			],
			"systems": [
				{
					"id": "agent:host-1",
					"type": "agent",
					"components": [
						{"connectionId": "agent:host-1", "type": "agent", "role": "primary"}
					]
				}
			]
		}`))
	}))
	defer server.Close()

	cmd := newTestFleetRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{"fleet", "connections"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute fleet connections: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}

	var response fleetConnectionsResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if len(response.Connections) != 1 || len(response.Systems) != 1 {
		t.Fatalf("connections response = %+v", response)
	}
	var connection struct {
		ID    string `json:"id"`
		Fleet struct {
			RemoteControl string `json:"remoteControl"`
		} `json:"fleet"`
	}
	if err := json.Unmarshal(response.Connections[0], &connection); err != nil {
		t.Fatalf("decode connection: %v", err)
	}
	if connection.ID != "agent:host-1" || connection.Fleet.RemoteControl != "enabled" {
		t.Fatalf("connection = %+v", connection)
	}
}

func TestFleetConnectionsCommandRequiresToken(t *testing.T) {
	cmd := newTestFleetRootCommand(nil)
	cmd.SetArgs([]string{"fleet", "connections", "--api-url", "http://127.0.0.1:7655"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func newTestFleetRootCommand(env map[string]string) *cobra.Command {
	return NewRootCommand(
		CommandSpec{
			Use:     "pulse",
			Short:   "Pulse",
			Long:    "Pulse",
			Version: "1.2.3",
		},
		RuntimeSpec{},
		CommandDeps{
			Fleet: &FleetDeps{
				Getenv: func(key string) string {
					if env == nil {
						return ""
					}
					return env[key]
				},
			},
		},
	)
}
