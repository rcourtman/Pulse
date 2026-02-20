package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
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

func isRetryableProviderStreamError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}

	retryable := []string{
		"timeout",
		"timed out",
		"context deadline exceeded",
		"connection reset",
		"connection refused",
		"connection aborted",
		"broken pipe",
		"eof",
		"unexpected eof",
		"network is unreachable",
		"no such host",
		"dial tcp",
		"temporarily unavailable",
		"http2: client connection lost",
		"stream error",
		"upstream",
		"502",
		"503",
		"504",
	}

	for _, token := range retryable {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func fallbackProviderStreamErrorMessage(err error) string {
	const defaultMessage = "AI response stream interrupted before completion. Please retry."
	if err == nil {
		return defaultMessage
	}

	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return defaultMessage
	}

	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "context canceled"):
		return "AI response was canceled before completion."
	case strings.Contains(msgLower, "timeout"),
		strings.Contains(msgLower, "timed out"),
		strings.Contains(msgLower, "deadline exceeded"):
		return "AI response timed out before completion. Please retry."
	}

	const maxLen = 220
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}
	return "AI response stream interrupted: " + msg
}

func (a *AgenticLoop) executeToolSafely(ctx context.Context, name string, input map[string]interface{}) (result tools.CallToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := fmt.Errorf("tool panic in %s: %v", name, recovered)
			log.Error().
				Str("tool", name).
				Interface("panic", recovered).
				Bytes("stack", debug.Stack()).
				Msg("[AgenticLoop] Recovered tool panic")
			result = tools.NewErrorResult(panicErr)
			err = panicErr
		}
	}()
	return a.executor.ExecuteTool(ctx, name, input)
}

// AgenticLoop handles the tool-calling loop with streaming
type AgenticLoop struct {
	provider         providers.StreamingProvider
	executor         *tools.PulseToolExecutor
	tools            []providers.Tool
	baseSystemPrompt string // Base prompt without mode context
	maxTurns         int
	orgID            string

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
	providerTools = append(providerTools, userQuestionTool())

	return &AgenticLoop{
		provider:         provider,
		executor:         executor,
		tools:            providerTools,
		baseSystemPrompt: systemPrompt,
		maxTurns:         MaxAgenticTurns,
		orgID:            approval.DefaultOrgID,
		aborted:          make(map[string]bool),
		pendingQs:        make(map[string]chan []QuestionAnswer),
	}
}

// UpdateTools refreshes the tool list from the executor
func (a *AgenticLoop) UpdateTools() {
	mcpTools := a.executor.ListTools()
	tools := ConvertMCPToolsToProvider(mcpTools)
	a.tools = append(tools, userQuestionTool())
}

// SetOrgID sets the org scope used when validating approval decisions.
func (a *AgenticLoop) SetOrgID(orgID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.orgID = approval.NormalizeOrgID(orgID)
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
	const maxConsecutiveToolOnlyTurns = 4
	totalToolCalls := 0
	wrapUpNudgeFired := false
	wrapUpEscalateFired := false
	consecutiveToolOnlyTurns := 0
	consecutiveAllErrorTurns := 0

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

		const maxProviderAttempts = 2
		err := error(nil)
		for attempt := 1; attempt <= maxProviderAttempts; attempt++ {
			attemptSawDone := false
			attemptEmittedVisibleEvents := false
			var attemptErrorMessages []string

			err = a.provider.ChatStream(ctx, req, func(event providers.StreamEvent) {
				switch event.Type {
				case "content":
					if data, ok := event.Data.(providers.ContentEvent); ok {
						attemptEmittedVisibleEvents = true
						// Check for DeepSeek DSML marker - if detected, stop streaming this chunk
						// The DSML format indicates the model is outputting internal function call
						// formatting instead of using the proper tool calling API
						if containsDeepSeekMarker(data.Text) {
							// Don't append or stream this content
							return
						}
						// Also check if the accumulated content already has the marker
						// (in case it arrived in a previous chunk)
						if containsDeepSeekMarker(contentBuilder.String()) {
							return
						}
						contentBuilder.WriteString(data.Text)
						// Forward to callback - send ContentData struct
						jsonData, _ := json.Marshal(ContentData{Text: data.Text})
						callback(StreamEvent{Type: "content", Data: jsonData})
					}

				case "thinking":
					if data, ok := event.Data.(providers.ThinkingEvent); ok {
						attemptEmittedVisibleEvents = true
						thinkingBuilder.WriteString(data.Text)
						// Forward to callback - send ThinkingData struct
						jsonData, _ := json.Marshal(ThinkingData{Text: data.Text})
						callback(StreamEvent{Type: "thinking", Data: jsonData})
					}

				case "tool_start":
					if data, ok := event.Data.(providers.ToolStartEvent); ok {
						attemptEmittedVisibleEvents = true
						// pulse_question is rendered as a dedicated "question" card; suppress tool UI events.
						if data.Name == pulseQuestionToolName {
							return
						}
						// Format input for frontend display
						// For control tools, show a human-readable summary instead of raw JSON to avoid "hallucination" look
						inputStr, rawInput := formatToolInputForFrontend(data.Name, data.Input, false)
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
						attemptSawDone = true
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
						if msg := strings.TrimSpace(data.Message); msg != "" {
							attemptErrorMessages = append(attemptErrorMessages, msg)
						}
					}
				}
			})

			effectiveErr := err
			if effectiveErr == nil && len(attemptErrorMessages) > 0 {
				effectiveErr = fmt.Errorf("stream error: %s", attemptErrorMessages[0])
			}
			if effectiveErr == nil {
				break
			}
			if attemptSawDone {
				// Some providers can return a transport error after already emitting
				// a terminal done event. The turn is complete; keep the response.
				log.Warn().
					Int("attempt", attempt).
					Err(effectiveErr).
					Str("session_id", sessionID).
					Msg("[AgenticLoop] Provider returned error after done event; ignoring")
				err = nil
				break
			}
			if attempt < maxProviderAttempts && !attemptEmittedVisibleEvents && isRetryableProviderStreamError(effectiveErr) && ctx.Err() == nil {
				backoff := time.Duration(200*attempt) * time.Millisecond
				log.Warn().
					Int("attempt", attempt).
					Err(effectiveErr).
					Dur("backoff", backoff).
					Str("session_id", sessionID).
					Msg("[AgenticLoop] Provider stream failed before events; retrying turn")
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					err = ctx.Err()
					break
				}
				continue
			}

			if len(attemptErrorMessages) > 0 {
				// Defer emitting error events until retries are exhausted so transient
				// provider blips don't leak to the client stream.
				jsonData, _ := json.Marshal(ErrorData{Message: attemptErrorMessages[0]})
				callback(StreamEvent{Type: "error", Data: jsonData})
			} else {
				// Transport-level failures may not include an explicit provider error event.
				// Emit a fallback error so clients can render a clear failure state.
				jsonData, _ := json.Marshal(ErrorData{Message: fallbackProviderStreamErrorMessage(effectiveErr)})
				callback(StreamEvent{Type: "error", Data: jsonData})
			}
			err = effectiveErr
			break
		}

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
		// Clean DeepSeek artifacts from the content before storing
		cleanedContent := cleanDeepSeekArtifacts(contentBuilder.String())
		assistantMsg := Message{
			ID:               uuid.New().String(),
			Role:             "assistant",
			Content:          cleanedContent,
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

		// Track turns where the model emitted tool calls but no user-facing text.
		// In chat mode this can otherwise spiral into long tool-only chains.
		if len(toolCalls) > 0 && strings.TrimSpace(assistantMsg.Content) == "" {
			consecutiveToolOnlyTurns++
		} else {
			consecutiveToolOnlyTurns = 0
		}

		// If no tool calls, we're done - but first check FSM and phantom execution
		if len(toolCalls) == 0 {
			// No tool calls breaks the "consecutive all-error tool turns" streak.
			consecutiveAllErrorTurns = 0

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

			log.Debug().Msg("agentic loop complete - no tool calls")
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
		anyToolSucceededThisTurn := false

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

		// pulse_question is interactive and must not run in parallel with other tools.
		// If the provider emits multiple tool calls alongside pulse_question, skip the
		// others and let the model retry after receiving the user's answer.
		hasPulseQuestion := false
		for _, tc := range toolCalls {
			if tc.Name == pulseQuestionToolName {
				hasPulseQuestion = true
				break
			}
		}
		if hasPulseQuestion {
			for _, tc := range toolCalls {
				log.Debug().
					Str("tool", tc.Name).
					Str("id", tc.ID).
					Msg("Processing interactive tool call set (pulse_question present)")

				toolKind := ClassifyToolCall(tc.Name, tc.Input)

				// FSM enforcement still applies (even if we're skipping execution).
				if fsm != nil {
					if fsmErr := fsm.CanExecuteTool(toolKind, tc.Name); fsmErr != nil {
						log.Warn().
							Str("tool", tc.Name).
							Str("kind", toolKind.String()).
							Str("state", string(fsm.State)).
							Err(fsmErr).
							Msg("[AgenticLoop] FSM blocked tool execution (interactive set)")

						fsmBlockedErr, ok := fsmErr.(*FSMBlockedError)
						if ok && fsmBlockedErr.Recoverable {
							fsm.TrackPendingRecovery("FSM_BLOCKED", tc.Name)
							if metrics := GetAIMetrics(); metrics != nil {
								metrics.RecordAutoRecoveryAttempt("FSM_BLOCKED", tc.Name)
							}
						}

						inputStr, rawInput := formatToolInputForFrontend(tc.Name, tc.Input, true)
						jsonData, _ := json.Marshal(ToolEndData{
							ID:       tc.ID,
							Name:     tc.Name,
							Input:    inputStr,
							RawInput: rawInput,
							Output:   fsmErr.Error(),
							Success:  false,
						})
						callback(StreamEvent{Type: "tool_end", Data: jsonData})

						toolResultMsg := Message{
							ID:        uuid.New().String(),
							Role:      "user",
							Timestamp: time.Now(),
							ToolResult: &ToolResult{
								ToolUseID: tc.ID,
								Content:   fsmErr.Error(),
								IsError:   true,
							},
						}
						resultMessages = append(resultMessages, toolResultMsg)
						providerMessages = append(providerMessages, providers.Message{
							Role: "user",
							ToolResult: &providers.ToolResult{
								ToolUseID: tc.ID,
								Content:   truncateToolResultForModel(fsmErr.Error()),
								IsError:   true,
							},
						})
						continue
					}
				}

				// LOOP DETECTION
				callKey := toolCallKey(tc.Name, tc.Input)
				recentCallCounts[callKey]++
				if recentCallCounts[callKey] > maxIdenticalCalls {
					loopMsg := fmt.Sprintf("LOOP_DETECTED: You have called %s with the same arguments %d times. This call is blocked. Try a different tool or approach.", tc.Name, recentCallCounts[callKey])

					inputStr, rawInput := formatToolInputForFrontend(tc.Name, tc.Input, true)
					jsonData, _ := json.Marshal(ToolEndData{
						ID:       tc.ID,
						Name:     tc.Name,
						Input:    inputStr,
						RawInput: rawInput,
						Output:   loopMsg,
						Success:  false,
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
							Content:   truncateToolResultForModel(loopMsg),
							IsError:   true,
						},
					})
					continue
				}

				// Skip non-question tools in this turn; the model must retry after user input.
				if tc.Name != pulseQuestionToolName {
					skipMsg := fmt.Sprintf("SKIPPED: %s was requested this turn. Wait for the user's answer, then re-issue this tool call with the clarified inputs.", pulseQuestionToolName)

					inputStr, rawInput := formatToolInputForFrontend(tc.Name, tc.Input, true)
					jsonData, _ := json.Marshal(ToolEndData{
						ID:       tc.ID,
						Name:     tc.Name,
						Input:    inputStr,
						RawInput: rawInput,
						Output:   skipMsg,
						Success:  false,
					})
					callback(StreamEvent{Type: "tool_end", Data: jsonData})

					toolResultMsg := Message{
						ID:        uuid.New().String(),
						Role:      "user",
						Timestamp: time.Now(),
						ToolResult: &ToolResult{
							ToolUseID: tc.ID,
							Content:   skipMsg,
							IsError:   true,
						},
					}
					resultMessages = append(resultMessages, toolResultMsg)
					providerMessages = append(providerMessages, providers.Message{
						Role: "user",
						ToolResult: &providers.ToolResult{
							ToolUseID: tc.ID,
							Content:   truncateToolResultForModel(skipMsg),
							IsError:   true,
						},
					})
					continue
				}

				// Execute the interactive question tool synchronously (blocks until user answers).
				resultText, isError := a.executeQuestionTool(ctx, sessionID, tc, callback)

				// For pulse_question we suppress tool_end on success (UI uses the question card).
				if isError {
					inputStr, rawInput := formatToolInputForFrontend(tc.Name, tc.Input, true)
					jsonData, _ := json.Marshal(ToolEndData{
						ID:       tc.ID,
						Name:     tc.Name,
						Input:    inputStr,
						RawInput: rawInput,
						Output:   resultText,
						Success:  false,
					})
					callback(StreamEvent{Type: "tool_end", Data: jsonData})
				}

				if fsm != nil && !isError {
					fsm.OnToolSuccess(toolKind, tc.Name)
				}

				toolResultMsg := Message{
					ID:        uuid.New().String(),
					Role:      "user",
					Timestamp: time.Now(),
					ToolResult: &ToolResult{
						ToolUseID: tc.ID,
						Content:   resultText,
						IsError:   isError,
					},
				}
				resultMessages = append(resultMessages, toolResultMsg)

				providerMessages = append(providerMessages, providers.Message{
					Role: "user",
					ToolResult: &providers.ToolResult{
						ToolUseID: tc.ID,
						Content:   truncateToolResultForModel(resultText),
						IsError:   isError,
					},
				})
			}

			// Mark the start of the next turn's messages for compaction tracking.
			currentTurnStartIndex = len(providerMessages)
			turn++
			continue
		}

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
					if ok && fsmBlockedErr.Recoverable {
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
						Output:  fsmErr.Error(),
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
							Content:   fsmErr.Error(),
							IsError:   true,
						},
					}
					resultMessages = append(resultMessages, toolResultMsg)

					// Add to provider messages for next turn
					providerMessages = append(providerMessages, providers.Message{
						Role: "user",
						ToolResult: &providers.ToolResult{
							ToolUseID: tc.ID,
							Content:   fsmErr.Error(),
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
						anyToolSucceededThisTurn = true

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
					r, e := a.executeToolSafely(ctx, tc.Name, tc.Input)
					execResults[idx] = parallelToolResult{Result: r, Err: e}
				}(j, pe.tc)
			}
			wg.Wait()
		} else if len(pendingExec) == 1 {
			r, e := a.executeToolSafely(ctx, pendingExec[0].tc.Name, pendingExec[0].tc.Input)
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

					retryResult, retryErr := a.executeToolSafely(ctx, tc.Name, modifiedInput)
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
					log.Error().Err(err).Str("data", approvalJSON).Msg("failed to parse approval request")
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
					loopOrgID := approval.NormalizeOrgID(a.orgID)
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
							decision, waitErr := waitForApprovalDecision(approvalCtx, store, approvalData.ApprovalID, loopOrgID)
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
								result, err = a.executeToolSafely(ctx, tc.Name, inputWithApproval)
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

			if !isError {
				anyToolSucceededThisTurn = true
			}

			// Send tool_end event
			// Convert input to JSON string for frontend display
			inputStr, rawInput := formatToolInputForFrontend(tc.Name, tc.Input, true)
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

				// If a write tool includes self-verification evidence, we can satisfy
				// the "verify after write" invariant without requiring a separate read
				// tool call (which may be stale depending on reporting cadence).
				//
				// Verification evidence is a structured field in the tool output:
				//   { "verification": { "ok": true, ... } }
				if toolKind == ToolKindWrite && toolResultHasVerificationOK(resultText) {
					fsm.OnToolSuccess(ToolKindRead, "self_verify")
					if fsm.State == StateVerifying && fsm.ReadAfterWrite {
						fsm.CompleteVerification()
					}
					writeCompletedLastTurn = true
					log.Info().
						Str("tool", tc.Name).
						Str("new_state", string(fsm.State)).
						Msg("[AgenticLoop] Write tool provided verification evidence; FSM verification satisfied")
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

		// Track consecutive turns where ALL tool calls failed/were blocked.
		// This catches stuck models that vary arguments to bypass identical-call detection.
		{
			if anyToolSucceededThisTurn {
				consecutiveAllErrorTurns = 0
			} else {
				consecutiveAllErrorTurns++
				if consecutiveAllErrorTurns >= 3 {
					toolBlockedLastTurn = true
					log.Warn().
						Int("consecutive_all_error_turns", consecutiveAllErrorTurns).
						Int("turn", turn).
						Str("session_id", sessionID).
						Msg("[AgenticLoop] All tool calls failed for 3 consecutive turns — forcing text-only response")
				}
			}
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

		// Guardrail: in interactive chat mode, force wrap-up after repeated
		// tool-only turns to avoid long no-answer chains.
		a.mu.Lock()
		autonomousMode := a.autonomousMode
		a.mu.Unlock()
		if !autonomousMode && !singleToolRequested && consecutiveToolOnlyTurns >= maxConsecutiveToolOnlyTurns {
			toolBlockedLastTurn = true
			log.Warn().
				Int("consecutive_tool_only_turns", consecutiveToolOnlyTurns).
				Int("turn", turn).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Consecutive tool-only turns exceeded threshold — forcing text-only response")
		}

		if singleToolEnforced && len(toolCalls) > 0 {
			// Single tool request completed - ensure we have a proper response
			// Don't just return raw tool output, let ensureFinalTextResponse synthesize if needed
			resultMessages = a.ensureFinalTextResponse(ctx, sessionID, resultMessages, providerMessages, callback)
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

	log.Warn().Int("max_turns", maxTurns).Str("session_id", sessionID).Msg("agentic loop hit max turns limit")
	resultMessages = a.ensureFinalTextResponse(ctx, sessionID, resultMessages, providerMessages, callback)
	return resultMessages, nil
}
