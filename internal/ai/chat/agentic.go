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
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

func emitWorkflowState(callback StreamCallback, phase, message, state, tool string) {
	if callback == nil {
		return
	}
	jsonData, _ := json.Marshal(WorkflowStateData{
		Phase:   phase,
		Message: message,
		State:   state,
		Tool:    tool,
	})
	callback(StreamEvent{Type: "workflow_state", Data: jsonData})
}

func sessionFSMState(fsm *SessionFSM) string {
	if fsm == nil {
		return ""
	}
	return string(fsm.State)
}

const defaultProviderStreamErrorMessage = "AI response stream interrupted before completion. Please retry."

func fallbackProviderStreamErrorMessage(err error) string {
	if err == nil {
		return defaultProviderStreamErrorMessage
	}
	return sanitizeProviderStreamErrorForUser(err.Error())
}

func currentResourceUnavailableToolResult(err error) string {
	reason := "current_resource is unavailable because no single attached Pulse resource is selected in this chat turn"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		reason = strings.TrimSpace(err.Error())
	}
	return fmt.Sprintf(
		"CURRENT_RESOURCE_UNAVAILABLE: %s. Ask which host, VM, container, app, or storage resource the user means before calling resource-targeted tools. Do not retry this tool with current_resource until the user provides or opens a specific resource context.",
		reason,
	)
}

func toolInputContainsCurrentResourceReference(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return tools.IsCurrentResourceReference(v)
	case map[string]interface{}:
		for _, child := range v {
			if toolInputContainsCurrentResourceReference(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range v {
			if toolInputContainsCurrentResourceReference(child) {
				return true
			}
		}
	case []string:
		for _, child := range v {
			if tools.IsCurrentResourceReference(child) {
				return true
			}
		}
	}
	return false
}

func (a *AgenticLoop) currentResourcePlaceholderBlock(tc providers.ToolCall) (string, bool) {
	if !toolInputContainsCurrentResourceReference(tc.Input) {
		return "", false
	}
	if a == nil || a.executor == nil {
		return currentResourceUnavailableToolResult(nil), true
	}
	if err := a.executor.ValidateCurrentResourceAvailable(); err != nil {
		return currentResourceUnavailableToolResult(err), true
	}
	return "", false
}

func removeToolCallFromResultMessages(messages []Message, toolUseID string) []Message {
	toolUseID = strings.TrimSpace(toolUseID)
	if toolUseID == "" {
		return messages
	}
	for idx := len(messages) - 1; idx >= 0; idx-- {
		if len(messages[idx].ToolCalls) == 0 {
			continue
		}
		filtered := messages[idx].ToolCalls[:0]
		for _, tc := range messages[idx].ToolCalls {
			if strings.TrimSpace(tc.ID) != toolUseID {
				filtered = append(filtered, tc)
			}
		}
		if len(filtered) == len(messages[idx].ToolCalls) {
			continue
		}
		messages[idx].ToolCalls = filtered
		if len(messages[idx].ToolCalls) == 0 && strings.TrimSpace(messages[idx].Content) == "" && messages[idx].ToolResult == nil {
			return append(messages[:idx], messages[idx+1:]...)
		}
		return messages
	}
	return messages
}

func toolStartKey(id, name string) string {
	if key := strings.TrimSpace(id); key != "" {
		return key
	}
	return strings.TrimSpace(name)
}

func toolStartKeys(id, name string) []string {
	keys := make([]string, 0, 2)
	if key := strings.TrimSpace(id); key != "" {
		keys = append(keys, key)
	}
	if key := strings.TrimSpace(name); key != "" {
		for _, existing := range keys {
			if existing == key {
				return keys
			}
		}
		keys = append(keys, key)
	}
	return keys
}

func toolStartMapHas(values map[string]bool, id, name string) bool {
	for _, key := range toolStartKeys(id, name) {
		if values[key] {
			return true
		}
	}
	return false
}

func markToolStartMap(values map[string]bool, id, name string) {
	for _, key := range toolStartKeys(id, name) {
		values[key] = true
	}
}

func normalizeSummaryOnlyPulseQueryInput(prefer bool, name string, input map[string]interface{}) map[string]interface{} {
	if !prefer || name != "pulse_query" || len(input) == 0 {
		return input
	}
	action, _ := input["action"].(string)
	if strings.ToLower(strings.TrimSpace(action)) != "topology" {
		return input
	}
	if _, exists := input["summary_only"]; exists {
		return input
	}
	normalized := make(map[string]interface{}, len(input)+1)
	for key, value := range input {
		normalized[key] = value
	}
	normalized["summary_only"] = true
	return normalized
}

func emitToolStartEvent(callback StreamCallback, id, name string, input map[string]interface{}) {
	if callback == nil {
		return
	}
	inputStr, rawInput := formatToolInputForFrontend(name, input, false)
	jsonData, _ := json.Marshal(ToolStartData{
		ID:       id,
		Name:     name,
		Input:    inputStr,
		RawInput: rawInput,
	})
	callback(StreamEvent{Type: "tool_start", Data: jsonData})
}

func emitToolProgressEvent(callback StreamCallback, id, name string, input map[string]interface{}, phase, message string) {
	emitToolProgressEventWithRawInput(callback, id, name, input, "", phase, message)
}

func emitToolProgressEventWithRawInput(callback StreamCallback, id, name string, input map[string]interface{}, rawInputOverride, phase, message string) {
	if callback == nil {
		return
	}
	inputStr, rawInput := formatToolInputForFrontend(name, input, false)
	if rawInputOverride != "" {
		rawInput = rawInputOverride
	}
	jsonData, _ := json.Marshal(ToolProgressData{
		ID:       id,
		Name:     name,
		Input:    inputStr,
		RawInput: rawInput,
		Phase:    phase,
		Message:  message,
	})
	callback(StreamEvent{Type: "tool_progress", Data: jsonData})
}

func emitToolCancelEvent(callback StreamCallback, id, name, reason string) {
	if callback == nil {
		return
	}
	jsonData, _ := json.Marshal(ToolCancelData{
		ID:     id,
		Name:   name,
		Reason: strings.TrimSpace(reason),
	})
	callback(StreamEvent{Type: "tool_cancel", Data: jsonData})
}

func emitToolEndEvent(callback StreamCallback, id, name string, input map[string]interface{}, output string, success bool) {
	if callback == nil {
		return
	}
	inputStr, rawInput := formatToolInputForFrontend(name, input, true)
	jsonData, _ := json.Marshal(ToolEndData{
		ID:       id,
		Name:     name,
		Input:    inputStr,
		RawInput: rawInput,
		Output:   output,
		Success:  success,
	})
	callback(StreamEvent{Type: "tool_end", Data: jsonData})
}

func toolExecutionProgressMessage(toolName string, input map[string]interface{}, toolKind ToolKind) string {
	switch strings.TrimSpace(toolName) {
	case "pulse_run_command", "run_command":
		return "Running command."
	case "pulse_query":
		return "Reading inventory."
	case "pulse_read", "read":
		return "Reading target."
	}
	if isKnownGovernedWriteProgress(toolName, input, toolKind) {
		return "Executing governed action."
	}
	return "Running."
}

func isKnownGovernedWriteProgress(toolName string, input map[string]interface{}, toolKind ToolKind) bool {
	if toolKind != ToolKindWrite {
		return false
	}
	action, _ := input["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	switch strings.TrimSpace(toolName) {
	case "pulse_control", "pulse_control_guest", "pulse_control_docker":
		return true
	case "pulse_alerts":
		return action == "resolve" || action == "dismiss"
	case "pulse_docker":
		return action == "control" || action == "update" || action == "check_updates" || action == "trigger_update"
	case "pulse_kubernetes":
		return action == "scale" || action == "restart" || action == "delete_pod" || action == "exec"
	case "pulse_file_edit":
		return action == "write" || action == "append"
	case "pulse_knowledge":
		return action == "remember" || action == "note" || action == "save"
	case "patrol_report_finding", "patrol_resolve_finding":
		return true
	default:
		return false
	}
}

// sanitizeProviderStreamErrorForUser turns a raw provider/transport error into a
// clean, human message safe to render in the chat. Upstream errors (especially
// from OpenAI-compatible gateways like OpenRouter) embed raw JSON bodies and
// dashboard URLs — e.g. a 402 carries the provider's billing JSON and a
// workspace-key link. Those must never reach the user: they are unreadable and
// can leak provider routing and key fragments. Known classes map to actionable
// messages; anything else has its JSON/URL payload stripped before display.
func sanitizeProviderStreamErrorForUser(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return defaultProviderStreamErrorMessage
	}

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "context canceled"):
		return "AI response was canceled before completion."
	case strings.Contains(lower, "timeout"),
		strings.Contains(lower, "timed out"),
		strings.Contains(lower, "deadline exceeded"):
		return "AI response timed out before completion. Please retry."
	case isProviderEndpointConfigurationError(lower):
		return "The AI provider endpoint is not configured correctly. Check the selected provider URL in Settings, then retry."
	case isProviderTransportError(lower):
		return "Pulse could not reach the AI provider endpoint. Check the selected provider URL and network connection, then retry."
	case isProviderCompletionBudgetLimitError(lower):
		return "The AI provider rejected the request because the requested completion budget exceeds the provider key limit. Increase the key limit, lower the completion budget, or choose another provider."
	case strings.Contains(lower, "402"),
		strings.Contains(lower, "more credits"),
		strings.Contains(lower, "insufficient"),
		strings.Contains(lower, "quota"),
		strings.Contains(lower, "billing"),
		strings.Contains(lower, "payment required"):
		return "The AI provider rejected the request for billing or quota reasons. Check your provider credits or token limit, then retry."
	case strings.Contains(lower, "401"),
		strings.Contains(lower, "403"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "invalid api key"),
		strings.Contains(lower, "authentication"):
		return "The AI provider rejected the credentials. Check your AI provider API key in Settings."
	case strings.Contains(lower, "429"),
		strings.Contains(lower, "rate limit"),
		strings.Contains(lower, "too many requests"):
		return "The AI provider is rate limiting requests. Wait a moment, then retry."
	}

	cleaned := stripProviderErrorPayload(msg)
	if cleaned == "" {
		return "The AI provider returned an error. Please retry."
	}
	return "AI response stream interrupted: " + cleaned
}

func isProviderCompletionBudgetLimitError(lower string) bool {
	hasCompletionBudgetField := strings.Contains(lower, "max_tokens") ||
		strings.Contains(lower, "max completion tokens") ||
		strings.Contains(lower, "completion budget") ||
		strings.Contains(lower, "completion tokens")
	hasAffordabilitySignal := strings.Contains(lower, "requested up to") ||
		strings.Contains(lower, "can only afford") ||
		strings.Contains(lower, "more credits") ||
		strings.Contains(lower, "insufficient") ||
		strings.Contains(lower, "key limit") ||
		strings.Contains(lower, "total limit") ||
		strings.Contains(lower, "token limit")
	return hasCompletionBudgetField && hasAffordabilitySignal ||
		strings.Contains(lower, "fewer max tokens") ||
		strings.Contains(lower, "fewer completion tokens")
}

func isProviderEndpointConfigurationError(lower string) bool {
	return strings.Contains(lower, `post ""`) ||
		strings.Contains(lower, `get ""`) ||
		strings.Contains(lower, "unsupported protocol scheme") ||
		strings.Contains(lower, "missing protocol scheme") ||
		strings.Contains(lower, "no host in request url") ||
		strings.Contains(lower, "invalid url") ||
		strings.Contains(lower, "failed to create request")
}

func isProviderTransportError(lower string) bool {
	return strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "dial tcp") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "network is unreachable") ||
		strings.Contains(lower, "proxyconnect") ||
		strings.Contains(lower, "tls handshake") ||
		strings.Contains(lower, "certificate") ||
		strings.Contains(lower, "x509")
}

// stripProviderErrorPayload removes any embedded JSON body or URL so raw
// provider payloads and links never surface in the chat, keeping just the
// leading human-readable summary (e.g. "API error (500)").
func stripProviderErrorPayload(msg string) string {
	cut := len(msg)
	if i := strings.Index(msg, "{"); i >= 0 && i < cut {
		cut = i
	}
	if i := strings.Index(strings.ToLower(msg), "http"); i >= 0 && i < cut {
		cut = i
	}
	cleaned := strings.TrimRight(strings.TrimSpace(msg[:cut]), " :-")

	const maxLen = 160
	if len(cleaned) > maxLen {
		cleaned = cleaned[:maxLen] + "..."
	}
	return cleaned
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
	executionID      string

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

	// Request sanitizer applied immediately before model-bound transport.
	requestSanitizer func(providers.ChatRequest) providers.ChatRequest

	// When true, provider terminal errors are returned to the caller without
	// emitting a stream error event. The chat service uses this to try another
	// configured provider before the browser sees a failed first attempt.
	suppressProviderErrorEvents bool

	// Query-only count/overview turns should not let a provider accidentally
	// expand a topology request into the full infrastructure tree.
	preferSummaryOnlyQueries bool
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

// SetRequestSanitizer installs a model-bound request sanitizer. It is called
// after compaction and before every provider turn so tool results from earlier
// turns cannot bypass resource policy redaction.
func (a *AgenticLoop) SetRequestSanitizer(fn func(providers.ChatRequest) providers.ChatRequest) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requestSanitizer = fn
}

func (a *AgenticLoop) SetSuppressProviderErrorEvents(suppress bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.suppressProviderErrorEvents = suppress
}

func (a *AgenticLoop) SetPreferSummaryOnlyQueries(prefer bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.preferSummaryOnlyQueries = prefer
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
//
// cost-recording-exempt: the loop accumulates token totals via stream
// callbacks but cost is recorded by the orchestrator that owns the
// loop (chat.Service.recordChatTurnCost for user chat; patrol_ai.go
// for patrol-via-chat). Recording here would double-count.
func (a *AgenticLoop) ExecuteWithTools(ctx context.Context, sessionID string, messages []Message, tools []providers.Tool, callback StreamCallback) ([]Message, error) {
	return a.executeWithTools(ctx, sessionID, messages, tools, callback)
}

// cost-recording-exempt: orchestrator (chat.Service or patrol caller)
// records cost from the loop's GetTotal{Input,Output}Tokens after this
// returns. See ExecuteWithTools above.
func (a *AgenticLoop) executeWithTools(ctx context.Context, sessionID string, messages []Message, tools []providers.Tool, callback StreamCallback) ([]Message, error) {
	// Snapshot maxTurns under the lock — callers may override via SetMaxTurns
	// before calling ExecuteWithTools, and this avoids races with concurrent sessions.
	a.mu.Lock()
	maxTurns := a.maxTurns
	suppressProviderErrorEvents := a.suppressProviderErrorEvents
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
	// Cache tool definition token estimate - tools don't change during the loop.
	cachedToolTokens := EstimateToolsTokens(tools)

	var resultMessages []Message
	turn := 0
	writeCompletedLastTurn := false // When true, request final text without offering tools
	toolBlockedLastTurn := false    // When true, request final text after budget/loop block

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
		requestSanitizer := a.requestSanitizer
		preferSummaryOnlyQueries := a.preferSummaryOnlyQueries
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
			Int("estimated_context_tokens", EstimateMessagesTokens(providerMessages)).
			Str("session_id", sessionID).
			Msg("[AgenticLoop] Starting turn")

		// Build the request with dynamic system prompt (includes current mode)
		systemPrompt := a.getSystemPrompt()
		req := providers.ChatRequest{
			Messages:    providerMessages,
			System:      systemPrompt,
			Tools:       tools,
			ExecutionID: a.executionID,
		}

		// Tool selection is model-owned. Pulse normally exposes the governed tool
		// manifest unchanged. When a run must stop for safety or budget reasons,
		// omit tools entirely rather than sending provider-specific tool_choice.
		textOnlySafetyBrake := false
		if turn >= maxTurns-1 {
			// Last turn before hitting the limit: ask the model to summarize with
			// the context already gathered.
			req.Tools = nil
			textOnlySafetyBrake = true
			log.Warn().
				Int("turn", turn).
				Int("max_turns", maxTurns).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Approaching max turns — omitting tools for final response")
		} else if writeCompletedLastTurn {
			// A write action completed successfully on the previous turn.
			// Ask for the final response with the execution result already in context.
			req.Tools = nil
			textOnlySafetyBrake = true
			writeCompletedLastTurn = false
			log.Debug().
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Write completed last turn — omitting tools for final response")
		} else if toolBlockedLastTurn {
			// Tool calls were blocked last turn (budget exceeded or loop detected).
			// Ask for a response using the data already gathered.
			req.Tools = nil
			textOnlySafetyBrake = true
			toolBlockedLastTurn = false
			log.Debug().
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Tool calls blocked last turn — omitting tools for final response")
		}

		// Pre-request context validation: catch overflow from message history growth.
		// Phase 2 handles first-turn overflow via seed budget; this catches
		// subsequent turns where tool results accumulate beyond the model's limit.
		{
			estimatedTokens := EstimateTokens(req.System) + EstimateMessagesTokens(req.Messages) + cachedToolTokens
			modelID := ""
			a.mu.Lock()
			modelID = a.providerName + ":" + a.modelName
			a.mu.Unlock()
			contextLimit := providers.ContextWindowTokens(modelID)

			if estimatedTokens > contextLimit {
				log.Warn().
					Int("estimated_tokens", estimatedTokens).
					Int("context_limit", contextLimit).
					Str("model", modelID).
					Str("session_id", sessionID).
					Msg("[AgenticLoop] Pre-request context overflow detected - emergency compaction")

				// Emergency: aggressively compact ALL old tool results (keepTurns=0, minChars=100)
				compactOldToolResults(providerMessages, currentTurnStartIndex, 0, 100, a.knowledgeAccumulator)

				// Re-build request with compacted messages
				req.Messages = providerMessages

				// Re-estimate after compaction.
				toolTokensForRequest := cachedToolTokens
				if req.Tools == nil {
					toolTokensForRequest = 0
				}
				estimatedTokens = EstimateTokens(req.System) + EstimateMessagesTokens(req.Messages) + toolTokensForRequest
				if estimatedTokens > contextLimit {
					log.Warn().
						Int("estimated_tokens", estimatedTokens).
						Int("context_limit", contextLimit).
						Str("session_id", sessionID).
						Msg("[AgenticLoop] Still over limit after compaction - dropping tools for this turn")

					req.Tools = nil
					textOnlySafetyBrake = true

					// Final check: if system + messages alone still exceed limit,
					// prune old messages as last resort.
					estimatedTokens = EstimateTokens(req.System) + EstimateMessagesTokens(req.Messages)
					if estimatedTokens > contextLimit {
						log.Warn().
							Int("estimated_tokens", estimatedTokens).
							Int("context_limit", contextLimit).
							Str("session_id", sessionID).
							Msg("[AgenticLoop] Still over limit after dropping tools - pruning old messages")

						// Keep the first message (original user prompt) and the most recent half.
						// This preserves the question context and recent investigation state.
						if len(providerMessages) > 4 {
							keepRecent := len(providerMessages) / 2
							if keepRecent < 2 {
								keepRecent = 2
							}
							pruned := make([]providers.Message, 0, 1+keepRecent)
							pruned = append(pruned, providerMessages[0])
							pruned = append(pruned, providerMessages[len(providerMessages)-keepRecent:]...)
							providerMessages = pruned
							req.Messages = providerMessages
							estimatedTokens = EstimateTokens(req.System) + EstimateMessagesTokens(req.Messages)

							log.Warn().
								Int("messages_after_prune", len(providerMessages)).
								Int("estimated_tokens_after_prune", estimatedTokens).
								Str("session_id", sessionID).
								Msg("[AgenticLoop] Pruned old messages to fit context window")
						}

						// If still over after all interventions, abort gracefully
						if estimatedTokens > contextLimit {
							log.Error().
								Int("estimated_tokens", estimatedTokens).
								Int("context_limit", contextLimit).
								Str("session_id", sessionID).
								Msg("[AgenticLoop] Cannot fit request in context window - aborting turn")
							break
						}
					}
				}
			}
		}

		// Collect streaming response
		var contentBuilder strings.Builder
		var thinkingBuilder strings.Builder
		var toolCalls []providers.ToolCall
		var suppressLeakedToolContent bool
		var pendingVisibleContent string
		var emittedThinkingWorkflow bool
		visibleToolStarts := make(map[string]bool)
		suppressedToolStarts := make(map[string]bool)
		emitToolStartIfNeeded := func(tc providers.ToolCall) {
			keys := toolStartKeys(tc.ID, tc.Name)
			if len(keys) == 0 || toolStartMapHas(visibleToolStarts, tc.ID, tc.Name) || toolStartMapHas(suppressedToolStarts, tc.ID, tc.Name) {
				return
			}
			emitToolStartEvent(callback, tc.ID, tc.Name, tc.Input)
			markToolStartMap(visibleToolStarts, tc.ID, tc.Name)
		}
		emitCurrentResourceBlock := func(tc providers.ToolCall, blockMsg string) {
			if toolStartMapHas(visibleToolStarts, tc.ID, tc.Name) {
				emitToolCancelEvent(callback, tc.ID, tc.Name, "current_resource unavailable")
			}
			markToolStartMap(suppressedToolStarts, tc.ID, tc.Name)
			resultMessages = removeToolCallFromResultMessages(resultMessages, tc.ID)
		}

		log.Debug().
			Str("session_id", sessionID).
			Int("system_prompt_len", len(systemPrompt)).
			Msg("[AgenticLoop] Calling provider.ChatStream")
		if requestSanitizer != nil {
			req = requestSanitizer(req)
		}

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
						if suppressLeakedToolContent {
							return
						}
						visibleText, leakFound := appendVisibleContentBeforeToolLeak(&contentBuilder, &pendingVisibleContent, data.Text)
						if visibleText != "" {
							jsonData, _ := json.Marshal(ContentData{Text: visibleText})
							callback(StreamEvent{Type: "content", Data: jsonData})
						}
						if leakFound {
							// The model started serializing an internal tool call as
							// assistant prose instead of using the structured tool_calls
							// channel. Stop forwarding the rest of this provider turn.
							suppressLeakedToolContent = true
							return
						}
					}

				case "thinking":
					if data, ok := event.Data.(providers.ThinkingEvent); ok {
						thinkingBuilder.WriteString(data.Text)
						if !emittedThinkingWorkflow && !attemptEmittedVisibleEvents {
							emitWorkflowState(callback, "model_thinking", "Model is reasoning before responding.", sessionFSMState(a.sessionFSM), "")
							emittedThinkingWorkflow = true
						}
					}

				case "tool_start":
					if data, ok := event.Data.(providers.ToolStartEvent); ok {
						// pulse_question is rendered as a dedicated "question" card; suppress tool UI events.
						if data.Name == pulseQuestionToolName {
							return
						}
						data.Input = normalizeSummaryOnlyPulseQueryInput(preferSummaryOnlyQueries, data.Name, data.Input)
						key := toolStartKey(data.ID, data.Name)
						if len(data.Input) == 0 {
							if key != "" && !toolStartMapHas(visibleToolStarts, data.ID, data.Name) && !toolStartMapHas(suppressedToolStarts, data.ID, data.Name) {
								attemptEmittedVisibleEvents = true
								emitToolStartEvent(callback, data.ID, data.Name, data.Input)
								markToolStartMap(visibleToolStarts, data.ID, data.Name)
							}
							return
						}
						if blockMsg, blocked := a.currentResourcePlaceholderBlock(providers.ToolCall{ID: data.ID, Name: data.Name, Input: data.Input}); blocked {
							log.Warn().
								Str("tool", data.Name).
								Str("id", data.ID).
								Str("reason", blockMsg).
								Msg("[AgenticLoop] Suppressed invalid current_resource tool_start before user-visible execution")
							markToolStartMap(suppressedToolStarts, data.ID, data.Name)
							return
						}
						attemptEmittedVisibleEvents = true
						markToolStartMap(visibleToolStarts, data.ID, data.Name)
						emitToolStartEvent(callback, data.ID, data.Name, data.Input)
					}

				case "tool_progress":
					if data, ok := event.Data.(providers.ToolProgressEvent); ok {
						if data.Name == pulseQuestionToolName {
							return
						}
						data.Input = normalizeSummaryOnlyPulseQueryInput(preferSummaryOnlyQueries, data.Name, data.Input)
						if toolStartMapHas(suppressedToolStarts, data.ID, data.Name) {
							return
						}
						if len(data.Input) > 0 {
							if blockMsg, blocked := a.currentResourcePlaceholderBlock(providers.ToolCall{ID: data.ID, Name: data.Name, Input: data.Input}); blocked {
								log.Warn().
									Str("tool", data.Name).
									Str("id", data.ID).
									Str("reason", blockMsg).
									Msg("[AgenticLoop] Suppressed invalid current_resource tool_progress before user-visible execution")
								emitCurrentResourceBlock(providers.ToolCall{ID: data.ID, Name: data.Name, Input: data.Input}, blockMsg)
								return
							}
						}
						if !toolStartMapHas(visibleToolStarts, data.ID, data.Name) {
							attemptEmittedVisibleEvents = true
							emitToolStartEvent(callback, data.ID, data.Name, data.Input)
							markToolStartMap(visibleToolStarts, data.ID, data.Name)
						}
						attemptEmittedVisibleEvents = true
						emitToolProgressEventWithRawInput(
							callback,
							data.ID,
							data.Name,
							data.Input,
							data.RawInput,
							firstNonEmptyTrimmed(data.Phase, "pending"),
							data.Message,
						)
					}

				case "done":
					if data, ok := event.Data.(providers.DoneEvent); ok {
						attemptSawDone = true
						toolCalls = data.ToolCalls
						if preferSummaryOnlyQueries {
							for i := range toolCalls {
								toolCalls[i].Input = normalizeSummaryOnlyPulseQueryInput(true, toolCalls[i].Name, toolCalls[i].Input)
							}
						}
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

			if !suppressProviderErrorEvents {
				if len(attemptErrorMessages) > 0 {
					// Defer emitting error events until retries are exhausted so transient
					// provider blips don't leak to the client stream. Sanitize so raw
					// provider JSON/URLs never reach the user.
					jsonData, _ := json.Marshal(ErrorData{Message: sanitizeProviderStreamErrorForUser(attemptErrorMessages[0])})
					callback(StreamEvent{Type: "error", Data: jsonData})
				} else {
					// Transport-level failures may not include an explicit provider error event.
					// Emit a fallback error so clients can render a clear failure state.
					jsonData, _ := json.Marshal(ErrorData{Message: fallbackProviderStreamErrorMessage(effectiveErr)})
					callback(StreamEvent{Type: "error", Data: jsonData})
				}
			}
			err = effectiveErr
			break
		}

		if err == nil && !suppressLeakedToolContent {
			if visibleText := flushPendingVisibleContent(&contentBuilder, &pendingVisibleContent); visibleText != "" {
				jsonData, _ := json.Marshal(ContentData{Text: visibleText})
				callback(StreamEvent{Type: "content", Data: jsonData})
			}
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

		// Guard: if a safety brake omitted tools but the model still returned tool
		// calls from conversation history, strip them so the model's text content
		// is treated as the final response.
		if textOnlySafetyBrake && len(toolCalls) > 0 {
			log.Warn().
				Str("session_id", sessionID).
				Int("stripped_tool_calls", len(toolCalls)).
				Msg("[AgenticLoop] Model returned tool calls after tools were omitted — stripping them")
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
		// Clean tool call artifacts from the content before storing.
		cleanedContent := cleanToolCallArtifacts(contentBuilder.String())
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
					verifyTarget := strings.TrimSpace(fsm.LastWriteTool)
					if verifyTarget == "" {
						verifyTarget = "the changed target"
					}
					verifyPrompt := fmt.Sprintf(
						"Verification evidence is required before responding about the write result for %s. Decide what available evidence or tool call is appropriate to verify the current state.",
						verifyTarget,
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

			log.Debug().Msg("agentic loop complete - no tool calls")
			resultMessages = a.ensureFinalTextResponse(ctx, sessionID, resultMessages, providerMessages, callback)
			return resultMessages, nil
		}

		// === Execute tool calls (three-phase pipeline) ===
		// Phase 1: Pre-check (sequential) — FSM, loop detection, budget checks
		// Phase 2: Execute (parallel) — actual tool calls via goroutines
		// Phase 3: Post-process (sequential) — streaming, FSM transitions, KA extraction
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
			emitWorkflowState(callback, "clarify", "Waiting for your answer before continuing.", sessionFSMState(fsm), pulseQuestionToolName)

			for _, tc := range toolCalls {
				log.Debug().
					Str("tool", tc.Name).
					Str("id", tc.ID).
					Msg("Processing interactive tool call set (pulse_question present)")

				toolKind := ClassifyToolCall(tc.Name, tc.Input)

				if blockMsg, blocked := a.currentResourcePlaceholderBlock(tc); blocked {
					log.Warn().
						Str("tool", tc.Name).
						Str("id", tc.ID).
						Str("reason", blockMsg).
						Msg("[AgenticLoop] Blocked invalid current_resource tool call before interactive-set execution")

					emitCurrentResourceBlock(tc, blockMsg)
					providerMessages = append(providerMessages, providers.Message{
						Role: "user",
						ToolResult: &providers.ToolResult{
							ToolUseID: tc.ID,
							Content:   truncateToolResultForModel(blockMsg),
							IsError:   true,
						},
					})
					continue
				}

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

						emitToolStartIfNeeded(tc)
						emitToolEndEvent(callback, tc.ID, tc.Name, tc.Input, fsmErr.Error(), false)

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

					emitToolStartIfNeeded(tc)
					emitToolEndEvent(callback, tc.ID, tc.Name, tc.Input, loopMsg, false)

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

					emitToolStartIfNeeded(tc)
					emitToolEndEvent(callback, tc.ID, tc.Name, tc.Input, skipMsg, false)

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

			if blockMsg, blocked := a.currentResourcePlaceholderBlock(tc); blocked {
				log.Warn().
					Str("tool", tc.Name).
					Str("id", tc.ID).
					Str("reason", blockMsg).
					Msg("[AgenticLoop] Blocked invalid current_resource tool call before execution")

				emitCurrentResourceBlock(tc, blockMsg)
				providerMessages = append(providerMessages, providers.Message{
					Role: "user",
					ToolResult: &providers.ToolResult{
						ToolUseID: tc.ID,
						Content:   truncateToolResultForModel(blockMsg),
						IsError:   true,
					},
				})
				continue
			}

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
						// Record that the model received a recoverable policy block.
						if metrics := GetAIMetrics(); metrics != nil {
							metrics.RecordAutoRecoveryAttempt("FSM_BLOCKED", tc.Name)
						}
					}

					// Send tool_end event with error
					emitToolStartIfNeeded(tc)
					emitToolEndEvent(callback, tc.ID, tc.Name, tc.Input, fsmErr.Error(), false)

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

				emitToolStartIfNeeded(tc)
				emitToolEndEvent(callback, tc.ID, tc.Name, tc.Input, loopMsg, false)

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

			// Tool passed all pre-checks — queue for execution
			pendingExec = append(pendingExec, pendingToolExec{tc: tc, toolKind: toolKind})
		}

		// --- Phase 2: Execute pending tools (parallel if multiple) ---
		// Tool execution is stateless I/O — safe to parallelize.
		// Cap concurrency at 4 to avoid overwhelming infrastructure.
		execResults := make([]parallelToolResult, len(pendingExec))
		if len(pendingExec) > 0 {
			for _, pe := range pendingExec {
				emitToolStartIfNeeded(pe.tc)
				emitToolProgressEvent(
					callback,
					pe.tc.ID,
					pe.tc.Name,
					pe.tc.Input,
					"running",
					toolExecutionProgressMessage(pe.tc.Name, pe.tc.Input, pe.toolKind),
				)
			}
			executeMessage := "Running infrastructure checks."
			workflowTool := pendingExec[0].tc.Name
			for _, pe := range pendingExec {
				if pe.toolKind == ToolKindWrite {
					executeMessage = "Running the planned action through governed execution."
					workflowTool = pe.tc.Name
					emitWorkflowState(callback, "plan", "Planning governed action and safety checks before execution.", sessionFSMState(fsm), workflowTool)
					break
				}
			}
			emitWorkflowState(callback, "execute", executeMessage, sessionFSMState(fsm), workflowTool)
		}

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

			// Check if this is an approval request
			if strings.HasPrefix(resultText, "APPROVAL_REQUIRED:") {
				// Parse approval request
				approvalJSON := strings.TrimPrefix(resultText, "APPROVAL_REQUIRED:")
				approvalJSON = strings.TrimSpace(approvalJSON)

				var approvalData struct {
					ApprovalID        string                         `json:"approval_id"`
					Command           string                         `json:"command"`
					Risk              string                         `json:"risk"`
					Description       string                         `json:"description"`
					Reason            string                         `json:"reason"`
					TargetHost        string                         `json:"target_host"`
					Host              string                         `json:"host"`
					DockerHost        string                         `json:"docker_host"`
					ResourceHost      string                         `json:"resource_host"`
					Cluster           string                         `json:"cluster"`
					TargetName        string                         `json:"target_name"`
					TargetType        string                         `json:"target_type"`
					TargetID          string                         `json:"target_id"`
					AuditID           string                         `json:"audit_id"`
					Plan              *ApprovalPlanData              `json:"plan"`
					ContextConfidence *ApprovalContextConfidenceData `json:"context_confidence"`
					Preflight         *ApprovalPreflightData         `json:"preflight"`
				}
				if err := json.Unmarshal([]byte(approvalJSON), &approvalData); err != nil {
					log.Error().Err(err).Str("data", approvalJSON).Msg("failed to parse approval request")
					resultText = "Error: failed to parse approval request"
					isError = true
				} else {
					targetHost := firstNonEmptyTrimmed(approvalData.TargetHost, approvalData.Host, approvalData.DockerHost, approvalData.ResourceHost, approvalData.Cluster, approvalData.TargetName)
					description := firstNonEmptyTrimmed(approvalData.Description, approvalData.Reason)
					// Send approval_needed event
					jsonData, _ := json.Marshal(ApprovalNeededData{
						ApprovalID:        approvalData.ApprovalID,
						ToolID:            tc.ID,
						ToolName:          tc.Name,
						Command:           approvalData.Command,
						RunOnHost:         true, // Control commands run on host
						TargetHost:        targetHost,
						TargetType:        approvalData.TargetType,
						TargetID:          approvalData.TargetID,
						Risk:              approvalData.Risk,
						Description:       description,
						AuditID:           approvalData.AuditID,
						Plan:              approvalData.Plan,
						ContextConfidence: approvalData.ContextConfidence,
						Preflight:         approvalData.Preflight,
					})
					emitWorkflowState(callback, "approve", "Waiting for approval before executing the planned action.", sessionFSMState(fsm), tc.Name)
					emitToolProgressEvent(callback, tc.ID, tc.Name, tc.Input, "waiting", "Waiting for approval.")
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
								if a.executor != nil {
									a.executor.RecordApprovalDecision(approvalData.ApprovalID, unifiedresources.ActionStateFailed, "pulse_assistant", waitErr.Error())
								}
								emitWorkflowState(callback, "complete", "Approval wait ended before execution.", sessionFSMState(fsm), tc.Name)
								resultText = fmt.Sprintf("Approval timeout or error: %v", waitErr)
								isError = true
							} else if decision.Status == approval.StatusApproved {
								if a.executor != nil {
									a.executor.RecordApprovalDecision(approvalData.ApprovalID, unifiedresources.ActionStateApproved, decision.DecidedBy, "approval granted")
								}
								emitWorkflowState(callback, "execute", "Approval granted. Executing the approved action.", sessionFSMState(fsm), tc.Name)
								emitToolProgressEvent(callback, tc.ID, tc.Name, tc.Input, "running", "Executing approved action.")
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
								if a.executor != nil {
									a.executor.RecordApprovalDecision(approvalData.ApprovalID, unifiedresources.ActionStateRejected, decision.DecidedBy, firstNonEmptyTrimmed(decision.DenyReason, "approval denied"))
								}
								emitWorkflowState(callback, "complete", "Approval denied. No action was executed.", sessionFSMState(fsm), tc.Name)
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
				if toolKind == ToolKindWrite && fsm.State == StateVerifying {
					emitWorkflowState(callback, "verify", "Verifying the write before the Assistant responds.", sessionFSMState(fsm), tc.Name)
				}

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

				// Check if this success resolves a pending policy block.
				if pr := fsm.CheckRecoverySuccess(tc.Name); pr != nil {
					log.Info().
						Str("tool", tc.Name).
						Str("error_code", pr.ErrorCode).
						Str("recovery_id", pr.RecoveryID).
						Msg("[AgenticLoop] model self-correction after policy block succeeded")
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
						Msg("[AgenticLoop] All tool calls failed for 3 consecutive turns — next turn omits tools")
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
				Msg("[AgenticLoop] Tool calls blocked this turn — next turn omits tools")
		}

		// Guardrail: in interactive chat mode, force wrap-up after repeated
		// tool-only turns to avoid long no-answer chains.
		a.mu.Lock()
		autonomousMode := a.autonomousMode
		a.mu.Unlock()
		if !autonomousMode && consecutiveToolOnlyTurns >= maxConsecutiveToolOnlyTurns {
			toolBlockedLastTurn = true
			log.Warn().
				Int("consecutive_tool_only_turns", consecutiveToolOnlyTurns).
				Int("turn", turn).
				Str("session_id", sessionID).
				Msg("[AgenticLoop] Consecutive tool-only turns exceeded threshold — next turn omits tools")
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
