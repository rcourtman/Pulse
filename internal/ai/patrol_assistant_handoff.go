package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
)

const maxPatrolRunAssistantHandoffResources = 8

var patrolRunDSMLTracePattern = regexp.MustCompile(`<｜DSML｜[^>]*>[\s\S]*?</｜DSML｜[^>]*>|<｜DSML｜[^>]*>`)

// PatrolRunAssistantHandoff is the backend-owned, model-only Assistant context
// for a persisted Patrol run. The fields are safe to store as chat session
// metadata, but still carry no approval or execution authority.
type PatrolRunAssistantHandoff struct {
	Context   string
	Resources []chat.HandoffResource
	Metadata  chat.HandoffMetadata
}

// BuildPatrolRunAssistantHandoff converts a durable Patrol run record into the
// canonical Assistant handoff envelope. Frontend callers should pass only the
// run identity; the backend rehydrates this context from Patrol history.
func BuildPatrolRunAssistantHandoff(run PatrolRunRecord) PatrolRunAssistantHandoff {
	run = normalizePatrolRunRecord(run)
	runID := strings.TrimSpace(run.ID)
	runType := patrolRunKindLabel(run.Type)
	status := patrolRunStatusLabel(run)
	runtimeFailure := patrolRunRuntimeFailureSummary(run)
	resources := patrolRunAssistantHandoffResources(run)

	return PatrolRunAssistantHandoff{
		Context:   buildPatrolRunAssistantContext(run, runType, status, runtimeFailure),
		Resources: resources,
		Metadata: chat.HandoffMetadata{
			Kind:           "patrol_run",
			RunID:          runID,
			RunType:        runType,
			RunStatus:      status,
			RuntimeFailure: runtimeFailure != "",
		},
	}
}

func buildPatrolRunAssistantContext(run PatrolRunRecord, runType, status, runtimeFailure string) string {
	lines := []string{
		"[Patrol Run Context]",
		"Source: Pulse Patrol run history",
		formatPatrolRunContextLine("Run ID", run.ID),
		formatPatrolRunContextLine("Run Type", runType),
		formatPatrolRunContextLine("Status", status),
		formatPatrolRunContextLine("Trigger", patrolTriggerReasonLabel(run.TriggerReason)),
		formatPatrolRunContextLine("Timing", patrolRunTimingSummary(run)),
		formatPatrolRunContextLine("Coverage", patrolRunCoverageSummary(run)),
		formatPatrolRunContextLine("Scope", patrolRunScopeSummary(run)),
		formatPatrolRunContextLine("Findings Snapshot", patrolRunFindingsSnapshot(run)),
		formatPatrolRunContextLine("Outcomes", patrolRunOutcomeSummary(run)),
		formatPatrolRunContextLine("Runtime Failure", runtimeFailure),
		formatPatrolRunContextLine("Effort", patrolRunEffortSummary(run)),
		formatPatrolRunContextLine("Findings Summary", run.FindingsSummary),
		formatPatrolRunContextLine("Patrol Analysis", truncatePatrolRunContextText(sanitizePatrolRunAnalysis(run.AIAnalysis), 500)),
		"Operator Boundary: This Patrol run handoff is model-only context for explanation and review. Configuration changes, diagnostics, remediation, and command execution require explicit governed operator action.",
	}

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func patrolRunAssistantHandoffResources(run PatrolRunRecord) []chat.HandoffResource {
	ids := run.EffectiveScopeResourceIDs
	if ids == nil {
		ids = run.ScopeResourceIDs
	}
	if len(ids) == 0 {
		return nil
	}

	resourceType := ""
	if len(run.ScopeResourceTypes) == 1 {
		resourceType = strings.TrimSpace(run.ScopeResourceTypes[0])
	}

	resources := make([]chat.HandoffResource, 0, min(len(ids), maxPatrolRunAssistantHandoffResources))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resources = append(resources, chat.HandoffResource{
			ID:   id,
			Type: resourceType,
		})
		if len(resources) >= maxPatrolRunAssistantHandoffResources {
			break
		}
	}
	if len(resources) == 0 {
		return nil
	}
	return resources
}

func patrolRunRuntimeFailureSummary(run PatrolRunRecord) string {
	summary := strings.TrimSpace(redactPatrolRuntimeFailureDetail(run.ErrorSummary))
	detail := strings.TrimSpace(summarizePatrolRuntimeFailureDetail(run.ErrorDetail))
	if summary != "" && detail != "" && summary != detail {
		return summary + ": " + truncatePatrolRunContextText(detail, 260)
	}
	if summary != "" {
		return summary
	}
	if detail != "" {
		return truncatePatrolRunContextText(detail, 260)
	}
	if run.ErrorCount > 0 {
		suffix := ""
		if run.ErrorCount != 1 {
			suffix = "s"
		}
		return fmt.Sprintf("%d Patrol runtime error%s recorded", run.ErrorCount, suffix)
	}
	return ""
}

func patrolRunKindLabel(runType string) string {
	switch strings.ToLower(strings.TrimSpace(runType)) {
	case "scoped":
		return "Scoped run"
	case "verification":
		return "Verification check"
	case "", "patrol", "full", "scheduled":
		return "Full patrol"
	default:
		return "Patrol run"
	}
}

func patrolRunStatusLabel(run PatrolRunRecord) string {
	status := strings.ToLower(strings.TrimSpace(run.Status))
	if run.ErrorCount > 0 && (status == "" || status == "healthy" || status == "completed") {
		status = "error"
	}
	switch status {
	case "issues_found":
		return "issues found"
	case "critical", "error", "healthy":
		return status
	case "":
		return "unknown"
	default:
		return strings.ReplaceAll(status, "_", " ")
	}
}

func patrolTriggerReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	case "scheduled":
		return "Scheduled"
	case "manual":
		return "Manual"
	case "startup":
		return "Startup"
	case "alert_fired":
		return "Alert fired"
	case "alert_cleared":
		return "Alert cleared"
	case "anomaly":
		return "Anomaly"
	case "user_action":
		return "User action"
	case "config_changed":
		return "Config change"
	case "":
		return ""
	default:
		return strings.ReplaceAll(strings.TrimSpace(reason), "_", " ")
	}
}

func patrolRunTimingSummary(run PatrolRunRecord) string {
	parts := []string{}
	if !run.StartedAt.IsZero() {
		parts = append(parts, "started "+run.StartedAt.UTC().Format(time.RFC3339))
	}
	if !run.CompletedAt.IsZero() {
		parts = append(parts, "completed "+run.CompletedAt.UTC().Format(time.RFC3339))
	}
	durationMs := run.DurationMs
	if durationMs == 0 && run.Duration > 0 {
		durationMs = run.Duration.Milliseconds()
	}
	if formatted := formatPatrolRunDurationMs(durationMs); formatted != "" {
		parts = append(parts, "duration "+formatted)
	}
	return strings.Join(parts, "; ")
}

func formatPatrolRunDurationMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := (ms + 500) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := (seconds + 30) / 60
	return fmt.Sprintf("%dm", minutes)
}

func patrolRunCoverageSummary(run PatrolRunRecord) string {
	resourcesChecked := max(run.ResourcesChecked, 0)
	scopedResourceCount := len(run.EffectiveScopeResourceIDs)
	if run.EffectiveScopeResourceIDs == nil {
		scopedResourceCount = len(run.ScopeResourceIDs)
	}
	if scopedResourceCount > 0 {
		if resourcesChecked > 0 && resourcesChecked < scopedResourceCount {
			return fmt.Sprintf("Checked %d of %d scoped resources", resourcesChecked, scopedResourceCount)
		}
		if resourcesChecked > 0 {
			return fmt.Sprintf("Checked %s", formatPatrolRunResourceCount(resourcesChecked, "scoped"))
		}
	}
	if resourcesChecked > 0 {
		return fmt.Sprintf("Checked %s", formatPatrolRunResourceCount(resourcesChecked, ""))
	}

	return joinPatrolRunContextParts([]string{
		countPatrolRunFact(run.NodesChecked, "nodes"),
		countPatrolRunFact(run.GuestsChecked, "VMs"),
		countPatrolRunFact(run.DockerChecked, "containers"),
		countPatrolRunFact(run.StorageChecked, "storage resources"),
		countPatrolRunFact(run.HostsChecked, "agents"),
		countPatrolRunFact(run.TrueNASChecked, "TrueNAS systems"),
		countPatrolRunFact(run.KubernetesChecked, "Kubernetes resources"),
	})
}

func formatPatrolRunResourceCount(count int, qualifier string) string {
	label := "resources"
	if count == 1 {
		label = "resource"
	}
	if strings.TrimSpace(qualifier) != "" {
		return fmt.Sprintf("%d %s %s", count, qualifier, label)
	}
	return fmt.Sprintf("%d %s", count, label)
}

func patrolRunScopeSummary(run PatrolRunRecord) string {
	ids := run.EffectiveScopeResourceIDs
	if ids == nil {
		ids = run.ScopeResourceIDs
	}
	if len(ids) > 0 {
		suffix := ""
		if len(ids) != 1 {
			suffix = "s"
		}
		return fmt.Sprintf("Scoped to %d resource%s", len(ids), suffix)
	}
	if len(run.ScopeResourceTypes) > 0 {
		return "Scoped to " + strings.Join(run.ScopeResourceTypes, ", ")
	}
	if strings.EqualFold(strings.TrimSpace(run.Type), "scoped") {
		return "Scoped"
	}
	return ""
}

func patrolRunFindingsSnapshot(run PatrolRunRecord) string {
	count := len(run.FindingIDs)
	suffix := ""
	if count != 1 {
		suffix = "s"
	}
	return fmt.Sprintf("%d finding ID%s captured", count, suffix)
}

func patrolRunOutcomeSummary(run PatrolRunRecord) string {
	return joinPatrolRunContextParts([]string{
		countPatrolRunSingularFact(run.NewFindings, "new finding"),
		countPatrolRunSingularFact(run.ExistingFindings, "existing finding"),
		countPatrolRunSingularFact(run.ResolvedFindings, "resolved finding"),
		countPatrolRunSingularFact(run.RejectedFindings, "rejected finding"),
		countPatrolRunSingularFact(run.AutoFixCount, "auto-remediation"),
		countPatrolRunSingularFact(run.ErrorCount, "error"),
	})
}

func patrolRunEffortSummary(run PatrolRunRecord) string {
	tokenCount := max(run.InputTokens, 0) + max(run.OutputTokens, 0)
	parts := []string{
		countPatrolRunSingularFact(run.ToolCallCount, "tool call"),
		countPatrolRunSingularFact(run.TriageFlags, "triage flag"),
	}
	if run.TriageSkippedLLM {
		parts = append(parts, "LLM skipped for deterministic triage")
	}
	if tokenCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", tokenCount))
	}
	return joinPatrolRunContextParts(parts)
}

func countPatrolRunFact(value int, label string) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", value, label)
}

func countPatrolRunSingularFact(value int, singular string) string {
	if value <= 0 {
		return ""
	}
	suffix := ""
	if value != 1 {
		suffix = "s"
	}
	return fmt.Sprintf("%d %s%s", value, singular, suffix)
}

func joinPatrolRunContextParts(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "; ")
}

func formatPatrolRunContextLine(label, value string) string {
	value = truncatePatrolRunContextText(value, 500)
	if value == "" {
		return ""
	}
	return label + ": " + value
}

func sanitizePatrolRunAnalysis(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	analysis := patrolRunDSMLTracePattern.ReplaceAllString(text, "")
	return strings.TrimSpace(redactPatrolRuntimeFailureDetail(analysis))
}

func truncatePatrolRunContextText(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return strings.TrimSpace(value[:limit])
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}
