package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	PatrolModelReadinessProbeVersion = "patrol-readiness/v1"

	PatrolModelReadinessPass        = "pass"
	PatrolModelReadinessWarning     = "warning"
	PatrolModelReadinessFail        = "fail"
	PatrolModelReadinessNotAssessed = "not_assessed"

	PatrolModeVerified    = "verified"
	PatrolModeWarning     = "warning"
	PatrolModeNotSuitable = "not_suitable"
	PatrolModeNotAssessed = "not_assessed"

	patrolModelReadinessCacheFilename = "ai_patrol_model_readiness.json"
	patrolReadinessObservationTool    = "readiness_record_observation"
	patrolReadinessInventoryTool      = "readiness_list_inventory"
	patrolReadinessChangeTool         = "readiness_apply_change"
	patrolReadinessReceiptTool        = "readiness_confirm_result"
	patrolReadinessMinimumContext     = 8192
	patrolReadinessWatchEnvelope      = 4 * time.Minute
)

// PatrolModelReadinessDimension is one independently reported piece of
// evidence. Readiness is deliberately not reduced to a single blended score:
// a fast model that emits malformed tools is still unsuitable for Patrol.
type PatrolModelReadinessDimension struct {
	Status               string `json:"status"`
	Summary              string `json:"summary"`
	Attempts             int    `json:"attempts,omitempty"`
	Passed               int    `json:"passed,omitempty"`
	DurationMs           int64  `json:"duration_ms,omitempty"`
	FirstResponseMs      int64  `json:"first_response_ms,omitempty"`
	WarmP50Ms            int64  `json:"warm_p50_ms,omitempty"`
	ProjectedWatchMs     int64  `json:"projected_watch_ms,omitempty"`
	ProjectedApprovalMs  int64  `json:"projected_approval_ms,omitempty"`
	ContinuationObserved bool   `json:"continuation_observed,omitempty"`
}

type PatrolModelReadinessDimensions struct {
	Connectivity   PatrolModelReadinessDimension `json:"connectivity"`
	ToolProtocol   PatrolModelReadinessDimension `json:"tool_protocol"`
	ContextQuality PatrolModelReadinessDimension `json:"context_quality"`
	Latency        PatrolModelReadinessDimension `json:"latency"`
}

type PatrolModeSuitability struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type PatrolModelReadinessModes struct {
	Monitor  PatrolModeSuitability `json:"monitor"`
	Approval PatrolModeSuitability `json:"approval"`
	Assisted PatrolModeSuitability `json:"assisted"`
	Full     PatrolModeSuitability `json:"full"`
}

type PatrolModelReadinessMetadata struct {
	Fingerprint       string   `json:"fingerprint,omitempty"`
	Family            string   `json:"family,omitempty"`
	ParameterSize     string   `json:"parameter_size,omitempty"`
	QuantizationLevel string   `json:"quantization_level,omitempty"`
	ContextWindow     int      `json:"context_window,omitempty"`
	Capabilities      []string `json:"capabilities,omitempty"`
}

// PatrolModelReadinessResult is the versioned evidence produced by an
// operator-triggered advisor run. CacheKey is local-only invalidation state and
// is intentionally excluded from every API response.
type PatrolModelReadinessResult struct {
	ProbeVersion    string                         `json:"probe_version"`
	Success         bool                           `json:"success"`
	Status          string                         `json:"status"`
	Provider        string                         `json:"provider,omitempty"`
	Model           string                         `json:"model,omitempty"`
	DurationMs      int64                          `json:"duration_ms"`
	MaxVerifiedMode string                         `json:"max_verified_mode,omitempty"`
	Cause           PatrolFailureCause             `json:"cause,omitempty"`
	Summary         string                         `json:"summary"`
	Recommendation  string                         `json:"recommendation,omitempty"`
	Metadata        *PatrolModelReadinessMetadata  `json:"metadata,omitempty"`
	Dimensions      PatrolModelReadinessDimensions `json:"dimensions"`
	Modes           PatrolModelReadinessModes      `json:"modes"`
	CacheKey        string                         `json:"-"`
	inputTokens     int
	outputTokens    int
	providerCalls   int
}

type patrolModelReadinessCache struct {
	mu         sync.RWMutex
	result     *PatrolModelReadinessResult
	recordedAt time.Time
}

type persistedPatrolModelReadiness struct {
	Result     PatrolModelReadinessResult `json:"result"`
	CacheKey   string                     `json:"cache_key"`
	RecordedAt time.Time                  `json:"recorded_at"`
}

func emptyPatrolModelReadinessResult() PatrolModelReadinessResult {
	notAssessed := PatrolModelReadinessDimension{Status: PatrolModelReadinessNotAssessed, Summary: "Not assessed."}
	return PatrolModelReadinessResult{
		ProbeVersion: PatrolModelReadinessProbeVersion,
		Status:       PatrolModelReadinessFail,
		Cause:        PatrolFailureCauseNone,
		Dimensions: PatrolModelReadinessDimensions{
			Connectivity:   notAssessed,
			ToolProtocol:   notAssessed,
			ContextQuality: notAssessed,
			Latency:        notAssessed,
		},
		Modes: PatrolModelReadinessModes{
			Monitor:  PatrolModeSuitability{Status: PatrolModeNotAssessed, Summary: "Not assessed."},
			Approval: PatrolModeSuitability{Status: PatrolModeNotAssessed, Summary: "Not assessed."},
			Assisted: PatrolModeSuitability{Status: PatrolModeNotAssessed, Summary: "Requires an extended remediation and verification canary."},
			Full:     PatrolModeSuitability{Status: PatrolModeNotAssessed, Summary: "Requires an extended governed Autopilot canary."},
		},
	}
}

func clonePatrolModelReadinessResult(result *PatrolModelReadinessResult) *PatrolModelReadinessResult {
	if result == nil {
		return nil
	}
	clone := *result
	if result.Metadata != nil {
		metadata := *result.Metadata
		metadata.Capabilities = append([]string(nil), result.Metadata.Capabilities...)
		clone.Metadata = &metadata
	}
	return &clone
}

// CachedPatrolModelReadiness returns the most recent result only when it still
// matches the current provider/model/transport configuration.
func (s *Service) CachedPatrolModelReadiness() (*PatrolModelReadinessResult, time.Time) {
	if s == nil {
		return nil, time.Time{}
	}
	s.patrolModelReadinessCache.mu.RLock()
	result := clonePatrolModelReadinessResult(s.patrolModelReadinessCache.result)
	recordedAt := s.patrolModelReadinessCache.recordedAt
	s.patrolModelReadinessCache.mu.RUnlock()
	if result == nil || recordedAt.IsZero() {
		return nil, time.Time{}
	}

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg == nil {
		return nil, time.Time{}
	}
	selectedProvider, selectedModel := config.ParseModelString(cfg.GetPatrolModel())
	if !strings.EqualFold(selectedProvider, result.Provider) || selectedModel != result.Model ||
		result.CacheKey != patrolModelReadinessCacheKey(cfg, result.Provider, result.Model) {
		return nil, time.Time{}
	}
	return result, recordedAt
}

func (s *Service) recordPatrolModelReadiness(result PatrolModelReadinessResult, at time.Time) {
	if s == nil {
		return
	}
	s.patrolModelReadinessCache.mu.Lock()
	if !s.patrolModelReadinessCache.recordedAt.IsZero() && at.Before(s.patrolModelReadinessCache.recordedAt) {
		s.patrolModelReadinessCache.mu.Unlock()
		return
	}
	s.patrolModelReadinessCache.result = clonePatrolModelReadinessResult(&result)
	s.patrolModelReadinessCache.recordedAt = at
	s.patrolModelReadinessCache.mu.Unlock()
	if err := s.persistPatrolModelReadiness(result, at); err != nil {
		log.Warn().Err(err).Msg("failed to persist Patrol model readiness evidence")
	}
}

// InvalidatePatrolModelReadiness removes evidence after a settings change that
// can alter provider transport or model behaviour.
func (s *Service) InvalidatePatrolModelReadiness() {
	if s == nil {
		return
	}
	s.patrolModelReadinessCache.mu.Lock()
	s.patrolModelReadinessCache.result = nil
	s.patrolModelReadinessCache.recordedAt = time.Time{}
	s.patrolModelReadinessCache.mu.Unlock()
	if path := s.patrolModelReadinessPath(); path != "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Msg("failed to remove stale Patrol model readiness evidence")
		}
	}
}

func (s *Service) patrolModelReadinessPath() string {
	if s == nil || s.persistence == nil {
		return ""
	}
	return filepath.Join(s.persistence.DataDir(), patrolModelReadinessCacheFilename)
}

func (s *Service) persistPatrolModelReadiness(result PatrolModelReadinessResult, at time.Time) error {
	path := s.patrolModelReadinessPath()
	if path == "" {
		return nil
	}
	payload, err := json.MarshalIndent(persistedPatrolModelReadiness{
		Result:     result,
		CacheKey:   result.CacheKey,
		RecordedAt: at.UTC(),
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Service) loadPatrolModelReadiness() {
	path := s.patrolModelReadinessPath()
	if path == "" {
		return
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		log.Warn().Err(err).Msg("failed to read Patrol model readiness evidence")
		return
	}
	var persisted persistedPatrolModelReadiness
	if err := json.Unmarshal(payload, &persisted); err != nil {
		log.Warn().Err(err).Msg("ignored invalid Patrol model readiness evidence")
		return
	}
	if persisted.Result.ProbeVersion != PatrolModelReadinessProbeVersion || persisted.RecordedAt.IsZero() {
		return
	}
	persisted.Result.CacheKey = persisted.CacheKey
	s.patrolModelReadinessCache.result = clonePatrolModelReadinessResult(&persisted.Result)
	s.patrolModelReadinessCache.recordedAt = persisted.RecordedAt
}

func patrolModelReadinessCacheKey(cfg *config.AIConfig, providerName, model string) string {
	if cfg == nil {
		return ""
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	endpoint := ""
	switch providerName {
	case config.AIProviderOllama:
		endpoint = strings.TrimSpace(cfg.OllamaBaseURL)
	case config.AIProviderOpenAI:
		endpoint = strings.TrimSpace(cfg.OpenAIBaseURL)
	case config.AIProviderZai:
		endpoint = strings.TrimSpace(cfg.ZaiBaseURL)
	}
	credentialMaterial := cfg.GetAPIKeyForProvider(providerName)
	if providerName == config.AIProviderOllama {
		credentialMaterial = cfg.OllamaUsername + "\n" + cfg.OllamaPassword
	}
	credentialFingerprint := sha256.Sum256([]byte(credentialMaterial))
	material := fmt.Sprintf("%s\n%s\n%s\n%x\n%d\n%d\n%d",
		providerName,
		strings.TrimSpace(model),
		endpoint,
		credentialFingerprint,
		cfg.GetRequestTimeout().Milliseconds(),
		cfg.GetPatrolInvestigationBudget(),
		cfg.GetPatrolInvestigationTimeout().Milliseconds(),
	)
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

// RunPatrolModelReadiness evaluates the exact configured or overridden model
// through the streaming transport used by Patrol. It uses only synthetic
// fixtures and in-memory tool results; no infrastructure tools are executed.
func (s *Service) RunPatrolModelReadiness(ctx context.Context, providerName, model string) PatrolModelReadinessResult {
	started := time.Now()
	result := emptyPatrolModelReadinessResult()

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	finish := func() PatrolModelReadinessResult {
		result.DurationMs = time.Since(started).Milliseconds()
		if cfg != nil && result.Provider != "" && result.Model != "" {
			result.CacheKey = patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
		}
		// Cancellation is an operator action or request-budget boundary, not new
		// evidence about the model. Preserve the last completed evaluation.
		if ctx.Err() == nil {
			s.recordPatrolModelReadiness(result, time.Now())
		}
		s.recordPatrolModelReadinessUsage(result)
		return result
	}

	if cfg == nil {
		result.Cause = PatrolFailureCauseSettingsPersistence
		result.Summary = "Pulse Intelligence settings could not be loaded."
		result.Recommendation = "Confirm settings persistence is healthy, then run the advisor again."
		return finish()
	}
	if !cfg.Enabled {
		result.Cause = PatrolFailureCauseAssistantDisabled
		result.Summary = "Pulse Intelligence is turned off."
		result.Recommendation = "Turn on Pulse Intelligence, then run the advisor again."
		return finish()
	}

	modelString := strings.TrimSpace(model)
	if modelString == "" {
		modelString = strings.TrimSpace(cfg.GetPatrolModel())
	}
	if modelString == "" {
		result.Cause = PatrolFailureCauseModelNotSelected
		result.Summary = "No Patrol model is selected."
		result.Recommendation = "Select a Patrol model, then run the advisor again."
		return finish()
	}
	if overrideProvider := strings.TrimSpace(providerName); overrideProvider != "" {
		_, bare := config.ParseModelString(modelString)
		if bare == "" {
			bare = modelString
		}
		modelString = overrideProvider + ":" + bare
	}
	result.Provider, result.Model = config.ParseModelString(modelString)
	if err := ctx.Err(); err != nil {
		result.Cause = PatrolFailureCauseProviderConnection
		result.Summary = "The Patrol model readiness evaluation was cancelled."
		result.Recommendation = "Run the advisor again when you are ready."
		return finish()
	}

	if IsDemoMode() {
		result.Provider = DemoPatrolProvider
		result.Model = DemoPatrolModel
		result.Success = true
		result.Status = PatrolModelReadinessPass
		result.MaxVerifiedMode = config.PatrolAutonomyApproval
		result.Summary = "Demo mode simulated a successful Patrol readiness evaluation."
		result.Dimensions.Connectivity = PatrolModelReadinessDimension{Status: PatrolModelReadinessPass, Summary: "Demo provider available."}
		result.Dimensions.ToolProtocol = PatrolModelReadinessDimension{Status: PatrolModelReadinessPass, Summary: "Demo tool protocol simulated.", Attempts: 3, Passed: 3, ContinuationObserved: true}
		result.Dimensions.ContextQuality = PatrolModelReadinessDimension{Status: PatrolModelReadinessPass, Summary: "Demo context fixtures simulated.", Attempts: 2, Passed: 2}
		result.Dimensions.Latency = PatrolModelReadinessDimension{Status: PatrolModelReadinessPass, Summary: "Demo latency simulated."}
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeVerified, Summary: "Verified for Watch only in demo mode."}
		result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeVerified, Summary: "Verified for Ask first in demo mode."}
		return finish()
	}
	if err := s.enforceBudget("patrol"); err != nil {
		result.Cause = PatrolFailureCauseProviderBilling
		result.Summary = "The configured Pulse Intelligence cost budget has been reached."
		result.Recommendation = "Raise the cost budget or wait for the rolling usage window to clear before running the advisor."
		return finish()
	}

	releaseSlot, err := s.acquireExecutionSlot(ctx, "patrol")
	if err != nil {
		failure := patrolRuntimeFailureFromError(err)
		result.Cause = failure.Cause
		result.Summary = "Patrol is already using the selected model."
		result.Recommendation = "Wait for the active Patrol work to finish, then run the advisor again."
		return finish()
	}
	defer releaseSlot()

	provider, err := providers.NewForModel(cfg, modelString)
	if err != nil {
		failure := patrolRuntimeFailureFromError(err)
		result.Cause = failure.Cause
		result.Summary = failure.Summary
		result.Recommendation = failure.Recommendation
		return finish()
	}
	result = runPatrolModelReadinessWithProvider(ctx, cfg, result.Provider, result.Model, modelString, provider)
	return finish()
}

type patrolReadinessScenario struct {
	name          string
	prompt        string
	nonce         string
	resourceID    string
	category      string
	severity      string
	evidenceToken string
	context       bool
}

type patrolReadinessProbeOutcome struct {
	toolCalls       []providers.ToolCall
	duration        time.Duration
	firstResponse   time.Duration
	inputTokens     int
	outputTokens    int
	completionEvent bool
}

func runPatrolModelReadinessWithProvider(ctx context.Context, cfg *config.AIConfig, providerName, model, modelString string, provider providers.Provider) PatrolModelReadinessResult {
	result := emptyPatrolModelReadinessResult()
	result.Provider = providerName
	result.Model = model
	started := time.Now()

	connectionStarted := time.Now()
	if err := provider.TestConnection(ctx); err != nil {
		failure := patrolRuntimeFailureFromError(err)
		result.Cause = failure.Cause
		result.Status = PatrolModelReadinessFail
		result.Summary = failure.Summary
		result.Recommendation = failure.Recommendation
		result.Dimensions.Connectivity = PatrolModelReadinessDimension{
			Status:     PatrolModelReadinessFail,
			Summary:    failure.Summary,
			DurationMs: time.Since(connectionStarted).Milliseconds(),
		}
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "The selected model is not reachable."}
		result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "The selected model is not reachable."}
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	connectionSummary := "Provider and selected model are reachable."
	if diagnosticsProvider, ok := provider.(providers.ModelDiagnosticsProvider); ok {
		if diagnostics, err := diagnosticsProvider.InspectModel(ctx, model); err == nil && diagnostics != nil {
			result.Metadata = &PatrolModelReadinessMetadata{
				Fingerprint:       diagnostics.Fingerprint,
				Family:            diagnostics.Family,
				ParameterSize:     diagnostics.ParameterSize,
				QuantizationLevel: diagnostics.QuantizationLevel,
				ContextWindow:     diagnostics.ContextWindow,
				Capabilities:      append([]string(nil), diagnostics.Capabilities...),
			}
		} else if err != nil {
			connectionSummary = "Provider and selected model are reachable; runtime model metadata was unavailable."
		}
	}
	result.Dimensions.Connectivity = PatrolModelReadinessDimension{
		Status:     PatrolModelReadinessPass,
		Summary:    connectionSummary,
		DurationMs: time.Since(connectionStarted).Milliseconds(),
	}

	streamingProvider, ok := provider.(providers.StreamingProvider)
	if !ok {
		result.Cause = PatrolFailureCauseModelUnsupportedTools
		result.Status = PatrolModelReadinessFail
		result.Summary = "The selected provider does not expose Patrol's streaming transport."
		result.Recommendation = "Choose a provider and model that support streaming tool calls."
		result.Dimensions.ToolProtocol = PatrolModelReadinessDimension{Status: PatrolModelReadinessFail, Summary: result.Summary}
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: result.Summary}
		result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: result.Summary}
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	scenarios := patrolReadinessScenarios()
	tools := patrolReadinessTools()
	durations := make([]time.Duration, 0, len(scenarios)+1)
	firstResponses := make([]time.Duration, 0, len(scenarios)+1)
	toolPassed := 0
	contextPassed := 0
	var continuationSource patrolReadinessProbeOutcome
	var continuationScenario patrolReadinessScenario
	var probeErr error

	for _, scenario := range scenarios {
		req := providers.ChatRequest{
			Model: modelString,
			System: "You are running a synthetic Pulse Patrol readiness evaluation. " +
				"Use exactly one appropriate readiness tool. Never call readiness_apply_change. " +
				"Do not answer in prose. Copy identifiers and the nonce exactly from the fixture.",
			Messages:    []providers.Message{{Role: "user", Content: scenario.prompt}},
			Tools:       tools,
			MaxTokens:   256,
			Temperature: 0,
		}
		if providerName == config.AIProviderGemini {
			req.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceRequired}
		}
		result.providerCalls++
		outcome, err := runPatrolReadinessStreamProbe(ctx, streamingProvider, req)
		result.inputTokens += outcome.inputTokens
		result.outputTokens += outcome.outputTokens
		if err != nil {
			probeErr = err
			break
		}
		durations = append(durations, outcome.duration)
		firstResponses = append(firstResponses, outcome.firstResponse)
		if validatePatrolReadinessProtocol(outcome.toolCalls, scenario.nonce) == nil {
			toolPassed++
		}
		if scenario.context && validatePatrolReadinessCall(outcome.toolCalls, scenario) == nil {
			contextPassed++
		}
		if scenario.name == "storage-pressure" {
			continuationSource = outcome
			continuationScenario = scenario
		}
	}

	continuationObserved := false
	if probeErr == nil && validatePatrolReadinessProtocol(continuationSource.toolCalls, continuationScenario.nonce) == nil {
		call := continuationSource.toolCalls[0]
		findingID := "synthetic-finding-" + continuationScenario.nonce
		toolResult, _ := json.Marshal(map[string]interface{}{
			"accepted":   true,
			"finding_id": findingID,
			"nonce":      continuationScenario.nonce,
		})
		followup := providers.ChatRequest{
			Model: modelString,
			System: "Continue the synthetic readiness evaluation. Call readiness_confirm_result exactly once " +
				"with the accepted tool result. Do not answer in prose.",
			Messages: []providers.Message{
				{Role: "user", Content: continuationScenario.prompt},
				{Role: "assistant", ToolCalls: []providers.ToolCall{call}},
				{Role: "user", ToolResult: &providers.ToolResult{ToolUseID: call.ID, Content: string(toolResult)}},
			},
			Tools:       []providers.Tool{patrolReadinessReceiptDefinition()},
			MaxTokens:   128,
			Temperature: 0,
		}
		if providerName == config.AIProviderGemini {
			followup.ToolChoice = &providers.ToolChoice{Type: providers.ToolChoiceRequired}
		}
		result.providerCalls++
		outcome, err := runPatrolReadinessStreamProbe(ctx, streamingProvider, followup)
		result.inputTokens += outcome.inputTokens
		result.outputTokens += outcome.outputTokens
		if err != nil {
			probeErr = err
		} else {
			durations = append(durations, outcome.duration)
			firstResponses = append(firstResponses, outcome.firstResponse)
			continuationObserved = validatePatrolReadinessReceipt(outcome.toolCalls, continuationScenario.nonce, findingID) == nil
		}
	}

	toolDuration := sumDurations(durations)
	toolStatus := PatrolModelReadinessFail
	toolSummary := fmt.Sprintf("Exact tool protocol passed %d/%d scenarios.", toolPassed, len(scenarios))
	if toolPassed == len(scenarios) {
		toolStatus = PatrolModelReadinessPass
		if continuationObserved {
			toolSummary += " Multi-turn tool-result continuation also passed."
		} else {
			toolSummary += " Initial tool use passed, but multi-turn continuation was not verified."
		}
	}
	if probeErr != nil {
		failure := patrolRuntimeFailureFromError(probeErr)
		result.Cause = failure.Cause
		toolSummary = failure.Summary
		result.Recommendation = failure.Recommendation
	}
	result.Dimensions.ToolProtocol = PatrolModelReadinessDimension{
		Status:               toolStatus,
		Summary:              toolSummary,
		Attempts:             len(scenarios),
		Passed:               toolPassed,
		DurationMs:           toolDuration.Milliseconds(),
		ContinuationObserved: continuationObserved,
	}

	contextStatus := PatrolModelReadinessFail
	contextSummary := fmt.Sprintf("Patrol-shaped context fixtures passed %d/2 scenarios.", contextPassed)
	if contextPassed == 2 {
		contextStatus = PatrolModelReadinessPass
		contextSummary = "Both Patrol-shaped fixtures selected the actionable resource without flagging healthy decoys."
	}
	if result.Metadata != nil && result.Metadata.ContextWindow > 0 && result.Metadata.ContextWindow < patrolReadinessMinimumContext {
		contextStatus = PatrolModelReadinessFail
		contextSummary = fmt.Sprintf("The runtime reports a %d-token context window; Patrol readiness requires at least %d tokens.", result.Metadata.ContextWindow, patrolReadinessMinimumContext)
	}
	result.Dimensions.ContextQuality = PatrolModelReadinessDimension{
		Status:   contextStatus,
		Summary:  contextSummary,
		Attempts: 2,
		Passed:   contextPassed,
	}

	warmP50 := medianDuration(durations)
	if len(durations) > 1 {
		warmP50 = medianDuration(durations[1:])
	}
	firstResponse := medianDuration(firstResponses)
	projectedWatch := warmP50 * 8
	projectedApproval := warmP50 * time.Duration(cfg.GetPatrolInvestigationBudget())
	latencyStatus := PatrolModelReadinessPass
	latencySummary := fmt.Sprintf("Warm median %s; projected 8-turn Watch-only loop %s.", formatReadinessDuration(warmP50), formatReadinessDuration(projectedWatch))
	if len(durations) == 0 || probeErr != nil {
		latencyStatus = PatrolModelReadinessFail
		latencySummary = "Latency could not be measured because the streaming probe did not complete."
	} else if projectedWatch > patrolReadinessWatchEnvelope {
		latencyStatus = PatrolModelReadinessWarning
		latencySummary = fmt.Sprintf("Projected 8-turn Watch-only loop is %s, above the four-minute readiness envelope.", formatReadinessDuration(projectedWatch))
	}
	result.Dimensions.Latency = PatrolModelReadinessDimension{
		Status:              latencyStatus,
		Summary:             latencySummary,
		DurationMs:          toolDuration.Milliseconds(),
		FirstResponseMs:     firstResponse.Milliseconds(),
		WarmP50Ms:           warmP50.Milliseconds(),
		ProjectedWatchMs:    projectedWatch.Milliseconds(),
		ProjectedApprovalMs: projectedApproval.Milliseconds(),
	}

	watchVerified := toolStatus == PatrolModelReadinessPass && contextStatus == PatrolModelReadinessPass && latencyStatus == PatrolModelReadinessPass
	approvalVerified := watchVerified && continuationObserved && projectedApproval <= cfg.GetPatrolInvestigationTimeout()
	if watchVerified {
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeVerified, Summary: "Verified for Watch only on this provider, model, and runtime."}
	} else if latencyStatus == PatrolModelReadinessWarning {
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeWarning, Summary: "Core protocol passed, but projected Watch-only latency exceeds the readiness envelope."}
	} else {
		result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "The model did not pass the minimum Watch-only protocol and context checks."}
	}
	if approvalVerified {
		result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeVerified, Summary: "Verified for Ask first, including multi-turn tool-result continuation."}
	} else if watchVerified {
		if !continuationObserved {
			result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "Ask first requires a correct multi-turn tool-result continuation."}
		} else {
			result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeWarning, Summary: "Projected investigation latency exceeds the configured investigation timeout."}
		}
	} else {
		result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "Ask first requires all Watch-only checks to pass first."}
	}

	result.Success = watchVerified
	result.Status = PatrolModelReadinessFail
	result.Cause = PatrolFailureCauseModelToolSupportUnverified
	result.Summary = "The selected model did not pass Patrol's tool protocol evaluation."
	result.Recommendation = "Choose a model with reliable streaming tool use, or lower the model's workload and retry."
	if contextStatus == PatrolModelReadinessFail && toolStatus == PatrolModelReadinessPass {
		result.Cause = PatrolFailureCauseContextQualityFailed
		result.Summary = "The selected model did not pass Patrol's context-quality evaluation."
		result.Recommendation = "Choose a model with stronger context handling or a larger runtime context window."
	}
	if latencyStatus == PatrolModelReadinessWarning && toolStatus == PatrolModelReadinessPass && contextStatus == PatrolModelReadinessPass {
		result.Status = PatrolModelReadinessWarning
		result.Cause = PatrolFailureCauseLatencyUnsuitable
		result.Summary = "The selected model passed protocol and context checks but is too slow for the Watch-only readiness envelope."
		result.Recommendation = "Increase available local compute, reduce model size, or select a faster model."
	}
	if watchVerified {
		result.Status = PatrolModelReadinessPass
		result.Cause = PatrolFailureCauseNone
		result.MaxVerifiedMode = config.PatrolAutonomyMonitor
		result.Summary = "Verified for Watch only on this install."
		result.Recommendation = "Keep Safe auto-fix and Autopilot disabled until an extended governed canary has passed."
		if approvalVerified {
			result.MaxVerifiedMode = config.PatrolAutonomyApproval
			result.Summary = "Verified for Watch only and Ask first on this install."
		}
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result
}

func patrolReadinessScenarios() []patrolReadinessScenario {
	protocolNonce := newPatrolReadinessNonce()
	backupNonce := newPatrolReadinessNonce()
	storageNonce := newPatrolReadinessNonce()
	return []patrolReadinessScenario{
		{
			name:          "typed-tool",
			nonce:         protocolNonce,
			resourceID:    "node-protocol",
			category:      "reliability",
			severity:      "warning",
			evidenceToken: "protocol-" + protocolNonce,
			prompt:        fmt.Sprintf("Record exactly this synthetic observation: resource_id=node-protocol, category=reliability, severity=warning, evidence_token=protocol-%s, nonce=%s.", protocolNonce, protocolNonce),
		},
		newPatrolContextScenario("backup-failure", backupNonce, "pbs-lab-a", "backup", "critical", "backup-failed-201"),
		newPatrolContextScenario("storage-pressure", storageNonce, "storage-lab-b", "capacity", "warning", "storage-91-percent"),
	}
}

func newPatrolContextScenario(name, nonce, resourceID, category, severity, evidenceToken string) patrolReadinessScenario {
	var fixture strings.Builder
	fixture.Grow(32 * 1024)
	fixture.WriteString("Synthetic Patrol snapshot. Record the single actionable observation. Ignore healthy decoys. Evaluation nonce: ")
	fixture.WriteString(nonce)
	fixture.WriteString("\n")
	actionPosition := 47
	if name == "storage-pressure" {
		actionPosition = 233
	}
	for i := 0; i < 280; i++ {
		if i == actionPosition {
			if name == "backup-failure" {
				fmt.Fprintf(&fixture, "ACTIONABLE resource_id=%s type=pbs category=%s severity=%s last_job=failed evidence_token=%s\n", resourceID, category, severity, evidenceToken)
			} else {
				fmt.Fprintf(&fixture, "ACTIONABLE resource_id=%s type=storage category=%s severity=%s usage=91%% evidence_token=%s\n", resourceID, category, severity, evidenceToken)
			}
		}
		fmt.Fprintf(&fixture, "healthy-decoy-%03d type=vm status=running cpu=%d%% memory=%d%% backup=ok evidence=healthy-%03d\n", i, 10+i%30, 20+i%40, i)
	}
	return patrolReadinessScenario{
		name:          name,
		prompt:        fixture.String(),
		nonce:         nonce,
		resourceID:    resourceID,
		category:      category,
		severity:      severity,
		evidenceToken: evidenceToken,
		context:       true,
	}
}

func patrolReadinessTools() []providers.Tool {
	return []providers.Tool{
		{
			Name:        patrolReadinessObservationTool,
			Description: "Record the one actionable observation from a synthetic Patrol fixture.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource_id":    map[string]interface{}{"type": "string"},
					"category":       map[string]interface{}{"type": "string", "enum": []string{"reliability", "backup", "capacity"}},
					"severity":       map[string]interface{}{"type": "string", "enum": []string{"warning", "critical"}},
					"evidence_token": map[string]interface{}{"type": "string"},
					"nonce":          map[string]interface{}{"type": "string"},
				},
				"required":             []string{"resource_id", "category", "severity", "evidence_token", "nonce"},
				"additionalProperties": false,
			},
		},
		{
			Name:        patrolReadinessInventoryTool,
			Description: "List inventory only when the fixture contains no resource records. Readiness fixtures always contain records.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "additionalProperties": false},
		},
		{
			Name:        patrolReadinessChangeTool,
			Description: "Synthetic change tool. Never call this during a readiness evaluation.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"nonce": map[string]interface{}{"type": "string"}},
				"required":   []string{"nonce"},
			},
		},
	}
}

func patrolReadinessReceiptDefinition() providers.Tool {
	return providers.Tool{
		Name:        patrolReadinessReceiptTool,
		Description: "Confirm the synthetic observation result returned by the readiness evaluator.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"accepted":   map[string]interface{}{"type": "boolean"},
				"finding_id": map[string]interface{}{"type": "string"},
				"nonce":      map[string]interface{}{"type": "string"},
			},
			"required":             []string{"accepted", "finding_id", "nonce"},
			"additionalProperties": false,
		},
	}
}

func runPatrolReadinessStreamProbe(ctx context.Context, provider providers.StreamingProvider, req providers.ChatRequest) (patrolReadinessProbeOutcome, error) {
	started := time.Now()
	var outcome patrolReadinessProbeOutcome
	var starts []providers.ToolCall
	err := provider.ChatStream(ctx, req, func(event providers.StreamEvent) {
		if outcome.firstResponse == 0 {
			outcome.firstResponse = time.Since(started)
		}
		switch event.Type {
		case "tool_start":
			if data, ok := event.Data.(providers.ToolStartEvent); ok {
				starts = append(starts, providers.ToolCall{ID: data.ID, Name: data.Name, Input: data.Input})
			} else if data, ok := event.Data.(*providers.ToolStartEvent); ok && data != nil {
				starts = append(starts, providers.ToolCall{ID: data.ID, Name: data.Name, Input: data.Input})
			}
		case "done":
			if data, ok := event.Data.(providers.DoneEvent); ok {
				outcome.toolCalls = append([]providers.ToolCall(nil), data.ToolCalls...)
				outcome.inputTokens = data.InputTokens
				outcome.outputTokens = data.OutputTokens
				outcome.completionEvent = true
			} else if data, ok := event.Data.(*providers.DoneEvent); ok && data != nil {
				outcome.toolCalls = append([]providers.ToolCall(nil), data.ToolCalls...)
				outcome.inputTokens = data.InputTokens
				outcome.outputTokens = data.OutputTokens
				outcome.completionEvent = true
			}
		}
	})
	outcome.duration = time.Since(started)
	if err != nil {
		return outcome, err
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	if !outcome.completionEvent {
		return outcome, fmt.Errorf("stream ended without a completion event")
	}
	if len(outcome.toolCalls) == 0 {
		outcome.toolCalls = starts
	}
	return outcome, nil
}

func validatePatrolReadinessCall(calls []providers.ToolCall, scenario patrolReadinessScenario) error {
	if err := validatePatrolReadinessProtocol(calls, scenario.nonce); err != nil {
		return err
	}
	call := calls[0]
	expected := map[string]string{
		"resource_id":    scenario.resourceID,
		"category":       scenario.category,
		"severity":       scenario.severity,
		"evidence_token": scenario.evidenceToken,
		"nonce":          scenario.nonce,
	}
	for key, value := range expected {
		actual, _ := call.Input[key].(string)
		if actual != value {
			return fmt.Errorf("argument %s did not match", key)
		}
	}
	return nil
}

func validatePatrolReadinessProtocol(calls []providers.ToolCall, nonce string) error {
	if len(calls) != 1 {
		return fmt.Errorf("expected exactly one tool call, got %d", len(calls))
	}
	call := calls[0]
	if call.Name != patrolReadinessObservationTool {
		return fmt.Errorf("expected tool %s, got %s", patrolReadinessObservationTool, call.Name)
	}
	if strings.TrimSpace(call.ID) == "" {
		return fmt.Errorf("tool call did not include an id for continuation")
	}
	required := []string{"resource_id", "category", "severity", "evidence_token", "nonce"}
	if len(call.Input) != len(required) {
		return fmt.Errorf("expected %d typed arguments, got %d", len(required), len(call.Input))
	}
	for _, key := range required {
		if _, ok := call.Input[key].(string); !ok {
			return fmt.Errorf("argument %s was not a string", key)
		}
	}
	if call.Input["nonce"] != nonce {
		return fmt.Errorf("nonce did not match")
	}
	category := call.Input["category"].(string)
	if category != "reliability" && category != "backup" && category != "capacity" {
		return fmt.Errorf("category was outside the tool schema")
	}
	severity := call.Input["severity"].(string)
	if severity != "warning" && severity != "critical" {
		return fmt.Errorf("severity was outside the tool schema")
	}
	return nil
}

func validatePatrolReadinessReceipt(calls []providers.ToolCall, nonce, findingID string) error {
	if len(calls) != 1 || calls[0].Name != patrolReadinessReceiptTool {
		return fmt.Errorf("expected exactly one %s call", patrolReadinessReceiptTool)
	}
	input := calls[0].Input
	if len(input) != 3 {
		return fmt.Errorf("unexpected receipt argument count")
	}
	accepted, ok := input["accepted"].(bool)
	if !ok || !accepted {
		return fmt.Errorf("receipt was not accepted")
	}
	receiptNonce, nonceOK := input["nonce"].(string)
	receiptFindingID, findingIDOK := input["finding_id"].(string)
	if !nonceOK || !findingIDOK || receiptNonce != nonce || receiptFindingID != findingID {
		return fmt.Errorf("receipt identifiers did not match")
	}
	return nil
}

func newPatrolReadinessNonce() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func sumDurations(values []time.Duration) time.Duration {
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func formatReadinessDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value.Milliseconds())
	}
	return value.Round(100 * time.Millisecond).String()
}

func modelStringForReadinessResult(result PatrolModelReadinessResult) string {
	if result.Provider == "" {
		return result.Model
	}
	return config.FormatModelString(result.Provider, result.Model)
}

func (s *Service) recordPatrolModelReadinessUsage(result PatrolModelReadinessResult) {
	if s == nil || result.providerCalls == 0 {
		return
	}
	s.mu.RLock()
	costStore := s.costStore
	s.mu.RUnlock()
	if costStore == nil {
		return
	}
	costStore.Record(cost.UsageEvent{
		Timestamp:    time.Now(),
		Provider:     result.Provider,
		RequestModel: modelStringForReadinessResult(result),
		UseCase:      "patrol_readiness",
		InputTokens:  result.inputTokens,
		OutputTokens: result.outputTokens,
	})
}
