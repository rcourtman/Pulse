package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

// AgenticLoop handles the tool-calling loop with streaming
type AgenticLoop struct {
	provider     providers.StreamingProvider
	executor     *tools.PulseToolExecutor
	tools        []providers.Tool
	systemPrompt string
	maxTurns     int

	// State for ongoing executions
	mu             sync.Mutex
	aborted        map[string]bool                  // sessionID -> aborted
	pendingQs      map[string]chan []QuestionAnswer // questionID -> answer channel
	autonomousMode bool                             // When true, don't wait for approvals (for investigations)
}

// NewAgenticLoop creates a new agentic loop
func NewAgenticLoop(provider providers.StreamingProvider, executor *tools.PulseToolExecutor, systemPrompt string) *AgenticLoop {
	// Convert MCP tools to provider format
	mcpTools := executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	return &AgenticLoop{
		provider:     provider,
		executor:     executor,
		tools:        providerTools,
		systemPrompt: systemPrompt,
		maxTurns:     MaxAgenticTurns,
		aborted:      make(map[string]bool),
		pendingQs:    make(map[string]chan []QuestionAnswer),
	}
}

// UpdateTools refreshes the tool list from the executor
func (a *AgenticLoop) UpdateTools() {
	mcpTools := a.executor.ListTools()
	a.tools = ConvertMCPToolsToProvider(mcpTools)
}

// Execute runs the agentic loop with streaming
func (a *AgenticLoop) Execute(ctx context.Context, sessionID string, messages []Message, callback StreamCallback) ([]Message, error) {
	return a.executeWithTools(ctx, sessionID, messages, a.tools, callback)
}

// ExecuteWithTools runs the agentic loop with a filtered tool set
func (a *AgenticLoop) ExecuteWithTools(ctx context.Context, sessionID string, messages []Message, tools []providers.Tool, callback StreamCallback) ([]Message, error) {
	return a.executeWithTools(ctx, sessionID, messages, tools, callback)
}

func (a *AgenticLoop) executeWithTools(ctx context.Context, sessionID string, messages []Message, tools []providers.Tool, callback StreamCallback) ([]Message, error) {
	// Track this session for potential abort
	a.mu.Lock()
	a.aborted[sessionID] = false
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.aborted, sessionID)
		a.mu.Unlock()
	}()

	// Convert our messages to provider format
	messagesForModel := pruneMessagesForModel(messages)
	providerMessages := convertToProviderMessages(messagesForModel)

	if tools == nil {
		tools = a.tools
	}

	var resultMessages []Message
	turn := 0

	for turn < a.maxTurns {
		// Check if aborted
		a.mu.Lock()
		if a.aborted[sessionID] {
			a.mu.Unlock()
			return resultMessages, fmt.Errorf("session aborted")
		}
		a.mu.Unlock()

		// Check context
		select {
		case <-ctx.Done():
			return resultMessages, ctx.Err()
		default:
		}

		log.Debug().
			Int("turn", turn).
			Int("messages", len(providerMessages)).
			Int("tools", len(tools)).
			Str("session_id", sessionID).
			Msg("[AgenticLoop] Starting turn")

		// Build the request
		req := providers.ChatRequest{
			Messages: providerMessages,
			System:   a.systemPrompt,
			Tools:    tools,
		}

		// Collect streaming response
		var contentBuilder strings.Builder
		var thinkingBuilder strings.Builder
		var toolCalls []providers.ToolCall

		log.Debug().
			Str("session_id", sessionID).
			Int("system_prompt_len", len(a.systemPrompt)).
			Msg("[AgenticLoop] Calling provider.ChatStream")

		err := a.provider.ChatStream(ctx, req, func(event providers.StreamEvent) {
			switch event.Type {
			case "content":
				if data, ok := event.Data.(providers.ContentEvent); ok {
					contentBuilder.WriteString(data.Text)
					// Forward to callback - send ContentData struct
					jsonData, _ := json.Marshal(ContentData{Text: data.Text})
					callback(StreamEvent{Type: "content", Data: jsonData})
				}

			case "thinking":
				if data, ok := event.Data.(providers.ThinkingEvent); ok {
					thinkingBuilder.WriteString(data.Text)
					// Forward to callback - send ThinkingData struct
					jsonData, _ := json.Marshal(ThinkingData{Text: data.Text})
					callback(StreamEvent{Type: "thinking", Data: jsonData})
				}

			case "tool_start":
				if data, ok := event.Data.(providers.ToolStartEvent); ok {
					// Convert input to JSON string for frontend
					inputStr := "{}"
					if data.Input != nil {
						if inputBytes, err := json.Marshal(data.Input); err == nil {
							inputStr = string(inputBytes)
						}
					}
					jsonData, _ := json.Marshal(ToolStartData{
						ID:    data.ID,
						Name:  data.Name,
						Input: inputStr,
					})
					callback(StreamEvent{Type: "tool_start", Data: jsonData})
				}

			case "done":
				if data, ok := event.Data.(providers.DoneEvent); ok {
					toolCalls = data.ToolCalls
				}

			case "error":
				if data, ok := event.Data.(providers.ErrorEvent); ok {
					jsonData, _ := json.Marshal(ErrorData{Message: data.Message})
					callback(StreamEvent{Type: "error", Data: jsonData})
				}
			}
		})

		log.Debug().
			Str("session_id", sessionID).
			Err(err).
			Int("content_len", contentBuilder.Len()).
			Int("tool_calls", len(toolCalls)).
			Msg("[AgenticLoop] provider.ChatStream returned")

		if err != nil {
			log.Error().
				Err(err).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Provider error")
			return resultMessages, fmt.Errorf("provider error: %w", err)
		}

		// Create assistant message
		assistantMsg := Message{
			ID:               uuid.New().String(),
			Role:             "assistant",
			Content:          contentBuilder.String(),
			ReasoningContent: thinkingBuilder.String(),
			Timestamp:        time.Now(),
		}

		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = make([]ToolCall, len(toolCalls))
			for i, tc := range toolCalls {
				assistantMsg.ToolCalls[i] = ToolCall{
					ID:               tc.ID,
					Name:             tc.Name,
					Input:            tc.Input,
					ThoughtSignature: tc.ThoughtSignature,
				}
			}
		}

		resultMessages = append(resultMessages, assistantMsg)

		// Convert to provider format for next turn
		providerAssistant := providers.Message{
			Role:             "assistant",
			Content:          assistantMsg.Content,
			ReasoningContent: assistantMsg.ReasoningContent,
		}
		for _, tc := range toolCalls {
			providerAssistant.ToolCalls = append(providerAssistant.ToolCalls, providers.ToolCall{
				ID:               tc.ID,
				Name:             tc.Name,
				Input:            tc.Input,
				ThoughtSignature: tc.ThoughtSignature,
			})
		}
		providerMessages = append(providerMessages, providerAssistant)

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			log.Debug().Msg("Agentic loop complete - no tool calls")
			return resultMessages, nil
		}

		// Execute tool calls
		for _, tc := range toolCalls {
			// Check for abort
			a.mu.Lock()
			if a.aborted[sessionID] {
				a.mu.Unlock()
				return resultMessages, fmt.Errorf("session aborted")
			}
			a.mu.Unlock()

			log.Debug().
				Str("tool", tc.Name).
				Str("id", tc.ID).
				Msg("Executing tool")

			// Execute the tool
			result, err := a.executor.ExecuteTool(ctx, tc.Name, tc.Input)

			var resultText string
			var isError bool

			if err != nil {
				resultText = fmt.Sprintf("Error: %v", err)
				isError = true
			} else {
				resultText = FormatToolResult(result)
				isError = result.IsError
			}

			// Check if this is an approval request
			if strings.HasPrefix(resultText, "APPROVAL_REQUIRED:") {
				// Parse approval request
				approvalJSON := strings.TrimPrefix(resultText, "APPROVAL_REQUIRED:")
				approvalJSON = strings.TrimSpace(approvalJSON)

				var approvalData struct {
					ApprovalID  string `json:"approval_id"`
					Command     string `json:"command"`
					Risk        string `json:"risk"`
					Description string `json:"description"`
				}
				if err := json.Unmarshal([]byte(approvalJSON), &approvalData); err != nil {
					log.Error().Err(err).Str("data", approvalJSON).Msg("Failed to parse approval request")
					resultText = "Error: failed to parse approval request"
					isError = true
				} else {
					// Send approval_needed event
					jsonData, _ := json.Marshal(ApprovalNeededData{
						ApprovalID:  approvalData.ApprovalID,
						ToolID:      tc.ID,
						ToolName:    tc.Name,
						Command:     approvalData.Command,
						RunOnHost:   true, // Control commands run on host
						TargetHost:  "",   // Will be filled from approval context if available
						Risk:        approvalData.Risk,
						Description: approvalData.Description,
					})
					callback(StreamEvent{Type: "approval_needed", Data: jsonData})

					// In autonomous mode (investigations), don't wait for approval.
					// Instead, return with approval info so the orchestrator can queue it.
					a.mu.Lock()
					isAutonomous := a.autonomousMode
					a.mu.Unlock()

					if isAutonomous {
						log.Debug().
							Str("approval_id", approvalData.ApprovalID).
							Str("command", approvalData.Command).
							Msg("[AgenticLoop] Autonomous mode: returning approval request without waiting")
						// Return special message indicating fix is queued for approval
						resultText = fmt.Sprintf("FIX_QUEUED: This action requires user approval. The fix has been queued for review. Approval ID: %s, Command: %s", approvalData.ApprovalID, approvalData.Command)
						isError = false
					} else {
						// Wait for approval decision (poll with timeout)
						store := approval.GetStore()
						if store != nil {
							approvalCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
							decision, waitErr := waitForApprovalDecision(approvalCtx, store, approvalData.ApprovalID)
							cancel()

							if waitErr != nil {
								resultText = fmt.Sprintf("Approval timeout or error: %v", waitErr)
								isError = true
							} else if decision.Status == approval.StatusApproved {
								// Re-execute the tool with approval granted
								// Add approval_id to input so tool knows this is pre-approved
								inputWithApproval := make(map[string]interface{})
								for k, v := range tc.Input {
									inputWithApproval[k] = v
								}
								inputWithApproval["_approval_id"] = approvalData.ApprovalID
								result, err = a.executor.ExecuteTool(ctx, tc.Name, inputWithApproval)
								if err != nil {
									resultText = fmt.Sprintf("Error after approval: %v", err)
									isError = true
								} else {
									resultText = FormatToolResult(result)
									isError = result.IsError
								}
							} else {
								resultText = fmt.Sprintf("Command denied: %s", decision.DenyReason)
								isError = false
							}
						} else {
							resultText = "Approval system not available"
							isError = true
						}
					}
				}
			}

			modelResultText := truncateToolResultForModel(resultText)

			// Send tool_end event
			// Convert input to JSON string for frontend display
			inputStr := ""
			if tc.Input != nil {
				if inputBytes, err := json.Marshal(tc.Input); err == nil {
					inputStr = string(inputBytes)
				}
			}
			jsonData, _ := json.Marshal(ToolEndData{
				ID:      tc.ID,
				Name:    tc.Name,
				Input:   inputStr,
				Output:  resultText,
				Success: !isError,
			})
			callback(StreamEvent{Type: "tool_end", Data: jsonData})

			// Create tool result message
			toolResultMsg := Message{
				ID:        uuid.New().String(),
				Role:      "user", // Tool results are sent as user messages
				Timestamp: time.Now(),
				ToolResult: &ToolResult{
					ToolUseID: tc.ID,
					Content:   resultText,
					IsError:   isError,
				},
			}
			resultMessages = append(resultMessages, toolResultMsg)

			// Add to provider messages for next turn
			providerMessages = append(providerMessages, providers.Message{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: tc.ID,
					Content:   modelResultText,
					IsError:   isError,
				},
			})
		}

		turn++
	}

	log.Warn().Int("max_turns", a.maxTurns).Msg("Agentic loop hit max turns limit")
	return resultMessages, nil
}

// Abort aborts an ongoing session
func (a *AgenticLoop) Abort(sessionID string) {
	a.mu.Lock()
	a.aborted[sessionID] = true
	a.mu.Unlock()
}

// SetAutonomousMode sets whether the loop is in autonomous mode (for investigations).
// When enabled, approval requests don't block waiting for user input.
func (a *AgenticLoop) SetAutonomousMode(enabled bool) {
	a.mu.Lock()
	a.autonomousMode = enabled
	a.mu.Unlock()
}

// AnswerQuestion provides an answer to a pending question
func (a *AgenticLoop) AnswerQuestion(questionID string, answers []QuestionAnswer) error {
	a.mu.Lock()
	ch, exists := a.pendingQs[questionID]
	a.mu.Unlock()

	if !exists {
		return fmt.Errorf("no pending question with ID: %s", questionID)
	}

	// Non-blocking send
	select {
	case ch <- answers:
		return nil
	default:
		return fmt.Errorf("question already answered: %s", questionID)
	}
}

// waitForApprovalDecision polls for an approval decision
func waitForApprovalDecision(ctx context.Context, store *approval.Store, approvalID string) (*approval.ApprovalRequest, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, ok := store.GetApproval(approvalID)
			if !ok {
				return nil, fmt.Errorf("approval request not found: %s", approvalID)
			}
			if req.Status != approval.StatusPending {
				return req, nil
			}
		}
	}
}

func pruneMessagesForModel(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	if StatelessContext {
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			if msg.Role == "user" && msg.ToolResult == nil && msg.Content != "" {
				return []Message{msg}
			}
		}
		return []Message{messages[len(messages)-1]}
	}

	if MaxContextMessagesLimit <= 0 || len(messages) <= MaxContextMessagesLimit {
		return messages
	}

	start := len(messages) - MaxContextMessagesLimit
	pruned := messages[start:]
	for len(pruned) > 0 && pruned[0].ToolResult != nil {
		pruned = pruned[1:]
	}

	return pruned
}

func truncateToolResultForModel(text string) string {
	if MaxToolResultCharsLimit <= 0 || len(text) <= MaxToolResultCharsLimit {
		return text
	}

	truncated := text[:MaxToolResultCharsLimit]
	return fmt.Sprintf("%s\n...[truncated %d chars]...", truncated, len(text)-MaxToolResultCharsLimit)
}

// convertToProviderMessages converts our messages to provider format
func convertToProviderMessages(messages []Message) []providers.Message {
	result := make([]providers.Message, 0, len(messages))

	for _, m := range messages {
		pm := providers.Message{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
		}

		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				pm.ToolCalls = append(pm.ToolCalls, providers.ToolCall{
					ID:               tc.ID,
					Name:             tc.Name,
					Input:            tc.Input,
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
		}

		if m.ToolResult != nil {
			pm.ToolResult = &providers.ToolResult{
				ToolUseID: m.ToolResult.ToolUseID,
				Content:   truncateToolResultForModel(m.ToolResult.Content),
				IsError:   m.ToolResult.IsError,
			}
		}

		result = append(result, pm)
	}

	return result
}
