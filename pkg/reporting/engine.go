package reporting

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog/log"
)

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
	case "vm", "guest":
		return "vm"
	case "system-container", "system_container", "container", "lxc":
		return "system-container"
	case "oci-container", "oci_container":
		return "oci-container"
	case "app-container", "app_container", "docker", "docker-container", "docker_container", "dockercontainer":
		return "app-container"
	case "docker-host", "docker_host", "dockerhost":
		return "docker-host"
	case "storage":
		return "storage"
	case "agent":
		return "agent"
	case "k8s", "k8s-node", "k8s-cluster", "cluster":
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

	log.Debug().
		Str("resourceType", reportData.ResourceType).
		Str("resourceID", req.ResourceID).
		Str("format", string(req.Format)).
		Int("dataPoints", reportData.TotalPoints).
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

	// Query metrics for each resource
	var successCount int
	for _, resReq := range req.Resources {
		resReq.Start = req.Start
		resReq.End = req.End
		resReq.MetricType = req.MetricType

		reportData, queryErr := e.queryMetrics(resReq)
		if queryErr != nil {
			log.Warn().
				Str("resourceType", resReq.ResourceType).
				Str("resourceID", resReq.ResourceID).
				Err(queryErr).
				Msg("Skipping resource in multi-report: failed to query metrics")
			continue
		}

		multiData.Resources = append(multiData.Resources, reportData)
		multiData.TotalPoints += reportData.TotalPoints
		successCount++
	}

	if successCount == 0 {
		return nil, "", fmt.Errorf("all resources failed to query metrics")
	}

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
