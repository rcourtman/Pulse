package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Issue #1614: llama.cpp's /v1/chat/completions commonly returns tool_calls
// without an id. The Ollama adapter synthesises one, but the OpenAI-compatible
// adapter copied the empty id through (streaming) or rejected the response
// outright (buffered), so the Patrol readiness validator failed tool protocol
// 0/3 regardless of the model's actual tool-calling ability.

func TestIssue1614StreamFinalizerSynthesisesMissingToolCallIDs(t *testing.T) {
	builders := map[int]*openaiStreamToolCallBuilder{
		0: {id: "", name: "readiness_record_observation"},
		1: {id: "server-provided", name: "readiness_confirm_result"},
	}
	builders[0].args.WriteString(`{"nonce":"abc"}`)
	builders[1].args.WriteString(`{}`)

	toolCalls := finalizeOpenAIStreamToolCalls(builders)
	if len(toolCalls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(toolCalls))
	}
	if strings.TrimSpace(toolCalls[0].ID) == "" {
		t.Fatal("missing server id must be synthesised, got empty")
	}
	if toolCalls[1].ID != "server-provided" {
		t.Fatalf("server-provided id must be preserved, got %q", toolCalls[1].ID)
	}
	if toolCalls[0].ID == toolCalls[1].ID {
		t.Fatal("synthesised ids must not collide with other calls")
	}
}

func TestIssue1614StreamedToolCallWithoutIDGetsSynthesisedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"readiness_record_observation","arguments":"{\"nonce\":"}}]}}]}` + "\n\n" +
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"abc\"}"}}]}}]}` + "\n\n" +
				`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n" +
				"data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "local-model", server.URL, 5*time.Second)
	var done DoneEvent
	err := client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "probe"}},
	}, func(event StreamEvent) {
		if event.Type == "done" {
			done = event.Data.(DoneEvent)
		}
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(done.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(done.ToolCalls))
	}
	call := done.ToolCalls[0]
	if strings.TrimSpace(call.ID) == "" {
		t.Fatalf("llama.cpp-style tool call without id must get a synthesised id: %+v", call)
	}
	if call.Name != "readiness_record_observation" || call.Input["nonce"] != "abc" {
		t.Fatalf("unexpected tool call: %+v", call)
	}
}

func TestIssue1614BufferedToolCallWithoutIDGetsSynthesisedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"local-model","choices":[{"message":{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"readiness_record_observation","arguments":"{\"nonce\":\"abc\"}"}}]},"finish_reason":"tool_calls"}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient("openai", "", "local-model", server.URL, 5*time.Second)
	response, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "probe"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(response.ToolCalls))
	}
	if strings.TrimSpace(response.ToolCalls[0].ID) == "" {
		t.Fatalf("buffered tool call without id must get a synthesised id: %+v", response.ToolCalls[0])
	}
}

func TestIssue1614BufferedToolCallWithoutNameStillRejected(t *testing.T) {
	response := openaiResponse{
		Choices: []openaiChoice{{
			Message: openaiRespMsg{
				Role:      "assistant",
				ToolCalls: []openaiToolCall{{ID: "call-1", Function: openaiToolFunction{Name: "", Arguments: "{}"}}},
			},
			FinishReason: "tool_calls",
		}},
	}
	if _, err := convertOpenAIResponse(response); err == nil {
		t.Fatal("a tool call without a function name must still be rejected")
	}
}
