package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubDiscoveryProvider struct {
	lastGetResourceType string
	lastGetTargetID     string
	lastGetResourceID   string
	getResp             *ResourceDiscoveryInfo
	getErr              error

	lastListTargetID string
	listResp         []*ResourceDiscoveryInfo
	listErr          error
}

func (s *stubDiscoveryProvider) GetDiscovery(_ string) (*ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (s *stubDiscoveryProvider) GetDiscoveryByResource(resourceType, targetID, resourceID string) (*ResourceDiscoveryInfo, error) {
	s.lastGetResourceType = resourceType
	s.lastGetTargetID = targetID
	s.lastGetResourceID = resourceID
	return s.getResp, s.getErr
}

func (s *stubDiscoveryProvider) ListDiscoveries() ([]*ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (s *stubDiscoveryProvider) ListDiscoveriesByType(_ string) ([]*ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (s *stubDiscoveryProvider) ListDiscoveriesByTarget(targetID string) ([]*ResourceDiscoveryInfo, error) {
	s.lastListTargetID = targetID
	return s.listResp, s.listErr
}

func (s *stubDiscoveryProvider) FormatForAIContext(_ []*ResourceDiscoveryInfo) string {
	return ""
}

func (s *stubDiscoveryProvider) TriggerDiscovery(_ context.Context, _, _, _ string) (*ResourceDiscoveryInfo, error) {
	return nil, nil
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// Transient errors — should return true
		{"nil error", nil, false},
		{"rate limit 429", errors.New("API returned status 429"), true},
		{"503 service unavailable", errors.New("HTTP 503 Service Unavailable"), true},
		{"rate_limit underscore", errors.New("rate_limit: too many requests"), true},
		{"rate limit space", errors.New("rate limit exceeded"), true},
		{"ratelimit single word", errors.New("ratelimit error from provider"), true},
		{"too many requests", errors.New("too many requests, slow down"), true},
		{"timeout", errors.New("request timeout after 30s"), true},
		{"context deadline", errors.New("context deadline exceeded"), true},
		{"failed after retries", errors.New("failed after 3 retries"), true},
		{"temporarily unavailable", errors.New("service temporarily unavailable"), true},
		{"server overloaded", errors.New("server overloaded, try later"), true},
		{"service unavailable text", errors.New("the service is service unavailable"), true},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"network unreachable", errors.New("network unreachable"), true},

		// Anthropic-style rate limit
		{"anthropic rate limit", errors.New("Error: 429 {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\"}}"), true},
		// OpenAI-style
		{"openai rate limit", errors.New("Rate limit reached for gpt-4"), true},
		// Gemini-style
		{"gemini quota", errors.New("429 Too Many Requests"), true},

		// Non-transient errors — should return false
		{"resource not found", errors.New("resource not found"), false},
		{"permission denied", errors.New("permission denied"), false},
		{"invalid argument", errors.New("invalid resource_type: foo"), false},
		{"generic error", errors.New("something went wrong"), false},
		{"empty error", errors.New(""), false},
		{"auth error", errors.New("authentication failed"), false},
		{"not found", errors.New("404 not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTransientError(tt.err)
			assert.Equal(t, tt.expected, result, "error: %v", tt.err)
		})
	}
}

func TestExecuteGetDiscovery_UsesTargetID(t *testing.T) {
	provider := &stubDiscoveryProvider{
		getResp: &ResourceDiscoveryInfo{
			ID:           "vm:node1:101",
			ResourceType: "vm",
			ResourceID:   "101",
			TargetID:     "node1",
			HostID:       "node1",
			Hostname:     "vm-101",
		},
	}
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: provider})

	result, err := exec.executeGetDiscovery(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "101",
		"target_id":     "node1",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "vm", provider.lastGetResourceType)
	assert.Equal(t, "node1", provider.lastGetTargetID)
	assert.Equal(t, "101", provider.lastGetResourceID)

	var payload map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
	assert.Equal(t, "node1", payload["target_id"])
}

func TestExecuteGetDiscovery_TargetIDRequired(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: &stubDiscoveryProvider{}})

	result, err := exec.executeGetDiscovery(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "101",
	})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "target_id is required")
}

func TestExecuteListDiscoveries_FiltersByTargetID(t *testing.T) {
	provider := &stubDiscoveryProvider{
		listResp: []*ResourceDiscoveryInfo{
			{
				ID:           "vm:node2:102",
				ResourceType: "vm",
				ResourceID:   "102",
				TargetID:     "node2",
				HostID:       "node2",
				Hostname:     "vm-102",
			},
		},
	}
	exec := NewPulseToolExecutor(ExecutorConfig{DiscoveryProvider: provider})

	result, err := exec.executeListDiscoveries(context.Background(), map[string]interface{}{
		"target_id": "node2",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "node2", provider.lastListTargetID)

	var payload map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
	assert.Equal(t, "node2", payload["filter_target_id"])
}
