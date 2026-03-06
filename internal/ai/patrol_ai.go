// patrol_ai.go handles all LLM interaction for patrol: seed context building,
// system/user prompt construction, the agentic analysis loop, evaluation passes,
// stale finding reconciliation, and thinking-token cleanup for model responses.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// AIAnalysisResult contains the results of an AI analysis
type AIAnalysisResult struct {
	Response         string     // The AI's raw response text
	Findings         []*Finding // Parsed findings from the response
	RejectedFindings int        // Findings rejected by threshold validation
	TriageFlags      int        // Number of deterministic triage flags
	TriageSkippedLLM bool       // True if LLM was skipped due to quiet triage
	InputTokens      int
	OutputTokens     int
	ToolCalls        []ToolCallRecord // Tool invocations during this analysis
	ReportedIDs      []string         // Finding IDs reported (created/re-reported) this run
	ResolvedIDs      []string         // Finding IDs explicitly resolved by LLM this run
	SeededFindingIDs []string         // Finding IDs that were presented in seed context
}

const (
	patrolMinTurns          = 20
	patrolMaxTurnsLimit     = 80
	patrolTurnsPer50Devices = 5
	patrolQuickMinTurns     = 10
	patrolQuickMaxTurns     = 30
)

// CleanThinkingTokens removes model-specific thinking markers from AI responses.
// Different AI models use different markers for their internal reasoning:
// - DeepSeek: <｜end▁of▁thinking｜> or similar unicode variants
// - DeepSeek: <｜DSML｜...> internal function call format (hallucinated tool calls)
// - Generic: <think>...</think>, <thought>...</thought>
// - Reasoning: <|reasoning|>...</|/reasoning|>
//
// This function is exported so it can be used by both patrol and chat responses.
func CleanThinkingTokens(content string) string {
	if content == "" {
		return content
	}

	// Phase 0: Remove DeepSeek internal function call format leakage.
	// When DeepSeek doesn't properly use the function calling API, it may output
	// its internal markup like <｜DSML｜function_calls>, <｜DSML｜invoke>, etc.
	// These patterns can appear with Unicode pipe (｜) or ASCII pipe (|).
	deepseekFunctionMarkers := []string{
		"<｜DSML｜",  // Unicode pipe variant (opening)
		"</｜DSML｜", // Unicode pipe variant (closing)
		"<|DSML|",  // ASCII pipe variant (opening)
		"</|DSML|", // ASCII pipe variant (closing)
		"<｜/DSML｜", // Alternative Unicode closing
		"<|/DSML|", // Alternative ASCII closing
	}
	for _, marker := range deepseekFunctionMarkers {
		if strings.Contains(content, marker) {
			// Find the start of the block and remove everything from there to the end
			// DeepSeek function call blocks typically appear at the end of responses
			idx := strings.Index(content, marker)
			if idx >= 0 {
				content = strings.TrimSpace(content[:idx])
			}
		}
	}

	// Phase 1: Remove entire block-level tags (opening + content + closing).
	// Case-insensitive matching via lowercased copy.
	type blockTag struct {
		open  string
		close string
	}
	blockTags := []blockTag{
		{"<think>", "</think>"},
		{"<thought>", "</thought>"},
		{"<|reasoning|>", "<|/reasoning|>"},
	}
	for _, bt := range blockTags {
		lower := strings.ToLower(content)
		for {
			openIdx := strings.Index(lower, bt.open)
			if openIdx < 0 {
				break
			}
			closeIdx := strings.Index(lower[openIdx+len(bt.open):], bt.close)
			if closeIdx < 0 {
				// Unclosed block — remove from open tag to end
				content = content[:openIdx]
				lower = strings.ToLower(content)
			} else {
				end := openIdx + len(bt.open) + closeIdx + len(bt.close)
				content = content[:openIdx] + content[end:]
				lower = strings.ToLower(content)
			}
		}
	}

	// Phase 2: Remove line-level end markers (DeepSeek and remaining close tags).
	thinkingMarkers := []string{
		"<｜end▁of▁thinking｜>", // DeepSeek Unicode variant
		"<|end_of_thinking|>", // ASCII variant
		"<|end▁of▁thinking|>", // Mixed variant
		"</think>",            // Generic thinking block end
		"</thought>",          // Thought block end
		"<|/reasoning|>",      // Reasoning block end
	}

	for _, marker := range thinkingMarkers {
		for strings.Contains(content, marker) {
			idx := strings.Index(content, marker)
			if idx >= 0 {
				// Find start of the line containing the marker
				lineStart := strings.LastIndex(content[:idx], "\n")
				if lineStart == -1 {
					lineStart = 0
				}
				// Find end of the line containing the marker
				markerEnd := idx + len(marker)
				lineEnd := strings.Index(content[markerEnd:], "\n")
				if lineEnd == -1 {
					lineEnd = len(content)
				} else {
					lineEnd = markerEnd + lineEnd
				}
				// Remove the entire line containing the marker
				content = content[:lineStart] + content[lineEnd:]
			}
		}
	}

	// Phase 3: Remove lines that look like internal reasoning.
	// These typically start with patterns like "Now, " or "Let's " after a blank line.
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	skipUntilContent := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip lines that look like internal reasoning
		if skipUntilContent {
			// Resume when we hit actual content (markdown headers, findings, etc.)
			if strings.HasPrefix(trimmed, "#") ||
				strings.HasPrefix(trimmed, "[FINDING]") ||
				strings.HasPrefix(trimmed, "**") ||
				strings.HasPrefix(trimmed, "-") ||
				strings.HasPrefix(trimmed, "1.") {
				skipUntilContent = false
			} else {
				continue
			}
		}

		// Detect reasoning patterns (typically after empty lines)
		if trimmed == "" && i+1 < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextTrimmed, "Now, ") ||
				strings.HasPrefix(nextTrimmed, "Let's ") ||
				strings.HasPrefix(nextTrimmed, "Let me ") ||
				strings.HasPrefix(nextTrimmed, "I should ") ||
				strings.HasPrefix(nextTrimmed, "I'll ") ||
				strings.HasPrefix(nextTrimmed, "I need to ") ||
				strings.HasPrefix(nextTrimmed, "Checking ") ||
				strings.HasPrefix(nextTrimmed, "Looking at ") {
				skipUntilContent = true
				continue
			}
		}

		cleanedLines = append(cleanedLines, line)
	}

	// Clean up excessive blank lines
	content = strings.Join(cleanedLines, "\n")
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

// runAIAnalysis uses the agentic tool-driven approach to analyze infrastructure.
// The LLM investigates using MCP tools and reports findings via patrol_report_finding.
// An optional scope focuses the patrol on specific resources.
func (p *PatrolService) runAIAnalysis(ctx context.Context, snap models.StateSnapshot, scope *PatrolScope) (*AIAnalysisResult, error) {
	return p.runAIAnalysisState(ctx, p.patrolRuntimeStateForSnapshot(snap), scope)
}

func (p *PatrolService) runAIAnalysisState(ctx context.Context, snap patrolRuntimeState, scope *PatrolScope) (*AIAnalysisResult, error) {
	if p.aiService == nil {
		return nil, fmt.Errorf("Pulse Patrol service not available")
	}

	// Pre-flight budget check: fail fast before building context or acquiring a chat service
	if err := p.aiService.CheckBudget("patrol"); err != nil {
		log.Warn().Err(err).Msg("AI Patrol: Budget exceeded, skipping analysis")
		return nil, fmt.Errorf("patrol skipped: %w", err)
	}

	// Gather guest intelligence (discovery + reachability) before building seed context
	intelCtx, intelCancel := context.WithTimeout(ctx, 5*time.Second)
	guestIntel := p.gatherGuestIntelligence(intelCtx)
	intelCancel()

	// Phase 1: Deterministic triage
	triageResult := p.runDeterministicTriageState(ctx, snap, scope, guestIntel)
	log.Info().
		Int("flags", len(triageResult.Flags)).
		Bool("quiet", triageResult.IsQuiet).
		Int("flagged_resources", len(triageResult.FlaggedIDs)).
		Msg("AI Patrol: Triage complete")
	metrics := GetPatrolMetrics()
	metrics.RecordTriageFlags(len(triageResult.Flags))
	if triageResult.IsQuiet {
		metrics.RecordTriageQuiet()
	}

	// Quiet infrastructure: skip LLM entirely
	if triageResult.IsQuiet {
		log.Info().Msg("AI Patrol: Infrastructure quiet, skipping LLM analysis")
		return &AIAnalysisResult{
			Response:         "Infrastructure healthy — deterministic triage found no issues.",
			TriageFlags:      0,
			TriageSkippedLLM: true,
		}, nil
	}

	// Phase 2: Build focused seed context from triage results
	seedContext, seededFindingIDs := p.buildTriageSeedContextState(triageResult, snap, scope, guestIntel)
	if strings.TrimSpace(seedContext) == "" {
		return nil, nil
	}
	log.Info().
		Int("seed_context_chars", len(seedContext)).
		Int("seed_context_estimated_tokens", chat.EstimateTokens(seedContext)).
		Msg("AI Patrol: Triage seed context built")

	log.Debug().Msg("AI Patrol: Starting agentic patrol analysis")

	maxTurns := computeTriageMaxTurns(len(triageResult.Flags), scope)
	log.Debug().
		Int("triage_flags", len(triageResult.Flags)).
		Int("max_turns", maxTurns).
		Msg("AI Patrol: Calculated agentic max turns")

	// Determine whether to skip streaming updates (verification runs are consumed
	// programmatically and must not interleave with a concurrent normal patrol's stream).
	noStream := scope != nil && scope.NoStream

	// Start streaming phase
	if !noStream {
		p.setStreamPhase("analyzing")
		p.broadcast(PatrolStreamEvent{Type: "start"})
	}

	// Create finding creator adapter
	adapter := newPatrolFindingCreatorAdapterState(p, snap)

	// Get chat service and set the finding creator on the executor
	cs := p.aiService.GetChatService()
	if cs == nil {
		if !noStream {
			p.setStreamPhase("idle")
		}
		return nil, fmt.Errorf("chat service not available")
	}

	// Type-assert to get executor access
	executorAccessor, ok := cs.(chatServiceExecutorAccessor)
	if !ok {
		if !noStream {
			p.setStreamPhase("idle")
		}
		return nil, fmt.Errorf("chat service does not support executor access")
	}
	executor := executorAccessor.GetExecutor()
	if executor == nil {
		if !noStream {
			p.setStreamPhase("idle")
		}
		return nil, fmt.Errorf("tool executor not available")
	}

	// Set the patrol finding creator for this run
	executor.SetPatrolFindingCreator(adapter)
	defer executor.SetPatrolFindingCreator(nil) // Clear after run

	// Execute the agentic patrol loop
	var contentBuffer strings.Builder
	var inputTokens, outputTokens int

	// Tool call collection
	var toolCallsMu sync.Mutex
	pendingToolCalls := make(map[string]ToolCallRecord)
	var pendingToolOrder []string
	anonToolCounter := 0
	var completedToolCalls []ToolCallRecord
	var rawToolOutputs []string

	chatResp, chatErr := cs.ExecutePatrolStream(ctx, PatrolExecuteRequest{
		Prompt:       seedContext,
		SystemPrompt: p.getPatrolSystemPromptForTriage(),
		SessionID:    "patrol-main",
		UseCase:      "patrol",
		MaxTurns:     maxTurns,
	}, func(event ChatStreamEvent) {
		switch event.Type {
		case "content":
			var contentData struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(event.Data, &contentData) == nil && contentData.Text != "" {
				contentBuffer.WriteString(contentData.Text)
				if !noStream {
					p.appendStreamContent(contentData.Text)
				}
			}
		case "thinking":
			var thinkingData struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(event.Data, &thinkingData) == nil && thinkingData.Text != "" {
				if !noStream {
					p.broadcast(PatrolStreamEvent{
						Type:    "thinking",
						Content: thinkingData.Text,
					})
				}
			}
		case "tool_start":
			var data struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Input    string `json:"input"`
				RawInput string `json:"raw_input"`
			}
			if json.Unmarshal(event.Data, &data) == nil {
				if data.ID == "" {
					anonToolCounter++
					data.ID = fmt.Sprintf("patrol-anon-%d", anonToolCounter)
				}
				if !noStream {
					p.broadcast(PatrolStreamEvent{
						Type:         "tool_start",
						ToolID:       data.ID,
						ToolName:     data.Name,
						ToolInput:    data.Input,
						ToolRawInput: data.RawInput,
					})
				}
				input := data.Input
				if data.RawInput != "" {
					input = data.RawInput
				}
				toolCallsMu.Lock()
				pendingToolOrder = append(pendingToolOrder, data.ID)
				pendingToolCalls[data.ID] = ToolCallRecord{
					ID:        data.ID,
					ToolName:  data.Name,
					Input:     truncateString(input, MaxToolInputSize),
					StartTime: time.Now().UnixMilli(),
				}
				toolCallsMu.Unlock()
			}
		case "tool_end":
			var data struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Input    string `json:"input"`
				RawInput string `json:"raw_input"`
				Output   string `json:"output"`
				Success  bool   `json:"success"`
			}
			if json.Unmarshal(event.Data, &data) == nil {
				if data.ID == "" {
					if len(pendingToolOrder) > 0 {
						data.ID = pendingToolOrder[0]
						pendingToolOrder = pendingToolOrder[1:]
					} else {
						anonToolCounter++
						data.ID = fmt.Sprintf("patrol-anon-end-%d", anonToolCounter)
					}
				} else if len(pendingToolOrder) > 0 {
					for i, id := range pendingToolOrder {
						if id == data.ID {
							pendingToolOrder = append(pendingToolOrder[:i], pendingToolOrder[i+1:]...)
							break
						}
					}
				}
				if !noStream {
					success := data.Success
					p.broadcast(PatrolStreamEvent{
						Type:         "tool_end",
						ToolID:       data.ID,
						ToolName:     data.Name,
						ToolInput:    data.Input,
						ToolRawInput: data.RawInput,
						ToolOutput:   data.Output,
						ToolSuccess:  &success,
					})
				}
				toolCallsMu.Lock()
				if pending, ok := pendingToolCalls[data.ID]; ok {
					now := time.Now().UnixMilli()
					input := data.Input
					if data.RawInput != "" {
						input = data.RawInput
					}
					if input != "" {
						pending.Input = truncateString(input, MaxToolInputSize)
					}
					pending.Output = truncateString(data.Output, MaxToolOutputSize)
					pending.Success = data.Success
					pending.EndTime = now
					pending.Duration = now - pending.StartTime
					completedToolCalls = append(completedToolCalls, pending)
					rawToolOutputs = append(rawToolOutputs, data.Output)
					delete(pendingToolCalls, data.ID)
				} else {
					now := time.Now().UnixMilli()
					input := data.Input
					if data.RawInput != "" {
						input = data.RawInput
					}
					completedToolCalls = append(completedToolCalls, ToolCallRecord{
						ID:        data.ID,
						ToolName:  data.Name,
						Input:     truncateString(input, MaxToolInputSize),
						Output:    truncateString(data.Output, MaxToolOutputSize),
						Success:   data.Success,
						StartTime: now,
						EndTime:   now,
						Duration:  0,
					})
					rawToolOutputs = append(rawToolOutputs, data.Output)
				}
				toolCallsMu.Unlock()
			}
		}
	})

	if chatErr != nil {
		if !noStream {
			p.setStreamPhase("idle")
			p.broadcast(PatrolStreamEvent{Type: "error", Content: chatErr.Error()})
		}
		return nil, fmt.Errorf("agentic patrol failed: %w", chatErr)
	}

	finalContent := chatResp.Content
	if finalContent == "" {
		finalContent = contentBuffer.String()
	}
	inputTokens = chatResp.InputTokens
	outputTokens = chatResp.OutputTokens
	p.recordPatrolUsage(chatResp.InputTokens, chatResp.OutputTokens)

	// Clean thinking tokens
	finalContent = CleanThinkingTokens(finalContent)

	log.Debug().
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Int("findings_created", len(adapter.getCollectedFindings())).
		Int("findings_resolved", adapter.getResolvedCount()).
		Msg("AI Patrol: Agentic patrol analysis complete")

	p.ensureInvestigationToolCall(ctx, executor, &toolCallsMu, &completedToolCalls, &rawToolOutputs, noStream)

	// Broadcast completion
	if !noStream {
		p.broadcast(PatrolStreamEvent{
			Type:   "complete",
			Tokens: outputTokens,
		})
		p.setStreamPhase("idle")
	}

	// Collect completed tool calls
	toolCallsMu.Lock()
	collectedToolCalls := completedToolCalls
	signalToolCalls := make([]ToolCallRecord, len(collectedToolCalls))
	for i, tc := range collectedToolCalls {
		signalToolCalls[i] = tc
		if i < len(rawToolOutputs) && rawToolOutputs[i] != "" {
			signalToolCalls[i].Output = rawToolOutputs[i]
		}
	}
	toolCallsMu.Unlock()

	// --- Deterministic signal detection + evaluation pass ---
	// Build signal thresholds from user config so detection aligns with alert settings
	p.mu.RLock()
	sigThresholds := SignalThresholdsFromPatrol(p.thresholds)
	p.mu.RUnlock()
	detectedSignals := DetectSignals(signalToolCalls, sigThresholds)

	// Merge reachability signals from pre-patrol guest probing
	reachabilitySignals := DetectReachabilitySignals(guestIntel)
	detectedSignals = append(detectedSignals, reachabilitySignals...)

	if len(detectedSignals) > 0 {
		log.Info().
			Int("detected_signals", len(detectedSignals)).
			Msg("AI Patrol: Deterministic signal detection found signals")

		unmatchedSignals := UnmatchedSignals(detectedSignals, adapter.getCollectedFindings())
		if len(unmatchedSignals) > 0 {
			log.Warn().
				Int("unmatched_signals", len(unmatchedSignals)).
				Msg("AI Patrol: Unmatched signals found, running evaluation pass")

			evalResp, evalErr := p.runEvaluationPass(ctx, adapter, unmatchedSignals)
			if evalErr != nil {
				log.Warn().Err(evalErr).Msg("AI Patrol: Evaluation pass failed")
			} else if evalResp != nil {
				inputTokens += evalResp.InputTokens
				outputTokens += evalResp.OutputTokens
				log.Info().
					Int("eval_input_tokens", evalResp.InputTokens).
					Int("eval_output_tokens", evalResp.OutputTokens).
					Int("total_findings", len(adapter.getCollectedFindings())).
					Msg("AI Patrol: Evaluation pass completed")
			}

			// Deterministic fallback: if unmatched signals remain, create findings directly.
			remaining := UnmatchedSignals(detectedSignals, adapter.getCollectedFindings())
			if len(remaining) > 0 {
				created := p.createFindingsFromSignals(adapter, remaining)
				if created > 0 {
					log.Info().
						Int("created", created).
						Int("remaining", len(remaining)).
						Msg("AI Patrol: Created deterministic findings for unmatched signals")
				}
			}
		} else {
			log.Debug().
				Int("detected_signals", len(detectedSignals)).
				Msg("AI Patrol: All detected signals already matched by findings")
		}
	}

	// Findings were already created via tool calls — collect them
	adapter.findingsMu.Lock()
	rejectedCount := adapter.rejectedCount
	adapter.findingsMu.Unlock()
	return &AIAnalysisResult{
		Response:         finalContent,
		Findings:         adapter.getCollectedFindings(),
		RejectedFindings: rejectedCount,
		TriageFlags:      len(triageResult.Flags),
		TriageSkippedLLM: false,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		ToolCalls:        collectedToolCalls,
		ReportedIDs:      adapter.getReportedFindingIDs(),
		ResolvedIDs:      adapter.getResolvedIDs(),
		SeededFindingIDs: seededFindingIDs,
	}, nil
}

func computePatrolMaxTurns(resourceCount int, scope *PatrolScope) int {
	minTurns := patrolMinTurns
	maxTurns := patrolMaxTurnsLimit
	if scope != nil && scope.Depth == PatrolDepthQuick {
		minTurns = patrolQuickMinTurns
		maxTurns = patrolQuickMaxTurns
	}

	extra := (resourceCount / 50) * patrolTurnsPer50Devices
	turns := minTurns + extra
	if turns < minTurns {
		return minTurns
	}
	if turns > maxTurns {
		return maxTurns
	}
	return turns
}

func computeTriageMaxTurns(flagCount int, scope *PatrolScope) int {
	const (
		triageBaseTurns    = 5
		triageTurnsPerFlag = 3
		triageMinTurns     = 8
		triageMaxTurns     = 40
	)

	turns := triageBaseTurns + flagCount*triageTurnsPerFlag
	if turns < triageMinTurns {
		turns = triageMinTurns
	}
	if turns > triageMaxTurns {
		turns = triageMaxTurns
	}
	if scope != nil && scope.Depth == PatrolDepthQuick {
		if turns > 20 {
			turns = 20
		}
	}

	return turns
}

func (p *PatrolService) ensureInvestigationToolCall(
	ctx context.Context,
	executor *tools.PulseToolExecutor,
	toolCallsMu *sync.Mutex,
	completedToolCalls *[]ToolCallRecord,
	rawToolOutputs *[]string,
	noStream bool,
) {
	if executor == nil {
		return
	}

	toolCallsMu.Lock()
	needsInvestigation := true
	for _, tc := range *completedToolCalls {
		if isInvestigationTool(tc.ToolName) {
			needsInvestigation = false
			break
		}
	}
	toolCallsMu.Unlock()

	if !needsInvestigation {
		return
	}

	fallbackName := "pulse_query"
	args := map[string]interface{}{"action": "health"}
	inputBytes, _ := json.Marshal(args)
	inputStr := string(inputBytes)
	fallbackID := fmt.Sprintf("patrol-fallback-%d", time.Now().UnixNano())

	start := time.Now().UnixMilli()
	if !noStream {
		p.broadcast(PatrolStreamEvent{
			Type:         "tool_start",
			ToolID:       fallbackID,
			ToolName:     fallbackName,
			ToolInput:    inputStr,
			ToolRawInput: inputStr,
		})
	}

	result, err := executor.ExecuteTool(ctx, fallbackName, args)
	output := ""
	success := false
	if err != nil {
		output = err.Error()
	} else {
		output = formatToolResult(result)
		success = !result.IsError
	}

	end := time.Now().UnixMilli()
	if !noStream {
		p.broadcast(PatrolStreamEvent{
			Type:         "tool_end",
			ToolID:       fallbackID,
			ToolName:     fallbackName,
			ToolInput:    inputStr,
			ToolRawInput: inputStr,
			ToolOutput:   output,
			ToolSuccess:  &success,
		})
	}

	toolCallsMu.Lock()
	*completedToolCalls = append(*completedToolCalls, ToolCallRecord{
		ID:        fallbackID,
		ToolName:  fallbackName,
		Input:     truncateString(inputStr, MaxToolInputSize),
		Output:    truncateString(output, MaxToolOutputSize),
		Success:   success,
		StartTime: start,
		EndTime:   end,
		Duration:  end - start,
	})
	*rawToolOutputs = append(*rawToolOutputs, output)
	toolCallsMu.Unlock()
}

func isInvestigationTool(name string) bool {
	switch name {
	case "pulse_query", "pulse_metrics", "pulse_storage", "pulse_read":
		return true
	default:
		return false
	}
}

func formatToolResult(result tools.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}

	var text string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			if text != "" {
				text += "\n"
			}
			text += c.Text
		}
	}
	return text
}

// runEvaluationPass runs a focused second LLM call to evaluate unmatched signals
// that the main patrol pass detected but did not report as findings.
func (p *PatrolService) runEvaluationPass(ctx context.Context, adapter *patrolFindingCreatorAdapter, unmatchedSignals []DetectedSignal) (*PatrolStreamResponse, error) {
	cs := p.aiService.GetChatService()
	if cs == nil {
		return nil, fmt.Errorf("chat service not available for evaluation pass")
	}
	if err := p.aiService.CheckBudget("patrol"); err != nil {
		log.Warn().Err(err).Msg("AI Patrol: Budget exceeded, skipping evaluation pass")
		return nil, fmt.Errorf("patrol evaluation skipped: %w", err)
	}

	systemPrompt := buildEvalSystemPrompt()
	userPrompt := buildEvalUserPrompt(unmatchedSignals)

	log.Info().
		Int("unmatched_signals", len(unmatchedSignals)).
		Msg("AI Patrol: Running evaluation pass for unmatched signals")

	resp, err := cs.ExecutePatrolStream(ctx, PatrolExecuteRequest{
		Prompt:       userPrompt,
		SystemPrompt: systemPrompt,
		SessionID:    "patrol-eval",
		UseCase:      "patrol",
		MaxTurns:     5,
	}, func(event ChatStreamEvent) {
		// Minimal callback — we don't stream eval pass to the frontend
		// but findings are still created via the adapter
	})

	if err != nil {
		log.Warn().Err(err).Msg("AI Patrol: Evaluation pass failed")
		return nil, err
	}

	log.Info().
		Int("input_tokens", resp.InputTokens).
		Int("output_tokens", resp.OutputTokens).
		Msg("AI Patrol: Evaluation pass complete")
	p.recordPatrolUsage(resp.InputTokens, resp.OutputTokens)

	return resp, nil
}

func (p *PatrolService) recordPatrolUsage(inputTokens, outputTokens int) {
	if p == nil || p.aiService == nil || (inputTokens <= 0 && outputTokens <= 0) {
		return
	}

	p.aiService.mu.RLock()
	store := p.aiService.costStore
	cfg := p.aiService.cfg
	provider := p.aiService.provider
	p.aiService.mu.RUnlock()

	if store == nil {
		return
	}

	model := ""
	if cfg != nil {
		model = strings.TrimSpace(cfg.GetPatrolModel())
		if model == "" {
			model = strings.TrimSpace(cfg.GetChatModel())
		}
	}

	providerName := ""
	if model != "" {
		parts := strings.SplitN(model, ":", 2)
		if len(parts) == 2 {
			providerName = strings.TrimSpace(strings.ToLower(parts[0]))
		}
	}
	if providerName == "" && provider != nil {
		providerName = strings.TrimSpace(strings.ToLower(provider.Name()))
	}

	store.Record(cost.UsageEvent{
		Timestamp:    time.Now(),
		Provider:     providerName,
		RequestModel: model,
		UseCase:      "patrol",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
}

// buildEvalSystemPrompt returns the system prompt for the evaluation pass.
func buildEvalSystemPrompt() string {
	return `You are a patrol evaluation agent reviewing infrastructure signals that were
detected but not reported as findings.

Tools: patrol_report_finding, patrol_get_findings

Instructions:
1. Call patrol_get_findings to check what already exists.
2. For each signal below, determine if it is a genuine issue requiring attention.
3. If yes, call patrol_report_finding with complete details.
4. If not actionable or already covered by an existing finding, skip it.
5. Do NOT investigate further — use only the evidence provided below.`
}

// buildEvalUserPrompt formats the unmatched signals into a user prompt for the evaluation pass.
func buildEvalUserPrompt(signals []DetectedSignal) string {
	var sb strings.Builder
	sb.WriteString("The following infrastructure signals were detected during patrol but were not reported as findings.\n")
	sb.WriteString("Review each one and report genuine issues using patrol_report_finding.\n\n")

	for i, s := range signals {
		sb.WriteString(fmt.Sprintf("## Signal %d: %s\n", i+1, s.SignalType))
		sb.WriteString(fmt.Sprintf("- **Resource**: %s (ID: %s, Type: %s)\n", s.ResourceName, s.ResourceID, s.ResourceType))
		sb.WriteString(fmt.Sprintf("- **Suggested Severity**: %s\n", s.SuggestedSeverity))
		sb.WriteString(fmt.Sprintf("- **Category**: %s\n", s.Category))
		sb.WriteString(fmt.Sprintf("- **Summary**: %s\n", s.Summary))
		sb.WriteString(fmt.Sprintf("- **Evidence**: ```\n%s\n```\n\n", s.Evidence))
	}

	return sb.String()
}

func (p *PatrolService) createFindingsFromSignals(adapter *patrolFindingCreatorAdapter, signals []DetectedSignal) int {
	if adapter == nil || len(signals) == 0 {
		return 0
	}
	created := 0
	for _, s := range signals {
		input := signalToFindingInput(s)
		if input.ResourceName == "" {
			input.ResourceName = input.ResourceID
		}
		if input.ResourceType == "" {
			input.ResourceType = inferFindingResourceType(input.ResourceID, input.ResourceName)
		}
		if input.Category == "" {
			input.Category = "general"
		}
		if input.Severity == "" {
			input.Severity = "warning"
		}
		if input.Recommendation == "" {
			input.Recommendation = defaultRecommendationForSignal(s)
		}
		if input.Title == "" {
			input.Title = s.Summary
		}
		if input.Description == "" {
			input.Description = s.Summary
		}

		if _, _, err := adapter.CreateFinding(input); err == nil {
			created++
		}
	}
	return created
}

func signalToFindingInput(s DetectedSignal) tools.PatrolFindingInput {
	key := signalKey(s)
	category := s.Category
	severity := s.SuggestedSeverity
	return tools.PatrolFindingInput{
		Key:          key,
		Severity:     severity,
		Category:     category,
		ResourceID:   s.ResourceID,
		ResourceName: s.ResourceName,
		ResourceType: s.ResourceType,
		Title:        signalTitle(s),
		Description:  s.Summary,
		Evidence:     s.Evidence,
	}
}

func signalKey(s DetectedSignal) string {
	switch s.SignalType {
	case SignalSMARTFailure:
		return "smart-failure"
	case SignalHighCPU:
		return "cpu-high"
	case SignalHighMemory:
		return "memory-high"
	case SignalHighDisk:
		return "disk-high"
	case SignalBackupFailed:
		return "backup-failed"
	case SignalBackupStale:
		return "backup-stale"
	case SignalActiveAlert:
		return "active-alert"
	case SignalGuestUnreachable:
		return "guest-unreachable"
	default:
		return "deterministic-signal"
	}
}

func signalTitle(s DetectedSignal) string {
	switch s.SignalType {
	case SignalSMARTFailure:
		return "SMART health check failed"
	case SignalHighCPU:
		return "High CPU usage detected"
	case SignalHighMemory:
		return "High memory usage detected"
	case SignalHighDisk:
		return "Storage usage is high"
	case SignalBackupFailed:
		return "Backup failed"
	case SignalBackupStale:
		return "Backup is stale"
	case SignalActiveAlert:
		return "Active alert detected"
	case SignalGuestUnreachable:
		return fmt.Sprintf("Guest unreachable: %s", s.ResourceName)
	default:
		return "Infrastructure signal detected"
	}
}

func defaultRecommendationForSignal(s DetectedSignal) string {
	switch s.SignalType {
	case SignalSMARTFailure:
		return "Inspect the disk for errors and consider replacing it if SMART failures persist."
	case SignalHighCPU:
		return "Identify processes causing high CPU usage and optimize or scale resources."
	case SignalHighMemory:
		return "Identify memory-heavy processes and consider increasing memory or tuning workloads."
	case SignalHighDisk:
		return "Investigate disk usage growth and clean up or expand storage as needed."
	case SignalBackupFailed:
		return "Review backup logs and fix the underlying error, then rerun the backup."
	case SignalBackupStale:
		return "Ensure backups are scheduled and completing successfully; run a new backup."
	case SignalActiveAlert:
		return "Investigate the active alert and resolve the underlying issue."
	case SignalGuestUnreachable:
		return "Investigate why this guest is not responding to ping. Check network configuration, firewall rules, or whether the guest has crashed."
	default:
		return "Investigate the signal and take corrective action if needed."
	}
}

// getPatrolSystemPrompt returns the system prompt for AI patrol analysis.
// The new agentic prompt instructs the LLM to use investigation tools and
// report findings via the patrol_report_finding tool instead of text blocks.
func (p *PatrolService) getPatrolSystemPrompt() string {
	autoFix := false
	if cfg := p.aiService.GetAIConfig(); cfg != nil {
		autoFix = cfg.PatrolAutoFix
	}

	basePrompt := `You are Pulse Patrol, an autonomous infrastructure analysis agent. Your job is to find issues that simple threshold-based alerts CANNOT catch — trends, capacity risks, misconfigurations, reliability gaps, and cross-resource correlations.

Pulse already has a real-time alerting system that fires when metrics cross thresholds (CPU, memory, disk, etc.) and when resources go down. Do NOT duplicate what alerts already handle. Your value is deeper analysis that requires looking at patterns over time and across resources.

## Investigation Tools

You have access to the following tools to investigate infrastructure:

**Infrastructure State:**
- pulse_query — Search resources, get details, list resources, check health overview
- pulse_metrics — Performance metrics, temperatures, network, disk I/O, baselines, patterns
- pulse_storage — Storage pools, config, backups, snapshots, Ceph, replication, PBS jobs, RAID, disk health

**Platform-Specific:**
- pulse_docker — Container status, updates, services, swarm
- pulse_kubernetes — Clusters, nodes, pods, deployments
- pulse_pmg — Proxmox Mail Gateway status, mail stats, queues

**Deep Investigation:**
- pulse_read — Read-only command execution, file reads, log tailing
- pulse_discovery — Infrastructure discovery details
- pulse_knowledge — User notes, incidents, event correlations

**Patrol Reporting:**
- patrol_report_finding — Report a finding (creates a structured finding with validation)
- patrol_resolve_finding — Resolve an existing finding that is no longer an issue
- patrol_get_findings — Check currently active findings (use before reporting to avoid duplicates)

## How Patrol Works

You are provided with the current state of the user's infrastructure below, including resource metrics, storage health, backup status, disk health, active alerts, baselines, and connection health. This gives you a complete point-in-time snapshot without needing to query for it.

The seed context includes service identity (from discovery) and reachability data when available. Guests marked UNREACHABLE are running according to Proxmox but did not respond to ICMP ping from their host node. This may indicate a network issue, guest crash, or firewall blocking ICMP. Use pulse_read to check guest logs or pulse_discovery for service details.

**Step 1 — Analyze the snapshot.** Scan the data for anything notable: high usage, backup gaps, disk health issues, resources above baseline, stopped resources that should be running, storage trending full, unreachable guests, etc.

**Step 2 — Investigate deeper.** For anything notable you spotted, use your tools to understand whether it's actually a problem:
- Use **pulse_metrics** with historical windows (1h, 6h, 24h) to check if a high metric is trending up or just a momentary spike. A resource at 60% and rising is more interesting than one sitting steady at 75%.
- Use **pulse_read** to check logs on resources that look unhealthy or abnormal.
- Use **pulse_storage** to check snapshot ages, replication status, or backup job details.
- Use **pulse_query** to check resource configuration for misconfigurations.
- Use **pulse_pmg** to check mail queues or spam volume if mail flow looks abnormal.

**Step 3 — Report or resolve findings.** Report findings for confirmed issues. Resolve active findings that are no longer issues based on current data.
Always call patrol_get_findings before reporting or resolving findings.

The snapshot eliminates routine data gathering, but you must still investigate to distinguish real problems from noise. Do not skip investigation — a snapshot alone cannot tell you whether a metric is stable or rapidly changing.

## Efficiency Rules
- Do NOT call the same tool with the same parameters twice in a single patrol run.
- Keep track of what you've already checked. If you've already retrieved metrics for a resource, use the data you have.
- Always call at least one investigation tool (pulse_query, pulse_metrics, pulse_storage, or pulse_read) in every patrol run, even if everything appears healthy.

## Severity & Thresholds

- **critical**: Data loss risk, unrecoverable misconfiguration, complete backup failure with no retention
- **warning**: Capacity will be exhausted within 7 days at current growth rate, backup gap >48h, replication broken, security misconfiguration
- **watch**: Capacity trending toward limits (14-30 days), minor config drift, optimization opportunity
- **info**: Almost never — only for significant findings that don't fit above

These are for Patrol-specific findings (trends, capacity, config issues). Simple metric thresholds (CPU >90%, memory >95%, etc.) are handled by the alerting system — do NOT report those.

## Noise to Avoid

- "CPU at 15% vs baseline 8%" — NORMAL variance, not an issue
- "Memory at 45% which is elevated" — FINE, lots of headroom
- "Disk at 30% is above baseline" — FINE, not actionable
- Stopped containers/VMs (unless autostart is enabled AND they crashed)
- Minor metric fluctuations compared to baseline
- Resources that are simply "busier than usual" but not near limits
- Simple threshold breaches (CPU/memory/disk above X%) — alerts handle these
- Resources that are down or stopped — alerts handle these
- Any condition that a metric-crosses-threshold alert would catch

## Before Reporting a Finding, Ask Yourself

1. Would an operator need to DO something about this?
2. Is this something the real-time alerting system would catch on its own? If yes — DO NOT report it.
3. Does this require analysis, trend detection, or correlation that a simple threshold can't provide?

If everything looks healthy, report no findings. Report findings for issues that require human planning or intervention — capacity risks, misconfigurations, reliability gaps, optimization opportunities, or emerging trends. Do NOT report simple threshold breaches (high CPU, high memory, high disk, resource down) — those are handled by the alerting system.

## Final Summary Format

After completing your investigation, write a concise summary using this structure:

### Infrastructure Status
One sentence overall health verdict (e.g., "All 3 nodes and 18 guests are operating normally." or "1 warning found across 3 nodes and 12 VMs.").

### Key Observations
- Bullet each noteworthy observation with the **resource name** bolded and the metric or finding inline
- Only include items worth mentioning — skip anything completely normal
- Group related items (e.g., all storage together, all compute together)

### Actions Taken
- List each finding you reported or resolved, with its severity badge: ` + "`" + `⚠ warning` + "`" + `, ` + "`" + `🔴 critical` + "`" + `, ` + "`" + `✅ resolved` + "`" + `
- If no findings were created or resolved, write "No findings reported — all clear."

Keep the summary factual, terse, and scannable. Do NOT repeat your investigation process or thinking. Do NOT use phrases like "Let me check..." or "I'll start by..." — only state results. Maximum 15 lines.`

	if autoFix {
		return basePrompt + `

## Auto-Fix Mode

Auto-fix is enabled. You may use pulse_control and pulse_read tools to attempt automatic remediation.

Safe operations you can perform autonomously:
- Restart services (systemctl restart)
- Clear caches and temp files
- Rotate/compress logs
- Trigger garbage collection

Always:
1. Run a verification command after any fix to confirm success
2. Report findings for issues you attempted to fix (include fix outcome in evidence)
3. Stop and report if the fix doesn't resolve the issue`
	}

	return basePrompt + `

## Observe Only Mode

You are in observation mode. Use read-only tools to gather diagnostic information but DO NOT modify anything. Report findings with clear recommendations for the user to review and action manually.`
}

const triageSystemPreamble = `You are Pulse Patrol, an autonomous infrastructure analysis agent.

Deterministic triage has already scanned all resources against thresholds, baselines, backup schedules, disk health, and connectivity. The flagged items are listed in your seed context under "Deterministic Triage Results".

Your job is to investigate each flagged item deeper using tools:
- Use pulse_metrics with historical windows to check if an elevated metric is trending up or stable
- Use pulse_read to check logs on flagged resources
- Use pulse_storage to verify backup/replication/RAID details
- Use pulse_query to check resource configuration

After investigation, report confirmed issues via patrol_report_finding and resolve any active findings that are no longer problems.

Do NOT re-scan healthy resources. Triage already verified they are within normal parameters. Focus your turns exclusively on the flagged items.`

func (p *PatrolService) getPatrolSystemPromptForTriage() string {
	fullPrompt := p.getPatrolSystemPrompt()

	const toolsMarker = "## Investigation Tools"
	toolsIdx := strings.Index(fullPrompt, toolsMarker)
	if toolsIdx < 0 {
		return triageSystemPreamble + "\n\n" + fullPrompt
	}

	return triageSystemPreamble + "\n\n" + fullPrompt[toolsIdx:]
}

// seedIntelligence holds pre-computed intelligence data used by multiple seed context sections.
type seedIntelligence struct {
	anomalies        []baseline.AnomalyReport
	forecasts        []seedForecast
	predictions      []FailurePrediction
	recentChanges    []memory.Change
	correlations     []*Correlation
	isQuiet          bool
	hasBaselineStore bool
}

type seedSection struct {
	priority int
	name     string
	content  string
	summary  string
}

// seedForecast represents a capacity forecast for seed context.
type seedForecast struct {
	name, resourceID, metric, severity string
	daysToFull                         int
	dailyChange, current               float64
}

// buildTriageSeedContext builds a focused seed context from deterministic triage output.
// Unlike buildSeedContext, this includes only flagged resource details plus required context.
func (p *PatrolService) buildTriageSeedContext(
	triage *TriageResult,
	snap models.StateSnapshot,
	scope *PatrolScope,
	guestIntel map[string]*GuestIntelligence,
) (string, []string) {
	return p.buildTriageSeedContextState(triage, p.patrolRuntimeStateForSnapshot(snap), scope, guestIntel)
}

func (p *PatrolService) buildTriageSeedContextState(
	triage *TriageResult,
	snap patrolRuntimeState,
	scope *PatrolScope,
	guestIntel map[string]*GuestIntelligence,
) (string, []string) {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	if triage == nil {
		triage = &TriageResult{}
	}
	flaggedSet := triage.FlaggedIDs
	if flaggedSet == nil {
		flaggedSet = map[string]bool{}
	}

	findingsCtx, seededFindingIDs := p.seedFindingsAndContextState(scope, snap)
	now := time.Now()

	sections := []seedSection{
		// P0 — always include.
		{priority: 0, name: "triage_briefing", content: FormatTriageBriefing(triage)},
		{priority: 0, name: "findings", content: findingsCtx},
		{priority: 0, name: "health_alerts", content: p.seedHealthAndAlertsState(snap, flaggedSet, cfg, now)},
		{priority: 0, name: "scope", content: buildScopeSection(scope)},

		// P1 — flagged resource details only.
		{
			priority: 1,
			name:     "flagged_inventory",
			content:  p.seedResourceInventoryState(snap, flaggedSet, cfg, now, false, guestIntel),
			summary:  p.seedResourceInventorySummaryState(snap, flaggedSet, cfg, now, guestIntel),
		},
	}

	budget := p.calculateSeedBudget()
	result := p.assembleSeedWithinBudget(sections, budget)

	return result, seededFindingIDs
}

// buildSeedContext produces the infrastructure state context for the agentic patrol loop.
// It pre-assembles current metrics, storage health, backup status, disk health, alerts,
// connection health, and baselines/trends so the model can analyze without tool calls.
// Tools remain available for targeted deep-dives.
func (p *PatrolService) buildSeedContext(snap models.StateSnapshot, scope *PatrolScope, guestIntel map[string]*GuestIntelligence) (string, []string) {
	return p.buildSeedContextState(p.patrolRuntimeStateForSnapshot(snap), scope, guestIntel)
}

func (p *PatrolService) buildSeedContextState(snap patrolRuntimeState, scope *PatrolScope, guestIntel map[string]*GuestIntelligence) (string, []string) {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	now := time.Now()
	scopedSet := p.buildScopedSet(scope)
	intel := p.seedPrecomputeIntelligenceState(snap, scopedSet, now)
	findingsCtx, seededFindingIDs := p.seedFindingsAndContextState(scope, snap)

	sections := []seedSection{
		// P0 — always include.
		{priority: 0, name: "findings", content: findingsCtx},
		{priority: 0, name: "health_alerts", content: p.seedHealthAndAlertsState(snap, scopedSet, cfg, now)},
		{priority: 0, name: "scope", content: buildScopeSection(scope)},

		// P1 — always include (typically compact).
		{priority: 1, name: "previous_run", content: p.seedPreviousRun(now)},

		// P2 — summarize when needed.
		{
			priority: 2,
			name:     "resource_inventory",
			content:  p.seedResourceInventoryState(snap, scopedSet, cfg, now, false, guestIntel),
			summary:  p.seedResourceInventorySummaryState(snap, scopedSet, cfg, now, guestIntel),
		},

		// P3 — droppable if budget is tight.
		{priority: 3, name: "intelligence", content: p.seedIntelligenceContext(intel, now)},
		{priority: 3, name: "backup_analysis", content: p.seedBackupAnalysisState(snap, now)},

		// P4 — least critical, dropped first.
		{priority: 4, name: "pmg_snapshot", content: p.seedPMGSnapshotStringState(snap, scopedSet, cfg, intel.isQuiet)},
	}

	budget := p.calculateSeedBudget()
	result := p.assembleSeedWithinBudget(sections, budget)

	return result, seededFindingIDs
}

func (p *PatrolService) calculateSeedBudget() int {
	const (
		systemPromptEstimate = 4_000
		toolEstimate         = 8_000
		outputReserve        = 8_000
		historyReserve       = 16_000
		minimumSeedBudget    = 16_000
	)

	model := ""
	if p.aiService != nil {
		if cfg := p.aiService.GetAIConfig(); cfg != nil {
			model = strings.TrimSpace(cfg.GetPatrolModel())
		}
	}

	contextWindow := providers.ContextWindowTokens(model)
	budget := contextWindow - systemPromptEstimate - toolEstimate - outputReserve - historyReserve

	// Clamp floor so small-context models aren't forced beyond practical capacity.
	floor := minimumSeedBudget
	if halfContext := contextWindow / 2; halfContext < floor {
		floor = halfContext
	}
	if budget < floor {
		budget = floor
	}

	log.Debug().
		Str("model", model).
		Int("context_window_tokens", contextWindow).
		Int("seed_budget_tokens", budget).
		Msg("AI Patrol: Calculated seed context token budget")

	return budget
}

func (p *PatrolService) assembleSeedWithinBudget(sections []seedSection, budgetTokens int) string {
	if len(sections) == 0 {
		return ""
	}

	ordered := append([]seedSection(nil), sections...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].priority < ordered[j].priority
	})

	var sb strings.Builder
	usedTokens := 0
	included := make([]string, 0, len(ordered))
	summarized := make([]string, 0, len(ordered))
	dropped := make([]string, 0, len(ordered))

	appendSection := func(sectionName, content string) {
		sb.WriteString(content)
		usedTokens += chat.EstimateTokens(content)
		included = append(included, sectionName)
	}

	for _, section := range ordered {
		if strings.TrimSpace(section.content) == "" {
			continue
		}

		contentTokens := chat.EstimateTokens(section.content)
		switch {
		case section.priority <= 1:
			appendSection(section.name, section.content)
		case section.priority == 2:
			if usedTokens+contentTokens <= budgetTokens {
				appendSection(section.name, section.content)
			} else if strings.TrimSpace(section.summary) != "" {
				summaryTokens := chat.EstimateTokens(section.summary)
				if usedTokens+summaryTokens <= budgetTokens {
					sb.WriteString(section.summary)
					usedTokens += summaryTokens
					summarized = append(summarized, section.name)
					included = append(included, section.name)
				} else {
					dropped = append(dropped, section.name)
				}
			} else {
				dropped = append(dropped, section.name)
			}
		default:
			if usedTokens+contentTokens <= budgetTokens {
				appendSection(section.name, section.content)
			} else {
				dropped = append(dropped, section.name)
			}
		}
	}

	log.Debug().
		Int("budget_tokens", budgetTokens).
		Int("used_tokens", usedTokens).
		Bool("over_budget", usedTokens > budgetTokens).
		Strs("included", included).
		Strs("summarized", summarized).
		Strs("dropped", dropped).
		Msg("AI Patrol: Assembled seed context within budget")

	return sb.String()
}

func buildScopeSection(scope *PatrolScope) string {
	if scope == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Patrol Scope\n")
	if scope.Reason != "" {
		sb.WriteString(fmt.Sprintf("Trigger: %s\n", scope.Reason))
	}
	if scope.Context != "" {
		sb.WriteString(fmt.Sprintf("Context: %s\n", scope.Context))
	}
	if len(scope.ResourceIDs) > 0 {
		sb.WriteString(fmt.Sprintf("Focus resources: %s\n", strings.Join(scope.ResourceIDs, ", ")))
	}
	if len(scope.ResourceTypes) > 0 {
		sb.WriteString(fmt.Sprintf("Resource types: %s\n", strings.Join(scope.ResourceTypes, ", ")))
	}
	if scope.AlertID != "" {
		sb.WriteString(fmt.Sprintf("Alert ID: %s\n", scope.AlertID))
	}
	if scope.FindingID != "" {
		sb.WriteString(fmt.Sprintf("Finding ID: %s\n", scope.FindingID))
	}
	sb.WriteString(fmt.Sprintf("Depth: %s\n", scope.Depth.String()))

	if scope.Depth == PatrolDepthQuick {
		sb.WriteString("\nThis is a quick check — focus on the scoped resources, limit investigation depth.\n")
	} else {
		sb.WriteString("\nPerform thorough investigation including trends, baselines, logs, and correlations.\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

// buildScopedSet constructs the set of resource IDs in scope, expanding with correlated resources.
func (p *PatrolService) buildScopedSet(scope *PatrolScope) map[string]bool {
	if scope == nil || len(scope.ResourceIDs) == 0 {
		return nil
	}

	p.mu.RLock()
	corrDet := p.correlationDetector
	p.mu.RUnlock()

	scopedSet := make(map[string]bool)
	for _, id := range scope.ResourceIDs {
		scopedSet[id] = true
	}
	if corrDet != nil {
		for _, id := range scope.ResourceIDs {
			for _, c := range corrDet.GetCorrelationsForResource(id) {
				scopedSet[c.SourceID] = true
				scopedSet[c.TargetID] = true
			}
		}
	}
	return scopedSet
}

// seedPreviousRun returns the previous patrol run summary section.
func (p *PatrolService) seedPreviousRun(now time.Time) string {
	if p.runHistoryStore == nil {
		return ""
	}
	recent := p.runHistoryStore.GetRecent(1)
	if len(recent) == 0 {
		return ""
	}

	var sb strings.Builder
	last := recent[0]
	sb.WriteString("# Previous Patrol Run\n")
	sb.WriteString(fmt.Sprintf("- Ran: %s (duration: %s)\n", seedFormatTimeAgo(now, last.StartedAt), seedFormatDuration(last.Duration)))
	sb.WriteString(fmt.Sprintf("- Status: %s\n", last.Status))
	sb.WriteString(fmt.Sprintf("- Findings: %d new, %d existing, %d resolved, %d rejected\n",
		last.NewFindings, last.ExistingFindings, last.ResolvedFindings, last.RejectedFindings))
	if last.FindingsSummary != "" {
		sb.WriteString(fmt.Sprintf("- Summary: %s\n", last.FindingsSummary))
	}
	trigger := last.TriggerReason
	if trigger == "" {
		trigger = "scheduled"
	}
	sb.WriteString(fmt.Sprintf("- Trigger: %s\n", trigger))
	sb.WriteString("\n")
	return sb.String()
}

// seedPrecomputeIntelligence pre-computes anomalies, forecasts, predictions, changes,
// and correlations used by multiple seed context sections.
func (p *PatrolService) seedPrecomputeIntelligence(snap models.StateSnapshot, scopedSet map[string]bool, now time.Time) seedIntelligence {
	return p.seedPrecomputeIntelligenceState(p.patrolRuntimeStateForSnapshot(snap), scopedSet, now)
}

func (p *PatrolService) seedPrecomputeIntelligenceState(snap patrolRuntimeState, scopedSet map[string]bool, now time.Time) seedIntelligence {
	p.mu.RLock()
	bs := p.baselineStore
	mh := p.metricsHistory
	pd := p.patternDetector
	cd := p.changeDetector
	corrDet := p.correlationDetector
	p.mu.RUnlock()

	var intel seedIntelligence
	intel.hasBaselineStore = bs != nil
	nodeSources := patrolPrecomputeNodeSources(snap, scopedSet)
	guestSources := patrolPrecomputeGuestSources(snap, scopedSet)
	storageSources := patrolPrecomputeStorageSources(snap, scopedSet)

	// Anomalies
	if bs != nil {
		for _, n := range nodeSources {
			metrics := map[string]float64{"cpu": n.cpuFraction, "memory": n.memPercent}
			anomalies := bs.CheckResourceAnomaliesReadOnly(n.id, metrics)
			for i := range anomalies {
				if anomalies[i].ResourceName == "" {
					anomalies[i].ResourceName = n.name
				}
			}
			intel.anomalies = append(intel.anomalies, anomalies...)
		}
		for _, g := range guestSources {
			if g.template || g.status != "running" {
				continue
			}
			metrics := map[string]float64{"memory": g.memPercent, "disk": g.diskPercent}
			if g.cpuFraction > 0 {
				metrics["cpu"] = g.cpuFraction
			}
			anomalies := bs.CheckResourceAnomaliesReadOnly(g.id, metrics)
			for i := range anomalies {
				if anomalies[i].ResourceName == "" {
					anomalies[i].ResourceName = g.name
				}
			}
			intel.anomalies = append(intel.anomalies, anomalies...)
		}
		for _, s := range storageSources {
			metrics := map[string]float64{"usage": s.usagePercent}
			anomalies := bs.CheckResourceAnomaliesReadOnly(s.id, metrics)
			for i := range anomalies {
				if anomalies[i].ResourceName == "" {
					anomalies[i].ResourceName = s.name
				}
			}
			intel.anomalies = append(intel.anomalies, anomalies...)
		}
	}

	// Capacity forecasts
	if mh != nil {
		addForecast := func(resourceID, resourceName, metricName string, points []MetricPoint, currentValue float64) {
			if len(points) < 5 {
				return
			}
			samples := make([]float64, len(points))
			for i, pt := range points {
				samples[i] = pt.Value
			}
			trend := baseline.CalculateTrend(samples, currentValue)
			if trend != nil && trend.DaysToFull > 0 && trend.DaysToFull <= 30 {
				intel.forecasts = append(intel.forecasts, seedForecast{
					name:        resourceName,
					resourceID:  resourceID,
					metric:      metricName,
					severity:    trend.Severity,
					daysToFull:  trend.DaysToFull,
					dailyChange: trend.DailyChange,
					current:     currentValue,
				})
			}
		}
		for _, n := range nodeSources {
			if pts := mh.GetNodeMetrics(n.id, "memory", 48*time.Hour); len(pts) >= 5 {
				addForecast(n.id, n.name, "memory", pts, n.memPercent)
			}
		}
		for _, g := range guestSources {
			if g.template || g.status != "running" {
				continue
			}
			if pts := mh.GetGuestMetrics(g.id, "memory", 48*time.Hour); len(pts) >= 5 {
				addForecast(g.id, g.name, "memory", pts, g.memPercent)
			}
			if pts := mh.GetGuestMetrics(g.id, "disk", 48*time.Hour); len(pts) >= 5 {
				addForecast(g.id, g.name, "disk", pts, g.diskPercent)
			}
		}
		for _, s := range storageSources {
			allMetrics := mh.GetAllStorageMetrics(s.id, 48*time.Hour)
			if pts, ok := allMetrics["usage"]; ok && len(pts) >= 5 {
				addForecast(s.id, s.name, "usage", pts, s.usagePercent)
			}
		}
	}

	// Failure predictions
	if pd != nil {
		allPredictions := pd.GetPredictions()
		for _, pred := range allPredictions {
			if seedIsInScope(scopedSet, pred.ResourceID) {
				intel.predictions = append(intel.predictions, pred)
			}
		}
	}

	// Recent changes
	if cd != nil {
		allChanges := cd.GetRecentChanges(20, now.Add(-24*time.Hour))
		for _, c := range allChanges {
			if seedIsInScope(scopedSet, c.ResourceID) {
				intel.recentChanges = append(intel.recentChanges, c)
			}
		}
	}

	// Correlations
	if corrDet != nil {
		allCorrs := corrDet.GetCorrelations()
		for _, c := range allCorrs {
			if !seedIsInScope(scopedSet, c.SourceID) && !seedIsInScope(scopedSet, c.TargetID) {
				continue
			}
			intel.correlations = append(intel.correlations, c)
			if len(intel.correlations) >= 10 {
				break
			}
		}
	}

	// Determine if infrastructure is quiet
	hasWarningForecasts := false
	for _, f := range intel.forecasts {
		if f.daysToFull <= 30 {
			hasWarningForecasts = true
			break
		}
	}
	intel.isQuiet = len(intel.anomalies) == 0 && !hasWarningForecasts &&
		len(intel.predictions) == 0 && len(intel.recentChanges) == 0 && len(snap.ActiveAlerts) == 0

	return intel
}

// seedResourceInventory builds the node, guest, docker, storage, ceph, and PBS sections.
func (p *PatrolService) seedResourceInventory(snap models.StateSnapshot, scopedSet map[string]bool, cfg PatrolConfig, now time.Time, isQuiet bool, guestIntel map[string]*GuestIntelligence) string {
	return p.seedResourceInventoryState(p.patrolRuntimeStateForSnapshot(snap), scopedSet, cfg, now, isQuiet, guestIntel)
}

type patrolNodeInventoryRow struct {
	id, name, status string
	cpu, mem, disk   float64
	load             []float64
	uptimeSeconds    int64
	pendingUpdates   int
}

type patrolGuestInventoryRow struct {
	name, gType, node, status string
	cpu, mem, disk            float64
	lastBackup                time.Time
	service                   string
	reachable                 string
}

type patrolPBSDatastoreRow struct {
	instance, name string
	usage          float64
	used, total    int64
}

type patrolDockerHostRow struct {
	host                string
	containerCount      int
	runningCount        int
	stoppedCount        int
	unhealthyContainers []string
}

type patrolPrecomputeNodeSource struct {
	id, name    string
	cpuFraction float64
	memPercent  float64
}

type patrolPrecomputeGuestSource struct {
	id, name    string
	template    bool
	status      string
	cpuFraction float64
	memPercent  float64
	diskPercent float64
}

type patrolPrecomputeStorageSource struct {
	id, name     string
	usagePercent float64
}

func patrolNodeInventoryRows(snap patrolRuntimeState, scopedSet map[string]bool) []patrolNodeInventoryRow {
	rs := snap.readState
	if rs != nil {
		scopedNodes := make([]patrolNodeInventoryRow, 0, len(rs.Nodes()))
		for _, nv := range rs.Nodes() {
			if !seedIsInScope(scopedSet, nv.ID()) {
				continue
			}
			scopedNodes = append(scopedNodes, patrolNodeInventoryRow{
				id:             nv.ID(),
				name:           nv.Name(),
				status:         string(nv.Status()),
				cpu:            nv.CPUPercent(),
				mem:            nv.MemoryPercent(),
				disk:           nv.DiskPercent(),
				load:           nv.LoadAverage(),
				uptimeSeconds:  nv.Uptime(),
				pendingUpdates: nv.PendingUpdates(),
			})
		}
		return scopedNodes
	}

	scopedNodes := make([]patrolNodeInventoryRow, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		if !seedIsInScope(scopedSet, n.ID) {
			continue
		}
		scopedNodes = append(scopedNodes, patrolNodeInventoryRow{
			id:             n.ID,
			name:           n.Name,
			status:         n.Status,
			cpu:            n.CPU * 100,
			mem:            n.Memory.Usage,
			disk:           n.Disk.Usage,
			load:           n.LoadAverage,
			uptimeSeconds:  n.Uptime,
			pendingUpdates: n.PendingUpdates,
		})
	}
	return scopedNodes
}

func patrolGuestInventoryRows(snap patrolRuntimeState, scopedSet map[string]bool, guestIntel map[string]*GuestIntelligence) []patrolGuestInventoryRow {
	rs := snap.readState
	if rs != nil {
		guests := make([]patrolGuestInventoryRow, 0, len(rs.VMs())+len(rs.Containers()))
		for _, vmv := range rs.VMs() {
			if vmv.Template() || !seedIsInScope(scopedSet, vmv.ID()) {
				continue
			}
			gi := guestIntel[vmv.ID()]
			guests = append(guests, patrolGuestInventoryRow{
				name:       vmv.Name(),
				gType:      "VM",
				node:       vmv.Node(),
				status:     string(vmv.Status()),
				cpu:        vmv.CPUPercent(),
				mem:        vmv.MemoryPercent(),
				disk:       vmv.DiskPercent(),
				lastBackup: vmv.LastBackup(),
				service:    formatService(gi),
				reachable:  formatReachable(reachableFromIntel(gi)),
			})
		}
		for _, ctv := range rs.Containers() {
			if ctv.Template() || !seedIsInScope(scopedSet, ctv.ID()) {
				continue
			}
			gi := guestIntel[ctv.ID()]
			guests = append(guests, patrolGuestInventoryRow{
				name:       ctv.Name(),
				gType:      "Container",
				node:       ctv.Node(),
				status:     string(ctv.Status()),
				cpu:        ctv.CPUPercent(),
				mem:        ctv.MemoryPercent(),
				disk:       ctv.DiskPercent(),
				lastBackup: ctv.LastBackup(),
				service:    formatService(gi),
				reachable:  formatReachable(reachableFromIntel(gi)),
			})
		}
		return guests
	}

	guests := make([]patrolGuestInventoryRow, 0, len(snap.VMs)+len(snap.Containers))
	for _, vm := range snap.VMs {
		if vm.Template || !seedIsInScope(scopedSet, vm.ID) {
			continue
		}
		gi := guestIntel[vm.ID]
		guests = append(guests, patrolGuestInventoryRow{
			name:       vm.Name,
			gType:      "VM",
			node:       vm.Node,
			status:     vm.Status,
			cpu:        vm.CPU * 100,
			mem:        vm.Memory.Usage,
			disk:       vm.Disk.Usage,
			lastBackup: vm.LastBackup,
			service:    formatService(gi),
			reachable:  formatReachable(reachableFromIntel(gi)),
		})
	}
	for _, ct := range snap.Containers {
		if ct.Template || !seedIsInScope(scopedSet, ct.ID) {
			continue
		}
		gi := guestIntel[ct.ID]
		guests = append(guests, patrolGuestInventoryRow{
			name:       ct.Name,
			gType:      "Container",
			node:       ct.Node,
			status:     ct.Status,
			cpu:        ct.CPU * 100,
			mem:        ct.Memory.Usage,
			disk:       ct.Disk.Usage,
			lastBackup: ct.LastBackup,
			service:    formatService(gi),
			reachable:  formatReachable(reachableFromIntel(gi)),
		})
	}
	return guests
}

func patrolPBSDatastoreRows(snap patrolRuntimeState, scopedSet map[string]bool) []patrolPBSDatastoreRow {
	rs := snap.readState
	if rs != nil {
		rows := make([]patrolPBSDatastoreRow, 0)
		for _, pbs := range rs.PBSInstances() {
			if !seedIsInScope(scopedSet, pbs.ID()) {
				continue
			}
			instanceName := strings.TrimSpace(pbs.Name())
			if instanceName == "" {
				instanceName = strings.TrimSpace(pbs.ID())
			}
			for _, ds := range pbs.Datastores() {
				rows = append(rows, patrolPBSDatastoreRow{
					instance: instanceName,
					name:     ds.Name,
					usage:    ds.UsagePercent,
					used:     ds.Used,
					total:    ds.Total,
				})
			}
		}
		return rows
	}

	rows := make([]patrolPBSDatastoreRow, 0)
	for _, pbs := range snap.PBSInstances {
		if !seedIsInScope(scopedSet, pbs.ID) {
			continue
		}
		for _, ds := range pbs.Datastores {
			rows = append(rows, patrolPBSDatastoreRow{
				instance: pbs.Name,
				name:     ds.Name,
				usage:    ds.Usage,
				used:     ds.Used,
				total:    ds.Total,
			})
		}
	}
	return rows
}

func patrolDockerHostRows(snap patrolRuntimeState, scopedSet map[string]bool) []patrolDockerHostRow {
	rs := snap.readState
	if rs != nil {
		containersByHost := make(map[string][]*unifiedresources.DockerContainerView)
		for _, cv := range rs.DockerContainers() {
			hostID := strings.TrimSpace(cv.ParentID())
			if hostID == "" {
				hostID = strings.TrimSpace(cv.HostSourceID())
			}
			if hostID == "" {
				continue
			}
			containersByHost[hostID] = append(containersByHost[hostID], cv)
		}

		rows := make([]patrolDockerHostRow, 0, len(rs.DockerHosts()))
		for _, dhv := range rs.DockerHosts() {
			if !seedIsInScope(scopedSet, dhv.ID()) {
				continue
			}

			host := strings.TrimSpace(dhv.Hostname())
			if host == "" {
				host = strings.TrimSpace(dhv.Name())
			}

			row := patrolDockerHostRow{
				host:           host,
				containerCount: dhv.ChildCount(),
			}

			for _, cv := range containersByHost[dhv.ID()] {
				state := strings.TrimSpace(cv.ContainerState())
				if state == "running" {
					row.runningCount++
				} else {
					row.stoppedCount++
				}
				health := strings.TrimSpace(cv.Health())
				if health != "" && health != "healthy" && state == "running" {
					row.unhealthyContainers = append(row.unhealthyContainers, fmt.Sprintf("%s/%s: health=%s", host, cv.Name(), health))
				}
			}

			if len(containersByHost[dhv.ID()]) > 0 {
				row.containerCount = len(containersByHost[dhv.ID()])
			}

			rows = append(rows, row)
		}
		return rows
	}

	rows := make([]patrolDockerHostRow, 0, len(snap.DockerHosts))
	for _, dh := range snap.DockerHosts {
		if !seedIsInScope(scopedSet, dh.ID) {
			continue
		}
		row := patrolDockerHostRow{
			host:           dh.Hostname,
			containerCount: len(dh.Containers),
		}
		for _, c := range dh.Containers {
			if c.State == "running" {
				row.runningCount++
			} else {
				row.stoppedCount++
			}
			if c.Health != "" && c.Health != "healthy" && c.State == "running" {
				row.unhealthyContainers = append(row.unhealthyContainers, fmt.Sprintf("%s/%s: health=%s", dh.Hostname, c.Name, c.Health))
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func patrolPrecomputeNodeSources(snap patrolRuntimeState, scopedSet map[string]bool) []patrolPrecomputeNodeSource {
	rs := snap.readState
	if rs != nil {
		rows := make([]patrolPrecomputeNodeSource, 0, len(rs.Nodes()))
		for _, nv := range rs.Nodes() {
			if !seedIsInScope(scopedSet, nv.ID()) {
				continue
			}
			rows = append(rows, patrolPrecomputeNodeSource{
				id:          nv.ID(),
				name:        nv.Name(),
				cpuFraction: nv.CPUPercent() / 100,
				memPercent:  nv.MemoryPercent(),
			})
		}
		return rows
	}

	rows := make([]patrolPrecomputeNodeSource, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		if !seedIsInScope(scopedSet, n.ID) {
			continue
		}
		rows = append(rows, patrolPrecomputeNodeSource{
			id:          n.ID,
			name:        n.Name,
			cpuFraction: n.CPU,
			memPercent:  n.Memory.Usage,
		})
	}
	return rows
}

func patrolPrecomputeGuestSources(snap patrolRuntimeState, scopedSet map[string]bool) []patrolPrecomputeGuestSource {
	rs := snap.readState
	if rs != nil {
		rows := make([]patrolPrecomputeGuestSource, 0, len(rs.VMs())+len(rs.Containers()))
		for _, vmv := range rs.VMs() {
			if !seedIsInScope(scopedSet, vmv.ID()) {
				continue
			}
			rows = append(rows, patrolPrecomputeGuestSource{
				id:          vmv.ID(),
				name:        vmv.Name(),
				template:    vmv.Template(),
				status:      string(vmv.Status()),
				cpuFraction: vmv.CPUPercent() / 100,
				memPercent:  vmv.MemoryPercent(),
				diskPercent: vmv.DiskPercent(),
			})
		}
		for _, ctv := range rs.Containers() {
			if !seedIsInScope(scopedSet, ctv.ID()) {
				continue
			}
			rows = append(rows, patrolPrecomputeGuestSource{
				id:          ctv.ID(),
				name:        ctv.Name(),
				template:    ctv.Template(),
				status:      string(ctv.Status()),
				cpuFraction: ctv.CPUPercent() / 100,
				memPercent:  ctv.MemoryPercent(),
				diskPercent: ctv.DiskPercent(),
			})
		}
		return rows
	}

	rows := make([]patrolPrecomputeGuestSource, 0, len(snap.VMs)+len(snap.Containers))
	for _, vm := range snap.VMs {
		if !seedIsInScope(scopedSet, vm.ID) {
			continue
		}
		rows = append(rows, patrolPrecomputeGuestSource{
			id:          vm.ID,
			name:        vm.Name,
			template:    vm.Template,
			status:      vm.Status,
			cpuFraction: vm.CPU,
			memPercent:  vm.Memory.Usage,
			diskPercent: vm.Disk.Usage,
		})
	}
	for _, ct := range snap.Containers {
		if !seedIsInScope(scopedSet, ct.ID) {
			continue
		}
		rows = append(rows, patrolPrecomputeGuestSource{
			id:          ct.ID,
			name:        ct.Name,
			template:    ct.Template,
			status:      ct.Status,
			cpuFraction: ct.CPU,
			memPercent:  ct.Memory.Usage,
			diskPercent: ct.Disk.Usage,
		})
	}
	return rows
}

func patrolPrecomputeStorageSources(snap patrolRuntimeState, scopedSet map[string]bool) []patrolPrecomputeStorageSource {
	rs := snap.readState
	if rs != nil {
		rows := make([]patrolPrecomputeStorageSource, 0, len(rs.StoragePools()))
		for _, spv := range rs.StoragePools() {
			if !seedIsInScope(scopedSet, spv.ID()) {
				continue
			}
			rows = append(rows, patrolPrecomputeStorageSource{
				id:           spv.ID(),
				name:         spv.Name(),
				usagePercent: spv.DiskPercent(),
			})
		}
		return rows
	}

	rows := make([]patrolPrecomputeStorageSource, 0, len(snap.Storage))
	for _, s := range snap.Storage {
		if !seedIsInScope(scopedSet, s.ID) {
			continue
		}
		rows = append(rows, patrolPrecomputeStorageSource{
			id:           s.ID,
			name:         s.Name,
			usagePercent: s.Usage,
		})
	}
	return rows
}

func (p *PatrolService) seedResourceInventoryState(snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig, now time.Time, isQuiet bool, guestIntel map[string]*GuestIntelligence) string {
	var sb strings.Builder

	// --- Node Metrics ---
	if cfg.AnalyzeNodes {
		scopedNodes := patrolNodeInventoryRows(snap, scopedSet)

		if len(scopedNodes) > 0 {
			if isQuiet && scopedSet == nil {
				minCPU, maxCPU := 100.0, 0.0
				minMem, maxMem := 100.0, 0.0
				allHealthy := true
				for _, n := range scopedNodes {
					if n.cpu < minCPU {
						minCPU = n.cpu
					}
					if n.cpu > maxCPU {
						maxCPU = n.cpu
					}
					if n.mem < minMem {
						minMem = n.mem
					}
					if n.mem > maxMem {
						maxMem = n.mem
					}
					if n.status != "online" {
						allHealthy = false
					}
				}
				status := "healthy"
				if !allHealthy {
					status = "mixed"
				}
				sb.WriteString(fmt.Sprintf("# Nodes: All %d %s (CPU %.0f-%.0f%%, Mem %.0f-%.0f%%)\n\n",
					len(scopedNodes), status, minCPU, maxCPU, minMem, maxMem))
			} else {
				sb.WriteString("# Node Metrics\n")
				sb.WriteString("| Node | Status | CPU | Mem | Disk | Load (1/5/15) | Uptime | Updates |\n")
				sb.WriteString("|------|--------|-----|-----|------|---------------|--------|---------|\n")
				for _, n := range scopedNodes {
					load := "—"
					if len(n.load) >= 3 {
						load = fmt.Sprintf("%.1f/%.1f/%.1f", n.load[0], n.load[1], n.load[2])
					}
					uptime := seedFormatDuration(time.Duration(n.uptimeSeconds) * time.Second)
					updates := "—"
					if n.pendingUpdates > 0 {
						updates = fmt.Sprintf("%d", n.pendingUpdates)
					}
					sb.WriteString(fmt.Sprintf("| %s | %s | %.0f%% | %.0f%% | %.0f%% | %s | %s | %s |\n",
						n.name, n.status, n.cpu, n.mem, n.disk, load, uptime, updates))
				}
				sb.WriteString("\n")
			}
		}
	}

	// --- Guest Metrics (VMs + Containers in one table) ---
	var guests []patrolGuestInventoryRow

	if cfg.AnalyzeGuests {
		guests = patrolGuestInventoryRows(snap, scopedSet, guestIntel)
	}

	if len(guests) > 0 {
		if isQuiet && scopedSet == nil {
			running, stopped := 0, 0
			var unreachableNames []string
			for _, g := range guests {
				if g.status == "running" {
					running++
				} else {
					stopped++
				}
				if g.reachable == "NO" {
					unreachableNames = append(unreachableNames, g.name)
				}
			}
			if len(unreachableNames) > 0 {
				sb.WriteString(fmt.Sprintf("# Guests: %d running, %d stopped. %d UNREACHABLE: %s\n\n",
					running, stopped, len(unreachableNames), strings.Join(unreachableNames, ", ")))
			} else {
				hasReachabilityData := false
				for _, g := range guests {
					if g.reachable != "-" {
						hasReachabilityData = true
						break
					}
				}
				if hasReachabilityData {
					sb.WriteString(fmt.Sprintf("# Guests: %d running, %d stopped, no issues detected. All reachable.\n\n", running, stopped))
				} else {
					sb.WriteString(fmt.Sprintf("# Guests: %d running, %d stopped, no issues detected.\n\n", running, stopped))
				}
			}
		} else {
			sb.WriteString("# Guest Metrics\n")
			sb.WriteString("| Name | Type | Node | Service | CPU | Mem | Disk | Status | Reachable | Last Backup |\n")
			sb.WriteString("|------|------|------|---------|-----|-----|------|--------|-----------|-------------|\n")
			for _, g := range guests {
				backup := "never"
				if !g.lastBackup.IsZero() {
					backup = seedFormatTimeAgo(now, g.lastBackup)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %.0f%% | %.0f%% | %.0f%% | %s | %s | %s |\n",
					g.name, g.gType, g.node, g.service, g.cpu, g.mem, g.disk, g.status, g.reachable, backup))
			}
			sb.WriteString("\n")

			// Add service health issues section for unreachable running guests
			var issues []serviceHealthIssue
			for _, g := range guests {
				if g.status == "running" && g.reachable == "NO" {
					svc := g.service
					if svc == "-" {
						svc = g.gType // "VM" or "Container"
					}
					issues = append(issues, serviceHealthIssue{
						name:    g.name,
						service: svc,
						node:    g.node,
					})
				}
			}
			if section := buildServiceHealthIssues(issues); section != "" {
				sb.WriteString(section)
			}
		}
	}

	// --- Docker ---
	if cfg.AnalyzeDocker {
		rows := patrolDockerHostRows(snap, scopedSet)
		if len(rows) > 0 {
			sb.WriteString("# Docker\n")
			sb.WriteString("| Host | Containers | Running | Stopped |\n")
			sb.WriteString("|------|------------|---------|--------|\n")
			for _, row := range rows {
				sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d |\n",
					row.host, row.containerCount, row.runningCount, row.stoppedCount))
			}
			for _, row := range rows {
				for _, issue := range row.unhealthyContainers {
					sb.WriteString("- " + issue + "\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	// --- Storage Pools ---
	if cfg.AnalyzeStorage {
		urp := snap.unifiedResourceProvider
		if urp != nil {
			storageResources := urp.GetByType(unifiedresources.ResourceTypeStorage)
			if len(storageResources) > 0 {
				type seedStorageRow struct {
					id, name, stype, node, status string
					used, total                   int64
					hasBytes                      bool
					usage                         float64
					zfsRead, zfsWrite, zfsCksum   int64
					hasZFSErrors                  bool
				}

				rows := make([]seedStorageRow, 0, len(storageResources))
				for _, r := range storageResources {
					if scopedSet != nil && !seedIsInScope(scopedSet, r.ID) {
						continue
					}
					if r.Storage == nil {
						continue
					}

					name := strings.TrimSpace(r.Name)
					if name == "" {
						name = strings.TrimSpace(r.ID)
					}
					stype := strings.TrimSpace(r.Storage.Type)
					if stype == "" {
						stype = "-"
					}

					node := ""
					if r.Proxmox != nil {
						node = strings.TrimSpace(r.Proxmox.NodeName)
					}
					if node == "" && r.Storage.Shared {
						node = "shared"
					}

					used, total := int64(0), int64(0)
					hasBytes := false
					usage := 0.0
					if r.Metrics != nil && r.Metrics.Disk != nil {
						if r.Metrics.Disk.Used != nil && r.Metrics.Disk.Total != nil {
							used, total = *r.Metrics.Disk.Used, *r.Metrics.Disk.Total
							hasBytes = true
						}
						if r.Metrics.Disk.Percent > 0 {
							usage = r.Metrics.Disk.Percent
						} else if hasBytes && total > 0 {
							usage = (float64(used) / float64(total)) * 100
						}
					}

					status := "active"
					switch r.Status {
					case unifiedresources.StatusOffline:
						status = "inactive"
					case unifiedresources.StatusUnknown:
						status = "unknown"
					}
					if r.Storage.IsZFS && strings.TrimSpace(r.Storage.ZFSPoolState) != "" {
						status = strings.TrimSpace(r.Storage.ZFSPoolState)
					}

					zfsRead := r.Storage.ZFSReadErrors
					zfsWrite := r.Storage.ZFSWriteErrors
					zfsCksum := r.Storage.ZFSChecksumErrors
					hasZFSErrors := r.Storage.IsZFS && (zfsRead > 0 || zfsWrite > 0 || zfsCksum > 0)

					rows = append(rows, seedStorageRow{
						id:           r.ID,
						name:         name,
						stype:        stype,
						node:         node,
						status:       status,
						used:         used,
						total:        total,
						hasBytes:     hasBytes,
						usage:        usage,
						zfsRead:      zfsRead,
						zfsWrite:     zfsWrite,
						zfsCksum:     zfsCksum,
						hasZFSErrors: hasZFSErrors,
					})
				}

				if len(rows) > 0 {
					sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
					if isQuiet && scopedSet == nil {
						minUsage, maxUsage := 100.0, 0.0
						for _, row := range rows {
							if row.usage < minUsage {
								minUsage = row.usage
							}
							if row.usage > maxUsage {
								maxUsage = row.usage
							}
						}
						sb.WriteString(fmt.Sprintf("# Storage: %d pools, all within normal range (%.0f-%.0f%% used).\n\n",
							len(rows), minUsage, maxUsage))
					} else {
						sb.WriteString("# Storage\n")
						sb.WriteString("| Pool | Type | Node | Usage | Used | Total | Status |\n")
						sb.WriteString("|------|------|------|-------|------|-------|--------|\n")
						for _, row := range rows {
							usedStr, totalStr := "—", "—"
							if row.hasBytes {
								usedStr, totalStr = seedFormatBytes(row.used), seedFormatBytes(row.total)
							}
							node := row.node
							if node == "" {
								node = "—"
							}
							sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f%% | %s | %s | %s |\n",
								row.name, row.stype, node, row.usage, usedStr, totalStr, row.status))
						}
						for _, row := range rows {
							if row.hasZFSErrors {
								sb.WriteString(fmt.Sprintf("- %s ZFS errors: read=%d write=%d checksum=%d\n",
									row.name, row.zfsRead, row.zfsWrite, row.zfsCksum))
							}
						}
						sb.WriteString("\n")
					}
				}
			}
		}
	}

	// --- Ceph Clusters ---
	urp := snap.unifiedResourceProvider
	if urp != nil {
		cephResources := urp.GetByType(unifiedresources.ResourceTypeCeph)
		if len(cephResources) > 0 {
			sb.WriteString("# Ceph\n")
			for _, r := range cephResources {
				if r.Ceph == nil {
					continue
				}
				c := r.Ceph
				usedBytes, totalBytes := seedCephBytes(r)
				usagePercent := 0.0
				if totalBytes > 0 {
					usagePercent = float64(usedBytes) / float64(totalBytes) * 100
				}
				sb.WriteString(fmt.Sprintf("- %s: %s — %.0f%% used (%s / %s), %d OSDs (%d up, %d in)\n",
					r.Name, c.HealthStatus, usagePercent,
					seedFormatBytes(usedBytes), seedFormatBytes(totalBytes),
					c.NumOSDs, c.NumOSDsUp, c.NumOSDsIn))
				if c.HealthMessage != "" && c.HealthStatus != "HEALTH_OK" {
					sb.WriteString(fmt.Sprintf("  Message: %s\n", c.HealthMessage))
				}
			}
			sb.WriteString("\n")
		}
	}

	// --- PBS Instances ---
	if cfg.AnalyzePBS {
		rows := patrolPBSDatastoreRows(snap, scopedSet)
		if len(rows) > 0 {
			sb.WriteString("# PBS Datastores\n")
			for _, row := range rows {
				sb.WriteString(fmt.Sprintf("- %s/%s: %.0f%% used (%s / %s)\n",
					row.instance, row.name, row.usage,
					seedFormatBytes(row.used), seedFormatBytes(row.total)))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// seedResourceInventorySummary builds a compact, always-condensed inventory snapshot.
// Unlike seedResourceInventory quiet mode, this summary condenses even when scoped.
func (p *PatrolService) seedResourceInventorySummary(snap models.StateSnapshot, scopedSet map[string]bool, cfg PatrolConfig, now time.Time, guestIntel map[string]*GuestIntelligence) string {
	return p.seedResourceInventorySummaryState(p.patrolRuntimeStateForSnapshot(snap), scopedSet, cfg, now, guestIntel)
}

func (p *PatrolService) seedResourceInventorySummaryState(snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig, now time.Time, guestIntel map[string]*GuestIntelligence) string {
	_ = now

	type compactResource struct {
		name, status string
		cpu, mem     float64
		disk         float64
	}

	var lines []string

	// --- Nodes ---
	if cfg.AnalyzeNodes {
		nodeRows := patrolNodeInventoryRows(snap, scopedSet)
		nodes := make([]compactResource, 0, len(nodeRows))
		for _, n := range nodeRows {
			nodes = append(nodes, compactResource{
				name:   n.name,
				status: n.status,
				cpu:    n.cpu,
				mem:    n.mem,
				disk:   n.disk,
			})
		}

		if len(nodes) > 0 {
			statusCounts := map[string]int{}
			minCPU, maxCPU := nodes[0].cpu, nodes[0].cpu
			minMem, maxMem := nodes[0].mem, nodes[0].mem
			outliers := []string{}
			for _, n := range nodes {
				statusCounts[n.status]++
				if n.cpu < minCPU {
					minCPU = n.cpu
				}
				if n.cpu > maxCPU {
					maxCPU = n.cpu
				}
				if n.mem < minMem {
					minMem = n.mem
				}
				if n.mem > maxMem {
					maxMem = n.mem
				}
				if outlier, ok := seedOutlierLabel(n.name, n.cpu, n.mem, n.disk); ok {
					outliers = append(outliers, outlier)
				}
			}

			line := fmt.Sprintf("Nodes: %d (%s), CPU %.0f-%.0f%%, Mem %.0f-%.0f%%",
				len(nodes),
				seedFormatStatusBreakdown(statusCounts, []string{"online", "offline", "unknown"}),
				minCPU, maxCPU, minMem, maxMem)
			if len(outliers) > 0 {
				line += fmt.Sprintf(". High usage: %s", seedTruncateOutlierList(outliers, 5))
			}
			lines = append(lines, line)
		}
	}

	// --- Guests ---
	if cfg.AnalyzeGuests {
		guestRows := patrolGuestInventoryRows(snap, scopedSet, guestIntel)
		guests := make([]compactResource, 0, len(guestRows))
		for _, g := range guestRows {
			guests = append(guests, compactResource{
				name:   g.name,
				status: g.status,
				cpu:    g.cpu,
				mem:    g.mem,
				disk:   g.disk,
			})
		}

		if len(guests) > 0 {
			statusCounts := map[string]int{}
			outliers := []string{}
			for _, g := range guests {
				statusCounts[g.status]++
				if outlier, ok := seedOutlierLabel(g.name, g.cpu, g.mem, g.disk); ok {
					outliers = append(outliers, outlier)
				}
			}

			line := fmt.Sprintf("Guests: %d (%s)",
				len(guests),
				seedFormatStatusBreakdown(statusCounts, []string{"running", "stopped", "paused"}))
			if len(outliers) > 0 {
				line += fmt.Sprintf(". High usage: %s", seedTruncateOutlierList(outliers, 5))
			}

			unreachable := 0
			for id, intel := range guestIntel {
				if !seedIsInScope(scopedSet, id) || intel == nil || intel.Reachable == nil || *intel.Reachable {
					continue
				}
				unreachable++
			}
			if unreachable > 0 {
				line += fmt.Sprintf(". Unreachable: %d", unreachable)
			}

			lines = append(lines, line)
		}
	}

	// --- Storage ---
	if cfg.AnalyzeStorage {
		type storageRow struct {
			name, status string
			usage        float64
		}

		urp := snap.unifiedResourceProvider

		rows := []storageRow{}
		if urp != nil {
			for _, r := range urp.GetByType(unifiedresources.ResourceTypeStorage) {
				if scopedSet != nil && !seedIsInScope(scopedSet, r.ID) {
					continue
				}
				if r.Storage == nil {
					continue
				}

				name := strings.TrimSpace(r.Name)
				if name == "" {
					name = strings.TrimSpace(r.ID)
				}
				status := "active"
				switch r.Status {
				case unifiedresources.StatusOffline:
					status = "inactive"
				case unifiedresources.StatusUnknown:
					status = "unknown"
				}
				if r.Storage.IsZFS && strings.TrimSpace(r.Storage.ZFSPoolState) != "" {
					status = strings.TrimSpace(r.Storage.ZFSPoolState)
				}

				usage := 0.0
				if r.Metrics != nil && r.Metrics.Disk != nil {
					if r.Metrics.Disk.Percent > 0 {
						usage = r.Metrics.Disk.Percent
					} else if r.Metrics.Disk.Used != nil && r.Metrics.Disk.Total != nil && *r.Metrics.Disk.Total > 0 {
						usage = (float64(*r.Metrics.Disk.Used) / float64(*r.Metrics.Disk.Total)) * 100
					}
				}

				rows = append(rows, storageRow{name: name, status: status, usage: usage})
			}
		} else {
			for _, s := range snap.Storage {
				if !seedIsInScope(scopedSet, s.ID) {
					continue
				}
				rows = append(rows, storageRow{name: s.Name, status: s.Status, usage: s.Usage})
			}
		}

		if len(rows) > 0 {
			statusCounts := map[string]int{}
			outliers := []string{}
			for _, row := range rows {
				statusCounts[row.status]++
				if row.usage > 80 {
					outliers = append(outliers, fmt.Sprintf("%s (%.0f%%)", row.name, row.usage))
				}
			}

			line := fmt.Sprintf("Storage: %d pools (%s)",
				len(rows), seedFormatStatusBreakdown(statusCounts, []string{"active", "online", "inactive", "unknown"}))
			if len(outliers) > 0 {
				line += fmt.Sprintf(". High usage: %s", seedTruncateOutlierList(outliers, 5))
			}
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Infrastructure Summary (condensed)\n")
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

func (p *PatrolService) seedPMGSnapshotString(snap models.StateSnapshot, scopedSet map[string]bool, cfg PatrolConfig, isQuiet bool) string {
	return p.seedPMGSnapshotStringState(p.patrolRuntimeStateForSnapshot(snap), scopedSet, cfg, isQuiet)
}

func (p *PatrolService) seedPMGSnapshotStringState(snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig, isQuiet bool) string {
	var sb strings.Builder
	p.seedPMGSnapshotState(&sb, snap, scopedSet, cfg, isQuiet)
	return sb.String()
}

// seedPMGSnapshot adds Proxmox Mail Gateway status to the seed context
func (p *PatrolService) seedPMGSnapshotState(sb *strings.Builder, snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig, isQuiet bool) {
	if !cfg.AnalyzePMG {
		return
	}

	rs := snap.readState

	if rs == nil {
		return
	}

	pmgViews := rs.PMGInstances()
	if len(pmgViews) == 0 {
		return
	}

	var scopedPMG []*unifiedresources.PMGInstanceView
	for _, pmgv := range pmgViews {
		if seedIsInScope(scopedSet, pmgv.ID()) {
			scopedPMG = append(scopedPMG, pmgv)
		}
	}
	if len(scopedPMG) == 0 {
		return
	}

	if isQuiet && scopedSet == nil {
		allHealthy := true
		for _, pmgv := range scopedPMG {
			if string(pmgv.Status()) != "online" {
				allHealthy = false
				break
			}
			if stats := pmgv.MailStats(); stats != nil && stats.AverageProcessTimeMs > 5000 {
				allHealthy = false
				break
			}
		}
		if allHealthy {
			sb.WriteString(fmt.Sprintf("# PMG: %d gateways, all healthy and processing mail normally.\n\n", len(scopedPMG)))
			return
		}
	}

	sb.WriteString("# Proxmox Mail Gateway (PMG)\n")
	sb.WriteString("| Instance | Status | Version | In/Out | Spam/Virus | Avg Time | Queue (Active/Deferred/Hold) |\n")
	sb.WriteString("|----------|--------|---------|--------|------------|----------|------------------------------|\n")

	for _, pmgv := range scopedPMG {
		version := strings.TrimSpace(pmgv.Version())
		if version == "" {
			version = "—"
		}

		traffic := "—"
		spamVirus := "—"
		avgTime := "—"

		if stats := pmgv.MailStats(); stats != nil {
			traffic = fmt.Sprintf("%.0f/%.0f", stats.CountIn, stats.CountOut)
			spamVirus = fmt.Sprintf("%.0f/%.0f", stats.SpamIn+stats.SpamOut, stats.VirusIn+stats.VirusOut)
			avgTime = fmt.Sprintf("%.0fms", stats.AverageProcessTimeMs)
		}

		queueStr := fmt.Sprintf("%d/%d/%d", pmgv.QueueActive(), pmgv.QueueDeferred(), pmgv.QueueHold())

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			pmgv.Name(), string(pmgv.Status()), version, traffic, spamVirus, avgTime, queueStr))
	}
	sb.WriteString("\n")
}

// seedBackupAnalysis builds the backup status section.
func (p *PatrolService) seedBackupAnalysis(snap models.StateSnapshot, now time.Time) string {
	return p.seedBackupAnalysisState(p.patrolRuntimeStateForSnapshot(snap), now)
}

func (p *PatrolService) seedBackupAnalysisState(snap patrolRuntimeState, now time.Time) string {
	type backupInfo struct {
		lastBackup time.Time
		source     string
	}
	guestBackups := make(map[string]*backupInfo)

	vmidToName := make(map[int]string)
	rs := snap.readState
	// Uses canonical ReadState view surface — legacy state fallbacks removed.
	if rs == nil {
		log.Warn().Msg("seedBackupAnalysis: ReadState not wired, backup analysis will be incomplete")
	}
	if rs != nil {
		for _, vmv := range rs.VMs() {
			if id := vmv.VMID(); id > 0 {
				vmidToName[id] = vmv.Name()
			}
		}
		for _, ctv := range rs.Containers() {
			if id := ctv.VMID(); id > 0 {
				vmidToName[id] = ctv.Name()
			}
		}
	}

	for _, bt := range snap.PVEBackups.BackupTasks {
		if bt.Status != "OK" {
			continue
		}
		name := vmidToName[bt.VMID]
		if name == "" {
			name = fmt.Sprintf("vmid-%d", bt.VMID)
		}
		if existing, ok := guestBackups[name]; !ok || bt.EndTime.After(existing.lastBackup) {
			guestBackups[name] = &backupInfo{lastBackup: bt.EndTime, source: "pve"}
		}
	}

	for _, stb := range snap.PVEBackups.StorageBackups {
		name := vmidToName[stb.VMID]
		if name == "" {
			name = fmt.Sprintf("vmid-%d", stb.VMID)
		}
		if existing, ok := guestBackups[name]; !ok || stb.Time.After(existing.lastBackup) {
			guestBackups[name] = &backupInfo{lastBackup: stb.Time, source: "pve"}
		}
	}

	for _, pb := range snap.PBSBackups {
		name := pb.VMID
		if id, err := strconv.Atoi(pb.VMID); err == nil {
			if n := vmidToName[id]; n != "" {
				name = n
			}
		}
		if existing, ok := guestBackups[name]; !ok || pb.BackupTime.After(existing.lastBackup) {
			guestBackups[name] = &backupInfo{lastBackup: pb.BackupTime, source: "pbs"}
		}
	}

	if rs != nil {
		for _, vmv := range rs.VMs() {
			if vmv.Template() || vmv.LastBackup().IsZero() {
				continue
			}
			name := vmv.Name()
			if existing, ok := guestBackups[name]; !ok || vmv.LastBackup().After(existing.lastBackup) {
				guestBackups[name] = &backupInfo{lastBackup: vmv.LastBackup(), source: "pve"}
			}
		}
		for _, ctv := range rs.Containers() {
			if ctv.Template() || ctv.LastBackup().IsZero() {
				continue
			}
			name := ctv.Name()
			if existing, ok := guestBackups[name]; !ok || ctv.LastBackup().After(existing.lastBackup) {
				guestBackups[name] = &backupInfo{lastBackup: ctv.LastBackup(), source: "pve"}
			}
		}
	}

	totalGuests := 0
	if rs != nil {
		for _, vmv := range rs.VMs() {
			if !vmv.Template() {
				totalGuests++
			}
		}
		for _, ctv := range rs.Containers() {
			if !ctv.Template() {
				totalGuests++
			}
		}
	}

	if totalGuests == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Backup Status\n")

	var staleGuests []string
	recentCount := 0
	threshold48h := now.Add(-48 * time.Hour)

	allGuestNames := make(map[string]bool)
	if rs != nil {
		for _, vmv := range rs.VMs() {
			if !vmv.Template() {
				allGuestNames[vmv.Name()] = true
			}
		}
		for _, ctv := range rs.Containers() {
			if !ctv.Template() {
				allGuestNames[ctv.Name()] = true
			}
		}
	}

	for name := range allGuestNames {
		info, hasBackup := guestBackups[name]
		if !hasBackup {
			staleGuests = append(staleGuests, fmt.Sprintf("%s (never)", name))
		} else if info.lastBackup.Before(threshold48h) {
			staleGuests = append(staleGuests, fmt.Sprintf("%s (last: %s)", name, seedFormatTimeAgo(now, info.lastBackup)))
		} else {
			recentCount++
		}
	}

	sort.Strings(staleGuests)

	if len(staleGuests) > 0 {
		sb.WriteString(fmt.Sprintf("Guests with no backup in >48h: %s\n", strings.Join(staleGuests, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Guests with recent backups: %d/%d\n", recentCount, totalGuests))
	sb.WriteString("\n")
	return sb.String()
}

// seedHealthAndAlerts builds the disk health, alerts, connection health, kubernetes, and hosts sections.
func (p *PatrolService) seedHealthAndAlerts(snap models.StateSnapshot, scopedSet map[string]bool, cfg PatrolConfig, now time.Time) string {
	return p.seedHealthAndAlertsState(p.patrolRuntimeStateForSnapshot(snap), scopedSet, cfg, now)
}

func (p *PatrolService) seedHealthAndAlertsState(snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig, now time.Time) string {
	var sb strings.Builder

	rs := snap.readState
	if rs == nil && (cfg.AnalyzeKubernetes || cfg.AnalyzeHosts) {
		log.Warn().Msg("seedHealthAndAlerts: ReadState not wired, Kubernetes/Hosts sections will be omitted")
	}

	// --- Disk Health ---
	diskURP := snap.unifiedResourceProvider
	if diskURP != nil {
		diskResources := diskURP.GetByType(unifiedresources.ResourceTypePhysicalDisk)
		if len(diskResources) > 0 {
			hasIssues := false
			for _, r := range diskResources {
				if r.PhysicalDisk == nil {
					continue
				}
				d := r.PhysicalDisk
				if d.Health != "PASSED" || (d.Wearout > 0 && d.Wearout < 20) || d.Temperature > 55 {
					hasIssues = true
					break
				}
			}
			sb.WriteString("# Disk Health\n")
			if !hasIssues {
				sb.WriteString(fmt.Sprintf("All %d disks healthy (SMART PASSED).\n", len(diskResources)))
			} else {
				sb.WriteString("| Node | Device | Model | Health | Wearout | Temp |\n")
				sb.WriteString("|------|--------|-------|--------|---------|------|\n")
				for _, r := range diskResources {
					if r.PhysicalDisk == nil {
						continue
					}
					d := r.PhysicalDisk
					node := r.ParentName
					if node == "" && len(r.Identity.Hostnames) > 0 {
						node = r.Identity.Hostnames[0]
					}
					wearout := "—"
					if d.Wearout >= 0 {
						wearout = fmt.Sprintf("%d%%", d.Wearout)
					}
					temp := "—"
					if d.Temperature > 0 {
						temp = fmt.Sprintf("%d°C", d.Temperature)
					}
					sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
						node, d.DevPath, d.Model, d.Health, wearout, temp))
				}
			}
			sb.WriteString("\n")
		}
	}

	// --- Active Alerts ---
	if len(snap.ActiveAlerts) > 0 {
		sb.WriteString("# Active Alerts\n")
		for _, a := range snap.ActiveAlerts {
			since := seedFormatTimeAgo(now, a.StartTime)
			sb.WriteString(fmt.Sprintf("- [%s] %s — since %s\n", a.Level, a.Message, since))
		}
		sb.WriteString("\n")
	}

	// --- Recently Resolved Alerts ---
	if len(snap.RecentlyResolved) > 0 {
		sb.WriteString("# Recently Resolved Alerts\n")
		for _, r := range snap.RecentlyResolved {
			ago := seedFormatTimeAgo(now, r.ResolvedTime)
			sb.WriteString(fmt.Sprintf("- %s — resolved %s\n", r.Alert.Message, ago))
		}
		sb.WriteString("\n")
	}

	// --- Connection Health ---
	if len(snap.ConnectionHealth) > 0 {
		allConnected := true
		var disconnected []string
		for name, healthy := range snap.ConnectionHealth {
			if !healthy {
				allConnected = false
				disconnected = append(disconnected, name)
			}
		}
		sb.WriteString("# Connections\n")
		if allConnected {
			sb.WriteString(fmt.Sprintf("All %d instances connected.\n", len(snap.ConnectionHealth)))
		} else {
			sort.Strings(disconnected)
			sb.WriteString(fmt.Sprintf("Disconnected: %s\n", strings.Join(disconnected, ", ")))
			sb.WriteString(fmt.Sprintf("Connected: %d/%d\n",
				len(snap.ConnectionHealth)-len(disconnected), len(snap.ConnectionHealth)))
		}
		sb.WriteString("\n")
	}

	// --- Kubernetes ---
	// Uses canonical ReadState view surface — legacy state fallbacks removed.
	if cfg.AnalyzeKubernetes && rs != nil {
		k8sViews := rs.K8sClusters()
		if len(k8sViews) > 0 {
			sb.WriteString("# Kubernetes Clusters\n")
			for _, kv := range k8sViews {
				if !seedIsInScope(scopedSet, kv.ID()) {
					continue
				}
				sb.WriteString(fmt.Sprintf("- %s (Nodes: %d)\n", kv.Name(), kv.ChildCount()))
			}
			sb.WriteString("\n")
		}
	}

	// --- Hosts ---
	// Uses canonical ReadState view surface — legacy state fallbacks removed.
	if cfg.AnalyzeHosts && rs != nil {
		hosts := rs.Hosts()
		if len(hosts) > 0 {
			sb.WriteString("# Hosts\n")
			for _, hv := range hosts {
				if !seedIsInScope(scopedSet, hv.ID()) {
					continue
				}
				name := hv.Hostname()
				if strings.TrimSpace(name) == "" {
					name = hv.Name()
				}
				sb.WriteString(fmt.Sprintf("- %s (ID: %s)\n", name, hv.ID()))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// seedIntelligenceContext builds the anomalies, forecasts, predictions, changes, and correlations sections.
func (p *PatrolService) seedIntelligenceContext(intel seedIntelligence, now time.Time) string {
	var sb strings.Builder

	// --- Anomalies ---
	if intel.hasBaselineStore {
		sb.WriteString("# Anomalies\n")
		if len(intel.anomalies) == 0 {
			sb.WriteString("No anomalies detected. All resources within learned baseline ranges.\n")
		} else {
			for _, a := range intel.anomalies {
				name := a.ResourceName
				if name == "" {
					name = a.ResourceID
				}
				currentDisp := a.CurrentValue
				baselineDisp := a.BaselineMean
				if a.Metric == "cpu" {
					currentDisp *= 100
					baselineDisp *= 100
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s %s: %.1fσ above baseline (current: %.0f%%, baseline: %.0f%%)\n",
					a.Severity, name, a.Metric, a.ZScore, currentDisp, baselineDisp))
			}
		}
		sb.WriteString("\n")
	}

	// --- Capacity Forecasts ---
	if len(intel.forecasts) > 0 {
		sb.WriteString("# Capacity Forecasts\n")
		for _, f := range intel.forecasts {
			sb.WriteString(fmt.Sprintf("- [%s] %s %s: full in ~%d days (current: %.0f%%, growing +%.1f%%/day)\n",
				f.severity, f.name, f.metric, f.daysToFull, f.current, f.dailyChange))
		}
		sb.WriteString("\n")
	}

	// --- Failure Predictions ---
	if len(intel.predictions) > 0 {
		sb.WriteString("# Failure Predictions\n")
		sb.WriteString("Based on historical patterns of recurring events:\n")
		for _, pred := range intel.predictions {
			name := pred.ResourceID
			sb.WriteString(fmt.Sprintf("- %s: %s predicted in %.0f days (confidence: %.0f%%) — %s\n",
				name, string(pred.EventType), pred.DaysUntil, pred.Confidence*100, pred.Basis))
		}
		sb.WriteString("\n")
	}

	// --- Recent Infrastructure Changes ---
	if len(intel.recentChanges) > 0 {
		sb.WriteString("# Recent Infrastructure Changes (last 24h)\n")
		for _, c := range intel.recentChanges {
			name := c.ResourceName
			if name == "" {
				name = c.ResourceID
			}
			ago := seedFormatTimeAgo(now, c.DetectedAt)
			sb.WriteString(fmt.Sprintf("- %s (%s): %s (%s)\n", name, c.ResourceType, c.Description, ago))
		}
		sb.WriteString("\n")
	}

	// --- Known Resource Correlations ---
	if len(intel.correlations) > 0 {
		sb.WriteString("# Known Resource Correlations\n")
		for _, c := range intel.correlations {
			sourceEvent := c.EventPattern
			if parts := strings.SplitN(c.EventPattern, " -> ", 2); len(parts) == 2 {
				sourceEvent = parts[0]
			}
			sourceName := c.SourceName
			if sourceName == "" {
				sourceName = c.SourceID
			}
			targetName := c.TargetName
			if targetName == "" {
				targetName = c.TargetID
			}
			sb.WriteString(fmt.Sprintf("- When %s experiences %s, %s usually follows within %s (confidence: %.0f%%, seen %dx)\n",
				sourceName, sourceEvent, targetName, seedFormatDuration(c.AvgDelay), c.Confidence*100, c.Occurrences))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// seedFindingsAndContext builds the thresholds, active findings, dismissed findings, and user notes sections.
func (p *PatrolService) seedFindingsAndContext(scope *PatrolScope, snap models.StateSnapshot) (string, []string) {
	return p.seedFindingsAndContextState(scope, p.patrolRuntimeStateForSnapshot(snap))
}

func (p *PatrolService) seedFindingsAndContextState(scope *PatrolScope, snap patrolRuntimeState) (string, []string) {
	var sb strings.Builder

	// --- Alert Thresholds ---
	p.mu.RLock()
	thresholds := p.thresholds
	p.mu.RUnlock()
	sb.WriteString("# Alert Thresholds\n")
	sb.WriteString(fmt.Sprintf("- Node CPU warning: %.0f%%\n", thresholds.NodeCPUWarning))
	sb.WriteString(fmt.Sprintf("- Node Memory warning: %.0f%%\n", thresholds.NodeMemWarning))
	sb.WriteString(fmt.Sprintf("- Guest Memory warning: %.0f%%\n", thresholds.GuestMemWarning))
	sb.WriteString(fmt.Sprintf("- Guest Disk warning: %.0f%%, critical: %.0f%%\n", thresholds.GuestDiskWarn, thresholds.GuestDiskCrit))
	sb.WriteString(fmt.Sprintf("- Storage warning: %.0f%%, critical: %.0f%%\n", thresholds.StorageWarning, thresholds.StorageCritical))
	sb.WriteString("Note: The real-time alerting system monitors these thresholds continuously. Do NOT report findings for threshold breaches — focus on trends, capacity planning, and issues alerts cannot detect.\n\n")

	scopedResources := patrolKnownResources(snap.readState)
	stateHasScopedResources := len(scopedResources) > 0
	globalResources := scopedResources
	if current := p.currentPatrolRuntimeState(); current.readState != nil {
		globalResources = patrolKnownResources(current.readState)
	}
	stateHasGlobalResources := len(globalResources) > 0

	// --- Active Findings to Re-check ---
	activeFindings := p.findings.GetActive(FindingSeverityInfo)
	var seededFindingIDs []string
	if len(activeFindings) > 0 {
		sb.WriteString("# Active Findings to Re-check\n")
		sb.WriteString("Verify whether these findings are still valid. Resolve any that are no longer issues.\n\n")
		for _, f := range activeFindings {
			inScopedState := !stateHasScopedResources || scopedResources[f.ResourceID] || scopedResources[f.ResourceName]
			inGlobalState := !stateHasGlobalResources || globalResources[f.ResourceID] || globalResources[f.ResourceName]

			// Auto-resolve findings only when the resource is gone from the full current state.
			// Scoped patrols should skip out-of-scope findings, not resolve them as deleted.
			if stateHasGlobalResources && !inGlobalState {
				if ok := p.findings.ResolveWithReason(f.ID, "Resource no longer exists in infrastructure"); ok {
					// Notify unified store
					p.mu.RLock()
					resolveUnified := p.unifiedFindingResolver
					p.mu.RUnlock()
					if resolveUnified != nil {
						resolveUnified(f.ID)
					}
					log.Info().
						Str("finding_id", f.ID).
						Str("resource_id", f.ResourceID).
						Str("resource_name", f.ResourceName).
						Msg("AI Patrol: Auto-resolved finding for deleted resource")
				}
				continue
			}
			if !inScopedState {
				continue
			}

			sb.WriteString(fmt.Sprintf("- [%s] %s on %s (ID: %s, Severity: %s, Detected: %s)\n",
				f.ID, f.Title, f.ResourceName, f.ResourceID, f.Severity, f.DetectedAt.Format("2006-01-02 15:04")))
			if f.UserNote != "" {
				sb.WriteString(fmt.Sprintf("  User note: %q\n", f.UserNote))
			}
			seededFindingIDs = append(seededFindingIDs, f.ID)
		}
		sb.WriteString("\n")
	}

	// --- Dismissed/Snoozed Findings ---
	feedbackContext := p.findings.GetDismissedForContextForResources(scopedResources)
	if feedbackContext != "" {
		sb.WriteString("# User Feedback on Previous Findings\n")
		sb.WriteString("Do NOT re-raise findings the user has dismissed or snoozed.\n\n")
		sb.WriteString(feedbackContext)
		sb.WriteString("\n\n")
	}

	// --- User Notes from Knowledge Store ---
	p.mu.RLock()
	knowledgeStore := p.knowledgeStore
	p.mu.RUnlock()
	if knowledgeStore != nil {
		var knowledgeContext string
		scopedKnowledgeIDs := patrolScopedKnowledgeResourceIDs(snap.readState)
		if len(scopedKnowledgeIDs) == 0 && scope != nil {
			scopedKnowledgeIDs = append(scopedKnowledgeIDs, scope.ResourceIDs...)
		}
		if len(scopedKnowledgeIDs) > 0 {
			knowledgeContext = knowledgeStore.FormatForContextForResources(scopedKnowledgeIDs)
		} else {
			knowledgeContext = knowledgeStore.FormatAllForContext()
		}
		if knowledgeContext != "" {
			sb.WriteString("# User Notes\n")
			sb.WriteString(knowledgeContext)
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), seededFindingIDs
}

func patrolKnownResources(rs unifiedresources.ReadState) map[string]bool {
	knownResources := make(map[string]bool)
	if rs == nil {
		return knownResources
	}
	for _, n := range rs.Nodes() {
		knownResources[n.ID()] = true
		knownResources[n.Name()] = true
	}
	for _, vm := range rs.VMs() {
		knownResources[vm.ID()] = true
		knownResources[vm.Name()] = true
	}
	for _, ct := range rs.Containers() {
		knownResources[ct.ID()] = true
		knownResources[ct.Name()] = true
	}
	for _, s := range rs.StoragePools() {
		knownResources[s.ID()] = true
		knownResources[s.Name()] = true
	}
	for _, dh := range rs.DockerHosts() {
		knownResources[dh.ID()] = true
		knownResources[dh.Name()] = true
		if hn := strings.TrimSpace(dh.Hostname()); hn != "" {
			knownResources[hn] = true
		}
	}
	for _, h := range rs.Hosts() {
		knownResources[h.ID()] = true
		knownResources[h.Name()] = true
		if hn := strings.TrimSpace(h.Hostname()); hn != "" {
			knownResources[hn] = true
		}
	}
	for _, pbs := range rs.PBSInstances() {
		knownResources[pbs.ID()] = true
		knownResources[pbs.Name()] = true
	}
	for _, pmg := range rs.PMGInstances() {
		knownResources[pmg.ID()] = true
		knownResources[pmg.Name()] = true
	}
	for _, k := range rs.K8sClusters() {
		knownResources[k.ID()] = true
		knownResources[k.Name()] = true
	}
	return knownResources
}

func patrolScopedKnowledgeResourceIDs(rs unifiedresources.ReadState) []string {
	if rs == nil {
		return nil
	}
	ids := make([]string, 0)
	add := func(id string) {
		if strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	for _, n := range rs.Nodes() {
		add(n.ID())
	}
	for _, vm := range rs.VMs() {
		add(vm.ID())
	}
	for _, ct := range rs.Containers() {
		add(ct.ID())
	}
	for _, s := range rs.StoragePools() {
		add(s.ID())
	}
	for _, dh := range rs.DockerHosts() {
		add(dh.ID())
	}
	for _, h := range rs.Hosts() {
		add(h.ID())
	}
	for _, pbs := range rs.PBSInstances() {
		add(pbs.ID())
	}
	for _, pmg := range rs.PMGInstances() {
		add(pmg.ID())
	}
	for _, k := range rs.K8sClusters() {
		add(k.ID())
	}
	return ids
}

func seedOutlierLabel(name string, cpu, mem, disk float64) (string, bool) {
	type metricOutlier struct {
		label string
		value float64
	}

	best := metricOutlier{}
	for _, m := range []metricOutlier{
		{label: "CPU", value: cpu},
		{label: "Mem", value: mem},
		{label: "Disk", value: disk},
	} {
		if m.value > 80 && m.value > best.value {
			best = m
		}
	}
	if best.label == "" {
		return "", false
	}
	return fmt.Sprintf("%s (%s %.0f%%)", name, best.label, best.value), true
}

func seedFormatStatusBreakdown(counts map[string]int, preferredOrder []string) string {
	if len(counts) == 0 {
		return "none"
	}

	parts := []string{}
	seen := map[string]bool{}
	for _, status := range preferredOrder {
		if count := counts[status]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", status, count))
			seen[status] = true
		}
	}

	remaining := []string{}
	for status, count := range counts {
		if count <= 0 || seen[status] {
			continue
		}
		remaining = append(remaining, status)
	}
	sort.Strings(remaining)
	for _, status := range remaining {
		parts = append(parts, fmt.Sprintf("%s: %d", status, counts[status]))
	}

	return strings.Join(parts, ", ")
}

func seedTruncateOutlierList(items []string, max int) string {
	if len(items) == 0 {
		return ""
	}
	if max <= 0 || len(items) <= max {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(items[:max], ", "), len(items)-max)
}

// seedIsInScope returns true when scopedSet is nil (unscoped) or the resource is in the set.
func seedIsInScope(scopedSet map[string]bool, resourceID string) bool {
	if scopedSet == nil {
		return true
	}
	return scopedSet[resourceID]
}

// seedFormatBytes formats bytes into a human-readable string (e.g. "1.5 GB").
// seedCephBytes extracts used/total bytes from a unified Ceph resource.
func seedCephBytes(r unifiedresources.Resource) (usedBytes, totalBytes int64) {
	if r.Metrics != nil && r.Metrics.Disk != nil && r.Metrics.Disk.Used != nil && r.Metrics.Disk.Total != nil {
		return *r.Metrics.Disk.Used, *r.Metrics.Disk.Total
	}
	if r.Ceph != nil {
		for _, p := range r.Ceph.Pools {
			usedBytes += p.StoredBytes
			totalBytes += p.StoredBytes + p.AvailableBytes
		}
	}
	return
}

func seedFormatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// seedFormatDuration formats a duration into a compact human-readable string (e.g. "45d", "3h").
func seedFormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// seedFormatTimeAgo formats a timestamp as a human-readable "ago" string.
func seedFormatTimeAgo(now, t time.Time) string {
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}

// reconcileStaleFindings auto-resolves active findings that were presented to the LLM
// in seed context but were neither re-reported nor explicitly resolved during the run.
// This handles the case where the LLM doesn't reliably use patrol_resolve_finding.
//
// Safety: only called after successful full patrols (not scoped), and only for findings
// that were in the seed context (the LLM had the opportunity to re-report them).
func (p *PatrolService) reconcileStaleFindings(reportedIDs, resolvedIDs, seededFindingIDs []string, runHadErrors bool) int {
	if runHadErrors {
		return 0
	}
	if len(seededFindingIDs) == 0 {
		return 0
	}

	reported := make(map[string]bool, len(reportedIDs))
	for _, id := range reportedIDs {
		reported[id] = true
	}
	resolved := make(map[string]bool, len(resolvedIDs))
	for _, id := range resolvedIDs {
		resolved[id] = true
	}
	seeded := make(map[string]bool, len(seededFindingIDs))
	for _, id := range seededFindingIDs {
		seeded[id] = true
	}

	count := 0
	for _, id := range seededFindingIDs {
		if reported[id] || resolved[id] {
			continue
		}
		// Only resolve if still active
		if ok := p.findings.ResolveWithReason(id, "No longer detected by patrol"); ok {
			count++

			// Notify unified store
			p.mu.RLock()
			resolveUnified := p.unifiedFindingResolver
			p.mu.RUnlock()
			if resolveUnified != nil {
				resolveUnified(id)
			}

			log.Info().
				Str("finding_id", id).
				Msg("AI Patrol: Auto-resolved stale finding (not re-reported by patrol)")
		}
	}

	return count
}
