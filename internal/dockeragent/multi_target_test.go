package dockeragent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog"
)

func TestDeliverReportBuffersOnlyFailedDockerDestination(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer primary.Close()
	observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer observer.Close()

	targets := []TargetConfig{
		{Name: "primary", URL: primary.URL, Token: "p", Authoritative: true},
		{Name: "dev", URL: observer.URL, Token: "o"},
	}
	a := &Agent{
		logger: zerolog.Nop(), targets: targets,
		httpClients:        map[bool]*http.Client{false: http.DefaultClient},
		trustedHTTPClients: map[string]*http.Client{},
		reportBuffer:       utils.New[agentsdocker.Report](10),
		reportBuffers: map[string]*utils.Queue[agentsdocker.Report]{
			"primary": utils.New[agentsdocker.Report](10),
			"dev":     utils.New[agentsdocker.Report](10),
		},
	}
	a.reportBuffer = a.reportBuffers["primary"]
	if err := a.deliverReport(context.Background(), agentsdocker.Report{}); err != nil {
		t.Fatalf("deliver report: %v", err)
	}
	if a.reportBuffers["primary"].Len() != 0 || a.reportBuffers["dev"].Len() != 1 {
		t.Fatalf("buffer depths primary=%d observer=%d", a.reportBuffers["primary"].Len(), a.reportBuffers["dev"].Len())
	}
}

func TestDockerObserverCannotIssueCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commands":[{"id":"cmd1","type":"stop"}]}`))
	}))
	defer server.Close()
	a := &Agent{logger: zerolog.Nop(), httpClients: map[bool]*http.Client{false: server.Client()}, trustedHTTPClients: map[string]*http.Client{}}
	if err := a.sendReportToTarget(context.Background(), TargetConfig{Name: "dev", URL: server.URL, Token: "o"}, []byte(`{}`), 0); err != nil {
		t.Fatalf("observer response must be acknowledgement-only: %v", err)
	}
}

func TestDockerObserverPlaintextPolicyIsDestinationScoped(t *testing.T) {
	targets := []TargetConfig{
		{Name: "primary", URL: "https://primary.example.test", Token: "p", Authoritative: true},
		{Name: "observer", URL: "http://203.0.113.10:7655", Token: "o"},
	}
	if _, err := normalizeTargets(targets); err == nil {
		t.Fatal("expected observer plaintext URL rejection without destination opt-in")
	}
	targets[1].AllowPlaintextHTTP = true
	normalized, err := normalizeTargets(targets)
	if err != nil {
		t.Fatalf("explicit observer plaintext opt-in: %v", err)
	}
	if !normalized[1].AllowPlaintextHTTP {
		t.Fatal("observer plaintext policy was not preserved after normalization")
	}
}

func TestManualUpdateCheckRunsOnceAcrossReplayAndConcurrentRequest(t *testing.T) {
	var (
		acksMu sync.Mutex
		acks   = make(map[string][]agentsdocker.CommandAck)
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ack agentsdocker.CommandAck
		if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		commandID := commandIDFromAckPath(r.URL.Path)
		acksMu.Lock()
		acks[commandID] = append(acks[commandID], ack)
		acksMu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	var collections atomic.Int32
	registryChecker := NewRegistryChecker(zerolog.Nop())
	registryChecker.cacheDigest("before-first-command", "sha256:cached")
	agent := &Agent{
		logger:          zerolog.Nop(),
		hostID:          "host1",
		registryChecker: registryChecker,
		httpClients:     map[bool]*http.Client{false: server.Client()},
		manualCheckCollect: func(ctx context.Context) (agentsdocker.Report, error) {
			if collections.Add(1) == 1 {
				close(started)
			}
			select {
			case <-ctx.Done():
				return agentsdocker.Report{}, ctx.Err()
			case <-release:
				return agentsdocker.Report{
					Containers: []agentsdocker.Container{{
						UpdateStatus: &agentsdocker.UpdateStatus{UpdateAvailable: true},
					}},
				}, nil
			}
		},
	}
	t.Cleanup(func() { _ = agent.Close() })
	target := TargetConfig{URL: server.URL, Token: "token"}
	first := agentsdocker.Command{ID: "check-1", Type: agentsdocker.CommandTypeCheckUpdates}

	if err := agent.handleCheckUpdatesCommand(context.Background(), target, first); err != nil {
		t.Fatalf("start manual update check: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("manual update check did not start")
	}

	registryChecker.cacheDigest("after-first-command", "sha256:sentinel")
	if err := agent.handleCheckUpdatesCommand(context.Background(), target, first); err != nil {
		t.Fatalf("replay manual update check: %v", err)
	}
	if err := agent.handleCheckUpdatesCommand(context.Background(), target, agentsdocker.Command{
		ID:   "check-2",
		Type: agentsdocker.CommandTypeCheckUpdates,
	}); err != nil {
		t.Fatalf("send concurrent manual update check: %v", err)
	}

	if got := collections.Load(); got != 1 {
		t.Fatalf("manual update check collections = %d, want 1", got)
	}
	if cached := registryChecker.getCached("after-first-command"); cached == nil {
		t.Fatal("replayed or concurrent command cleared the registry cache again")
	}
	waitForDockerCommandAck(t, &acksMu, acks, "check-2", func(ack agentsdocker.CommandAck) bool {
		return ack.Status == agentsdocker.CommandStatusFailed &&
			strings.Contains(ack.Message, "already running")
	})

	close(release)
	completed := waitForDockerCommandAck(t, &acksMu, acks, "check-1", func(ack agentsdocker.CommandAck) bool {
		return ack.Status == agentsdocker.CommandStatusCompleted
	})
	if completed.Message != "Container update check completed: 1 checked, 1 updates available" {
		t.Fatalf("unexpected completion message %q", completed.Message)
	}

	if err := agent.handleCheckUpdatesCommand(context.Background(), target, first); err != nil {
		t.Fatalf("replay completed manual update check: %v", err)
	}
	if got := collections.Load(); got != 1 {
		t.Fatalf("completed command replay collections = %d, want 1", got)
	}
}

func TestManualUpdateCheckTimeoutRetriesTerminalAckWithoutReexecution(t *testing.T) {
	swap(t, &manualUpdateCheckTimeout, 30*time.Millisecond)
	swap(t, &manualUpdateCheckAckRetryDelay, time.Duration(0))

	var (
		terminalAttempts atomic.Int32
		lastTerminalMu   sync.Mutex
		lastTerminal     agentsdocker.CommandAck
		collections      atomic.Int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ack agentsdocker.CommandAck
		if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ack.Status != agentsdocker.CommandStatusFailed {
			w.WriteHeader(http.StatusOK)
			return
		}
		lastTerminalMu.Lock()
		lastTerminal = ack
		lastTerminalMu.Unlock()
		if terminalAttempts.Add(1) < manualUpdateCheckAckAttempts {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := &Agent{
		logger:      zerolog.Nop(),
		hostID:      "host1",
		httpClients: map[bool]*http.Client{false: server.Client()},
		manualCheckCollect: func(ctx context.Context) (agentsdocker.Report, error) {
			collections.Add(1)
			<-ctx.Done()
			return agentsdocker.Report{}, ctx.Err()
		},
	}
	t.Cleanup(func() { _ = agent.Close() })
	target := TargetConfig{URL: server.URL, Token: "token"}
	command := agentsdocker.Command{ID: "timeout-check", Type: agentsdocker.CommandTypeCheckUpdates}

	if err := agent.handleCheckUpdatesCommand(context.Background(), target, command); err != nil {
		t.Fatalf("start timeout check: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for terminalAttempts.Load() < manualUpdateCheckAckAttempts && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := terminalAttempts.Load(); got != manualUpdateCheckAckAttempts {
		t.Fatalf("terminal acknowledgement attempts = %d, want %d", got, manualUpdateCheckAckAttempts)
	}
	lastTerminalMu.Lock()
	terminal := lastTerminal
	lastTerminalMu.Unlock()
	if !strings.Contains(terminal.Message, "timed out after 30ms") {
		t.Fatalf("timeout acknowledgement message = %q", terminal.Message)
	}
	if got := collections.Load(); got != 1 {
		t.Fatalf("timeout command collections = %d, want 1", got)
	}

	if err := agent.handleCheckUpdatesCommand(context.Background(), target, command); err != nil {
		t.Fatalf("replay timed-out check: %v", err)
	}
	if got := collections.Load(); got != 1 {
		t.Fatalf("timed-out command replay collections = %d, want 1", got)
	}
}

func TestManualUpdateCheckReportsRegistryErrorsAndRateLimits(t *testing.T) {
	var (
		acksMu sync.Mutex
		acks   = make(map[string][]agentsdocker.CommandAck)
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ack agentsdocker.CommandAck
		if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		acksMu.Lock()
		acks["summary-check"] = append(acks["summary-check"], ack)
		acksMu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	agent := &Agent{
		logger:      zerolog.Nop(),
		hostID:      "host1",
		httpClients: map[bool]*http.Client{false: server.Client()},
		manualCheckCollect: func(context.Context) (agentsdocker.Report, error) {
			return agentsdocker.Report{Containers: []agentsdocker.Container{
				{UpdateStatus: &agentsdocker.UpdateStatus{UpdateAvailable: true}},
				{UpdateStatus: &agentsdocker.UpdateStatus{Error: "rate limited"}},
				{UpdateStatus: &agentsdocker.UpdateStatus{Error: "authentication required"}},
				{UpdateStatus: &agentsdocker.UpdateStatus{Error: "digest-pinned image"}},
				{},
			}}, nil
		},
	}
	t.Cleanup(func() { _ = agent.Close() })
	if err := agent.handleCheckUpdatesCommand(
		context.Background(),
		TargetConfig{URL: server.URL, Token: "token"},
		agentsdocker.Command{ID: "summary-check", Type: agentsdocker.CommandTypeCheckUpdates},
	); err != nil {
		t.Fatalf("start summary check: %v", err)
	}

	completed := waitForDockerCommandAck(t, &acksMu, acks, "summary-check", func(ack agentsdocker.CommandAck) bool {
		return ack.Status == agentsdocker.CommandStatusCompleted
	})
	want := "Container update check completed: 3 checked, 1 updates available, 2 skipped, 2 registry errors (1 rate limited)"
	if completed.Message != want {
		t.Fatalf("completion message = %q, want %q", completed.Message, want)
	}
}

func TestRegistryRequestHonorsManualCheckDeadline(t *testing.T) {
	checker := &RegistryChecker{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	_, _, err := checker.fetchDigest(ctx, "example.test", "repo", "tag", "", "", "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hanging registry request error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("hanging registry request took %s after its deadline", elapsed)
	}
}

func commandIDFromAckPath(path string) string {
	path = strings.TrimSuffix(path, "/ack")
	return path[strings.LastIndex(path, "/")+1:]
}

func waitForDockerCommandAck(
	t *testing.T,
	mu *sync.Mutex,
	acks map[string][]agentsdocker.CommandAck,
	commandID string,
	matches func(agentsdocker.CommandAck) bool,
) agentsdocker.CommandAck {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		commandAcks := append([]agentsdocker.CommandAck(nil), acks[commandID]...)
		mu.Unlock()
		for _, ack := range commandAcks {
			if matches(ack) {
				return ack
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for matching acknowledgement for %q", commandID)
	return agentsdocker.CommandAck{}
}
