package eval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPatrolModelOverride(t *testing.T) {
	// Setup mock server
	currentModel := "gpt-3.5-turbo"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/settings/ai" && r.Method == "GET" {
			json.NewEncoder(w).Encode(aiSettingsResponse{PatrolModel: currentModel})
			return
		}
		if r.URL.Path == "/api/settings/ai/update" && r.Method == "PUT" {
			var req aiSettingsUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.PatrolModel != nil {
				currentModel = *req.PatrolModel
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Model = "gpt-4" // Target override

	// 1. Test override triggers update
	restore, err := runner.applyPatrolModelOverride(context.Background())
	require.NoError(t, err)
	require.NotNil(t, restore)

	// Check server state updated
	assert.Equal(t, "openai:gpt-4", currentModel)

	// 2. Test restore reverts update
	restore()
	assert.Equal(t, "gpt-3.5-turbo", currentModel)
}

func TestApplyPatrolModelOverride_NoChangeNeeded(t *testing.T) {
	currentModel := "openai:gpt-4" // Use normalized format

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/settings/ai" && r.Method == "GET" {
			json.NewEncoder(w).Encode(aiSettingsResponse{PatrolModel: currentModel})
			return
		}
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Model = "gpt-4" // Normalized to openai:gpt-4

	restore, err := runner.applyPatrolModelOverride(context.Background())
	require.NoError(t, err)
	assert.Nil(t, restore) // No override applied
}

func TestApplyPatrolModelOverride_EnvVar(t *testing.T) {
	// Note: NormalizeModelString uses config.ParseModelString which might default to openai
	// if provider is missing. But for "claude-3-opus" it likely detects anthropic.
	os.Setenv("EVAL_PATROL_MODEL", "claude-3-opus")
	defer os.Unsetenv("EVAL_PATROL_MODEL")

	currentModel := "openai:gpt-4"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/settings/ai" && r.Method == "GET" {
			json.NewEncoder(w).Encode(aiSettingsResponse{PatrolModel: currentModel})
			return
		}
		if r.URL.Path == "/api/settings/ai/update" && r.Method == "PUT" {
			var req aiSettingsUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.PatrolModel != nil {
				currentModel = *req.PatrolModel
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	// config.Model doesn't matter, env var takes precedence

	restore, err := runner.applyPatrolModelOverride(context.Background())
	require.NoError(t, err)
	require.NotNil(t, restore)

	assert.Equal(t, "anthropic:claude-3-opus", currentModel)
	restore()
	assert.Equal(t, "openai:gpt-4", currentModel)
}
