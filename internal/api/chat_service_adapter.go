package api

import (
	"context"
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// chatServiceAdapter wraps chat.Service to implement ai.ChatServiceProvider.
// This bridges the chat package (concrete) to the ai package (interface) without
// creating an import cycle.
type chatServiceAdapter struct {
	svc *chat.Service
}

func (a *chatServiceAdapter) CreateSession(ctx context.Context) (*ai.ChatSession, error) {
	session, err := a.svc.CreateSession(ctx)
	if err != nil {
		return nil, err
	}
	return &ai.ChatSession{ID: session.ID}, nil
}

func (a *chatServiceAdapter) ExecuteStream(ctx context.Context, req ai.ChatExecuteRequest, callback ai.ChatStreamCallback) error {
	return a.svc.ExecuteStream(ctx, chat.ExecuteRequest{
		Prompt:    req.Prompt,
		SessionID: req.SessionID,
	}, adaptCallback(callback))
}

func (a *chatServiceAdapter) ExecutePatrolStream(ctx context.Context, req ai.PatrolExecuteRequest, callback ai.ChatStreamCallback) (*ai.PatrolStreamResponse, error) {
	resp, err := a.svc.ExecutePatrolStream(ctx, chat.PatrolRequest{
		Prompt:         req.Prompt,
		SystemPrompt:   req.SystemPrompt,
		SessionID:      req.SessionID,
		UseCase:        req.UseCase,
		MaxTurns:       req.MaxTurns,
		MaxTotalTokens: req.MaxTotalTokens,
	}, adaptCallback(callback))
	if err != nil {
		return nil, err
	}
	return &ai.PatrolStreamResponse{
		Content:      resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		StopReason:   resp.StopReason,
	}, nil
}

func (a *chatServiceAdapter) GetMessages(ctx context.Context, sessionID string) ([]ai.ChatMessage, error) {
	messages, err := a.svc.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]ai.ChatMessage, len(messages))
	for i, m := range messages {
		msg := ai.ChatMessage{
			ID:               m.ID,
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Timestamp:        m.Timestamp,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ai.ChatToolCall{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			})
		}
		if m.ToolResult != nil {
			msg.ToolResult = &ai.ChatToolResult{
				ToolUseID: m.ToolResult.ToolUseID,
				Content:   m.ToolResult.Content,
				IsError:   m.ToolResult.IsError,
			}
		}
		result[i] = msg
	}
	return result, nil
}

func (a *chatServiceAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	return a.svc.DeleteSession(ctx, sessionID)
}

func (a *chatServiceAdapter) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return a.svc.Restart(ctx, cfg)
}

// GetExecutor exposes the underlying chat service's tool executor so that
// patrol can set the finding creator. This satisfies the
// chatServiceExecutorAccessor interface in the ai package.
func (a *chatServiceAdapter) GetExecutor() *tools.PulseToolExecutor {
	return a.svc.GetExecutor()
}

// adaptCallback converts an ai.ChatStreamCallback to a chat.StreamCallback.
// The ai package uses []byte for event data, while the chat package uses json.RawMessage.
func adaptCallback(callback ai.ChatStreamCallback) chat.StreamCallback {
	return func(event chat.StreamEvent) {
		callback(ai.ChatStreamEvent{
			Type: event.Type,
			Data: json.RawMessage(event.Data),
		})
	}
}
