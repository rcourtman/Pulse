package reporting

import "time"

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
type ReportingCatalog struct {
	ID                string                      `json:"id"`
	Title             string                      `json:"title"`
	Description       string                      `json:"description"`
	PerformanceReport PerformanceReportDefinition `json:"performanceReport"`
	VMInventoryExport VMInventoryExportDefinition `json:"vmInventoryExport"`
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
		ID:                "advanced_reporting",
		Title:             "Detailed Reporting",
		Description:       "Generate performance reports and current-state exports across infrastructure and workloads.",
		PerformanceReport: DescribePerformanceReport(),
		VMInventoryExport: DescribeVMInventoryExport(),
	}
}
