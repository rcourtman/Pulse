package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// realFSCollector returns a mockCollector wired to real filesystem operations.
func realFSCollector() *mockCollector {
	return &mockCollector{
		readFileFn:  func(name string) ([]byte, error) { return os.ReadFile(name) },
		writeFileFn: func(name string, data []byte, perm os.FileMode) error { return os.WriteFile(name, data, perm) },
		mkdirAllFn:  func(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) },
		chmodFn:     func(name string, mode os.FileMode) error { return os.Chmod(name, mode) },
	}
}

func TestEnroll_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != enrollEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var req enrollPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Hostname != "test-host" {
			t.Errorf("expected hostname test-host, got %s", req.Hostname)
		}
		if !req.CommandsEnabled {
			t.Error("expected commandsEnabled to be true")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(enrollResponse{
			AgentID:        "host-test-host",
			RuntimeToken:   "runtime-tok-123",
			RuntimeTokenID: "tok-id-456",
			ReportInterval: "30s",
		})
	}))
	defer server.Close()

	stateDir := t.TempDir()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken:       "bootstrap-tok",
			EnableCommands: true,
			Enroll:         true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		osName:          "linux",
		architecture:    "amd64",
		agentVersion:    "1.0.0",
		trimmedPulseURL: server.URL,
		stateDir:        stateDir,
		collector:       realFSCollector(),
	}

	err := agent.runEnrollmentLoop(context.Background())
	if err != nil {
		t.Fatalf("enrollment failed: %v", err)
	}

	// Verify token was updated.
	if agent.cfg.APIToken != "runtime-tok-123" {
		t.Errorf("expected token runtime-tok-123, got %s", agent.cfg.APIToken)
	}

	// Verify runtime token was persisted.
	tokenPath := filepath.Join(stateDir, runtimeTokenFile)
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read runtime token file: %v", err)
	}
	if string(data) != "runtime-tok-123" {
		t.Errorf("expected persisted token runtime-tok-123, got %s", string(data))
	}

	// Verify agent-id was persisted.
	agentIDPath := filepath.Join(stateDir, "agent-id")
	data, err = os.ReadFile(agentIDPath)
	if err != nil {
		t.Fatalf("read agent-id file: %v", err)
	}
	if string(data) != "host-test-host" {
		t.Errorf("expected agent-id host-test-host, got %s", string(data))
	}
}

func TestEnroll_PrefersAgentIDFromResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(enrollResponse{
			AgentID:      "agent-canonical",
			RuntimeToken: "runtime-tok-123",
		})
	}))
	defer server.Close()

	stateDir := t.TempDir()
	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "bootstrap-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		osName:          "linux",
		architecture:    "amd64",
		agentVersion:    "1.0.0",
		trimmedPulseURL: server.URL,
		stateDir:        stateDir,
		collector:       realFSCollector(),
	}

	if err := agent.runEnrollmentLoop(context.Background()); err != nil {
		t.Fatalf("enrollment failed: %v", err)
	}

	agentIDPath := filepath.Join(stateDir, "agent-id")
	data, err := os.ReadFile(agentIDPath)
	if err != nil {
		t.Fatalf("read agent-id file: %v", err)
	}
	if got := string(data); got != "agent-canonical" {
		t.Fatalf("expected persisted agent-id agent-canonical, got %s", got)
	}
}

func TestEnroll_AlreadyEnrolled(t *testing.T) {
	stateDir := t.TempDir()

	// Write an existing runtime token.
	tokenPath := filepath.Join(stateDir, runtimeTokenFile)
	if err := os.WriteFile(tokenPath, []byte("existing-runtime-tok"), 0600); err != nil {
		t.Fatal(err)
	}

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "bootstrap-tok",
			Enroll:   true,
		},
		logger:   logger,
		stateDir: stateDir,
		collector: &mockCollector{
			readFileFn: func(name string) ([]byte, error) {
				return os.ReadFile(name)
			},
		},
	}

	err := agent.runEnrollmentLoop(context.Background())
	if err != nil {
		t.Fatalf("enrollment failed: %v", err)
	}

	// Should use the existing runtime token.
	if agent.cfg.APIToken != "existing-runtime-tok" {
		t.Errorf("expected token existing-runtime-tok, got %s", agent.cfg.APIToken)
	}
}

func TestEnroll_403_SkipsEnrollment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "manual-install-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	err := agent.runEnrollmentLoop(context.Background())
	if err != nil {
		t.Fatalf("enrollment should skip on 403, got error: %v", err)
	}

	// Token should remain unchanged.
	if agent.cfg.APIToken != "manual-install-tok" {
		t.Errorf("expected token unchanged, got %s", agent.cfg.APIToken)
	}
}

func TestEnroll_RetryThenSuccess(t *testing.T) {
	// Speed up retries for this test.
	origInitial := enrollInitialDelay
	origMax := enrollMaxDelay
	enrollInitialDelay = 10 * time.Millisecond
	enrollMaxDelay = 50 * time.Millisecond
	defer func() {
		enrollInitialDelay = origInitial
		enrollMaxDelay = origMax
	}()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(enrollResponse{
			AgentID:      "host-retry",
			RuntimeToken: "runtime-after-retry",
		})
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "bootstrap-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "retry-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := agent.runEnrollmentLoop(ctx)
	if err != nil {
		t.Fatalf("enrollment should succeed after retries, got: %v", err)
	}

	if agent.cfg.APIToken != "runtime-after-retry" {
		t.Errorf("expected token runtime-after-retry, got %s", agent.cfg.APIToken)
	}

	if got := attempts.Load(); got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}

func TestEnroll_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "bootstrap-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "cancel-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := agent.runEnrollmentLoop(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestEnrollStatusError(t *testing.T) {
	err := &enrollStatusError{StatusCode: 401, Body: "Unauthorized"}
	expected := "enrollment returned HTTP 401: Unauthorized"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestEnroll_401_PermanentFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "expired-bootstrap-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	err := agent.runEnrollmentLoop(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 permanent failure")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestEnroll_409_PermanentFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Conflict", http.StatusConflict)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "consumed-bootstrap-tok",
			Enroll:   true,
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	err := agent.runEnrollmentLoop(context.Background())
	if err == nil {
		t.Fatal("expected error for 409 permanent failure")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("expected 409 in error, got: %v", err)
	}
}

func TestEnroll_MissingRuntimeToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a response missing runtimeToken.
		fmt.Fprint(w, `{"agentId":"agent-1"}`)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewTestWriter(t))

	agent := &Agent{
		cfg: Config{
			APIToken: "bootstrap-tok",
		},
		logger:          logger,
		httpClient:      server.Client(),
		hostname:        "test-host",
		trimmedPulseURL: server.URL,
		stateDir:        t.TempDir(),
		collector:       &mockCollector{},
	}

	_, err := agent.enroll(context.Background())
	if err == nil {
		t.Fatal("expected error for missing runtimeToken")
	}
}
