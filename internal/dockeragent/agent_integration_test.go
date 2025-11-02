package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestSendReportIntegration(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		requests    []agentsdocker.Report
		tokenValues []string
		userAgents  []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/api/agents/docker/report" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		_ = r.Body.Close()

		var report agentsdocker.Report
		if err := json.Unmarshal(body, &report); err != nil {
			t.Fatalf("failed to unmarshal report: %v", err)
		}

		mu.Lock()
		requests = append(requests, report)
		tokenValues = append(tokenValues, r.Header.Get("X-API-Token"))
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"commands":[]}`))
	}))
	defer server.Close()

	client := server.Client()

	agent := &Agent{
		cfg: Config{
			Targets: []TargetConfig{{
				URL:   server.URL,
				Token: "secret-token",
			}},
		},
		httpClients: map[bool]*http.Client{
			false: client,
		},
		logger: zerolog.New(io.Discard),
		targets: []TargetConfig{{
			URL:   server.URL,
			Token: "secret-token",
		}},
	}

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname: "stub-host",
		},
		Containers: []agentsdocker.Container{
			{ID: "container-1"},
		},
	}

	if err := agent.sendReport(context.Background(), report); err != nil {
		t.Fatalf("sendReport returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got := len(requests); got != 1 {
		t.Fatalf("expected 1 request, got %d", got)
	}

	if tokenValues[0] != "secret-token" {
		t.Fatalf("expected token %q, got %q", "secret-token", tokenValues[0])
	}

	if requests[0].Host.Hostname != "stub-host" {
		t.Fatalf("expected hostname stub-host, got %s", requests[0].Host.Hostname)
	}

	if len(requests[0].Containers) != 1 {
		t.Fatalf("expected 1 container reported, got %d", len(requests[0].Containers))
	}

	if userAgents[0] == "" {
		t.Fatalf("missing user-agent header")
	}
	if !strings.HasPrefix(userAgents[0], "pulse-docker-agent/") {
		t.Fatalf("unexpected user-agent header: %s", userAgents[0])
	}
}

func TestSendReportHostRemoved(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"docker host \"5316f5e1\" was removed at 2025-11-02T13:45:15Z and cannot report again","code":"invalid_report"}`))
	}))
	defer server.Close()

	agent := &Agent{
		cfg: Config{
			Targets: []TargetConfig{{
				URL:   server.URL,
				Token: "secret-token",
			}},
		},
		httpClients: map[bool]*http.Client{
			false: server.Client(),
		},
		logger: zerolog.New(io.Discard),
		targets: []TargetConfig{{
			URL:   server.URL,
			Token: "secret-token",
		}},
		hostID: "5316f5e1",
	}

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname: "homeassistant",
		},
	}

	err := agent.sendReport(context.Background(), report)
	if !errors.Is(err, ErrStopRequested) {
		t.Fatalf("expected ErrStopRequested, got %v", err)
	}
}
