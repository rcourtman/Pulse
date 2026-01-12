package reporting

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog/log"
)

// ReportEngine implements the reporting.Engine interface with
// full CSV and PDF generation capabilities.
type ReportEngine struct {
	metricsStore *metrics.Store
	csvGen       *CSVGenerator
	pdfGen       *PDFGenerator
}

// EngineConfig holds configuration for the report engine.
type EngineConfig struct {
	MetricsStore *metrics.Store
}

// NewReportEngine creates a new reporting engine.
func NewReportEngine(cfg EngineConfig) *ReportEngine {
	return &ReportEngine{
		metricsStore: cfg.MetricsStore,
		csvGen:       NewCSVGenerator(),
		pdfGen:       NewPDFGenerator(),
	}
}

// Generate creates a report in the specified format.
func (e *ReportEngine) Generate(req MetricReportRequest) (data []byte, contentType string, err error) {
	if e.metricsStore == nil {
		return nil, "", fmt.Errorf("metrics store not initialized")
	}

	// Query metrics data
	reportData, err := e.queryMetrics(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query metrics: %w", err)
	}

	log.Debug().
		Str("resourceType", req.ResourceType).
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
	data := &ReportData{
		Title:        req.Title,
		ResourceType: req.ResourceType,
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
		data.Title = fmt.Sprintf("%s Report: %s", req.ResourceType, req.ResourceID)
	}

	var metricsMap map[string][]metrics.MetricPoint
	var err error

	if req.MetricType != "" {
		// Query specific metric
		points, queryErr := e.metricsStore.Query(req.ResourceType, req.ResourceID, req.MetricType, req.Start, req.End)
		if queryErr != nil {
			return nil, queryErr
		}
		metricsMap = map[string][]metrics.MetricPoint{
			req.MetricType: points,
		}
	} else {
		// Query all metrics for the resource
		metricsMap, err = e.metricsStore.QueryAll(req.ResourceType, req.ResourceID, req.Start, req.End)
		if err != nil {
			return nil, err
		}
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

// GetResourceTypeDisplayName returns a human-readable name for resource types.
func GetResourceTypeDisplayName(resourceType string) string {
	switch resourceType {
	case "node":
		return "Node"
	case "vm":
		return "Virtual Machine"
	case "container":
		return "LXC Container"
	case "dockerHost":
		return "Docker Host"
	case "dockerContainer":
		return "Docker Container"
	case "storage":
		return "Storage"
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
