package reporting

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// narrativeTimeout caps how long the engine waits for an external narrator
// (typically an LLM-backed implementation). On timeout the engine falls
// back to the heuristic narrator so a report is always returned.
const narrativeTimeout = 30 * time.Second

// ReportEngine implements the reporting.Engine interface with
// full CSV and PDF generation capabilities.
type ReportEngine struct {
	metricsStore       *metrics.Store
	metricsStoreGetter func() *metrics.Store
	csvGen             *CSVGenerator
	pdfGen             *PDFGenerator
}

// EngineConfig holds configuration for the report engine.
type EngineConfig struct {
	// MetricsStore is a direct reference to the metrics store.
	// Use MetricsStoreGetter instead when the store may be replaced
	// (e.g., after monitor reloads).
	MetricsStore *metrics.Store

	// MetricsStoreGetter dynamically resolves the current metrics store.
	// When set, this is used instead of MetricsStore, ensuring queries
	// always target the active store even after monitor reloads.
	MetricsStoreGetter func() *metrics.Store
}

// NewReportEngine creates a new reporting engine.
func NewReportEngine(cfg EngineConfig) *ReportEngine {
	return &ReportEngine{
		metricsStore:       cfg.MetricsStore,
		metricsStoreGetter: cfg.MetricsStoreGetter,
		csvGen:             NewCSVGenerator(),
		pdfGen:             NewPDFGenerator(),
	}
}

// getMetricsStore returns the current metrics store, preferring the dynamic
// getter over the static reference.
func (e *ReportEngine) getMetricsStore() *metrics.Store {
	if e.metricsStoreGetter != nil {
		return e.metricsStoreGetter()
	}
	return e.metricsStore
}

// CanonicalResourceType normalizes report resource type inputs to canonical v6 names.
// Returns an empty string when the type is unsupported.
func CanonicalResourceType(resourceType string) string {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "node":
		return "node"
	case "vm":
		return "vm"
	case "system-container":
		return "system-container"
	case "oci-container":
		return "oci-container"
	case "app-container":
		return "app-container"
	case "docker-host":
		return "docker-host"
	case "storage":
		return "storage"
	case "agent":
		return "agent"
	case "k8s":
		return "k8s"
	case "disk":
		return "disk"
	case "pbs":
		return "pbs"
	case "pmg":
		return "pmg"
	case "pod":
		return "pod"
	case "datastore":
		return "datastore"
	case "pool":
		return "pool"
	case "dataset":
		return "dataset"
	case "network-endpoint":
		return "network-endpoint"
	default:
		return ""
	}
}

func metricsStoreResourceTypes(canonicalType string) []string {
	switch canonicalType {
	case "system-container", "oci-container":
		return []string{"container"}
	case "app-container":
		return []string{"dockerContainer", "docker"}
	case "docker-host":
		return []string{"dockerHost"}
	case "":
		return nil
	default:
		return []string{canonicalType}
	}
}

func mergeMetricPointsByTimestamp(existing, incoming []metrics.MetricPoint) []metrics.MetricPoint {
	if len(incoming) == 0 {
		return existing
	}
	if len(existing) == 0 {
		out := make([]metrics.MetricPoint, len(incoming))
		copy(out, incoming)
		sort.Slice(out, func(i, j int) bool {
			return out[i].Timestamp.Before(out[j].Timestamp)
		})
		return out
	}

	seen := make(map[int64]struct{}, len(existing)+len(incoming))
	out := make([]metrics.MetricPoint, len(existing))
	copy(out, existing)
	for _, point := range existing {
		seen[point.Timestamp.Unix()] = struct{}{}
	}

	for _, point := range incoming {
		key := point.Timestamp.Unix()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, point)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func mergeMetricMapsByTimestamp(dst, src map[string][]metrics.MetricPoint) {
	for metricType, incomingPoints := range src {
		dst[metricType] = mergeMetricPointsByTimestamp(dst[metricType], incomingPoints)
	}
}

// Generate creates a report in the specified format.
func (e *ReportEngine) Generate(req MetricReportRequest) (data []byte, contentType string, err error) {
	if e.getMetricsStore() == nil {
		return nil, "", fmt.Errorf("metrics store not initialized")
	}

	// Query metrics data
	reportData, err := e.queryMetrics(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query metrics: %w", err)
	}

	// Build narrative interpretation. If req.Narrator is supplied (typically
	// an LLM-backed implementation) it is invoked with a bounded timeout;
	// nil/error/timeout falls back to the heuristic narrator. The prior
	// period is queried with the same window length offset back so deltas
	// can be expressed when the narrator wants them.
	e.attachNarrative(reportData, req)

	log.Debug().
		Str("resourceType", reportData.ResourceType).
		Str("resourceID", req.ResourceID).
		Str("format", string(req.Format)).
		Int("dataPoints", reportData.TotalPoints).
		Str("narrativeSource", narrativeSource(reportData)).
		Msg("Generating report")

	switch req.Format {
	case FormatCSV:
		data, err = e.csvGen.Generate(reportData)
		if err != nil {
			return nil, "", fmt.Errorf("CSV generation failed: %w", err)
		}
		contentType = "text/csv"

	case FormatPDF:
		data, err = e.pdfGen.Generate(reportData)
		if err != nil {
			return nil, "", fmt.Errorf("PDF generation failed: %w", err)
		}
		contentType = "application/pdf"

	default:
		return nil, "", fmt.Errorf("unsupported format: %s", req.Format)
	}

	return data, contentType, nil
}

// ReportData holds the data for report generation.
type ReportData struct {
	Title        string
	ResourceType string
	ResourceID   string
	Start        time.Time
	End          time.Time
	GeneratedAt  time.Time
	Metrics      map[string][]MetricDataPoint
	TotalPoints  int
	Summary      MetricSummary

	// Enrichment data (optional, for richer PDF reports)
	Resource *ResourceInfo
	Alerts   []AlertInfo
	Backups  []BackupInfo
	Storage  []StorageInfo
	Disks    []DiskInfo

	// Interpretation layer (optional). When set, the renderer prefers these
	// over recomputing heuristic observations/recommendations inline. The
	// engine populates Narrative when the request supplies a Narrator.
	Narrative   *Narrative
	PriorPeriod *PriorPeriodInput
	Findings    []FindingSummary
}

// MetricDataPoint represents a single data point in a report.
type MetricDataPoint struct {
	Timestamp time.Time
	Value     float64
	Min       float64
	Max       float64
}

// MetricSummary holds aggregated statistics for a report.
type MetricSummary struct {
	ByMetric map[string]MetricStats
}

// MetricStats holds statistics for a single metric type.
type MetricStats struct {
	MetricType string
	Count      int
	Min        float64
	Max        float64
	Avg        float64
	Current    float64
}

// queryMetrics fetches metrics from the store and prepares report data.
func (e *ReportEngine) queryMetrics(req MetricReportRequest) (*ReportData, error) {
	canonicalType := CanonicalResourceType(req.ResourceType)
	if canonicalType == "" {
		canonicalType = strings.TrimSpace(req.ResourceType)
	}

	data := &ReportData{
		Title:        req.Title,
		ResourceType: canonicalType,
		ResourceID:   req.ResourceID,
		Start:        req.Start,
		End:          req.End,
		GeneratedAt:  time.Now(),
		Metrics:      make(map[string][]MetricDataPoint),
		Summary: MetricSummary{
			ByMetric: make(map[string]MetricStats),
		},
	}

	if data.Title == "" {
		data.Title = fmt.Sprintf("%s Report: %s", GetResourceTypeDisplayName(data.ResourceType), req.ResourceID)
	}

	// Copy enrichment data from request
	data.Resource = req.Resource
	data.Alerts = req.Alerts
	data.Backups = req.Backups
	data.Storage = req.Storage
	data.Disks = req.Disks

	store := e.getMetricsStore()
	storeTypes := metricsStoreResourceTypes(canonicalType)
	if len(storeTypes) == 0 {
		storeTypes = []string{canonicalType}
	}

	var metricsMap map[string][]metrics.MetricPoint

	if req.MetricType != "" {
		// Query specific metric, merging across storage aliases during migrations.
		var points []metrics.MetricPoint
		for _, storeType := range storeTypes {
			aliasPoints, queryErr := store.Query(storeType, req.ResourceID, req.MetricType, req.Start, req.End, 0)
			if queryErr != nil {
				return nil, queryErr
			}
			points = mergeMetricPointsByTimestamp(points, aliasPoints)
		}
		metricsMap = map[string][]metrics.MetricPoint{
			req.MetricType: points,
		}
	} else {
		// Query all metrics for the resource, merging across storage aliases.
		metricsMap = make(map[string][]metrics.MetricPoint)
		for _, storeType := range storeTypes {
			aliasMap, queryErr := store.QueryAll(storeType, req.ResourceID, req.Start, req.End, 0)
			if queryErr != nil {
				return nil, queryErr
			}
			mergeMetricMapsByTimestamp(metricsMap, aliasMap)
		}
	}

	if len(metricsMap) == 0 {
		log.Warn().
			Str("resourceType", data.ResourceType).
			Str("resourceID", req.ResourceID).
			Str("metricType", req.MetricType).
			Time("start", req.Start).
			Time("end", req.End).
			Msg("Report query returned no metrics — verify resource ID matches stored metrics and time range contains data")
	}

	// Convert to report format and calculate statistics
	for metricType, points := range metricsMap {
		if len(points) == 0 {
			continue
		}

		dataPoints := make([]MetricDataPoint, len(points))
		var sum float64
		stats := MetricStats{
			MetricType: metricType,
			Count:      len(points),
			Min:        points[0].Value,
			Max:        points[0].Value,
		}

		for i, p := range points {
			dataPoints[i] = MetricDataPoint{
				Timestamp: p.Timestamp,
				Value:     p.Value,
				Min:       p.Min,
				Max:       p.Max,
			}

			sum += p.Value
			if p.Value < stats.Min {
				stats.Min = p.Value
			}
			if p.Value > stats.Max {
				stats.Max = p.Value
			}
		}

		stats.Avg = sum / float64(len(points))
		stats.Current = points[len(points)-1].Value
		data.TotalPoints += len(points)
		data.Metrics[metricType] = dataPoints
		data.Summary.ByMetric[metricType] = stats
	}

	return data, nil
}

// GenerateMulti creates a multi-resource report in the specified format.
func (e *ReportEngine) GenerateMulti(req MultiReportRequest) (data []byte, contentType string, err error) {
	if e.getMetricsStore() == nil {
		return nil, "", fmt.Errorf("metrics store not initialized")
	}

	multiData := &MultiReportData{
		Title:       req.Title,
		Start:       req.Start,
		End:         req.End,
		GeneratedAt: time.Now(),
	}

	if multiData.Title == "" {
		multiData.Title = "Fleet Performance Report"
	}

	// Query metrics for each resource. When the multi-report request
	// supplies a per-resource Narrator or FindingsProvider, propagate
	// them so each per-resource report carries its own narrative.
	var successCount int
	for _, resReq := range req.Resources {
		resReq.Start = req.Start
		resReq.End = req.End
		resReq.MetricType = req.MetricType
		if resReq.Narrator == nil {
			resReq.Narrator = req.Narrator
		}
		if resReq.FindingsProvider == nil {
			resReq.FindingsProvider = req.FindingsProvider
		}

		reportData, queryErr := e.queryMetrics(resReq)
		if queryErr != nil {
			log.Warn().
				Str("resourceType", resReq.ResourceType).
				Str("resourceID", resReq.ResourceID).
				Err(queryErr).
				Msg("Skipping resource in multi-report: failed to query metrics")
			continue
		}

		// Per-resource narrative is intentionally skipped on the multi
		// path: a fleet PDF aggregates 50 resources, and running an AI
		// call per resource would be cost-hostile and slow. The single
		// fleet-level call carries the summary instead.
		_ = resReq.Narrator
		_ = resReq.FindingsProvider

		multiData.Resources = append(multiData.Resources, reportData)
		multiData.TotalPoints += reportData.TotalPoints
		successCount++
	}

	if successCount == 0 {
		return nil, "", fmt.Errorf("all resources failed to query metrics")
	}

	// Build fleet-level narrative. If req.FleetNarrator is supplied
	// (typically AI-backed) it is invoked with a bounded timeout;
	// nil/error/timeout falls back to the heuristic fleet narrator so
	// the fleet PDF always has narrative content.
	e.attachFleetNarrative(multiData, req)

	log.Debug().
		Int("resources", successCount).
		Int("skipped", len(req.Resources)-successCount).
		Str("format", string(req.Format)).
		Int("totalPoints", multiData.TotalPoints).
		Msg("Generating multi-resource report")

	switch req.Format {
	case FormatCSV:
		data, err = e.csvGen.GenerateMulti(multiData)
		if err != nil {
			return nil, "", fmt.Errorf("CSV generation failed: %w", err)
		}
		contentType = "text/csv"

	case FormatPDF:
		data, err = e.pdfGen.GenerateMulti(multiData)
		if err != nil {
			return nil, "", fmt.Errorf("PDF generation failed: %w", err)
		}
		contentType = "application/pdf"

	default:
		return nil, "", fmt.Errorf("unsupported format: %s", req.Format)
	}

	return data, contentType, nil
}

// attachNarrative populates reportData.PriorPeriod, Findings and Narrative
// using req.Narrator (when supplied). Always populates Narrative — falling
// back to the heuristic narrator on nil/error/timeout — so the renderer
// has a single source of truth.
func (e *ReportEngine) attachNarrative(reportData *ReportData, req MetricReportRequest) {
	if reportData == nil {
		return
	}

	prior := e.priorPeriodFor(req)
	reportData.PriorPeriod = prior

	var findings []FindingSummary
	if req.FindingsProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), narrativeTimeout)
		findings = req.FindingsProvider.FindingsForReport(ctx, req.ResourceID, req.Start, req.End)
		cancel()
	}
	reportData.Findings = findings

	input := narrativeInputFromReport(reportData, prior, findings)

	if req.Narrator == nil {
		out, _ := HeuristicNarrator{}.Narrate(context.Background(), input)
		out.Source = NarrativeSourceHeuristic
		reportData.Narrative = &out
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), narrativeTimeout)
	defer cancel()
	out := narrate(ctx, req.Narrator, input)
	reportData.Narrative = &out
}

// priorPeriodFor returns aggregate stats for the comparable prior window
// (same length, ending at req.Start). Returns nil when the window is empty
// or the metrics store has no data for the prior range.
func (e *ReportEngine) priorPeriodFor(req MetricReportRequest) *PriorPeriodInput {
	store := e.getMetricsStore()
	if store == nil {
		return nil
	}
	duration := req.End.Sub(req.Start)
	if duration <= 0 {
		return nil
	}
	priorEnd := req.Start
	priorStart := priorEnd.Add(-duration)
	priorReq := req
	priorReq.Start = priorStart
	priorReq.End = priorEnd
	priorReq.Title = ""
	priorReq.Resource = nil
	priorReq.Alerts = nil
	priorReq.Backups = nil
	priorReq.Storage = nil
	priorReq.Disks = nil
	priorReq.Narrator = nil
	priorReq.FindingsProvider = nil

	priorData, err := e.queryMetrics(priorReq)
	if err != nil || priorData == nil || len(priorData.Summary.ByMetric) == 0 {
		return nil
	}
	return &PriorPeriodInput{
		Period:      TimeRange{Start: priorStart, End: priorEnd},
		MetricStats: priorData.Summary.ByMetric,
	}
}

func narrativeSource(data *ReportData) string {
	if data == nil || data.Narrative == nil {
		return ""
	}
	return data.Narrative.Source
}

// attachFleetNarrative populates multiData.FleetNarrative using
// req.FleetNarrator (when supplied). Always populates so the renderer
// has a single source of truth for the fleet summary section.
func (e *ReportEngine) attachFleetNarrative(multiData *MultiReportData, req MultiReportRequest) {
	if multiData == nil {
		return
	}
	input := buildFleetNarrativeInput(multiData)

	if req.FleetNarrator == nil {
		out, _ := HeuristicFleetNarrator{}.NarrateFleet(context.Background(), input)
		out.Source = NarrativeSourceHeuristic
		multiData.FleetNarrative = &out
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), narrativeTimeout)
	defer cancel()
	out := narrateFleet(ctx, req.FleetNarrator, input)
	multiData.FleetNarrative = &out
}

// NarrativeFor returns the structured narrative for a single-resource
// report request without rendering PDF or CSV. This is the entry point
// non-rendering callers (e.g. the Assistant chat session) use when they
// want the same retrospective synthesis the report PDF carries but in a
// form they can present in chat. Same query path, same narrator
// resolution, same fail-closed-to-heuristic fallback as Generate; just
// no fpdf/csv output stage.
func (e *ReportEngine) NarrativeFor(req MetricReportRequest) (*Narrative, error) {
	if e.getMetricsStore() == nil {
		return nil, fmt.Errorf("metrics store not initialized")
	}
	reportData, err := e.queryMetrics(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	e.attachNarrative(reportData, req)
	if reportData.Narrative == nil {
		return nil, fmt.Errorf("narrative generation produced no result")
	}
	return reportData.Narrative, nil
}

// FleetNarrativeFor returns the structured fleet narrative for a
// multi-resource report request without rendering. Same shape as
// NarrativeFor for the single-resource path. Returns an error if no
// resource queries succeed; otherwise returns the narrative even if a
// subset of resources failed (the narrator is given what loaded, and
// the failed-resource list is logged at warn level by queryMetrics).
func (e *ReportEngine) FleetNarrativeFor(req MultiReportRequest) (*FleetNarrative, error) {
	if e.getMetricsStore() == nil {
		return nil, fmt.Errorf("metrics store not initialized")
	}
	multiData := &MultiReportData{
		Title:       req.Title,
		Start:       req.Start,
		End:         req.End,
		GeneratedAt: time.Now(),
	}
	if multiData.Title == "" {
		multiData.Title = "Fleet Performance Report"
	}
	var successCount int
	for _, resReq := range req.Resources {
		resReq.Start = req.Start
		resReq.End = req.End
		resReq.MetricType = req.MetricType
		if resReq.Narrator == nil {
			resReq.Narrator = req.Narrator
		}
		if resReq.FindingsProvider == nil {
			resReq.FindingsProvider = req.FindingsProvider
		}
		reportData, queryErr := e.queryMetrics(resReq)
		if queryErr != nil {
			log.Warn().
				Str("resourceType", resReq.ResourceType).
				Str("resourceID", resReq.ResourceID).
				Err(queryErr).
				Msg("Skipping resource in fleet narrative: failed to query metrics")
			continue
		}
		multiData.Resources = append(multiData.Resources, reportData)
		multiData.TotalPoints += reportData.TotalPoints
		successCount++
	}
	if successCount == 0 {
		return nil, fmt.Errorf("all resources failed to query metrics")
	}
	e.attachFleetNarrative(multiData, req)
	if multiData.FleetNarrative == nil {
		return nil, fmt.Errorf("fleet narrative generation produced no result")
	}
	return multiData.FleetNarrative, nil
}

// GetResourceTypeDisplayName returns a human-readable name for resource types.
func GetResourceTypeDisplayName(resourceType string) string {
	switch CanonicalResourceType(resourceType) {
	case "node":
		return "Node"
	case "vm":
		return "Virtual Machine"
	case "system-container":
		return "System Container"
	case "oci-container":
		return "OCI Container"
	case "app-container":
		return "App Container"
	case "docker-host":
		return "Container Runtime"
	case "storage":
		return "Storage"
	case "agent":
		return "Agent"
	case "k8s":
		return "Kubernetes"
	case "disk":
		return "Disk"
	case "pbs":
		return "PBS"
	case "pmg":
		return "PMG"
	case "pod":
		return "Pod"
	case "datastore":
		return "Datastore"
	case "pool":
		return "Pool"
	case "dataset":
		return "Dataset"
	default:
		return resourceType
	}
}

// GetMetricTypeDisplayName returns a human-readable name for metric types.
func GetMetricTypeDisplayName(metricType string) string {
	switch metricType {
	case "cpu":
		return "CPU Usage"
	case "memory":
		return "Memory Usage"
	case "disk":
		return "Disk Usage"
	case "usage":
		return "Storage Usage"
	case "used":
		return "Used Space"
	case "total":
		return "Total Space"
	case "avail":
		return "Available Space"
	default:
		return metricType
	}
}

// GetMetricUnit returns the unit for a metric type.
func GetMetricUnit(metricType string) string {
	switch metricType {
	case "cpu", "memory", "disk", "usage":
		return "%"
	case "used", "total", "avail":
		return "bytes"
	default:
		return ""
	}
}
