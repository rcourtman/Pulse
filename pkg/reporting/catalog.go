package reporting

import (
	"fmt"
	"strings"
	"time"
)

// ReportingFormatDefinition describes one supported output format for an
// operator-facing reporting surface.
type ReportingFormatDefinition struct {
	Value ReportFormat `json:"value"`
	Label string       `json:"label"`
}

// ReportingRangeDefinition describes one supported time window for historical
// performance reporting.
type ReportingRangeDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	WindowHours int    `json:"windowHours"`
}

// PerformanceReportDefinition describes the canonical performance reporting
// surface exposed to operators.
type PerformanceReportDefinition struct {
	ID                     string                      `json:"id"`
	Title                  string                      `json:"title"`
	Description            string                      `json:"description"`
	SingleResourceEndpoint string                      `json:"singleResourceEndpoint"`
	MultiResourceEndpoint  string                      `json:"multiResourceEndpoint"`
	SingleFilenamePrefix   string                      `json:"singleFilenamePrefix"`
	MultiFilenamePrefix    string                      `json:"multiFilenamePrefix"`
	Formats                []ReportingFormatDefinition `json:"formats"`
	DefaultFormat          ReportFormat                `json:"defaultFormat"`
	Ranges                 []ReportingRangeDefinition  `json:"ranges"`
	DefaultRange           string                      `json:"defaultRange"`
	MultiResourceMax       int                         `json:"multiResourceMax"`
	SupportsMetricFilter   bool                        `json:"supportsMetricFilter"`
	SupportsCustomTitle    bool                        `json:"supportsCustomTitle"`
}

// ReportingCatalog describes the backend-owned admin reporting surface for the
// Pulse settings UI.
type ReportingLockedStateDefinition struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ReportingGuidanceDefinition struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ReportingCatalog struct {
	ID                string                         `json:"id"`
	Title             string                         `json:"title"`
	Description       string                         `json:"description"`
	LockedState       ReportingLockedStateDefinition `json:"lockedState"`
	Guidance          ReportingGuidanceDefinition    `json:"guidance"`
	PerformanceReport PerformanceReportDefinition    `json:"performanceReport"`
	VMInventoryExport VMInventoryExportDefinition    `json:"vmInventoryExport"`
}

// SupportsFormat reports whether the performance reporting surface allows the
// given output format.
func (d PerformanceReportDefinition) SupportsFormat(format ReportFormat) bool {
	for _, candidate := range d.Formats {
		if candidate.Value == format {
			return true
		}
	}
	return false
}

// InvalidFormatError returns the canonical validation message for unsupported
// performance report formats.
func (d PerformanceReportDefinition) InvalidFormatError() string {
	allowed := make([]ReportFormat, 0, len(d.Formats))
	for _, candidate := range d.Formats {
		allowed = append(allowed, candidate.Value)
	}
	return invalidFormatErrorMessage(allowed)
}

// SingleAttachmentFilename returns the canonical attachment filename for a
// single-resource performance report download.
func (d PerformanceReportDefinition) SingleAttachmentFilename(resourceID string, generatedAt time.Time, format ReportFormat) string {
	return fmt.Sprintf("%s-%s-%s.%s", d.SingleFilenamePrefix, resourceID, reportingDateStamp(generatedAt), format)
}

// MultiAttachmentFilename returns the canonical attachment filename for a
// multi-resource performance report download.
func (d PerformanceReportDefinition) MultiAttachmentFilename(generatedAt time.Time, format ReportFormat) string {
	return fmt.Sprintf("%s-%s.%s", d.MultiFilenamePrefix, reportingDateStamp(generatedAt), format)
}

// DefaultRangeDuration returns the canonical fallback time window for the
// performance reporting surface.
func (d PerformanceReportDefinition) DefaultRangeDuration() time.Duration {
	for _, candidate := range d.Ranges {
		if candidate.Key == d.DefaultRange {
			return time.Duration(candidate.WindowHours) * time.Hour
		}
	}
	return 24 * time.Hour
}

func invalidFormatErrorMessage(allowed []ReportFormat) string {
	quoted := make([]string, 0, len(allowed))
	for _, format := range allowed {
		if format == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf("%q", format))
	}
	if len(quoted) == 0 {
		return "Format is not supported"
	}
	if len(quoted) == 1 {
		return fmt.Sprintf("Format must be %s", quoted[0])
	}
	if len(quoted) == 2 {
		return fmt.Sprintf("Format must be %s or %s", quoted[0], quoted[1])
	}
	return fmt.Sprintf("Format must be %s, or %s", strings.Join(quoted[:len(quoted)-1], ", "), quoted[len(quoted)-1])
}

func reportingDateStamp(generatedAt time.Time) string {
	return generatedAt.UTC().Format("20060102")
}

// DescribePerformanceReport returns the canonical definition for Pulse's
// historical performance reporting surface.
func DescribePerformanceReport() PerformanceReportDefinition {
	return PerformanceReportDefinition{
		ID:                     "performance_reports",
		Title:                  "Performance Reports",
		Description:            "Generate PDF summaries or CSV metric exports from historical monitoring data for one or more selected resources.",
		SingleResourceEndpoint: "/api/admin/reports/generate",
		MultiResourceEndpoint:  "/api/admin/reports/generate-multi",
		SingleFilenamePrefix:   "report",
		MultiFilenamePrefix:    "fleet-report",
		Formats: []ReportingFormatDefinition{
			{Value: FormatPDF, Label: "PDF Report"},
			{Value: FormatCSV, Label: "CSV Data"},
		},
		DefaultFormat: FormatPDF,
		Ranges: []ReportingRangeDefinition{
			{Key: "24h", Label: "Last 24 Hours", Description: "Current-day operational summary for short-term regressions.", WindowHours: 24},
			{Key: "7d", Label: "Last 7 Days", Description: "Weekly trend window for recent performance changes.", WindowHours: 168},
			{Key: "30d", Label: "Last 30 Days", Description: "Monthly review window for sustained capacity or reliability shifts.", WindowHours: 720},
		},
		DefaultRange:         "24h",
		MultiResourceMax:     50,
		SupportsMetricFilter: true,
		SupportsCustomTitle:  true,
	}
}

// DescribeReportingCatalog returns the canonical backend-owned settings
// definition for the advanced reporting feature.
func DescribeReportingCatalog() ReportingCatalog {
	return ReportingCatalog{
		ID:          "advanced_reporting",
		Title:       "Detailed Reporting",
		Description: "Generate performance reports and current-state exports across infrastructure and workloads.",
		LockedState: ReportingLockedStateDefinition{
			Title:       "Advanced Reporting (Pro)",
			Description: "Generate PDF and CSV performance reports plus current-state VM inventory exports across infrastructure and workload resources.",
		},
		Guidance: ReportingGuidanceDefinition{
			Title:       "Advanced Insights",
			Description: "Performance reports come from the historical metrics store, while VM inventory export captures the current runtime state for spreadsheet-friendly fleet reviews. Use reports for trends and the inventory export for current allocation and usage snapshots.",
		},
		PerformanceReport: DescribePerformanceReport(),
		VMInventoryExport: DescribeVMInventoryExport(),
	}
}
