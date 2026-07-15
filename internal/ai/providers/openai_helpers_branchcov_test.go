package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeOpenAICompatibleChatURL_BranchCov drives every branch of
// normalizeOpenAICompatibleChatURL directly. The sibling client-construction
// tests already exercise the empty, /chat/completions passthrough, /completions
// rewrite, empty-host-path, and non-empty-path append branches; this table adds
// the previously uncovered guard arm where url.Parse fails or the parsed URL has
// no scheme/host (relative or bare-token base URLs), and both of its sub-arms.
func TestNormalizeOpenAICompatibleChatURL_BranchCov(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		// Branch: empty input falls back to the canonical OpenAI endpoint.
		{
			name:    "empty returns default openai url",
			baseURL: "",
			want:    openaiAPIURL,
		},
		// Branch: whitespace-only trims to empty and falls back too.
		{
			name:    "whitespace only returns default openai url",
			baseURL: "   ",
			want:    openaiAPIURL,
		},
		// Branch: already targets /chat/completions -> returned verbatim (trailing
		// slashes stripped).
		{
			name:    "chat completions suffix passes through",
			baseURL: "https://api.deepseek.com/chat/completions",
			want:    "https://api.deepseek.com/chat/completions",
		},
		// Branch: legacy /completions endpoint rewritten to /chat/completions.
		{
			name:    "completions suffix rewritten to chat completions",
			baseURL: "https://api.mistral.ai/v1/completions",
			want:    "https://api.mistral.ai/v1/chat/completions",
		},
		// Branch: invalid URL (url.Parse error from a malformed IPv6 literal)
		// without scheme/host takes the relative-with-slash arm.
		{
			name:    "malformed url with slash appends chat completions",
			baseURL: "http://[::1",
			want:    "http://[::1/chat/completions",
		},
		// Branch: url.Parse error (empty protocol scheme) with a slash.
		{
			name:    "missing protocol scheme with slash appends chat completions",
			baseURL: "://bad-scheme",
			want:    "://bad-scheme/chat/completions",
		},
		// Branch: relative URL with a slash (no scheme/host) appends /chat/completions.
		{
			name:    "relative path with slash appends chat completions",
			baseURL: "foo/bar",
			want:    "foo/bar/chat/completions",
		},
		// Branch: leading-slash relative URL still counts as having a slash.
		{
			name:    "leading slash relative path appends chat completions",
			baseURL: "/relative/path",
			want:    "/relative/path/chat/completions",
		},
		// Branch: bare token with no slash and no scheme/host defaults to /v1.
		{
			name:    "no slash token defaults to v1 chat completions",
			baseURL: "localhost",
			want:    "localhost/v1/chat/completions",
		},
		// Branch: host:port with no path parses without scheme/host (bare token),
		// so it also takes the no-slash default arm.
		{
			name:    "host port without scheme defaults to v1 chat completions",
			baseURL: "localhost:8080",
			want:    "localhost:8080/v1/chat/completions",
		},
		// Branch: parsed absolute URL with an empty path gets /v1/chat/completions.
		{
			name:    "root host with empty path defaults to v1 chat completions",
			baseURL: "https://my-local-llm:8080",
			want:    "https://my-local-llm:8080/v1/chat/completions",
		},
		// Branch: parsed absolute URL with a non-empty path appends /chat/completions.
		{
			name:    "host with versioned path appends chat completions",
			baseURL: "https://api.openai.com/v1",
			want:    "https://api.openai.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOpenAICompatibleChatURL(tt.baseURL)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNotableProviderForOpenRouterModel_BranchCov covers each branch of
// notableProviderForOpenRouterModel: the no-slash default, the empty-provider
// default, and the passthrough (with whitespace and case normalization).
func TestNotableProviderForOpenRouterModel_BranchCov(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    string
	}{
		// Branch: no slash -> default to openai.
		{
			name:    "bare model without provider defaults to openai",
			modelID: "gpt-4",
			want:    "openai",
		},
		// Branch: SplitN yields a single part for an identifier with no slash.
		{
			name:    "deepseek bare model defaults to openai",
			modelID: "deepseek-chat",
			want:    "openai",
		},
		// Branch: leading slash yields an empty provider segment -> default.
		{
			name:    "leading slash empty provider defaults to openai",
			modelID: "/gpt-4",
			want:    "openai",
		},
		// Branch: whitespace collapses to empty provider -> default.
		{
			name:    "whitespace only defaults to openai",
			modelID: "   ",
			want:    "openai",
		},
		// Branch: passthrough lowercases and trims the provider segment.
		{
			name:    "provider segment lowercased and passed through",
			modelID: "anthropic/claude-sonnet-4.5",
			want:    "anthropic",
		},
		// Branch: passthrough preserves hyphenated multi-word providers.
		{
			name:    "hyphenated provider passed through",
			modelID: "meta-llama/llama-3.3-70b-instruct",
			want:    "meta-llama",
		},
		// Branch: provider whitespace and mixed case normalized before passthrough.
		{
			name:    "provider with surrounding whitespace and case normalized",
			modelID: "Anthropic /claude-opus-4-5",
			want:    "anthropic",
		},
		// Branch: openai provider passes through explicitly.
		{
			name:    "openai provider passed through",
			modelID: "openai/gpt-4o-mini",
			want:    "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notableProviderForOpenRouterModel(tt.modelID)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNormalizeOpenAIStreamStopReason_BranchCov drives every branch of
// normalizeOpenAIStreamStopReason directly. The streaming integration tests
// reach the tool_use and end_turn arms indirectly, but no test exercises the
// passthrough arm for a non-empty, non-"stop" finish reason; this table covers
// all three arms with real return-value assertions.
func TestNormalizeOpenAIStreamStopReason_BranchCov(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
		toolCalls    []ToolCall
		want         string
	}{
		// Branch: tool calls present -> tool_use regardless of finish reason.
		{
			name:         "tool calls force tool_use over stop",
			finishReason: "stop",
			toolCalls: []ToolCall{
				{ID: "call_1", Name: "get_weather", Input: map[string]interface{}{"q": "NYC"}},
			},
			want: "tool_use",
		},
		{
			name:         "tool calls force tool_use over tool_calls reason",
			finishReason: "tool_calls",
			toolCalls: []ToolCall{
				{ID: "call_2", Name: "search", Input: map[string]interface{}{}},
			},
			want: "tool_use",
		},
		// Branch: empty finish reason with no tool calls -> end_turn.
		{
			name:         "empty finish reason defaults to end_turn",
			finishReason: "",
			toolCalls:    nil,
			want:         "end_turn",
		},
		// Branch: literal "stop" finish reason -> end_turn.
		{
			name:         "stop finish reason becomes end_turn",
			finishReason: "stop",
			toolCalls:    nil,
			want:         "end_turn",
		},
		// Branch: passthrough for a non-empty, non-"stop" reason (uncovered arm).
		{
			name:         "length reason passes through",
			finishReason: "length",
			toolCalls:    nil,
			want:         "length",
		},
		// Branch: passthrough for an arbitrary provider-specific reason.
		{
			name:         "content filter reason passes through",
			finishReason: "content_filter",
			toolCalls:    nil,
			want:         "content_filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOpenAIStreamStopReason(tt.finishReason, tt.toolCalls)
			assert.Equal(t, tt.want, got)
		})
	}
}
