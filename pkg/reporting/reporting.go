package reporting

import (
	"time"
)

// ReportFormat represents the output format of a report
type ReportFormat string

const (
	FormatCSV ReportFormat = "csv"
	FormatPDF ReportFormat = "pdf"
)

// MetricReportRequest defines the parameters for generating a report
type MetricReportRequest struct {
	ResourceType string
	ResourceID   string
	MetricType   string // Optional, if empty all metrics for the resource are included
	Start        time.Time
	End          time.Time
	Format       ReportFormat
	Title        string
}

// Engine defines the interface for report generation.
// This allows the enterprise version to provide PDF/CSV generation.
type Engine interface {
	Generate(req MetricReportRequest) (data []byte, contentType string, err error)
}

var (
	globalEngine Engine
)

// SetEngine sets the global report engine.
func SetEngine(e Engine) {
	globalEngine = e
}

// GetEngine returns the current global report engine.
func GetEngine() Engine {
	return globalEngine
}
