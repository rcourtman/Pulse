package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatStream_ToolResultsConnection(t *testing.T) {
	// Setup a mock server to capture the request sent by ChatStream
	var capturedBody geminiRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models/gemini-pro:streamGenerateContent" {
			json.NewDecoder(r.Body).Decode(&capturedBody)
			// Return a minimal SSE response to complete the request
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"candidates\": [{\"content\": {\"parts\": [{\"text\": \"Response\"}]}}]}\n\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewGeminiClient("fake-key", "gemini-pro", ts.URL, 10*time.Second)

	// Create a conversation history with tool usage
	// 1. User asks question
	// 2. Assistant calls tool
	// 3. Tool returns result
	toolID := "list_alerts_0"
	toolName := "list_alerts"

	messages := []Message{
		{
			Role:    "user",
			Content: "List alerts",
		},
		{
			Role: "assistant", // Model calls use "model" role in Gemini, "assistant" in generic
			ToolCalls: []ToolCall{
				{
					ID:   toolID,
					Name: toolName,
					Input: map[string]interface{}{
						"limit": 10,
					},
				},
			},
		},
		{
			Role: "user",
			ToolResult: &ToolResult{
				ToolUseID: toolID, // Result uses ID
				Content:   `{"alerts": []}`,
			},
		},
	}

	req := ChatRequest{
		Messages: messages,
		Model:    "gemini-pro",
	}

	err := client.ChatStream(context.Background(), req, func(event StreamEvent) {})
	assert.NoError(t, err)

	// Verify the request sent to Gemini
	// We expect the ToolResult message to be converted to "functionResponse"
	// AND the name should be resolved to "list_alerts" (from previous assistant msg) not "list_alerts_0"

	// Check content structure
	// Index 0: User "List alerts"
	// Index 1: Model "functionCall"
	// Index 2: User "functionResponse"

	assert.Equal(t, 3, len(capturedBody.Contents))

	// Verify Index 2 (Tool Result)
	lastContent := capturedBody.Contents[2]
	assert.Equal(t, "user", lastContent.Role)
	assert.Equal(t, 1, len(lastContent.Parts))

	part := lastContent.Parts[0]
	assert.NotNil(t, part.FunctionResponse)
	// THIS IS THE KEY ASSERTION: Name must match function name, not ID
	assert.Equal(t, toolName, part.FunctionResponse.Name)
	assert.Equal(t, `{"alerts": []}`, part.FunctionResponse.Response.Content)
}

func TestChatStream_ToolResults_MultipleMerged(t *testing.T) {
	// Setup a mock server
	var capturedBody geminiRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {}\n\n"))
	}))
	defer ts.Close()

	client := NewGeminiClient("fake-key", "gemini-pro", ts.URL, 10*time.Second)

	messages := []Message{
		{Role: "user", Content: "Run"},
		{
			Role: "assistant", // Assistant calls 3 tools
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "func1", Input: nil},
				{ID: "call_2", Name: "func2", Input: nil},
				{ID: "call_3", Name: "func3", Input: nil},
			},
		},
		{
			Role:       "user",
			ToolResult: &ToolResult{ToolUseID: "call_1", Content: "res1"},
		},
		{
			Role:       "user",
			ToolResult: &ToolResult{ToolUseID: "call_2", Content: "res2"},
		},
		{
			Role:       "user",
			ToolResult: &ToolResult{ToolUseID: "call_3", Content: "res3"},
		},
	}

	req := ChatRequest{Messages: messages}
	client.ChatStream(context.Background(), req, func(e StreamEvent) {})

	// Expect merged content for the tool results
	// Contents: [User, Model(3 calls), User(merged 3 results)]
	assert.Equal(t, 3, len(capturedBody.Contents))

	mergedUserMsg := capturedBody.Contents[2]
	assert.Equal(t, "user", mergedUserMsg.Role)
	assert.Equal(t, 3, len(mergedUserMsg.Parts))

	// Check correctness of resolved names for ALL parts
	// Previously, the 3rd part likely failed name resolution
	assert.Equal(t, "func1", mergedUserMsg.Parts[0].FunctionResponse.Name)
	assert.Equal(t, "res1", mergedUserMsg.Parts[0].FunctionResponse.Response.Content)

	assert.Equal(t, "func2", mergedUserMsg.Parts[1].FunctionResponse.Name)
	assert.Equal(t, "res2", mergedUserMsg.Parts[1].FunctionResponse.Response.Content)

	assert.Equal(t, "func3", mergedUserMsg.Parts[2].FunctionResponse.Name)
	assert.Equal(t, "res3", mergedUserMsg.Parts[2].FunctionResponse.Response.Content)
}
