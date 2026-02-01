package investigation

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rs/zerolog/log"
)

// ChatServiceAdapter adapts the chat.Service to the ChatService interface
type ChatServiceAdapter struct {
	service *chat.Service
}

// NewChatServiceAdapter creates a new chat service adapter
func NewChatServiceAdapter(service *chat.Service) *ChatServiceAdapter {
	return &ChatServiceAdapter{service: service}
}

// CreateSession creates a new chat session
func (a *ChatServiceAdapter) CreateSession(ctx context.Context) (*Session, error) {
	session, err := a.service.CreateSession(ctx)
	if err != nil {
		return nil, err
	}
	return &Session{ID: session.ID}, nil
}

// ExecuteStream sends a prompt and streams the response
func (a *ChatServiceAdapter) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	log.Debug().
		Str("session_id", req.SessionID).
		Int("prompt_len", len(req.Prompt)).
		Bool("service_running", a.service != nil && a.service.IsRunning()).
		Msg("[ChatAdapter] ExecuteStream called")

	if a.service == nil {
		log.Error().Msg("[ChatAdapter] Service is nil!")
		return fmt.Errorf("chat service is nil")
	}

	if !a.service.IsRunning() {
		log.Error().Msg("[ChatAdapter] Service is not running!")
		return fmt.Errorf("chat service is not running")
	}

	chatReq := chat.ExecuteRequest{
		Prompt:         req.Prompt,
		SessionID:      req.SessionID,
		MaxTurns:       req.MaxTurns,
		AutonomousMode: req.AutonomousMode,
	}

	log.Debug().
		Str("session_id", req.SessionID).
		Msg("[ChatAdapter] Calling chat.Service.ExecuteStream")

	err := a.service.ExecuteStream(ctx, chatReq, func(event chat.StreamEvent) {
		log.Debug().
			Str("session_id", req.SessionID).
			Str("event_type", event.Type).
			Msg("[ChatAdapter] Received stream event")
		callback(StreamEvent{
			Type: event.Type,
			Data: event.Data,
		})
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("session_id", req.SessionID).
			Msg("[ChatAdapter] ExecuteStream returned error")
	} else {
		log.Debug().
			Str("session_id", req.SessionID).
			Msg("[ChatAdapter] ExecuteStream completed successfully")
	}

	return err
}

// GetMessages retrieves messages from a session
func (a *ChatServiceAdapter) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	chatMessages, err := a.service.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, len(chatMessages))
	for i, msg := range chatMessages {
		m := Message{
			ID:               msg.ID,
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			Timestamp:        msg.Timestamp,
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, ToolCallInfo{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			})
		}
		if msg.ToolResult != nil {
			m.ToolResult = &ToolResultInfo{
				ToolUseID: msg.ToolResult.ToolUseID,
				Content:   msg.ToolResult.Content,
				IsError:   msg.ToolResult.IsError,
			}
		}
		messages[i] = m
	}
	return messages, nil
}

// DeleteSession deletes a chat session
func (a *ChatServiceAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	return a.service.DeleteSession(ctx, sessionID)
}

// ListAvailableTools returns tool names available for the given prompt.
func (a *ChatServiceAdapter) ListAvailableTools(ctx context.Context, prompt string) []string {
	if a.service == nil {
		return nil
	}
	return a.service.ListAvailableTools(ctx, prompt)
}

// IsRunning checks if the chat service is running
func (a *ChatServiceAdapter) IsRunning() bool {
	return a.service != nil && a.service.IsRunning()
}

// SetAutonomousMode enables or disables autonomous mode for investigations
// When enabled, read-only commands can be auto-approved without user confirmation
func (a *ChatServiceAdapter) SetAutonomousMode(enabled bool) {
	if a.service != nil {
		a.service.SetAutonomousMode(enabled)
	}
}

// ExecuteCommand executes a command directly via the chat service (bypasses LLM)
// This is used for auto-executing fixes in full autonomy mode
func (a *ChatServiceAdapter) ExecuteCommand(ctx context.Context, command, targetHost string) (output string, exitCode int, err error) {
	if a.service == nil {
		return "", -1, fmt.Errorf("chat service not available")
	}
	return a.service.ExecuteCommand(ctx, command, targetHost)
}
