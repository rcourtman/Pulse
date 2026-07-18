package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCompactChatMessageForTransportStripsBulkFields(t *testing.T) {
	isError := true
	msg := chat.Message{
		ID:      "m1",
		Role:    "assistant",
		Content: "ran a tool",
		ToolCalls: []chat.ToolCall{
			{
				ID:               "tool-1",
				Name:             "run_command",
				Output:           strings.Repeat("x", 10_000),
				ThoughtSignature: json.RawMessage(`"sig"`),
			},
		},
		ToolResult: &chat.ToolResult{
			ToolUseID: "tool-1",
			Content:   strings.Repeat("y", 10_000),
			IsError:   isError,
		},
	}

	compacted := compactChatMessageForTransport(msg)

	if compacted.ToolCalls[0].Output != "" {
		t.Errorf("expected tool output stripped, got %d bytes", len(compacted.ToolCalls[0].Output))
	}
	if compacted.ToolCalls[0].ThoughtSignature != nil {
		t.Error("expected thought signature stripped")
	}
	if compacted.ToolCalls[0].ID != "tool-1" || compacted.ToolCalls[0].Name != "run_command" {
		t.Error("expected tool identity preserved")
	}
	if compacted.ToolResult == nil {
		t.Fatal("expected tool result retained")
	}
	if compacted.ToolResult.Content != "" {
		t.Errorf("expected tool result content stripped, got %d bytes", len(compacted.ToolResult.Content))
	}
	if compacted.ToolResult.ToolUseID != "tool-1" {
		t.Error("expected tool_use_id linkage preserved")
	}
	if !compacted.ToolResult.IsError {
		t.Error("expected is_error preserved")
	}
	if compacted.Content != "ran a tool" {
		t.Error("expected content untouched")
	}
}

func TestTrimChatMessagesToByteBudgetKeepsNewestThatFit(t *testing.T) {
	messages := []chat.Message{
		{ID: "old", Role: "user", Content: strings.Repeat("a", 500)},
		{ID: "mid", Role: "assistant", Content: strings.Repeat("b", 500)},
		{ID: "new", Role: "user", Content: strings.Repeat("c", 500)},
	}

	// Budget fits roughly two messages, not three.
	trimmed := trimChatMessagesToByteBudget(messages, 1400)

	if len(trimmed) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(trimmed))
	}
	if trimmed[0].ID != "mid" || trimmed[1].ID != "new" {
		t.Errorf("expected newest messages kept in order, got %s, %s", trimmed[0].ID, trimmed[1].ID)
	}

	encoded, err := json.Marshal(trimmed)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 1400 {
		t.Errorf("encoded result exceeds budget: %d bytes", len(encoded))
	}
}

func TestTrimChatMessagesToByteBudgetTruncatesSingleOversizedMessage(t *testing.T) {
	messages := []chat.Message{
		{ID: "huge", Role: "assistant", Content: strings.Repeat("z", 100_000)},
	}

	trimmed := trimChatMessagesToByteBudget(messages, 2000)

	if len(trimmed) != 1 {
		t.Fatalf("expected the newest message returned, got %d", len(trimmed))
	}
	if !strings.HasSuffix(trimmed[0].Content, chatMessageTruncationNotice) {
		t.Error("expected truncation notice appended")
	}

	encoded, err := json.Marshal(trimmed)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 2000 {
		t.Errorf("encoded result exceeds budget: %d bytes", len(encoded))
	}
}

func TestTrimChatMessagesToByteBudgetNoBudgetReturnsAll(t *testing.T) {
	messages := []chat.Message{
		{ID: "a", Role: "user", Content: "one"},
		{ID: "b", Role: "assistant", Content: "two"},
	}

	if got := trimChatMessagesToByteBudget(messages, 0); len(got) != 2 {
		t.Errorf("zero budget should return input unchanged, got %d messages", len(got))
	}
	if got := trimChatMessagesToByteBudget(nil, 1000); got != nil {
		t.Errorf("nil input should stay nil")
	}
}

func TestHandleMessagesCompactAndByteBudget(t *testing.T) {
	cfg := &config.Config{}
	h := newTestAIHandler(cfg, nil, nil)
	mockSvc := new(MockAIService)
	h.defaultService = mockSvc

	mockSvc.On("IsRunning").Return(true)
	messages := []chat.Message{
		{ID: "old", Role: "user", Content: strings.Repeat("a", 30_000)},
		{
			ID:      "tools",
			Role:    "assistant",
			Content: "ran a tool",
			ToolCalls: []chat.ToolCall{
				{ID: "t1", Name: "run_command", Output: strings.Repeat("o", 20_000)},
			},
		},
		{ID: "new", Role: "user", Content: "latest question"},
	}
	mockSvc.On("GetMessages", mock.Anything, "s1").Return(messages, nil)

	req := httptest.NewRequest("GET", "/api/ai/sessions/s1/messages?limit=30&compact=1&max_bytes=4096", nil)
	w := httptest.NewRecorder()

	h.HandleMessages(w, req, "s1")

	assert.Equal(t, http.StatusOK, w.Code)
	if w.Body.Len() > 4096 {
		t.Fatalf("response exceeds byte budget: %d", w.Body.Len())
	}
	assert.Contains(t, w.Body.String(), "latest question")
	assert.Contains(t, w.Body.String(), `"tools"`)
	assert.NotContains(t, w.Body.String(), strings.Repeat("o", 100))
	assert.NotContains(t, w.Body.String(), `"old"`)
}
