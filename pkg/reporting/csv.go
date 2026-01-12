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
		return nil, err
	}

	// Write summary section
	if err := g.writeSummary(w, data); err != nil {
		return nil, err
	}

	// Write data section
	if err := g.writeData(w, data); err != nil {
		return nil, err
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
			return err
		}
	}

	return nil
}

// writeSummary writes the metrics summary section.
func (g *CSVGenerator) writeSummary(w *csv.Writer, data *ReportData) error {
	// Section header
	if err := w.Write([]string{"# SUMMARY"}); err != nil {
		return err
	}

	// Column headers
	if err := w.Write([]string{"Metric", "Count", "Min", "Max", "Average", "Current", "Unit"}); err != nil {
		return err
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
			return err
		}
	}

	// Empty row as separator
	if err := w.Write([]string{""}); err != nil {
		return err
	}

	return nil
}

// writeData writes the detailed metrics data section.
func (g *CSVGenerator) writeData(w *csv.Writer, data *ReportData) error {
	// Section header
	if err := w.Write([]string{"# DATA"}); err != nil {
		return err
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
		return err
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
			return err
		}
	}

	return nil
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
