package chat

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockmode"
	"github.com/rs/zerolog/log"
)

const (
	mockAssistantModelRoute    = "pulse:mock-assistant"
	mockAssistantQueryToolID   = "mock-pulse-query"
	mockAssistantQueryToolName = "pulse_query"
	mockAssistantReadToolID    = "mock-pulse-read"
	mockAssistantReadToolName  = "pulse_read"
	mockAssistantToolID        = mockAssistantQueryToolID
	mockAssistantToolName      = mockAssistantQueryToolName
)

var mockAssistantStreamPace = 220 * time.Millisecond

func pauseMockAssistantStream(ctx context.Context) bool {
	if mockAssistantStreamPace <= 0 {
		return true
	}
	timer := time.NewTimer(mockAssistantStreamPace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func streamMockAssistantTurnIfEnabled(ctx context.Context, sessions *SessionStore, sessionID, prompt string, streamCallback StreamCallback) bool {
	if !mockmode.IsEnabled() {
		return false
	}
	if streamCallback == nil {
		streamCallback = func(StreamEvent) {}
	}

	queryInput := map[string]interface{}{
		"action":       "topology",
		"summary_only": true,
		"source":       "mock_assistant_fixture",
	}
	queryOutputBytes, _ := json.Marshal(map[string]interface{}{
		"source": "mock_assistant_fixture",
		"summary": map[string]interface{}{
			"nodes":           3,
			"workloads":       8,
			"active_findings": 2,
		},
	})
	queryOutput := string(queryOutputBytes)
	readInput := map[string]interface{}{
		"action":      "exec",
		"target_host": "current_resource",
		"command":     "ls /dev | wc -l && echo '---' && lsblk -d -o NAME,TYPE,SIZE",
	}
	readRawInput := `pulse_read(target_host="current_resource",command="ls/dev|wc-l&&echo'---'&&lsblk-d-oNAME,TYPE,SIZE2>/dev/null||echo'lsblknotavailable'")`
	readOutput := "devices=42\nblock_devices=8"

	emitWorkflowState(streamCallback, "mock_context", "Preparing mock Assistant fixture.", "mock", "")
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolStartEvent(streamCallback, mockAssistantQueryToolID, mockAssistantQueryToolName, queryInput)
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolProgressEvent(streamCallback, mockAssistantQueryToolID, mockAssistantQueryToolName, queryInput, "running", "Reading synthetic Pulse inventory.")
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolProgressEvent(streamCallback, mockAssistantQueryToolID, mockAssistantQueryToolName, queryInput, "running", "Summarizing mock inventory result.")
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolEndEvent(streamCallback, mockAssistantQueryToolID, mockAssistantQueryToolName, queryInput, queryOutput, true)
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolStartEvent(streamCallback, mockAssistantReadToolID, mockAssistantReadToolName, readInput)
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolProgressEventWithRawInput(streamCallback, mockAssistantReadToolID, mockAssistantReadToolName, readInput, readRawInput, "running", "Reading mock device inventory.")
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolProgressEvent(streamCallback, mockAssistantReadToolID, mockAssistantReadToolName, readInput, "running", "Summarizing mock device count.")
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitToolEndEvent(streamCallback, mockAssistantReadToolID, mockAssistantReadToolName, readInput, readOutput, true)
	if !pauseMockAssistantStream(ctx) {
		return true
	}
	emitWorkflowState(streamCallback, "mock_response", "Composing mock Assistant response.", "mock", mockAssistantReadToolName)

	chunks := mockAssistantResponseChunks(prompt)
	answer := strings.Join(chunks, "")
	if sessions != nil {
		success := true
		if err := sessions.AddMessage(sessionID, Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   answer,
			Model:     mockAssistantModelRoute,
			Timestamp: time.Now(),
			ToolCalls: []ToolCall{{
				ID:      mockAssistantQueryToolID,
				Name:    mockAssistantQueryToolName,
				Input:   queryInput,
				Output:  queryOutput,
				Success: &success,
			}, {
				ID:      mockAssistantReadToolID,
				Name:    mockAssistantReadToolName,
				Input:   readInput,
				Output:  readOutput,
				Success: &success,
			}},
		}); err != nil {
			log.Warn().Err(err).Str("session_id", sessionID).Msg("[ChatService] Failed to save mock Assistant answer")
		}
	}

	for _, chunk := range chunks {
		if !pauseMockAssistantStream(ctx) {
			return true
		}
		contentData, _ := json.Marshal(ContentData{Text: chunk})
		streamCallback(StreamEvent{Type: "content", Data: contentData})
	}
	doneData, _ := json.Marshal(DoneData{SessionID: sessionID, Model: mockAssistantModelRoute})
	streamCallback(StreamEvent{Type: "done", Data: doneData})
	return true
}

func mockAssistantResponseChunks(prompt string) []string {
	prompt = strings.TrimSpace(prompt)
	firstChunk := "Mock Assistant fixture completed. "
	if prompt != "" {
		firstChunk = "Mock Assistant fixture completed for this request. "
	}
	return []string{
		firstChunk,
		"I read a synthetic Pulse inventory snapshot with pulse_query, then inspected mock device inventory with pulse_read. ",
		"The fixture found 3 nodes, 8 workloads, 42 device entries, and 2 active findings. ",
		"This deterministic stream exercises status, tool, content, and done updates without waiting on a live model provider.",
	}
}

func mockAssistantProviderIfEnabled() (providers.StreamingProvider, bool) {
	if !mockmode.IsEnabled() {
		return nil, false
	}
	return mockAssistantStreamingProvider{}, true
}

type mockAssistantStreamingProvider struct{}

func (mockAssistantStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	prompt := ""
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}
	return &providers.ChatResponse{
		Content:      strings.Join(mockAssistantResponseChunks(prompt), ""),
		Model:        mockAssistantModelRoute,
		InputTokens:  0,
		OutputTokens: 0,
	}, nil
}

func (mockAssistantStreamingProvider) TestConnection(ctx context.Context) error { return nil }

func (mockAssistantStreamingProvider) Name() string { return "pulse-mock" }

func (mockAssistantStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{{
		ID:       mockAssistantModelRoute,
		Name:     "Pulse mock Assistant",
		Provider: "pulse",
	}}, nil
}

func (mockAssistantStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if callback == nil {
		return nil
	}
	prompt := ""
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}
	for _, chunk := range mockAssistantResponseChunks(prompt) {
		callback(providers.StreamEvent{
			Type: "content",
			Data: providers.ContentEvent{Text: chunk},
		})
	}
	callback(providers.StreamEvent{
		Type: "done",
		Data: providers.DoneEvent{},
	})
	return nil
}

func (mockAssistantStreamingProvider) SupportsThinking(model string) bool { return false }
