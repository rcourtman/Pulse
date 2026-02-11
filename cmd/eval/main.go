// Command eval runs Pulse Assistant evaluation scenarios against a live Pulse instance.
//
// Usage:
//
//	go run ./cmd/eval                    # Run quick smoke test
//	go run ./cmd/eval -scenario all      # Run all scenarios
//	go run ./cmd/eval -scenario readonly # Run read-only infrastructure scenario
//	go run ./cmd/eval -list              # List available scenarios
//
// Options:
//
//	-scenario string  Scenario to run: smoke, readonly, enforce, routing, routing-recovery, logs, readonly-recovery, search-id, disambiguate, context-target, discovery, writeverify, strict, strict-block, strict-recovery, readonly-guardrails, noninteractive, approval, approval-approve, approval-deny, approval-combo, patrol, patrol-basic, patrol-investigation, patrol-finding-quality, patrol-signal-coverage, matrix, all (default "smoke")
//	-url string       Pulse API base URL (default "http://127.0.0.1:7655")
//	-user string      Username for auth (default "admin")
//	-pass string      Password for auth (default "admin")
//	-model string     Model override for chat requests
//	-models string    Comma-separated list of models to run (overrides -model)
//	-auto-models      Auto-select latest models per provider
//	-list             List available scenarios and exit
//	-quiet            Only show summary, not step-by-step output
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/eval"
)

func main() {
	scenario := flag.String("scenario", "smoke", "Scenario to run: smoke, readonly, enforce, routing, routing-recovery, logs, readonly-recovery, search-id, disambiguate, context-target, discovery, writeverify, guest-control, guest-idempotent, guest-discovery, guest-natural, guest-multi, readonly-filtering, read-loop-recovery, ambiguous-intent, strict, strict-block, strict-recovery, readonly-guardrails, noninteractive, approval, approval-approve, approval-deny, approval-combo, patrol, patrol-basic, patrol-investigation, patrol-finding-quality, patrol-signal-coverage, matrix, all")
	url := flag.String("url", "http://127.0.0.1:7655", "Pulse API base URL")
	user := flag.String("user", "admin", "Username for auth")
	pass := flag.String("pass", "admin", "Password for auth")
	model := flag.String("model", "", "Model override for chat requests")
	models := flag.String("models", "", "Comma-separated list of models to run (overrides -model)")
	autoModels := flag.Bool("auto-models", false, "Auto-select latest models per provider")
	list := flag.Bool("list", false, "List available scenarios and exit")
	quiet := flag.Bool("quiet", false, "Only show summary, not step-by-step output")

	flag.Parse()

	if *list {
		listScenarios()
		return
	}

	baseConfig := eval.DefaultConfig()
	baseConfig.BaseURL = *url
	baseConfig.Username = *user
	baseConfig.Password = *pass
	baseConfig.Verbose = !*quiet

	if value, ok := envBool("EVAL_PREFLIGHT"); ok {
		baseConfig.Preflight = value
	}
	if value, ok := envInt("EVAL_PREFLIGHT_TIMEOUT"); ok && value > 0 {
		baseConfig.PreflightTimeout = time.Duration(value) * time.Second
	} else if baseConfig.PreflightTimeout == 0 {
		baseConfig.PreflightTimeout = 15 * time.Second
	}

	modelList := parseModelList(*models)
	if len(modelList) == 0 && *autoModels {
		autoList, details, stats, err := fetchAutoModels(baseConfig.BaseURL, baseConfig.Username, baseConfig.Password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to auto-select models: %v\n", err)
			os.Exit(1)
		}
		modelList = autoList
		fmt.Printf(">>> Auto-selected models: %s\n", strings.Join(modelList, ", "))
		if len(stats) > 0 {
			fmt.Println(">>> Auto-selection provider summary:")
			providers := sortedProviders(stats)
			for _, provider := range providers {
				stat := stats[provider]
				fmt.Printf("  - %s: %d models (%d notable)\n", provider, stat.Total, stat.Notable)
			}
		}
		if len(details) > 0 {
			fmt.Println(">>> Auto-selection details:")
			for _, detail := range details {
				meta := detail.Reason
				if meta == "" {
					meta = "selected"
				}
				fmt.Printf("  - %s: %s (%s)\n", detail.Provider, detail.ID, meta)
			}
		}
	}
	if len(modelList) == 0 {
		modelList = []string{strings.TrimSpace(*model)}
	}
	if len(modelList) == 0 {
		modelList = []string{""}
	}

	patrolScenarios := getPatrolScenarios(*scenario)
	scenarios := getScenarios(*scenario)
	if len(patrolScenarios) == 0 && len(scenarios) == 0 {
		fmt.Fprintf(os.Stderr, "Unknown scenario: %s\n", *scenario)
		fmt.Fprintf(os.Stderr, "Use -list to see available scenarios\n")
		os.Exit(1)
	}

	allPassed := true
	for _, modelID := range modelList {
		config := baseConfig
		config.Model = strings.TrimSpace(modelID)

		if config.Model != "" {
			fmt.Printf("\n>>> Using model: %s\n", config.Model)
		}

		if config.Preflight {
			fmt.Printf(">>> Preflight enabled (timeout %s)\n", config.PreflightTimeout)
		}

		runner := eval.NewRunner(config)

		if len(patrolScenarios) > 0 {
			modelPassed := true
			for _, ps := range patrolScenarios {
				fmt.Printf("\n>>> Running patrol scenario: %s\n", ps.Name)
				fmt.Printf(">>> %s\n", ps.Description)

				result := runner.RunPatrolScenario(ps)
				runner.PrintPatrolSummary(result)

				if !result.Success {
					modelPassed = false
				}
			}

			if !modelPassed {
				allPassed = false
			}
			continue
		}

		modelPassed := true
		for _, s := range scenarios {
			fmt.Printf("\n>>> Running scenario: %s\n", s.Name)
			fmt.Printf(">>> %s\n", s.Description)

			result := runner.RunScenario(s)
			runner.PrintSummary(result)

			if !result.Passed {
				modelPassed = false
			}
		}

		if !modelPassed {
			allPassed = false
		}
	}

	if allPassed {
		fmt.Printf("\n>>> ALL SCENARIOS PASSED\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n>>> SOME SCENARIOS FAILED\n")
		os.Exit(1)
	}
}

func listScenarios() {
	fmt.Println("Available scenarios:")
	fmt.Println()
	fmt.Println("  Basic:")
	fmt.Println("    smoke        - Quick smoke test (1 step)")
	fmt.Println("    readonly     - Read-only infrastructure test (3 steps)")
	fmt.Println("    enforce      - Explicit tool enforcement (1 step)")
	fmt.Println("    routing      - Routing validation test (2 steps)")
	fmt.Println("    routing-recovery - Routing mismatch recovery (2 steps)")
	fmt.Println("    logs         - Log tailing/bounded command test (2 steps)")
	fmt.Println("    readonly-recovery - Read-only violation recovery (1 step)")
	fmt.Println("    search-id    - Search then get by resource ID (1 step)")
	fmt.Println("    disambiguate - Ambiguous resource disambiguation (1 step)")
	fmt.Println("    context-target - Context target carryover (2 steps)")
	fmt.Println("    discovery    - Infrastructure discovery test (2 steps)")
	fmt.Println()
	fmt.Println("  Guest Control:")
	fmt.Println("    guest-control    - Stop + start a guest via @mentions (2 steps)")
	fmt.Println("    guest-idempotent - Idempotent stop (stop twice + start, 3 steps)")
	fmt.Println("    guest-discovery  - Stop without @mentions (discovery path, 2 steps)")
	fmt.Println("    guest-natural    - Natural language variations (turn off, shut down, 4 steps)")
	fmt.Println("    guest-multi      - Multi-mention status query (2 resources, 1 step)")
	fmt.Println()
	fmt.Println("  Safety & Filtering:")
	fmt.Println("    readonly-filtering  - Control tools excluded from read-only queries (3 steps)")
	fmt.Println("    read-loop-recovery  - Model produces text after budget blocks (2 steps)")
	fmt.Println("    ambiguous-intent    - Ambiguous requests default to read-only (3 steps)")
	fmt.Println()
	fmt.Println("  Advanced:")
	fmt.Println("    troubleshoot - Multi-step troubleshooting workflow (4 steps)")
	fmt.Println("    deepdive     - Deep investigation of a service (4 steps)")
	fmt.Println("    config       - Configuration file inspection (3 steps)")
	fmt.Println("    resources    - Resource analysis and comparison (3 steps)")
	fmt.Println("    multinode    - Multi-node operations (3 steps)")
	fmt.Println("    docker       - Docker-in-LXC operations (3 steps)")
	fmt.Println("    context      - Context chain / follow-up questions (4 steps)")
	fmt.Println("    writeverify  - Write + verify FSM flow (1 step)")
	fmt.Println("    strict       - Strict resolution block + recovery (2 steps)")
	fmt.Println("    strict-block - Strict resolution block only (1 step)")
	fmt.Println("    strict-recovery - Strict resolution recovery (1 step)")
	fmt.Println("    readonly-guardrails - Read-only enforcement (1 step)")
	fmt.Println("    noninteractive    - Non-interactive guardrails (1 step)")
	fmt.Println("    approval    - Approval flow (1 step, opt-in)")
	fmt.Println("    approval-approve - Approval approve flow (1 step, opt-in)")
	fmt.Println("    approval-deny - Approval deny flow (1 step, opt-in)")
	fmt.Println("    approval-combo - Approval approve + deny in one session (2 steps, opt-in)")
	fmt.Println()
	fmt.Println("  Patrol:")
	fmt.Println("    patrol              - Run all patrol scenarios")
	fmt.Println("    patrol-basic        - Basic patrol run (completion, tools, findings)")
	fmt.Println("    patrol-investigation - Investigation quality (investigate before report)")
	fmt.Println("    patrol-finding-quality - Finding validation (well-formed findings)")
	fmt.Println("    patrol-signal-coverage - Signal-to-finding coverage scoring")
	fmt.Println()
	fmt.Println("  Collections:")
	fmt.Println("    all          - Run all basic scenarios")
	fmt.Println("    matrix       - Model matrix quick run (smoke + readonly)")
	fmt.Println("    advanced     - Run all advanced scenarios")
	fmt.Println("    full         - Run everything")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  go run ./cmd/eval -scenario troubleshoot")
	fmt.Println("  go run ./cmd/eval -scenario patrol-basic")
}

func getPatrolScenarios(name string) []eval.PatrolScenario {
	switch name {
	case "patrol":
		return eval.AllPatrolScenarios()
	case "patrol-basic":
		return []eval.PatrolScenario{eval.PatrolBasicScenario()}
	case "patrol-investigation":
		return []eval.PatrolScenario{eval.PatrolInvestigationScenario()}
	case "patrol-finding-quality":
		return []eval.PatrolScenario{eval.PatrolFindingQualityScenario()}
	case "patrol-signal-coverage", "patrol-quality":
		return []eval.PatrolScenario{eval.PatrolSignalCoverageScenario()}
	default:
		return nil
	}
}

func getScenarios(name string) []eval.Scenario {
	switch name {
	// Basic scenarios
	case "smoke":
		return []eval.Scenario{eval.QuickSmokeTest()}
	case "readonly":
		return []eval.Scenario{eval.ReadOnlyInfrastructureScenario()}
	case "enforce":
		return []eval.Scenario{eval.ExplicitToolEnforcementScenario()}
	case "routing":
		return []eval.Scenario{eval.RoutingValidationScenario()}
	case "routing-recovery":
		return []eval.Scenario{eval.RoutingMismatchRecoveryScenario()}
	case "logs":
		return []eval.Scenario{eval.LogTailingScenario()}
	case "readonly-recovery":
		return []eval.Scenario{eval.ReadOnlyViolationRecoveryScenario()}
	case "search-id":
		return []eval.Scenario{eval.SearchByIDScenario()}
	case "disambiguate":
		return []eval.Scenario{eval.AmbiguousResourceDisambiguationScenario()}
	case "context-target":
		return []eval.Scenario{eval.ContextTargetCarryoverScenario()}
	case "discovery":
		return []eval.Scenario{eval.DiscoveryScenario()}

	// Guest control scenarios
	case "guest-control":
		return []eval.Scenario{eval.GuestControlStopScenario()}
	case "guest-idempotent":
		return []eval.Scenario{eval.GuestControlIdempotentScenario()}
	case "guest-discovery":
		return []eval.Scenario{eval.GuestControlDiscoveryScenario()}
	case "guest-natural":
		return []eval.Scenario{eval.GuestControlNaturalLanguageScenario()}
	case "guest-multi":
		return []eval.Scenario{eval.GuestControlMultiMentionScenario()}

	// Safety & filtering scenarios
	case "readonly-filtering":
		return []eval.Scenario{eval.ReadOnlyToolFilteringScenario()}
	case "read-loop-recovery":
		return []eval.Scenario{eval.ReadLoopRecoveryScenario()}
	case "ambiguous-intent":
		return []eval.Scenario{eval.AmbiguousIntentScenario()}

	// Advanced scenarios
	case "troubleshoot":
		return []eval.Scenario{eval.TroubleshootingScenario()}
	case "deepdive":
		return []eval.Scenario{eval.DeepDiveScenario()}
	case "config":
		return []eval.Scenario{eval.ConfigInspectionScenario()}
	case "resources":
		return []eval.Scenario{eval.ResourceAnalysisScenario()}
	case "multinode":
		return []eval.Scenario{eval.MultiNodeScenario()}
	case "docker":
		return []eval.Scenario{eval.DockerInDockerScenario()}
	case "context":
		return []eval.Scenario{eval.ContextChainScenario()}
	case "writeverify":
		return []eval.Scenario{eval.WriteVerifyScenario()}
	case "strict":
		return []eval.Scenario{eval.StrictResolutionScenario()}
	case "strict-block":
		return []eval.Scenario{eval.StrictResolutionBlockScenario()}
	case "strict-recovery":
		return []eval.Scenario{eval.StrictResolutionRecoveryScenario()}
	case "readonly-guardrails":
		return []eval.Scenario{eval.ReadOnlyEnforcementScenario()}
	case "noninteractive":
		return []eval.Scenario{eval.NonInteractiveGuardrailScenario()}
	case "approval":
		return []eval.Scenario{eval.ApprovalScenario()}
	case "approval-approve":
		return []eval.Scenario{eval.ApprovalApproveScenario()}
	case "approval-deny":
		return []eval.Scenario{eval.ApprovalDenyScenario()}
	case "approval-combo":
		return []eval.Scenario{eval.ApprovalComboScenario()}

	// Collections
	case "all":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.ExplicitToolEnforcementScenario(),
			eval.RoutingValidationScenario(),
			eval.RoutingMismatchRecoveryScenario(),
			eval.LogTailingScenario(),
			eval.ReadOnlyViolationRecoveryScenario(),
			eval.SearchByIDScenario(),
			eval.AmbiguousResourceDisambiguationScenario(),
			eval.ContextTargetCarryoverScenario(),
			eval.DiscoveryScenario(),
		}
	case "matrix":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
		}
	case "advanced":
		return []eval.Scenario{
			eval.TroubleshootingScenario(),
			eval.DeepDiveScenario(),
			eval.ConfigInspectionScenario(),
			eval.ResourceAnalysisScenario(),
			eval.MultiNodeScenario(),
			eval.DockerInDockerScenario(),
			eval.ContextChainScenario(),
			eval.WriteVerifyScenario(),
			eval.GuestControlStopScenario(),
			eval.GuestControlIdempotentScenario(),
			eval.GuestControlDiscoveryScenario(),
			eval.GuestControlNaturalLanguageScenario(),
			eval.GuestControlMultiMentionScenario(),
			eval.ReadOnlyToolFilteringScenario(),
			eval.ReadLoopRecoveryScenario(),
			eval.AmbiguousIntentScenario(),
			eval.StrictResolutionScenario(),
			eval.StrictResolutionBlockScenario(),
			eval.StrictResolutionRecoveryScenario(),
			eval.ReadOnlyEnforcementScenario(),
			eval.NonInteractiveGuardrailScenario(),
			eval.ApprovalComboScenario(),
		}
	case "full":
		return []eval.Scenario{
			eval.QuickSmokeTest(),
			eval.ReadOnlyInfrastructureScenario(),
			eval.ExplicitToolEnforcementScenario(),
			eval.RoutingValidationScenario(),
			eval.RoutingMismatchRecoveryScenario(),
			eval.LogTailingScenario(),
			eval.ReadOnlyViolationRecoveryScenario(),
			eval.SearchByIDScenario(),
			eval.AmbiguousResourceDisambiguationScenario(),
			eval.ContextTargetCarryoverScenario(),
			eval.DiscoveryScenario(),
			eval.TroubleshootingScenario(),
			eval.DeepDiveScenario(),
			eval.ConfigInspectionScenario(),
			eval.ResourceAnalysisScenario(),
			eval.MultiNodeScenario(),
			eval.DockerInDockerScenario(),
			eval.ContextChainScenario(),
			eval.WriteVerifyScenario(),
			eval.GuestControlStopScenario(),
			eval.GuestControlIdempotentScenario(),
			eval.GuestControlDiscoveryScenario(),
			eval.GuestControlNaturalLanguageScenario(),
			eval.GuestControlMultiMentionScenario(),
			eval.ReadOnlyToolFilteringScenario(),
			eval.ReadLoopRecoveryScenario(),
			eval.AmbiguousIntentScenario(),
			eval.StrictResolutionScenario(),
			eval.StrictResolutionBlockScenario(),
			eval.StrictResolutionRecoveryScenario(),
			eval.ReadOnlyEnforcementScenario(),
			eval.NonInteractiveGuardrailScenario(),
			eval.ApprovalComboScenario(),
		}
	default:
		return nil
	}
}

func envBool(key string) (bool, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func envInt(key string) (int, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}
	var parsed int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err != nil {
		return 0, false
	}
	return parsed, true
}

func parseModelList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		models = append(models, trimmed)
	}
	return models
}

type apiModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Notable     bool   `json:"notable"`
	CreatedAt   int64  `json:"created_at,omitempty"`
}

type apiModelsResponse struct {
	Models []apiModelInfo `json:"models"`
	Error  string         `json:"error,omitempty"`
}

type providerStats struct {
	Total   int
	Notable int
}

type autoSelectionDetail struct {
	Provider  string
	ID        string
	Name      string
	Notable   bool
	CreatedAt int64
	Reason    string
}

func fetchAutoModels(baseURL, user, pass string) ([]string, []autoSelectionDetail, map[string]providerStats, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, nil, nil, fmt.Errorf("base URL is required")
	}

	req, err := http.NewRequest("GET", strings.TrimRight(baseURL, "/")+"/api/ai/models", nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build models request: %w", err)
	}
	req.SetBasicAuth(user, pass)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("models request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			readBodyErr := fmt.Errorf("failed to read models error response body: %w", readErr)
			if closeErr != nil {
				return nil, nil, nil, errors.Join(readBodyErr, fmt.Errorf("failed to close models response body: %w", closeErr))
			}
			return nil, nil, nil, readBodyErr
		}
		if closeErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to close models response body: %w", closeErr)
		}
		return nil, nil, nil, fmt.Errorf("models request returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload apiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		decodeErr := fmt.Errorf("failed to decode models response: %w", err)
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, nil, nil, errors.Join(decodeErr, fmt.Errorf("failed to close models response body: %w", closeErr))
		}
		return nil, nil, nil, decodeErr
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		return nil, nil, nil, fmt.Errorf("failed to close models response body: %w", closeErr)
	}
	if payload.Error != "" {
		return nil, nil, nil, fmt.Errorf("models API returned error: %s", payload.Error)
	}

	providerFilter := parseProviderFilterWithDefault(os.Getenv("EVAL_MODEL_PROVIDERS"))
	excludeKeywords := parseExcludeKeywords(os.Getenv("EVAL_MODEL_EXCLUDE_KEYWORDS"))
	limit := 2
	if value, ok := envInt("EVAL_MODEL_LIMIT"); ok && value > 0 {
		limit = value
	}

	grouped := make(map[string][]apiModelInfo)
	stats := make(map[string]providerStats)
	for _, model := range payload.Models {
		if model.ID == "" {
			continue
		}
		parts := strings.SplitN(model.ID, ":", 2)
		provider := parts[0]
		if provider == "" {
			continue
		}
		if len(providerFilter) > 0 && !providerFilter[provider] {
			continue
		}
		if len(excludeKeywords) > 0 && hasAnyKeyword(model, excludeKeywords) {
			continue
		}
		grouped[provider] = append(grouped[provider], model)
		stat := stats[provider]
		stat.Total++
		if model.Notable {
			stat.Notable++
		}
		stats[provider] = stat
	}

	if len(grouped) == 0 {
		return nil, nil, stats, fmt.Errorf("no models found for auto-selection")
	}

	providers := make([]string, 0, len(grouped))
	for provider := range grouped {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	seen := make(map[string]bool)
	selected := make([]string, 0, len(grouped)*limit)
	details := make([]autoSelectionDetail, 0, len(grouped)*limit)
	for _, provider := range providers {
		models := grouped[provider]
		sort.Slice(models, func(i, j int) bool {
			if models[i].Notable != models[j].Notable {
				return models[i].Notable
			}
			if models[i].CreatedAt != models[j].CreatedAt {
				return models[i].CreatedAt > models[j].CreatedAt
			}
			return models[i].ID < models[j].ID
		})
		for _, model := range models {
			if len(selected) >= len(grouped)*limit {
				break
			}
			if seen[model.ID] {
				continue
			}
			seen[model.ID] = true
			selected = append(selected, model.ID)
			details = append(details, autoSelectionDetail{
				Provider:  provider,
				ID:        model.ID,
				Name:      model.Name,
				Notable:   model.Notable,
				CreatedAt: model.CreatedAt,
				Reason:    selectionReason(model, stats[provider]),
			})
			if countProvider(selected, provider) >= limit {
				break
			}
		}
	}

	if len(selected) == 0 {
		return nil, nil, stats, fmt.Errorf("auto-selection produced no models")
	}
	return selected, details, stats, nil
}

func parseProviderFilter(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out[trimmed] = true
	}
	return out
}

func parseProviderFilterWithDefault(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]bool{
			"openai":    true,
			"anthropic": true,
			"deepseek":  true,
			"gemini":    true,
			"ollama":    true,
		}
	}
	return parseProviderFilter(raw)
}

func parseExcludeKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{
			"codex",
			"openai:gpt-5.2-pro",
			"image",
			"vision",
			"video",
			"audio",
			"speech",
			"embed",
			"embedding",
			"moderation",
			"rerank",
			"tts",
			"realtime",
			"transcribe",
		}
	}
	switch strings.ToLower(raw) {
	case "0", "false", "off", "none":
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, strings.ToLower(trimmed))
	}
	return out
}

func hasAnyKeyword(model apiModelInfo, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	target := strings.ToLower(model.ID + " " + model.Name + " " + model.Description)
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(target, keyword) {
			return true
		}
	}
	return false
}

func countProvider(models []string, provider string) int {
	if provider == "" {
		return 0
	}
	count := 0
	prefix := provider + ":"
	for _, model := range models {
		if strings.HasPrefix(model, prefix) {
			count++
		}
	}
	return count
}

func selectionReason(model apiModelInfo, stat providerStats) string {
	parts := make([]string, 0, 2)
	if stat.Notable == 0 {
		parts = append(parts, "no notable models")
	}
	if model.Notable {
		parts = append(parts, "notable")
	} else if model.CreatedAt > 0 {
		created := time.Unix(model.CreatedAt, 0).UTC().Format("2006-01-02")
		parts = append(parts, "created_at="+created)
	} else {
		parts = append(parts, "fallback")
	}
	return strings.Join(parts, "; ")
}

func sortedProviders(stats map[string]providerStats) []string {
	providers := make([]string, 0, len(stats))
	for provider := range stats {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}
