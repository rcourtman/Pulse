package eval

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner_ExecuteStep_NetworkError(t *testing.T) {
	// Server immediately closes connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic to force connection reset")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.StepRetries = 0 // Fail fast

	step := Step{Name: "NetFail", Prompt: "Hi"}
	result := runner.executeStep(step, "")

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}

func TestRunner_ExecuteStep_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.StepRetries = 0

	step := Step{Name: "HTTPFail", Prompt: "Hi"}
	result := runner.executeStep(step, "")

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "500")
}

func TestRunner_ExecuteStep_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.RequestTimeout = 100 * time.Millisecond
	runner := NewRunner(cfg)
	runner.config.BaseURL = server.URL

	step := Step{Name: "Timeout", Prompt: "Hi"}
	result := runner.executeStep(step, "")

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "context deadline exceeded")
}

func TestRunner_ExecuteStep_SSE_BadContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json") // Wrong content type
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL

	step := Step{Name: "BadContentType", Prompt: "Hi"}
	result := runner.executeStep(step, "")

	// It might succeed if it just parses the body, or fail if it strictly checks content type/stream
	// The current implementation likely tries to scan it as SSE.
	// If it's not SSE, scan might fail or return nothing.
	// Let's see what happens.

	// If parseSSEStream expects data: prefix lines, it acts as empty response?
	// And empty response triggers retry or failure?
	assert.False(t, result.Success)
}
