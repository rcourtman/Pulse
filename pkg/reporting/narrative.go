package reporting

import (
	"context"
	"fmt"
	"time"
)

// NarrativeSource identifies who produced a Narrative.
const (
	NarrativeSourceHeuristic = "heuristic"
	NarrativeSourceAI        = "ai"
)

// NarrativeBulletSeverity classifies a single observation bullet.
const (
	NarrativeSeverityOK       = "ok"
	NarrativeSeverityInfo     = "info"
	NarrativeSeverityWarning  = "warning"
	NarrativeSeverityCritical = "critical"
)

// Narrative is the textual interpretation layer of a performance report.
// It is rendered into the executive summary section of the PDF and is
// produced either by the heuristic narrator (deterministic thresholds) or
// by an AI narrator that reads the same NarrativeInput.
type Narrative struct {
	Source           string            `json:"source"`                      // NarrativeSourceHeuristic or NarrativeSourceAI
	HealthStatus     string            `json:"health_status"`               // HEALTHY / WARNING / CRITICAL
	HealthMessage    string            `json:"health_message"`              // one-line summary shown next to HealthStatus
	ExecutiveSummary string            `json:"executive_summary,omitempty"` // optional prose (AI populates, heuristic leaves empty)
	Observations     []NarrativeBullet `json:"observations,omitempty"`      // ordered list rendered as bullets
	Recommendations  []string          `json:"recommendations,omitempty"`   // ordered list rendered as numbered actions
	PeriodComparison string            `json:"period_comparison,omitempty"` // optional prose summarising deltas vs PriorPeriod
	Disclaimer       string            `json:"disclaimer,omitempty"`        // optional footer (e.g. AI provenance note)
}

// NarrativeBullet is a single observation entry with a severity classification
// the renderer maps to a colour swatch. JSON tags use lowercase keys to
// match the shape the AI narrator's system prompt asks for and the chat
// tool consumers expect — without them the embedded type serialises with
// capital field names, which is inconsistent with the rest of the JSON
// surface and confuses downstream models that get back a different shape
// than the schema they were taught.
type NarrativeBullet struct {
	Text     string `json:"text"`
	Severity string `json:"severity"`
}

// NarrativeInput is the data passed to a Narrator. It is denormalised from
// ReportData so the AI implementation does not need to traverse internal
// structures.
type NarrativeInput struct {
	Title        string
	ResourceType string
	ResourceID   string
	GeneratedAt  time.Time
	Period       TimeRange
	PriorPeriod  *PriorPeriodInput
	Resource     *ResourceInfo
	MetricStats  map[string]MetricStats
	Alerts       []AlertInfo
	Storage      []StorageInfo
	Disks        []DiskInfo
	Backups      []BackupInfo
	Findings     []FindingSummary
}

// TimeRange is a half-open interval [Start, End).
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// PriorPeriodInput captures the comparable prior-window aggregates the
// narrator can use to describe deltas. Empty MetricStats means no prior data
// was available.
type PriorPeriodInput struct {
	Period      TimeRange
	MetricStats map[string]MetricStats
	AlertCount  int
}

// FindingSummary is the report-facing projection of a Patrol finding. It is
// defined here (rather than imported from internal/ai) so the reporting
// package stays free of AI-internal dependencies.
type FindingSummary struct {
	ID             string
	Severity       string // critical, high, medium, low, info
	Category       string
	Title          string
	Description    string
	Recommendation string
	DetectedAt     time.Time
	Resolved       bool
}

// Narrator produces a Narrative from a NarrativeInput. Implementations must
// be safe for concurrent use and must respect ctx cancellation.
type Narrator interface {
	Narrate(ctx context.Context, in NarrativeInput) (Narrative, error)
}

// FindingsProvider returns Patrol findings overlapping the given window for
// a single resource. An empty slice means no findings in scope.
type FindingsProvider interface {
	FindingsForReport(ctx context.Context, resourceID string, start, end time.Time) []FindingSummary
}

// narrate is the helper used by the engine: it tries the supplied narrator
// (typically AI), and on nil/error falls back to the heuristic narrator so a
// report is always produced. The returned Narrative.Source records which
// path actually ran.
func narrate(ctx context.Context, n Narrator, in NarrativeInput) Narrative {
	if n != nil {
		out, err := n.Narrate(ctx, in)
		if err == nil {
			if out.Source == "" {
				out.Source = NarrativeSourceAI
			}
			return out
		}
	}
	heuristic := HeuristicNarrator{}
	out, _ := heuristic.Narrate(ctx, in)
	out.Source = NarrativeSourceHeuristic
	return out
}

// HeuristicNarrator is the deterministic fallback narrator. It encodes the
// same observation and recommendation rules as the original PDF generator,
// lifted here so both renderers and the AI fallback path share one source of
// truth.
type HeuristicNarrator struct{}

// Narrate implements Narrator. It never returns an error.
func (HeuristicNarrator) Narrate(_ context.Context, in NarrativeInput) (Narrative, error) {
	return Narrative{
		Source:          NarrativeSourceHeuristic,
		HealthStatus:    heuristicHealthStatus(in),
		HealthMessage:   heuristicHealthMessage(in),
		Observations:    heuristicObservations(in),
		Recommendations: heuristicRecommendations(in),
	}, nil
}

func heuristicHealthStatus(in NarrativeInput) string {
	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range in.Alerts {
		if alert.ResolvedTime != nil {
			continue
		}
		if alert.Level == "critical" {
			criticalAlerts++
		} else {
			warningAlerts++
		}
	}
	switch {
	case criticalAlerts > 0:
		return "CRITICAL"
	case warningAlerts > 0:
		return "WARNING"
	default:
		return "HEALTHY"
	}
}

func heuristicHealthMessage(in NarrativeInput) string {
	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range in.Alerts {
		if alert.ResolvedTime != nil {
			continue
		}
		if alert.Level == "critical" {
			criticalAlerts++
		} else {
			warningAlerts++
		}
	}
	switch {
	case criticalAlerts == 1:
		return "1 critical issue requires immediate attention"
	case criticalAlerts > 1:
		return fmt.Sprintf("%d critical issues require immediate attention", criticalAlerts)
	case warningAlerts == 1:
		return "1 warning detected - review recommended"
	case warningAlerts > 1:
		return fmt.Sprintf("%d warnings detected - review recommended", warningAlerts)
	default:
		return "All systems operating normally"
	}
}

// heuristicObservations mirrors the original generateObservations rules.
func heuristicObservations(in NarrativeInput) []NarrativeBullet {
	if len(in.MetricStats) == 0 {
		return []NarrativeBullet{{
			Text:     "Insufficient data for analysis",
			Severity: NarrativeSeverityInfo,
		}}
	}

	var obs []NarrativeBullet

	if stats, ok := in.MetricStats["cpu"]; ok {
		switch {
		case stats.Max > 90:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("CPU peaked at %.1f%% - potential capacity constraint", stats.Max),
				Severity: NarrativeSeverityCritical,
			})
		case stats.Avg < 20:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("CPU averaging %.1f%% - resource is underutilized", stats.Avg),
				Severity: NarrativeSeverityOK,
			})
		default:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("CPU usage normal (avg %.1f%%, max %.1f%%)", stats.Avg, stats.Max),
				Severity: NarrativeSeverityOK,
			})
		}
	}

	if stats, ok := in.MetricStats["memory"]; ok {
		switch {
		case stats.Avg > 85:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Memory consistently high at %.1f%% avg - consider scaling", stats.Avg),
				Severity: NarrativeSeverityCritical,
			})
		case stats.Max > 95:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Memory peaked at %.1f%% - near capacity", stats.Max),
				Severity: NarrativeSeverityWarning,
			})
		default:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Memory usage healthy (avg %.1f%%)", stats.Avg),
				Severity: NarrativeSeverityOK,
			})
		}
	}

	diskKey := "disk"
	if _, hasDisk := in.MetricStats["disk"]; !hasDisk {
		if _, hasUsage := in.MetricStats["usage"]; hasUsage {
			diskKey = "usage"
		}
	}
	if stats, ok := in.MetricStats[diskKey]; ok {
		switch {
		case stats.Avg > 85:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Disk at %.1f%% - plan capacity expansion", stats.Avg),
				Severity: NarrativeSeverityCritical,
			})
		case stats.Avg > 70:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Disk at %.1f%% - monitor growth trend", stats.Avg),
				Severity: NarrativeSeverityWarning,
			})
		default:
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("Disk usage acceptable at %.1f%%", stats.Avg),
				Severity: NarrativeSeverityOK,
			})
		}
	}

	resolved := 0
	for _, alert := range in.Alerts {
		if alert.ResolvedTime != nil {
			resolved++
		}
	}
	if resolved > 0 {
		obs = append(obs, NarrativeBullet{
			Text:     fmt.Sprintf("%d alerts were triggered and resolved during this period", resolved),
			Severity: NarrativeSeverityInfo,
		})
	}

	for _, disk := range in.Disks {
		if disk.WearLevel > 0 && disk.WearLevel <= 10 {
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("CRITICAL: Disk %s at %d%% life remaining", disk.Device, disk.WearLevel),
				Severity: NarrativeSeverityCritical,
			})
		}
		if disk.Health == "FAILED" {
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("CRITICAL: Disk %s SMART health check failed", disk.Device),
				Severity: NarrativeSeverityCritical,
			})
		}
	}

	if in.Resource != nil && in.Resource.Uptime > 0 {
		uptimeDays := in.Resource.Uptime / 86400
		if uptimeDays > 90 {
			obs = append(obs, NarrativeBullet{
				Text:     fmt.Sprintf("System uptime is %d days - schedule maintenance window", uptimeDays),
				Severity: NarrativeSeverityWarning,
			})
		}
	}

	return obs
}

// heuristicRecommendations mirrors the original generateRecommendations rules.
func heuristicRecommendations(in NarrativeInput) []string {
	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range in.Alerts {
		if alert.ResolvedTime != nil {
			continue
		}
		if alert.Level == "critical" {
			criticalAlerts++
		} else {
			warningAlerts++
		}
	}
	_ = warningAlerts // retained for future shaping; matches original signature

	var recs []string

	for _, disk := range in.Disks {
		if disk.WearLevel > 0 && disk.WearLevel <= 10 {
			recs = append(recs, fmt.Sprintf("Replace disk %s immediately (only %d%% life remaining)", disk.Device, disk.WearLevel))
		} else if disk.WearLevel > 0 && disk.WearLevel <= 30 {
			recs = append(recs, fmt.Sprintf("Schedule replacement for disk %s within 3-6 months (%d%% life remaining)", disk.Device, disk.WearLevel))
		}
		if disk.Health == "FAILED" {
			recs = append(recs, fmt.Sprintf("Investigate and replace disk %s - SMART health check failed", disk.Device))
		}
	}

	if criticalAlerts > 0 {
		recs = append(recs, "Investigate and resolve critical alerts immediately")
	}

	if stats, ok := in.MetricStats["memory"]; ok {
		if stats.Avg > 85 {
			recs = append(recs, "Consider adding memory or optimizing memory-intensive workloads")
		}
	}
	if stats, ok := in.MetricStats["cpu"]; ok {
		if stats.Max > 90 {
			recs = append(recs, "Review CPU-intensive processes during peak usage periods")
		}
	}

	diskKey := "disk"
	if _, ok := in.MetricStats["disk"]; !ok {
		diskKey = "usage"
	}
	if stats, ok := in.MetricStats[diskKey]; ok {
		if stats.Avg > 85 {
			recs = append(recs, "Clean up disk space or expand storage capacity")
		}
	}

	for _, storage := range in.Storage {
		if storage.UsagePerc >= 90 {
			recs = append(recs, fmt.Sprintf("Expand storage pool '%s' (currently at %.0f%% capacity)", storage.Name, storage.UsagePerc))
		}
	}

	if in.Resource != nil && in.Resource.Uptime > 0 {
		uptimeDays := in.Resource.Uptime / 86400
		if uptimeDays > 90 {
			recs = append(recs, "Schedule maintenance window to apply pending updates and reboot")
		}
	}

	if stats, ok := in.MetricStats["cpu"]; ok {
		if stats.Avg < 10 && len(recs) == 0 {
			recs = append(recs, "System is underutilized - consider consolidating workloads")
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "No immediate action required - continue monitoring")
	}

	return recs
}

// narrativeInputFromReport builds a NarrativeInput from a fully-populated
// ReportData plus the optional prior-period summary. Findings are passed in
// separately because they are not part of the metric snapshot.
func narrativeInputFromReport(data *ReportData, prior *PriorPeriodInput, findings []FindingSummary) NarrativeInput {
	if data == nil {
		return NarrativeInput{}
	}
	return NarrativeInput{
		Title:        data.Title,
		ResourceType: data.ResourceType,
		ResourceID:   data.ResourceID,
		GeneratedAt:  data.GeneratedAt,
		Period:       TimeRange{Start: data.Start, End: data.End},
		PriorPeriod:  prior,
		Resource:     data.Resource,
		MetricStats:  data.Summary.ByMetric,
		Alerts:       data.Alerts,
		Storage:      data.Storage,
		Disks:        data.Disks,
		Backups:      data.Backups,
		Findings:     findings,
	}
}
