package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client communicates with the OpenCode server
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OpenCode client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: newHTTPClient(5 * time.Minute),
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// Session represents an OpenCode chat session
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Message represents a message in a session
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // "user" or "assistant"
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"toolCalls,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Input  map[string]interface{} `json:"input"`
	Output string                 `json:"output,omitempty"`
}

// PromptRequest is the request body for sending a prompt
type PromptRequest struct {
	Prompt       string                 `json:"prompt"`
	SessionID    string                 `json:"sessionId,omitempty"`
	Model        string                 `json:"model,omitempty"`
	SystemPrompt string                 `json:"systemPrompt,omitempty"`
	Tools        []string               `json:"tools,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// PromptResponse is the response from a prompt request
type PromptResponse struct {
	SessionID string    `json:"sessionId"`
	Message   Message   `json:"message"`
	Usage     Usage     `json:"usage"`
	Model     ModelInfo `json:"model"`
}

// Usage contains token usage information
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// ModelInfo contains model information
type ModelInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

// StreamEvent represents an event from the SSE stream
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ContentEvent is emitted when content is generated
type ContentEvent struct {
	Content string `json:"content"`
	Delta   string `json:"delta,omitempty"`
}

// ToolUseEvent is emitted when a tool is invoked
type ToolUseEvent struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResultEvent is emitted when a tool completes
type ToolResultEvent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// CompleteEvent is emitted when the response is complete
type CompleteEvent struct {
	SessionID string    `json:"sessionId"`
	Message   Message   `json:"message"`
	Usage     Usage     `json:"usage"`
	Model     ModelInfo `json:"model"`
}

// ErrorEvent is emitted when an error occurs
type ErrorEvent struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Health checks the OpenCode server health
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/global/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// ListSessions returns all sessions
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list sessions failed: status %d", resp.StatusCode)
	}

	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// CreateSession creates a new session
func (c *Client) CreateSession(ctx context.Context) (*Session, error) {
	// OpenCode requires a JSON body, even if empty
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session", strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create session failed: status %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

// DeleteSession deletes a session
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/session/"+sessionID, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete session failed: status %d", resp.StatusCode)
	}

	return nil
}

// Prompt sends a prompt and returns the response (non-streaming)
func (c *Client) Prompt(ctx context.Context, req PromptRequest) (*PromptResponse, error) {
	// Ensure we have a session
	sessionID := req.SessionID
	if sessionID == "" {
		session, err := c.CreateSession(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		sessionID = session.ID
	}

	// OpenCode API uses /session/{id}/message with parts array format
	messageReq := map[string]interface{}{
		"parts": []map[string]interface{}{
			{"type": "text", "text": req.Prompt},
		},
	}
	if req.Model != "" {
		messageReq["model"] = req.Model
	}

	body, err := json.Marshal(messageReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session/"+sessionID+"/message", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prompt failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the OpenCode response format which has info and parts fields
	var rawResponse struct {
		Info struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionID"`
			Role      string `json:"role"`
		} `json:"info"`
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, err
	}

	// Extract text content from parts
	var contentParts []string
	for _, part := range rawResponse.Parts {
		if part.Type == "text" && part.Text != "" {
			contentParts = append(contentParts, part.Text)
		}
	}

	var result PromptResponse
	result.SessionID = sessionID
	result.Message = Message{
		ID:      rawResponse.Info.ID,
		Role:    rawResponse.Info.Role,
		Content: strings.Join(contentParts, ""),
	}

	return &result, nil
}

// PromptStream sends a prompt and streams the response via SSE
func (c *Client) PromptStream(ctx context.Context, req PromptRequest) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent, 100)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		// Ensure we have a valid OpenCode session
		// OpenCode requires session IDs to start with "ses" - if the user passed
		// their own session ID (e.g., from the frontend), we need to create a new
		// OpenCode session anyway
		sessionID := req.SessionID
		if sessionID == "" || !strings.HasPrefix(sessionID, "ses") {
			log.Debug().
				Str("provided_session_id", req.SessionID).
				Msg("OpenCode: Creating new session (no valid session ID provided)")
			session, err := c.CreateSession(ctx)
			if err != nil {
				errs <- fmt.Errorf("failed to create session: %w", err)
				return
			}
			sessionID = session.ID
			log.Debug().Str("new_session_id", sessionID).Msg("OpenCode: Created new session")
		} else {
			log.Debug().Str("session_id", sessionID).Msg("OpenCode: Using existing session")
		}

		// OpenCode uses /session/{id}/prompt_async for async/streaming with parts array format
		messageReq := map[string]interface{}{
			"parts": []map[string]interface{}{
				{"type": "text", "text": req.Prompt},
			},
		}
		if req.Model != "" {
			messageReq["model"] = req.Model
		}

		body, err := json.Marshal(messageReq)
		if err != nil {
			errs <- err
			return
		}

		// IMPORTANT: Subscribe to event stream FIRST, before sending prompt
		// This prevents a race condition where fast responses (like from Gemini)
		// complete before we've established the event subscription
		eventReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/global/event", nil)
		if err != nil {
			errs <- err
			return
		}
		eventReq.Header.Set("Accept", "text/event-stream")

		// Use a client without timeout for streaming
		streamClient := &http.Client{}
		eventResp, err := streamClient.Do(eventReq)
		if err != nil {
			errs <- err
			return
		}
		defer eventResp.Body.Close()

		if eventResp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(eventResp.Body)
			errs <- fmt.Errorf("event stream failed: status %d, body: %s", eventResp.StatusCode, string(bodyBytes))
			return
		}

		// NOW send the async prompt request (after event stream is established)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session/"+sessionID+"/prompt_async", bytes.NewReader(body))
		if err != nil {
			errs <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errs <- err
			return
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
			errs <- fmt.Errorf("prompt_async failed: status %d", resp.StatusCode)
			return
		}

		// Parse SSE stream - OpenCode sends events in format:
		// data: {"directory":"...","payload":{"type":"event.type","properties":{...}}}
		// Events contain sessionID in properties - we must filter to only process our session
		scanner := bufio.NewScanner(eventResp.Body)
		// SSE lines can be very long (>64KB for some events), increase buffer
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var contentBuffer strings.Builder
		var receivedContent bool // Track if we've received actual content from our prompt

		log.Debug().Str("sessionID", sessionID).Msg("OpenCode: Starting to read event stream")

		// Start a parallel poll goroutine as safety net for fast models
		// If SSE doesn't receive content within 5 seconds, poll messages directly
		pollDone := make(chan struct{})
		go func() {
			defer close(pollDone)
			time.Sleep(5 * time.Second)

			// Check if we've already received content from SSE
			if receivedContent {
				return
			}

			log.Debug().Msg("OpenCode: SSE timeout, falling back to message polling")

			// Poll up to 10 times with 1 second intervals
			for i := 0; i < 10; i++ {
				if receivedContent {
					return // SSE finally delivered
				}

				messages, err := c.GetMessages(ctx, sessionID)
				if err != nil {
					log.Warn().Err(err).Msg("OpenCode: Poll fallback failed")
					time.Sleep(1 * time.Second)
					continue
				}

				// Look for assistant message
				for _, msg := range messages {
					if msg.Role == "assistant" && msg.Content != "" {
						// Found response! Send as content event
						log.Info().Str("content", msg.Content[:min(50, len(msg.Content))]).Msg("OpenCode: Got response via polling")
						contentData, _ := json.Marshal(msg.Content)
						events <- StreamEvent{Type: "content", Data: contentData}
						events <- StreamEvent{Type: "done", Data: nil}
						receivedContent = true
						return
					}
				}

				time.Sleep(1 * time.Second)
			}

			// Timeout - send error
			log.Warn().Msg("OpenCode: Polling timeout")
			events <- StreamEvent{Type: "done", Data: nil}
		}()

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
			}

			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}

			// Parse the OpenCode event envelope
			var envelope struct {
				Payload struct {
					Type       string          `json:"type"`
					Properties json.RawMessage `json:"properties"`
				} `json:"payload"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err != nil {
				continue
			}

			eventType := envelope.Payload.Type

			// Extract session ID from properties to filter events
			var baseProps struct {
				SessionID string `json:"sessionID"`
				Part      struct {
					SessionID string `json:"sessionID"`
				} `json:"part"`
			}
			json.Unmarshal(envelope.Payload.Properties, &baseProps)
			eventSessionID := baseProps.SessionID
			if eventSessionID == "" {
				eventSessionID = baseProps.Part.SessionID
			}

			log.Debug().
				Str("eventType", eventType).
				Str("eventSessionID", eventSessionID).
				Str("ourSessionID", sessionID).
				Msg("OpenCode: Received event")

			// Skip events from other sessions
			if eventSessionID != "" && eventSessionID != sessionID {
				log.Debug().Msg("OpenCode: Skipping event from other session")
				continue
			}

			switch eventType {
			case "message.part.updated":
				// Content streaming - extract delta or text
				var props struct {
					Part struct {
						Type   string `json:"type"`
						Text   string `json:"text"`
						Reason string `json:"reason"` // For step-finish: "stop" or "tool-calls"
						Tool   string `json:"tool"`   // For tool parts
						State  struct {
							Status string `json:"status"`
							Output string `json:"output"`
						} `json:"state"`
						Time struct {
							End int64 `json:"end"`
						} `json:"time"`
					} `json:"part"`
					Delta string `json:"delta"`
				}
				if err := json.Unmarshal(envelope.Payload.Properties, &props); err != nil {
					continue
				}
				log.Debug().Str("partType", props.Part.Type).Str("delta", props.Delta).Str("reason", props.Part.Reason).Msg("OpenCode: Processing message part")
				if props.Part.Type == "text" || props.Part.Type == "reasoning" {
					// Stream both text and reasoning content
					if props.Delta != "" {
						contentBuffer.WriteString(props.Delta)
						receivedContent = true // Mark that we've received actual content
						// Frontend expects just the delta string as data
						deltaData, _ := json.Marshal(props.Delta)
						log.Debug().Str("type", props.Part.Type).Str("delta", props.Delta).Msg("OpenCode: Sending content event")
						events <- StreamEvent{Type: "content", Data: deltaData}
					}
				} else if props.Part.Type == "tool" {
					// Tool execution - translate to frontend format
					status := props.Part.State.Status
					log.Debug().Str("tool", props.Part.Tool).Str("status", status).Msg("OpenCode: Tool execution")

					if status == "pending" || status == "running" {
						// Tool starting - send tool_start event
						toolInfo := map[string]interface{}{
							"name":  props.Part.Tool,
							"input": props.Part.Tool, // Use tool name as input display
						}
						toolData, _ := json.Marshal(toolInfo)
						events <- StreamEvent{Type: "tool_start", Data: toolData}
					} else if status == "completed" {
						// Tool completed - send tool_end event
						toolInfo := map[string]interface{}{
							"name":    props.Part.Tool,
							"input":   props.Part.Tool,
							"output":  props.Part.State.Output,
							"success": true,
						}
						toolData, _ := json.Marshal(toolInfo)
						events <- StreamEvent{Type: "tool_end", Data: toolData}
					}
				} else if props.Part.Type == "step-finish" {
					// Step completed - only terminate if reason is "stop" (not "tool-calls")
					log.Debug().Str("reason", props.Part.Reason).Msg("OpenCode: Step finished")
					if props.Part.Reason == "stop" || props.Part.Reason == "" {
						// Final response complete
						events <- StreamEvent{Type: "done", Data: nil}
						return
					}
					// If reason is "tool-calls", continue waiting for tool results and next response
				}

			case "session.idle":
				// Session done processing - but only if we've received content
				// Otherwise this is a stale event from before our prompt was processed
				if receivedContent {
					events <- StreamEvent{Type: "done", Data: nil}
					return
				}
				log.Debug().Msg("OpenCode: Ignoring session.idle (no content received yet)")

			case "session.status":
				// Check if status is idle - but only finish if we've received content
				var props struct {
					Status struct {
						Type string `json:"type"`
					} `json:"status"`
				}
				if err := json.Unmarshal(envelope.Payload.Properties, &props); err == nil {
					if props.Status.Type == "idle" && receivedContent {
						events <- StreamEvent{Type: "done", Data: nil}
						return
					}
				}

			case "question.asked":
				// OpenCode is asking the user a question
				var props struct {
					ID        string `json:"id"`
					Questions []struct {
						ID       string `json:"id"`
						Type     string `json:"type"` // "text" or "select"
						Question string `json:"question"`
						Options  []struct {
							Label string `json:"label"`
							Value string `json:"value"`
						} `json:"options,omitempty"`
					} `json:"questions"`
				}
				if err := json.Unmarshal(envelope.Payload.Properties, &props); err != nil {
					log.Warn().Err(err).Msg("OpenCode: Failed to parse question.asked event")
					continue
				}
				log.Debug().Str("questionID", props.ID).Int("count", len(props.Questions)).Msg("OpenCode: Question asked")

				// Send question event to frontend
				questionData, _ := json.Marshal(map[string]interface{}{
					"question_id": props.ID,
					"questions":   props.Questions,
					"session_id":  sessionID,
				})
				events <- StreamEvent{Type: "question", Data: questionData}
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- err
		}
	}()

	return events, errs
}

// GetMessages returns all messages in a session
func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/session/"+sessionID+"/message", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get messages failed: status %d", resp.StatusCode)
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}

	return messages, nil
}

// AbortSession aborts the current operation in a session
func (c *Client) AbortSession(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/session/"+sessionID+"/abort", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("abort session failed: status %d", resp.StatusCode)
	}

	return nil
}

// AnswerQuestion answers a pending question from OpenCode
func (c *Client) AnswerQuestion(ctx context.Context, questionID string, answers []QuestionAnswer) error {
	// Build answers in OpenCode format
	answerReq := map[string]interface{}{
		"answers": answers,
	}

	body, err := json.Marshal(answerReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/question/"+questionID+"/answer", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("answer question failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// QuestionAnswer represents an answer to a question
type QuestionAnswer struct {
	ID    string `json:"id"`    // Question ID this answer is for
	Value string `json:"value"` // The answer value (text or selected option)
}

// ListModels returns available models
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/config/providers", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list models failed: status %d", resp.StatusCode)
	}

	var providers map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, err
	}

	// Transform provider response to model list
	var models []ModelInfo
	for provider := range providers {
		// OpenCode returns provider configs, we'll need to map to actual models
		models = append(models, ModelInfo{
			Provider: provider,
		})
	}

	return models, nil
}
