package reporting

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"time"
)

// CSVGenerator handles CSV report generation.
type CSVGenerator struct{}

// NewCSVGenerator creates a new CSV generator.
func NewCSVGenerator() *CSVGenerator {
	return &CSVGenerator{}
}

// Generate creates a CSV report from the provided data.
func (g *CSVGenerator) Generate(data *ReportData) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header comment rows
	if err := g.writeHeader(w, data); err != nil {
		return nil, fmt.Errorf("write CSV header section: %w", err)
	}

	// Write summary section
	if err := g.writeSummary(w, data); err != nil {
		return nil, fmt.Errorf("write CSV summary section: %w", err)
	}

	// Write data section
	if err := g.writeData(w, data); err != nil {
		return nil, fmt.Errorf("write CSV data section: %w", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}

	return buf.Bytes(), nil
}

// writeHeader writes the report header information.
func (g *CSVGenerator) writeHeader(w *csv.Writer, data *ReportData) error {
	headers := [][]string{
		{"# Pulse Metrics Report"},
		{"# Title:", data.Title},
		{"# Resource Type:", GetResourceTypeDisplayName(data.ResourceType)},
		{"# Resource ID:", data.ResourceID},
		{"# Period:", fmt.Sprintf("%s to %s", data.Start.Format(time.RFC3339), data.End.Format(time.RFC3339))},
		{"# Generated:", data.GeneratedAt.Format(time.RFC3339)},
		{"# Total Data Points:", fmt.Sprintf("%d", data.TotalPoints)},
		{""}, // Empty row as separator
	}

	for _, row := range headers {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write header row %q: %w", row[0], err)
		}
	}

	return nil
}

// writeSummary writes the metrics summary section.
func (g *CSVGenerator) writeSummary(w *csv.Writer, data *ReportData) error {
	// Section header
	if err := w.Write([]string{"# SUMMARY"}); err != nil {
		return fmt.Errorf("write summary section heading: %w", err)
	}

	// Column headers
	if err := w.Write([]string{"Metric", "Count", "Min", "Max", "Average", "Current", "Unit"}); err != nil {
		return fmt.Errorf("write summary column headers: %w", err)
	}

	// Get sorted metric names for consistent output
	metricNames := make([]string, 0, len(data.Summary.ByMetric))
	for name := range data.Summary.ByMetric {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Write summary rows
	for _, metricType := range metricNames {
		stats := data.Summary.ByMetric[metricType]
		unit := GetMetricUnit(metricType)
		row := []string{
			GetMetricTypeDisplayName(metricType),
			fmt.Sprintf("%d", stats.Count),
			formatValue(stats.Min, unit),
			formatValue(stats.Max, unit),
			formatValue(stats.Avg, unit),
			formatValue(stats.Current, unit),
			unit,
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write summary row for metric %q: %w", metricType, err)
		}
	}

	// Empty row as separator
	if err := w.Write([]string{""}); err != nil {
		return fmt.Errorf("write summary separator row: %w", err)
	}

	return nil
}

// writeData writes the detailed metrics data section.
func (g *CSVGenerator) writeData(w *csv.Writer, data *ReportData) error {
	// Section header
	if err := w.Write([]string{"# DATA"}); err != nil {
		return fmt.Errorf("write data section heading: %w", err)
	}

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Metrics))
	for name := range data.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Build header row with timestamp + all metrics
	headerRow := []string{"Timestamp"}
	for _, name := range metricNames {
		unit := GetMetricUnit(name)
		if unit != "" {
			headerRow = append(headerRow, fmt.Sprintf("%s (%s)", GetMetricTypeDisplayName(name), unit))
		} else {
			headerRow = append(headerRow, GetMetricTypeDisplayName(name))
		}
	}
	if err := w.Write(headerRow); err != nil {
		return fmt.Errorf("write data column headers: %w", err)
	}

	// Collect all unique timestamps and build a map for lookup
	timestampSet := make(map[int64]bool)
	metricsByTime := make(map[string]map[int64]float64)

	for metricName, points := range data.Metrics {
		metricsByTime[metricName] = make(map[int64]float64)
		for _, p := range points {
			ts := p.Timestamp.Unix()
			timestampSet[ts] = true
			metricsByTime[metricName][ts] = p.Value
		}
	}

	// Sort timestamps
	timestamps := make([]int64, 0, len(timestampSet))
	for ts := range timestampSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// Write data rows
	for _, ts := range timestamps {
		t := time.Unix(ts, 0)
		row := []string{t.Format(time.RFC3339)}

		for _, metricName := range metricNames {
			if val, ok := metricsByTime[metricName][ts]; ok {
				row = append(row, fmt.Sprintf("%.2f", val))
			} else {
				row = append(row, "") // Missing data point
			}
		}

		if err := w.Write(row); err != nil {
			return fmt.Errorf("write data row for timestamp %d: %w", ts, err)
		}
	}

	return nil
}

// GenerateMulti creates a multi-resource CSV report from the provided data.
func (g *CSVGenerator) GenerateMulti(data *MultiReportData) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header comment rows
	headers := [][]string{
		{"# Pulse Multi-Resource Metrics Report"},
		{"# Title:", data.Title},
		{"# Resources:", fmt.Sprintf("%d", len(data.Resources))},
		{"# Period:", fmt.Sprintf("%s to %s", data.Start.Format(time.RFC3339), data.End.Format(time.RFC3339))},
		{"# Generated:", data.GeneratedAt.Format(time.RFC3339)},
		{"# Total Data Points:", fmt.Sprintf("%d", data.TotalPoints)},
		{""},
	}
	for _, row := range headers {
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write multi-resource header row %q: %w", row[0], err)
		}
	}

	// Write summary section
	if err := w.Write([]string{"# SUMMARY"}); err != nil {
		return nil, fmt.Errorf("write multi-resource summary section heading: %w", err)
	}

	// Summary column headers
	summaryHeaders := []string{"Resource Name", "Resource Type", "Resource ID", "Metric", "Count", "Min", "Max", "Average", "Current", "Unit"}
	if err := w.Write(summaryHeaders); err != nil {
		return nil, fmt.Errorf("write multi-resource summary column headers: %w", err)
	}

	// Write summary rows for each resource
	for _, rd := range data.Resources {
		resourceName := rd.ResourceID
		if rd.Resource != nil && rd.Resource.Name != "" {
			resourceName = rd.Resource.Name
		}
		resourceTypeDisplay := GetResourceTypeDisplayName(rd.ResourceType)

		metricNames := make([]string, 0, len(rd.Summary.ByMetric))
		for name := range rd.Summary.ByMetric {
			metricNames = append(metricNames, name)
		}
		sort.Strings(metricNames)

		for _, metricType := range metricNames {
			stats := rd.Summary.ByMetric[metricType]
			unit := GetMetricUnit(metricType)
			row := []string{
				resourceName,
				resourceTypeDisplay,
				rd.ResourceID,
				GetMetricTypeDisplayName(metricType),
				fmt.Sprintf("%d", stats.Count),
				formatValue(stats.Min, unit),
				formatValue(stats.Max, unit),
				formatValue(stats.Avg, unit),
				formatValue(stats.Current, unit),
				unit,
			}
			if err := w.Write(row); err != nil {
				return nil, fmt.Errorf("write multi-resource summary row for resource %q metric %q: %w", rd.ResourceID, metricType, err)
			}
		}
	}

	// Empty separator
	if err := w.Write([]string{""}); err != nil {
		return nil, fmt.Errorf("write multi-resource summary separator row: %w", err)
	}

	// Write data section
	if err := w.Write([]string{"# DATA"}); err != nil {
		return nil, fmt.Errorf("write multi-resource data section heading: %w", err)
	}

	// Collect all unique metric names across all resources
	metricNameSet := make(map[string]bool)
	for _, rd := range data.Resources {
		for name := range rd.Metrics {
			metricNameSet[name] = true
		}
	}
	metricNames := make([]string, 0, len(metricNameSet))
	for name := range metricNameSet {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Build data header row
	headerRow := []string{"Timestamp", "Resource Name", "Resource Type", "Resource ID"}
	for _, name := range metricNames {
		unit := GetMetricUnit(name)
		if unit != "" {
			headerRow = append(headerRow, fmt.Sprintf("%s (%s)", GetMetricTypeDisplayName(name), unit))
		} else {
			headerRow = append(headerRow, GetMetricTypeDisplayName(name))
		}
	}
	if err := w.Write(headerRow); err != nil {
		return nil, fmt.Errorf("write multi-resource data column headers: %w", err)
	}

	// Collect all data points across all resources, with resource info
	type dataRow struct {
		timestamp    int64
		resourceName string
		resourceType string
		resourceID   string
		values       map[string]float64
	}

	var allRows []dataRow
	for _, rd := range data.Resources {
		resourceName := rd.ResourceID
		if rd.Resource != nil && rd.Resource.Name != "" {
			resourceName = rd.Resource.Name
		}
		resourceTypeDisplay := GetResourceTypeDisplayName(rd.ResourceType)

		// Build a map of timestamp -> metric values for this resource
		timestampValues := make(map[int64]map[string]float64)
		for metricName, points := range rd.Metrics {
			for _, p := range points {
				ts := p.Timestamp.Unix()
				if timestampValues[ts] == nil {
					timestampValues[ts] = make(map[string]float64)
				}
				timestampValues[ts][metricName] = p.Value
			}
		}

		for ts, values := range timestampValues {
			allRows = append(allRows, dataRow{
				timestamp:    ts,
				resourceName: resourceName,
				resourceType: resourceTypeDisplay,
				resourceID:   rd.ResourceID,
				values:       values,
			})
		}
	}

	// Sort by timestamp, then by resource name
	sort.Slice(allRows, func(i, j int) bool {
		if allRows[i].timestamp != allRows[j].timestamp {
			return allRows[i].timestamp < allRows[j].timestamp
		}
		return allRows[i].resourceName < allRows[j].resourceName
	})

	// Write data rows
	for _, row := range allRows {
		t := time.Unix(row.timestamp, 0)
		csvRow := []string{t.Format(time.RFC3339), row.resourceName, row.resourceType, row.resourceID}
		for _, metricName := range metricNames {
			if val, ok := row.values[metricName]; ok {
				csvRow = append(csvRow, fmt.Sprintf("%.2f", val))
			} else {
				csvRow = append(csvRow, "")
			}
		}
		if err := w.Write(csvRow); err != nil {
			return nil, fmt.Errorf("write multi-resource data row for resource %q at timestamp %d: %w", row.resourceID, row.timestamp, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}

	return buf.Bytes(), nil
}

// formatValue formats a metric value with appropriate precision.
func formatValue(value float64, unit string) string {
	if unit == "bytes" {
		return formatBytes(value)
	}
	return fmt.Sprintf("%.2f", value)
}

// formatBytes converts bytes to human-readable format.
func formatBytes(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := float64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", bytes/div, "KMGTPE"[exp])
}
