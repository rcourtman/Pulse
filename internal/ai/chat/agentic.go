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

// parallelToolResult holds the result of a single tool execution during
// parallel tool execution in the agentic loop. Defined at package level
// because the executeWithTools function's `tools` parameter shadows the
// tools package import, preventing inline type references.
type parallelToolResult struct {
	Result tools.CallToolResult
	Err    error
}

// AgenticLoop handles the tool-calling loop with streaming
type AgenticLoop struct {
	provider         providers.StreamingProvider
	executor         *tools.PulseToolExecutor
	tools            []providers.Tool
	baseSystemPrompt string // Base prompt without mode context
	maxTurns         int

	// Provider info for telemetry (e.g., "anthropic", "claude-3-sonnet")
	providerName string
	modelName    string

	// Token accumulation across all turns
	totalInputTokens  int
	totalOutputTokens int

	// State for ongoing executions
	mu             sync.Mutex
	aborted        map[string]bool                  // sessionID -> aborted
	pendingQs      map[string]chan []QuestionAnswer // questionID -> answer channel
	autonomousMode bool                             // When true, don't wait for approvals (for investigations)

	// Per-session FSMs for workflow enforcement (set before each execution)
	sessionFSM *SessionFSM

	// Knowledge accumulator for fact extraction across turns
	knowledgeAccumulator *KnowledgeAccumulator

	// Budget checker called after each turn to enforce token spending limits
	budgetChecker func() error
}

// NewAgenticLoop creates a new agentic loop
func NewAgenticLoop(provider providers.StreamingProvider, executor *tools.PulseToolExecutor, systemPrompt string) *AgenticLoop {
	// Convert MCP tools to provider format
	mcpTools := executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	return &AgenticLoop{
		provider:         provider,
		executor:         executor,
		tools:            providerTools,
		baseSystemPrompt: systemPrompt,
		maxTurns:         MaxAgenticTurns,
		aborted:          make(map[string]bool),
		pendingQs:        make(map[string]chan []QuestionAnswer),
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
	// Snapshot maxTurns under the lock — callers may override via SetMaxTurns
	// before calling ExecuteWithTools, and this avoids races with concurrent sessions.
	a.mu.Lock()
	maxTurns := a.maxTurns
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
	toolsSucceededThisEpisode := false // Track if any tool executed successfully this episode
	writeCompletedLastTurn := false    // When true, force text-only response on next turn
	toolBlockedLastTurn := false       // When true, force text-only response after budget/loop block
	preferredToolName := ""
	preferredToolRetried := false
	singleToolRequested := isSingleToolRequest(providerMessages)
	singleToolEnforced := false

	// Fresh-data intent: if the user's latest message indicates they want
	// fresh/updated data, bypass the knowledge gate so tools re-execute.
	userWantsFresh := detectFreshDataIntent(messages)

	// Loop detection: track identical tool calls (name + serialized input).
	// After maxIdenticalCalls identical invocations, the next one is blocked.
	const maxIdenticalCalls = 3
	recentCallCounts := make(map[string]int)

	// Track where each turn's messages begin in providerMessages for compaction.
	// We keep the last N turns' tool results in full; older ones get compacted.
	const compactionKeepTurns = 2                  // Keep last 2 turns' tool results in full (KA preserves key facts)
	const compactionMinChars = 300                 // Only compact results longer than this
	currentTurnStartIndex := len(providerMessages) // Initial messages are never compacted

	// Wrap-up nudge: after this many cumulative tool calls, hint the model to start wrapping up.
	const wrapUpNudgeAfterCalls = 12
	const wrapUpEscalateAfterCalls = 18
	totalToolCalls := 0
	wrapUpNudgeFired := false
	wrapUpEscalateFired := false

	for turn < maxTurns {
		// === CONTEXT COMPACTION: Compact old tool results to prevent context blowout ===
		if turn > 0 {
			compactOldToolResults(providerMessages, currentTurnStartIndex, compactionKeepTurns, compactionMinChars, a.knowledgeAccumulator)
		}

		// Check if aborted
		a.mu.Lock()
		if a.aborted[sessionID] {
			a.mu.Unlock()
			return resultMessages, fmt.Errorf("session aborted")
		}
		providerName := a.providerName
		modelName := a.modelName
		a.mu.Unlock()

		// Record telemetry for loop iteration
		if metrics := GetAIMetrics(); metrics != nil {
			metrics.RecordAgenticIteration(providerName, modelName)
		}

		// Check context
		select {
		case <-ctx.Done():
			return resultMessages, ctx.Err()
		default:
		}

		log.Info().
			Int("turn", turn).
			Int("messages", len(providerMessages)).
			Int("tools", len(tools)).
			Int("total_input_tokens", a.totalInputTokens).
			Int("total_output_tokens", a.totalOutputTokens).
			Str("session_id", sessionID).
			Msg("[AgenticLoop] Starting turn")

		// Build the request with dynamic system prompt (includes current mode)
		systemPrompt := a.getSystemPrompt()
		req := providers.ChatRequest{
			Messages: providerMessages,
			System:   systemPrompt,
			Tools:    tools,
		}

		// Determine tool_choice based on turn, intent, and explicit tool requests.
		// We only force tool use when:
		// 1. Tools are available
		// 2. It's the first turn
		// 3. The user's message indicates they need live data or an action
		// This prevents forcing tool calls on conceptual questions like "What is TCP?"
		forcedTextOnly := false
		if turn >= maxTurns-1 {
			// Last turn before hitting the limit — force a text-only response so
			// the model summarizes its findings instead of silently stopping.
			req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceNone}
			forcedTextOnly = true
			log.Warn().
				Int("turn", turn).
				Int("max_turns", maxTurns).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Approaching max turns — forcing text-only response for summary")
		} else if writeCompletedLastTurn {
			// A write action completed successfully on the previous turn.
			// Force text-only response so the model summarizes the result instead of
			// making more tool calls (which often return stale cached data and cause loops).
			req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceNone}
			forcedTextOnly = true
			writeCompletedLastTurn = false
			log.Debug().
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Write completed last turn — forcing text-only response")
		} else if toolBlockedLastTurn {
			// Tool calls were blocked last turn (budget exceeded or loop detected).
			// The model already has the data it gathered — force it to produce a text
			// response instead of continuing to call tools that will just be blocked again.
			req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceNone}
			forcedTextOnly = true
			toolBlockedLastTurn = false
			log.Debug().
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Tool calls blocked last turn — forcing text-only response")
		} else if len(tools) > 0 {
			if preferredToolName == "" {
				preferredToolName = getPreferredTool(providerMessages, tools)
			}
			if preferredToolName != "" {
				req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceTool, Name: preferredToolName}
				if singleToolRequested {
					singleToolEnforced = true
				}
				log.Debug().
					Str("session_id", sessionID).
					Str("tool", preferredToolName).
					Msg("[AgenticLoop] Explicit tool request - forcing tool")
			} else if turn == 0 && requiresToolUse(providerMessages) {
				// First turn with action intent: force the model to use a tool
				req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceAny}
				log.Debug().
					Str("session_id", sessionID).
					Msg("[AgenticLoop] First turn with action intent - forcing tool use")
			} else {
				req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceAuto}
				if turn == 0 {
					log.Debug().
						Str("session_id", sessionID).
						Msg("[AgenticLoop] First turn appears conceptual - using auto tool choice")
				}
			}
		}

		// Collect streaming response
		var contentBuilder strings.Builder
		var thinkingBuilder strings.Builder
		var toolCalls []providers.ToolCall

		log.Debug().
			Str("session_id", sessionID).
			Int("system_prompt_len", len(systemPrompt)).
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
					// Format input for frontend display
					// For control tools, show a human-readable summary instead of raw JSON to avoid "hallucination" look
					inputStr := "{}"
					rawInput := ""
					if data.Input != nil {
						if inputBytes, err := json.Marshal(data.Input); err == nil {
							rawInput = string(inputBytes)
						}
						// Special handling for command execution tools to avoid showing raw JSON
						if data.Name == "pulse_control" || data.Name == "pulse_run_command" || data.Name == "control" {
							if cmd, ok := data.Input["command"].(string); ok {
								// Just show the command being run
								inputStr = fmt.Sprintf("Running: %s", cmd)
							} else if action, ok := data.Input["action"].(string); ok {
								// Show action (e.g. for guest control)
								target := ""
								if t, ok := data.Input["guest_id"].(string); ok {
									target = t
								} else if t, ok := data.Input["container"].(string); ok {
									target = t
								}
								inputStr = fmt.Sprintf("%s %s", action, target)
							} else {
								// Fallback to JSON
								if inputBytes, err := json.Marshal(data.Input); err == nil {
									inputStr = string(inputBytes)
								}
							}
						} else {
							// Standard JSON for other tools
							if inputBytes, err := json.Marshal(data.Input); err == nil {
								inputStr = string(inputBytes)
							}
						}
					}
					jsonData, _ := json.Marshal(ToolStartData{
						ID:       data.ID,
						Name:     data.Name,
						Input:    inputStr,
						RawInput: rawInput,
					})
					callback(StreamEvent{Type: "tool_start", Data: jsonData})
				}

			case "done":
				if data, ok := event.Data.(providers.DoneEvent); ok {
					toolCalls = data.ToolCalls
					a.totalInputTokens += data.InputTokens
					a.totalOutputTokens += data.OutputTokens
					log.Info().
						Int("turn", turn).
						Int("input_tokens_this_turn", data.InputTokens).
						Int("output_tokens_this_turn", data.OutputTokens).
						Int("total_input_tokens", a.totalInputTokens).
						Int("total_output_tokens", a.totalOutputTokens).
						Int("tool_calls", len(data.ToolCalls)).
						Str("session_id", sessionID).
						Msg("[AgenticLoop] Turn completed")
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

		// Guard: if we forced text-only but the model still returned tool calls
		// (some providers like Gemini can hallucinate function calls from conversation
		// history even when tools are not offered in the request), strip them so the
		// model's text content is treated as the final response.
		if forcedTextOnly && len(toolCalls) > 0 {
			log.Warn().
				Str("session_id", sessionID).
				Int("stripped_tool_calls", len(toolCalls)).
				Msg("[AgenticLoop] Model returned tool calls despite ToolChoiceNone — stripping them")
			toolCalls = nil
		}

		// Check mid-run budget after each turn completes
		if a.budgetChecker != nil {
			if budgetErr := a.budgetChecker(); budgetErr != nil {
				log.Warn().Err(budgetErr).Int("turn", turn).Str("session_id", sessionID).
					Msg("[AgenticLoop] Budget exceeded mid-run, stopping")
				return resultMessages, fmt.Errorf("budget exceeded: %w", budgetErr)
			}
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

		// If no tool calls, we're done - but first check FSM and phantom execution
		if len(toolCalls) == 0 {
			// If the user explicitly requested a tool and the model didn't comply, retry once.
			if preferredToolName != "" && !preferredToolRetried {
				preferredToolRetried = true

				retryPrompt := fmt.Sprintf("Tool required: use %s for this request.", preferredToolName)
				if len(resultMessages) > 0 {
					resultMessages[len(resultMessages)-1].Content = retryPrompt
				}

				turn++
				continue
			}

			// === FSM ENFORCEMENT GATE 2: Check if final answer is allowed ===
			a.mu.Lock()
			fsm := a.sessionFSM
			a.mu.Unlock()

			if fsm != nil {
				if fsmErr := fsm.CanFinalAnswer(); fsmErr != nil {
					log.Warn().
						Str("session_id", sessionID).
						Str("state", string(fsm.State)).
						Bool("wrote_this_episode", fsm.WroteThisEpisode).
						Bool("read_after_write", fsm.ReadAfterWrite).
						Msg("[AgenticLoop] FSM blocked final answer - must verify write first")

					// Record telemetry for FSM final answer block
					if metrics := GetAIMetrics(); metrics != nil {
						metrics.RecordFSMFinalBlock(fsm.State)
					}

					// Inject a minimal, factual constraint - not a narrative or example.
					// This tells the model what is required, not how to do it.
					verifyPrompt := fmt.Sprintf(
						"Verification required: perform a read or status check on %s before responding.",
						fsm.LastWriteTool,
					)

					// Update the last assistant message to include verification constraint
					if len(resultMessages) > 0 {
						resultMessages[len(resultMessages)-1].Content = verifyPrompt
					}

					// Note: verification constraint is injected into resultMessages above (for the model).
					// We intentionally do NOT emit this to the user callback — it's an internal protocol
					// prompt that would appear as spam in the chat output.

					// Mark that we completed verification (the next read will set ReadAfterWrite)
					// and continue the loop to force a verification read
					turn++
					continue
				}

				// If we're completing successfully and there was a write, mark verification complete
				if fsm.State == StateVerifying && fsm.ReadAfterWrite {
					fsm.CompleteVerification()
					log.Debug().
						Str("session_id", sessionID).
						Str("new_state", string(fsm.State)).
						Msg("[AgenticLoop] FSM verification complete, transitioning to READING")
				}
			}

			// Detect phantom execution: model claims to have done something without tool calls
			// This is especially important for providers that can't force tool use (e.g., Ollama)
			// IMPORTANT: Skip this check if tools already succeeded this episode - the model is
			// legitimately summarizing tool results, not hallucinating.
			log.Debug().
				Bool("toolsSucceededThisEpisode", toolsSucceededThisEpisode).
				Bool("hasPhantomExecution", hasPhantomExecution(assistantMsg.Content)).
				Str("content_preview", truncateForLog(assistantMsg.Content, 200)).
				Msg("[AgenticLoop] Phantom detection check")
			if !toolsSucceededThisEpisode && hasPhantomExecution(assistantMsg.Content) {
				log.Warn().
					Str("session_id", sessionID).
					Str("content_preview", truncateForLog(assistantMsg.Content, 200)).
					Msg("[AgenticLoop] Phantom execution detected - model claims action without tool call")

				// Record telemetry for phantom detection
				if metrics := GetAIMetrics(); metrics != nil {
					metrics.RecordPhantomDetected(providerName, modelName)
				}

				// Replace the response with a safe failure message
				safeResponse := "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request. This can happen when:\n\n" +
					"1. The tools aren't available right now\n" +
					"2. There was a connection issue\n" +
					"3. The model I'm running on doesn't support function calling\n\n" +
					"Please try again, or let me know if you have a question I can answer without checking live infrastructure."

				// Update the last result message
				if len(resultMessages) > 0 {
					resultMessages[len(resultMessages)-1].Content = safeResponse
				}

				// Send corrected content to callback
				jsonData, _ := json.Marshal(ContentData{Text: "\n\n---\n" + safeResponse})
				callback(StreamEvent{Type: "content", Data: jsonData})
			}

			log.Debug().Msg("Agentic loop complete - no tool calls")
			resultMessages = a.ensureFinalTextResponse(ctx, sessionID, resultMessages, providerMessages, callback)
			return resultMessages, nil
		}

		// === Execute tool calls (three-phase pipeline) ===
		// Phase 1: Pre-check (sequential) — FSM, loop detection, knowledge gate
		// Phase 2: Execute (parallel) — actual tool calls via goroutines
		// Phase 3: Post-process (sequential) — streaming, FSM transitions, KA extraction
		if len(toolCalls) > 0 && preferredToolName != "" {
			// Clear preferred tool once the model has used any tool.
			preferredToolName = ""
		}
		firstToolResultText := ""
		budgetBlockedThisTurn := 0

		// --- Phase 1: Pre-check all tool calls sequentially ---
		// Pre-checks share mutable state (FSM, loop counts) so must be sequential.
		a.mu.Lock()
		if a.aborted[sessionID] {
			a.mu.Unlock()
			return resultMessages, fmt.Errorf("session aborted")
		}
		fsm := a.sessionFSM
		a.mu.Unlock()

		type pendingToolExec struct {
			tc       providers.ToolCall
			toolKind ToolKind
		}
		var pendingExec []pendingToolExec

		for _, tc := range toolCalls {
			log.Debug().
				Str("tool", tc.Name).
				Str("id", tc.ID).
				Msg("Pre-checking tool call")

			// === FSM ENFORCEMENT GATE 1: Check if tool is allowed in current state ===
			toolKind := ClassifyToolCall(tc.Name, tc.Input)
			if fsm != nil {
				if fsmErr := fsm.CanExecuteTool(toolKind, tc.Name); fsmErr != nil {
					log.Warn().
						Str("tool", tc.Name).
						Str("kind", toolKind.String()).
						Str("state", string(fsm.State)).
						Err(fsmErr).
						Msg("[AgenticLoop] FSM blocked tool execution")

					// Record telemetry for FSM tool block
					if metrics := GetAIMetrics(); metrics != nil {
						metrics.RecordFSMToolBlock(fsm.State, tc.Name, toolKind)
					}

					// Return the FSM error as a tool result so the model can self-correct
					fsmBlockedErr, ok := fsmErr.(*FSMBlockedError)
					var recoveryHint string
					if ok && fsmBlockedErr.Recoverable {
						recoveryHint = " Use a discovery or read tool first, then retry."
						// Track pending recovery for success correlation
						fsm.TrackPendingRecovery("FSM_BLOCKED", tc.Name)
						// Record auto-recovery attempt (model gets a chance to self-correct)
						if metrics := GetAIMetrics(); metrics != nil {
							metrics.RecordAutoRecoveryAttempt("FSM_BLOCKED", tc.Name)
						}
					}

					// Send tool_end event with error
					jsonData, _ := json.Marshal(ToolEndData{
						ID:      tc.ID,
						Name:    tc.Name,
						Input:   "",
						Output:  fsmErr.Error() + recoveryHint,
						Success: false,
					})
					callback(StreamEvent{Type: "tool_end", Data: jsonData})

					// Create tool result message with the error
					toolResultMsg := Message{
						ID:        uuid.New().String(),
						Role:      "user",
						Timestamp: time.Now(),
						ToolResult: &ToolResult{
							ToolUseID: tc.ID,
							Content:   fsmErr.Error() + recoveryHint,
							IsError:   true,
						},
					}
					resultMessages = append(resultMessages, toolResultMsg)

					// Add to provider messages for next turn
					providerMessages = append(providerMessages, providers.Message{
						Role: "user",
						ToolResult: &providers.ToolResult{
							ToolUseID: tc.ID,
							Content:   fsmErr.Error() + recoveryHint,
							IsError:   true,
						},
					})

					// Skip execution but continue the loop to process other tool calls
					continue
				}
			}

			// === LOOP DETECTION: Block identical repeated tool calls ===
			callKey := toolCallKey(tc.Name, tc.Input)
			recentCallCounts[callKey]++
			if recentCallCounts[callKey] > maxIdenticalCalls {
				log.Warn().
					Str("tool", tc.Name).
					Int("count", recentCallCounts[callKey]).
					Str("session_id", sessionID).
					Msg("[AgenticLoop] LOOP_DETECTED: blocking repeated identical tool call")

				loopMsg := fmt.Sprintf("LOOP_DETECTED: You have called %s with the same arguments %d times. This call is blocked. Try a different tool or approach.", tc.Name, recentCallCounts[callKey])

				jsonData, _ := json.Marshal(ToolEndData{
					ID:      tc.ID,
					Name:    tc.Name,
					Input:   "",
					Output:  loopMsg,
					Success: false,
				})
				callback(StreamEvent{Type: "tool_end", Data: jsonData})

				toolResultMsg := Message{
					ID:        uuid.New().String(),
					Role:      "user",
					Timestamp: time.Now(),
					ToolResult: &ToolResult{
						ToolUseID: tc.ID,
						Content:   loopMsg,
						IsError:   true,
					},
				}
				resultMessages = append(resultMessages, toolResultMsg)
				providerMessages = append(providerMessages, providers.Message{
					Role: "user",
					ToolResult: &providers.ToolResult{
						ToolUseID: tc.ID,
						Content:   loopMsg,
						IsError:   true,
					},
				})
				budgetBlockedThisTurn++
				continue
			}

			// === KNOWLEDGE GATE: Return cached facts for redundant tool calls ===
			// Skip gate on first turn if the user explicitly asked for fresh data.
			if a.knowledgeAccumulator != nil && !(userWantsFresh && turn == 0) {
				if keys := PredictFactKeys(tc.Name, tc.Input); len(keys) > 0 {
					var cachedParts []string
					for _, key := range keys {
						if value, found := a.knowledgeAccumulator.Lookup(key); found {
							cachedParts = append(cachedParts, value)
						}
					}
					if len(cachedParts) > 0 {
						// Enrich marker-based cache hits with related per-resource facts
						for _, key := range keys {
							if prefix, ok := MarkerExpansions[key]; ok {
								if related := a.knowledgeAccumulator.RelatedFacts(prefix); related != "" {
									cachedParts = append(cachedParts, related)
								}
							}
						}
						cachedResult := fmt.Sprintf("Already known (from earlier investigation): %s. If you need fresh data, use a different query or approach.", strings.Join(cachedParts, "; "))

						log.Info().
							Str("tool", tc.Name).
							Str("session_id", sessionID).
							Strs("matched_keys", keys).
							Int("cached_parts", len(cachedParts)).
							Msg("[AgenticLoop] Knowledge gate: returning cached fact instead of re-executing tool")

						jsonData, _ := json.Marshal(ToolEndData{
							ID:      tc.ID,
							Name:    tc.Name,
							Input:   "",
							Output:  cachedResult,
							Success: true,
						})
						callback(StreamEvent{Type: "tool_end", Data: jsonData})

						toolResultMsg := Message{
							ID:        uuid.New().String(),
							Role:      "user",
							Timestamp: time.Now(),
							ToolResult: &ToolResult{
								ToolUseID: tc.ID,
								Content:   cachedResult,
								IsError:   false,
							},
						}
						resultMessages = append(resultMessages, toolResultMsg)
						providerMessages = append(providerMessages, providers.Message{
							Role: "user",
							ToolResult: &providers.ToolResult{
								ToolUseID: tc.ID,
								Content:   cachedResult,
								IsError:   false,
							},
						})
						continue
					}
				}
			}

			// Tool passed all pre-checks — queue for execution
			pendingExec = append(pendingExec, pendingToolExec{tc: tc, toolKind: toolKind})
		}

		// --- Phase 2: Execute pending tools (parallel if multiple) ---
		// Tool execution is stateless I/O — safe to parallelize.
		// Cap concurrency at 4 to avoid overwhelming infrastructure.
		execResults := make([]parallelToolResult, len(pendingExec))

		if len(pendingExec) > 1 {
			log.Info().
				Int("tool_count", len(pendingExec)).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Executing multiple tools in parallel")

			var wg sync.WaitGroup
			sem := make(chan struct{}, 4)

			for j, pe := range pendingExec {
				wg.Add(1)
				go func(idx int, tc providers.ToolCall) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					r, e := a.executor.ExecuteTool(ctx, tc.Name, tc.Input)
					execResults[idx] = parallelToolResult{Result: r, Err: e}
				}(j, pe.tc)
			}
			wg.Wait()
		} else if len(pendingExec) == 1 {
			r, e := a.executor.ExecuteTool(ctx, pendingExec[0].tc.Name, pendingExec[0].tc.Input)
			execResults[0] = parallelToolResult{Result: r, Err: e}
		}

		// --- Phase 3: Post-process results in original order (sequential) ---
		// Streaming events, FSM transitions, KA extraction, approval flow
		// must all be sequential and in the original tool call order.
		for j, pe := range pendingExec {
			tc := pe.tc
			toolKind := pe.toolKind

			result := execResults[j].Result
			err := execResults[j].Err

			var resultText string
			var isError bool

			if err != nil {
				resultText = fmt.Sprintf("Error: %v", err)
				isError = true
			} else {
				resultText = FormatToolResult(result)
				isError = result.IsError
				if !isError {
					toolsSucceededThisEpisode = true // Tool executed successfully
					log.Debug().
						Str("tool", tc.Name).
						Msg("[AgenticLoop] Tool succeeded - toolsSucceededThisEpisode set to true")
				}
				// Extract and accumulate knowledge facts from both success and structured error responses
				if a.knowledgeAccumulator != nil {
					a.knowledgeAccumulator.SetTurn(turn)
					facts := ExtractFacts(tc.Name, tc.Input, resultText)
					for _, f := range facts {
						a.knowledgeAccumulator.AddFactForTool(tc.ID, f.Category, f.Key, f.Value)
						log.Debug().
							Str("tool", tc.Name).
							Str("fact_key", f.Key).
							Int("value_len", len(f.Value)).
							Msg("[AgenticLoop] Stored knowledge fact")
					}

					// If no facts were extracted but we predicted keys, store negative markers
					// to prevent the gate from re-executing the same tool call.
					if len(facts) == 0 && !isError {
						if predictedKeys := PredictFactKeys(tc.Name, tc.Input); len(predictedKeys) > 0 {
							summary := summarizeForNegativeMarker(resultText)
							for _, key := range predictedKeys {
								if _, found := a.knowledgeAccumulator.Lookup(key); !found {
									cat := categoryForPredictedKey(key)
									a.knowledgeAccumulator.AddFactForTool(tc.ID, cat, key, fmt.Sprintf("checked: %s", summary))
									log.Debug().
										Str("tool", tc.Name).
										Str("fact_key", key).
										Msg("[AgenticLoop] Stored negative marker (text response)")
								}
							}
						}
					}
				}
			}

			if firstToolResultText == "" {
				firstToolResultText = resultText
			}

			// Track pending recovery for strict resolution blocks
			// (FSM blocks are tracked above; strict resolution blocks come from the executor)
			if isError && fsm != nil && strings.Contains(resultText, "STRICT_RESOLUTION") {
				fsm.TrackPendingRecovery("STRICT_RESOLUTION", tc.Name)
				log.Debug().
					Str("tool", tc.Name).
					Msg("[AgenticLoop] Tracking pending recovery for strict resolution block")
			}

			// === AUTO-RECOVERY FOR NONINTERACTIVEONLY BLOCKS ===
			// If tool blocked with auto_recoverable=true and has a suggested_rewrite,
			// automatically apply the rewrite and retry once.
			// Note: err == nil means executor didn't throw, isError means the tool result indicates error/block
			if err == nil && isError && strings.Contains(resultText, `"auto_recoverable":true`) {
				// Result is a blocked response (not a hard error)
				if suggestedRewrite, recoveryAttempted := tryAutoRecovery(result, tc, a.executor, ctx); suggestedRewrite != "" && !recoveryAttempted {
					// This is a fresh recoverable block - attempt auto-recovery
					log.Info().
						Str("tool", tc.Name).
						Str("original_command", getCommandFromInput(tc.Input)).
						Str("suggested_rewrite", suggestedRewrite).
						Msg("[AgenticLoop] Attempting auto-recovery with suggested rewrite")

					// Record auto-recovery attempt
					if metrics := GetAIMetrics(); metrics != nil {
						metrics.RecordAutoRecoveryAttempt("READ_ONLY_VIOLATION", tc.Name)
					}

					// Apply the rewrite and retry
					modifiedInput := make(map[string]interface{})
					for k, v := range tc.Input {
						modifiedInput[k] = v
					}
					modifiedInput["command"] = suggestedRewrite
					modifiedInput["_auto_recovery_attempt"] = true // Prevent infinite loops

					retryResult, retryErr := a.executor.ExecuteTool(ctx, tc.Name, modifiedInput)
					if retryErr != nil {
						log.Warn().
							Err(retryErr).
							Str("tool", tc.Name).
							Msg("[AgenticLoop] Auto-recovery retry failed with error")
					} else if !retryResult.IsError {
						// Recovery succeeded!
						log.Info().
							Str("tool", tc.Name).
							Msg("[AgenticLoop] Auto-recovery succeeded")
						if metrics := GetAIMetrics(); metrics != nil {
							metrics.RecordAutoRecoverySuccess("READ_ONLY_VIOLATION", tc.Name)
						}
						// Use the successful result
						result = retryResult
						resultText = FormatToolResult(result)
						isError = false
					} else {
						log.Warn().
							Str("tool", tc.Name).
							Str("retry_error", FormatToolResult(retryResult)).
							Msg("[AgenticLoop] Auto-recovery retry still blocked")
						// Keep original error but note the failed recovery attempt
						resultText = resultText + "\n\n[Auto-recovery attempted but failed. Please use the suggested command manually or switch to pulse_control.]"
					}
				}
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

			// Send tool_end event
			// Convert input to JSON string for frontend display
			inputStr := ""
			rawInput := ""
			if tc.Input != nil {
				if inputBytes, err := json.Marshal(tc.Input); err == nil {
					rawInput = string(inputBytes)
				}
				// Special handling for command execution tools to avoid showing raw JSON
				if tc.Name == "pulse_control" || tc.Name == "pulse_run_command" || tc.Name == "control" {
					if cmd, ok := tc.Input["command"].(string); ok {
						// Just show the command being run
						inputStr = fmt.Sprintf("Running: %s", cmd)
					} else if action, ok := tc.Input["action"].(string); ok {
						// Show action (e.g. for guest control)
						target := ""
						if t, ok := tc.Input["guest_id"].(string); ok {
							target = t
						} else if t, ok := tc.Input["container"].(string); ok {
							target = t
						}
						inputStr = fmt.Sprintf("%s %s", action, target)
					} else {
						// Fallback to JSON
						if inputBytes, err := json.Marshal(tc.Input); err == nil {
							inputStr = string(inputBytes)
						}
					}
				} else {
					if inputBytes, err := json.Marshal(tc.Input); err == nil {
						inputStr = string(inputBytes)
					}
				}
			}
			jsonData, _ := json.Marshal(ToolEndData{
				ID:       tc.ID,
				Name:     tc.Name,
				Input:    inputStr,
				RawInput: rawInput,
				Output:   resultText,
				Success:  !isError,
			})
			callback(StreamEvent{Type: "tool_end", Data: jsonData})

			// === FSM STATE TRANSITION: Update FSM after successful tool execution ===
			if fsm != nil && !isError {
				fsm.OnToolSuccess(toolKind, tc.Name)

				// If we just completed verification (read after write in VERIFYING), transition to READING
				// This allows subsequent writes to proceed without being blocked
				// CRITICAL: Must call this IMMEDIATELY after OnToolSuccess, not just when model gives final answer
				if fsm.State == StateVerifying && fsm.ReadAfterWrite {
					fsm.CompleteVerification()
					log.Debug().
						Str("tool", tc.Name).
						Str("new_state", string(fsm.State)).
						Msg("[AgenticLoop] FSM verification complete after read, transitioning to READING")
				}

				// === AUTO-VERIFY: After a write, advance the FSM past VERIFYING ===
				// The FSM requires a read after every write. Rather than querying
				// infrastructure (which returns stale cached data that contradicts
				// the success message and confuses the model), we advance the FSM
				// directly. The control tool already confirms success/failure —
				// that IS the verification.
				if fsm.State == StateVerifying && toolKind == ToolKindWrite {
					log.Info().
						Str("write_tool", tc.Name).
						Msg("[AgenticLoop] Auto-advancing FSM past VERIFYING — control tool result is the verification")

					// Simulate a successful read to satisfy the FSM
					fsm.OnToolSuccess(ToolKindRead, "auto_verify")
					if fsm.State == StateVerifying && fsm.ReadAfterWrite {
						fsm.CompleteVerification()
					}
					log.Info().
						Str("new_state", string(fsm.State)).
						Msg("[AgenticLoop] FSM advanced past VERIFYING")

					// Force the model to produce a text response on the next turn.
					// Without this, the model calls read tools to verify the write,
					// but Pulse's cached state is stale and contradicts the success
					// message, causing the model to loop.
					writeCompletedLastTurn = true
				}

				log.Debug().
					Str("tool", tc.Name).
					Str("kind", toolKind.String()).
					Str("new_state", string(fsm.State)).
					Bool("wrote_this_episode", fsm.WroteThisEpisode).
					Bool("read_after_write", fsm.ReadAfterWrite).
					Msg("[AgenticLoop] FSM state transition after tool success")

				// Check if this success resolves a pending recovery
				if pr := fsm.CheckRecoverySuccess(tc.Name); pr != nil {
					log.Info().
						Str("tool", tc.Name).
						Str("error_code", pr.ErrorCode).
						Str("recovery_id", pr.RecoveryID).
						Msg("[AgenticLoop] Auto-recovery succeeded")
					if metrics := GetAIMetrics(); metrics != nil {
						metrics.RecordAutoRecoverySuccess(pr.ErrorCode, pr.Tool)
					}
				}
			}

			// Compute model-facing result text AFTER auto-verify may have appended data.
			// This ensures the model sees the verification result and task-completion signal.
			modelResultText := truncateToolResultForModel(resultText)

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

		// If any tool call this turn was budget-blocked or loop-detected, force
		// the model to produce text on the next turn. It already has the data from
		// earlier successful calls — making more tool calls will just waste tokens.
		if budgetBlockedThisTurn > 0 {
			toolBlockedLastTurn = true
			log.Warn().
				Int("blocked", budgetBlockedThisTurn).
				Int("total_calls", len(toolCalls)).
				Int("turn", turn).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Tool calls blocked this turn — will force text-only next turn")
		}

		if singleToolEnforced && len(toolCalls) > 0 {
			summary := firstToolResultText
			if strings.TrimSpace(summary) == "" {
				if preferredToolName != "" {
					summary = fmt.Sprintf("Tool %s completed.", preferredToolName)
				} else {
					summary = "Tool call completed."
				}
			}
			if len(resultMessages) > 0 {
				lastIdx := len(resultMessages) - 1
				if resultMessages[lastIdx].Role == "assistant" && strings.TrimSpace(resultMessages[lastIdx].Content) == "" {
					resultMessages[lastIdx].Content = summary
				}
			}
			jsonData, _ := json.Marshal(ContentData{Text: summary})
			callback(StreamEvent{Type: "content", Data: jsonData})
			return resultMessages, nil
		}

		// Track cumulative tool calls and inject wrap-up nudge/escalation if threshold exceeded
		totalToolCalls += len(toolCalls)
		if !wrapUpNudgeFired && totalToolCalls >= wrapUpNudgeAfterCalls {
			maybeInjectWrapUpNudge(providerMessages, totalToolCalls, maxTurns, turn, wrapUpNudgeAfterCalls)
			wrapUpNudgeFired = true
		} else if wrapUpNudgeFired && !wrapUpEscalateFired && totalToolCalls >= wrapUpEscalateAfterCalls {
			maybeInjectWrapUpEscalation(providerMessages, totalToolCalls)
			wrapUpEscalateFired = true
		}

		// Mark the start of the next turn's messages for compaction tracking
		currentTurnStartIndex = len(providerMessages)
		turn++
	}

	log.Warn().Int("max_turns", maxTurns).Str("session_id", sessionID).Msg("Agentic loop hit max turns limit")
	resultMessages = a.ensureFinalTextResponse(ctx, sessionID, resultMessages, providerMessages, callback)
	return resultMessages, nil
}

// ensureFinalTextResponse checks if the result messages contain any assistant text.
// If not, it makes one last text-only LLM call to force the model to summarize its findings.
// This prevents the loop from exiting silently after making tool calls without answering.
func (a *AgenticLoop) ensureFinalTextResponse(
	ctx context.Context,
	sessionID string,
	resultMessages []Message,
	providerMessages []providers.Message,
	callback StreamCallback,
) []Message {
	// Check if any assistant message has text content
	for i := len(resultMessages) - 1; i >= 0; i-- {
		if resultMessages[i].Role == "assistant" && strings.TrimSpace(resultMessages[i].Content) != "" {
			return resultMessages // Already has text — nothing to do
		}
	}

	// No text content from the model. Make a final text-only call.
	log.Warn().Str("session_id", sessionID).Msg("[AgenticLoop] No text content produced — making final summary call")

	// Build clean message history for the summary call:
	// 1. Strip any trailing empty assistant messages (the model already failed to produce
	//    text with these, so including them would just get the same empty result).
	// 2. Append a user-role nudge to give the model a clear instruction.
	cleanMessages := make([]providers.Message, len(providerMessages))
	copy(cleanMessages, providerMessages)
	for len(cleanMessages) > 0 {
		last := cleanMessages[len(cleanMessages)-1]
		if last.Role == "assistant" && strings.TrimSpace(last.Content) == "" && len(last.ToolCalls) == 0 {
			cleanMessages = cleanMessages[:len(cleanMessages)-1]
		} else {
			break
		}
	}
	cleanMessages = append(cleanMessages, providers.Message{
		Role:    "user",
		Content: "You've gathered information above using tools. Now provide your analysis and answer to the original question. Summarize your findings concisely.",
	})

	summaryReq := providers.ChatRequest{
		Messages:   cleanMessages,
		System:     a.getSystemPrompt(),
		ToolChoice: &providers.ToolChoice{Type: providers.ToolChoiceNone},
		// No Tools field — completely omit tools to prevent hallucinated function calls
	}

	var summaryBuilder strings.Builder

	summaryErr := a.provider.ChatStream(ctx, summaryReq, func(event providers.StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(providers.ContentEvent); ok {
				summaryBuilder.WriteString(data.Text)
				jsonData, _ := json.Marshal(ContentData{Text: data.Text})
				callback(StreamEvent{Type: "content", Data: jsonData})
			}
		case "done":
			if data, ok := event.Data.(providers.DoneEvent); ok {
				a.totalInputTokens += data.InputTokens
				a.totalOutputTokens += data.OutputTokens
			}
		}
	})

	if summaryErr == nil && summaryBuilder.Len() > 0 {
		summaryMsg := Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   summaryBuilder.String(),
			Timestamp: time.Now(),
		}
		resultMessages = append(resultMessages, summaryMsg)
		log.Info().Str("session_id", sessionID).Int("summary_len", summaryBuilder.Len()).Msg("[AgenticLoop] Final summary produced")
	} else if summaryErr != nil {
		log.Error().Err(summaryErr).Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call failed")
	} else {
		log.Warn().Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call returned empty content")
	}

	return resultMessages
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

// SetSessionFSM sets the workflow FSM for the current session.
// This must be called before ExecuteWithTools to enable structural guarantees.
func (a *AgenticLoop) SetSessionFSM(fsm *SessionFSM) {
	a.mu.Lock()
	a.sessionFSM = fsm
	a.mu.Unlock()
}

// SetKnowledgeAccumulator sets the knowledge accumulator for fact extraction.
// This must be called before Execute to enable knowledge accumulation.
func (a *AgenticLoop) SetKnowledgeAccumulator(ka *KnowledgeAccumulator) {
	a.mu.Lock()
	a.knowledgeAccumulator = ka
	a.mu.Unlock()
}

// SetMaxTurns overrides the maximum number of agentic turns for this loop.
func (a *AgenticLoop) SetMaxTurns(n int) {
	a.mu.Lock()
	a.maxTurns = n
	a.mu.Unlock()
}

// SetProviderInfo sets the provider/model info for telemetry.
func (a *AgenticLoop) SetProviderInfo(provider, model string) {
	a.mu.Lock()
	a.providerName = provider
	a.modelName = model
	a.mu.Unlock()
}

// SetBudgetChecker sets a function called after each agentic turn to enforce
// token spending limits. If the checker returns an error, the loop stops.
func (a *AgenticLoop) SetBudgetChecker(fn func() error) {
	a.budgetChecker = fn
}

// GetTotalInputTokens returns the accumulated input tokens across all turns.
func (a *AgenticLoop) GetTotalInputTokens() int {
	return a.totalInputTokens
}

// GetTotalOutputTokens returns the accumulated output tokens across all turns.
func (a *AgenticLoop) GetTotalOutputTokens() int {
	return a.totalOutputTokens
}

// ResetTokenCounts resets the accumulated token counts (for reuse across executions).
func (a *AgenticLoop) ResetTokenCounts() {
	a.totalInputTokens = 0
	a.totalOutputTokens = 0
}

// hasPhantomExecution detects when the model claims to have executed something
// but no actual tool calls were made. This catches models that "hallucinate"
// tool execution by writing about it instead of calling tools.
//
// We're intentionally conservative here to avoid false positives like:
// - "I checked the docs..." (not a tool)
// - "I ran through the logic..." (not a command)
//
// We only trigger when the model asserts:
// 1. Concrete system metrics/values (CPU %, memory usage, etc.)
// 2. Infrastructure state that requires live queries (running/stopped)
// 3. Fake tool call formatting
// summarizeForNegativeMarker creates a concise summary of a tool result for
// use in negative markers. Tries to extract meaningful context from JSON
// responses rather than blindly truncating.
func summarizeForNegativeMarker(resultText string) string {
	if len(resultText) == 0 {
		return "empty response"
	}

	// Try to parse as JSON and extract key indicators
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(resultText), &obj); err == nil {
		var indicators []string

		// Check for common empty-result patterns
		for _, arrayKey := range []string{"points", "pools", "disks", "alerts", "findings", "jobs", "tasks", "snapshots", "resources", "containers", "vms", "nodes", "updates"} {
			if arr, ok := obj[arrayKey]; ok {
				if slice, ok := arr.([]interface{}); ok {
					indicators = append(indicators, fmt.Sprintf("%s: %d items", arrayKey, len(slice)))
				}
			}
		}

		// Check for total field
		if total, ok := obj["total"]; ok {
			indicators = append(indicators, fmt.Sprintf("total=%v", total))
		}

		// Check for period/resource_id context
		if rid, ok := obj["resource_id"]; ok {
			indicators = append(indicators, fmt.Sprintf("resource=%v", rid))
		}
		if period, ok := obj["period"]; ok {
			indicators = append(indicators, fmt.Sprintf("period=%v", period))
		}

		// Check for error field
		if errVal, ok := obj["error"]; ok {
			indicators = append(indicators, fmt.Sprintf("error=%v", errVal))
		}

		if len(indicators) > 0 {
			result := strings.Join(indicators, ", ")
			if len(result) > 200 {
				result = result[:200]
			}
			return result
		}
	}

	// Try JSON array
	var arr []interface{}
	if err := json.Unmarshal([]byte(resultText), &arr); err == nil {
		return fmt.Sprintf("array with %d items", len(arr))
	}

	// Fall back to truncated text
	summary := resultText
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return summary
}

// detectFreshDataIntent returns true when the user's latest message explicitly
// requests updated/fresh data (e.g. "check again", "refresh", "what's the
// latest status"). This bypasses the knowledge gate for the first turn so
// tools re-execute instead of returning cached results.
func detectFreshDataIntent(messages []Message) bool {
	// Walk backwards to find the last user message
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	// Strong refresh signals — these clearly indicate the user wants re-execution
	strongPatterns := []string{
		"check again", "look again", "try again", "run again",
		"refresh", "re-check", "recheck", "re check",
		"fresh data", "fresh look", "latest data",
		"has it changed", "did it change", "any changes",
		"what's happening now", "what is happening now",
	}
	for _, p := range strongPatterns {
		if strings.Contains(lastUserContent, p) {
			return true
		}
	}
	return false
}

func hasPhantomExecution(content string) bool {
	if content == "" {
		return false
	}

	lower := strings.ToLower(content)

	// Category 1: Concrete metrics/values that MUST come from tools
	// These are specific enough that they can't be "general knowledge"
	metricsPatterns := []string{
		"cpu usage is ", "cpu is at ", "cpu at ",
		"memory usage is ", "memory is at ", "memory at ",
		"disk usage is ", "disk is at ", "storage at ",
		"using % ", "% cpu", "% memory", "% disk",
		"mb of ram", "gb of ram", "mb of memory", "gb of memory",
	}

	for _, pattern := range metricsPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 2: Claims of infrastructure state that require live queries
	// Must be specific claims about current state, not general discussion
	statePatterns := []string{
		"is currently running", "is currently stopped", "is currently down",
		"is now running", "is now stopped", "is now restarted",
		"the service is running", "the container is running",
		"the service is stopped", "the container is stopped",
		"the logs show", "the output shows", "the result shows",
		"according to the logs", "according to the output",
	}

	for _, pattern := range statePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 3: Fake tool call formatting (definite hallucination)
	fakeToolPatterns := []string{
		"```tool", "```json\n{\"tool", "tool_result:",
		"function_call:", "<tool_call>", "</tool_call>",
		"pulse_query(", "pulse_run_command(", "pulse_control(",
	}

	for _, pattern := range fakeToolPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 4: Past tense claims of SPECIFIC infrastructure actions
	// Only trigger if followed by concrete results (not "I checked and...")
	actionResultPatterns := []string{
		"i restarted the", "i stopped the", "i started the",
		"i killed the", "i terminated the",
		"successfully restarted", "successfully stopped", "successfully started",
		"has been restarted", "has been stopped", "has been started",
	}

	for _, pattern := range actionResultPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// truncateForLog truncates a string for logging, adding "..." if truncated
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// tryAutoRecovery checks if a tool result is auto-recoverable and returns the suggested rewrite.
// Returns (suggestedRewrite, alreadyAttempted) where:
// - suggestedRewrite is the command to retry with (empty if not recoverable)
// - alreadyAttempted is true if auto-recovery was already attempted for this call
func tryAutoRecovery(result tools.CallToolResult, tc providers.ToolCall, executor *tools.PulseToolExecutor, ctx context.Context) (string, bool) {
	// Check if this is already a recovery attempt
	if _, ok := tc.Input["_auto_recovery_attempt"]; ok {
		return "", true // Already attempted, don't retry again
	}

	// Parse the result to check for auto_recoverable flag
	resultStr := FormatToolResult(result)

	// Look for the structured error response pattern
	// The result should contain JSON with auto_recoverable and suggested_rewrite
	if !strings.Contains(resultStr, `"auto_recoverable"`) {
		return "", false
	}

	// Extract the JSON portion from the result
	// Results are formatted as "Error: {json}" or just "{json}"
	jsonStart := strings.Index(resultStr, "{")
	if jsonStart == -1 {
		return "", false
	}

	var parsed struct {
		Error struct {
			Details struct {
				AutoRecoverable  bool   `json:"auto_recoverable"`
				SuggestedRewrite string `json:"suggested_rewrite"`
				Category         string `json:"category"`
			} `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(resultStr[jsonStart:]), &parsed); err != nil {
		// Try alternative format where details are at top level
		var altParsed struct {
			AutoRecoverable  bool   `json:"auto_recoverable"`
			SuggestedRewrite string `json:"suggested_rewrite"`
		}
		if err2 := json.Unmarshal([]byte(resultStr[jsonStart:]), &altParsed); err2 != nil {
			return "", false
		}
		if altParsed.AutoRecoverable && altParsed.SuggestedRewrite != "" {
			return altParsed.SuggestedRewrite, false
		}
		return "", false
	}

	if parsed.Error.Details.AutoRecoverable && parsed.Error.Details.SuggestedRewrite != "" {
		return parsed.Error.Details.SuggestedRewrite, false
	}

	return "", false
}

// toolCallKey returns a string key for a tool call (name + serialized input)
// used to detect repeated identical calls in the agentic loop.
func toolCallKey(name string, input map[string]interface{}) string {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return name
	}
	return name + ":" + string(inputBytes)
}

// getCommandFromInput extracts the command from tool input for logging.
func getCommandFromInput(input map[string]interface{}) string {
	if cmd, ok := input["command"].(string); ok {
		return cmd
	}
	return "<unknown>"
}

// requiresToolUse determines if the user's message requires live data or an action.
// Returns true for messages that need infrastructure access (check status, restart, etc.)
// Returns false for conceptual questions (What is TCP?, How does Docker work?)
func requiresToolUse(messages []providers.Message) bool {
	// Find the last user message
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}

	if lastUserContent == "" {
		return false
	}

	// First, check for explicit conceptual question patterns
	// These should NOT require tools even if they mention infrastructure terms
	conceptualPatterns := []string{
		"what is ", "what's the difference", "what are the",
		"explain ", "how does ", "how do i ", "how to ",
		"why do ", "why does ", "why is it ",
		"tell me about ", "describe ",
		"can you explain", "help me understand",
		"difference between", "best way to", "best practice",
		"is it hard", "is it difficult", "is it easy",
		"should i ", "would you recommend", "what do you think",
	}

	for _, pattern := range conceptualPatterns {
		if strings.Contains(lastUserContent, pattern) {
			// Exception: questions about MY specific infrastructure state are action queries
			// e.g., "what is the status of my server" or "what is my CPU usage"
			hasMyInfra := strings.Contains(lastUserContent, "my ") ||
				strings.Contains(lastUserContent, "on my") ||
				strings.Contains(lastUserContent, "@")
			hasStateQuery := strings.Contains(lastUserContent, "status") ||
				strings.Contains(lastUserContent, "doing") ||
				strings.Contains(lastUserContent, "running") ||
				strings.Contains(lastUserContent, "using") ||
				strings.Contains(lastUserContent, "usage")

			if hasMyInfra && hasStateQuery {
				return true // Explicit state query about user's infrastructure
			}

			// Exception: explicit resource references should trigger tools even in "tell me about" queries.
			resourceNouns := []string{
				"container", "vm", "lxc", "node", "pod", "deployment", "service", "host", "cluster",
			}
			hasResourceNoun := false
			for _, noun := range resourceNouns {
				if strings.Contains(lastUserContent, noun) {
					hasResourceNoun = true
					break
				}
			}
			explicitIndicator := strings.Contains(lastUserContent, "@") ||
				strings.Contains(lastUserContent, "\"") ||
				strings.Contains(lastUserContent, "-") ||
				strings.Contains(lastUserContent, "_") ||
				strings.Contains(lastUserContent, "/")

			if hasResourceNoun && explicitIndicator {
				return true // Treat as action: specific resource is referenced
			}

			return false
		}
	}

	// Pattern 1: @mentions indicate infrastructure references
	if strings.Contains(lastUserContent, "@") {
		return true
	}

	// Pattern 2: Action verbs that require live data
	// These are more specific to avoid matching conceptual discussions
	actionPatterns := []string{
		// Direct action commands
		"restart ", "start ", "stop ", "reboot ", "shutdown ",
		"kill ", "terminate ",
		// Status checks (specific phrasing)
		"check ", "check the", "status of", "is it running", "is it up", "is it down",
		"is running", "is stopped", "is down",
		// "is X running?" pattern
		" running?", " up?", " down?", " stopped?",
		// Live data queries
		"show me the", "list my", "list the", "list all",
		"what's the cpu", "what's the memory", "what's the disk",
		"cpu usage", "memory usage", "disk usage", "storage usage",
		"how much memory", "how much cpu", "how much disk",
		// Logs and debugging
		"show logs", "show the logs", "check logs", "view logs",
		"why is my", "why did my", "troubleshoot my", "debug my", "diagnose my",
		// Discovery of MY resources
		"where is my", "which of my", "find my",
		// Questions about "my" specific infrastructure
		"my server", "my container", "my vm", "my host", "my infrastructure",
		"my node", "my cluster", "my proxmox", "my docker",
		// Inventory-style queries
		"what nodes do i have", "what proxmox nodes",
		"what containers do i have", "what vms do i have",
		"what is running on", "what's running on",
	}

	for _, pattern := range actionPatterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	// Logs or journal queries should always hit tools.
	if strings.Contains(lastUserContent, "logs") ||
		strings.Contains(lastUserContent, " log") ||
		strings.Contains(lastUserContent, "journal") ||
		strings.Contains(lastUserContent, "journald") {
		return true
	}

	// Default: assume conceptual question, don't force tools
	return false
}

// getPreferredTool returns a tool name if the user explicitly requested one.
// Only returns tools that are available for this request.
func getPreferredTool(messages []providers.Message, tools []providers.Tool) string {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return ""
	}

	toolSet := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			toolSet[tool.Name] = true
		}
	}

	// Explicit tool mentions
	explicitTools := []string{
		"pulse_read",
		"pulse_control",
		"pulse_query",
		"pulse_discovery",
		"pulse_docker",
		"pulse_kubernetes",
		"pulse_metrics",
		"pulse_storage",
	}
	for _, tool := range explicitTools {
		if strings.Contains(lastUserContent, tool) && toolSet[tool] {
			return tool
		}
	}

	// Natural language aliases
	if (strings.Contains(lastUserContent, "read-only tool") || strings.Contains(lastUserContent, "read only tool")) && toolSet["pulse_read"] {
		return "pulse_read"
	}
	if strings.Contains(lastUserContent, "control tool") && toolSet["pulse_control"] {
		return "pulse_control"
	}
	if strings.Contains(lastUserContent, "query tool") && toolSet["pulse_query"] {
		return "pulse_query"
	}

	// Context carryover: if we injected an explicit target and logs are requested, force pulse_read.
	if strings.Contains(lastUserContent, "explicit target") &&
		(strings.Contains(lastUserContent, "log") || strings.Contains(lastUserContent, "journal")) &&
		toolSet["pulse_read"] {
		return "pulse_read"
	}

	return ""
}

// isSingleToolRequest detects user instructions to use exactly one tool call.
func isSingleToolRequest(messages []providers.Message) bool {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	patterns := []string{
		"only that tool once",
		"only this tool once",
		"call only that tool once",
		"call only this tool once",
		"call only that tool",
		"call only this tool",
		"call only one tool",
		"only one tool",
		"single tool",
		"use only that tool",
		"use only this tool",
		"do not call any other tools",
		"don't call any other tools",
		"no other tools",
	}

	for _, pattern := range patterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	return false
}

// hasWriteIntent checks if the user's message contains explicit write/control intent.
// Returns true if the user is asking for an action (stop, start, restart, run command, etc.).
// Returns false if the intent is read-only (status check, logs, monitoring).
// This is used to structurally block control tools on read-only requests.
func hasWriteIntent(messages []providers.Message) bool {
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].ToolResult == nil {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	// Explicit write/control action verbs
	writePatterns := []string{
		"stop ", "start ", "restart ", "reboot ", "shutdown ", "shut down",
		"kill ", "terminate ",
		"turn off", "turn on", "bring up", "bring down", "bring back",
		"run command", "run the command", "execute ",
		"using the control tool", "use pulse_control",
		"using pulse_control",
		// File editing
		"edit ", "modify ", "change ", "update ", "write ",
		"use pulse_file_edit",
	}

	for _, pattern := range writePatterns {
		if strings.Contains(lastUserContent, pattern) {
			return true
		}
	}

	return false
}

// isWriteTool returns true if the tool name is a write/control tool that modifies infrastructure.
func isWriteTool(name string) bool {
	switch name {
	case "pulse_control", "pulse_docker", "pulse_file_edit":
		return true
	default:
		return false
	}
}

// getSystemPrompt builds the full system prompt including the current mode context.
// This is called at request time so the prompt reflects the current mode.
func (a *AgenticLoop) getSystemPrompt() string {
	a.mu.Lock()
	isAutonomous := a.autonomousMode
	a.mu.Unlock()

	var modeContext string
	if isAutonomous {
		modeContext = `
EXECUTION MODE: Autonomous
Commands execute immediately without user approval. Follow the Discover → Investigate → Act
workflow. Gather information before taking action. Use the tools freely to explore logs, check
status, and understand the situation before attempting fixes.`
	} else {
		modeContext = `
EXECUTION MODE: Controlled
Commands require user approval before execution. The system handles this automatically via a
confirmation prompt - you don't need to ask "Would you like me to...?" Just execute what's
needed and the system will prompt the user to approve if required.`
	}

	prompt := a.baseSystemPrompt + modeContext

	// Append accumulated knowledge facts to system prompt
	if ka := a.knowledgeAccumulator; ka != nil && ka.Len() > 0 {
		prompt += "\n\n" + ka.Render()
	}

	return prompt
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
	log.Warn().
		Int("total_messages", len(messages)).
		Int("limit", MaxContextMessagesLimit).
		Int("dropped", start).
		Msg("[AgenticLoop] Pruning oldest messages to fit context limit")
	pruned := messages[start:]

	// Skip leading tool results (orphaned from pruned tool calls)
	for len(pruned) > 0 && pruned[0].ToolResult != nil {
		pruned = pruned[1:]
	}

	// If we start with an assistant message that has tool calls,
	// skip it and its following tool results — we've pruned the
	// user message that preceded it, so the sequence is broken.
	for len(pruned) > 0 && pruned[0].Role == "assistant" && len(pruned[0].ToolCalls) > 0 {
		pruned = pruned[1:]
		// Also skip the tool results that followed
		for len(pruned) > 0 && pruned[0].ToolResult != nil {
			pruned = pruned[1:]
		}
	}

	return pruned
}

func truncateToolResultForModel(text string) string {
	if MaxToolResultCharsLimit <= 0 || len(text) <= MaxToolResultCharsLimit {
		return text
	}

	truncated := text[:MaxToolResultCharsLimit]
	truncatedChars := len(text) - MaxToolResultCharsLimit
	log.Warn().
		Int("original_chars", len(text)).
		Int("truncated_to", MaxToolResultCharsLimit).
		Int("chars_cut", truncatedChars).
		Msg("[AgenticLoop] Truncating oversized tool result")
	return fmt.Sprintf("%s\n\n---\n[TRUNCATED: %d characters cut. The result was too large. If you need specific details that may have been cut, make a more targeted query (e.g., filter by specific resource or type).]", truncated, truncatedChars)
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

// compactOldToolResults replaces full tool result content with short summaries
// for tool results from older turns. This prevents context window blowout during
// long agentic loops (e.g., patrol runs with 20+ tool calls).
//
// Only tool results before currentTurnStartIndex are candidates for compaction.
// Results from the most recent keepTurns turns are kept in full.
// Results shorter than minChars are not compacted (not worth it).
//
// The model retains all its assistant messages (reasoning, analysis, findings) in full.
// Only the raw tool result data from older turns gets replaced with a summary line.
func compactOldToolResults(messages []providers.Message, currentTurnStartIndex, keepTurns, minChars int, ka *KnowledgeAccumulator) {
	if currentTurnStartIndex <= 0 || keepTurns < 0 {
		return
	}

	// Walk backwards from currentTurnStartIndex to find the compaction boundary.
	// We keep the last keepTurns turns' tool results in full. Each "turn" starts
	// with an assistant message. Once we've skipped keepTurns assistant messages,
	// everything before that point is old enough to compact.
	var compactBefore int
	if keepTurns <= 0 {
		// Compact everything before the current turn
		compactBefore = currentTurnStartIndex
	} else {
		turnsFound := 0
		for i := currentTurnStartIndex - 1; i >= 0; i-- {
			if messages[i].Role == "assistant" {
				turnsFound++
				if turnsFound >= keepTurns {
					// This is the keepTurns-th assistant message from the end.
					// Everything before this index is old enough to compact.
					compactBefore = i
					break
				}
			}
		}
	}

	// Nothing old enough to compact
	if compactBefore <= 0 {
		return
	}

	// Build a map of tool call ID -> (name, input) from assistant messages,
	// so we can label compacted results with the tool name and key params.
	toolCallInfo := make(map[string]struct {
		Name  string
		Input map[string]interface{}
	})
	for i := 0; i < compactBefore; i++ {
		msg := messages[i]
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				toolCallInfo[tc.ID] = struct {
					Name  string
					Input map[string]interface{}
				}{Name: tc.Name, Input: tc.Input}
			}
		}
	}

	// Compact tool results before the boundary
	compacted := 0
	savedChars := 0
	for i := 0; i < compactBefore; i++ {
		msg := &messages[i]
		if msg.ToolResult == nil || msg.ToolResult.IsError {
			continue
		}
		content := msg.ToolResult.Content
		if len(content) < minChars {
			continue
		}

		// Build summary
		toolName := "unknown_tool"
		var toolInput map[string]interface{}
		if info, ok := toolCallInfo[msg.ToolResult.ToolUseID]; ok {
			toolName = info.Name
			toolInput = info.Input
		}

		summary := buildCompactSummary(toolName, toolInput, content, ka, msg.ToolResult.ToolUseID)
		savedChars += len(content) - len(summary)
		msg.ToolResult.Content = summary
		compacted++
	}

	if compacted > 0 {
		log.Info().
			Int("compacted_results", compacted).
			Int("saved_chars", savedChars).
			Int("compact_before_index", compactBefore).
			Int("total_messages", len(messages)).
			Msg("[AgenticLoop] Compacted old tool results to reduce context size")
	}
}

// buildCompactSummary creates a short summary line for a compacted tool result.
// When a KnowledgeAccumulator is provided and has facts for this tool_use_id,
// the summary includes those facts so the model knows what it learned.
func buildCompactSummary(toolName string, toolInput map[string]interface{}, originalContent string, ka *KnowledgeAccumulator, toolUseID string) string {
	params := formatKeyParams(toolInput)
	charCount := len(originalContent)

	// Try to include KA facts for this specific tool call
	if ka != nil && toolUseID != "" {
		if factSummary := ka.FactSummaryForTool(toolUseID); factSummary != "" {
			var summary string
			if params != "" {
				summary = fmt.Sprintf("[Compacted: %s(%s) — Key facts: %s]",
					toolName, params, factSummary)
			} else {
				summary = fmt.Sprintf("[Compacted: %s — Key facts: %s]",
					toolName, factSummary)
			}
			log.Info().
				Str("tool", toolName).
				Str("tool_use_id", toolUseID).
				Int("original_chars", charCount).
				Int("summary_chars", len(summary)).
				Msg("[SmartCompaction] Used KA facts for compacted summary")
			return summary
		}
	}

	// Fallback: generic format when no KA facts available
	lineCount := strings.Count(originalContent, "\n") + 1
	if params != "" {
		return fmt.Sprintf("[Tool result compacted: %s(%s) — %d chars, %d lines. Full data was provided to the model in an earlier turn and has already been processed.]",
			toolName, params, charCount, lineCount)
	}
	return fmt.Sprintf("[Tool result compacted: %s — %d chars, %d lines. Full data was provided to the model in an earlier turn and has already been processed.]",
		toolName, charCount, lineCount)
}

// maybeInjectWrapUpNudge appends a system hint to the last non-error tool result in
// providerMessages when totalCalls exceeds the threshold. This nudges the model to
// start wrapping up without forcing text-only mode.
// Returns true if a nudge was injected.
func maybeInjectWrapUpNudge(messages []providers.Message, totalCalls, maxTurns, currentTurn, threshold int) bool {
	if totalCalls < threshold {
		return false
	}

	turnsRemaining := maxTurns - currentTurn - 1
	nudge := fmt.Sprintf("\n\n[System: You have made %d tool calls (%d turns remaining). You likely have enough data to answer. Start forming your response. You may make 1-2 more targeted calls if critical information is missing, but avoid exploratory calls.]",
		totalCalls, turnsRemaining)

	// Find the last non-error tool result in messages and append the nudge
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].ToolResult != nil && !messages[i].ToolResult.IsError {
			messages[i].ToolResult.Content += nudge
			log.Info().
				Int("total_calls", totalCalls).
				Int("turns_remaining", turnsRemaining).
				Int("message_index", i).
				Msg("[WrapUpNudge] Injected wrap-up nudge into tool result")
			return true
		}
	}
	return false
}

// maybeInjectWrapUpEscalation appends a strong wrap-up directive to the last non-error
// tool result. Called once when the model ignores the initial nudge and reaches 18+ calls.
// Returns true if an escalation was injected.
func maybeInjectWrapUpEscalation(messages []providers.Message, totalCalls int) bool {
	escalation := fmt.Sprintf("\n\n[System: WRAP UP NOW. You have made %d tool calls — well past the recommended limit. You MUST respond with your findings on this turn. Do NOT make any more tool calls.]",
		totalCalls)

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].ToolResult != nil && !messages[i].ToolResult.IsError {
			messages[i].ToolResult.Content += escalation
			log.Info().
				Int("total_calls", totalCalls).
				Int("message_index", i).
				Msg("[WrapUpEscalation] Injected wrap-up escalation into tool result")
			return true
		}
	}
	return false
}

// formatKeyParams extracts the most important parameters from tool input for display.
func formatKeyParams(input map[string]interface{}) string {
	if len(input) == 0 {
		return ""
	}

	// Priority keys that are most informative
	priorityKeys := []string{"type", "resource_id", "action", "host", "node", "instance", "query", "command", "period"}
	var parts []string

	for _, key := range priorityKeys {
		if val, ok := input[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, str))
			}
		}
	}

	// If nothing from priority keys, take the first 2 non-empty string values
	if len(parts) == 0 {
		for key, val := range input {
			if str, ok := val.(string); ok && str != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, str))
				if len(parts) >= 2 {
					break
				}
			}
		}
	}

	return strings.Join(parts, ", ")
}
