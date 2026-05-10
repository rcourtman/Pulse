package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// reportFleetNarratorMaxTokens caps fleet narrative response budget.
// Fleet narratives are larger than single-resource because they
// summarise N resources, but the structured output is still bounded:
// up to fleetMaxOutliers outliers, a few patterns, and a few
// recommendations. 2500 tokens is generous enough for that envelope
// without inviting padding.
const reportFleetNarratorMaxTokens = 2500

// reportFleetNarratorUseCase is the cost-ledger label for AI fleet
// narrative calls. Distinct from report_narrative so operators can see
// fleet vs single-resource spend separately in the AI usage dashboard.
const reportFleetNarratorUseCase = "report_narrative_fleet"

// reportFleetNarratorSystemPrompt instructs the model to interpret a
// cross-resource view. Severity, outlier count, and JSON schema are
// constrained so unknown values do not silently render as muted.
const reportFleetNarratorSystemPrompt = `You are Pulse Assistant generating the executive summary section of a sysadmin FLEET performance report.

You MUST:
- Interpret the structured fleet data in the user message. If the data does not support a claim, do not make it.
- Reference specific named resources for outliers. Use the resource_name field where present, otherwise resource_id verbatim. Do NOT invent resource names.
- Use observation severity strictly from this set: "ok", "info", "warning", "critical". Map clean state to "ok", informational facts to "info", concerning trends to "warning", and immediate-action items to "critical".
- Pick at most 5 outliers — the resources most worth investigating. Order by severity. Do not list every resource.
- Patterns describe cross-cutting trends ("3 of 8 resources show memory pressure"), not individual resources.
- Recommendations are fleet-scoped imperatives ("review memory allocation across the fleet"), not per-resource fixes.
- Keep prose concrete and short. Avoid hedging adverbs.

Respond ONLY with a single JSON object matching this exact schema (no markdown fences, no commentary outside the JSON):

{
  "health_status": "HEALTHY" | "WARNING" | "CRITICAL",
  "health_message": "<one short sentence>",
  "executive_summary": "<2-4 sentence paragraph framing the fleet's week>",
  "outliers": [
    { "resource_id": "<id>", "resource_name": "<name>", "reason": "<one sentence>", "severity": "ok" | "info" | "warning" | "critical" }
  ],
  "patterns": [
    { "text": "<one sentence cross-cutting pattern>", "severity": "ok" | "info" | "warning" | "critical" }
  ],
  "recommendations": [ "<one sentence fleet-scoped imperative>" ],
  "period_comparison": "<optional paragraph; empty string if no prior data>"
}`

// reportFleetPayload is what the model receives. Compact per-resource
// rows so the prompt scales with fleet size without exploding token
// usage.
type reportFleetPayload struct {
	Title       string                       `json:"title"`
	Period      reportNarratorPeriod         `json:"period"`
	PriorPeriod *reportFleetPeriodOnly       `json:"prior_period,omitempty"`
	Aggregate   reportFleetAggregate         `json:"aggregate"`
	Resources   []reportFleetResourceSummary `json:"resources"`
}

type reportFleetPeriodOnly struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type reportFleetAggregate struct {
	ResourceCount       int     `json:"resource_count"`
	TotalActiveAlerts   int     `json:"total_active_alerts"`
	TotalCriticalAlerts int     `json:"total_critical_alerts"`
	TotalResolvedAlerts int     `json:"total_resolved_alerts"`
	TotalFindings       int     `json:"total_findings"`
	AvgCPUMean          float64 `json:"avg_cpu_mean"`
	AvgMemoryMean       float64 `json:"avg_memory_mean"`
	AvgDiskMean         float64 `json:"avg_disk_mean"`
	MaxCPUSeen          float64 `json:"max_cpu_seen"`
	MaxMemorySeen       float64 `json:"max_memory_seen"`
	MaxDiskSeen         float64 `json:"max_disk_seen"`
}

type reportFleetResourceSummary struct {
	ResourceID       string  `json:"resource_id"`
	ResourceName     string  `json:"resource_name,omitempty"`
	ResourceType     string  `json:"resource_type"`
	Status           string  `json:"status,omitempty"`
	AvgCPU           float64 `json:"avg_cpu"`
	MaxCPU           float64 `json:"max_cpu"`
	AvgMemory        float64 `json:"avg_memory"`
	MaxMemory        float64 `json:"max_memory"`
	AvgDisk          float64 `json:"avg_disk"`
	MaxDisk          float64 `json:"max_disk"`
	ActiveAlerts     int     `json:"active_alerts"`
	CriticalAlerts   int     `json:"critical_alerts"`
	ResolvedAlerts   int     `json:"resolved_alerts"`
	UnhealthyDisks   int     `json:"unhealthy_disks,omitempty"`
	StoragePoolsHigh int     `json:"storage_pools_high,omitempty"`
	Findings         int     `json:"findings,omitempty"`
}

type reportFleetResponse struct {
	HealthStatus     string                       `json:"health_status"`
	HealthMessage    string                       `json:"health_message"`
	ExecutiveSummary string                       `json:"executive_summary"`
	Outliers         []reportFleetResponseOutlier `json:"outliers"`
	Patterns         []reportFleetResponsePattern `json:"patterns"`
	Recommendations  []string                     `json:"recommendations"`
	PeriodComparison string                       `json:"period_comparison"`
}

type reportFleetResponseOutlier struct {
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Reason       string `json:"reason"`
	Severity     string `json:"severity"`
}

type reportFleetResponsePattern struct {
	Text     string `json:"text"`
	Severity string `json:"severity"`
}

// Compile-time assertion the Service satisfies the FleetNarrator interface.
var _ reporting.FleetNarrator = (*Service)(nil)

// NarrateFleet implements reporting.FleetNarrator. Same shape as
// Narrate: single-turn JSON call, fail closed so the engine falls
// back to the heuristic fleet narrator on any error.
func (s *Service) NarrateFleet(ctx context.Context, in reporting.FleetNarrativeInput) (reporting.FleetNarrative, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.cfg
	costStore := s.costStore
	s.mu.RUnlock()

	if provider == nil {
		return reporting.FleetNarrative{}, errors.New("Pulse Assistant is not configured")
	}

	model := ""
	if cfg != nil {
		if cfg.PatrolModel != "" {
			model = cfg.PatrolModel
		} else {
			model = cfg.GetChatModel()
		}
	}

	if err := s.enforceBudget(reportFleetNarratorUseCase); err != nil {
		return reporting.FleetNarrative{}, err
	}

	payload := buildReportFleetPayload(in)
	body, err := json.Marshal(payload)
	if err != nil {
		return reporting.FleetNarrative{}, fmt.Errorf("encode fleet payload: %w", err)
	}

	chatReq := providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: string(body)},
		},
		Model:       model,
		System:      reportFleetNarratorSystemPrompt,
		MaxTokens:   reportFleetNarratorMaxTokens,
		ExecutionID: uuid.NewString(),
	}
	if sanitizer := s.requestSanitizerForModel(model); sanitizer != nil {
		chatReq = sanitizer(chatReq)
	}

	resp, err := provider.Chat(ctx, chatReq)
	if err != nil {
		return reporting.FleetNarrative{}, fmt.Errorf("provider chat: %w", err)
	}

	// Record token usage in the operator-facing cost ledger. Recording
	// happens before parsing so failed-but-billed calls are still
	// visible — operator was billed regardless.
	if costStore != nil {
		providerName, _ := config.ParseModelString(model)
		if providerName == "" {
			providerName = provider.Name()
		}
		costStore.Record(cost.UsageEvent{
			Timestamp:     time.Now(),
			Provider:      providerName,
			RequestModel:  model,
			ResponseModel: resp.Model,
			UseCase:       reportFleetNarratorUseCase,
			InputTokens:   resp.InputTokens,
			OutputTokens:  resp.OutputTokens,
			TargetType:    "fleet",
			TargetID:      strings.TrimSpace(in.Title),
		})
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return reporting.FleetNarrative{}, errors.New("provider returned empty fleet narrative")
	}

	parsed, err := parseReportFleetResponse(content)
	if err != nil {
		return reporting.FleetNarrative{}, err
	}

	narrative := reporting.FleetNarrative{
		Source:           reporting.NarrativeSourceAI,
		HealthStatus:     normalizeReportHealthStatus(parsed.HealthStatus),
		HealthMessage:    strings.TrimSpace(parsed.HealthMessage),
		ExecutiveSummary: strings.TrimSpace(parsed.ExecutiveSummary),
		PeriodComparison: strings.TrimSpace(parsed.PeriodComparison),
		Disclaimer:       "Fleet narrative generated by Pulse Assistant. Verify against the resource summary table and per-resource pages.",
	}

	for _, o := range parsed.Outliers {
		reason := strings.TrimSpace(o.Reason)
		id := strings.TrimSpace(o.ResourceID)
		name := strings.TrimSpace(o.ResourceName)
		if reason == "" || (id == "" && name == "") {
			continue
		}
		narrative.Outliers = append(narrative.Outliers, reporting.FleetOutlier{
			ResourceID:   id,
			ResourceName: name,
			Reason:       reason,
			Severity:     normalizeBulletSeverity(o.Severity),
		})
	}
	for _, p := range parsed.Patterns {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			continue
		}
		narrative.Patterns = append(narrative.Patterns, reporting.NarrativeBullet{
			Text:     text,
			Severity: normalizeBulletSeverity(p.Severity),
		})
	}
	for _, r := range parsed.Recommendations {
		r = strings.TrimSpace(r)
		if r != "" {
			narrative.Recommendations = append(narrative.Recommendations, r)
		}
	}
	if narrative.HealthStatus == "" || (len(narrative.Outliers) == 0 && len(narrative.Patterns) == 0 && len(narrative.Recommendations) == 0) {
		return reporting.FleetNarrative{}, errors.New("provider returned empty or invalid fleet narrative")
	}
	return narrative, nil
}

func buildReportFleetPayload(in reporting.FleetNarrativeInput) reportFleetPayload {
	out := reportFleetPayload{
		Title: in.Title,
		Period: reportNarratorPeriod{
			Start: in.Period.Start.UTC().Format("2006-01-02T15:04:05Z"),
			End:   in.Period.End.UTC().Format("2006-01-02T15:04:05Z"),
			Hours: int(in.Period.End.Sub(in.Period.Start).Hours()),
		},
		Aggregate: reportFleetAggregate{
			ResourceCount:       in.Aggregate.ResourceCount,
			TotalActiveAlerts:   in.Aggregate.TotalActiveAlerts,
			TotalCriticalAlerts: in.Aggregate.TotalCriticalAlerts,
			TotalResolvedAlerts: in.Aggregate.TotalResolvedAlerts,
			TotalFindings:       in.Aggregate.TotalFindings,
			AvgCPUMean:          in.Aggregate.AvgCPUMean,
			AvgMemoryMean:       in.Aggregate.AvgMemoryMean,
			AvgDiskMean:         in.Aggregate.AvgDiskMean,
			MaxCPUSeen:          in.Aggregate.MaxCPUSeen,
			MaxMemorySeen:       in.Aggregate.MaxMemorySeen,
			MaxDiskSeen:         in.Aggregate.MaxDiskSeen,
		},
	}
	if in.PriorPeriod != nil {
		out.PriorPeriod = &reportFleetPeriodOnly{
			Start: in.PriorPeriod.Start.UTC().Format("2006-01-02T15:04:05Z"),
			End:   in.PriorPeriod.End.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	out.Resources = make([]reportFleetResourceSummary, 0, len(in.Resources))
	for _, r := range in.Resources {
		out.Resources = append(out.Resources, reportFleetResourceSummary{
			ResourceID:       r.ResourceID,
			ResourceName:     r.ResourceName,
			ResourceType:     r.ResourceType,
			Status:           r.Status,
			AvgCPU:           r.AvgCPU,
			MaxCPU:           r.MaxCPU,
			AvgMemory:        r.AvgMemory,
			MaxMemory:        r.MaxMemory,
			AvgDisk:          r.AvgDisk,
			MaxDisk:          r.MaxDisk,
			ActiveAlerts:     r.ActiveAlerts,
			CriticalAlerts:   r.CriticalAlerts,
			ResolvedAlerts:   r.ResolvedAlerts,
			UnhealthyDisks:   r.UnhealthyDisks,
			StoragePoolsHigh: r.StoragePoolsHigh,
			Findings:         r.Findings,
		})
	}
	return out
}

func parseReportFleetResponse(raw string) (reportFleetResponse, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	var parsed reportFleetResponse
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return reportFleetResponse{}, fmt.Errorf("decode fleet narrative JSON: %w", err)
	}
	return parsed, nil
}
