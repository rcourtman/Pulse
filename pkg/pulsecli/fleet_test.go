package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func runFleetInventoryFixture(t *testing.T, body string, chunked bool) (string, string, error) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/connections" || r.Header.Get("Authorization") != "Bearer synthetic-fleet-token" {
			t.Errorf("unexpected fleet request: method=%s path=%s bearer=%t", r.Method, r.URL.Path, r.Header.Get("Authorization") == "Bearer synthetic-fleet-token")
		}
		w.Header().Set("Content-Type", "application/json")
		if chunked {
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	cmd := newTestFleetRootCommand(map[string]string{"PULSE_API_TOKEN": "synthetic-fleet-token", "PULSE_API_URL": server.URL})
	cmd.SetArgs([]string{"fleet", "connections"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := cmd.ExecuteContext(ctx)
	if got := requests.Load(); got != 1 {
		t.Fatalf("fleet read made %d requests, want one without retry", got)
	}
	return stdout.String(), stderr.String(), err
}

func TestFleetConnectionsCommandRequiresExplicitInventory(t *testing.T) {
	for _, framing := range []string{"fixed", "chunked"} {
		t.Run(framing, func(t *testing.T) {
			for _, tc := range []struct{ name, body string }{
				{"null-envelope", `null`},
				{"empty-object", `{}`},
				{"systems-only", `{"systems":[{"id":"agent:saved-host"}]}`},
				{"error-envelope", `{"error":"synthetic-private-response"}`},
				{"null-inventory", `{"connections":null}`},
				{"object-inventory", `{"connections":{}}`},
				{"string-inventory", `{"connections":"not an inventory"}`},
				{"boolean-inventory", `{"connections":false}`},
				{"number-inventory", `{"connections":0}`},
				{"array-envelope", `[]`},
				{"empty-body", ``},
				{"truncated", `{"connections":[`},
				{"trailing-json", `{"connections":[]} null`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					stdout, stderr, err := runFleetInventoryFixture(t, tc.body, framing == "chunked")
					if err == nil || !strings.Contains(err.Error(), "fleet connections response") {
						t.Errorf("incomplete inventory must return a response error, got %v", err)
					}
					if stdout != "" {
						t.Errorf("incomplete inventory emitted successful fleet JSON: %s", stdout)
					}
					if strings.Contains(stderr, "synthetic-private-response") || (err != nil && strings.Contains(err.Error(), "synthetic-private-response")) {
						t.Error("invalid envelope diagnostic echoed unrelated response content")
					}
				})
			}
		})
	}
}

func TestFleetConnectionsCommandPreservesExplicitInventory(t *testing.T) {
	for _, framing := range []string{"fixed", "chunked"} {
		t.Run(framing, func(t *testing.T) {
			for _, tc := range []struct{ name, body string }{
				{"empty", `{"connections":[]}`},
				{"empty-with-systems", `{"connections":[],"systems":[]}`},
				{"empty-with-null-systems", `{"connections":[],"systems":null}`},
				{"populated", `{"connections":[{"id":"agent:host-1","future":{"counter":9007199254740993},"state":"unknown"},{"id":"pbs:backup","state":"auth-failed"}],"systems":[{"id":"agent:host-1","components":[{"connectionId":"agent:host-1"}]}]}`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					stdout, stderr, err := runFleetInventoryFixture(t, tc.body, framing == "chunked")
					if err != nil || stderr != "" {
						t.Fatalf("explicit inventory rejected: %v, stderr=%q", err, stderr)
					}
					var want fleetConnectionsResponse
					if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
						t.Fatal(err)
					}
					encoded, err := json.MarshalIndent(want, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if stdout != string(encoded)+"\n" {
						t.Fatalf("valid inventory output changed: got %s, want %s", stdout, encoded)
					}
				})
			}
		})
	}
}
