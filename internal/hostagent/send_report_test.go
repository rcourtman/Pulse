package hostagent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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
		var bodyReader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, gzErr := gzip.NewReader(r.Body)
			if gzErr != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			bodyReader = gz
		}

		var report agentshost.Report
		if err := json.NewDecoder(bodyReader).Decode(&report); err != nil {
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
	if got.path != "/api/agents/agent/report" {
		t.Fatalf("path = %q, want %q", got.path, "/api/agents/agent/report")
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

func TestAgentSendReport_OmitsAuthHeadersWhenTokenMissing(t *testing.T) {
	t.Parallel()

	type received struct {
		authorization string
		apiToken      string
	}

	var (
		mu  sync.Mutex
		got received
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyReader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, gzErr := gzip.NewReader(r.Body)
			if gzErr != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			bodyReader = gz
		}
		if err := json.NewDecoder(bodyReader).Decode(&agentshost.Report{}); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		mu.Lock()
		got = received{
			authorization: r.Header.Get("Authorization"),
			apiToken:      r.Header.Get("X-API-Token"),
		}
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := &Agent{
		cfg:             Config{APIToken: "   "},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	if err := agent.sendReport(context.Background(), agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
	}); err != nil {
		t.Fatalf("sendReport: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got.authorization != "" {
		t.Fatalf("Authorization = %q, want empty", got.authorization)
	}
	if got.apiToken != "" {
		t.Fatalf("X-API-Token = %q, want empty", got.apiToken)
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
	if statusErr.Endpoint != agentReportEndpoint {
		t.Fatalf("endpoint = %q, want %q", statusErr.Endpoint, agentReportEndpoint)
	}
}

func TestAgentProcess_ForbiddenResponseDoesNotBuffer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	agent := &Agent{
		cfg:             Config{APIToken: "test-token"},
		logger:          zerolog.New(&logBuf),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](8),
		collector:       &mockCollector{},
	}

	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !agent.reportBuffer.IsEmpty() {
		t.Fatalf("buffer should stay empty for forbidden response, len=%d", agent.reportBuffer.Len())
	}
	if !strings.Contains(logBuf.String(), "Unified Agent reporting") {
		t.Fatalf("expected forbidden log to mention Unified Agent reporting scope, got %q", logBuf.String())
	}
}

func TestAgentProcess_UnauthorizedResponseDoesNotBuffer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	agent := &Agent{
		cfg:             Config{APIToken: "stale-token"},
		logger:          zerolog.New(&logBuf),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](8),
		collector:       &mockCollector{},
	}

	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	// A rejected token loops forever; the report must be dropped, not buffered.
	if !agent.reportBuffer.IsEmpty() {
		t.Fatalf("buffer should stay empty for unauthorized response, len=%d", agent.reportBuffer.Len())
	}
	if !strings.Contains(logBuf.String(), "rejected this agent's API token") {
		t.Fatalf("expected 401 log to be actionable, got %q", logBuf.String())
	}
}

// An HTTP status is not a credential-loss diagnosis. These controls exercise
// the live and buffered rejection paths, then ordinary recovery with the same
// credential; neither a new token nor disabling host monitoring is required.
func TestHostReportingAuthGuidanceAndSameCredentialRecovery(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		for _, buffered := range []bool{false, true} {
			name := http.StatusText(status) + "/current"
			if buffered {
				name = http.StatusText(status) + "/buffered"
			}
			t.Run(name, func(t *testing.T) {
				const token = "synthetic-report-credential-not-for-logs"
				const responseSecret = "synthetic-upstream-body-not-for-logs"
				var responseStatus, accepted atomic.Int32
				responseStatus.Store(int32(status))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != agentReportEndpoint || r.Header.Get("X-API-Token") != token || r.Header.Get("Authorization") != "Bearer "+token {
						t.Error("ordinary report changed its endpoint or credential")
					}
					code := int(responseStatus.Load())
					w.WriteHeader(code)
					if code == http.StatusOK {
						accepted.Add(1)
						_, _ = io.WriteString(w, `{"success":true}`)
					} else {
						_, _ = io.WriteString(w, responseSecret)
					}
				}))
				defer server.Close()

				var logs bytes.Buffer
				primaryAcknowledgements := 0
				a := &Agent{
					cfg: Config{APIToken: token, EnableHost: true, CommandAuthorityProfile: CommandAuthorityMonitoringOnly,
						OnPrimaryReportAccepted: func() { primaryAcknowledgements++ }},
					logger: zerolog.New(&logs), httpClient: server.Client(), trimmedPulseURL: server.URL,
					reportBuffer: utils.New[agentshost.Report](8),
				}
				report := agentshost.Report{Agent: agentshost.AgentInfo{ID: "auth-diagnostic-agent"}}
				if buffered {
					a.reportBuffer.Push(report)
				}
				if err := a.deliverPrimaryReport(context.Background(), report); err != nil {
					t.Fatal("rejection unexpectedly stopped ordinary collection")
				}
				assertSafeReportAuthLog(t, logs.String(), server.URL, status, false, token, responseSecret)
				if !a.reportBuffer.IsEmpty() || primaryAcknowledgements != 0 || accepted.Load() != 0 {
					t.Fatal("rejected report was buffered or treated as accepted")
				}

				responseStatus.Store(http.StatusOK)
				if err := a.deliverPrimaryReport(context.Background(), report); err != nil {
					t.Fatal("ordinary reporting did not recover")
				}
				if primaryAcknowledgements != 1 || accepted.Load() != 1 || !a.lastAuthFailureLog.IsZero() || !a.reportBuffer.IsEmpty() {
					t.Fatal("successful current report did not restore acceptance and reset the diagnostic throttle")
				}
				if a.cfg.APIToken != token || !a.cfg.EnableHost || a.cfg.EnableCommands || a.cfg.CommandAuthorityProfile != CommandAuthorityMonitoringOnly {
					t.Fatal("authentication guidance changed credentials, monitoring or command authority")
				}
			})
		}
	}
}

func assertSafeReportAuthLog(t *testing.T, logs, pulseURL string, status int, observer bool, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(logs, secret) {
			t.Fatal("authentication diagnostic disclosed a credential or response body")
		}
	}
	for _, claim := range []string{"Re-run the agent install", "mint a fresh token", "mint a correctly scoped token", "restored/reinstalled", "upgraded (for example", "Set PULSE_ENABLE_HOST=false", "until the token is replaced"} {
		if strings.Contains(logs, claim) {
			t.Fatal("authentication diagnostic prescribed credential churn or guessed a cause")
		}
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal("diagnostic is not a structured log entry")
		}
		if entry["status_code"] != float64(status) {
			continue
		}
		found = true
		if entry["endpoint"] != agentReportEndpoint {
			t.Fatal("diagnostic lost the failed reporting endpoint")
		}
		message, _ := entry["message"].(string)
		if observer || status == http.StatusUnauthorized {
			if entry["pulse_url"] != pulseURL || !strings.Contains(message, "API Access record") || !strings.Contains(message, "status alone does not identify the cause") || !strings.Contains(message, "do not share token values or service files") && !strings.Contains(message, "or share token values or service files") {
				t.Fatal("diagnostic omitted its safe instance/record distinction or disclosure warning")
			}
		} else if !strings.Contains(message, "agent:report") || !strings.Contains(message, "Command execution permission is not needed") || !strings.Contains(message, "Do not disable host monitoring or reinstall") {
			t.Fatal("forbidden diagnostic did not preserve host monitoring and least privilege")
		}
		if !strings.Contains(message, "dropped while") {
			t.Fatal("diagnostic did not explain the current delivery consequence")
		}
	}
	if !found {
		t.Fatal("diagnostic lost the observed HTTP status")
	}
}

func TestAgentProcess_UnauthorizedLogThrottled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	agent := &Agent{
		cfg:             Config{APIToken: "stale-token"},
		logger:          zerolog.New(&logBuf),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](8),
		collector:       &mockCollector{},
	}

	for i := 0; i < 3; i++ {
		if err := agent.process(context.Background()); err != nil {
			t.Fatalf("process %d: %v", i, err)
		}
	}
	if got := strings.Count(logBuf.String(), "rejected this agent's API token"); got != 1 {
		t.Fatalf("expected actionable 401 log exactly once (throttled), got %d in %q", got, logBuf.String())
	}

	// Resetting the throttle (which a successful report does) re-emits the error.
	agent.lastAuthFailureLog = time.Time{}
	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process after reset: %v", err)
	}
	if got := strings.Count(logBuf.String(), "rejected this agent's API token"); got != 2 {
		t.Fatalf("expected actionable 401 log to re-emit after throttle reset, got %d", got)
	}
}

func TestAgentFlushBuffer_UnauthorizedDropsBuffer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	buffer := utils.New[agentshost.Report](8)
	buffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "a1"}})
	buffer.Push(agentshost.Report{Agent: agentshost.AgentInfo{ID: "a2"}})

	agent := &Agent{
		cfg:             Config{APIToken: "stale-token"},
		logger:          zerolog.New(&logBuf),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    buffer,
		collector:       &mockCollector{},
	}

	agent.flushBuffer(context.Background())

	// Buffered reports will never be accepted with a rejected token; drop them
	// so the buffer cannot grow unbounded across restarts.
	if !agent.reportBuffer.IsEmpty() {
		t.Fatalf("buffer should be drained after unauthorized flush, len=%d", agent.reportBuffer.Len())
	}
	if !strings.Contains(logBuf.String(), "rejected this agent's API token") {
		t.Fatalf("expected flush 401 log to be actionable, got %q", logBuf.String())
	}
}

func TestAgentProcess_SuccessLogsUnifiedAgentReport(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	agent := &Agent{
		cfg:             Config{APIToken: "test-token"},
		logger:          zerolog.New(&logBuf).Level(zerolog.DebugLevel),
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
		reportBuffer:    utils.New[agentshost.Report](8),
		collector:       &mockCollector{},
	}

	if err := agent.process(context.Background()); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !strings.Contains(logBuf.String(), "Unified Agent report sent") {
		t.Fatalf("expected success log to mention Unified Agent report, got %q", logBuf.String())
	}
}

func newServerVersionAckServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestAgentSendReport_SurfacesAckServerVersion(t *testing.T) {
	t.Parallel()

	server := newServerVersionAckServer(`{"success":true,"agentId":"agent-1","serverVersion":"6.2.1"}`)
	defer server.Close()

	var seen []string
	agent := &Agent{
		cfg: Config{
			APIToken:        "test-token",
			OnServerVersion: func(version string) { seen = append(seen, version) },
		},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	if err := agent.sendReport(context.Background(), agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
		Host:  agentshost.HostInfo{Hostname: "test-host"},
	}); err != nil {
		t.Fatalf("sendReport: %v", err)
	}

	if len(seen) != 1 || seen[0] != "6.2.1" {
		t.Fatalf("OnServerVersion calls = %v, want exactly [6.2.1]", seen)
	}
}

func TestAgentSendReport_SkipsCallbackWithoutAckServerVersion(t *testing.T) {
	t.Parallel()

	server := newServerVersionAckServer(`{"success":true,"agentId":"agent-1"}`)
	defer server.Close()

	called := false
	agent := &Agent{
		cfg: Config{
			APIToken:        "test-token",
			OnServerVersion: func(string) { called = true },
		},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	if err := agent.sendReport(context.Background(), agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
		Host:  agentshost.HostInfo{Hostname: "test-host"},
	}); err != nil {
		t.Fatalf("sendReport: %v", err)
	}

	if called {
		t.Fatal("OnServerVersion called for an ack without serverVersion")
	}
}

func TestAgentSendReport_ObserverAckNeverInvokesCallback(t *testing.T) {
	t.Parallel()

	// Observer destinations may run any Pulse version; only the authoritative
	// server may steer this agent's updater.
	server := newServerVersionAckServer(`{"success":true,"agentId":"agent-1","serverVersion":"9.9.9"}`)
	defer server.Close()

	called := false
	agent := &Agent{
		cfg: Config{
			APIToken:        "test-token",
			OnServerVersion: func(string) { called = true },
		},
		httpClient:      server.Client(),
		trimmedPulseURL: server.URL,
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-1"},
		Host:  agentshost.HostInfo{Hostname: "test-host"},
	}
	if err := agent.sendReportToDestination(context.Background(), report, server.URL, "observer-token", server.Client(), false); err != nil {
		t.Fatalf("sendReportToDestination: %v", err)
	}

	if called {
		t.Fatal("OnServerVersion called from a non-authoritative destination ack")
	}
}
