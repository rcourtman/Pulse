package reporting

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// PDFGenerator handles PDF report generation.
type PDFGenerator struct{}

// NewPDFGenerator creates a new PDF generator.
func NewPDFGenerator() *PDFGenerator {
	return &PDFGenerator{}
}

// Generate creates a PDF report from the provided data.
func (g *PDFGenerator) Generate(data *ReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// Add first page
	pdf.AddPage()

	// Header
	g.writeHeader(pdf, data)

	// Summary section
	g.writeSummary(pdf, data)

	// Charts for each metric
	g.writeCharts(pdf, data)

	// Data table
	g.writeDataTable(pdf, data)

	// Footer with generation info
	g.writeFooter(pdf, data)

	// Output to buffer
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("PDF output error: %w", err)
	}

	return buf.Bytes(), nil
}

// writeHeader writes the report header.
func (g *PDFGenerator) writeHeader(pdf *fpdf.Fpdf, data *ReportData) {
	// Title
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(51, 51, 51)
	pdf.CellFormat(0, 12, "Pulse Metrics Report", "", 1, "C", false, 0, "")

	// Subtitle with title
	pdf.SetFont("Arial", "", 14)
	pdf.SetTextColor(102, 102, 102)
	pdf.CellFormat(0, 8, data.Title, "", 1, "C", false, 0, "")

	pdf.Ln(5)

	// Report details box
	pdf.SetFillColor(245, 245, 245)
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(51, 51, 51)

	boxX := 15.0
	boxWidth := 180.0
	boxHeight := 35.0

	pdf.Rect(boxX, pdf.GetY(), boxWidth, boxHeight, "FD")
	pdf.SetXY(boxX+5, pdf.GetY()+3)

	// Resource info
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 6, "Resource Type:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(50, 6, GetResourceTypeDisplayName(data.ResourceType), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 6, "Resource ID:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, data.ResourceID, "", 1, "L", false, 0, "")

	pdf.SetX(boxX + 5)

	// Time period
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 6, "Period:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	periodStr := fmt.Sprintf("%s to %s", data.Start.Format("2006-01-02 15:04"), data.End.Format("2006-01-02 15:04"))
	pdf.CellFormat(0, 6, periodStr, "", 1, "L", false, 0, "")

	pdf.SetX(boxX + 5)

	// Data points
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 6, "Data Points:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(50, 6, fmt.Sprintf("%d", data.TotalPoints), "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 6, "Generated:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, data.GeneratedAt.Format("2006-01-02 15:04:05"), "", 1, "L", false, 0, "")

	pdf.Ln(10)
}

// writeSummary writes the metrics summary table.
func (g *PDFGenerator) writeSummary(pdf *fpdf.Fpdf, data *ReportData) {
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(51, 51, 51)
	pdf.CellFormat(0, 10, "Summary", "", 1, "L", false, 0, "")

	// Table header
	pdf.SetFillColor(66, 139, 202)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10)

	colWidths := []float64{40, 25, 25, 25, 25, 25, 15}
	headers := []string{"Metric", "Count", "Min", "Max", "Avg", "Current", "Unit"}

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 8, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFillColor(255, 255, 255)
	pdf.SetTextColor(51, 51, 51)
	pdf.SetFont("Arial", "", 9)

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Summary.ByMetric))
	for name := range data.Summary.ByMetric {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	fill := false
	for _, metricType := range metricNames {
		stats := data.Summary.ByMetric[metricType]
		unit := GetMetricUnit(metricType)

		if fill {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		pdf.CellFormat(colWidths[0], 7, GetMetricTypeDisplayName(metricType), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[1], 7, fmt.Sprintf("%d", stats.Count), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[2], 7, formatValue(stats.Min, unit), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[3], 7, formatValue(stats.Max, unit), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[4], 7, formatValue(stats.Avg, unit), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[5], 7, formatValue(stats.Current, unit), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[6], 7, unit, "1", 0, "C", fill, 0, "")
		pdf.Ln(-1)

		fill = !fill
	}

	pdf.Ln(10)
}

// writeCharts writes simple line charts for each metric.
func (g *PDFGenerator) writeCharts(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Metrics) == 0 {
		return
	}

	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(51, 51, 51)
	pdf.CellFormat(0, 10, "Charts", "", 1, "L", false, 0, "")

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Metrics))
	for name := range data.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	chartWidth := 180.0
	chartHeight := 50.0
	colors := [][]int{
		{66, 139, 202},  // Blue
		{92, 184, 92},   // Green
		{240, 173, 78},  // Orange
		{217, 83, 79},   // Red
		{153, 102, 204}, // Purple
	}

	for i, metricType := range metricNames {
		points := data.Metrics[metricType]
		if len(points) < 2 {
			continue
		}

		// Check if we need a new page
		if pdf.GetY() > 220 {
			pdf.AddPage()
		}

		// Chart title
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(51, 51, 51)
		unit := GetMetricUnit(metricType)
		titleStr := GetMetricTypeDisplayName(metricType)
		if unit != "" {
			titleStr = fmt.Sprintf("%s (%s)", titleStr, unit)
		}
		pdf.CellFormat(0, 8, titleStr, "", 1, "L", false, 0, "")

		// Draw chart background
		chartX := 15.0
		chartY := pdf.GetY()

		pdf.SetFillColor(250, 250, 250)
		pdf.SetDrawColor(200, 200, 200)
		pdf.Rect(chartX, chartY, chartWidth, chartHeight, "FD")

		// Find min/max for scaling
		minVal, maxVal := points[0].Value, points[0].Value
		for _, p := range points {
			if p.Value < minVal {
				minVal = p.Value
			}
			if p.Value > maxVal {
				maxVal = p.Value
			}
		}

		// Add padding to min/max
		valRange := maxVal - minVal
		if valRange < 0.1 {
			valRange = 10 // Minimum range for flat lines
		}
		minVal = math.Max(0, minVal-valRange*0.1)
		maxVal = maxVal + valRange*0.1

		// Draw Y-axis labels
		pdf.SetFont("Arial", "", 7)
		pdf.SetTextColor(128, 128, 128)

		// Max label
		pdf.SetXY(chartX-12, chartY)
		pdf.CellFormat(10, 5, fmt.Sprintf("%.0f", maxVal), "", 0, "R", false, 0, "")

		// Min label
		pdf.SetXY(chartX-12, chartY+chartHeight-5)
		pdf.CellFormat(10, 5, fmt.Sprintf("%.0f", minVal), "", 0, "R", false, 0, "")

		// Draw line chart
		color := colors[i%len(colors)]
		pdf.SetDrawColor(color[0], color[1], color[2])
		pdf.SetLineWidth(0.5)

		startTime := points[0].Timestamp.Unix()
		endTime := points[len(points)-1].Timestamp.Unix()
		timeRange := float64(endTime - startTime)
		if timeRange == 0 {
			timeRange = 1
		}

		prevX, prevY := 0.0, 0.0
		for j, p := range points {
			// Calculate position
			xPos := chartX + 2 + (float64(p.Timestamp.Unix()-startTime)/timeRange)*(chartWidth-4)
			yPos := chartY + chartHeight - 2 - ((p.Value-minVal)/(maxVal-minVal))*(chartHeight-4)

			// Clamp Y position
			if yPos < chartY+2 {
				yPos = chartY + 2
			}
			if yPos > chartY+chartHeight-2 {
				yPos = chartY + chartHeight - 2
			}

			if j > 0 {
				pdf.Line(prevX, prevY, xPos, yPos)
			}
			prevX, prevY = xPos, yPos
		}

		// Draw X-axis labels (start and end time)
		pdf.SetFont("Arial", "", 7)
		pdf.SetTextColor(128, 128, 128)
		pdf.SetXY(chartX, chartY+chartHeight+1)
		pdf.CellFormat(40, 4, points[0].Timestamp.Format("01/02 15:04"), "", 0, "L", false, 0, "")
		pdf.SetXY(chartX+chartWidth-40, chartY+chartHeight+1)
		pdf.CellFormat(40, 4, points[len(points)-1].Timestamp.Format("01/02 15:04"), "", 0, "R", false, 0, "")

		pdf.SetY(chartY + chartHeight + 10)
	}

	pdf.Ln(5)
}

// writeDataTable writes a detailed data table (limited rows).
func (g *PDFGenerator) writeDataTable(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Metrics) == 0 {
		return
	}

	// Check if we need a new page
	if pdf.GetY() > 200 {
		pdf.AddPage()
	}

	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(51, 51, 51)
	pdf.CellFormat(0, 10, "Data Sample", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(128, 128, 128)
	pdf.CellFormat(0, 5, "Showing first 50 data points. Export as CSV for complete data.", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Metrics))
	for name := range data.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Build columns
	numCols := len(metricNames) + 1 // +1 for timestamp
	colWidth := 180.0 / float64(numCols)
	if colWidth < 25 {
		colWidth = 25
	}

	// Table header
	pdf.SetFillColor(66, 139, 202)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	pdf.CellFormat(35, 6, "Timestamp", "1", 0, "C", true, 0, "")
	for _, name := range metricNames {
		displayName := GetMetricTypeDisplayName(name)
		if len(displayName) > 12 {
			displayName = displayName[:12]
		}
		pdf.CellFormat(colWidth, 6, displayName, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Collect all timestamps
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

	timestamps := make([]int64, 0, len(timestampSet))
	for ts := range timestampSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// Limit to 50 rows
	if len(timestamps) > 50 {
		timestamps = timestamps[:50]
	}

	// Table rows
	pdf.SetTextColor(51, 51, 51)
	pdf.SetFont("Arial", "", 8)
	fill := false

	for _, ts := range timestamps {
		// Check page break
		if pdf.GetY() > 270 {
			pdf.AddPage()
			// Re-draw header
			pdf.SetFillColor(66, 139, 202)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetFont("Arial", "B", 8)
			pdf.CellFormat(35, 6, "Timestamp", "1", 0, "C", true, 0, "")
			for _, name := range metricNames {
				displayName := GetMetricTypeDisplayName(name)
				if len(displayName) > 12 {
					displayName = displayName[:12]
				}
				pdf.CellFormat(colWidth, 6, displayName, "1", 0, "C", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetTextColor(51, 51, 51)
			pdf.SetFont("Arial", "", 8)
			fill = false
		}

		if fill {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		t := time.Unix(ts, 0)
		pdf.CellFormat(35, 5, t.Format("01/02 15:04:05"), "1", 0, "L", fill, 0, "")

		for _, metricName := range metricNames {
			if val, ok := metricsByTime[metricName][ts]; ok {
				pdf.CellFormat(colWidth, 5, fmt.Sprintf("%.2f", val), "1", 0, "C", fill, 0, "")
			} else {
				pdf.CellFormat(colWidth, 5, "-", "1", 0, "C", fill, 0, "")
			}
		}
		pdf.Ln(-1)
		fill = !fill
	}
}

// writeFooter writes the report footer.
func (g *PDFGenerator) writeFooter(pdf *fpdf.Fpdf, data *ReportData) {
	pdf.SetY(-20)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(128, 128, 128)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated by Pulse - %s", data.GeneratedAt.Format(time.RFC3339)), "", 0, "C", false, 0, "")
}
