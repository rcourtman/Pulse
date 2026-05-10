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

// reportNarratorUseCase is the cost-ledger label for AI-narrated report
// generations. Operators see this value in the AI usage dashboard so
// report-narrative spend is distinguishable from chat or patrol.
const reportNarratorUseCase = "report_narrative"

// reportNarratorMaxTokens caps the response budget for narrative generation.
// The output is short prose plus a small JSON envelope, so a tight ceiling
// keeps cost predictable and discourages padding.
const reportNarratorMaxTokens = 1500

// reportNarratorSystemPrompt instructs the model to ground every claim in
// the provided structured data and to refuse interpretation it cannot
// support from the input. Severity is constrained to the set the renderer
// understands so unknown values do not silently render as muted.
const reportNarratorSystemPrompt = `You are Pulse Assistant generating the executive summary section of a sysadmin performance report.

You MUST:
- Ground every observation and recommendation in the structured data provided in the user message. If the data does not support a claim, do not make it.
- Reference the deterministic data tables that accompany this narrative (Performance Summary, Active Alerts, Storage, Disks). Do NOT fabricate metric values, alert messages, disk identifiers, or finding titles.
- Use observation severity strictly from this set: "ok", "info", "warning", "critical". Map clean state to "ok", informational facts to "info", concerning trends to "warning", and immediate-action items to "critical".
- Keep observations to short, concrete sentences. Avoid hedging adverbs ("perhaps", "seems"). State the fact and its implication.
- When prior-period data is supplied, write a period_comparison paragraph describing the most material deltas (resource trends, new or resolved alerts, new findings). When no prior data is supplied, leave period_comparison empty.
- Do NOT invent recommendations for problems that aren't in the data. If everything is healthy, say so plainly.

Respond ONLY with a single JSON object matching this exact schema (no markdown fences, no commentary outside the JSON):

{
  "health_status": "HEALTHY" | "WARNING" | "CRITICAL",
  "health_message": "<one short sentence>",
  "executive_summary": "<2-4 sentence paragraph>",
  "observations": [
    { "text": "<one sentence>", "severity": "ok" | "info" | "warning" | "critical" }
  ],
  "recommendations": [ "<one sentence imperative>" ],
  "period_comparison": "<optional paragraph; empty string if no prior data>"
}`

// reportNarratorPayload is the structured data sent to the model. Keeping
// it explicit (rather than json.Marshal-ing reporting types directly) makes
// the prompt surface stable as internal types evolve.
type reportNarratorPayload struct {
	Title        string                       `json:"title"`
	ResourceType string                       `json:"resource_type"`
	ResourceID   string                       `json:"resource_id"`
	Period       reportNarratorPeriod         `json:"period"`
	Resource     *reportNarratorResource      `json:"resource,omitempty"`
	MetricStats  map[string]reportNarratorMS  `json:"metric_stats"`
	PriorPeriod  *reportNarratorPriorPeriod   `json:"prior_period,omitempty"`
	Alerts       []reportNarratorAlert        `json:"alerts,omitempty"`
	Storage      []reportNarratorStorage      `json:"storage,omitempty"`
	Disks        []reportNarratorDisk         `json:"disks,omitempty"`
	Findings     []reportNarratorFindingEntry `json:"patrol_findings,omitempty"`
}

type reportNarratorPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Hours int    `json:"hours"`
}

type reportNarratorResource struct {
	Name        string  `json:"name,omitempty"`
	DisplayName string  `json:"display_name,omitempty"`
	Status      string  `json:"status,omitempty"`
	Node        string  `json:"node,omitempty"`
	UptimeDays  int64   `json:"uptime_days,omitempty"`
	CPUCores    int     `json:"cpu_cores,omitempty"`
	MemoryGB    float64 `json:"memory_gb,omitempty"`
}

type reportNarratorMS struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
	Current float64 `json:"current"`
	Count   int     `json:"count"`
}

type reportNarratorPriorPeriod struct {
	Start       string                      `json:"start"`
	End         string                      `json:"end"`
	MetricStats map[string]reportNarratorMS `json:"metric_stats"`
}

type reportNarratorAlert struct {
	Type      string  `json:"type"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Value     float64 `json:"value,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Resolved  bool    `json:"resolved"`
}

type reportNarratorStorage struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	UsagePerc float64 `json:"usage_perc"`
	ZFSHealth string  `json:"zfs_health,omitempty"`
}

type reportNarratorDisk struct {
	Device    string `json:"device"`
	Type      string `json:"type"`
	Health    string `json:"health,omitempty"`
	WearLevel int    `json:"wear_level,omitempty"`
}

type reportNarratorFindingEntry struct {
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	Resolved       bool   `json:"resolved"`
}

// reportNarratorResponse is the JSON envelope the model is asked to return.
type reportNarratorResponse struct {
	HealthStatus     string                         `json:"health_status"`
	HealthMessage    string                         `json:"health_message"`
	ExecutiveSummary string                         `json:"executive_summary"`
	Observations     []reportNarratorResponseBullet `json:"observations"`
	Recommendations  []string                       `json:"recommendations"`
	PeriodComparison string                         `json:"period_comparison"`
}

type reportNarratorResponseBullet struct {
	Text     string `json:"text"`
	Severity string `json:"severity"`
}

// Compile-time assertion the Service satisfies the reporting.Narrator interface.
var _ reporting.Narrator = (*Service)(nil)

// Narrate implements reporting.Narrator. It builds a single-turn prompt
// from the supplied input, asks the configured provider for a JSON
// response, and parses it into a reporting.Narrative. Returning an error
// causes the engine to fall back to the heuristic narrator, so callers do
// not need to distinguish "AI disabled" from "AI failed".
func (s *Service) Narrate(ctx context.Context, in reporting.NarrativeInput) (reporting.Narrative, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.cfg
	costStore := s.costStore
	s.mu.RUnlock()

	if provider == nil {
		return reporting.Narrative{}, errors.New("Pulse Assistant is not configured")
	}

	model := ""
	if cfg != nil {
		if cfg.PatrolModel != "" {
			model = cfg.PatrolModel
		} else {
			model = cfg.GetChatModel()
		}
	}

	if err := s.enforceBudget(reportNarratorUseCase); err != nil {
		return reporting.Narrative{}, err
	}

	payload := buildReportNarratorPayload(in)
	body, err := json.Marshal(payload)
	if err != nil {
		return reporting.Narrative{}, fmt.Errorf("encode report payload: %w", err)
	}

	chatReq := providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: string(body)},
		},
		Model:       model,
		System:      reportNarratorSystemPrompt,
		MaxTokens:   reportNarratorMaxTokens,
		ExecutionID: uuid.NewString(),
	}
	if sanitizer := s.requestSanitizerForModel(model); sanitizer != nil {
		chatReq = sanitizer(chatReq)
	}

	resp, err := provider.Chat(ctx, chatReq)
	if err != nil {
		return reporting.Narrative{}, fmt.Errorf("provider chat: %w", err)
	}

	// Record token usage in the operator-facing cost ledger so AI-narrated
	// report generation shows up in the AI usage dashboard alongside chat
	// and patrol spend. Recording happens regardless of whether the
	// response parses, so failed-but-billed calls are still visible.
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
			UseCase:       reportNarratorUseCase,
			InputTokens:   resp.InputTokens,
			OutputTokens:  resp.OutputTokens,
			TargetType:    in.ResourceType,
			TargetID:      in.ResourceID,
		})
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return reporting.Narrative{}, errors.New("provider returned empty narrative")
	}

	parsed, err := parseReportNarratorResponse(content)
	if err != nil {
		return reporting.Narrative{}, err
	}

	narrative := reporting.Narrative{
		Source:           reporting.NarrativeSourceAI,
		HealthStatus:     normalizeReportHealthStatus(parsed.HealthStatus),
		HealthMessage:    strings.TrimSpace(parsed.HealthMessage),
		ExecutiveSummary: strings.TrimSpace(parsed.ExecutiveSummary),
		PeriodComparison: strings.TrimSpace(parsed.PeriodComparison),
		Disclaimer:       "Narrative generated by Pulse Assistant. Verify against the data tables in this report.",
	}
	for _, b := range parsed.Observations {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		narrative.Observations = append(narrative.Observations, reporting.NarrativeBullet{
			Text:     text,
			Severity: normalizeBulletSeverity(b.Severity),
		})
	}
	for _, r := range parsed.Recommendations {
		r = strings.TrimSpace(r)
		if r != "" {
			narrative.Recommendations = append(narrative.Recommendations, r)
		}
	}
	if narrative.HealthStatus == "" || (len(narrative.Observations) == 0 && len(narrative.Recommendations) == 0) {
		return reporting.Narrative{}, errors.New("provider returned empty or invalid narrative")
	}
	return narrative, nil
}

func buildReportNarratorPayload(in reporting.NarrativeInput) reportNarratorPayload {
	payload := reportNarratorPayload{
		Title:        in.Title,
		ResourceType: in.ResourceType,
		ResourceID:   in.ResourceID,
		Period: reportNarratorPeriod{
			Start: in.Period.Start.UTC().Format("2006-01-02T15:04:05Z"),
			End:   in.Period.End.UTC().Format("2006-01-02T15:04:05Z"),
			Hours: int(in.Period.End.Sub(in.Period.Start).Hours()),
		},
		MetricStats: convertMetricStats(in.MetricStats),
	}

	if in.Resource != nil {
		payload.Resource = &reportNarratorResource{
			Name:        in.Resource.Name,
			DisplayName: in.Resource.DisplayName,
			Status:      in.Resource.Status,
			Node:        in.Resource.Node,
			CPUCores:    in.Resource.CPUCores,
		}
		if in.Resource.Uptime > 0 {
			payload.Resource.UptimeDays = in.Resource.Uptime / 86400
		}
		if in.Resource.MemoryTotal > 0 {
			payload.Resource.MemoryGB = float64(in.Resource.MemoryTotal) / (1024 * 1024 * 1024)
		}
	}

	if in.PriorPeriod != nil {
		payload.PriorPeriod = &reportNarratorPriorPeriod{
			Start:       in.PriorPeriod.Period.Start.UTC().Format("2006-01-02T15:04:05Z"),
			End:         in.PriorPeriod.Period.End.UTC().Format("2006-01-02T15:04:05Z"),
			MetricStats: convertMetricStats(in.PriorPeriod.MetricStats),
		}
	}

	for _, alert := range in.Alerts {
		payload.Alerts = append(payload.Alerts, reportNarratorAlert{
			Type:      alert.Type,
			Level:     alert.Level,
			Message:   alert.Message,
			Value:     alert.Value,
			Threshold: alert.Threshold,
			Resolved:  alert.ResolvedTime != nil,
		})
	}

	for _, st := range in.Storage {
		payload.Storage = append(payload.Storage, reportNarratorStorage{
			Name:      st.Name,
			Type:      st.Type,
			UsagePerc: st.UsagePerc,
			ZFSHealth: st.ZFSHealth,
		})
	}

	for _, disk := range in.Disks {
		payload.Disks = append(payload.Disks, reportNarratorDisk{
			Device:    disk.Device,
			Type:      disk.Type,
			Health:    disk.Health,
			WearLevel: disk.WearLevel,
		})
	}

	for _, f := range in.Findings {
		payload.Findings = append(payload.Findings, reportNarratorFindingEntry{
			Severity:       f.Severity,
			Category:       f.Category,
			Title:          f.Title,
			Description:    f.Description,
			Recommendation: f.Recommendation,
			Resolved:       f.Resolved,
		})
	}

	return payload
}

func convertMetricStats(stats map[string]reporting.MetricStats) map[string]reportNarratorMS {
	if len(stats) == 0 {
		return nil
	}
	out := make(map[string]reportNarratorMS, len(stats))
	for k, v := range stats {
		out[k] = reportNarratorMS{
			Min:     v.Min,
			Max:     v.Max,
			Avg:     v.Avg,
			Current: v.Current,
			Count:   v.Count,
		}
	}
	return out
}

// parseReportNarratorResponse strips an optional code fence and unmarshals
// the JSON envelope. Models occasionally wrap the response despite the
// "no markdown fences" instruction; tolerate it rather than fall through to
// the heuristic for a stylistic miss.
func parseReportNarratorResponse(raw string) (reportNarratorResponse, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		// drop optional language tag like "json"
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = trimmed[newline+1:]
		}
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	var parsed reportNarratorResponse
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return reportNarratorResponse{}, fmt.Errorf("decode narrative JSON: %w", err)
	}
	return parsed, nil
}

func normalizeReportHealthStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "HEALTHY":
		return "HEALTHY"
	case "WARNING":
		return "WARNING"
	case "CRITICAL":
		return "CRITICAL"
	default:
		return ""
	}
}

func normalizeBulletSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case reporting.NarrativeSeverityCritical, "high", "danger":
		return reporting.NarrativeSeverityCritical
	case reporting.NarrativeSeverityWarning, "medium":
		return reporting.NarrativeSeverityWarning
	case reporting.NarrativeSeverityInfo:
		return reporting.NarrativeSeverityInfo
	case reporting.NarrativeSeverityOK, "good", "healthy":
		return reporting.NarrativeSeverityOK
	default:
		return reporting.NarrativeSeverityInfo
	}
}
