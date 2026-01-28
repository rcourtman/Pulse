package api

import (
	"context"
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
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
		Prompt:       req.Prompt,
		SystemPrompt: req.SystemPrompt,
		SessionID:    req.SessionID,
		UseCase:      req.UseCase,
	}, adaptCallback(callback))
	if err != nil {
		return nil, err
	}
	return &ai.PatrolStreamResponse{
		Content:      resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
}

func (a *chatServiceAdapter) GetMessages(ctx context.Context, sessionID string) ([]ai.ChatMessage, error) {
	messages, err := a.svc.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]ai.ChatMessage, len(messages))
	for i, m := range messages {
		result[i] = ai.ChatMessage{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp,
		}
	}
	return result, nil
}

func (a *chatServiceAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	return a.svc.DeleteSession(ctx, sessionID)
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
