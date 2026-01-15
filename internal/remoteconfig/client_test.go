package remoteconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Fetch(t *testing.T) {
	t.Run("successful fetch with full config", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/agents/host/agent-1/config" {
				t.Errorf("Expected path /api/agents/host/agent-1/config, got %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Header.Get("Authorization") != "Bearer token-123" {
				t.Errorf("Expected Authorization header 'Bearer token-123', got %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"success": true,
				"hostId": "agent-1",
				"config": {
					"commandsEnabled": true,
					"settings": {
						"interval": "1m",
						"enable_docker": false
					}
				}
			}`))
		}))
		defer ts.Close()

		client := New(Config{
			PulseURL: ts.URL,
			APIToken: "token-123",
			AgentID:  "agent-1",
		})

		settings, commandsEnabled, err := client.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		if commandsEnabled == nil || *commandsEnabled != true {
			t.Errorf("Expected commandsEnabled=true, got %v", commandsEnabled)
		}

		if settings["interval"] != "1m" {
			t.Errorf("Expected interval='1m', got %v", settings["interval"])
		}
		if settings["enable_docker"] != false {
			t.Errorf("Expected enable_docker=false, got %v", settings["enable_docker"])
		}
	})

	t.Run("signature required without signature", func(t *testing.T) {
		t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "true")
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"success": true,
				"hostId": "agent-1",
				"config": {
					"commandsEnabled": true,
					"settings": {
						"interval": "1m"
					}
				}
			}`))
		}))
		defer ts.Close()

		client := New(Config{
			PulseURL: ts.URL,
			APIToken: "token-123",
			AgentID:  "agent-1",
		})

		_, _, err := client.Fetch(context.Background())
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !strings.Contains(err.Error(), "signature required") {
			t.Fatalf("Expected signature required error, got %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := New(Config{PulseURL: ts.URL, APIToken: "t", AgentID: "a"})
		_, _, err := client.Fetch(context.Background())
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})

	t.Run("missing agent ID", func(t *testing.T) {
		client := New(Config{PulseURL: "http://localhost", APIToken: "t", AgentID: ""})
		_, _, err := client.Fetch(context.Background())
		if err == nil {
			t.Fatal("Expected error for missing agent ID, got nil")
		}
	})
}
