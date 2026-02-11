package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestAgentSendReport_SetsHeadersAndPostsJSON(t *testing.T) {
	t.Parallel()

	type received struct {
		method        string
		path          string
		authorization string
		apiToken      string
		contentType   string
		userAgent     string
		body          agentshost.Report
	}

	var (
		mu  sync.Mutex
		got received
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var report agentshost.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		mu.Lock()
		got = received{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			apiToken:      r.Header.Get("X-API-Token"),
			contentType:   r.Header.Get("Content-Type"),
			userAgent:     r.Header.Get("User-Agent"),
			body:          report,
		}
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := &Agent{
		cfg:             Config{APIToken: "test-token"},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	wantReport := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
		Host:  agentshost.HostInfo{Hostname: "test-host"},
	}

	if err := agent.sendReport(context.Background(), wantReport); err != nil {
		t.Fatalf("sendReport: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got.method != http.MethodPost {
		t.Fatalf("method = %q, want %q", got.method, http.MethodPost)
	}
	if got.path != "/api/agents/host/report" {
		t.Fatalf("path = %q, want %q", got.path, "/api/agents/host/report")
	}
	if got.authorization != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want %q", got.authorization, "Bearer test-token")
	}
	if got.apiToken != "test-token" {
		t.Fatalf("X-API-Token = %q, want %q", got.apiToken, "test-token")
	}
	if got.contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got.contentType, "application/json")
	}
	if got.userAgent == "" {
		t.Fatalf("User-Agent should be set")
	}
	if got.body.Agent.ID != wantReport.Agent.ID {
		t.Fatalf("decoded report Agent.ID = %q, want %q", got.body.Agent.ID, wantReport.Agent.ID)
	}
	if got.body.Host.Hostname != wantReport.Host.Hostname {
		t.Fatalf("decoded report Host.Hostname = %q, want %q", got.body.Host.Hostname, wantReport.Host.Hostname)
	}
}

func TestAgentSendReport_Non2xxReturnsError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	agent := &Agent{
		cfg:             Config{APIToken: "test-token"},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	err := agent.sendReport(context.Background(), agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
	})
	if err == nil {
		t.Fatalf("expected error for non-2xx response")
	}

	var statusErr *reportHTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected reportHTTPStatusError, got %T", err)
	}
	if statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", statusErr.StatusCode, http.StatusInternalServerError)
	}
	if statusErr.Endpoint != hostReportEndpoint {
		t.Fatalf("endpoint = %q, want %q", statusErr.Endpoint, hostReportEndpoint)
	}
}

func TestAgentProcess_ForbiddenResponseDoesNotBuffer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	agent := &Agent{
		cfg:             Config{APIToken: "test-token"},
		logger:          zerolog.Nop(),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    buffer.New[agentshost.Report](8),
		collector:       &mockCollector{},
	}

	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !agent.reportBuffer.IsEmpty() {
		t.Fatalf("buffer should stay empty for forbidden response, len=%d", agent.reportBuffer.Len())
	}
}
