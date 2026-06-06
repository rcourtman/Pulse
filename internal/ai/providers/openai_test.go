package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type readOnceEOFBody struct {
	payload []byte
	read    bool
}

func (b *readOnceEOFBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, io.EOF
	}
	b.read = true
	n := copy(p, b.payload)
	return n, io.EOF
}

func (b *readOnceEOFBody) Close() error {
	return nil
}

type blockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (b *blockingReadCloser) Read([]byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingReadCloser) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return nil
}

type readOnceThenBlockBody struct {
	payload []byte
	read    bool
	closed  chan struct{}
	once    sync.Once
}

func newReadOnceThenBlockBody(payload string) *readOnceThenBlockBody {
	return &readOnceThenBlockBody{
		payload: []byte(payload),
		closed:  make(chan struct{}),
	}
}

func (b *readOnceThenBlockBody) Read(p []byte) (int, error) {
	if !b.read {
		b.read = true
		return copy(p, b.payload), nil
	}
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *readOnceThenBlockBody) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return nil
}

func TestOpenAIClient_ChatStream_Success(t *testing.T) {
	// Mock OpenAI SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Send SSE events
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hello"}}],"object":"chat.completion.chunk"}`,
			`{"id":"chatcmpl-1","choices":[{"delta":{"content":" World"}}],"object":"chat.completion.chunk"}`,
			`[DONE]`,
		}

		for _, event := range events {
			if event == "[DONE]" {
				fmt.Fprintf(w, "data: %s\n\n", event)
			} else {
				fmt.Fprintf(w, "data: %s\n\n", event)
			}
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	var receivedContent string
	var doneCalled bool

	callback := func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				receivedContent += data.Text
			}
		case "done":
			doneCalled = true
		}
	}

	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}

	err := client.ChatStream(context.Background(), req, callback)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", receivedContent)
	assert.True(t, doneCalled)
}

func TestOpenAIClient_ChatStream_RetriesTransientStartupError(t *testing.T) {
	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", 0)
	attempts := 0
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
						"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n" +
						"data: [DONE]\n",
				)),
			}, nil
		}),
	}

	var content string
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(event StreamEvent) {
		if event.Type == "content" {
			content += event.Data.(ContentEvent).Text
		}
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, "ok", content)
}

func TestNewOpenAIClient_BoundsStreamResponseHeaderTimeout(t *testing.T) {
	client := NewOpenAIClient("sk-test", "gpt-4", "https://api.openai.com/v1", 0)
	transport, ok := client.streamClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, openaiStreamResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.Equal(t, openaiStreamChunkTimeout, client.streamChunkTimeout)

	shortTimeoutClient := NewOpenAIClient("sk-test", "gpt-4", "https://api.openai.com/v1", 2*time.Second)
	shortTransport, ok := shortTimeoutClient.streamClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 2*time.Second, shortTransport.ResponseHeaderTimeout)
	assert.Equal(t, 2*time.Second, shortTimeoutClient.streamChunkTimeout)
}

func TestOpenAIClient_ChatStream_TimesOutWaitingForFirstStreamChunk(t *testing.T) {
	body := newBlockingReadCloser()
	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", time.Second)
	client.streamChunkTimeout = 10 * time.Millisecond
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       body,
			}, nil
		}),
	}

	started := time.Now()
	var eventCount int
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(StreamEvent) {
		eventCount++
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream chunk timed out")
	assert.Less(t, time.Since(started), 200*time.Millisecond)
	assert.Equal(t, 0, eventCount)
}

func TestOpenAIClient_ChatStream_TimesOutWaitingForLaterStreamChunk(t *testing.T) {
	body := newReadOnceThenBlockBody(`data: {"choices":[{"delta":{"content":"first"}}]}` + "\n\n")
	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", time.Second)
	client.streamChunkTimeout = 10 * time.Millisecond
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       body,
			}, nil
		}),
	}

	started := time.Now()
	var content string
	var doneCalled bool
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			content += event.Data.(ContentEvent).Text
		case "done":
			doneCalled = true
		}
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream chunk timed out")
	assert.Less(t, time.Since(started), 200*time.Millisecond)
	assert.Equal(t, "first", content)
	assert.False(t, doneCalled)
}

// TestOpenAIClient_ChatStream_OpenRouterReasoning guards the OpenRouter reasoning
// path. OpenRouter (and other OpenAI-compatible gateways) stream chain-of-thought
// in the "reasoning" delta field rather than DeepSeek's "reasoning_content". A
// reasoning model routed via OpenRouter must surface those tokens as live thinking
// events; previously they were dropped, leaving the user with a long dead pause.
func TestOpenAIClient_ChatStream_OpenRouterReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []string{
			`{"id":"gen-1","choices":[{"delta":{"reasoning":"Let me "}}]}`,
			`{"id":"gen-1","choices":[{"delta":{"reasoning":"think."}}]}`,
			`{"id":"gen-1","choices":[{"delta":{"content":"Answer"}}]}`,
			`{"id":"gen-1","choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`[DONE]`,
		}
		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "deepseek/deepseek-v4-pro", server.URL, 0)

	var thinking, content string
	var doneCalled bool
	callback := func(event StreamEvent) {
		switch event.Type {
		case "thinking":
			if data, ok := event.Data.(ThinkingEvent); ok {
				thinking += data.Text
			}
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		case "done":
			doneCalled = true
		}
	}

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}}, callback)
	require.NoError(t, err)
	assert.Equal(t, "Let me think.", thinking, "OpenRouter 'reasoning' deltas should surface as thinking events")
	assert.Equal(t, "Answer", content)
	assert.True(t, doneCalled)
}

func TestOpenAIClient_ChatStream_OpenRouterDefaultsCompletionBudget(t *testing.T) {
	var captured openaiStreamRequest
	client := NewOpenAIClient("sk-test", "openrouter:deepseek/deepseek-v4-pro", "https://openrouter.ai/api/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
						"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n" +
						"data: [DONE]\n",
				)),
			}, nil
		}),
	}

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(StreamEvent) {})

	require.NoError(t, err)
	assert.Equal(t, openrouterDefaultMaxCompletionTokens, captured.MaxCompletionTokens)
	assert.Zero(t, captured.MaxTokens)
}

func TestOpenAIClient_ChatStream_OpenAIDoesNotDefaultCompletionBudget(t *testing.T) {
	var captured openaiStreamRequest
	client := NewOpenAIClient("sk-test", "gpt-4", "https://api.openai.com/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n" +
						"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n" +
						"data: [DONE]\n",
				)),
			}, nil
		}),
	}

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}, func(StreamEvent) {})

	require.NoError(t, err)
	assert.Zero(t, captured.MaxCompletionTokens)
	assert.Zero(t, captured.MaxTokens)
}

func TestOpenAIClient_Chat_OpenRouterDefaultsCompletionBudget(t *testing.T) {
	var captured openaiRequest
	client := NewOpenAIClient("sk-test", "openrouter:deepseek/deepseek-v4-pro", "https://openrouter.ai/api/v1", 0)
	client.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"id":"chatcmpl-1","model":"deepseek/deepseek-v4-pro","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`,
				)),
			}, nil
		}),
	}

	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, openrouterDefaultMaxCompletionTokens, captured.MaxCompletionTokens)
	assert.Zero(t, captured.MaxTokens)
}

func TestOpenAIClient_ChatStream_ToolCall(t *testing.T) {
	// Mock tool call stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		events := []string{
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}`,
			`{"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]}}]}`,
			`[DONE]`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	var toolCalls []ToolCall
	var toolStartIndex int

	callback := func(event StreamEvent) {
		t.Logf("Received event type: %s", event.Type)
		if event.Type == "tool_start" {
			toolStartIndex++
		}
		if event.Type == "done" {
			if data, ok := event.Data.(DoneEvent); ok {
				t.Logf("Received DONE event with %d tool calls", len(data.ToolCalls))
				toolCalls = data.ToolCalls
			} else {
				t.Logf("Received DONE event but type assertion to DoneEvent failed. Actual type: %T", event.Data)
			}
		}
	}

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user"}}}, callback)
	require.NoError(t, err)

	// Check that we got a tool_start event
	assert.Equal(t, 1, toolStartIndex, "Should have received 1 tool_start event")

	// Check accumulated tool calls in done event
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_123", toolCalls[0].ID)
	assert.Equal(t, "get_weather", toolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"location": "NYC"}, toolCalls[0].Input)
}

func TestOpenAIClient_ChatStream_ToolCallEOFWithBufferedDone(t *testing.T) {
	payload := strings.Join([]string{
		`data: {"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
		``,
		`data: {"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}`,
		``,
		`data: {"id":"chatcmpl-2","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &readOnceEOFBody{payload: []byte(payload)},
			}, nil
		}),
	}

	var toolStarts int
	var doneEvent DoneEvent
	var doneCalled bool

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}}, func(event StreamEvent) {
		switch event.Type {
		case "tool_start":
			toolStarts++
		case "done":
			doneCalled = true
			doneEvent = event.Data.(DoneEvent)
		}
	})
	require.NoError(t, err)
	require.True(t, doneCalled)
	assert.Equal(t, 1, toolStarts)
	require.Len(t, doneEvent.ToolCalls, 1)
	assert.Equal(t, "tool_use", doneEvent.StopReason)
	assert.Equal(t, "call_123", doneEvent.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", doneEvent.ToolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"location": "NYC"}, doneEvent.ToolCalls[0].Input)
}

func TestOpenAIClient_ChatStream_EOFWithoutDoneFinalizesToolCalls(t *testing.T) {
	payload := strings.Join([]string{
		`data: {"id":"chatcmpl-3","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_999","type":"function","function":{"name":"lookup_host","arguments":"{\"host\":\"nas01\"}"}}]}}]}`,
		``,
		`data: {"id":"chatcmpl-3","choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":12,"completion_tokens":8}}`,
		``,
	}, "\n")

	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &readOnceEOFBody{payload: []byte(payload)},
			}, nil
		}),
	}

	var doneEvent DoneEvent
	var doneCalled bool

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}}, func(event StreamEvent) {
		if event.Type == "done" {
			doneCalled = true
			doneEvent = event.Data.(DoneEvent)
		}
	})
	require.NoError(t, err)
	require.True(t, doneCalled)
	require.Len(t, doneEvent.ToolCalls, 1)
	assert.Equal(t, "tool_use", doneEvent.StopReason)
	assert.Equal(t, 12, doneEvent.InputTokens)
	assert.Equal(t, 8, doneEvent.OutputTokens)
	assert.Equal(t, "call_999", doneEvent.ToolCalls[0].ID)
	assert.Equal(t, "lookup_host", doneEvent.ToolCalls[0].Name)
	assert.Equal(t, map[string]interface{}{"host": "nas01"}, doneEvent.ToolCalls[0].Input)
}

func TestOpenAIClient_ChatStream_EOFWithoutDoneRejectsIncompleteToolCall(t *testing.T) {
	payload := strings.Join([]string{
		`data: {"id":"chatcmpl-4","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_incomplete","type":"function","function":{"name":"lookup_host","arguments":"{\"host\""}}]}}]}`,
		``,
	}, "\n")

	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &readOnceEOFBody{payload: []byte(payload)},
			}, nil
		}),
	}

	var doneCalled bool

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}}, func(event StreamEvent) {
		if event.Type == "done" {
			doneCalled = true
		}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream ended before tool call completion")
	assert.False(t, doneCalled)
}

func TestOpenAIClient_ChatStream_EOFWithoutDoneFinalizesStopResponse(t *testing.T) {
	payload := strings.Join([]string{
		`data: {"id":"chatcmpl-5","choices":[{"delta":{"content":"Done"}}]}`,
		``,
		`data: {"id":"chatcmpl-5","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`,
		``,
	}, "\n")

	client := NewOpenAIClient("sk-test", "gpt-4", "https://example.invalid/v1", 0)
	client.streamClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &readOnceEOFBody{payload: []byte(payload)},
			}, nil
		}),
	}

	var content string
	var doneEvent DoneEvent
	var doneCalled bool

	err := client.ChatStream(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "Hi"}}}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			content += event.Data.(ContentEvent).Text
		case "done":
			doneCalled = true
			doneEvent = event.Data.(DoneEvent)
		}
	})
	require.NoError(t, err)
	require.True(t, doneCalled)
	assert.Equal(t, "Done", content)
	assert.Equal(t, "end_turn", doneEvent.StopReason)
	assert.Equal(t, 7, doneEvent.InputTokens)
	assert.Equal(t, 3, doneEvent.OutputTokens)
	assert.Empty(t, doneEvent.ToolCalls)
}

func TestOpenAIClient_Chat_ToolChoiceNone_DropsTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		if tools, ok := got["tools"]; ok {
			toolList, isList := tools.([]interface{})
			require.True(t, isList, "tools field should be a JSON array when present")
			assert.Len(t, toolList, 0, "tools should be omitted or empty when tool_choice is none")
		}
		_, hasToolChoice := got["tool_choice"]
		assert.False(t, hasToolChoice, "tool_choice should be omitted when tools are dropped")

		_ = json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-none-tools",
			Model: "gpt-4",
			Choices: []openaiChoice{
				{
					Message:      openaiRespMsg{Role: "assistant", Content: "No tools"},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
		ToolChoice: &ToolChoice{Type: ToolChoiceNone},
	})
	require.NoError(t, err)
	assert.Equal(t, "No tools", resp.Content)
}

func TestOpenAIClient_ChatStream_ToolChoiceNone_DropsTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		if tools, ok := got["tools"]; ok {
			toolList, isList := tools.([]interface{})
			require.True(t, isList, "tools field should be a JSON array when present")
			assert.Len(t, toolList, 0, "tools should be omitted or empty when tool_choice is none")
		}
		_, hasToolChoice := got["tool_choice"]
		assert.False(t, hasToolChoice, "tool_choice should be omitted when tools are dropped")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		events := []string{
			`{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hello"}}],"object":"chat.completion.chunk"}`,
			`[DONE]`,
		}
		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	var content string
	var doneCalled bool

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
		ToolChoice: &ToolChoice{Type: ToolChoiceNone},
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(ContentEvent); ok {
				content += data.Text
			}
		case "done":
			doneCalled = true
		}
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", content)
	assert.True(t, doneCalled)
}

func TestOpenAIClient_Chat_DeepSeekPreservesReasoningContentForToolTurns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got openaiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		require.Len(t, got.Messages, 3)
		assert.Equal(t, "assistant", got.Messages[1].Role)
		assert.Equal(t, "I need current state first.", got.Messages[1].ReasoningContent)
		require.Len(t, got.Messages[1].ToolCalls, 1)

		_ = json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-deepseek-reasoning",
			Model: "deepseek-v4-flash",
			Choices: []openaiChoice{
				{
					Message:      openaiRespMsg{Role: "assistant", Content: "ok"},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "deepseek-v4-flash", server.URL, 0)
	client.baseURL = strings.TrimSuffix(server.URL, "/") + "/v1/chat/completions#deepseek.com"

	_, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Check status"},
			{
				Role:             "assistant",
				ReasoningContent: "I need current state first.",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "pulse_read", Input: map[string]interface{}{"action": "status"}},
				},
			},
			{Role: "tool", ToolResult: &ToolResult{ToolUseID: "call-1", Content: "healthy"}},
		},
	})
	require.NoError(t, err)
}

func TestOpenAIClient_ChatStream_DeepSeekPreservesReasoningContentForToolTurns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got openaiStreamRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		require.Len(t, got.Messages, 3)
		assert.Equal(t, "assistant", got.Messages[1].Role)
		assert.Equal(t, "I need current state first.", got.Messages[1].ReasoningContent)
		require.Len(t, got.Messages[1].ToolCalls, 1)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"chatcmpl-1","choices":[{"delta":{"content":"ok"}}],"object":"chat.completion.chunk"}`)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "deepseek-v4-flash", server.URL, 0)
	client.baseURL = strings.TrimSuffix(server.URL, "/") + "/v1/chat/completions#deepseek.com"

	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Check status"},
			{
				Role:             "assistant",
				ReasoningContent: "I need current state first.",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "pulse_read", Input: map[string]interface{}{"action": "status"}},
				},
			},
			{Role: "tool", ToolResult: &ToolResult{ToolUseID: "call-1", Content: "healthy"}},
		},
	}, func(event StreamEvent) {})
	require.NoError(t, err)
}

func TestOpenAIClient_ChatStream_Errors(t *testing.T) {
	t.Run("401 Unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "Invalid API key"},
			})
		}))
		defer server.Close()

		client := NewOpenAIClient("bad-key", "gpt-4", server.URL, 0)
		err := client.ChatStream(context.Background(), ChatRequest{}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("Invalid Context", func(t *testing.T) {
		// No server needed if context cancelled immediately
		client := NewOpenAIClient("sk", "gpt", "http://localhost:12345", 0)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.ChatStream(ctx, ChatRequest{}, nil)
		assert.Error(t, err)
	})
}

func TestOpenAIClient_Configuration(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "Default",
			baseURL:  "",
			expected: "https://api.openai.com/v1/chat/completions",
		},
		{
			name:     "Custom Base URL",
			baseURL:  "https://custom.api/v1",
			expected: "https://custom.api/v1/chat/completions",
		},
		{
			name:     "Custom Full URL",
			baseURL:  "https://custom.api/v1/chat/completions",
			expected: "https://custom.api/v1/chat/completions",
		},
		{
			name:     "OpenRouter Style",
			baseURL:  "https://openrouter.ai/api/v1",
			expected: "https://openrouter.ai/api/v1/chat/completions",
		},
		{
			name:     "DeepSeek Default Full URL",
			baseURL:  "https://api.deepseek.com/chat/completions",
			expected: "https://api.deepseek.com/chat/completions",
		},
		{
			name:     "Root URL",
			baseURL:  "https://my-local-llm:8080",
			expected: "https://my-local-llm:8080/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOpenAIClient("key", "model", tt.baseURL, 0)
			assert.NotNil(t, client)
			assert.Equal(t, tt.expected, client.baseURL)
		})
	}
}

func TestNewOpenAIClient_StripsOpenRouterPrefix(t *testing.T) {
	client := NewOpenAIClient("key", "openrouter:openai/gpt-4o-mini", "https://openrouter.ai/api/v1", 0)
	assert.Equal(t, "openai/gpt-4o-mini", client.model)
}

func TestOpenAIClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "gpt-4", "object": "model", "created": 1234567890, "owned_by": "openai"},
				{"id": "gpt-3.5-turbo", "object": "model", "created": 1234567890, "owned_by": "openai"},
				{"id": "claude-3", "object": "model", "created": 1234567890, "owned_by": "anthropic"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)

	assert.Len(t, models, 2)
	assert.Equal(t, "gpt-4", models[0].ID)
	assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
}

func TestOpenAIClient_ListModels_OpenRouterReturnsCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "https://pulse.app", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "Pulse", r.Header.Get("X-Title"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "anthropic/claude-sonnet-4.5", "name": "Claude Sonnet 4.5"},
				{"id": "openai/gpt-4o-mini", "name": "GPT-4o mini", "description": "Fast and cheap"},
				{"id": "meta-llama/llama-3.3-70b-instruct", "name": "Llama 3.3 70B Instruct"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "openai/gpt-4o-mini", server.URL, 0)
	client.baseURL = strings.TrimSuffix(server.URL, "/") + "/api/v1/chat/completions#openrouter.ai"

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", models[0].ID)
	assert.Equal(t, "Claude Sonnet 4.5", models[0].Name)
	assert.Equal(t, "Fast and cheap", models[1].Description)
	assert.Equal(t, "meta-llama/llama-3.3-70b-instruct", models[2].ID)
}

func TestOpenAIClient_TestConnection_OpenRouterValidatesCurrentKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/key", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "https://pulse.app", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "Pulse", r.Header.Get("X-Title"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"label":           "sk-or-v1-test",
				"limit_remaining": 10,
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "openai/gpt-4o-mini", server.URL, 0)
	client.baseURL = strings.TrimSuffix(server.URL, "/") + "/api/v1/chat/completions#openrouter.ai"

	require.NoError(t, client.TestConnection(context.Background()))
}

func TestOpenAIClient_TestConnection_OpenRouterRejectsUnauthenticatedKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/key", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(openaiError{
			Error: openaiErrorDetail{
				Message: "Missing Authentication header",
				Code:    "401",
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("", "openai/gpt-4o-mini", server.URL, 0)
	client.baseURL = strings.TrimSuffix(server.URL, "/") + "/api/v1/chat/completions#openrouter.ai"

	err := client.TestConnection(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openrouter test connection failed")
	assert.Contains(t, err.Error(), "Missing Authentication header")
}

func TestOpenAIClient_Chat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))

		var req openaiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "gpt-4", req.Model)
		assert.Equal(t, 123, req.MaxCompletionTokens)
		assert.Equal(t, 0.7, req.Temperature)
		require.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_time", req.Tools[0].Function.Name)
		assert.Nil(t, req.ToolChoice)

		_ = json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-1",
			Model: "gpt-4",
			Choices: []openaiChoice{
				{
					Message:      openaiRespMsg{Role: "assistant", Content: "Hello"},
					FinishReason: "stop",
				},
			},
			Usage: openaiUsage{PromptTokens: 2, CompletionTokens: 3},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	resp, err := client.Chat(context.Background(), ChatRequest{
		System:      "You are helpful",
		MaxTokens:   123,
		Temperature: 0.7,
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Tools: []Tool{
			{
				Name:        "get_time",
				Description: "get time",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Type: "web_search_20250305",
				Name: "web_search",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Content)
	assert.Equal(t, 2, resp.InputTokens)
	assert.Equal(t, 3, resp.OutputTokens)
}

func TestOpenAIClient_HelperFlags(t *testing.T) {
	client := NewOpenAIClient("sk", "gpt-4", "https://api.openai.com", 0)
	assert.True(t, client.requiresMaxCompletionTokens("o1-mini"))
	assert.False(t, client.requiresMaxCompletionTokens("gpt-4"))
}

func TestOpenAIClient_SupportsThinking(t *testing.T) {
	client := NewOpenAIClient("sk", "deepseek-reasoner", "https://api.deepseek.com", 0)
	assert.True(t, client.SupportsThinking("deepseek-reasoner"))

	client = NewOpenAIClient("sk", "gpt-4", "https://api.openai.com", 0)
	assert.False(t, client.SupportsThinking("gpt-4"))
}

func TestOpenAIClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "gpt-4", "object": "model", "created": 123, "owned_by": "openai"},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient("sk-test", "gpt-4", server.URL, 0)
	err := client.TestConnection(context.Background())
	require.NoError(t, err)
}
